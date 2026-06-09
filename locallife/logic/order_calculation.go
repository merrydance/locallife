package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

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
	SuggestedVoucher    *SuggestedVoucher
	LadderPromotions    []LadderPromotion
	VoucherTrials       []VoucherTrial
	PaymentAssessment   PaymentAssessment
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
		OrderType:  input.OrderType,
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
				return result, fmt.Errorf("customizations handler: not configured")
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
	result.LadderPromotions = []LadderPromotion{}
	result.VoucherTrials = []VoucherTrial{}

	if input.OrderType == "takeout" {
		merchant, err := store.GetMerchant(ctx, input.MerchantID)
		if err != nil {
			return result, err
		}

		var userLat, userLng float64
		regionID := merchant.RegionID

		if input.AddressID != nil {
			address, err := loadOwnedUserAddress(ctx, store, input.UserID, *input.AddressID)
			if err != nil {
				return result, err
			}
			if address.Latitude.Valid && address.Longitude.Valid {
				lat, _ := address.Latitude.Float64Value()
				lng, _ := address.Longitude.Float64Value()
				userLat = lat.Float64
				userLng = lng.Float64
			}
			regionID = address.RegionID
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
			return result, fmt.Errorf("delivery fee calculator: not configured")
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

	if input.VoucherCode != "" {
		return result, NewRequestError(http.StatusBadRequest, errors.New("请使用 user_voucher_id 进行金额预览"))
	}
	if input.UserVoucherID != nil {
		_, err := ValidateVoucher(ctx, store, VoucherValidationInput{
			UserID:        input.UserID,
			MerchantID:    input.MerchantID,
			OrderType:     input.OrderType,
			Subtotal:      result.Subtotal,
			UserVoucherID: input.UserVoucherID,
		})
		if err != nil {
			return result, err
		}
		if resolvedDiscount, getErr := ResolveMerchantDiscount(ctx, store, OrderContext{
			MerchantID: input.MerchantID,
			OrderType:  input.OrderType,
			Subtotal:   result.Subtotal,
		}); getErr == nil && !resolvedDiscount.AllowWithVoucher {
			return result, NewRequestError(http.StatusBadRequest, errors.New("当前活动不可与所选优惠券叠加"))
		}
	}

	engine := NewPromotionEngine(store)
	calcResult, err := engine.CalculateFinalPrice(ctx, OrderContext{
		MerchantID:          input.MerchantID,
		UserID:              input.UserID,
		OrderType:           input.OrderType,
		Subtotal:            result.Subtotal,
		VoucherID:           input.UserVoucherID,
		DeliveryFee:         result.DeliveryFee,
		DeliveryFeeDiscount: result.DeliveryFeeDiscount,
	})
	if err != nil {
		return result, err
	}

	result.DeliveryFee = calcResult.DeliveryFee
	result.DeliveryFeeDiscount = calcResult.DeliveryFeeDiscount
	result.DiscountAmount = calcResult.MerchantDiscount + calcResult.VoucherDiscount
	result.TotalAmount = calcResult.TotalAmount
	result.SuggestedVoucher = calcResult.SuggestedVoucher
	result.LadderPromotions = calcResult.LadderPromotions
	result.VoucherTrials = calcResult.VoucherTrials
	result.PaymentAssessment = calcResult.PaymentAssessment
	result.Promotions = make([]OrderPromotion, 0, len(calcResult.AppliedPromotions))
	for _, promo := range calcResult.AppliedPromotions {
		result.Promotions = append(result.Promotions, OrderPromotion{
			Type:   promo.Type,
			Title:  promo.Title,
			Amount: promo.Amount,
		})
	}

	return result, nil
}
