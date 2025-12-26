# In-Store Operations & KDS API (Waiters & Chefs)

Focus: Optimizing the physical dining experience and kitchen efficiency.

## 1. Table & Room Management

### Digital Table Entry
- `GET /v1/tables/:id/qrcode`: Generates the QR code for a specific table for scan-to-order.
- `PATCH /v1/tables/:id/status`: Update state (Occupied, Idle, Cleaning).
- `GET /v1/merchants/:id/rooms`: Real-time availability for private dining rooms.

### Private Dining Reservations
- `POST /v1/reservations`: User books a room and optionally pre-orders dishes.
- `POST /v1/reservations/:id/confirm`: Merchant locks the booking (usually after deposit).
- `POST /v1/reservations/:id/no-show`: Mark as missed (triggers trust score penalty).

---

## 2. KDS (Kitchen Display System)

### Order Awareness
- `GET /v1/kitchen/orders`: Continuous polling or WebSocket-driven list of tickets to prepare.
- `GET /v1/kitchen/orders/:id`: Detailed view of a ticket (including customizations like "No Onion").

### Preparation Lifecycle
- `POST /v1/kitchen/orders/:id/preparing`: Chef accepts the ticket; status moves to `preparing`.
- `POST /v1/kitchen/orders/:id/ready`: Chef completes cooking; status moves to `ready` for pickup/server.

---

## 3. On-Site Hardware

### Printer Management
- `POST /v1/merchant/devices`: Register cloud printers (for kitchen slips or guest checks).
- `POST /v1/merchant/devices/:id/test`: Print a test page to verify connectivity.
