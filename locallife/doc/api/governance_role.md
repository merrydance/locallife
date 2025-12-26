# Governance Role API (Stability & Audit)

Focus: Maintaining ecosystem health through trust scores and conflict resolution.

## 1. Trust Scoring (Proof of Reliability)

### Get Trust Profile
- `GET /v1/trust-score/profiles/{role}/{id}`
- **Enums**: `customer`, `merchant`, `rider`.
- **Response**: Role-specific stats (e.g., `violation_count` for users, `food_safety_incidents` for merchants).

### Trust Change History
- `GET /v1/trust-score/history/{role}/{id}`
- **Fields**: `old_score`, `new_score`, `reason_type`, `is_auto`.

---

## 2. Claim Management (Eco-Security)

### Submit Claim (AI-First)
- `POST /v1/trust-score/claims`
- **Payload (`SubmitClaimRequest`)**:
  - `claim_type`: `foreign-object`, `damage`, `timeout`, `food-safety`.
  - `claim_amount`: (int64, in cents).
  - `evidence_photos`: URL list (max 10).
- **Processing Logic**: AI evaluates trust scores; high-score users get **instant** approval. Low-score users trigger **manual** review.

### Human Auditor
- `PATCH /v1/trust-score/claims/:id/review`: Approve/Reject with rationale.

---

## 3. Advanced Governance

### Fraud Detection
- `POST /v1/trust-score/fraud/detect`
- **Modes**: `claim_id` (coordinated), `device_fingerprint` (multiplier), `address_id` (clustering).

### Regional Steering
- `PATCH /v1/regions/:id/recommendation-config`: Update weights for `DistanceWeight`, `RouteWeight`, etc.
