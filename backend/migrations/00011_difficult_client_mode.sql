-- +goose Up
UPDATE scenarios
SET rules = jsonb_set(rules, '{behavior}', '{
  "mode": "difficult",
  "emotionality": 2,
  "volatility": 2,
  "initialPriority": "соблюдение исходного срока",
  "priorityShifts": [
    {"turn": 3, "priority": "снижение рисков запуска"},
    {"turn": 6, "priority": "поэтапная поставка результата"}
  ]
}'::jsonb)
WHERE id = 'project-deadline';

-- +goose Down
UPDATE scenarios
SET rules = rules - 'behavior'
WHERE id = 'project-deadline';
