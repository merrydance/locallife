# LocalLife Business Flows (Implementation Truth)

Detailed state transitions as implemented in the Go API layer.

## 1. Core Order Transaction Flow

```mermaid
stateDiagram-v2
    [*] --> pending : createOrder
    pending --> paid : handlePaymentNotify
    pending --> cancelled : OrderTimeout / UserCancel
    
    paid --> preparing : acceptOrder (Merchant)
    paid --> cancelled : rejectOrder (Merchant)
    
    preparing --> ready : markOrderReady (Merchant)
    
    ready --> delivering : confirmPickup (Rider)
    ready --> completed : completeOrder (Dine-in/Takeaway)
    
    delivering --> completed : delivered (Rider)
```

### Transition Logic Truth
- **Automatic Cancellation**: If `pending` isn't paid within 30 mins, a background task (`DistributeTaskOrderTimeout`) cancels it.
- **Merchant Responsibility**: Merchants move orders from `paid` to `ready`.
- **Rider Responsibility**: Riders move orders from `ready` (assigned) to `completed`.

---

## 2. Onboarding Lifecycle (WeChat V3)

```mermaid
stateDiagram-v2
    [*] --> draft : UserApply
    draft --> approved : AdminReview
    approved --> submitted : bindBank
    submitted --> auditing : WeChatProcess
    auditing --> to_be_signed : WeChatApprove
    to_be_signed --> finish : UserSign
    finish --> active : SystemActivate
```

---

## 3. Trust-Based Settlement Flow

```mermaid
stateDiagram-v2
    [*] --> Analysis
    state Analysis {
        HighTrust --> InstantApproval
        MediumTrust --> AutoApproval
        LowTrust --> ManualAudit
    }
    
    InstantApproval --> FundReleased : Seconds
    AutoApproval --> FundReleased : 24h
    ManualAudit --> FundReleased : After客服Review
```
