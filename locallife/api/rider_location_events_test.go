package api

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/algorithm"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestHasGeofenceDwell_EnoughSamples(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.config.GeofenceRadiusMeters = 80
	server.config.GeofenceDwellMinSeconds = 60
	server.config.GeofenceDwellMinSamples = 3
	server.config.GeofenceMinAccuracyMeters = 80

	now := time.Now()
	deliveryID := int64(123)
	target := algorithm.Location{Longitude: 116.404, Latitude: 39.915}

	store.EXPECT().
		ListDeliveryLocationsSince(gomock.Any(), gomock.Any()).
		Times(1).
		Return([]db.RiderLocation{
			{
				Longitude:  numericFromFloat(116.404),
				Latitude:   numericFromFloat(39.915),
				Accuracy:   numericFromFloat(10),
				RecordedAt: now.Add(-60 * time.Second),
			},
			{
				Longitude:  numericFromFloat(116.404),
				Latitude:   numericFromFloat(39.915),
				Accuracy:   numericFromFloat(10),
				RecordedAt: now.Add(-30 * time.Second),
			},
			{
				Longitude:  numericFromFloat(116.404),
				Latitude:   numericFromFloat(39.915),
				Accuracy:   numericFromFloat(10),
				RecordedAt: now,
			},
		}, nil)

	ok := server.hasGeofenceDwell(t.Context(), deliveryID, target, now)
	require.True(t, ok)
}

func TestHasGeofenceDwell_InsufficientSamples(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.config.GeofenceRadiusMeters = 80
	server.config.GeofenceDwellMinSeconds = 60
	server.config.GeofenceDwellMinSamples = 3
	server.config.GeofenceMinAccuracyMeters = 80

	now := time.Now()
	deliveryID := int64(456)
	target := algorithm.Location{Longitude: 116.404, Latitude: 39.915}

	store.EXPECT().
		ListDeliveryLocationsSince(gomock.Any(), gomock.Any()).
		Times(1).
		Return([]db.RiderLocation{
			{
				Longitude:  numericFromFloat(116.404),
				Latitude:   numericFromFloat(39.915),
				Accuracy:   numericFromFloat(10),
				RecordedAt: now.Add(-60 * time.Second),
			},
			{
				Longitude:  numericFromFloat(116.404),
				Latitude:   numericFromFloat(39.915),
				Accuracy:   numericFromFloat(10),
				RecordedAt: now.Add(-30 * time.Second),
			},
		}, nil)

	ok := server.hasGeofenceDwell(t.Context(), deliveryID, target, now)
	require.False(t, ok)
}

func TestHasGeofenceDwell_SkipLowAccuracy(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.config.GeofenceRadiusMeters = 80
	server.config.GeofenceDwellMinSeconds = 60
	server.config.GeofenceDwellMinSamples = 3
	server.config.GeofenceMinAccuracyMeters = 80

	now := time.Now()
	deliveryID := int64(789)
	target := algorithm.Location{Longitude: 116.404, Latitude: 39.915}

	store.EXPECT().
		ListDeliveryLocationsSince(gomock.Any(), gomock.Any()).
		Times(1).
		Return([]db.RiderLocation{
			{
				Longitude:  numericFromFloat(116.404),
				Latitude:   numericFromFloat(39.915),
				Accuracy:   numericFromFloat(200),
				RecordedAt: now.Add(-60 * time.Second),
			},
			{
				Longitude:  numericFromFloat(116.404),
				Latitude:   numericFromFloat(39.915),
				Accuracy:   numericFromFloat(200),
				RecordedAt: now.Add(-30 * time.Second),
			},
			{
				Longitude:  numericFromFloat(116.404),
				Latitude:   numericFromFloat(39.915),
				Accuracy:   numericFromFloat(200),
				RecordedAt: now,
			},
		}, nil)

	ok := server.hasGeofenceDwell(t.Context(), deliveryID, target, now)
	require.False(t, ok)
}

func TestGeofenceAccuracyGate(t *testing.T) {
	minAccuracy := 80
	bad := 120.0
	good := 50.0

	require.True(t, shouldSkipByAccuracy(&bad, minAccuracy))
	require.False(t, shouldSkipByAccuracy(&good, minAccuracy))
	require.False(t, shouldSkipByAccuracy(nil, minAccuracy))
}

func TestGeofenceWithinRadius(t *testing.T) {
	loc := locationPoint{Longitude: 116.404, Latitude: 39.915}
	target := algorithm.Location{Longitude: 116.404, Latitude: 39.915}

	require.True(t, isWithinGeofence(loc, target, 0))
	require.True(t, isWithinGeofence(loc, target, 80))
}

func TestGeofenceTargetForDelivery(t *testing.T) {
	pickupDelivery := db.Delivery{
		PickupLongitude:   numericFromFloat(116.404),
		PickupLatitude:    numericFromFloat(39.915),
		Status:            "assigned",
		DeliveryLongitude: numericFromFloat(116.410),
		DeliveryLatitude:  numericFromFloat(39.920),
	}

	target, arrive, dwell, ok := geofenceTargetForDelivery(pickupDelivery)
	require.True(t, ok)
	require.Equal(t, geofenceEventArrivePickup, arrive)
	require.Equal(t, geofenceEventDwellPickup, dwell)
	require.Equal(t, 116.404, target.Longitude)
	require.Equal(t, 39.915, target.Latitude)

	dropoffDelivery := db.Delivery{
		PickupLongitude:   numericFromFloat(116.404),
		PickupLatitude:    numericFromFloat(39.915),
		DeliveryLongitude: numericFromFloat(116.410),
		DeliveryLatitude:  numericFromFloat(39.920),
		Status:            "delivering",
		RiderID:           pgtype.Int8{Int64: 1, Valid: true},
	}

	target, arrive, dwell, ok = geofenceTargetForDelivery(dropoffDelivery)
	require.True(t, ok)
	require.Equal(t, geofenceEventArriveDropoff, arrive)
	require.Equal(t, geofenceEventDwellDropoff, dwell)
	require.Equal(t, 116.410, target.Longitude)
	require.Equal(t, 39.920, target.Latitude)
}
