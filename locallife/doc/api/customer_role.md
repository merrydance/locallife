# Customer Role API (Discovery & Transactions)

Focus: Helping users find food and complete orders with transparency.

## 1. Discovery (Search & Recommendation)

### Search Merchants
- `GET /v1/search/merchants`
- **Query Params**: 
  - `keyword` (string, required)
  - `user_latitude` (float64, optional)
  - `user_longitude` (float64, optional)
  - `page_id`, `page_size`
- **Response**: List of merchants with `distance` (meters) and `estimated_delivery_fee` (分) if location provided.

### Search Dishes
- `GET /v1/search/dishes`
- **Query Params**: `keyword`, `merchant_id` (optional), `page_id`, `page_size`.
- **Response Fields**: `id`, `merchant_id`, `name`, `description`, `price` (float64, in Yuan), `is_available`.

---

## 2. Cart (Client-Side State Mirror)

### Add to Cart
- `POST /v1/cart/items`
- **Payload**:
  ```json
  {
    "merchant_id": 10001,
    "dish_id": 1001,
    "quantity": 2,
    "customizations": { "辣度": "中辣" }
  }
  ```
- **Constraint**: Provide either `dish_id` or `combo_id`.

---

## 3. Order Lifecycle

### Create Order
- `POST /v1/orders`
- **Payload (`createOrderRequest`)**:
  ```json
  {
    "merchant_id": 20001,
    "order_type": "takeout", 
    "address_id": 5001,
    "items": [
      {
        "dish_id": 1001,
        "quantity": 2,
        "customizations": [
          { "name": "辣度", "value": "中辣", "extra_price": 0 }
        ]
      }
    ],
    "notes": "不要香菜",
    "user_voucher_id": 9001,
    "use_balance": false
  }
  ```
- **Order Types**: `takeout` (addr required), `dine_in` (table required), `takeaway`, `reservation` (reserv_id required).

### Initiate Payment
- `POST /v1/payments`
- **Payload**: `{"order_id": 123, "payment_type": "miniprogram", "business_type": "order"}`
- **Response**: Includes `pay_params` for WeChat Mini-Program JSAPI call.

---

## 4. Personal Space
- `GET /v1/users/me`: Current profile & role status.
- `GET /v1/addresses`: Delivery address list.
- `GET /v1/vouchers/me`: Collected discount coupons.
