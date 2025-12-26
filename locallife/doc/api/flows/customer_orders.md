# Customer Order Life-cycle Flows

This document details the visual sequences for the most complex customer interactions. Technical endpoint details can be found in [customer_v1.json](../swagger/customer_v1.json).

## 1. Discovery to Cart Flow
The sequence from landing on the app to preparing a cart.

```mermaid
sequenceDiagram
    participant User as Consumer App
    participant API as LocalLife API
    participant Map as Tencent Map Service
    participant Reco as Recommendation Engine

    User->>API: GET /v1/regions/available?lat=...&long=...
    API-->>User: Region ID (Current Location)
    
    User->>API: GET /v1/recommendations/dishes?limit=20
    API->>Reco: Fetch EE-Algorithm Result
    Reco-->>API: Personalized Dish IDs
    API-->>User: Top Recommended Dishes (with Dist/Fee)

    User->>API: GET /v1/merchants/:id/rooms/all
    API-->>User: List of Rooms & Availability

    User->>API: POST /v1/cart/items
    Note over User,API: User adds Dishes to Cart
    API-->>User: Cart Updated
```

---

## 2. Combined Order Checkout (Multi-Merchant)
LocalLife supports multi-merchant combined checkout.

```mermaid
sequenceDiagram
    participant User as Consumer App
    participant API as LocalLife API
    participant Pay as WeChat Pay (Ecommerce)

    User->>API: GET /v1/cart/summary
    API-->>User: List of Carts grouped by Merchant

    User->>API: POST /v1/cart/combined-checkout/preview
    Note right of API: Calculates combined delivery fees,<br/>discounts, and coupons.
    API-->>User: Preview Result (Total Amount, Savings)

    User->>API: POST /v1/orders
    API->>API: Create Parent Order & Sub-orders
    API-->>User: Parent Order ID
    
    User->>API: POST /v1/payments
    API->>Pay: Initiate Combine Payment
    Pay-->>User: WeChat Pay Parameter (JSAPI/SDK)
```

---

## 3. Order Tracking & Delivery Status
State transitions during the delivery process.

```mermaid
stateDiagram-v2
    [*] --> Placed: User Submits Order
    Placed --> Paid: Payment Successful
    Paid --> Merchant_Accepted: Store accepts order
    Merchant_Accepted --> Preparing: KDS status: Preparing
    Preparing --> Ready: Food is cooked
    Ready --> Rider_Picked: Rider confirmed pickup
    Rider_Picked --> Delivering: Proximity < 200m
    Delivering --> Completed: Confirmation
    
    Paid --> Cancelled: No Merchant Acceptance
    Merchant_Accepted --> Refunded: Exceptional cancel
```
