package db

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestProcessSelfCloudPrintCallbackTxMarksPrintLogTerminalAndDedupesEvent(t *testing.T) {
	ctx := context.Background()
	printJobID := "psj_tx_success_" + util.RandomString(12)
	eventID := "evt_tx_success_" + util.RandomString(12)
	printLog := createSelfCloudPendingPrintLog(t, printJobID)
	payload := []byte(fmt.Sprintf(`{"event_id":%q,"print_job_id":%q,"status":"success"}`, eventID, printJobID))

	first, err := testStore.ProcessSelfCloudPrintCallbackTx(ctx, ProcessSelfCloudPrintCallbackTxParams{
		EventID:        eventID,
		PrintJobID:     printJobID,
		PrintLogID:     pgtype.Int8{Int64: printLog.ID, Valid: true},
		CallbackStatus: "success",
		PrintLogStatus: PrintLogStatusSuccess,
		RawPayload:     payload,
	})
	require.NoError(t, err)
	require.False(t, first.Duplicate)
	require.False(t, first.AlreadyTerminal)
	require.Equal(t, PrintLogStatusSuccess, first.PrintLog.Status)
	require.Equal(t, printLog.ID, first.Event.PrintLogID.Int64)
	require.JSONEq(t, string(payload), string(first.Event.RawPayload))

	updated, err := testStore.GetPrintLog(ctx, printLog.ID)
	require.NoError(t, err)
	require.Equal(t, PrintLogStatusSuccess, updated.Status)
	require.True(t, updated.PrintedAt.Valid)

	replayed, err := testStore.ProcessSelfCloudPrintCallbackTx(ctx, ProcessSelfCloudPrintCallbackTxParams{
		EventID:        eventID,
		PrintJobID:     printJobID,
		PrintLogID:     pgtype.Int8{Int64: printLog.ID, Valid: true},
		CallbackStatus: "success",
		PrintLogStatus: PrintLogStatusSuccess,
		RawPayload:     payload,
	})
	require.NoError(t, err)
	require.True(t, replayed.Duplicate)
	require.Equal(t, first.Event.ID, replayed.Event.ID)
}

func TestProcessSelfCloudPrintCallbackTxRecordsEventWhenPollingAlreadyTerminal(t *testing.T) {
	ctx := context.Background()
	printJobID := "psj_tx_race_" + util.RandomString(12)
	eventID := "evt_tx_race_" + util.RandomString(12)
	printLog := createSelfCloudPendingPrintLog(t, printJobID)

	_, err := testStore.MarkProviderStatusPrintLogTerminal(ctx, MarkProviderStatusPrintLogTerminalParams{
		ID:     printLog.ID,
		Status: PrintLogStatusSuccess,
	})
	require.NoError(t, err)

	result, err := testStore.ProcessSelfCloudPrintCallbackTx(ctx, ProcessSelfCloudPrintCallbackTxParams{
		EventID:        eventID,
		PrintJobID:     printJobID,
		PrintLogID:     pgtype.Int8{Int64: printLog.ID, Valid: true},
		CallbackStatus: "success",
		PrintLogStatus: PrintLogStatusSuccess,
		RawPayload:     []byte(fmt.Sprintf(`{"event_id":%q,"print_job_id":%q,"status":"success"}`, eventID, printJobID)),
	})
	require.NoError(t, err)
	require.False(t, result.Duplicate)
	require.True(t, result.AlreadyTerminal)
	require.Equal(t, PrintLogStatusSuccess, result.PrintLog.Status)
	require.Equal(t, printLog.ID, result.Event.PrintLogID.Int64)
}

func TestProcessSelfCloudPrintCallbackTxRejectsMismatchedPrintLogID(t *testing.T) {
	ctx := context.Background()
	printJobID := "psj_tx_mismatch_" + util.RandomString(12)
	eventID := "evt_tx_mismatch_" + util.RandomString(12)
	_ = createSelfCloudPendingPrintLog(t, printJobID)

	_, err := testStore.ProcessSelfCloudPrintCallbackTx(ctx, ProcessSelfCloudPrintCallbackTxParams{
		EventID:        eventID,
		PrintJobID:     printJobID,
		PrintLogID:     pgtype.Int8{Int64: 999999999, Valid: true},
		CallbackStatus: "success",
		PrintLogStatus: PrintLogStatusSuccess,
		RawPayload:     []byte(fmt.Sprintf(`{"event_id":%q,"print_job_id":%q,"status":"success"}`, eventID, printJobID)),
	})
	require.ErrorIs(t, err, ErrRecordNotFound)

	_, lookupErr := testStore.GetSelfCloudPrintCallbackEventByEventID(ctx, eventID)
	require.ErrorIs(t, lookupErr, ErrRecordNotFound)
}

func createSelfCloudPendingPrintLog(t *testing.T, printJobID string) PrintLog {
	t.Helper()

	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	order := createRandomOrderWithUserAndMerchant(t, user.ID, merchant.ID)
	printer, err := testStore.CreateCloudPrinter(context.Background(), CreateCloudPrinterParams{
		MerchantID:       merchant.ID,
		PrinterName:      "self cloud printer",
		PrinterSn:        "MDP" + util.RandomString(17),
		PrinterKey:       util.RandomString(32),
		PrinterType:      CloudPrinterProviderSelfCloud,
		PrinterRole:      "front",
		PrintTakeout:     true,
		PrintDineIn:      true,
		PrintReservation: true,
	})
	require.NoError(t, err)

	printLog, err := testStore.CreatePrintLog(context.Background(), CreatePrintLogParams{
		OrderID:          order.ID,
		PrinterID:        printer.ID,
		PrintContent:     "receipt",
		Status:           PrintLogStatusPending,
		TaskKey:          pgtype.Text{String: "print_log:" + util.RandomString(12), Valid: true},
		ProviderOriginID: pgtype.Text{String: util.RandomString(32), Valid: true},
	})
	require.NoError(t, err)

	printLog, err = testStore.UpdatePrintLogStatus(context.Background(), UpdatePrintLogStatusParams{
		ID:            printLog.ID,
		Status:        PrintLogStatusPending,
		VendorOrderID: pgtype.Text{String: printJobID, Valid: true},
	})
	require.NoError(t, err)
	return printLog
}
