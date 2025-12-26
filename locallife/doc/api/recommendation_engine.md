# Recommendation Engine

The LocalLife platform uses a sophisticated recommendation engine to personalizes the user experience across dishes, combos, and merchants. It balances user behavior data with geographic proximity and business priorities.

## 1. Behavior Tracking (埋点)
The engine relies on a stream of user interaction events to understand preferences.

### Tracking Endpoint
`POST /v1/behaviors/track`

### Event Types
- **`view`**: Impression of an item in a list.
- **`detail`**: User clicked into the item's detail page.
- **`cart`**: User added the item to their shopping cart.
- **`purchase`**: Final transaction event.

### Metadata Logged
- `duration`: Time spent on page (for `detail` views).
- `merchant_id`, `dish_id`, `combo_id`: Targeted entities.

---

## 2. Personalization API
Recommendations are delivered via specific feeds, each using a proprietary EE (Exploration & Exploitation) algorithm.

### Available Feeds
- **Dishes**: `GET /v1/recommendations/dishes`
- **Combos**: `GET /v1/recommendations/combos`
- **Merchants**: `GET /v1/recommendations/merchants`
- **In-Store Rooms**: `GET /v1/recommendations/rooms` (Focuses on "Explore Nearby" logic)

### Response Metadata
Each recommendation response includes:
- `algorithm`: The ID of the algorithm model used (for A/B testing).
- `expired_at`: TTL for the recommendation result to optimize caching.

---

## 3. Collaborative & Geospatial Logic
The recommendation engine combines several signals:
1. **User Profile**: Interests derived from historic `track` data.
2. **Geospatial Distance**: Heavily weights items within the user's delivery radius.
3. **Business Performance**: Incorporates monthly sales and ratings.
4. **Estimated Costs**: Real-time delivery fee estimates are integrated into the recommendation card.

---

## 4. Operational Oversight
Regional operators can fine-tune the recommendation engine for their specific market.

### Configuration
`PATCH /v1/regions/:id/recommendation-config`

Allows operators to:
- Adjust weights for different behaviors (e.g., weigh `purchase` 5x more than `view`).
- Set "Freshness" bias for new merchants.
- Configure default search radii.
