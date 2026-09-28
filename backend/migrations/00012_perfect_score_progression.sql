-- +goose Up
WITH desired_difficulty AS (
    SELECT
        player.id,
        CASE
            WHEN player.unlocked_difficulty = 'hard'
                OR player.successful_sessions >= 5
                OR EXISTS (
                    SELECT 1
                    FROM negotiation_sessions AS session
                    JOIN results AS result ON result.session_id = session.id
                    JOIN scenarios AS scenario ON scenario.id = session.scenario_id
                    WHERE session.player_id = player.id
                      AND result.final_score >= 100
                      AND result.outcome_code IN ('mutual_gain', 'advantageous_agreement', 'compromise')
                      AND scenario.difficulty = 'medium'
                )
                THEN 'hard'
            WHEN player.unlocked_difficulty = 'medium'
                OR player.successful_sessions >= 2
                OR EXISTS (
                    SELECT 1
                    FROM negotiation_sessions AS session
                    JOIN results AS result ON result.session_id = session.id
                    JOIN scenarios AS scenario ON scenario.id = session.scenario_id
                    WHERE session.player_id = player.id
                      AND result.final_score >= 100
                      AND result.outcome_code IN ('mutual_gain', 'advantageous_agreement', 'compromise')
                      AND scenario.difficulty = 'easy'
                )
                THEN 'medium'
            ELSE 'easy'
        END AS difficulty
    FROM player_profiles AS player
)
UPDATE player_profiles AS player
SET unlocked_difficulty = desired.difficulty,
    updated_at = now()
FROM desired_difficulty AS desired
WHERE desired.id = player.id
  AND player.unlocked_difficulty IS DISTINCT FROM desired.difficulty;

-- +goose Down
UPDATE player_profiles
SET unlocked_difficulty = CASE
        WHEN successful_sessions >= 5 THEN 'hard'
        WHEN successful_sessions >= 2 THEN 'medium'
        ELSE 'easy'
    END,
    updated_at = now();
