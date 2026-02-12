package logic

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/algorithm"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/maps"
)

// DeliveryEstimateInput provides data for recomputing delivery ETA after a rider accepts.
type DeliveryEstimateInput struct {
	Delivery                db.Delivery
	Rider                   db.Rider
	Merchant                db.Merchant
	RiderSpeedMetersPerHour int
	MinTotalMinutes         float64
	MapClient               maps.TencentMapClientInterface
}

// DeliveryEstimateResult reports ETA recalculation details.
type DeliveryEstimateResult struct {
	Updated                    bool
	Skipped                    bool
	SkippedReason              string
	RiderToMerchantDistance    int
	MerchantToCustomerDistance int
	PrepareWaitMinutes         float64
	TotalMinutes               float64
	NewEstimatedDeliveryAt     time.Time
	MapError                   error
}

// UpdateDeliveryEstimatedTime recomputes ETA and persists it on the delivery.
func UpdateDeliveryEstimatedTime(ctx context.Context, store db.Store, input DeliveryEstimateInput) (DeliveryEstimateResult, error) {
	result := DeliveryEstimateResult{}

	if !input.Rider.CurrentLatitude.Valid || !input.Rider.CurrentLongitude.Valid {
		result.Skipped = true
		result.SkippedReason = "rider_location"
		return result, nil
	}

	riderLat, _ := floatFromNumeric(input.Rider.CurrentLatitude)
	riderLng, _ := floatFromNumeric(input.Rider.CurrentLongitude)
	merchantLat, _ := floatFromNumeric(input.Merchant.Latitude)
	merchantLng, _ := floatFromNumeric(input.Merchant.Longitude)
	deliveryLat, _ := floatFromNumeric(input.Delivery.DeliveryLatitude)
	deliveryLng, _ := floatFromNumeric(input.Delivery.DeliveryLongitude)

	riderToMerchantDistance := 0
	merchantToCustomerDistance := int(input.Delivery.Distance)

	var riderToMerchantMinutes float64
	var merchantToCustomerMinutes float64

	if input.MapClient != nil {
		riderLoc := maps.Location{Lat: riderLat, Lng: riderLng}
		merchantLoc := maps.Location{Lat: merchantLat, Lng: merchantLng}
		deliveryLoc := maps.Location{Lat: deliveryLat, Lng: deliveryLng}

		froms := []maps.Location{riderLoc, merchantLoc}
		tos := []maps.Location{merchantLoc, deliveryLoc}

		distanceResult, err := input.MapClient.GetDistanceMatrix(ctx, froms, tos, "bicycling")
		if err != nil {
			result.MapError = err
		} else if distanceResult != nil && len(distanceResult.Rows) >= 2 {
			if len(distanceResult.Rows[0].Elements) > 0 {
				riderToMerchantDistance = distanceResult.Rows[0].Elements[0].Distance
				riderToMerchantMinutes = float64(distanceResult.Rows[0].Elements[0].Duration) / 60.0
			}
			if len(distanceResult.Rows[1].Elements) > 0 {
				merchantToCustomerDistance = distanceResult.Rows[1].Elements[0].Distance
				merchantToCustomerMinutes = float64(distanceResult.Rows[1].Elements[0].Duration) / 60.0
			}
		}
	}

	if riderToMerchantMinutes <= 0 {
		if riderToMerchantDistance == 0 {
			dist := algorithm.HaversineDistance(
				algorithm.Location{Latitude: riderLat, Longitude: riderLng},
				algorithm.Location{Latitude: merchantLat, Longitude: merchantLng},
			)
			riderToMerchantDistance = int(float64(dist) * 1.3)
		}
		riderToMerchantMinutes = float64(riderToMerchantDistance) / float64(input.RiderSpeedMetersPerHour) * 60
	}

	if merchantToCustomerMinutes <= 0 {
		merchantToCustomerMinutes = float64(merchantToCustomerDistance) / float64(input.RiderSpeedMetersPerHour) * 60
	}

	prepareWaitMinutes := 0.0
	if input.Delivery.EstimatedPickupAt.Valid {
		waitDuration := time.Until(input.Delivery.EstimatedPickupAt.Time)
		if waitDuration > 0 {
			prepareWaitMinutes = waitDuration.Minutes()
		}
	}

	totalMinutes := riderToMerchantMinutes + prepareWaitMinutes + merchantToCustomerMinutes
	minTotal := input.MinTotalMinutes
	if minTotal <= 0 {
		minTotal = 10
	}
	if totalMinutes < minTotal {
		totalMinutes = minTotal
	}

	newEstimatedDeliveryAt := time.Now().Add(time.Duration(totalMinutes) * time.Minute)
	_, err := store.UpdateDeliveryEstimatedTime(ctx, db.UpdateDeliveryEstimatedTimeParams{
		ID:                  input.Delivery.ID,
		EstimatedDeliveryAt: pgtype.Timestamptz{Time: newEstimatedDeliveryAt, Valid: true},
	})
	if err != nil {
		return result, err
	}

	result.Updated = true
	result.RiderToMerchantDistance = riderToMerchantDistance
	result.MerchantToCustomerDistance = merchantToCustomerDistance
	result.PrepareWaitMinutes = prepareWaitMinutes
	result.TotalMinutes = totalMinutes
	result.NewEstimatedDeliveryAt = newEstimatedDeliveryAt

	return result, nil
}
