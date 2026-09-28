-- +goose Up
ALTER TABLE results
    ADD COLUMN outcome_code text NOT NULL DEFAULT 'legacy';

UPDATE scenarios
SET rules = rules || jsonb_build_object(
    'minimumArgumentScoreForAgreement', COALESCE(rules->'minimumArgumentScoreForAgreement', '2'::jsonb),
    'maximumPressureForAgreement', COALESCE(rules->'maximumPressureForAgreement', '4'::jsonb),
    'proposal', COALESCE(rules->'proposal', '{}'::jsonb) || jsonb_build_object(
        'preferredValue', COALESCE(rules->'proposal'->'preferredValue', rules->'proposal'->'maximumValue', '0'::jsonb),
        'inputMaximumValue', COALESCE(rules->'proposal'->'inputMaximumValue', rules->'proposal'->'maximumValue', '0'::jsonb),
        'preferredAlternativeIds', COALESCE(rules->'proposal'->'preferredAlternativeIds', '[]'::jsonb)
    )
);

UPDATE scenarios
SET rules = jsonb_set(
    jsonb_set(
        jsonb_set(rules, '{proposal,preferredValue}', '6'::jsonb),
        '{proposal,inputMaximumValue}', '30'::jsonb
    ),
    '{proposal,preferredAlternativeIds}', '["review_in_3_months"]'::jsonb
)
WHERE id = 'salary-negotiation';

UPDATE scenarios
SET rules = jsonb_set(
    jsonb_set(
        jsonb_set(
            jsonb_set(rules, '{minimumArgumentScoreForAgreement}', '3'::jsonb),
            '{maximumPressureForAgreement}', '2'::jsonb
        ),
        '{proposal,preferredValue}', '7'::jsonb
    ),
    '{proposal,inputMaximumValue}', '60'::jsonb
)
WHERE id = 'project-deadline';

UPDATE scenarios
SET rules = jsonb_set(rules, '{proposal,preferredAlternativeIds}', '["phased_delivery"]'::jsonb)
WHERE id = 'project-deadline';

-- +goose Down
UPDATE scenarios
SET rules = (rules - 'minimumArgumentScoreForAgreement' - 'maximumPressureForAgreement') || jsonb_build_object(
    'proposal', (rules->'proposal') - 'preferredValue' - 'inputMaximumValue' - 'preferredAlternativeIds'
);

ALTER TABLE results DROP COLUMN outcome_code;
