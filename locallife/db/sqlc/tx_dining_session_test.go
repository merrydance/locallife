package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
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
