# Backend Error Handling Standards

This document defines the error handling and logging contract for the three-layer backend architecture
(`api/` → `logic/` → `db/sqlc/`). Follow these rules to ensure every unexpected failure leaves exactly
one structured log entry and no internal detail leaks to clients.

---

## Architecture Overview

```
api/ (transport)          logic/ (business rules)       db/sqlc/ (persistence)
─────────────────         ──────────────────────────    ──────────────────────
internalError(ctx, err)   NewRequestError(4xx, err)     return fmt.Errorf(...)
errorResponse(err)        return fmt.Errorf(...)
writeLogicRequestError()
```

### Key infrastructure

| Component | File | Role |
|-----------|------|------|
| `internalError(ctx, err)` | `api/server.go` | Logs structured 5xx + returns safe body |
| `errorResponse(err)` | `api/server.go` | Shapes 4xx body, no logging |
| `writeLogicRequestError(ctx, err)` | `api/payment_order.go` | Extracts `*logic.RequestError` → `errorResponse`; returns false for plain errors |
| `RequestLoggingMiddleware` | `api/middleware_tracing.go` | Records every response including `ctx.Errors` |
| `ResponseEnvelopeMiddleware` | `api/response_envelope.go` | Sanitizes 5xx body to `"internal server error"` |
| `APIError` constants | `api/apierrors.go` | Stable numeric codes for client-parseable errors |

---

## Correct Patterns

### 1. Business rule violation (4xx) originating in logic/

```go
// logic/ — user-facing text, no logging
return result, logic.NewRequestError(http.StatusBadRequest, errors.New("table is disabled"))
return result, logic.NewRequestError(http.StatusNotFound, errors.New("order not found"))
return result, logic.NewRequestError(http.StatusConflict, errors.New("该时间段已被预订"))
```

```go
// api/ handler — routes through writeLogicRequestError, no logging needed
if err != nil {
    if writeLogicRequestError(ctx, err) {
        return
    }
    ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
    return
}
```

### 2. Unexpected infrastructure failure originating in logic/

```go
// logic/ — wrap with context, do NOT log here, do NOT use NewRequestError
return result, fmt.Errorf("get merchant payment config: %w", err)
return result, fmt.Errorf("create payment order: %w", err)
```

These propagate as plain errors. `writeLogicRequestError` returns false, and the handler
falls through to `internalError(ctx, err)`, which logs once with request_id, file, func,
and any PG error fields.

### 3. Unexpected failure originating directly in api/

```go
// api/ handler — use internalError, not errorResponse
result, err := server.store.GetMerchant(ctx, id)
if err != nil {
    ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
    return
}
```

`internalError` calls `ctx.Error(err)` (recorded in gin context), logs via zerolog, then
returns the safe response body.

### 3a. Upstream/provider failure with non-500 status (502/503)

When the response status must remain `502 Bad Gateway` or `503 Service Unavailable`, do not
return `errorResponse(err)`. Log the real error and return a stable public message.

```go
if err := server.printerClient.QueryOrderState(ctx, vendorOrderID); err != nil {
    ctx.JSON(http.StatusBadGateway,
        loggedServerError(ctx, err, "cloud print status unavailable", "cloud print order state query failed"))
    return
}
```

Use this pattern for cloud printers, payment providers, feature metadata dependencies, or
other upstream integrations where the status should remain 502/503 but internal/provider
error text must not be exposed.

### 4. User input validation or business guard in api/ (no logic/ involvement)

```go
// api/ handler — 4xx, no logging needed
ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid time format, use HH:MM")))
```

### 5. Stable machine-readable error codes

Use `APIError` constants from `api/apierrors.go` when the client must distinguish error
sub-types programmatically.

```go
ctx.JSON(http.StatusNotFound, errorResponse(api.ErrRiderNotFound))
```

### 6. worker/ and scheduler/ error handling

Workers are outside the HTTP request/response cycle. Log at the point where the decision
to abort, retry, or skip is made:

```go
if err != nil {
    log.Error().Err(err).Int64("order_id", orderID).Msg("create payment order failed")
    return err  // Asynq retries based on returned error
}
```

Do **not** add an additional `log.Error` before returning — Asynq's processor logs the
final returned error at task level.

---

## Anti-Patterns to Avoid

### ❌ Anti-pattern A — `NewRequestError(5xx, ...)` in logic/

`writeLogicRequestError` intercepts any `*logic.RequestError` and calls `errorResponse()`,
not `internalError()`. When the status is 5xx, this silently skips structured logging.

```go
// WRONG — 500 bypasses internalError(), leaves no log trace
return result, logic.NewRequestError(http.StatusInternalServerError,
    errors.New("payment client not configured"))

// CORRECT — plain fmt.Errorf propagates to internalError() in the handler
return result, fmt.Errorf("payment client: not configured")
```

**Known instances** (as of initial audit; some paths have since been removed or renamed during payment-channel cleanup):

| File | Line(s) | Message |
|------|---------|---------|
| `logic/order_calculation.go` | 108, 194 | "customizations handler not configured", "delivery fee calculator required" |
| `logic/order_service.go` | 168, 693 | "delivery fee calculator required", "print scheduler not configured" |
| `logic/claim_recovery_payment.go` | 100 | "payment client not configured" (503) |

### ❌ Anti-pattern B — `ctx.JSON(5xx, errorResponse(...))` in api/

`errorResponse` is for 4xx bodies only. Using it for 5xx skips the `internalError` logging
path entirely.

```go
// WRONG — no structured log, no request_id in error record
ctx.JSON(http.StatusInternalServerError, errorResponse(err))

// CORRECT
ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
```

For `502`/`503`, the equivalent correct pattern is `loggedServerError(ctx, err, publicMessage, logMessage)`.

**Known instances** (as of initial audit):

| File | Lines |
|------|-------|
| `api/operator_features.go` | 96, 109, 161, 167, 184, 193, 233, 288, 305, 322, 326, 330 |
| `api/device_reconciliation.go` | 223, 232 |
| `api/group_admin.go` | 45 |
| `api/agreement.go` | 26, 56 |

### ❌ Anti-pattern C — Raw upstream error text in client responses

Concatenating external API error text into a client-visible error string leaks unstable
implementation details and may expose PHI or credentials. It also typically uses
`errorResponse` at 502/503 level, meaning no structured logging.

```go
// WRONG — leaks WeChat internal error, no structured log
ctx.JSON(http.StatusBadGateway, errorResponse(
    errors.New("WeChat subsidy API failed: "+wxErr.Error()),
))

// CORRECT — log the upstream detail, return a stable message
log.Error().Err(wxErr).Int64("payment_order_id", id).Msg("wechat subsidy api failed")
ctx.JSON(http.StatusBadGateway, errorResponse(errors.New("subsidy api unavailable")))
```

**Known instances** (as of initial audit):

| File | Lines |
|------|-------|
| `api/subsidy.go` | 194, 300, 370 |

### ❌ Anti-pattern D — Log-then-propagate (double logging)

Logging at the origin site in logic/ and then propagating the error to a handler that
calls `internalError` produces duplicate log entries for a single failure. Log only at
one point: preferably the handler via `internalError`, not in the middle of the call
stack.

```go
// WRONG — logs twice: once here, once when internalError() fires in the handler
log.Error().Err(err).Msg("get merchant payment config for reservation refund failed")
return ReservationStatusUpdateResult{}, fmt.Errorf("get merchant payment config: %w", err)

// CORRECT — wrap with context, let internalError() log once
return ReservationStatusUpdateResult{}, fmt.Errorf("get merchant payment config: %w", err)
```

**Exception**: logging IS appropriate in logic/ or a helper when it controls retry/recovery
decisions (e.g., inside `execTx` deadlock retry, or after a fallback). In those cases,
use `Warn` and do not re-propagate the same error instance.

**Known instances** (as of initial audit):

| File | Line | Context |
|------|------|---------|
| `logic/merchant_reject_refund.go` | 123 | get merchant payment config |
| `logic/reservation.go` | 550 | get merchant payment config (reservation refund) |
| `logic/delivery_broadcast.go` | 41, 82 | list nearby riders for broadcast |

---

## Decision Reference

```
Did the failure originate in logic/?
├─ YES — is it a business rule violation the user caused?
│   ├─ YES → NewRequestError(4xx, errors.New("user-facing message"))
│   └─ NO  → return fmt.Errorf("context: %w", err)   ← let handler log via internalError()
│
└─ NO — is it in api/ (handler, middleware)?
    ├─ Is it a 4xx (input validation, authz guard)?
    │   └─ ctx.JSON(status, errorResponse(errors.New("...")))
    └─ Is it an unexpected failure (db call, external API, nil pointer)?
        └─ ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
```

---

## Checklist for Code Review

- [ ] No `NewRequestError(http.StatusInternalServerError, ...)` or `NewRequestError(http.StatusServiceUnavailable, ...)` in `logic/`
- [ ] No `ctx.JSON(5xx, errorResponse(...))` in `api/` — must use `internalError(ctx, err)` for all 5xx
- [ ] For `502`/`503`, no `errorResponse(err)` with raw internal/provider errors — use `loggedServerError(...)` with a stable public message
- [ ] No upstream error text (vendor API, third-party service) concatenated into client-visible strings
- [ ] No `log.Error/Warn` in logic/ followed immediately by `return ..., err` for the same error (double logging)
- [ ] Workers use `log.Error` at the decision point and return the error to the Asynq framework for retry handling
