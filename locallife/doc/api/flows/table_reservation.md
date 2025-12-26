# In-Store Operations: Table & Room Reservation Flows

This document details the sequences for offline-to-online (O2O) in-store interactions. Technical endpoint details can be found in [instore_v1.json](../swagger/instore_v1.json).

## 1. Table/Room Reservation & Dish Pre-order
The flow for scheduling a meal and pre-selecting dishes.

```mermaid
sequenceDiagram
    participant Cust as Customer App
    participant API as LocalLife API
    participant Pay as WeChat Pay
    participant KDS as Kitchen Terminal

    Cust->>API: GET /v1/search/rooms?cap=8
    API-->>Cust: List of Available Rooms & Timeslots
    
    Cust->>API: POST /v1/reservations
    Note over Cust,API: Selects Room + Timeslot + Deposit
    API-->>Cust: Reservation Created (Status: Pending_Pay)

    Cust->>API: POST /v1/payments (Business: reservation)
    API->>Pay: Unified Order
    Pay-->>Cust: Success
    API-->>Cust: Reservation Confirmed

    Cust->>API: POST /v1/reservations/:id/dishes
    Note right of API: User pre-orders appetizers
```

---

## 2. QR Scanning & In-Store Joint Ordering
How multiple users join a single table's ordering session.

```mermaid
sequenceDiagram
    participant U1 as User A (Host)
    participant U2 as User B (Guest)
    participant API as LocalLife API
    participant WS as WebSocket Hub

    U1->>API: GET /v1/scan/table?no=A01
    API-->>U1: Session ID (Order In-Progress)
    
    U2->>API: GET /v1/scan/table?no=A01
    API-->>U2: Syncing with Session ID
    
    U1->>API: POST /v1/cart/items (Dish: Chicken)
    API->>WS: Broadcast: User A added Chicken
    WS-->>U2: PUSH: Cart Updated
```
