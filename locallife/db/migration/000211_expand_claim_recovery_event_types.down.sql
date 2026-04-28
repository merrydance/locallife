ALTER TABLE claim_recovery_events
    DROP CONSTRAINT IF EXISTS claim_recovery_events_event_type_check;

DELETE FROM claim_recovery_events
WHERE event_type IN ('payable', 'payment_started', 'overdue', 'disputed', 'closed');

ALTER TABLE claim_recovery_events
    ADD CONSTRAINT claim_recovery_events_event_type_check
    CHECK (event_type IN ('created', 'paid', 'waived', 'resumed', 'overturned'));