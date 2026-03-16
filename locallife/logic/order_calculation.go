package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/maps"
)

const (
	// 地理常量已迁移至 logic/geo_constants.go，这里仅保留本包内的别名以兼容现有代码。
	defaultDeliveryDistance = DefaultDeliveryDistance
	minDeliveryDistance     = MinDeliveryDistance
	metersPerLatDegree      = MetersPerLatDegree
	metersPerLngDegree      = MetersPerLngDegree
)

// OrderCalculationInput defines the input for order preview calculation.
type OrderCalculationInput struct {
	UserID        int64
	MerchantID    int64
	OrderType     string
	Latitude      *float64
	Longitude     *float64
	AddressID     *int64
	UserVoucherID *int64
	VoucherCode   string
}

// OrderCalculationItem describes a cart item for preview.
type OrderCalculationItem struct {
	DishID    *int64
	ComboID   *int64
	Name      string
	UnitPrice int64
	Quantity  int16
	Subtotal  int64
}

// OrderPromotion describes an applied promotion.
type OrderPromotion struct {
	Type   string
	Title  string
	Amount int64
}

// OrderCalculationResult contains computed preview totals.
type OrderCalculationResult struct {
	Subtotal            int64
	DeliveryFee         int64
	DeliveryFeeDiscount int64
	DiscountAmount      int64
	TotalAmount         int64
	Promotions          []OrderPromotion
	Items               []OrderCalculationItem
}

// CalculateOrderPreview computes order totals based on cart and delivery inputs.
func CalculateOrderPreview(
	ctx context.Context,
	store db.Store,
	mapClient maps.TencentMapClientInterface,
	input OrderCalculationInput,
	normalize NormalizeDishCustomizationsFunc,
	calculateFee DeliveryFeeCalculator,
) (OrderCalculationResult, error) {
	var result OrderCalculationResult

	cart, err := store.GetCartByUserAndMerchant(ctx, db.GetCartByUserAndMerchantParams{
		UserID:     input.UserID,
		MerchantID: input.MerchantID,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusBadRequest, errors.New("cart is empty"))
		}
		return result, err
	}

	cartItems, err := store.ListCartItems(ctx, cart.ID)
	if err != nil {
		return result, err
	}
	if len(cartItems) == 0 {
		return result, NewRequestError(http.StatusBadRequest, errors.New("cart is empty"))
	}

	items := make([]OrderCalculationItem, len(cartItems))
	for i, item := range cartItems {
		var name string
		var price int64
		if item.DishID.Valid {
			name = item.DishName.String
			price = item.DishPrice.Int64

			var customizationMap map[string]interface{}
			if len(item.Customizations) > 0 {
				if err := json.Unmarshal(item.Customizations, &customizationMap); err != nil {
					return result, NewRequestError(http.StatusBadRequest, errors.New("invalid customizations in cart"))
				}
			}
			if normalize == nil {
				return result, NewRequestError(http.StatusInternalServerError, errors.New("customizations handler is not configured"))
			}
			_, extraPrice, err := normalize(ctx, item.DishID.Int64, customizationMap)
			if err != nil {
				return result, NewRequestError(http.StatusBadRequest, err)
			}
			price += extraPrice
		} else if item.ComboID.Valid {
			name = item.ComboName.String
			price = item.ComboPrice.Int64
			if len(item.Customizations) > 0 {
				return result, NewRequestError(http.StatusBadRequest, errors.New("customizations not supported for combo items"))
			}
		}

		itemSubtotal := price * int64(item.Quantity)
		items[i] = OrderCalculationItem{
			Name:      name,
			UnitPrice: price,
			Quantity:  item.Quantity,
			Subtotal:  itemSubtotal,
		}
		if item.DishID.Valid {
			dishID := item.DishID.Int64
			items[i].DishID = &dishID
		}
		if item.ComboID.Valid {
			comboID := item.ComboID.Int64
			items[i].ComboID = &comboID
		}
		result.Subtotal += itemSubtotal
	}

	result.Items = items
	result.Promotions = []OrderPromotion{}

	if input.OrderType == "takeout" {
		merchant, err := store.GetMerchant(ctx, input.MerchantID)
		if err != nil {
			return result, err
		}

		var userLat, userLng float64
		regionID := merchant.RegionID

		if input.AddressID != nil {
			address, err := store.GetUserAddress(ctx, *input.AddressID)
			if err == nil && address.UserID == input.UserID {
				if address.Latitude.Valid && address.Longitude.Valid {
					lat, _ := address.Latitude.Float64Value()
					lng, _ := address.Longitude.Float64Value()
					userLat = lat.Float64
					userLng = lng.Float64
				}
				regionID = address.RegionID
			}
		} else if input.Latitude != nil && input.Longitude != nil {
			userLat = *input.Latitude
			userLng = *input.Longitude
		}

		deliveryDistance := int32(defaultDeliveryDistance)
		if userLat != 0 && userLng != 0 && merchant.Latitude.Valid && merchant.Longitude.Valid {
			merchantLat, _ := merchant.Latitude.Float64Value()
			merchantLng, _ := merchant.Longitude.Float64Value()
			if mapClient != nil {
				fromLoc := maps.Location{Lat: merchantLat.Float64, Lng: merchantLng.Float64}
				toLoc := maps.Location{Lat: userLat, Lng: userLng}
				routeResult, err := mapClient.GetBicyclingRoute(ctx, fromLoc, toLoc)
				if err == nil && routeResult != nil {
					deliveryDistance = int32(routeResult.Distance)
				}
			} else {
				latDiff := (userLat - merchantLat.Float64) * metersPerLatDegree
				lngDiff := (userLng - merchantLng.Float64) * metersPerLngDegree
				dist := int32(latDiff*latDiff + lngDiff*lngDiff)
				if dist > 0 {
					deliveryDistance = int32(float64(dist) / 1000)
					if deliveryDistance < minDeliveryDistance {
						deliveryDistance = minDeliveryDistance
					}
				}
			}
		}

		if calculateFee == nil {
			return result, NewRequestError(http.StatusInternalServerError, errors.New("delivery fee calculator is required"))
		}
		feeResult, err := calculateFee(ctx, regionID, input.MerchantID, deliveryDistance, result.Subtotal)
		if err == nil {
			result.DeliveryFee = feeResult.Fee
			if feeResult.Discount > 0 {
				result.DeliveryFeeDiscount = feeResult.Discount
				result.Promotions = append(result.Promotions, OrderPromotion{
					Type:   "delivery_fee_return",
					Title:  "满额返运费",
					Amount: feeResult.Discount,
				})
			}
		}
	}

	discountRules, err := store.ListActiveDiscountRules(ctx, input.MerchantID)
	if err == nil {
		var bestDiscount db.DiscountRule
		var bestFound bool
		for _, rule := range discountRules {
			if result.Subtotal >= rule.MinOrderAmount {
				if !bestFound || rule.DiscountAmount > bestDiscount.DiscountAmount {
					bestDiscount = rule
					bestFound = true
				}
			}
		}
		if bestFound {
			result.DiscountAmount = bestDiscount.DiscountAmount
			result.Promotions = append(result.Promotions, OrderPromotion{
				Type:   "discount",
				Title:  fmt.Sprintf("满%d减%d", bestDiscount.MinOrderAmount/100, bestDiscount.DiscountAmount/100),
				Amount: bestDiscount.DiscountAmount,
			})
		}
	}

	if input.VoucherCode != "" {
		return result, NewRequestError(http.StatusBadRequest, errors.New("请使用 user_voucher_id 进行金额预览"))
	}
	if input.UserVoucherID != nil {
		voucher, err := store.GetUserVoucher(ctx, *input.UserVoucherID)
		if err != nil {
			if errors.Is(err, db.ErrRecordNotFound) {
				return result, NewRequestError(http.StatusNotFound, errors.New("优惠券不存在"))
			}
			return result, err
		}
		if voucher.UserID != input.UserID {
			return result, NewRequestError(http.StatusForbidden, errors.New("优惠券不属于您"))
		}
		if voucher.Status != "unused" {
			return result, NewRequestError(http.StatusBadRequest, errors.New("优惠券已使用或已过期"))
		}
		if time.Now().After(voucher.ExpiresAt) {
			return result, NewRequestError(http.StatusBadRequest, errors.New("优惠券已过期"))
		}
		if voucher.MerchantID != input.MerchantID {
			return result, NewRequestError(http.StatusBadRequest, errors.New("该优惠券不能在此商户使用"))
		}
		if result.Subtotal < voucher.MinOrderAmount {
			return result, NewRequestError(http.StatusBadRequest, fmt.Errorf("未达到最低消费 %d 元", voucher.MinOrderAmount/100))
		}

		orderTypeAllowed := false
		for _, allowed := range voucher.AllowedOrderTypes {
			if allowed == input.OrderType {
				orderTypeAllowed = true
				break
			}
		}
		if !orderTypeAllowed {
			return result, NewRequestError(http.StatusBadRequest, errors.New("该代金券不适用于此订单类型"))
		}

		result.DiscountAmount += voucher.Amount
		result.Promotions = append(result.Promotions, OrderPromotion{
			Type:   "voucher",
			Title:  voucher.Name,
			Amount: voucher.Amount,
		})
	}

	result.TotalAmount = result.Subtotal + result.DeliveryFee - result.DeliveryFeeDiscount - result.DiscountAmount
	if result.TotalAmount < 0 {
		result.TotalAmount = 0
	}

	return result, nil
}
