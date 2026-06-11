package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestMerchantOfflineCustomerMigration264BackfillsHistoricalReservationIdentity(t *testing.T) {
	dbSource := createMembershipSettingsMigrationDB(t)
	runMembershipSettingsMigrations(t, dbSource, 263)

	pool := openMembershipSettingsMigrationPool(t, dbSource)
	defer pool.Close()

	merchantID := insertMembershipSettingsMigrationMerchant(t, pool)
	operatorID := insertMerchantOfflineCustomerMigrationUser(t, pool, "operator")
	tableID := insertMerchantOfflineCustomerMigrationRoom(t, pool, merchantID)

	firstCreated := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)
	secondCreated := time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC)
	secondUpdated := time.Date(2026, 1, 6, 9, 0, 0, 0, time.UTC)

	firstReservationID := insertMerchantOfflineCustomerMigrationReservation(t, pool, tableID, operatorID, merchantID, "early customer", " 13800138000 ", "phone", firstCreated, firstCreated)
	secondReservationID := insertMerchantOfflineCustomerMigrationReservation(t, pool, tableID, operatorID, merchantID, "later customer", "13800138000", "phone", secondCreated, secondUpdated)

	runMembershipSettingsMigrations(t, dbSource, 264)

	var offlineCustomerID int64
	var contactName string
	var firstSeenAt time.Time
	var lastSeenAt time.Time
	err := pool.QueryRow(context.Background(), `
		SELECT id, contact_name, first_seen_at, last_seen_at
		FROM merchant_offline_customers
		WHERE merchant_id = $1
		  AND contact_phone = '13800138000'
	`, merchantID).Scan(&offlineCustomerID, &contactName, &firstSeenAt, &lastSeenAt)
	require.NoError(t, err)
	require.NotZero(t, offlineCustomerID)
	require.Equal(t, "later customer", contactName)
	require.WithinDuration(t, firstCreated, firstSeenAt, time.Second)
	require.WithinDuration(t, secondUpdated, lastSeenAt, time.Second)

	var offlineCustomerCount int64
	err = pool.QueryRow(context.Background(), `
		SELECT COUNT(*)
		FROM merchant_offline_customers
		WHERE merchant_id = $1
		  AND contact_phone = '13800138000'
	`, merchantID).Scan(&offlineCustomerCount)
	require.NoError(t, err)
	require.Equal(t, int64(1), offlineCustomerCount)

	var offlineCustomerReservationIndexExists bool
	err = pool.QueryRow(context.Background(), `
		SELECT EXISTS (
			SELECT 1
			FROM pg_indexes
			WHERE schemaname = current_schema()
			  AND tablename = 'table_reservations'
			  AND indexname = 'table_reservations_offline_customer_id_idx'
		)
	`).Scan(&offlineCustomerReservationIndexExists)
	require.NoError(t, err)
	require.True(t, offlineCustomerReservationIndexExists)

	rows, err := pool.Query(context.Background(), `
		SELECT offline_customer_id, created_by_user_id, contact_phone
		FROM table_reservations
		WHERE id = ANY($1::bigint[])
		ORDER BY id
	`, []int64{firstReservationID, secondReservationID})
	require.NoError(t, err)
	defer rows.Close()

	var linkedCount int
	for rows.Next() {
		var linkedOfflineCustomerID int64
		var createdByUserID int64
		var reservationContactPhone string
		require.NoError(t, rows.Scan(&linkedOfflineCustomerID, &createdByUserID, &reservationContactPhone))
		require.Equal(t, offlineCustomerID, linkedOfflineCustomerID)
		require.Equal(t, operatorID, createdByUserID)
		require.Equal(t, "13800138000", reservationContactPhone)
		linkedCount++
	}
	require.NoError(t, rows.Err())
	require.Equal(t, 2, linkedCount)
}

func TestMerchantOfflineCustomerMigration264CleansBlankHistoricalContactName(t *testing.T) {
	dbSource := createMembershipSettingsMigrationDB(t)
	runMembershipSettingsMigrations(t, dbSource, 263)

	pool := openMembershipSettingsMigrationPool(t, dbSource)
	defer pool.Close()

	merchantID := insertMembershipSettingsMigrationMerchant(t, pool)
	operatorID := insertMerchantOfflineCustomerMigrationUser(t, pool, "blank-name")
	tableID := insertMerchantOfflineCustomerMigrationRoom(t, pool, merchantID)
	reservationCreated := time.Date(2026, 1, 7, 10, 0, 0, 0, time.UTC)

	reservationID := insertMerchantOfflineCustomerMigrationReservation(t, pool, tableID, operatorID, merchantID, "   ", " 13900139000 ", "phone", reservationCreated, reservationCreated)

	runMembershipSettingsMigrations(t, dbSource, 264)

	var offlineCustomerID int64
	var offlineContactName string
	err := pool.QueryRow(context.Background(), `
		SELECT id, contact_name
		FROM merchant_offline_customers
		WHERE merchant_id = $1
		  AND contact_phone = '13900139000'
	`, merchantID).Scan(&offlineCustomerID, &offlineContactName)
	require.NoError(t, err)
	require.NotZero(t, offlineCustomerID)
	require.Equal(t, "offline customer", offlineContactName)

	var reservationOfflineCustomerID int64
	var reservationContactName string
	var reservationContactPhone string
	err = pool.QueryRow(context.Background(), `
		SELECT offline_customer_id, contact_name, contact_phone
		FROM table_reservations
		WHERE id = $1
	`, reservationID).Scan(&reservationOfflineCustomerID, &reservationContactName, &reservationContactPhone)
	require.NoError(t, err)
	require.Equal(t, offlineCustomerID, reservationOfflineCustomerID)
	require.Equal(t, "offline customer", reservationContactName)
	require.Equal(t, "13900139000", reservationContactPhone)
}

func TestMerchantOfflineCustomerMigration264IgnoresHistoricalOnlineSourceWhitespace(t *testing.T) {
	dbSource := createMembershipSettingsMigrationDB(t)
	runMembershipSettingsMigrations(t, dbSource, 263)

	pool := openMembershipSettingsMigrationPool(t, dbSource)
	defer pool.Close()

	merchantID := insertMembershipSettingsMigrationMerchant(t, pool)
	operatorID := insertMerchantOfflineCustomerMigrationUser(t, pool, "online-source")
	tableID := insertMerchantOfflineCustomerMigrationRoom(t, pool, merchantID)
	reservationCreated := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)

	blankSourceReservationID := insertMerchantOfflineCustomerMigrationReservation(t, pool, tableID, operatorID, merchantID, "online customer", " 13900139001 ", "   ", reservationCreated, reservationCreated)
	onlineSourceReservationID := insertMerchantOfflineCustomerMigrationReservation(t, pool, tableID, operatorID, merchantID, "online customer", " 13900139002 ", " online ", reservationCreated, reservationCreated)

	runMembershipSettingsMigrations(t, dbSource, 264)

	var offlineCustomerCount int64
	err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*)
		FROM merchant_offline_customers
		WHERE merchant_id = $1
		  AND contact_phone IN ('13900139001', '13900139002')
	`, merchantID).Scan(&offlineCustomerCount)
	require.NoError(t, err)
	require.Zero(t, offlineCustomerCount)

	var linkedCount int64
	err = pool.QueryRow(context.Background(), `
		SELECT COUNT(*)
		FROM table_reservations
		WHERE id = ANY($1::bigint[])
		  AND offline_customer_id IS NOT NULL
	`, []int64{blankSourceReservationID, onlineSourceReservationID}).Scan(&linkedCount)
	require.NoError(t, err)
	require.Zero(t, linkedCount)
}

func insertMerchantOfflineCustomerMigrationUser(t *testing.T, pool *pgxpool.Pool, suffix string) int64 {
	t.Helper()

	var userID int64
	err := pool.QueryRow(context.Background(), `
		INSERT INTO users (wechat_openid, full_name)
		VALUES ($1, $2)
		RETURNING id
	`, "offline-customer-migration-"+suffix, "offline customer migration "+suffix).Scan(&userID)
	require.NoError(t, err)
	return userID
}

func insertMerchantOfflineCustomerMigrationRoom(t *testing.T, pool *pgxpool.Pool, merchantID int64) int64 {
	t.Helper()

	var tableID int64
	err := pool.QueryRow(context.Background(), `
		INSERT INTO tables (
			merchant_id,
			table_no,
			table_type,
			capacity,
			status
		) VALUES ($1, 'MIG-264', 'room', 8, 'available')
		RETURNING id
	`, merchantID).Scan(&tableID)
	require.NoError(t, err)
	return tableID
}

func insertMerchantOfflineCustomerMigrationReservation(
	t *testing.T,
	pool *pgxpool.Pool,
	tableID int64,
	operatorID int64,
	merchantID int64,
	contactName string,
	contactPhone string,
	source string,
	createdAt time.Time,
	updatedAt time.Time,
) int64 {
	t.Helper()

	var reservationID int64
	err := pool.QueryRow(context.Background(), `
		INSERT INTO table_reservations (
			table_id,
			user_id,
			merchant_id,
			reservation_date,
			reservation_time,
			guest_count,
			contact_name,
			contact_phone,
			refund_deadline,
			payment_deadline,
			status,
			source,
			created_at,
			updated_at
		) VALUES (
			$1, $2, $3, $4, '18:30', 4, $5, $6, $7, $8, 'confirmed', $9, $10, $11
		)
		RETURNING id
	`, tableID, operatorID, merchantID, createdAt, contactName, contactPhone, createdAt.Add(-time.Hour), createdAt.Add(time.Hour), source, createdAt, updatedAt).Scan(&reservationID)
	require.NoError(t, err)
	return reservationID
}
