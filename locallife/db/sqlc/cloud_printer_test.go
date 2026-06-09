package db

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func createRandomCloudPrinter(t *testing.T, merchantID int64) CloudPrinter {
	arg := CreateCloudPrinterParams{
		MerchantID:       merchantID,
		PrinterName:      "test printer",
		PrinterSn:        util.RandomString(20),
		PrinterKey:       util.RandomString(32),
		PrinterType:      "feieyun",
		PrinterRole:      "front",
		PrintTakeout:     true,
		PrintDineIn:      true,
		PrintReservation: true,
	}

	printer, err := testStore.CreateCloudPrinter(context.Background(), arg)
	require.NoError(t, err)
	require.NotZero(t, printer.ID)
	require.Equal(t, arg.MerchantID, printer.MerchantID)
	require.Equal(t, arg.PrinterSn, printer.PrinterSn)
	require.True(t, printer.IsActive)

	return printer
}

func TestDeleteCloudPrinterSoftDeletesWithHistoricalPrintLogs(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	order := createRandomOrderWithUserAndMerchant(t, user.ID, merchant.ID)
	printer := createRandomCloudPrinter(t, merchant.ID)

	_, err := testStore.CreatePrintLog(context.Background(), CreatePrintLogParams{
		OrderID:          order.ID,
		PrinterID:        printer.ID,
		PrintContent:     "receipt",
		Status:           PrintLogStatusSuccess,
		TaskKey:          pgtype.Text{String: util.RandomString(16), Valid: true},
		ProviderOriginID: pgtype.Text{String: util.RandomString(16), Valid: true},
	})
	require.NoError(t, err)

	err = testStore.DeleteCloudPrinter(context.Background(), printer.ID)
	require.NoError(t, err)

	_, err = testStore.GetCloudPrinter(context.Background(), printer.ID)
	require.ErrorIs(t, err, ErrRecordNotFound)

	_, err = testStore.GetCloudPrinterBySN(context.Background(), printer.PrinterSn)
	require.ErrorIs(t, err, ErrRecordNotFound)

	listed, err := testStore.ListCloudPrintersByMerchant(context.Background(), merchant.ID)
	require.NoError(t, err)
	for _, item := range listed {
		require.NotEqual(t, printer.ID, item.ID)
	}

	logs, err := testStore.ListPrintLogsByPrinter(context.Background(), ListPrintLogsByPrinterParams{
		PrinterID: printer.ID,
		Limit:     10,
		Offset:    0,
	})
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Equal(t, printer.ID, logs[0].PrinterID)

	replacement, err := testStore.CreateCloudPrinter(context.Background(), CreateCloudPrinterParams{
		MerchantID:       merchant.ID,
		PrinterName:      "replacement printer",
		PrinterSn:        printer.PrinterSn,
		PrinterKey:       util.RandomString(32),
		PrinterType:      printer.PrinterType,
		PrinterRole:      "front",
		PrintTakeout:     true,
		PrintDineIn:      true,
		PrintReservation: true,
	})
	require.NoError(t, err)
	require.NotEqual(t, printer.ID, replacement.ID)
}

func TestGetCloudPrinterIncludingDeletedKeepsHistoricalPrinterReadable(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	printer := createRandomCloudPrinter(t, merchant.ID)

	err := testStore.DeleteCloudPrinter(context.Background(), printer.ID)
	require.NoError(t, err)

	current, err := testStore.GetCloudPrinterIncludingDeleted(context.Background(), printer.ID)
	require.NoError(t, err)
	require.Equal(t, printer.ID, current.ID)
	require.Equal(t, printer.PrinterSn, current.PrinterSn)
	require.False(t, current.IsActive)
	require.True(t, current.DeletedAt.Valid)

	_, err = testStore.GetCloudPrinter(context.Background(), printer.ID)
	require.ErrorIs(t, err, ErrRecordNotFound)
}
