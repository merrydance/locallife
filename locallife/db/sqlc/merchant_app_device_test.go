package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestDeactivateStaleMerchantAppDevices(t *testing.T) {
	ctx := context.Background()
	merchant := createRandomMerchantForTest(t)
	user := createRandomUser(t)

	staleDevice := registerMerchantAppDeviceForTest(t, merchant.ID, user.ID, "stale")
	recentDevice := registerMerchantAppDeviceForTest(t, merchant.ID, user.ID, "recent")
	inactiveDevice := registerMerchantAppDeviceForTest(t, merchant.ID, user.ID, "inactive")

	cutoff := time.Now().Add(-90 * 24 * time.Hour)
	setMerchantAppDeviceLastActiveAt(t, staleDevice.ID, cutoff.Add(-time.Hour))
	setMerchantAppDeviceLastActiveAt(t, recentDevice.ID, cutoff.Add(time.Hour))

	_, err := testStore.UnregisterMerchantAppDevice(ctx, UnregisterMerchantAppDeviceParams{
		MerchantID: merchant.ID,
		DeviceID:   inactiveDevice.DeviceID,
	})
	require.NoError(t, err)
	setMerchantAppDeviceLastActiveAt(t, inactiveDevice.ID, cutoff.Add(-2*time.Hour))

	count, err := testStore.DeactivateStaleMerchantAppDevices(ctx, cutoff)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)

	stale := getMerchantAppDeviceByID(t, staleDevice.ID)
	require.Equal(t, MerchantAppDeviceStatusInactive, stale.Status)
	require.True(t, stale.UnregisteredAt.Valid)

	recent := getMerchantAppDeviceByID(t, recentDevice.ID)
	require.Equal(t, MerchantAppDeviceStatusActive, recent.Status)
	require.False(t, recent.UnregisteredAt.Valid)

	inactive := getMerchantAppDeviceByID(t, inactiveDevice.ID)
	require.Equal(t, MerchantAppDeviceStatusInactive, inactive.Status)
}

func TestMerchantAppDeviceStaleCleanupIndexExists(t *testing.T) {
	indexdef := queryMerchantAppDeviceStaleCleanupIndexDef(t, testStore.(*SQLStore).connPool)
	require.Contains(t, indexdef, "ON public.merchant_app_devices USING btree (last_active_at)")
	require.Contains(t, indexdef, "WHERE")
	require.Contains(t, indexdef, "status")
	require.Contains(t, indexdef, "active")
}

func TestMerchantAppDeviceStaleCleanupIndexMigrationCleanDatabaseContract(t *testing.T) {
	dbSource := createMembershipSettingsMigrationDB(t)
	runMembershipSettingsMigrations(t, dbSource, 265)

	pool := openMembershipSettingsMigrationPool(t, dbSource)
	defer pool.Close()

	indexdef := queryMerchantAppDeviceStaleCleanupIndexDef(t, pool)
	require.Contains(t, indexdef, "ON public.merchant_app_devices USING btree (last_active_at)")
	require.Contains(t, indexdef, "WHERE")
	require.Contains(t, indexdef, "status")
	require.Contains(t, indexdef, "active")
}

func TestMerchantAppDeviceStaleCleanupIndexMigrationIncrementalContract(t *testing.T) {
	dbSource := createMembershipSettingsMigrationDB(t)
	runMembershipSettingsMigrations(t, dbSource, 264)
	runMembershipSettingsMigrations(t, dbSource, 265)

	pool := openMembershipSettingsMigrationPool(t, dbSource)
	defer pool.Close()

	indexdef := queryMerchantAppDeviceStaleCleanupIndexDef(t, pool)
	require.Contains(t, indexdef, "ON public.merchant_app_devices USING btree (last_active_at)")
	require.Contains(t, indexdef, "WHERE")
	require.Contains(t, indexdef, "status")
	require.Contains(t, indexdef, "active")
}

func registerMerchantAppDeviceForTest(t *testing.T, merchantID, userID int64, suffix string) MerchantAppDevice {
	t.Helper()

	device, err := testStore.RegisterMerchantAppDeviceTx(context.Background(), RegisterMerchantAppDeviceParams{
		MerchantID:  merchantID,
		UserID:      userID,
		DeviceID:    "device-" + suffix + "-" + util.RandomString(8),
		Platform:    MerchantAppDevicePlatformAndroid,
		Provider:    MerchantAppDeviceProviderXiaomi,
		PushToken:   "push-token-" + suffix + "-" + util.RandomString(12),
		DeviceModel: pgtype.Text{String: "Pixel 8", Valid: true},
		OsVersion:   pgtype.Text{String: "Android 15", Valid: true},
		AppVersion:  pgtype.Text{String: "1.0.0", Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, MerchantAppDeviceStatusActive, device.Status)
	return device
}

func setMerchantAppDeviceLastActiveAt(t *testing.T, deviceID int64, lastActiveAt time.Time) {
	t.Helper()

	_, err := testStore.(*SQLStore).connPool.Exec(
		context.Background(),
		`UPDATE merchant_app_devices SET last_active_at = $2, updated_at = now() WHERE id = $1`,
		deviceID,
		lastActiveAt,
	)
	require.NoError(t, err)
}

func getMerchantAppDeviceByID(t *testing.T, id int64) MerchantAppDevice {
	t.Helper()

	var device MerchantAppDevice
	err := testStore.(*SQLStore).connPool.QueryRow(
		context.Background(),
		`SELECT id, merchant_id, user_id, device_id, platform, provider, push_token, status, device_model, os_version, app_version, last_registered_at, last_active_at, unregistered_at, created_at, updated_at
		 FROM merchant_app_devices
		 WHERE id = $1`,
		id,
	).Scan(
		&device.ID,
		&device.MerchantID,
		&device.UserID,
		&device.DeviceID,
		&device.Platform,
		&device.Provider,
		&device.PushToken,
		&device.Status,
		&device.DeviceModel,
		&device.OsVersion,
		&device.AppVersion,
		&device.LastRegisteredAt,
		&device.LastActiveAt,
		&device.UnregisteredAt,
		&device.CreatedAt,
		&device.UpdatedAt,
	)
	require.NoError(t, err)
	return device
}

func queryMerchantAppDeviceStaleCleanupIndexDef(t *testing.T, pool interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}) string {
	t.Helper()

	var indexdef string
	err := pool.QueryRow(
		context.Background(),
		`SELECT indexdef
		 FROM pg_indexes
		 WHERE schemaname = 'public'
		   AND tablename = 'merchant_app_devices'
		   AND indexname = 'idx_merchant_app_devices_active_last_active_at'`,
	).Scan(&indexdef)
	require.NoError(t, err)
	return indexdef
}
