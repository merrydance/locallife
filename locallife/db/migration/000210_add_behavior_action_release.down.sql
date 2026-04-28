ALTER TABLE behavior_actions
    DROP CONSTRAINT IF EXISTS behavior_actions_action_type_check;

DELETE FROM behavior_actions
WHERE action_type = 'release';

ALTER TABLE behavior_actions
    ADD CONSTRAINT behavior_actions_action_type_check
    CHECK (action_type IN ('block', 'payout', 'notify', 'observe', 'recovery'));