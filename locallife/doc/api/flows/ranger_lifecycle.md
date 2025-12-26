# Rider (Ranger) Delivery Lifecycle Flows

This document details the operational and financial sequences for Riders. Technical endpoint details can be found in [rider_v1.json](../swagger/rider_v1.json).

## 1. Delivery Execution Flow
The standard "Grab-to-Complete" cycle for a Rider.

```mermaid
sequenceDiagram
    participant Rider as Ranger App
    participant API as LocalLife API
    participant WS as WebSocket Hub
    participant Cust as Customer App

    API->>WS: New Order Recommendation
    WS-->>Rider: PUSH: Order Nearby (Estimated Earning: 5.5)
    
    Rider->>API: POST /v1/delivery/grab/:order_id
    API-->>Rider: Success: Delivery ID Created

    Rider->>API: POST /v1/delivery/:id/start-pickup
    Rider->>API: POST /v1/delivery/:id/confirm-pickup
    API-->>Cust: PUSH: Rider is delivering your food

    loop Location Tracking
        Rider->>API: POST /v1/rider/location (Every 30s)
        API-->>Cust: Update Map UI
    end

    Rider->>API: POST /v1/delivery/:id/confirm-delivery
    Note right of API: Verifies coordinates < 200m
    API-->>Rider: Earning Credited to Pending
    API-->>Cust: Order Completed
```

---

## 2. Trust Score & Premium Order Eligibility
How riders unlock high-value orders through service quality.

```mermaid
sequenceDiagram
    participant Rider as Ranger App
    participant TS as TrustScore Service
    participant Alloc as Order Allocation Engine

    Rider->>TS: GET /v1/rider/score
    TS-->>Rider: Current Score: 95 (Platinum)

    Note over Alloc: High-value Order Incoming (> 200 CNY)
    Alloc->>TS: Query Nearby Riders Eligibility
    TS-->>Alloc: List of Riders with Score > 90
    Alloc->>Rider: Priority PUSH Notification
    
    Rider->>API: POST /v1/rider/orders/:id/exception
    Note right of API: Late delivery (-2 points)
    API->>TS: Penalize Score
```

---

## 3. Financials: Deposit & Withdrawal
Managing the mandatory safety deposit.

```mermaid
stateDiagram-v2
    [*] --> Unregistered: First application
    Unregistered --> Pending_Deposit: Application Approved
    Pending_Deposit --> Active: Deposit Paid (e.g. 500 CNY)
    Active --> Withdrawing: Request Withdrawal (Online status: No)
    Withdrawing --> Settled: 7-Day Waiting Period
    Settled --> [*]: Account Closed
```
