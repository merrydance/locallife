package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/cloudprint"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCloudPrinterStatusPollSchedulerRunOnceMarksShangpengSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &printClientRecorder{queryOrderStateResult: true}
	manager := printProviderManagerStub{providers: map[string]cloudprint.Client{
		string(cloudprint.ProviderShangpeng): client,
	}}
	scheduler := NewCloudPrinterStatusPollScheduler(store, manager, util.Config{
		CloudPrinterStatusPollInterval:     time.Minute,
		CloudPrinterStatusPollBatchSize:    10,
		CloudPrinterStatusPollInitialDelay: 30 * time.Second,
		CloudPrinterStatusPollMaxAge:       12 * time.Hour,
	})

	store.EXPECT().
		ExpireProviderStatusPrintLogs(gomock.Any(), gomock.Any()).
		Return([]db.PrintLog{}, nil)
	store.EXPECT().
		ClaimPendingProviderStatusPrintLogs(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.ClaimPendingProviderStatusPrintLogsParams) ([]db.ClaimPendingProviderStatusPrintLogsRow, error) {
			require.Equal(t, []string{string(cloudprint.ProviderShangpeng)}, arg.PrinterTypes)
			require.Equal(t, int32(10), arg.LimitCount)
			require.True(t, arg.ReadyBefore.Before(time.Now()))
			require.True(t, arg.CreatedAfter.Before(arg.ReadyBefore))
			return []db.ClaimPendingProviderStatusPrintLogsRow{pendingStatusPollRow(101, "sp-order-101")}, nil
		})
	store.EXPECT().
		MarkProviderStatusPrintLogTerminal(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.MarkProviderStatusPrintLogTerminalParams) (db.PrintLog, error) {
			require.Equal(t, int64(101), arg.ID)
			require.Equal(t, printLogStatusSuccess, arg.Status)
			require.False(t, arg.ErrorMessage.Valid)
			return db.PrintLog{ID: arg.ID, Status: arg.Status}, nil
		})

	scheduler.RunOnce()

	require.Equal(t, []string{"sp-order-101"}, client.queryOrderStateCalls)
}

func TestCloudPrinterStatusPollSchedulerRunOnceLeavesUnprintedShangpengPending(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &printClientRecorder{queryOrderStateResult: false}
	manager := printProviderManagerStub{providers: map[string]cloudprint.Client{
		string(cloudprint.ProviderShangpeng): client,
	}}
	scheduler := NewCloudPrinterStatusPollScheduler(store, manager, util.Config{
		CloudPrinterStatusPollInterval:     time.Minute,
		CloudPrinterStatusPollBatchSize:    10,
		CloudPrinterStatusPollInitialDelay: 30 * time.Second,
		CloudPrinterStatusPollMaxAge:       12 * time.Hour,
	})

	store.EXPECT().ExpireProviderStatusPrintLogs(gomock.Any(), gomock.Any()).Return([]db.PrintLog{}, nil)
	store.EXPECT().
		ClaimPendingProviderStatusPrintLogs(gomock.Any(), gomock.Any()).
		Return([]db.ClaimPendingProviderStatusPrintLogsRow{pendingStatusPollRow(102, "sp-order-102")}, nil)
	store.EXPECT().MarkProviderStatusPrintLogTerminal(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().RecordProviderStatusPollError(gomock.Any(), gomock.Any()).Times(0)

	scheduler.RunOnce()

	require.Equal(t, []string{"sp-order-102"}, client.queryOrderStateCalls)
}

func TestCloudPrinterStatusPollSchedulerRunOnceRecordsProviderErrorWithoutTerminalStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &printClientRecorder{queryOrderStateErr: errors.New("provider timeout with appsecret=should-not-leak")}
	manager := printProviderManagerStub{providers: map[string]cloudprint.Client{
		string(cloudprint.ProviderShangpeng): client,
	}}
	scheduler := NewCloudPrinterStatusPollScheduler(store, manager, util.Config{
		CloudPrinterStatusPollInterval:     time.Minute,
		CloudPrinterStatusPollBatchSize:    10,
		CloudPrinterStatusPollInitialDelay: 30 * time.Second,
		CloudPrinterStatusPollMaxAge:       12 * time.Hour,
	})

	store.EXPECT().ExpireProviderStatusPrintLogs(gomock.Any(), gomock.Any()).Return([]db.PrintLog{}, nil)
	store.EXPECT().
		ClaimPendingProviderStatusPrintLogs(gomock.Any(), gomock.Any()).
		Return([]db.ClaimPendingProviderStatusPrintLogsRow{pendingStatusPollRow(103, "sp-order-103")}, nil)
	store.EXPECT().MarkProviderStatusPrintLogTerminal(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().
		RecordProviderStatusPollError(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.RecordProviderStatusPollErrorParams) (db.PrintLog, error) {
			require.Equal(t, int64(103), arg.ID)
			require.True(t, arg.ProviderStatusLastError.Valid)
			require.Contains(t, arg.ProviderStatusLastError.String, "provider timeout")
			require.NotContains(t, arg.ProviderStatusLastError.String, "should-not-leak")
			return db.PrintLog{ID: arg.ID, Status: printLogStatusPending}, nil
		})

	scheduler.RunOnce()

	require.Equal(t, []string{"sp-order-103"}, client.queryOrderStateCalls)
}

func TestCloudPrinterStatusPollSchedulerRunOnceExpiresOldPendingBeforeProviderCall(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := &printClientRecorder{queryOrderStateResult: true}
	manager := printProviderManagerStub{providers: map[string]cloudprint.Client{
		string(cloudprint.ProviderShangpeng): client,
	}}
	scheduler := NewCloudPrinterStatusPollScheduler(store, manager, util.Config{
		CloudPrinterStatusPollInterval:     time.Minute,
		CloudPrinterStatusPollBatchSize:    10,
		CloudPrinterStatusPollInitialDelay: 30 * time.Second,
		CloudPrinterStatusPollMaxAge:       12 * time.Hour,
	})

	store.EXPECT().
		ExpireProviderStatusPrintLogs(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.ExpireProviderStatusPrintLogsParams) ([]db.PrintLog, error) {
			require.Equal(t, []string{string(cloudprint.ProviderShangpeng)}, arg.PrinterTypes)
			require.Equal(t, int32(10), arg.LimitCount)
			require.True(t, arg.ErrorMessage.Valid)
			require.Equal(t, "provider_print_status_expired", arg.ErrorMessage.String)
			return []db.PrintLog{{ID: 104, Status: printLogStatusFailed}}, nil
		})
	store.EXPECT().
		ClaimPendingProviderStatusPrintLogs(gomock.Any(), gomock.Any()).
		Return([]db.ClaimPendingProviderStatusPrintLogsRow{}, nil)

	scheduler.RunOnce()

	require.Empty(t, client.queryOrderStateCalls)
}

func TestCloudPrinterStatusPollSchedulerRunOnceSkipsWhenNoPollableProviderConfigured(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	manager := printProviderManagerStub{providers: map[string]cloudprint.Client{
		string(cloudprint.ProviderFeieyun): &printClientRecorder{},
	}}
	scheduler := NewCloudPrinterStatusPollScheduler(store, manager, util.Config{
		CloudPrinterStatusPollInterval:     time.Minute,
		CloudPrinterStatusPollBatchSize:    10,
		CloudPrinterStatusPollInitialDelay: 30 * time.Second,
		CloudPrinterStatusPollMaxAge:       12 * time.Hour,
	})

	store.EXPECT().ExpireProviderStatusPrintLogs(gomock.Any(), gomock.Any()).Times(0)
	store.EXPECT().ClaimPendingProviderStatusPrintLogs(gomock.Any(), gomock.Any()).Times(0)

	scheduler.RunOnce()
}

func pendingStatusPollRow(id int64, vendorOrderID string) db.ClaimPendingProviderStatusPrintLogsRow {
	return db.ClaimPendingProviderStatusPrintLogsRow{
		ID:            id,
		OrderID:       2000 + id,
		PrinterID:     3000 + id,
		Status:        printLogStatusPending,
		CreatedAt:     time.Now().Add(-time.Minute),
		VendorOrderID: pgtype.Text{String: vendorOrderID, Valid: true},
		PrinterSn:     "SP-SN-001",
		PrinterType:   string(cloudprint.ProviderShangpeng),
	}
}
