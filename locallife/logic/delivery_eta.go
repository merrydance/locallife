package logic

import (
	"context"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

// DeliveryETAResult describes ETA breakdowns (minutes).
type DeliveryETAResult struct {
	DeliveryEtaMinutes  int32
	PrepareMinutes      int32
	RiderToStoreMinutes int32
	StoreToUserMinutes  int32
	BufferMinutes       int32
}

const (
	defaultPrepareMinutes      = int32(10)
	defaultRiderToStoreMinutes = int32(10)
	defaultBufferMinutes       = int32(20)
	prepareTimeLookbackDays    = 30
)

// ComputeDeliveryETA estimates delivery ETA with prep, rider, route, and buffer time.
func ComputeDeliveryETA(ctx context.Context, store db.Store, merchantID int64, routeDistance int32, routeDurationSec int) DeliveryETAResult {
	res := DeliveryETAResult{
		PrepareMinutes:      defaultPrepareMinutes,
		RiderToStoreMinutes: defaultRiderToStoreMinutes,
		BufferMinutes:       defaultBufferMinutes,
	}

	if routeDistance <= 0 || routeDurationSec <= 0 {
		return res
	}

	if avgPrep, err := store.GetMerchantAvgPrepareTime(ctx, db.GetMerchantAvgPrepareTimeParams{
		MerchantID: merchantID,
		StartAt:    time.Now().AddDate(0, 0, -prepareTimeLookbackDays),
	}); err == nil && avgPrep > 0 {
		res.PrepareMinutes = int32(avgPrep)
	}

	res.StoreToUserMinutes = int32((routeDurationSec + 59) / 60)
	if res.StoreToUserMinutes > 0 {
		half := res.StoreToUserMinutes / 2
		if half > res.RiderToStoreMinutes {
			res.RiderToStoreMinutes = half
		}
	}

	res.DeliveryEtaMinutes = res.PrepareMinutes + res.RiderToStoreMinutes + res.StoreToUserMinutes + res.BufferMinutes
	return res
}

// ExtractDistance prefers delivery distance, falling back to order distance.
func ExtractDistance(deliveryDistance int32, orderDistance pgtype.Int4) int32 {
	if deliveryDistance > 0 {
		return deliveryDistance
	}
	if orderDistance.Valid {
		return orderDistance.Int32
	}
	return 0
}

// EstimateDurationSecByDistance returns a rough delivery duration in seconds (assumes 15km/h).
func EstimateDurationSecByDistance(distance int32) int {
	if distance <= 0 {
		return 0
	}
	return int(math.Round(float64(distance) / 250 * 60))
}
