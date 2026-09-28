-- +goose Up
ALTER TABLE scenarios
    ADD COLUMN rules jsonb NOT NULL DEFAULT '{"maxTurns":8,"minimumTrustForAgreement":55,"requiresInterestExploration":true,"requiresEvidence":true,"proposal":{"kind":"none","maximumValue":0,"alternativeIds":[]},"trustScoreWeight":1,"argumentScoreWeight":1,"pressureScoreWeight":1}'::jsonb;

ALTER TABLE negotiation_sessions
    ADD COLUMN state jsonb NOT NULL DEFAULT '{"phase":"opening","interestsExplored":false,"evidencePresented":false,"offerMade":false,"offerAccepted":false}'::jsonb;

UPDATE scenarios SET rules = '{"maxTurns":8,"minimumTrustForAgreement":55,"requiresInterestExploration":true,"requiresEvidence":true,"proposal":{"kind":"raise_percent","maximumValue":10,"alternativeIds":["review_in_3_months"]},"trustScoreWeight":1,"argumentScoreWeight":1,"pressureScoreWeight":1}'::jsonb
WHERE id = 'salary-negotiation';

UPDATE scenarios SET rules = '{"maxTurns":10,"minimumTrustForAgreement":60,"requiresInterestExploration":true,"requiresEvidence":true,"proposal":{"kind":"extension_days","maximumValue":14,"alternativeIds":["phased_delivery"]},"trustScoreWeight":1,"argumentScoreWeight":1,"pressureScoreWeight":1}'::jsonb
WHERE id = 'project-deadline';

UPDATE negotiation_sessions
SET state = jsonb_set(state, '{phase}', to_jsonb(
    CASE WHEN status = 'finished' THEN 'finished'
         WHEN turn > 0 THEN 'exploration'
         ELSE 'opening' END
));

-- +goose Down
ALTER TABLE negotiation_sessions DROP COLUMN state;
ALTER TABLE scenarios DROP COLUMN rules;
