package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kulich00/negotiation-arena/backend/internal/domain"
)

type PostgresRepository struct{ db *pgxpool.Pool }

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository { return &PostgresRepository{db: db} }

func (r *PostgresRepository) ListScenarios(ctx context.Context) ([]domain.Scenario, error) {
	rows, err := r.db.Query(ctx, `SELECT id,title,sphere,topic,difficulty,opponent_role,opponent_tone,player_goal,opponent_goal,initial_message FROM scenarios ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Scenario, 0)
	for rows.Next() {
		var s domain.Scenario
		if err := rows.Scan(&s.ID, &s.Title, &s.Sphere, &s.Topic, &s.Difficulty, &s.OpponentRole, &s.OpponentTone, &s.PlayerGoal, &s.OpponentGoal, &s.InitialMessage); err != nil {
			return nil, err
		}
		items = append(items, s)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) SaveScenario(ctx context.Context, s domain.Scenario) error {
	_, err := r.db.Exec(ctx, `INSERT INTO scenarios (id,title,sphere,topic,difficulty,opponent_role,opponent_tone,player_goal,opponent_goal,initial_message) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT (id) DO NOTHING`, s.ID, s.Title, s.Sphere, s.Topic, s.Difficulty, s.OpponentRole, s.OpponentTone, s.PlayerGoal, s.OpponentGoal, s.InitialMessage)
	return err
}

func (r *PostgresRepository) Scenario(ctx context.Context, id string) (domain.Scenario, error) {
	var s domain.Scenario
	err := r.db.QueryRow(ctx, `SELECT id,title,sphere,topic,difficulty,opponent_role,opponent_tone,player_goal,opponent_goal,initial_message FROM scenarios WHERE id=$1`, id).Scan(&s.ID, &s.Title, &s.Sphere, &s.Topic, &s.Difficulty, &s.OpponentRole, &s.OpponentTone, &s.PlayerGoal, &s.OpponentGoal, &s.InitialMessage)
	return s, notFound(err)
}

func (r *PostgresRepository) SaveSession(ctx context.Context, s domain.Session) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO negotiation_sessions (id,scenario_id,status,turn,trust_score,argument_score,pressure_score,started_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, s.ID, s.ScenarioID, s.Status, s.Turn, s.TrustScore, s.ArgumentScore, s.PressureScore, s.StartedAt)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO messages (session_id,sender,content) VALUES ($1,'opponent',$2)`, s.ID, s.InitialMessage)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) Session(ctx context.Context, id string) (domain.Session, error) {
	var s domain.Session
	err := r.db.QueryRow(ctx, `SELECT n.id,n.scenario_id,n.status,n.turn,n.trust_score,n.argument_score,n.pressure_score,s.initial_message,n.started_at FROM negotiation_sessions n JOIN scenarios s ON s.id=n.scenario_id WHERE n.id=$1`, id).Scan(&s.ID, &s.ScenarioID, &s.Status, &s.Turn, &s.TrustScore, &s.ArgumentScore, &s.PressureScore, &s.InitialMessage, &s.StartedAt)
	return s, notFound(err)
}

func (r *PostgresRepository) ApplyTurn(ctx context.Context, s domain.Session, expectedTurn int, message, reply string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	command, err := tx.Exec(ctx, `UPDATE negotiation_sessions SET turn=$2,trust_score=$3,argument_score=$4,pressure_score=$5 WHERE id=$1 AND status='active' AND turn=$6`, s.ID, s.Turn, s.TrustScore, s.ArgumentScore, s.PressureScore, expectedTurn)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return ErrConflict
	}
	_, err = tx.Exec(ctx, `INSERT INTO messages (session_id,sender,content) VALUES ($1,'player',$2),($1,'opponent',$3)`, s.ID, message, reply)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) Messages(ctx context.Context, id string) ([]domain.Message, error) {
	rows, err := r.db.Query(ctx, `SELECT sender,content FROM messages WHERE session_id=$1 ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Message, 0)
	for rows.Next() {
		var m domain.Message
		if err := rows.Scan(&m.Sender, &m.Content); err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) Finish(ctx context.Context, s domain.Session, result domain.Result) error {
	strengths, err := json.Marshal(result.Strengths)
	if err != nil {
		return err
	}
	mistakes, err := json.Marshal(result.Mistakes)
	if err != nil {
		return err
	}
	recommendations, err := json.Marshal(result.Recommendations)
	if err != nil {
		return err
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	command, err := tx.Exec(ctx, `UPDATE negotiation_sessions SET status='finished',finished_at=now() WHERE id=$1 AND status='active' AND turn=$2`, s.ID, s.Turn)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return ErrConflict
	}
	_, err = tx.Exec(ctx, `INSERT INTO results (session_id,final_score,outcome,strengths,mistakes,recommendations) VALUES ($1,$2,$3,$4,$5::jsonb,$6::jsonb)`, result.SessionID, result.FinalScore, result.Outcome, string(strengths), string(mistakes), string(recommendations))
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) Result(ctx context.Context, id string) (domain.Result, error) {
	var result domain.Result
	var strengths, mistakes, recommendations []byte
	err := r.db.QueryRow(ctx, `SELECT session_id,final_score,outcome,strengths,mistakes,recommendations FROM results WHERE session_id=$1`, id).Scan(&result.SessionID, &result.FinalScore, &result.Outcome, &strengths, &mistakes, &recommendations)
	if err != nil {
		return result, notFound(err)
	}
	if err := json.Unmarshal(strengths, &result.Strengths); err != nil {
		return result, err
	}
	if err := json.Unmarshal(mistakes, &result.Mistakes); err != nil {
		return result, err
	}
	if err := json.Unmarshal(recommendations, &result.Recommendations); err != nil {
		return result, err
	}
	return result, nil
}

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
