package scheduler

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestDataCleanupScheduler_CheckTimedOutPrintAnomalies_PublishesAlert(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	allowPlatformAlertEventPersistence(store)
	publisher := &recordingPublisher{}
	s := NewDataCleanupScheduler(store, nil, publisher)

	createdAt := time.Now().Add(-25 * time.Minute)
	store.EXPECT().ListTimedOutPrintAnomalies(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ListTimedOutPrintAnomaliesParams) ([]db.ListTimedOutPrintAnomaliesRow, error) {
		require.Equal(t, timedOutPrintAnomalyBatchLimit, arg.Limit)
		require.WithinDuration(t, time.Now().Add(-timedOutPrintAnomalyThreshold), arg.CreatedAt, 2*time.Second)
		return []db.ListTimedOutPrintAnomaliesRow{
			{
				ID:               101,
				MerchantID:       11,
				MerchantName:     "测试商户A",
				OrderID:          201,
				OrderNo:          "ORD-201",
				PrinterID:        301,
				PrinterName:      "前台打印机",
				Status:           "failed",
				ErrorMessage:     pgtype.Text{String: "printer offline", Valid: true},
				AnomalyCreatedAt: createdAt,
			},
			{
				ID:               102,
				MerchantID:       12,
				MerchantName:     "测试商户B",
				OrderID:          202,
				OrderNo:          "ORD-202",
				PrinterID:        302,
				PrinterName:      "后厨打印机",
				Status:           "pending",
				AnomalyCreatedAt: createdAt.Add(-5 * time.Minute),
			},
		}, nil
	})

	s.checkTimedOutPrintAnomalies()

	published := publisher.snapshot()
	require.Len(t, published, 1)
	require.Equal(t, worker.AlertChannel, published[0].channel)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(published[0].payload, &payload))
	alertData := payload["data"].(map[string]any)
	require.Equal(t, string(worker.AlertTypePrintAnomalyTimeout), alertData["alert_type"])
	require.Equal(t, string(worker.AlertLevelWarning), alertData["level"])
	require.Equal(t, "商户打印异常超时未恢复", alertData["title"])
	require.Contains(t, alertData["message"], "2 条打印异常")
	extra := alertData["extra"].(map[string]any)
	require.EqualValues(t, 2, extra["total"])
	require.EqualValues(t, 1, extra["failed_count"])
	require.EqualValues(t, 1, extra["pending_count"])
	samples := extra["sample_print_anomaly"].([]any)
	require.Len(t, samples, 2)
}

func TestDataCleanupScheduler_CheckTimedOutPrintAnomalies_DeduplicatesRepeatedAlerts(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	allowPlatformAlertEventPersistence(store)
	publisher := &recordingPublisher{}
	s := NewDataCleanupScheduler(store, nil, publisher)

	items := []db.ListTimedOutPrintAnomaliesRow{{
		ID:               501,
		MerchantID:       21,
		MerchantName:     "测试商户",
		OrderID:          601,
		OrderNo:          "ORD-601",
		PrinterID:        701,
		PrinterName:      "前台打印机",
		Status:           "failed",
		AnomalyCreatedAt: time.Now().Add(-30 * time.Minute),
	}}

	store.EXPECT().ListTimedOutPrintAnomalies(gomock.Any(), gomock.Any()).Times(2).Return(items, nil)

	s.checkTimedOutPrintAnomalies()
	s.checkTimedOutPrintAnomalies()

	published := publisher.snapshot()
	require.Len(t, published, 1)
}

func TestDataCleanupScheduler_CheckTimedOutPrintAnomalies_DeduplicatesByOrderAndPrinter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	allowPlatformAlertEventPersistence(store)
	publisher := &recordingPublisher{}
	s := NewDataCleanupScheduler(store, nil, publisher)

	first := db.ListTimedOutPrintAnomaliesRow{
		ID:               801,
		MerchantID:       31,
		MerchantName:     "测试商户",
		OrderID:          901,
		OrderNo:          "ORD-901",
		PrinterID:        1001,
		PrinterName:      "前台打印机",
		Status:           "failed",
		AnomalyCreatedAt: time.Now().Add(-35 * time.Minute),
	}
	second := first
	second.ID = 802
	second.AnomalyCreatedAt = time.Now().Add(-20 * time.Minute)

	store.EXPECT().ListTimedOutPrintAnomalies(gomock.Any(), gomock.Any()).Times(2).Return([]db.ListTimedOutPrintAnomaliesRow{first}, nil)
	store.EXPECT().ListTimedOutPrintAnomalies(gomock.Any(), gomock.Any()).Times(1).Return([]db.ListTimedOutPrintAnomaliesRow{second}, nil)

	s.checkTimedOutPrintAnomalies()
	s.checkTimedOutPrintAnomalies()
	s.checkTimedOutPrintAnomalies()

	published := publisher.snapshot()
	require.Len(t, published, 1)
}
