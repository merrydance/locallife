# Rider Role API (Logistics Marketplace)

Focus: Zero-commission logistics with trust-based scheduling.

## 1. Logistics Marketplace

### Get Recommended Orders
- `GET /v1/delivery/recommend`
- **Query**: `longitude`, `latitude` (float64)
- **Response**: Sorted list by `total_score` (distance, route, urgency, profit). Includes `pickup_latitude` and `delivery_latitude`.

### Grab Order (Commitment)
- `POST /v1/delivery/grab/:order_id`
- **Logic Truth**:
  - Requires `IsOnline = true`.
  - **Freezes 5000 cents (50 CNY) deposit** per grab.
  - High-value orders (fee >= 10 CNY) require `premium_score >= 0`.

---

## 2. Delivery Lifecycle

### Progression
1. `POST /v1/delivery/:id/start-pickup`: Notifies user rider is en route.
2. `POST /v1/delivery/:id/confirm-pickup`: Notifies user food is picked up.
3. `POST /v1/delivery/:id/delivered`: Completes the cycle.

---

## 3. Financials & Trust

### Bind Settlement Card
- `POST /v1/rider/applyment/bindbank`: Used for withdrawing earnings.

### Rider Profile
- `GET /v1/users/me`: Check `trust_score` and `premium_score`.
- `GET /v1/rider/stats`: Earnings dashboard.
