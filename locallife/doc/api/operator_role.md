# Operator Role API (Regional Management)

Focus: Steering the micro-economy of a specific region (District/City).

## 1. Regional Steering & Configuration

### Update Region Recommendation Config
- `PATCH /v1/regions/:id/recommendation-config`
- **Purpose**: Fine-tune the logic that recommends orders to riders.
- **Weights**: `distance_weight`, `route_weight`, `urgency_weight`, `profit_weight`.

### Peak Hour Management
- `POST /v1/operator/regions/:region_id/peak-hours`
- **Purpose**: Define high-demand periods to apply surcharges or prioritizations.

---

## 2. Business Audit & Supervision

### Merchant & Rider Audit
- `GET /v1/admin/merchants/applications`: List applications in assigned regions.
- `POST /v1/admin/merchants/applications/review`: Approve/Reject with rationale.
- `POST /v1/operator/merchants/:id/suspend`: Circuit-break a merchant due to quality issues.

### Dispute Resolution (Appeals)
- `GET /v1/operator/appeals`: List appeals from riders/merchants against claims or bans.
- `POST /v1/operator/appeals/:id/review`: Final arbitration.

---

## 3. Financial Oversight

### Revenue & Commission
- `GET /operators/me/commission`: View earned commission from regional transactions.
- `GET /operators/me/finance/overview`: Regional financial health dashboard.

### Regional Analytics
- `GET /v1/operator/regions/:region_id/stats`: Real-time volume, active riders, and fulfillment rates.
- `GET /v1/operator/merchants/ranking`: Identifying top/bottom performers.
