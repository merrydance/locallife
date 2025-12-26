# Marketing & CRM Systems

This module describes the mechanisms used by merchants to drive traffic and retention, and how customers interact with these promotional tools.

## 1. Membership Program
The membership system allows merchants to build a loyal customer base through prepaid balances and exclusive member pricing.

### Core Business Logic
- **Prepaid Model**: Users join a merchant's membership and recharge their balance.
- **Bonus Incentives**: Merchants define recharge rules (e.g., "Recharge 100, get 20 bonus").
- **Member Pricing**: Dishes can have a `member_price` which is automatically applied for members.
- **Transaction History**: All balance changes (recharge/consumption) are logged.

### Key Workflows
1. **Joining**: User calls `POST /v1/memberships` with `merchant_id`.
2. **Recharging**: User selects a rule and支付 (handled via payment gateway).
3. **Usage**: Balance is deducted during checkout if the user selects "Membership Balance" as the payment method.

---

## 2. Voucher System
Vouchers (代金券) are "claimable" discounts that users store in their "wallet" and apply at checkout.

### Voucher Types & Constraints
- **Amount & MinSpend**: Fixed discount (e.g., 10 off 50).
- **Redemption Limit**: Total number of vouchers that can be issued.
- **Quota per User**: How many times a single user can claim this voucher.
- **Applicability**: Can be restricted to specific order types (e.g., `DINE_IN`, `DELIVERY`).

### Workflows
- **Merchant Creation**: `POST /v1/merchants/{id}/vouchers`.
- **User Discovery**: `GET /v1/merchants/{id}/vouchers/active` shows available coupons.
- **Claiming**: `POST /v1/vouchers/{voucher_id}/claim`.
- **Automatic Optimization**: `GET /v1/vouchers/available?merchant_id=...` returns the best voucher for the current cart.

---

## 3. Discount Rules (Auto-Applied)
Unlike vouchers, Discount Rules (满减规则) are automatically applied to the order if conditions are met.

### Rule Properties
- **Stacking**: A boolean `can_stack_with_membership` determines if the rule can be combined with member discounts.
- **Priority**: System automatically selects the rule that provides the maximum discount.
- **Validity**: Rules can be scheduled for future dates.

---

## Technical Enums & Structs

### Behavior Types
Used for tracking and influencing the recommendation engine:
- `view`: General browsing.
- `detail`: Viewing specific dish/item details.
- `cart`: Adding items to cart.
- `purchase`: Successful transaction.

### Voucher Status
- `Active`: Available for claiming.
- `Inactive`: Template exists but not currently issued.
- `Expired`: Past the `valid_until` date.
