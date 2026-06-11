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

func TestRecordMerchantAppDevicePermanentPushFailureDegradesBeforeThreshold(t *testing.T) {
	ctx := context.Background()
	merchant := createRandomMerchantForTest(t)
	user := createRandomUser(t)

	target := registerMerchantAppDeviceForTest(t, merchant.ID, user.ID, "terminal-target")
	other := registerMerchantAppDeviceForTest(t, merchant.ID, user.ID, "terminal-other")

	count, err := testStore.RecordMerchantAppDevicePermanentPushFailure(ctx, RecordMerchantAppDevicePermanentPushFailureParams{
		ID:                    target.ID,
		PushToken:             target.PushToken,
		LastPushFailureReason: pgtype.Text{String: "huawei invalid token", Valid: true},
		DeactivateAfterCount:  3,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), count)

	degraded := getMerchantAppDeviceByID(t, target.ID)
	require.Equal(t, MerchantAppDeviceStatusActive, degraded.Status)
	require.Equal(t, int32(1), degraded.PushFailureCount)
	require.Equal(t, pgtype.Text{String: "huawei invalid token", Valid: true}, degraded.LastPushFailureReason)
	require.True(t, degraded.LastPushFailureAt.Valid)
	require.True(t, degraded.PushDegradedAt.Valid)
	require.False(t, degraded.UnregisteredAt.Valid)

	unaffected := getMerchantAppDeviceByID(t, other.ID)
	require.Equal(t, MerchantAppDeviceStatusActive, unaffected.Status)
	require.Zero(t, unaffected.PushFailureCount)
	require.False(t, unaffected.LastPushFailureReason.Valid)
	require.False(t, unaffected.LastPushFailureAt.Valid)
	require.False(t, unaffected.PushDegradedAt.Valid)
}

func TestRecordMerchantAppDevicePermanentPushFailureDeactivatesOnlyTargetAtThreshold(t *testing.T) {
	ctx := context.Background()
	merchant := createRandomMerchantForTest(t)
	user := createRandomUser(t)

	target := registerMerchantAppDeviceForTest(t, merchant.ID, user.ID, "terminal-threshold-target")
	other := registerMerchantAppDeviceForTest(t, merchant.ID, user.ID, "terminal-threshold-other")

	for i := 0; i < 3; i++ {
		count, err := testStore.RecordMerchantAppDevicePermanentPushFailure(ctx, RecordMerchantAppDevicePermanentPushFailureParams{
			ID:                    target.ID,
			PushToken:             target.PushToken,
			LastPushFailureReason: pgtype.Text{String: "huawei invalid token", Valid: true},
			DeactivateAfterCount:  3,
		})
		require.NoError(t, err)
		require.Equal(t, int64(1), count)
	}

	deactivated := getMerchantAppDeviceByID(t, target.ID)
	require.Equal(t, MerchantAppDeviceStatusInactive, deactivated.Status)
	require.Equal(t, int32(3), deactivated.PushFailureCount)
	require.Equal(t, pgtype.Text{String: "huawei invalid token", Valid: true}, deactivated.LastPushFailureReason)
	require.True(t, deactivated.LastPushFailureAt.Valid)
	require.True(t, deactivated.PushDegradedAt.Valid)
	require.True(t, deactivated.UnregisteredAt.Valid)

	unaffected := getMerchantAppDeviceByID(t, other.ID)
	require.Equal(t, MerchantAppDeviceStatusActive, unaffected.Status)
	require.Zero(t, unaffected.PushFailureCount)
	require.False(t, unaffected.UnregisteredAt.Valid)
}

func TestClearMerchantAppDevicePushFailure(t *testing.T) {
	ctx := context.Background()
	merchant := createRandomMerchantForTest(t)
	user := createRandomUser(t)

	device := registerMerchantAppDeviceForTest(t, merchant.ID, user.ID, "terminal-clear")
	_, err := testStore.RecordMerchantAppDevicePermanentPushFailure(ctx, RecordMerchantAppDevicePermanentPushFailureParams{
		ID:                    device.ID,
		PushToken:             device.PushToken,
		LastPushFailureReason: pgtype.Text{String: "xiaomi invalid token", Valid: true},
		DeactivateAfterCount:  3,
	})
	require.NoError(t, err)

	count, err := testStore.ClearMerchantAppDevicePushFailure(ctx, ClearMerchantAppDevicePushFailureParams{
		ID:        device.ID,
		PushToken: device.PushToken,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), count)

	cleared := getMerchantAppDeviceByID(t, device.ID)
	require.Equal(t, MerchantAppDeviceStatusActive, cleared.Status)
	require.Zero(t, cleared.PushFailureCount)
	require.False(t, cleared.LastPushFailureReason.Valid)
	require.False(t, cleared.LastPushFailureAt.Valid)
	require.False(t, cleared.PushDegradedAt.Valid)
	require.False(t, cleared.UnregisteredAt.Valid)
}

func TestUpdateMerchantAppDeviceHeartbeatClearsPushFailure(t *testing.T) {
	ctx := context.Background()
	merchant := createRandomMerchantForTest(t)
	user := createRandomUser(t)

	device := registerMerchantAppDeviceForTest(t, merchant.ID, user.ID, "terminal-heartbeat")
	_, err := testStore.RecordMerchantAppDevicePermanentPushFailure(ctx, RecordMerchantAppDevicePermanentPushFailureParams{
		ID:                    device.ID,
		PushToken:             device.PushToken,
		LastPushFailureReason: pgtype.Text{String: "vivo invalid token", Valid: true},
		DeactivateAfterCount:  3,
	})
	require.NoError(t, err)

	heartbeat, err := testStore.UpdateMerchantAppDeviceHeartbeatTx(ctx, UpdateMerchantAppDeviceHeartbeatParams{
		MerchantID: merchant.ID,
		DeviceID:   device.DeviceID,
	})
	require.NoError(t, err)
	require.Equal(t, MerchantAppDeviceStatusActive, heartbeat.Status)
	require.Zero(t, heartbeat.PushFailureCount)
	require.False(t, heartbeat.LastPushFailureReason.Valid)
	require.False(t, heartbeat.LastPushFailureAt.Valid)
	require.False(t, heartbeat.PushDegradedAt.Valid)

	cleared := getMerchantAppDeviceByID(t, device.ID)
	require.Zero(t, cleared.PushFailureCount)
	require.False(t, cleared.LastPushFailureReason.Valid)
	require.False(t, cleared.LastPushFailureAt.Valid)
	require.False(t, cleared.PushDegradedAt.Valid)
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

func TestMerchantAppDevicePushDegradationMigrationCleanDatabaseContract(t *testing.T) {
	dbSource := createMembershipSettingsMigrationDB(t)
	runMembershipSettingsMigrations(t, dbSource, 266)

	pool := openMembershipSettingsMigrationPool(t, dbSource)
	defer pool.Close()

	requireMerchantAppDevicePushDegradationSchema(t, pool)
}

func TestMerchantAppDevicePushDegradationMigrationIncrementalContract(t *testing.T) {
	dbSource := createMembershipSettingsMigrationDB(t)
	runMembershipSettingsMigrations(t, dbSource, 265)
	runMembershipSettingsMigrations(t, dbSource, 266)

	pool := openMembershipSettingsMigrationPool(t, dbSource)
	defer pool.Close()

	requireMerchantAppDevicePushDegradationSchema(t, pool)
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
		`SELECT id, merchant_id, user_id, device_id, platform, provider, push_token, status, device_model, os_version, app_version, last_registered_at, last_active_at, unregistered_at, created_at, updated_at, push_failure_count, last_push_failure_reason, last_push_failure_at, push_degraded_at
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
		&device.PushFailureCount,
		&device.LastPushFailureReason,
		&device.LastPushFailureAt,
		&device.PushDegradedAt,
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

func requireMerchantAppDevicePushDegradationSchema(t *testing.T, pool interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}) {
	t.Helper()

	var matchingColumns int
	err := pool.QueryRow(
		context.Background(),
		`SELECT count(*)
		 FROM information_schema.columns
		 WHERE table_schema = 'public'
		   AND table_name = 'merchant_app_devices'
		   AND (
		       (column_name = 'push_failure_count'
		        AND data_type = 'integer'
		        AND is_nullable = 'NO'
		        AND column_default = '0')
		       OR (column_name = 'last_push_failure_reason'
		           AND data_type = 'character varying'
		           AND character_maximum_length = 255
		           AND is_nullable = 'YES')
		       OR (column_name = 'last_push_failure_at'
		           AND data_type = 'timestamp with time zone'
		           AND is_nullable = 'YES')
		       OR (column_name = 'push_degraded_at'
		           AND data_type = 'timestamp with time zone'
		           AND is_nullable = 'YES')
		   )`,
	).Scan(&matchingColumns)
	require.NoError(t, err)
	require.Equal(t, 4, matchingColumns)

	var constraintDef string
	err = pool.QueryRow(
		context.Background(),
		`SELECT pg_get_constraintdef(oid)
		 FROM pg_constraint
		 WHERE conrelid = 'merchant_app_devices'::regclass
		   AND conname = 'merchant_app_devices_push_failure_count_check'`,
	).Scan(&constraintDef)
	require.NoError(t, err)
	require.Contains(t, constraintDef, "push_failure_count >= 0")
}
