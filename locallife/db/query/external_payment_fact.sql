-- name: CreateExternalPaymentCommand :one
INSERT INTO external_payment_commands (
    provider,
    channel,
    capability,
    command_type,
    business_owner,
    business_object_type,
    business_object_id,
    external_object_type,
    external_object_key,
    external_secondary_key,
    command_status,
    submitted_at,
    accepted_at,
    rejected_at,
    last_error_code,
    last_error_message,
    request_fingerprint,
    response_snapshot
) VALUES (
    sqlc.arg(provider),
    sqlc.arg(channel),
    sqlc.arg(capability),
    sqlc.arg(command_type),
    sqlc.arg(business_owner),
    sqlc.narg(business_object_type),
    sqlc.narg(business_object_id),
    sqlc.arg(external_object_type),
    sqlc.arg(external_object_key),
    sqlc.narg(external_secondary_key),
    sqlc.arg(command_status),
    sqlc.arg(submitted_at),
    sqlc.narg(accepted_at),
    sqlc.narg(rejected_at),
    sqlc.narg(last_error_code),
    sqlc.narg(last_error_message),
    sqlc.narg(request_fingerprint),
    sqlc.narg(response_snapshot)
)
ON CONFLICT (provider, channel, capability, command_type, external_object_type, external_object_key)
DO UPDATE SET
    command_status = CASE
        WHEN external_payment_commands.command_status IN ('accepted', 'rejected') THEN external_payment_commands.command_status
        WHEN excluded.command_status = 'submitted' THEN external_payment_commands.command_status
        ELSE excluded.command_status
    END,
    accepted_at = CASE
        WHEN external_payment_commands.command_status IN ('accepted', 'rejected') THEN external_payment_commands.accepted_at
        WHEN external_payment_commands.accepted_at IS NOT NULL THEN external_payment_commands.accepted_at
        WHEN excluded.command_status = 'accepted' THEN excluded.accepted_at
        ELSE external_payment_commands.accepted_at
    END,
    rejected_at = CASE
        WHEN external_payment_commands.command_status IN ('accepted', 'rejected') THEN external_payment_commands.rejected_at
        WHEN external_payment_commands.rejected_at IS NOT NULL THEN external_payment_commands.rejected_at
        WHEN excluded.command_status = 'rejected' THEN excluded.rejected_at
        ELSE external_payment_commands.rejected_at
    END,
    last_error_code = CASE
        WHEN external_payment_commands.command_status IN ('accepted', 'rejected') THEN external_payment_commands.last_error_code
        WHEN excluded.command_status IN ('rejected', 'unknown') THEN excluded.last_error_code
        WHEN excluded.command_status = 'accepted' THEN NULL
        ELSE external_payment_commands.last_error_code
    END,
    last_error_message = CASE
        WHEN external_payment_commands.command_status IN ('accepted', 'rejected') THEN external_payment_commands.last_error_message
        WHEN excluded.command_status IN ('rejected', 'unknown') THEN excluded.last_error_message
        WHEN excluded.command_status = 'accepted' THEN NULL
        ELSE external_payment_commands.last_error_message
    END,
    response_snapshot = CASE
        WHEN external_payment_commands.command_status IN ('accepted', 'rejected') THEN external_payment_commands.response_snapshot
        WHEN excluded.command_status = 'submitted' THEN external_payment_commands.response_snapshot
        ELSE COALESCE(excluded.response_snapshot, external_payment_commands.response_snapshot)
    END,
    updated_at = now()
RETURNING id, provider, channel, capability, command_type, business_owner, business_object_type, business_object_id, external_object_type, external_object_key, external_secondary_key, command_status, submitted_at, accepted_at, rejected_at, last_error_code, last_error_message, request_fingerprint, response_snapshot, created_at, updated_at;

-- name: UpdateExternalPaymentCommandOutcome :one
UPDATE external_payment_commands
SET command_status = CASE
        WHEN command_status IN ('accepted', 'rejected') THEN command_status
        ELSE sqlc.arg(command_status)
    END,
    accepted_at = CASE
        WHEN command_status IN ('accepted', 'rejected') THEN accepted_at
        ELSE sqlc.narg(accepted_at)
    END,
    rejected_at = CASE
        WHEN command_status IN ('accepted', 'rejected') THEN rejected_at
        ELSE sqlc.narg(rejected_at)
    END,
    last_error_code = CASE
        WHEN command_status IN ('accepted', 'rejected') THEN last_error_code
        ELSE sqlc.narg(last_error_code)
    END,
    last_error_message = CASE
        WHEN command_status IN ('accepted', 'rejected') THEN last_error_message
        ELSE sqlc.narg(last_error_message)
    END,
    response_snapshot = CASE
        WHEN command_status IN ('accepted', 'rejected') THEN response_snapshot
        ELSE sqlc.narg(response_snapshot)
    END,
    updated_at = CASE
        WHEN command_status IN ('accepted', 'rejected') THEN updated_at
        ELSE now()
    END
WHERE id = sqlc.arg(id)
    AND command_status IN ('submitted', 'unknown', sqlc.arg(command_status))
RETURNING id, provider, channel, capability, command_type, business_owner, business_object_type, business_object_id, external_object_type, external_object_key, external_secondary_key, command_status, submitted_at, accepted_at, rejected_at, last_error_code, last_error_message, request_fingerprint, response_snapshot, created_at, updated_at;

-- name: ClaimSubmittedExternalPaymentCommandForDispatch :one
UPDATE external_payment_commands
SET command_status = sqlc.arg(command_status),
    last_error_code = sqlc.narg(last_error_code),
    last_error_message = sqlc.narg(last_error_message),
    response_snapshot = sqlc.narg(response_snapshot),
    updated_at = now()
WHERE id = sqlc.arg(id)
    AND command_status = 'submitted'
RETURNING id, provider, channel, capability, command_type, business_owner, business_object_type, business_object_id, external_object_type, external_object_key, external_secondary_key, command_status, submitted_at, accepted_at, rejected_at, last_error_code, last_error_message, request_fingerprint, response_snapshot, created_at, updated_at;

-- name: GetExternalPaymentCommand :one
SELECT id, provider, channel, capability, command_type, business_owner, business_object_type, business_object_id, external_object_type, external_object_key, external_secondary_key, command_status, submitted_at, accepted_at, rejected_at, last_error_code, last_error_message, request_fingerprint, response_snapshot, created_at, updated_at
FROM external_payment_commands
WHERE id = $1
LIMIT 1;

-- name: GetExternalPaymentCommandByExternalObject :one
SELECT id, provider, channel, capability, command_type, business_owner, business_object_type, business_object_id, external_object_type, external_object_key, external_secondary_key, command_status, submitted_at, accepted_at, rejected_at, last_error_code, last_error_message, request_fingerprint, response_snapshot, created_at, updated_at
FROM external_payment_commands
WHERE provider = $1
    AND channel = $2
    AND capability = $3
    AND command_type = $4
    AND external_object_type = $5
    AND external_object_key = $6
LIMIT 1;

-- name: ListSubmittedBaofuWithdrawalCommandsForDispatch :many
SELECT id, provider, channel, capability, command_type, business_owner, business_object_type, business_object_id, external_object_type, external_object_key, external_secondary_key, command_status, submitted_at, accepted_at, rejected_at, last_error_code, last_error_message, request_fingerprint, response_snapshot, created_at, updated_at
FROM external_payment_commands
WHERE provider = 'baofu'
    AND channel = 'baofu_aggregate'
    AND capability = 'baofu_withdraw'
    AND command_type = 'create_baofu_withdraw'
    AND external_object_type = 'withdraw'
    AND command_status = 'submitted'
    AND submitted_at <= sqlc.arg(submitted_before)
    AND COALESCE(response_snapshot, '{}'::jsonb) @> '{"dispatch_mode":"async_worker"}'::jsonb
ORDER BY submitted_at ASC, id ASC
LIMIT sqlc.arg(limit_count);

-- name: CreateExternalPaymentFact :one
INSERT INTO external_payment_facts (
    provider,
    channel,
    capability,
    fact_source,
    source_event_id,
    source_event_type,
    external_object_type,
    external_object_key,
    external_secondary_key,
    business_owner,
    business_object_type,
    business_object_id,
    upstream_state,
    terminal_status,
    is_terminal,
    amount,
    currency,
    occurred_at,
    upstream_updated_at,
    observed_at,
    raw_resource,
    dedupe_key,
    processing_status
) VALUES (
    sqlc.arg(provider),
    sqlc.arg(channel),
    sqlc.arg(capability),
    sqlc.arg(fact_source),
    sqlc.narg(source_event_id),
    sqlc.narg(source_event_type),
    sqlc.arg(external_object_type),
    sqlc.arg(external_object_key),
    sqlc.narg(external_secondary_key),
    sqlc.narg(business_owner),
    sqlc.narg(business_object_type),
    sqlc.narg(business_object_id),
    sqlc.arg(upstream_state),
    sqlc.arg(terminal_status),
    sqlc.arg(is_terminal),
    sqlc.narg(amount),
    sqlc.arg(currency),
    sqlc.narg(occurred_at),
    sqlc.narg(upstream_updated_at),
    sqlc.arg(observed_at),
    sqlc.arg(raw_resource),
    sqlc.arg(dedupe_key),
    sqlc.arg(processing_status)
)
ON CONFLICT (dedupe_key)
DO UPDATE SET dedupe_key = external_payment_facts.dedupe_key
WHERE external_payment_facts.provider = excluded.provider
    AND external_payment_facts.channel = excluded.channel
    AND external_payment_facts.capability = excluded.capability
    AND external_payment_facts.fact_source = excluded.fact_source
    AND external_payment_facts.source_event_id IS NOT DISTINCT FROM excluded.source_event_id
    AND external_payment_facts.source_event_type IS NOT DISTINCT FROM excluded.source_event_type
    AND external_payment_facts.external_object_type = excluded.external_object_type
    AND external_payment_facts.external_object_key = excluded.external_object_key
    AND external_payment_facts.external_secondary_key IS NOT DISTINCT FROM excluded.external_secondary_key
    AND external_payment_facts.business_owner IS NOT DISTINCT FROM excluded.business_owner
    AND external_payment_facts.business_object_type IS NOT DISTINCT FROM excluded.business_object_type
    AND external_payment_facts.business_object_id IS NOT DISTINCT FROM excluded.business_object_id
    AND external_payment_facts.upstream_state = excluded.upstream_state
    AND external_payment_facts.terminal_status = excluded.terminal_status
    AND external_payment_facts.is_terminal = excluded.is_terminal
    AND external_payment_facts.amount IS NOT DISTINCT FROM excluded.amount
    AND external_payment_facts.currency = excluded.currency
RETURNING id, provider, channel, capability, fact_source, source_event_id, source_event_type, external_object_type, external_object_key, external_secondary_key, business_owner, business_object_type, business_object_id, upstream_state, terminal_status, is_terminal, amount, currency, occurred_at, upstream_updated_at, observed_at, raw_resource, dedupe_key, processing_status, processing_error, processed_at, created_at, updated_at;

-- name: GetExternalPaymentFact :one
SELECT id, provider, channel, capability, fact_source, source_event_id, source_event_type, external_object_type, external_object_key, external_secondary_key, business_owner, business_object_type, business_object_id, upstream_state, terminal_status, is_terminal, amount, currency, occurred_at, upstream_updated_at, observed_at, raw_resource, dedupe_key, processing_status, processing_error, processed_at, created_at, updated_at
FROM external_payment_facts
WHERE id = $1
LIMIT 1;

-- name: GetExternalPaymentFactByDedupeKey :one
SELECT id, provider, channel, capability, fact_source, source_event_id, source_event_type, external_object_type, external_object_key, external_secondary_key, business_owner, business_object_type, business_object_id, upstream_state, terminal_status, is_terminal, amount, currency, occurred_at, upstream_updated_at, observed_at, raw_resource, dedupe_key, processing_status, processing_error, processed_at, created_at, updated_at
FROM external_payment_facts
WHERE dedupe_key = $1
LIMIT 1;

-- name: ListExternalPaymentFactsByExternalObject :many
SELECT id, provider, channel, capability, fact_source, source_event_id, source_event_type, external_object_type, external_object_key, external_secondary_key, business_owner, business_object_type, business_object_id, upstream_state, terminal_status, is_terminal, amount, currency, occurred_at, upstream_updated_at, observed_at, raw_resource, dedupe_key, processing_status, processing_error, processed_at, created_at, updated_at
FROM external_payment_facts
WHERE provider = $1
    AND channel = $2
    AND external_object_type = $3
    AND external_object_key = $4
ORDER BY created_at DESC, id DESC;

-- name: UpdateExternalPaymentFactProcessingStatus :one
UPDATE external_payment_facts
SET processing_status = sqlc.arg(processing_status),
    processing_error = sqlc.narg(processing_error),
    processed_at = sqlc.narg(processed_at),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, provider, channel, capability, fact_source, source_event_id, source_event_type, external_object_type, external_object_key, external_secondary_key, business_owner, business_object_type, business_object_id, upstream_state, terminal_status, is_terminal, amount, currency, occurred_at, upstream_updated_at, observed_at, raw_resource, dedupe_key, processing_status, processing_error, processed_at, created_at, updated_at;

-- name: CreateExternalPaymentFactApplication :one
INSERT INTO external_payment_fact_applications (
    fact_id,
    consumer,
    business_object_type,
    business_object_id,
    status,
    next_retry_at
) VALUES (
    sqlc.arg(fact_id),
    sqlc.arg(consumer),
    sqlc.arg(business_object_type),
    sqlc.arg(business_object_id),
    sqlc.arg(status),
    sqlc.narg(next_retry_at)
)
ON CONFLICT (fact_id, consumer, business_object_type, business_object_id)
DO UPDATE SET updated_at = external_payment_fact_applications.updated_at
RETURNING id, fact_id, consumer, business_object_type, business_object_id, status, attempt_count, last_error, next_retry_at, applied_at, created_at, updated_at;

-- name: GetExternalPaymentFactApplication :one
SELECT id, fact_id, consumer, business_object_type, business_object_id, status, attempt_count, last_error, next_retry_at, applied_at, created_at, updated_at
FROM external_payment_fact_applications
WHERE id = $1
LIMIT 1;

-- name: ListExternalPaymentFactApplicationsByFact :many
SELECT id, fact_id, consumer, business_object_type, business_object_id, status, attempt_count, last_error, next_retry_at, applied_at, created_at, updated_at
FROM external_payment_fact_applications
WHERE fact_id = $1
ORDER BY created_at ASC, id ASC;

-- name: ClaimExternalPaymentFactApplication :one
UPDATE external_payment_fact_applications
SET status = 'processing',
    attempt_count = attempt_count + 1,
    updated_at = now()
WHERE id = sqlc.arg(id)
    AND status IN ('pending', 'failed')
RETURNING id, fact_id, consumer, business_object_type, business_object_id, status, attempt_count, last_error, next_retry_at, applied_at, created_at, updated_at;

-- name: MarkExternalPaymentFactApplicationApplied :one
UPDATE external_payment_fact_applications
SET status = 'applied',
    last_error = NULL,
    next_retry_at = NULL,
    applied_at = sqlc.arg(applied_at),
    updated_at = now()
WHERE id = sqlc.arg(id)
    AND status = 'processing'
RETURNING id, fact_id, consumer, business_object_type, business_object_id, status, attempt_count, last_error, next_retry_at, applied_at, created_at, updated_at;

-- name: MarkExternalPaymentFactApplicationFailed :one
UPDATE external_payment_fact_applications
SET status = 'failed',
    last_error = sqlc.arg(last_error),
    next_retry_at = sqlc.arg(next_retry_at),
    updated_at = now()
WHERE id = sqlc.arg(id)
    AND status = 'processing'
RETURNING id, fact_id, consumer, business_object_type, business_object_id, status, attempt_count, last_error, next_retry_at, applied_at, created_at, updated_at;

-- name: ListRetryableExternalPaymentFactApplications :many
SELECT id, fact_id, consumer, business_object_type, business_object_id, status, attempt_count, last_error, next_retry_at, applied_at, created_at, updated_at
FROM external_payment_fact_applications
WHERE status IN ('pending', 'failed')
    AND (next_retry_at IS NULL OR next_retry_at <= sqlc.arg(now_at))
ORDER BY COALESCE(next_retry_at, created_at) ASC, id ASC
LIMIT sqlc.arg(limit_count);

-- name: ListRetryableExternalPaymentFactApplicationsByTarget :many
SELECT id, fact_id, consumer, business_object_type, business_object_id, status, attempt_count, last_error, next_retry_at, applied_at, created_at, updated_at
FROM external_payment_fact_applications
WHERE consumer = sqlc.arg(consumer)
    AND business_object_type = sqlc.arg(business_object_type)
    AND status IN ('pending', 'failed')
    AND (next_retry_at IS NULL OR next_retry_at <= sqlc.arg(now_at))
ORDER BY COALESCE(next_retry_at, created_at) ASC, id ASC
LIMIT sqlc.arg(limit_count);

-- name: CreatePaymentDomainOutbox :one
INSERT INTO payment_domain_outbox (
    event_type,
    aggregate_type,
    aggregate_id,
    payload,
    status,
    next_retry_at
) VALUES (
    sqlc.arg(event_type),
    sqlc.arg(aggregate_type),
    sqlc.arg(aggregate_id),
    sqlc.arg(payload),
    sqlc.arg(status),
    sqlc.narg(next_retry_at)
)
RETURNING id, event_type, aggregate_type, aggregate_id, payload, status, attempt_count, next_retry_at, last_error, created_at, updated_at;

-- name: CreatePaymentDomainOutboxOnce :one
INSERT INTO payment_domain_outbox (
    event_type,
    aggregate_type,
    aggregate_id,
    payload,
    status,
    next_retry_at
) VALUES (
    sqlc.arg(event_type),
    sqlc.arg(aggregate_type),
    sqlc.arg(aggregate_id),
    sqlc.arg(payload),
    sqlc.arg(status),
    sqlc.narg(next_retry_at)
)
ON CONFLICT (event_type, aggregate_type, aggregate_id)
DO UPDATE SET updated_at = payment_domain_outbox.updated_at
WHERE payment_domain_outbox.payload = excluded.payload
   OR (
      payment_domain_outbox.payload - 'external_payment_fact_id' - 'payment_fact_application_id'
      =
      excluded.payload - 'external_payment_fact_id' - 'payment_fact_application_id'
   )
RETURNING id, event_type, aggregate_type, aggregate_id, payload, status, attempt_count, next_retry_at, last_error, created_at, updated_at;

-- name: ListPendingPaymentDomainOutbox :many
SELECT id, event_type, aggregate_type, aggregate_id, payload, status, attempt_count, next_retry_at, last_error, created_at, updated_at
FROM payment_domain_outbox
WHERE status IN ('pending', 'failed')
    AND (next_retry_at IS NULL OR next_retry_at <= sqlc.arg(now_at))
ORDER BY COALESCE(next_retry_at, created_at) ASC, id ASC
LIMIT sqlc.arg(limit_count);

-- name: ListPendingPaymentDomainOutboxByEventType :many
SELECT id, event_type, aggregate_type, aggregate_id, payload, status, attempt_count, next_retry_at, last_error, created_at, updated_at
FROM payment_domain_outbox
WHERE event_type = sqlc.arg(event_type)
    AND status IN ('pending', 'failed')
    AND (next_retry_at IS NULL OR next_retry_at <= sqlc.arg(now_at))
ORDER BY COALESCE(next_retry_at, created_at) ASC, id ASC
LIMIT sqlc.arg(limit_count);

-- name: ClaimPaymentDomainOutbox :one
UPDATE payment_domain_outbox
SET status = 'processing',
    attempt_count = attempt_count + 1,
    updated_at = now()
WHERE id = sqlc.arg(id)
    AND status IN ('pending', 'failed')
    AND (next_retry_at IS NULL OR next_retry_at <= sqlc.arg(now_at))
RETURNING id, event_type, aggregate_type, aggregate_id, payload, status, attempt_count, next_retry_at, last_error, created_at, updated_at;

-- name: MarkPaymentDomainOutboxPublished :one
UPDATE payment_domain_outbox
SET status = 'published',
    next_retry_at = NULL,
    last_error = NULL,
    updated_at = now()
WHERE id = sqlc.arg(id)
    AND status = 'processing'
RETURNING id, event_type, aggregate_type, aggregate_id, payload, status, attempt_count, next_retry_at, last_error, created_at, updated_at;

-- name: MarkPaymentDomainOutboxFailed :one
UPDATE payment_domain_outbox
SET status = 'failed',
    last_error = sqlc.arg(last_error),
    next_retry_at = sqlc.arg(next_retry_at),
    updated_at = now()
WHERE id = sqlc.arg(id)
    AND status = 'processing'
RETURNING id, event_type, aggregate_type, aggregate_id, payload, status, attempt_count, next_retry_at, last_error, created_at, updated_at;
