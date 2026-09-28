package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kulich00/negotiation-arena/backend/internal/domain"
)

type PostgresRepository struct{ db *pgxpool.Pool }

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository { return &PostgresRepository{db: db} }

func (r *PostgresRepository) CreatePlayer(ctx context.Context, player domain.PlayerProfile) error {
	command, err := r.db.Exec(ctx, `INSERT INTO player_profiles
		(id,display_name,completed_sessions,successful_sessions,current_win_streak,best_win_streak,unlocked_difficulty,created_at,updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT (id) DO NOTHING`,
		player.ID, player.DisplayName, player.CompletedSessions, player.SuccessfulSessions,
		player.CurrentWinStreak, player.BestWinStreak, player.UnlockedDifficulty,
		player.CreatedAt, player.UpdatedAt)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return ErrConflict
	}
	return nil
}

func (r *PostgresRepository) Player(ctx context.Context, id string) (domain.PlayerProfile, error) {
	var player domain.PlayerProfile
	err := r.db.QueryRow(ctx, `SELECT id,display_name,completed_sessions,successful_sessions,current_win_streak,best_win_streak,unlocked_difficulty,created_at,updated_at
		FROM player_profiles WHERE id=$1`, id).Scan(
		&player.ID, &player.DisplayName, &player.CompletedSessions, &player.SuccessfulSessions,
		&player.CurrentWinStreak, &player.BestWinStreak, &player.UnlockedDifficulty,
		&player.CreatedAt, &player.UpdatedAt,
	)
	if err != nil {
		return domain.PlayerProfile{}, notFound(err)
	}
	rows, err := r.db.Query(ctx, `SELECT code,title,description,unlocked_at
		FROM player_achievements WHERE player_id=$1 ORDER BY unlocked_at,code`, id)
	if err != nil {
		return domain.PlayerProfile{}, err
	}
	defer rows.Close()
	player.Achievements = []domain.UnlockedAchievement{}
	for rows.Next() {
		var achievement domain.UnlockedAchievement
		if err := rows.Scan(&achievement.Code, &achievement.Title, &achievement.Description, &achievement.UnlockedAt); err != nil {
			return domain.PlayerProfile{}, err
		}
		player.Achievements = append(player.Achievements, achievement)
	}
	return player, rows.Err()
}

func (r *PostgresRepository) SetAdminPassword(ctx context.Context, email, passwordHash string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `INSERT INTO admins (email,password_hash) VALUES ($1,$2)
		ON CONFLICT (email) DO UPDATE SET password_hash=EXCLUDED.password_hash, updated_at=now()`, email, passwordHash); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM admin_sessions WHERE admin_email=$1`, email); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) AdminPasswordHash(ctx context.Context, email string) (string, error) {
	var passwordHash string
	err := r.db.QueryRow(ctx, `SELECT password_hash FROM admins WHERE email=$1`, email).Scan(&passwordHash)
	return passwordHash, notFound(err)
}

func (r *PostgresRepository) SaveAdminSession(ctx context.Context, tokenHash, email string, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx, `INSERT INTO admin_sessions (token_hash,admin_email,expires_at) VALUES ($1,$2,$3)`, tokenHash, email, expiresAt)
	return err
}

func (r *PostgresRepository) ValidAdminSession(ctx context.Context, tokenHash string, now time.Time) (bool, error) {
	var valid bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM admin_sessions WHERE token_hash=$1 AND expires_at>$2)`, tokenHash, now).Scan(&valid)
	return valid, err
}

func (r *PostgresRepository) DeleteAdminSession(ctx context.Context, tokenHash string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM admin_sessions WHERE token_hash=$1`, tokenHash)
	return err
}

func (r *PostgresRepository) DeleteExpiredAdminSessions(ctx context.Context, now time.Time) error {
	_, err := r.db.Exec(ctx, `DELETE FROM admin_sessions WHERE expires_at<=$1`, now)
	return err
}

func (r *PostgresRepository) ListScenarios(ctx context.Context) ([]domain.Scenario, error) {
	rows, err := r.db.Query(ctx, `SELECT id,title,sphere,topic,difficulty,opponent_role,opponent_tone,player_goal,opponent_goal,initial_message,rules FROM scenarios ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Scenario, 0)
	for rows.Next() {
		var s domain.Scenario
		var rules []byte
		if err := rows.Scan(&s.ID, &s.Title, &s.Sphere, &s.Topic, &s.Difficulty, &s.OpponentRole, &s.OpponentTone, &s.PlayerGoal, &s.OpponentGoal, &s.InitialMessage, &rules); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(rules, &s.Rules); err != nil {
			return nil, err
		}
		s.Rules = s.Rules.WithDefaults()
		items = append(items, s)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) SaveScenario(ctx context.Context, s domain.Scenario) error {
	s.Rules = s.Rules.WithDefaults()
	rules, err := json.Marshal(s.Rules)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, `INSERT INTO scenarios (id,title,sphere,topic,difficulty,opponent_role,opponent_tone,player_goal,opponent_goal,initial_message,rules) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::jsonb) ON CONFLICT (id) DO NOTHING`, s.ID, s.Title, s.Sphere, s.Topic, s.Difficulty, s.OpponentRole, s.OpponentTone, s.PlayerGoal, s.OpponentGoal, s.InitialMessage, string(rules))
	return err
}

func (r *PostgresRepository) CreateScenario(ctx context.Context, s domain.Scenario) error {
	s.Rules = s.Rules.WithDefaults()
	rules, err := json.Marshal(s.Rules)
	if err != nil {
		return err
	}
	command, err := r.db.Exec(ctx, `INSERT INTO scenarios (id,title,sphere,topic,difficulty,opponent_role,opponent_tone,player_goal,opponent_goal,initial_message,rules) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::jsonb) ON CONFLICT (id) DO NOTHING`, s.ID, s.Title, s.Sphere, s.Topic, s.Difficulty, s.OpponentRole, s.OpponentTone, s.PlayerGoal, s.OpponentGoal, s.InitialMessage, string(rules))
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return ErrConflict
	}
	return nil
}

func (r *PostgresRepository) UpdateScenario(ctx context.Context, s domain.Scenario) error {
	s.Rules = s.Rules.WithDefaults()
	rules, err := json.Marshal(s.Rules)
	if err != nil {
		return err
	}
	command, err := r.db.Exec(ctx, `UPDATE scenarios SET title=$2,sphere=$3,topic=$4,difficulty=$5,opponent_role=$6,opponent_tone=$7,player_goal=$8,opponent_goal=$9,initial_message=$10,rules=$11::jsonb
		WHERE id=$1 AND NOT EXISTS (SELECT 1 FROM negotiation_sessions WHERE scenario_id=$1 AND status='active')`, s.ID, s.Title, s.Sphere, s.Topic, s.Difficulty, s.OpponentRole, s.OpponentTone, s.PlayerGoal, s.OpponentGoal, s.InitialMessage, string(rules))
	if err != nil {
		return err
	}
	if command.RowsAffected() == 1 {
		return nil
	}
	var exists bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM scenarios WHERE id=$1)`, s.ID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	return ErrConflict
}

func (r *PostgresRepository) DeleteScenario(ctx context.Context, id string) error {
	command, err := r.db.Exec(ctx, `DELETE FROM scenarios WHERE id=$1`, id)
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23503" {
			return ErrConflict
		}
		return err
	}
	if command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) Scenario(ctx context.Context, id string) (domain.Scenario, error) {
	var s domain.Scenario
	var rules []byte
	err := r.db.QueryRow(ctx, `SELECT id,title,sphere,topic,difficulty,opponent_role,opponent_tone,player_goal,opponent_goal,initial_message,rules FROM scenarios WHERE id=$1`, id).Scan(&s.ID, &s.Title, &s.Sphere, &s.Topic, &s.Difficulty, &s.OpponentRole, &s.OpponentTone, &s.PlayerGoal, &s.OpponentGoal, &s.InitialMessage, &rules)
	if err != nil {
		return s, notFound(err)
	}
	if err := json.Unmarshal(rules, &s.Rules); err != nil {
		return s, err
	}
	s.Rules = s.Rules.WithDefaults()
	return s, nil
}

func (r *PostgresRepository) SaveSession(ctx context.Context, s domain.Session) error {
	if s.State.Phase == "" {
		s.State = domain.InitialSessionState()
	}
	state, err := json.Marshal(s.State)
	if err != nil {
		return err
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO negotiation_sessions (id,scenario_id,player_id,parent_session_id,forked_from_turn,status,turn,trust_score,argument_score,pressure_score,initial_message,started_at,finished_at,state) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14::jsonb)`, s.ID, s.ScenarioID, nullIfEmpty(s.PlayerID), nullIfEmpty(s.ParentSessionID), s.ForkedFromTurn, s.Status, s.Turn, s.TrustScore, s.ArgumentScore, s.PressureScore, s.InitialMessage, s.StartedAt, s.FinishedAt, string(state))
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO messages (session_id,sender,content) VALUES ($1,'opponent',$2)`, s.ID, s.InitialMessage)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO session_checkpoints (session_id,turn,trust_score,argument_score,pressure_score,state,created_at)
		VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7)`, s.ID, s.Turn, s.TrustScore, s.ArgumentScore, s.PressureScore, string(state), s.StartedAt)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) ForkSession(ctx context.Context, parentID string, child domain.Session) error {
	state, err := json.Marshal(child.State)
	if err != nil {
		return err
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var checkpointExists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM session_checkpoints WHERE session_id=$1 AND turn=$2)`, parentID, child.Turn).Scan(&checkpointExists); err != nil {
		return err
	}
	if !checkpointExists {
		return ErrNotFound
	}
	_, err = tx.Exec(ctx, `INSERT INTO negotiation_sessions (id,scenario_id,player_id,parent_session_id,forked_from_turn,status,turn,trust_score,argument_score,pressure_score,initial_message,started_at,state)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::jsonb)`, child.ID, child.ScenarioID, nullIfEmpty(child.PlayerID), parentID, child.ForkedFromTurn, child.Status, child.Turn, child.TrustScore, child.ArgumentScore, child.PressureScore, child.InitialMessage, child.StartedAt, string(state))
	if err != nil {
		return err
	}
	messageCount := 1 + 2*child.Turn
	command, err := tx.Exec(ctx, `INSERT INTO messages (session_id,sender,content,analysis,created_at)
		SELECT $2,sender,content,analysis,created_at FROM messages
		WHERE session_id=$1 ORDER BY id LIMIT $3`, parentID, child.ID, messageCount)
	if err != nil {
		return err
	}
	if command.RowsAffected() != int64(messageCount) {
		return ErrConflict
	}
	_, err = tx.Exec(ctx, `INSERT INTO session_checkpoints (session_id,turn,trust_score,argument_score,pressure_score,state,created_at)
		SELECT $2,turn,
			CASE WHEN turn=$3 THEN $4 ELSE trust_score END,
			CASE WHEN turn=$3 THEN $5 ELSE argument_score END,
			CASE WHEN turn=$3 THEN $6 ELSE pressure_score END,
			CASE WHEN turn=$3 THEN $7::jsonb ELSE state END,
			created_at
		FROM session_checkpoints WHERE session_id=$1 AND turn<=$3 ORDER BY turn`, parentID, child.ID, child.Turn, child.TrustScore, child.ArgumentScore, child.PressureScore, string(state))
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) Session(ctx context.Context, id string) (domain.Session, error) {
	var s domain.Session
	var state []byte
	err := r.db.QueryRow(ctx, `SELECT id,scenario_id,COALESCE(player_id,''),COALESCE(parent_session_id,''),forked_from_turn,status,turn,trust_score,argument_score,pressure_score,initial_message,started_at,finished_at,state FROM negotiation_sessions WHERE id=$1`, id).Scan(&s.ID, &s.ScenarioID, &s.PlayerID, &s.ParentSessionID, &s.ForkedFromTurn, &s.Status, &s.Turn, &s.TrustScore, &s.ArgumentScore, &s.PressureScore, &s.InitialMessage, &s.StartedAt, &s.FinishedAt, &state)
	if err != nil {
		return s, notFound(err)
	}
	return s, json.Unmarshal(state, &s.State)
}

func (r *PostgresRepository) ApplyTurn(ctx context.Context, s domain.Session, expectedTurn int, message, reply string, analysis domain.TurnAnalysis) error {
	state, err := json.Marshal(s.State)
	if err != nil {
		return err
	}
	analysisJSON, err := json.Marshal(analysis)
	if err != nil {
		return err
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	command, err := tx.Exec(ctx, `UPDATE negotiation_sessions SET turn=$2,trust_score=$3,argument_score=$4,pressure_score=$5,state=$7::jsonb WHERE id=$1 AND status='active' AND turn=$6`, s.ID, s.Turn, s.TrustScore, s.ArgumentScore, s.PressureScore, expectedTurn, string(state))
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return ErrConflict
	}
	_, err = tx.Exec(ctx, `INSERT INTO messages (session_id,sender,content,analysis) VALUES ($1,'player',$2,$4::jsonb),($1,'opponent',$3,NULL)`, s.ID, message, reply, string(analysisJSON))
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO session_checkpoints (session_id,turn,trust_score,argument_score,pressure_score,state)
		VALUES ($1,$2,$3,$4,$5,$6::jsonb)`, s.ID, s.Turn, s.TrustScore, s.ArgumentScore, s.PressureScore, string(state))
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) Messages(ctx context.Context, id string) ([]domain.Message, error) {
	rows, err := r.db.Query(ctx, `SELECT sender,content,analysis FROM messages WHERE session_id=$1 ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Message, 0)
	for rows.Next() {
		var m domain.Message
		var analysisJSON []byte
		if err := rows.Scan(&m.Sender, &m.Content, &analysisJSON); err != nil {
			return nil, err
		}
		if len(analysisJSON) > 0 {
			var analysis domain.TurnAnalysis
			if err := json.Unmarshal(analysisJSON, &analysis); err != nil {
				return nil, err
			}
			m.Analysis = &analysis
		}
		items = append(items, m)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) Checkpoints(ctx context.Context, id string) ([]domain.TurnCheckpoint, error) {
	rows, err := r.db.Query(ctx, `SELECT turn,trust_score,argument_score,pressure_score,state,created_at
		FROM session_checkpoints WHERE session_id=$1 ORDER BY turn`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.TurnCheckpoint, 0)
	for rows.Next() {
		var checkpoint domain.TurnCheckpoint
		var state []byte
		if err := rows.Scan(&checkpoint.Turn, &checkpoint.TrustScore, &checkpoint.ArgumentScore, &checkpoint.PressureScore, &state, &checkpoint.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(state, &checkpoint.State); err != nil {
			return nil, err
		}
		items = append(items, checkpoint)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, ErrNotFound
	}
	return items, nil
}

func (r *PostgresRepository) Finish(ctx context.Context, s domain.Session, result domain.Result) error {
	state, err := json.Marshal(s.State)
	if err != nil {
		return err
	}
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
	achievements, err := json.Marshal(result.Achievements)
	if err != nil {
		return err
	}
	analysis, err := json.Marshal(result.Analysis)
	if err != nil {
		return err
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	command, err := tx.Exec(ctx, `UPDATE negotiation_sessions SET status=$3,finished_at=$4,state=$5::jsonb WHERE id=$1 AND status='active' AND turn=$2`, s.ID, s.Turn, s.Status, s.FinishedAt, string(state))
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return ErrConflict
	}
	_, err = tx.Exec(ctx, `INSERT INTO results (session_id,final_score,outcome_code,outcome,strengths,mistakes,recommendations,achievements,analysis) VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7::jsonb,$8::jsonb,$9::jsonb)`, result.SessionID, result.FinalScore, result.OutcomeCode, result.Outcome, string(strengths), string(mistakes), string(recommendations), string(achievements), string(analysis))
	if err != nil {
		return err
	}
	if s.PlayerID != "" {
		finishedAt := time.Now().UTC()
		if s.FinishedAt != nil {
			finishedAt = *s.FinishedAt
		}
		var player domain.PlayerProfile
		err := tx.QueryRow(ctx, `SELECT completed_sessions,successful_sessions,current_win_streak,best_win_streak,unlocked_difficulty
			FROM player_profiles WHERE id=$1 FOR UPDATE`, s.PlayerID).Scan(
			&player.CompletedSessions, &player.SuccessfulSessions, &player.CurrentWinStreak,
			&player.BestWinStreak, &player.UnlockedDifficulty,
		)
		if err != nil {
			return notFound(err)
		}
		var completedDifficulty string
		if err := tx.QueryRow(ctx, `SELECT difficulty FROM scenarios WHERE id=$1`, s.ScenarioID).Scan(&completedDifficulty); err != nil {
			return notFound(err)
		}
		updatePlayerProgress(&player, result, &finishedAt, completedDifficulty)
		command, err := tx.Exec(ctx, `UPDATE player_profiles SET
			completed_sessions=$2,successful_sessions=$3,current_win_streak=$4,
			best_win_streak=$5,unlocked_difficulty=$6,updated_at=$7 WHERE id=$1`,
			s.PlayerID, player.CompletedSessions, player.SuccessfulSessions, player.CurrentWinStreak,
			player.BestWinStreak, player.UnlockedDifficulty, finishedAt)
		if err != nil {
			return err
		}
		if command.RowsAffected() != 1 {
			return ErrNotFound
		}
		for _, achievement := range result.Achievements {
			if _, err := tx.Exec(ctx, `INSERT INTO player_achievements (player_id,code,title,description,unlocked_at)
				VALUES ($1,$2,$3,$4,$5) ON CONFLICT (player_id,code) DO NOTHING`,
				s.PlayerID, achievement.Code, achievement.Title, achievement.Description, finishedAt); err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) Result(ctx context.Context, id string) (domain.Result, error) {
	var result domain.Result
	var strengths, mistakes, recommendations, achievements, analysis []byte
	err := r.db.QueryRow(ctx, `SELECT session_id,final_score,outcome_code,outcome,strengths,mistakes,recommendations,achievements,analysis FROM results WHERE session_id=$1`, id).Scan(&result.SessionID, &result.FinalScore, &result.OutcomeCode, &result.Outcome, &strengths, &mistakes, &recommendations, &achievements, &analysis)
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
	if err := json.Unmarshal(achievements, &result.Achievements); err != nil {
		return result, err
	}
	if err := json.Unmarshal(analysis, &result.Analysis); err != nil {
		return result, err
	}
	return result, nil
}

func (r *PostgresRepository) ListSessions(ctx context.Context, filter SessionFilter) (domain.SessionPage, error) {
	filter = filter.WithDefaults()
	page := domain.SessionPage{Items: []domain.SessionSummary{}, Limit: filter.Limit, Offset: filter.Offset}
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM negotiation_sessions AS session
		WHERE ($1='' OR session.status=$1) AND ($2='' OR session.scenario_id=$2)`, filter.Status, filter.ScenarioID).Scan(&page.Total); err != nil {
		return domain.SessionPage{}, err
	}
	rows, err := r.db.Query(ctx, `SELECT session.id,session.scenario_id,scenario.title,COALESCE(session.player_id,''),COALESCE(session.parent_session_id,''),session.forked_from_turn,session.status,session.turn,
		COALESCE(result.final_score,-1),COALESCE(result.outcome_code,''),session.started_at,session.finished_at
		FROM negotiation_sessions AS session
		JOIN scenarios AS scenario ON scenario.id=session.scenario_id
		LEFT JOIN results AS result ON result.session_id=session.id
		WHERE ($1='' OR session.status=$1) AND ($2='' OR session.scenario_id=$2)
		ORDER BY session.started_at DESC,session.id DESC LIMIT $3 OFFSET $4`, filter.Status, filter.ScenarioID, filter.Limit, filter.Offset)
	if err != nil {
		return domain.SessionPage{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var summary domain.SessionSummary
		var finalScore int
		if err := rows.Scan(&summary.ID, &summary.ScenarioID, &summary.ScenarioTitle, &summary.PlayerID, &summary.ParentSessionID, &summary.ForkedFromTurn, &summary.Status, &summary.Turn, &finalScore, &summary.OutcomeCode, &summary.StartedAt, &summary.FinishedAt); err != nil {
			return domain.SessionPage{}, err
		}
		if finalScore >= 0 {
			summary.FinalScore = &finalScore
		}
		page.Items = append(page.Items, summary)
	}
	return page, rows.Err()
}

func (r *PostgresRepository) SessionStatistics(ctx context.Context, scenarioID string) (domain.SessionStatistics, error) {
	statistics := domain.SessionStatistics{Outcomes: []domain.OutcomeCount{}}
	err := r.db.QueryRow(ctx, `SELECT count(*),
		count(*) FILTER (WHERE session.status='active'),
		count(*) FILTER (WHERE session.status='finished'),
		count(*) FILTER (WHERE session.status='abandoned'),
		COALESCE(avg(result.final_score)::float8,0)
		FROM negotiation_sessions AS session
		LEFT JOIN results AS result ON result.session_id=session.id
		WHERE ($1='' OR session.scenario_id=$1)`, scenarioID).Scan(
		&statistics.Total, &statistics.Active, &statistics.Finished, &statistics.Abandoned, &statistics.AverageFinalScore,
	)
	if err != nil {
		return domain.SessionStatistics{}, err
	}
	rows, err := r.db.Query(ctx, `SELECT result.outcome_code,count(*)
		FROM results AS result
		JOIN negotiation_sessions AS session ON session.id=result.session_id
		WHERE ($1='' OR session.scenario_id=$1)
		GROUP BY result.outcome_code ORDER BY result.outcome_code`, scenarioID)
	if err != nil {
		return domain.SessionStatistics{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var outcome domain.OutcomeCount
		if err := rows.Scan(&outcome.OutcomeCode, &outcome.Count); err != nil {
			return domain.SessionStatistics{}, err
		}
		statistics.Outcomes = append(statistics.Outcomes, outcome)
	}
	return statistics, rows.Err()
}

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
