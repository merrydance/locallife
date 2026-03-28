package api

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/algorithm"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/rs/zerolog/log"
)

const (
	geofenceEventArrivePickup  = "arrive_pickup"
	geofenceEventDwellPickup   = "dwell_pickup"
	geofenceEventArriveDropoff = "arrive_dropoff"
	geofenceEventDwellDropoff  = "dwell_dropoff"
)

func (server *Server) processDeliveryLocationEvents(ctx context.Context, rider db.Rider, deliveryID int64, latest locationPoint) {
	if server.config.GeofenceRadiusMeters <= 0 {
		return
	}

	delivery, err := server.store.GetDelivery(ctx, deliveryID)
	if err != nil {
		log.Warn().Err(err).Int64("delivery_id", deliveryID).Msg("failed to load delivery for geofence")
		return
	}

	if delivery.RiderID.Valid && delivery.RiderID.Int64 != rider.ID {
		log.Warn().Int64("delivery_id", deliveryID).Int64("rider_id", rider.ID).Msg("geofence delivery rider mismatch")
		return
	}

	target, arriveEvent, dwellEvent, ok := geofenceTargetForDelivery(delivery)
	if !ok {
		return
	}

	if shouldSkipByAccuracy(latest.Accuracy, server.config.GeofenceMinAccuracyMeters) {
		return
	}

	if !isWithinGeofence(latest, target, server.config.GeofenceRadiusMeters) {
		return
	}

	source := normalizeEventSource(latest.Source)
	_, err = server.createDeliveryLocationEvent(ctx, delivery, rider, latest, arriveEvent, source)
	if err != nil {
		log.Warn().Err(err).Int64("delivery_id", deliveryID).Str("event", arriveEvent).Msg("failed to create geofence arrive event")
	}

	if !server.hasGeofenceDwell(ctx, delivery.ID, target, latest.RecordedAt) {
		return
	}

	created, err := server.createDeliveryLocationEvent(ctx, delivery, rider, latest, dwellEvent, source)
	if err != nil {
		log.Warn().Err(err).Int64("delivery_id", deliveryID).Str("event", dwellEvent).Msg("failed to create geofence dwell event")
		return
	}

	if created && dwellEvent == geofenceEventDwellPickup {
		server.maybeAutoAdvancePickup(ctx, delivery, rider)
		server.maybeAutoConfirmPickup(ctx, delivery, rider)
	}

	if created && dwellEvent == geofenceEventDwellDropoff {
		server.maybeAutoConfirmDelivery(ctx, delivery, rider)
	}
}

func geofenceTargetForDelivery(delivery db.Delivery) (algorithm.Location, string, string, bool) {
	if delivery.Status == "assigned" || delivery.Status == "picking" {
		pickup, ok := locationFromDeliveryPickup(delivery)
		return pickup, geofenceEventArrivePickup, geofenceEventDwellPickup, ok
	}
	if delivery.Status == "picked" || delivery.Status == "delivering" {
		dropoff, ok := locationFromDeliveryDropoff(delivery)
		return dropoff, geofenceEventArriveDropoff, geofenceEventDwellDropoff, ok
	}
	return algorithm.Location{}, "", "", false
}

func locationFromDeliveryPickup(delivery db.Delivery) (algorithm.Location, bool) {
	lng, ok := floatFromNumeric(delivery.PickupLongitude)
	if !ok {
		return algorithm.Location{}, false
	}
	lat, ok := floatFromNumeric(delivery.PickupLatitude)
	if !ok {
		return algorithm.Location{}, false
	}
	return algorithm.Location{Longitude: lng, Latitude: lat}, true
}

func locationFromDeliveryDropoff(delivery db.Delivery) (algorithm.Location, bool) {
	lng, ok := floatFromNumeric(delivery.DeliveryLongitude)
	if !ok {
		return algorithm.Location{}, false
	}
	lat, ok := floatFromNumeric(delivery.DeliveryLatitude)
	if !ok {
		return algorithm.Location{}, false
	}
	return algorithm.Location{Longitude: lng, Latitude: lat}, true
}

func isWithinGeofence(loc locationPoint, target algorithm.Location, radiusMeters int) bool {
	distance := algorithm.HaversineDistance(algorithm.Location{Longitude: loc.Longitude, Latitude: loc.Latitude}, target)
	return distance <= radiusMeters
}

func shouldSkipByAccuracy(accuracy *float64, minAccuracyMeters int) bool {
	if minAccuracyMeters <= 0 || accuracy == nil {
		return false
	}
	return *accuracy > float64(minAccuracyMeters)
}

func normalizeEventSource(source string) string {
	if source == "" {
		return "gps"
	}
	return source
}

func (server *Server) createDeliveryLocationEvent(ctx context.Context, delivery db.Delivery, rider db.Rider, loc locationPoint, eventType string, source string) (bool, error) {
	params := db.CreateDeliveryLocationEventParams{
		DeliveryID: delivery.ID,
		OrderID:    delivery.OrderID,
		RiderID:    rider.ID,
		Longitude:  numericFromFloat(loc.Longitude),
		Latitude:   numericFromFloat(loc.Latitude),
		Accuracy:   optionalNumericFromFloat(loc.Accuracy),
		Speed:      optionalNumericFromFloat(loc.Speed),
		EventType:  eventType,
		Source:     source,
		RecordedAt: loc.RecordedAt,
	}

	_, err := server.store.CreateDeliveryLocationEvent(ctx, params)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (server *Server) hasGeofenceDwell(ctx context.Context, deliveryID int64, target algorithm.Location, latest time.Time) bool {
	if server.config.GeofenceDwellMinSeconds <= 0 || server.config.GeofenceDwellMinSamples <= 0 {
		return false
	}

	windowStart := latest.Add(-time.Duration(server.config.GeofenceDwellMinSeconds) * time.Second)
	locations, err := server.store.ListDeliveryLocationsSince(ctx, db.ListDeliveryLocationsSinceParams{
		DeliveryID: pgtype.Int8{Int64: deliveryID, Valid: true},
		RecordedAt: windowStart,
	})
	if err != nil {
		log.Warn().Err(err).Int64("delivery_id", deliveryID).Msg("failed to list delivery locations for dwell")
		return false
	}

	minAccuracy := server.config.GeofenceMinAccuracyMeters
	var earliest time.Time
	var latestInFence time.Time
	count := 0

	for _, point := range locations {
		lng, ok := floatFromNumeric(point.Longitude)
		if !ok {
			continue
		}
		lat, ok := floatFromNumeric(point.Latitude)
		if !ok {
			continue
		}
		if minAccuracy > 0 && point.Accuracy.Valid {
			acc, ok := floatFromNumeric(point.Accuracy)
			if ok && acc > float64(minAccuracy) {
				continue
			}
		}

		distance := algorithm.HaversineDistance(algorithm.Location{Longitude: lng, Latitude: lat}, target)
		if distance > server.config.GeofenceRadiusMeters {
			continue
		}

		if count == 0 {
			earliest = point.RecordedAt
		}
		latestInFence = point.RecordedAt
		count++
	}

	if count < server.config.GeofenceDwellMinSamples {
		return false
	}

	minDuration := time.Duration(server.config.GeofenceDwellMinSeconds) * time.Second
	return latestInFence.Sub(earliest) >= minDuration
}

func (server *Server) maybeAutoAdvancePickup(ctx context.Context, delivery db.Delivery, rider db.Rider) {
	if !server.config.GeofenceAutoAdvanceEnabled {
		return
	}
	if delivery.Status != "assigned" {
		return
	}

	result, err := logic.AutoAdvancePickup(ctx, server.store, delivery, rider)
	if err != nil {
		log.Warn().Err(err).Int64("delivery_id", delivery.ID).Msg("failed to auto advance delivery to picking")
		return
	}
	if result.LoadOrderErr != nil {
		log.Warn().Err(result.LoadOrderErr).Int64("order_id", delivery.OrderID).Msg("failed to load order for geofence auto advance")
	}
	if !result.Updated {
		return
	}

	updated := result.Delivery
	order := result.Order
	if !result.OrderLoaded {
		loadedOrder, err := server.store.GetOrder(ctx, updated.OrderID)
		if err != nil {
			return
		}
		order = loadedOrder
	}

	server.sendDeliveryStatusNotification(
		ctx,
		order.UserID,
		updated.OrderID,
		updated.ID,
		"picking",
		"骑手正在取餐",
		"骑手已到店并开始取餐",
	)
}

func (server *Server) maybeAutoConfirmPickup(ctx context.Context, delivery db.Delivery, rider db.Rider) {
	if !server.config.GeofenceAutoPickupEnabled {
		return
	}
	if delivery.Status != "picking" {
		return
	}

	result, err := logic.AutoConfirmPickup(ctx, server.store, delivery, rider)
	if err != nil {
		log.Warn().Err(err).Int64("delivery_id", delivery.ID).Msg("failed to auto confirm pickup")
		return
	}
	if !result.Updated || !result.OrderLoaded {
		return
	}

	server.sendDeliveryStatusNotification(
		ctx,
		result.Order.UserID,
		result.Delivery.OrderID,
		result.Delivery.ID,
		"picked",
		"骑手已取餐",
		"骑手已到店完成取餐",
	)
}

func (server *Server) maybeAutoConfirmDelivery(ctx context.Context, delivery db.Delivery, rider db.Rider) {
	if !server.config.GeofenceAutoDeliverEnabled {
		return
	}
	if delivery.Status != "delivering" {
		return
	}

	result, err := logic.AutoConfirmDelivery(ctx, server.store, delivery, rider, DeliveryConfirmRadiusMeters, DeliveryConfirmLocationMaxAgeSec, 5000)
	if err != nil {
		var confirmErr *logic.DeliveryConfirmValidationError
		if errors.As(err, &confirmErr) {
			log.Warn().
				Err(err).
				Int64("delivery_id", delivery.ID).
				Int64("rider_id", rider.ID).
				Str("reason", confirmErr.Reason).
				Int("distance_m", confirmErr.DistanceMeters).
				Int("radius_m", confirmErr.RadiusMeters).
				Int("location_age_s", confirmErr.LocationAgeSec).
				Int("location_max_age_s", confirmErr.MaxAgeSec).
				Msg("failed geofence delivery confirm validation")
			return
		}
		log.Warn().Err(err).Int64("delivery_id", delivery.ID).Msg("failed to auto confirm delivery")
		return
	}
	if !result.Updated || !result.OrderLoaded {
		return
	}

	server.sendDeliveryStatusNotification(
		ctx,
		result.Order.UserID,
		result.Delivery.OrderID,
		result.Delivery.ID,
		"delivered",
		"订单已送达",
		"骑手已送达，请确认收餐",
	)
}

func optionalNumericFromFloat(value *float64) pgtype.Numeric {
	if value == nil {
		return pgtype.Numeric{}
	}
	return numericFromFloat(*value)
}

func floatFromNumeric(value pgtype.Numeric) (float64, bool) {
	if !value.Valid {
		return 0, false
	}
	floatValue, err := value.Float64Value()
	if err != nil || !floatValue.Valid {
		return 0, false
	}
	return floatValue.Float64, true
}
