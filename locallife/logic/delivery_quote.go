package logic

import (
	"context"
	"errors"
	"math"
	"net/http"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/maps"
)

const (
	// 代取距离下限与经纬度转米常量已迁移至 logic/geo_constants.go，这里保留内部别名
	minDeliveryDistanceMeters = MinDeliveryDistance
	metersPerDegree           = MetersPerLatDegree
)

// DeliveryFeeComputation describes the fee computation output.
type DeliveryFeeComputation struct {
	Fee           int64
	Discount      int64
	Suspended     bool
	SuspendReason string
}

// DeliveryFeeCalculator calculates a delivery fee quote.
type DeliveryFeeCalculator func(ctx context.Context, regionID, merchantID int64, distance int32, orderAmount int64) (DeliveryFeeComputation, error)

// DeliveryQuoteInput defines the input for delivery quote computation.
type DeliveryQuoteInput struct {
	UserID    int64
	OrderType string
	Subtotal  int64
	Merchant  db.Merchant
	Address   db.UserAddress
}

// DeliveryQuoteResult holds computed delivery quote data.
type DeliveryQuoteResult struct {
	Fee           int64
	Discount      int64
	Distance      int32
	Duration      int32
	SuspendReason string
}

// ComputeDeliveryQuote calculates delivery distance and fee for a takeout order.
func ComputeDeliveryQuote(ctx context.Context, input DeliveryQuoteInput, mapClient maps.TencentMapClientInterface, calc DeliveryFeeCalculator) (DeliveryQuoteResult, error) {
	var result DeliveryQuoteResult

	if input.OrderType != "takeout" {
		return result, nil
	}

	if input.Address.UserID != input.UserID {
		return result, NewRequestError(http.StatusForbidden, errors.New("address does not belong to you"))
	}

	if !input.Address.Latitude.Valid || !input.Address.Longitude.Valid || !input.Merchant.Latitude.Valid || !input.Merchant.Longitude.Valid {
		return result, NewRequestError(http.StatusBadRequest, errors.New("invalid address or merchant location"))
	}

	userLat, _ := input.Address.Latitude.Float64Value()
	userLng, _ := input.Address.Longitude.Float64Value()
	merchantLat, _ := input.Merchant.Latitude.Float64Value()
	merchantLng, _ := input.Merchant.Longitude.Float64Value()

	calculatedDistance := int32(0)
	calculatedDuration := int32(0)
	if mapClient != nil {
		fromLoc := maps.Location{Lat: merchantLat.Float64, Lng: merchantLng.Float64}
		toLoc := maps.Location{Lat: userLat.Float64, Lng: userLng.Float64}
		routeResult, err := mapClient.GetBicyclingRoute(ctx, fromLoc, toLoc)
		if err == nil && routeResult != nil {
			calculatedDistance = int32(routeResult.Distance)
			calculatedDuration = int32(routeResult.Duration)
		}
	}

	if calculatedDistance == 0 {
		latDiff := (userLat.Float64 - merchantLat.Float64) * metersPerDegree
		avgLatRad := (userLat.Float64 + merchantLat.Float64) / 2.0 * math.Pi / 180.0
		lngDiff := (userLng.Float64 - merchantLng.Float64) * metersPerDegree * math.Cos(avgLatRad)
		calculatedDistance = int32(math.Sqrt(latDiff*latDiff+lngDiff*lngDiff) * 1.4)
	}

	if calculatedDistance < minDeliveryDistanceMeters {
		calculatedDistance = minDeliveryDistanceMeters
	}

	if calc == nil {
		return result, errors.New("delivery fee calculator is required")
	}

	feeResult, err := calc(ctx, input.Address.RegionID, input.Merchant.ID, calculatedDistance, input.Subtotal)
	if err != nil {
		return result, err
	}
	if feeResult.Suspended {
		reason := feeResult.SuspendReason
		if reason == "" {
			reason = "delivery suspended"
		}
		return result, NewRequestError(http.StatusForbidden, errors.New(reason))
	}

	result.Distance = calculatedDistance
	result.Duration = calculatedDuration
	result.Fee = feeResult.Fee
	result.Discount = feeResult.Discount
	result.SuspendReason = feeResult.SuspendReason
	return result, nil
}
