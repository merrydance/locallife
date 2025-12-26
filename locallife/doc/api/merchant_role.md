# Merchant Role API (Operations & SaaS)

Focus: Managing menus, tables, and financial onboarding (WeChat V3).

## 1. Onboarding & Compliance (Digital Entry)

### Bind Bank Account (WeChat Ecommerce V3)
- `POST /v1/merchant/bindbank`
- **Payload (`merchantBindBankRequest`)**:
  ```json
  {
    "account_type": "ACCOUNT_TYPE_BUSINESS", 
    "account_bank": "招商银行",
    "bank_address_code": "440300",
    "account_number": "6222...",
    "account_name": "某某餐厅",
    "contact_phone": "138..."
  }
  ```
- **States**: `pending` -> `submitted` -> `auditing` -> `to_be_signed` -> `finish`.

---

## 2. Menu & Supply Management (SaaS Setup)

### Dishes & Categories
- `POST /v1/dishes`: Create a new dish (supports price in **cents**, image URLs, and ingredient tags).
- `POST /v1/dishes/categories`: Logical grouping (e.g., "Main Course").
- `PATCH /v1/dishes/:id/status`: Toggle online/offline (Shelving).

### Combos & Sets
- `POST /v1/combos`: Define a set meal (e.g., "Family Feast").
- `PUT /v1/combos/:id/dishes`: Add/Remove specific items from a combo.
- `PUT /v1/combos/:id/online`: Publish the set to the store.

### Daily Inventory (Stock Control)
- `POST /v1/inventory`: Bulk initialize stock for the day.
- `PATCH /v1/inventory/:dish_id`: Real-time stock adjustment (e.g., "Sold Out" override).
- `GET /v1/inventory/stats`: Low-stock alerts and sell-through rates.

---

## 3. Order & Kitchen Mgmt

### List Store Orders
- `GET /v1/merchants/:id/orders`
- **Response**: List of `orderResponse` objects.

### Update Order Status
- `POST /v1/orders/:id/prepare`: Mark as cooking.
- `POST /v1/orders/:id/ready`: Mark as ready for pickup.

---

## 4. Operational BI
- `GET /v1/merchant/stats/daily`: Sales volume & order count.
- `GET /v1/merchant/finance/balance`: Detailed account statement.
