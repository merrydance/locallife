# Regional Oversight & Trust Governance Flows

This document details the platform-level governance and operational steering. Technical endpoint details can be found in [operator_v1.json](../swagger/operator_v1.json).

## 1. Regional Steering & Audit Flow
How operators manage their assigned territory.

```mermaid
sequenceDiagram
    participant Op as Operator App
    participant API as LocalLife API
    participant Store as Merchant
    participant BI as Analytics Engine

    Op->>API: GET /v1/operator/regions/:id/stats
    API->>BI: Aggregated Regional Volume
    BI-->>API: High Latency Warning in Zone B
    API-->>Op: Alert: Delivery Delay Spike
    
    Op->>API: POST /v1/operator/regions/:id/peak-hours
    Note right of API: Adjusts dynamic fee multipliers
    API-->>Op: Multiplier updated to 1.5x

    Op->>API: GET /v1/operator/audit/merchants/pending
    API-->>Op: List of license OCRs to verify
    Op->>API: POST /v1/operator/audit/merchants/:id/approve
```

---

## 2. Trust Score Arbitration & Auto-Claim
The logic behind the "Governance" role.

```mermaid
sequenceDiagram
    participant User as Consumer
    participant API as LocalLife API
    participant TS as Trust System
    participant Pay as Settlement Engine

    User->>API: POST /v1/claims (Refund Request)
    API->>TS: GET /v1/users/me/trust-score
    TS-->>API: Score: 98 (Trusted)
    
    API->>API: Evaluate AI-AutoApproval (Decision: Instant)
    API->>Pay: Initiate Refund Request
    Pay-->>API: Success
    API-->>User: PUSH: Mutual Trust Approved (Instant Refund)

    Note over API,TS: If score was < 60, routed to Manual Review
```

---

## 3. Recommendation Intelligence Flow
Data loop for the Personalized Feed.

```mermaid
sequenceDiagram
    participant App as Frontend
    participant API as LocalLife API
    participant Reco as Reco-Engine
    participant Redis as Feature Cache

    App->>API: POST /v1/track/behavior (Dish_Click)
    API->>Redis: Update User Interest Profile
    
    App->>API: GET /v1/recommendations/feeds
    API->>Reco: Fetch candidates (Location + Performance)
    Reco->>Redis: Filter by User Affinity (Real-time)
    Redis-->>Reco: Filtered Ranking
    Reco-->>API: Ranked Dish/Store IDs
    API-->>App: Personalized Discovery Feed
```
