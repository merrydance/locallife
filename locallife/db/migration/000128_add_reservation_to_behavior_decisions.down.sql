ALTER TABLE behavior_decisions
    DROP CONSTRAINT IF EXISTS check_behavior_decisions_source;

DROP INDEX IF EXISTS idx_behavior_decisions_reservation_id;

ALTER TABLE behavior_decisions
    DROP COLUMN IF EXISTS reservation_id;

-- Only safe if no rows have NULL order_id
ALTER TABLE behavior_decisions
    ALTER COLUMN order_id SET NOT NULL;
