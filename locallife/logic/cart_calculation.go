package logic

import (
	"context"
	"errors"
	"math"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/maps"
)

// CartPreviewInput holds the inputs for cart price preview calculation.
type CartPreviewInput struct {
	UserID        int64
	MerchantID    int64
	OrderType     string
	TableID       *int64
	ReservationID *int64
	// AddressID is used to fetch the delivery address; takes precedence over Latitude/Longitude.
	AddressID *int64
	// Latitude / Longitude are used when no AddressID is provided.
	Latitude  *float64
	Longitude *float64
	VoucherID *int64
}

// CartPreviewResult holds the fully computed cart preview.
type CartPreviewResult struct {
	Subtotal            int64
	DeliveryFee         int64
	DeliveryFeeDiscount int64
	DeliveryDistance    int32
	RouteDurationSec    int
	ETA                 DeliveryETAResult
	Promotion           *PriceCalculationResult
	MinOrderAmount      int64
	MeetsMinOrder       bool
}

// CalculateCartPreview computes the full cart preview including delivery fee, ETA,
// and applied promotions. The caller is responsible for validating merchant status
// before calling this function.
//
// merchant must already be fetched and its status already verified by the caller.
// feeFn bridges the server-layer delivery fee calculator into this pure-logic function.
func CalculateCartPreview(
	ctx context.Context,
	store db.Store,
	mapClient maps.TencentMapClientInterface,
	merchant db.Merchant,
	feeFn DeliveryFeeCalculator,
	input CartPreviewInput,
) (CartPreviewResult, error) {
	var result CartPreviewResult

	// Build pgtype wrappers for optional FK fields used in the cart query.
	var tableID, reservationID pgtype.Int8
	if input.TableID != nil {
		tableID = pgtype.Int8{Int64: *input.TableID, Valid: true}
	}
	if input.ReservationID != nil {
		reservationID = pgtype.Int8{Int64: *input.ReservationID, Valid: true}
	}

	cart, err := store.GetCartByUserAndMerchant(ctx, db.GetCartByUserAndMerchantParams{
		UserID:        input.UserID,
		MerchantID:    input.MerchantID,
		OrderType:     input.OrderType,
		TableID:       tableID,
		ReservationID: reservationID,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusBadRequest, errors.New("购物车为空"))
		}
		return result, err
	}

	items, err := store.ListCartItems(ctx, cart.ID)
	if err != nil {
		return result, err
	}
	if len(items) == 0 {
		return result, NewRequestError(http.StatusBadRequest, errors.New("购物车为空"))
	}

	// Calculate order subtotal from cart items.
	var subtotal int64
	for _, item := range items {
		var unitPrice int64
		if item.DishID.Valid {
			unitPrice = item.DishPrice.Int64
		} else if item.ComboID.Valid {
			unitPrice = item.ComboPrice.Int64
		}
		subtotal += unitPrice * int64(item.Quantity)
	}
	result.Subtotal = subtotal

	if input.OrderType == db.OrderTypeTakeout {
		// Resolve delivery route + fee.
		if input.AddressID != nil {
			address, err := store.GetUserAddress(ctx, *input.AddressID)
			if err != nil || address.UserID != input.UserID {
				return result, NewRequestError(http.StatusBadRequest, errors.New("地址无效"))
			}
			if !address.Latitude.Valid || !address.Longitude.Valid || !merchant.Latitude.Valid || !merchant.Longitude.Valid {
				return result, NewRequestError(http.StatusBadRequest, errors.New("无法获取距离，请重新选择地址"))
			}
			distance, durationSec, feeComp := resolveRouteAndFee(ctx, merchant, mapClient, feeFn, pgNumericToFloat64(address.Latitude), pgNumericToFloat64(address.Longitude), subtotal)
			result.DeliveryDistance = distance
			result.RouteDurationSec = durationSec
			if !feeComp.Suspended {
				result.DeliveryFee = feeComp.Fee
				result.DeliveryFeeDiscount = feeComp.Discount
			}
		} else if input.Latitude != nil && input.Longitude != nil {
			if !merchant.Latitude.Valid || !merchant.Longitude.Valid {
				return result, NewRequestError(http.StatusBadRequest, errors.New("无法获取距离，请重新选择位置"))
			}
			distance, durationSec, feeComp := resolveRouteAndFee(ctx, merchant, mapClient, feeFn, *input.Latitude, *input.Longitude, subtotal)
			result.DeliveryDistance = distance
			result.RouteDurationSec = durationSec
			if !feeComp.Suspended {
				result.DeliveryFee = feeComp.Fee
				result.DeliveryFeeDiscount = feeComp.Discount
			}
		}

		result.ETA = ComputeDeliveryETA(ctx, store, merchant.ID, result.DeliveryDistance, result.RouteDurationSec)
	}

	// Run the promotion engine for discounts, vouchers, and payment assessment.
	engine := NewPromotionEngine(store)
	calcResult, err := engine.CalculateFinalPrice(ctx, OrderContext{
		MerchantID:          merchant.ID,
		UserID:              input.UserID,
		OrderType:           input.OrderType,
		Subtotal:            subtotal,
		VoucherID:           input.VoucherID,
		DeliveryFee:         result.DeliveryFee,
		DeliveryFeeDiscount: result.DeliveryFeeDiscount,
	})
	if err != nil {
		return result, err
	}
	result.Promotion = calcResult

	// MinOrderAmount 当前固定为 0，表示无起送金额限制。
	// 商户级起送金额配置尚未引入（merchants 表不含此字段），当 DB 层添加该字段后
	// 应改为从 merchant.MinOrderAmount 读取，并在 createOrder 处同步校验。
	result.MinOrderAmount = 0
	result.MeetsMinOrder = result.Subtotal >= result.MinOrderAmount // 0 时恒为 true

	return result, nil
}

// resolveRouteAndFee computes the cycling route between merchant and the delivery
// location, then calculates the delivery fee. It always returns a result — if the
// route call fails it falls back to a straight-line Haversine estimate; if the fee
// calculator fails the returned DeliveryFeeComputation has zero values.
func resolveRouteAndFee(
	ctx context.Context,
	merchant db.Merchant,
	mapClient maps.TencentMapClientInterface,
	feeFn DeliveryFeeCalculator,
	userLat, userLng float64,
	subtotal int64,
) (distance int32, durationSec int, fee DeliveryFeeComputation) {
	merchantLat := pgNumericToFloat64(merchant.Latitude)
	merchantLng := pgNumericToFloat64(merchant.Longitude)

	if mapClient != nil {
		fromLoc := maps.Location{Lat: merchantLat, Lng: merchantLng}
		toLoc := maps.Location{Lat: userLat, Lng: userLng}
		if route, err := mapClient.GetBicyclingRoute(ctx, fromLoc, toLoc); err == nil && route != nil {
			distance = int32(route.Distance)
			durationSec = route.Duration
		}
	}

	// Haversine straight-line fallback when map client is unavailable or the call failed.
	if distance == 0 {
		latDiff := (userLat - merchantLat) * metersPerDegree
		avgLatRad := (userLat + merchantLat) / 2.0 * math.Pi / 180.0
		lngDiff := (userLng - merchantLng) * metersPerDegree * math.Cos(avgLatRad)
		distance = int32(math.Sqrt(latDiff*latDiff+lngDiff*lngDiff) * 1.4)
		if distance < minDeliveryDistanceMeters {
			distance = minDeliveryDistanceMeters
		}
	}

	if feeFn != nil {
		if comp, err := feeFn(ctx, merchant.RegionID, merchant.ID, distance, subtotal); err == nil {
			fee = comp
		}
	}
	return distance, durationSec, fee
}

// pgNumericToFloat64 converts a pgtype.Numeric to float64, returning 0 on error.
func pgNumericToFloat64(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	v, err := n.Float64Value()
	if err != nil || !v.Valid {
		return 0
	}
	return v.Float64
}
