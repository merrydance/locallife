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

func TestOpenDiningSessionTxCreatesBillingGroup(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	table := createRandomTable(t, merchant.ID)
	user := createRandomUser(t)

	result, err := testStore.OpenDiningSessionTx(context.Background(), OpenDiningSessionTxParams{
		TableID:                table.ID,
		MerchantID:             merchant.ID,
		UserID:                 user.ID,
		ReservationID:          pgtype.Int8{Valid: false},
		ImportReservationItems: false,
	})
	require.NoError(t, err)
	require.NotZero(t, result.Session.ID)
	require.NotZero(t, result.BillingGroup.ID)
	require.True(t, result.BillingGroup.IsDefault)
	require.Equal(t, result.Session.ID, result.BillingGroup.DiningSessionID)

	defaultGroup, err := testStore.GetDefaultBillingGroupBySession(context.Background(), result.Session.ID)
	require.NoError(t, err)
	require.Equal(t, result.BillingGroup.ID, defaultGroup.ID)

	member, err := testStore.GetActiveBillingGroupMember(context.Background(), GetActiveBillingGroupMemberParams{
		BillingGroupID: result.BillingGroup.ID,
		UserID:         user.ID,
	})
	require.NoError(t, err)
	require.Equal(t, "owner", member.Role)
}

func TestOpenDiningSessionTxSkipsDefaultBillingGroupMember(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	table := createRandomTable(t, merchant.ID)
	user := createRandomUser(t)

	result, err := testStore.OpenDiningSessionTx(context.Background(), OpenDiningSessionTxParams{
		TableID:                       table.ID,
		MerchantID:                    merchant.ID,
		UserID:                        user.ID,
		ReservationID:                 pgtype.Int8{Valid: false},
		ImportReservationItems:        false,
		SkipDefaultBillingGroupMember: true,
	})
	require.NoError(t, err)
	require.NotZero(t, result.Session.ID)
	require.NotZero(t, result.BillingGroup.ID)
	require.True(t, result.BillingGroup.IsDefault)

	_, err = testStore.GetActiveBillingGroupMember(context.Background(), GetActiveBillingGroupMemberParams{
		BillingGroupID: result.BillingGroup.ID,
		UserID:         user.ID,
	})
	require.ErrorIs(t, err, ErrRecordNotFound)
}

func TestBillingGroupMemberUnique(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	table := createRandomTable(t, merchant.ID)
	user := createRandomUser(t)

	result, err := testStore.OpenDiningSessionTx(context.Background(), OpenDiningSessionTxParams{
		TableID:                table.ID,
		MerchantID:             merchant.ID,
		UserID:                 user.ID,
		ReservationID:          pgtype.Int8{Valid: false},
		ImportReservationItems: false,
	})
	require.NoError(t, err)

	_, err = testStore.CreateBillingGroupMember(context.Background(), CreateBillingGroupMemberParams{
		BillingGroupID: result.BillingGroup.ID,
		UserID:         user.ID,
		Role:           "member",
	})
	require.Error(t, err)
}

func TestBillingGroupDefaultUnique(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	table := createRandomTable(t, merchant.ID)
	user := createRandomUser(t)

	result, err := testStore.OpenDiningSessionTx(context.Background(), OpenDiningSessionTxParams{
		TableID:                table.ID,
		MerchantID:             merchant.ID,
		UserID:                 user.ID,
		ReservationID:          pgtype.Int8{Valid: false},
		ImportReservationItems: false,
	})
	require.NoError(t, err)

	_, err = testStore.CreateBillingGroup(context.Background(), CreateBillingGroupParams{
		DiningSessionID: result.Session.ID,
		Status:          "open",
		IsDefault:       true,
		TotalAmount:     0,
		PaidAmount:      0,
	})
	require.Error(t, err)
}

func TestDiningSessionRemainsOpenUntilClosed(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	table := createRandomTable(t, merchant.ID)
	user := createRandomUser(t)

	result, err := testStore.OpenDiningSessionTx(context.Background(), OpenDiningSessionTxParams{
		TableID:                table.ID,
		MerchantID:             merchant.ID,
		UserID:                 user.ID,
		ReservationID:          pgtype.Int8{Valid: false},
		ImportReservationItems: false,
	})
	require.NoError(t, err)

	active, err := testStore.GetActiveDiningSessionByTable(context.Background(), table.ID)
	require.NoError(t, err)
	require.Equal(t, result.Session.ID, active.ID)
	require.Equal(t, "open", active.Status)

	closed, err := testStore.CloseDiningSession(context.Background(), result.Session.ID)
	require.NoError(t, err)
	require.Equal(t, "closed", closed.Status)
}

func TestListDiningSessionsByUserUsesIDTieBreaker(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	tiedCreatedAt := time.Now().UTC().Truncate(time.Microsecond)

	table1 := createRandomTable(t, merchant.ID)
	table2 := createRandomTable(t, merchant.ID)

	session1, err := testStore.OpenDiningSessionTx(context.Background(), OpenDiningSessionTxParams{
		TableID:                table1.ID,
		MerchantID:             merchant.ID,
		UserID:                 user.ID,
		ReservationID:          pgtype.Int8{Valid: false},
		ImportReservationItems: false,
	})
	require.NoError(t, err)

	session2, err := testStore.OpenDiningSessionTx(context.Background(), OpenDiningSessionTxParams{
		TableID:                table2.ID,
		MerchantID:             merchant.ID,
		UserID:                 user.ID,
		ReservationID:          pgtype.Int8{Valid: false},
		ImportReservationItems: false,
	})
	require.NoError(t, err)

	_, err = testStore.(*SQLStore).connPool.Exec(context.Background(),
		"UPDATE dining_sessions SET created_at = $1 WHERE id = ANY($2)",
		tiedCreatedAt,
		[]int64{session1.Session.ID, session2.Session.ID},
	)
	require.NoError(t, err)

	sessions, err := testStore.ListDiningSessionsByUser(context.Background(), ListDiningSessionsByUserParams{
		UserID: user.ID,
		Limit:  2,
		Offset: 0,
	})
	require.NoError(t, err)
	require.Len(t, sessions, 2)
	require.Equal(t, session2.Session.ID, sessions[0].ID)
	require.Equal(t, session1.Session.ID, sessions[1].ID)
}

func TestListPaidOpenDineInSessionsForCheckoutRecovery(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	cutoff := time.Now().Add(-5 * time.Minute).UTC().Truncate(time.Microsecond)
	oldOpenedAt := cutoff.Add(-time.Minute)
	recentOpenedAt := cutoff.Add(time.Minute)

	eligible := createDineInSessionWithActiveOrderForRecovery(t, merchant.ID, user.ID, dbRecoverySessionSeed{
		SessionStatus: "open",
		OrderStatus:   OrderStatusPaid,
		OrderType:     OrderTypeDineIn,
		OpenedAt:      oldOpenedAt,
	})
	pending := createDineInSessionWithActiveOrderForRecovery(t, merchant.ID, user.ID, dbRecoverySessionSeed{
		SessionStatus: "open",
		OrderStatus:   OrderStatusPending,
		OrderType:     OrderTypeDineIn,
		OpenedAt:      oldOpenedAt,
	})
	takeaway := createDineInSessionWithActiveOrderForRecovery(t, merchant.ID, user.ID, dbRecoverySessionSeed{
		SessionStatus: "open",
		OrderStatus:   OrderStatusPaid,
		OrderType:     OrderTypeTakeaway,
		OpenedAt:      oldOpenedAt,
	})
	recent := createDineInSessionWithActiveOrderForRecovery(t, merchant.ID, user.ID, dbRecoverySessionSeed{
		SessionStatus: "open",
		OrderStatus:   OrderStatusPaid,
		OrderType:     OrderTypeDineIn,
		OpenedAt:      recentOpenedAt,
	})
	closed := createDineInSessionWithActiveOrderForRecovery(t, merchant.ID, user.ID, dbRecoverySessionSeed{
		SessionStatus: "closed",
		OrderStatus:   OrderStatusPaid,
		OrderType:     OrderTypeDineIn,
		OpenedAt:      oldOpenedAt,
	})
	noActiveOrder := createDineInSessionWithoutActiveOrderForRecovery(t, merchant.ID, user.ID, oldOpenedAt)
	mismatchedMerchant := createMismatchedMerchantActiveOrderSessionForRecovery(t, merchant.ID, user.ID, oldOpenedAt)

	sessions, err := testStore.ListPaidOpenDineInSessionsForCheckoutRecovery(context.Background(), ListPaidOpenDineInSessionsForCheckoutRecoveryParams{
		OpenedBefore: cutoff,
		Limit:        1000,
	})
	require.NoError(t, err)

	gotIDs := make(map[int64]DiningSession, len(sessions))
	for _, session := range sessions {
		gotIDs[session.ID] = session
	}

	gotEligible, ok := gotIDs[eligible.ID]
	require.True(t, ok)
	require.Equal(t, eligible.MerchantID, gotEligible.MerchantID)
	require.True(t, gotEligible.ActiveOrderID.Valid)
	require.NotContains(t, gotIDs, pending.ID)
	require.NotContains(t, gotIDs, takeaway.ID)
	require.NotContains(t, gotIDs, recent.ID)
	require.NotContains(t, gotIDs, closed.ID)
	require.NotContains(t, gotIDs, noActiveOrder.ID)
	require.NotContains(t, gotIDs, mismatchedMerchant.ID)
}

func TestPaidOpenDineInCheckoutRecoveryIndexExists(t *testing.T) {
	indexdef := queryPaidOpenDineInCheckoutRecoveryIndexDef(t, testStore.(*SQLStore).connPool)
	require.Contains(t, indexdef, "ON public.dining_sessions USING btree (opened_at, id)")
	require.Contains(t, indexdef, "WHERE")
	require.Contains(t, indexdef, "status")
	require.Contains(t, indexdef, "open")
	require.Contains(t, indexdef, "active_order_id IS NOT NULL")
}

func TestPaidOpenDineInCheckoutRecoveryIndexMigrationCleanDatabaseContract(t *testing.T) {
	dbSource := createMembershipSettingsMigrationDB(t)
	runMembershipSettingsMigrations(t, dbSource, 269)

	pool := openMembershipSettingsMigrationPool(t, dbSource)
	defer pool.Close()

	indexdef := queryPaidOpenDineInCheckoutRecoveryIndexDef(t, pool)
	require.Contains(t, indexdef, "ON public.dining_sessions USING btree (opened_at, id)")
	require.Contains(t, indexdef, "WHERE")
	require.Contains(t, indexdef, "status")
	require.Contains(t, indexdef, "open")
	require.Contains(t, indexdef, "active_order_id IS NOT NULL")
}

func TestPaidOpenDineInCheckoutRecoveryIndexMigrationIncrementalContract(t *testing.T) {
	dbSource := createMembershipSettingsMigrationDB(t)
	runMembershipSettingsMigrations(t, dbSource, 268)
	runMembershipSettingsMigrations(t, dbSource, 269)

	pool := openMembershipSettingsMigrationPool(t, dbSource)
	defer pool.Close()

	indexdef := queryPaidOpenDineInCheckoutRecoveryIndexDef(t, pool)
	require.Contains(t, indexdef, "ON public.dining_sessions USING btree (opened_at, id)")
	require.Contains(t, indexdef, "WHERE")
	require.Contains(t, indexdef, "status")
	require.Contains(t, indexdef, "open")
	require.Contains(t, indexdef, "active_order_id IS NOT NULL")
}

type dbRecoverySessionSeed struct {
	SessionStatus string
	OrderStatus   string
	OrderType     string
	OpenedAt      time.Time
}

func createDineInSessionWithActiveOrderForRecovery(t *testing.T, merchantID, userID int64, seed dbRecoverySessionSeed) DiningSession {
	table := createRandomTable(t, merchantID)
	order, err := testStore.CreateOrder(context.Background(), CreateOrderParams{
		OrderNo:             "REC" + time.Now().Format("150405.000000") + util.RandomString(8),
		UserID:              userID,
		MerchantID:          merchantID,
		OrderType:           seed.OrderType,
		TableID:             pgtype.Int8{Int64: table.ID, Valid: seed.OrderType == OrderTypeDineIn},
		DeliveryFee:         0,
		Subtotal:            3000,
		DiscountAmount:      0,
		DeliveryFeeDiscount: 0,
		TotalAmount:         3000,
		Status:              seed.OrderStatus,
	})
	require.NoError(t, err)

	session, err := testStore.CreateDiningSession(context.Background(), CreateDiningSessionParams{
		MerchantID:    merchantID,
		TableID:       table.ID,
		ReservationID: pgtype.Int8{Valid: false},
		UserID:        userID,
		ActiveOrderID: pgtype.Int8{Int64: order.ID, Valid: true},
		Status:        seed.SessionStatus,
	})
	require.NoError(t, err)

	_, err = testStore.(*SQLStore).connPool.Exec(context.Background(),
		`UPDATE dining_sessions SET opened_at = $1, updated_at = $1 WHERE id = $2`,
		seed.OpenedAt,
		session.ID,
	)
	require.NoError(t, err)

	session, err = testStore.GetDiningSession(context.Background(), session.ID)
	require.NoError(t, err)
	return session
}

func createDineInSessionWithoutActiveOrderForRecovery(t *testing.T, merchantID, userID int64, openedAt time.Time) DiningSession {
	table := createRandomTable(t, merchantID)
	session, err := testStore.CreateDiningSession(context.Background(), CreateDiningSessionParams{
		MerchantID:    merchantID,
		TableID:       table.ID,
		ReservationID: pgtype.Int8{Valid: false},
		UserID:        userID,
		ActiveOrderID: pgtype.Int8{Valid: false},
		Status:        "open",
	})
	require.NoError(t, err)

	_, err = testStore.(*SQLStore).connPool.Exec(context.Background(),
		`UPDATE dining_sessions SET opened_at = $1, updated_at = $1 WHERE id = $2`,
		openedAt,
		session.ID,
	)
	require.NoError(t, err)

	return session
}

func queryPaidOpenDineInCheckoutRecoveryIndexDef(t *testing.T, pool interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}) string {
	t.Helper()

	var indexdef string
	err := pool.QueryRow(
		context.Background(),
		`SELECT indexdef
		 FROM pg_indexes
		 WHERE schemaname = 'public'
		   AND tablename = 'dining_sessions'
		   AND indexname = 'idx_dining_sessions_open_active_order_opened_at'`,
	).Scan(&indexdef)
	require.NoError(t, err)
	return indexdef
}

func createMismatchedMerchantActiveOrderSessionForRecovery(t *testing.T, merchantID, userID int64, openedAt time.Time) DiningSession {
	otherOwner := createRandomUser(t)
	otherMerchant := createRandomMerchantWithOwner(t, otherOwner.ID)
	otherTable := createRandomTable(t, otherMerchant.ID)
	order, err := testStore.CreateOrder(context.Background(), CreateOrderParams{
		OrderNo:             "MIS" + time.Now().Format("150405.000000") + util.RandomString(8),
		UserID:              userID,
		MerchantID:          otherMerchant.ID,
		OrderType:           OrderTypeDineIn,
		TableID:             pgtype.Int8{Int64: otherTable.ID, Valid: true},
		DeliveryFee:         0,
		Subtotal:            3000,
		DiscountAmount:      0,
		DeliveryFeeDiscount: 0,
		TotalAmount:         3000,
		Status:              OrderStatusPaid,
	})
	require.NoError(t, err)

	table := createRandomTable(t, merchantID)
	session, err := testStore.CreateDiningSession(context.Background(), CreateDiningSessionParams{
		MerchantID:    merchantID,
		TableID:       table.ID,
		ReservationID: pgtype.Int8{Valid: false},
		UserID:        userID,
		ActiveOrderID: pgtype.Int8{Int64: order.ID, Valid: true},
		Status:        "open",
	})
	require.NoError(t, err)

	_, err = testStore.(*SQLStore).connPool.Exec(context.Background(),
		`UPDATE dining_sessions SET opened_at = $1, updated_at = $1 WHERE id = $2`,
		openedAt,
		session.ID,
	)
	require.NoError(t, err)

	return session
}
