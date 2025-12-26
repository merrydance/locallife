# Merchant Onboarding & Operations Flows

This document details the business and technical sequences for Merchant SaaS operations. Technical endpoint details can be found in [merchant_v1.json](../swagger/merchant_v1.json).

## 1. Merchant Onboarding (WeChat Ecommerce V3)
The complex multi-stage process from draft to active store.

```mermaid
sequenceDiagram
    participant Merchant as Merchant App
    participant API as LocalLife API
    participant Audit as Operator / Auto-Audit
    participant WC as WeChat Pay (Ecommerce)

    Merchant->>API: POST /v1/merchant/application/license/ocr
    API-->>Merchant: Extracted Name, Registered No.
    
    Merchant->>API: POST /v1/merchant/application/submit
    API->>Audit: Forward to Audit Engine
    Audit-->>API: Status: APPROVED
    API-->>Merchant: Application Success

    Merchant->>API: POST /v1/merchant/applyment/bindbank
    Note over API,WC: Initiates WeChat Merchant Applyment
    API->>WC: Upload Docs to WeChat V3
    WC-->>API: Applyment ID
    API-->>Merchant: Pending WeChat Review

    loop Status Check
        Merchant->>API: GET /v1/merchant/applyment/status
        API->>WC: Query Applyment
        WC-->>API: State: FINISHED/REJECTED
        API-->>Merchant: WeChat Entry Result
    end
```

---

## 2. Menu & Kitchen Operation (KDS)
Flow of items from management to kitchen display.

```mermaid
sequenceDiagram
    participant Merchant as Merchant App
    participant API as LocalLife API
    participant KDS as Kitchen Terminal
    participant Printer as Hardware Printer

    Merchant->>API: POST /v1/dishes (Create item)
    API-->>Merchant: Status: Active

    Note over API: When Order is Paid
    API->>KDS: Webhook/WS: New Order Items
    KDS-->>API: GET /v1/kitchen/orders
    
    KDS->>API: POST /v1/kitchen/orders/:id/preparing
    API-->>Printer: EscPos: Print Order Ticket
    
    KDS->>API: POST /v1/kitchen/orders/:id/ready
    API-->>Merchant: WS: Order Ready for Pickup/Delivery
```

---

## 3. Financial Settlement
Weekly cycles and service fee deductions.

```mermaid
stateDiagram-v2
    OrderPaid --> ActiveBalance: Service Fee Deducted (e.g. 5%)
    ActiveBalance --> Generating_Settlement: Sunday 23:59
    Generating_Settlement --> Pending_Review: System check
    Pending_Review --> Transferring: Automated Batch
    Transferring --> Successful: Bank Receipt
    Transferring --> Failed: Check Bank Info
```
