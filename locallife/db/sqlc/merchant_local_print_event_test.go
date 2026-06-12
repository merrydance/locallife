package db

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestUpsertMerchantLocalPrintEventDedupesByMerchantEventKey(t *testing.T) {
	ctx := context.Background()
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	order := createRandomOrderWithUserAndMerchant(t, user.ID, merchant.ID)
	eventKey := "accepted-receipt:" + util.RandomString(12)

	started, err := testStore.UpsertMerchantLocalPrintEvent(ctx, UpsertMerchantLocalPrintEventParams{
		MerchantID:  merchant.ID,
		OrderID:     order.ID,
		EventKey:    eventKey,
		Source:      MerchantLocalPrintEventSourceBle,
		Status:      MerchantLocalPrintEventStatusStarted,
		PrinterName: pgtype.Text{String: "前台蓝牙打印机", Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, MerchantLocalPrintEventStatusStarted, started.Status)
	require.False(t, started.PrintedAt.Valid)

	success, err := testStore.UpsertMerchantLocalPrintEvent(ctx, UpsertMerchantLocalPrintEventParams{
		MerchantID:  merchant.ID,
		OrderID:     order.ID,
		EventKey:    eventKey,
		Source:      MerchantLocalPrintEventSourceBle,
		Status:      MerchantLocalPrintEventStatusSuccess,
		PrinterName: pgtype.Text{String: "前台蓝牙打印机", Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, started.ID, success.ID)
	require.Equal(t, MerchantLocalPrintEventStatusSuccess, success.Status)
	require.True(t, success.PrintedAt.Valid)

	failedAfterSuccess, err := testStore.UpsertMerchantLocalPrintEvent(ctx, UpsertMerchantLocalPrintEventParams{
		MerchantID:   merchant.ID,
		OrderID:      order.ID,
		EventKey:     eventKey,
		Source:       MerchantLocalPrintEventSourceBle,
		Status:       MerchantLocalPrintEventStatusFailed,
		ErrorMessage: pgtype.Text{String: "late duplicate failure", Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, started.ID, failedAfterSuccess.ID)
	require.Equal(t, MerchantLocalPrintEventStatusSuccess, failedAfterSuccess.Status)
	require.False(t, failedAfterSuccess.ErrorMessage.Valid)
}

func TestUpsertMerchantLocalPrintEventRejectsOrderOutsideMerchant(t *testing.T) {
	ctx := context.Background()
	merchant := createRandomMerchantForTest(t)
	otherMerchant := createRandomMerchantForTest(t)
	user := createRandomUser(t)
	order := createRandomOrderWithUserAndMerchant(t, user.ID, otherMerchant.ID)

	_, err := testStore.UpsertMerchantLocalPrintEvent(ctx, UpsertMerchantLocalPrintEventParams{
		MerchantID: merchant.ID,
		OrderID:    order.ID,
		EventKey:   "accepted-receipt:" + util.RandomString(12),
		Source:     MerchantLocalPrintEventSourceBle,
		Status:     MerchantLocalPrintEventStatusStarted,
	})
	require.ErrorIs(t, err, ErrRecordNotFound)
}
