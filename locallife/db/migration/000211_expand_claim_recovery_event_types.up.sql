ALTER TABLE claim_recovery_events
    DROP CONSTRAINT IF EXISTS claim_recovery_events_event_type_check;

ALTER TABLE claim_recovery_events
    ADD CONSTRAINT claim_recovery_events_event_type_check
    CHECK (event_type IN (
        'created',
        'payable',
        'payment_started',
        'paid',
        'overdue',
        'disputed',
        'waived',
        'resumed',
        'closed',
        'overturned'
    ));