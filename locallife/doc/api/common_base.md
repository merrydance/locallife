# Common API Components (LocalLife)

This document contains shared schemas and components used across all role-based API modules.

## 1. Response Envelope
The API supports an optional unified response envelope. To enable it, clients **MUST** send the header `X-Response-Envelope: 1`.

### Envelope Structure
```json
{
  "code": 0,
  "message": "ok",
  "data": {} 
}
```

### Technical Codes (Implementation Truth)
- `0`: Success (`CodeOK`)
- `40000`: Bad Request (`CodeBadRequest`)
- `40100`: Unauthorized (`CodeUnauthorized`)
- `40300`: Forbidden (`CodeForbidden`)
- `40400`: Not Found (`CodeNotFound`)
- `40900`: Conflict (`CodeConflict`)
- `42200`: Unprocessable Entity (`CodeUnprocessable`)
- `42900`: Too Many Requests (`CodeTooManyRequest`)
- `50000`: Internal Server Error (`CodeInternalError`)
- `50200`: Bad Gateway (`CodeBadGateway`)
- `50300`: Service Unavailable (`CodeServiceUnavail`)
- `50400`: Gateway Timeout (`CodeGatewayTimeout`)

---

## 2. Common Models

### ErrorResponse (Internal Data)
When `code != 0`, the `data` field of the envelope (or the raw root if envelope is disabled) contains:
```json
{
  "error": "Detailed error description"
}
```

### Pagination Params (Query)
- `page_id` (int32, min: 1): Current page.
- `page_size` (int32, min: 1, max: 50): Items per page.

### Coordinates
Used in search and location tracking:
- `longitude`: float64 (range: -180 to 180)
- `latitude`: float64 (range: -90 to 90)

---

## 4. Documentation Strategy
To ensure speed and accuracy, LocalLife documentation is hierarchical:
- **Role Guides (Markdown)**: *Where you are now.* These documents contain the business logic, state transitions, and core pathways for each user role.
- **Swagger UI (Technical Reference)**: For exhaustive details on every endpoint (Full JSON schemas, URL parameters, and Status Code maps), visit `/swagger/index.html` on your development server.

---

## 5. File Index
- [Customer Role](file:///\\wsl.localhost\Debian\home\sam\locallife\doc\api\customer_role.md): Discovery, Cart, Orders. ([Flows](file:///\\wsl.localhost\Debian\home\sam\locallife\doc\api\flows\customer_orders.md), [Swagger](file:///\\wsl.localhost\Debian\home\sam\locallife\doc\api\swagger\customer_v1.json))
- [Merchant Role](file:///\\wsl.localhost\Debian\home\sam\locallife\doc\api\merchant_role.md): SaaS, Onboarding, Store Mgmt. ([Flows](file:///\\wsl.localhost\Debian\home\sam\locallife\doc\api\flows\merchant_store_mgmt.md), [Swagger](file:///\\wsl.localhost\Debian\home\sam\locallife\doc\api\swagger\merchant_v1.json))
- [Rider Role](file:///\\wsl.localhost\Debian\home\sam\locallife\doc\api\rider_role.md): Logistics Marketplace. ([Flows](file:///\\wsl.localhost\Debian\home\sam\locallife\doc\api\flows\ranger_lifecycle.md), [Swagger](file:///\\wsl.localhost\Debian\home\sam\locallife\doc\api\swagger\rider_v1.json))
- [Marketing & CRM](file:///\\wsl.localhost\Debian\home\sam\locallife\doc\api\marketing_and_crm.md): Vouchers, Memberships. ([Swagger](file:///\\wsl.localhost\Debian\home\sam\locallife\doc\api\swagger\marketing_v1.json))
- [Recommendation Engine](file:///\\wsl.localhost\Debian\home\sam\locallife\doc\api\recommendation_engine.md): Behaviors, Feeds. ([Flows](file:///\\wsl.localhost\Debian\home\sam\locallife\doc\api\flows\personalized_feed.md), [Swagger](file:///\\wsl.localhost\Debian\home\sam\locallife\doc\api\swagger\reco_v1.json))
- [In-Store Ops & KDS](file:///\\wsl.localhost\Debian\home\sam\locallife\doc\api\in_store_and_kds.md): Tables, QR, Kitchen. ([Flows](file:///\\wsl.localhost\Debian\home\sam\locallife\doc\api\flows\table_reservation.md), [Swagger](file:///\\wsl.localhost\Debian\home\sam\locallife\doc\api\swagger\instore_v1.json))
- [Operator Role](file:///\\wsl.localhost\Debian\home\sam\locallife\doc\api\operator_role.md): Regional BI, Audits. ([Flows](file:///\\wsl.localhost\Debian\home\sam\locallife\doc\api\flows\regional_oversight.md), [Swagger](file:///\\wsl.localhost\Debian\home\sam\locallife\doc\api\swagger\operator_v1.json))
- [Governance Role](file:///\\wsl.localhost\Debian\home\sam\locallife\doc\api\governance_role.md): Trust scores, Claims.
- [Business Flows](file:///\\wsl.localhost\Debian\home\sam\locallife\doc\api\business_flows.md): Global state machines.
