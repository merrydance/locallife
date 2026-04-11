ALTER TABLE behavior_actions
    DROP CONSTRAINT IF EXISTS behavior_actions_action_type_check;

ALTER TABLE behavior_actions
    ADD CONSTRAINT behavior_actions_action_type_check
    CHECK (action_type IN ('block', 'payout', 'notify', 'observe'));

COMMENT ON COLUMN user_balance_logs.type IS '变动类型：claim_payout/claim_payout_reversal/appeal_compensation/order_pay/withdraw/recharge/adjustment';