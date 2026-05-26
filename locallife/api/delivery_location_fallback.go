package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/rs/zerolog/log"
)

func latestLocationResponseWithRiderCurrent(ctx *gin.Context, store db.Store, delivery db.Delivery, location db.RiderLocation) (locationResponse, bool) {
	resp := newLocationResponse(location)
	if !canExposeAssignedRiderCurrentLocation(delivery.Status) {
		return resp, true
	}

	riderResp, found, handled := loadAssignedRiderCurrentLocation(ctx, store, delivery, false)
	if handled {
		log.Warn().Int64("delivery_id", delivery.ID).Int64("rider_id", delivery.RiderID.Int64).Msg("load assigned rider current location failed, using delivery-bound location")
		return resp, true
	}
	if found && riderResp.RecordedAt.After(resp.RecordedAt) {
		return riderResp, true
	}
	return resp, true
}

func writeAssignedRiderCurrentLocation(ctx *gin.Context, store db.Store, delivery db.Delivery) bool {
	resp, found, handled := loadAssignedRiderCurrentLocation(ctx, store, delivery, true)
	if handled {
		return true
	}
	if !found {
		return false
	}

	ctx.JSON(http.StatusOK, resp)
	return true
}

func loadAssignedRiderCurrentLocation(ctx *gin.Context, store db.Store, delivery db.Delivery, writeUnexpectedError bool) (locationResponse, bool, bool) {
	if !delivery.RiderID.Valid {
		return locationResponse{}, false, false
	}
	if !canExposeAssignedRiderCurrentLocation(delivery.Status) {
		return locationResponse{}, false, false
	}

	rider, err := store.GetRider(ctx, delivery.RiderID.Int64)
	if err != nil {
		if isNotFoundError(err) {
			return locationResponse{}, false, false
		}
		if writeUnexpectedError {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return locationResponse{}, false, true
		}
		return locationResponse{}, false, true
	}
	if !rider.CurrentLongitude.Valid || !rider.CurrentLatitude.Valid || !rider.LocationUpdatedAt.Valid {
		return locationResponse{}, false, false
	}

	lng, lngErr := rider.CurrentLongitude.Float64Value()
	lat, latErr := rider.CurrentLatitude.Float64Value()
	if lngErr != nil || latErr != nil {
		return locationResponse{}, false, false
	}

	return locationResponse{
		Longitude:  lng.Float64,
		Latitude:   lat.Float64,
		RecordedAt: rider.LocationUpdatedAt.Time,
	}, true, false
}

func canExposeAssignedRiderCurrentLocation(status string) bool {
	switch status {
	case db.DeliveryStatusAssigned, db.DeliveryStatusPicking, db.DeliveryStatusPicked, db.DeliveryStatusDelivering:
		return true
	default:
		return false
	}
}

func newLocationResponse(location db.RiderLocation) locationResponse {
	lng, _ := location.Longitude.Float64Value()
	lat, _ := location.Latitude.Float64Value()

	resp := locationResponse{
		Longitude:  lng.Float64,
		Latitude:   lat.Float64,
		RecordedAt: location.RecordedAt,
	}

	resp.Accuracy = numericFloatPtr(location.Accuracy)
	resp.Speed = numericFloatPtr(location.Speed)
	resp.Heading = numericFloatPtr(location.Heading)

	return resp
}

func numericFloatPtr(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	v, err := n.Float64Value()
	if err != nil {
		return nil
	}
	return &v.Float64
}
