package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestDineInCheckoutRecoveryScheduler_ClosesPaidOpenSessions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	scheduler := NewDineInCheckoutRecoveryScheduler(store)

	session := db.DiningSession{
		ID:         101,
		MerchantID: 202,
		Status:     "open",
		OpenedAt:   time.Now().Add(-10 * time.Minute),
	}

	store.EXPECT().
		ListPaidOpenDineInSessionsForCheckoutRecovery(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, arg db.ListPaidOpenDineInSessionsForCheckoutRecoveryParams) ([]db.DiningSession, error) {
			require.EqualValues(t, dineInCheckoutRecoveryBatchLimit, arg.Limit)
			require.WithinDuration(t, time.Now().Add(-DineInCheckoutRecoveryDelay), arg.OpenedBefore, 2*time.Second)
			return []db.DiningSession{session}, nil
		})
	store.EXPECT().
		CloseDiningSessionTx(gomock.Any(), db.CloseDiningSessionTxParams{
			ID:         session.ID,
			MerchantID: session.MerchantID,
		}).
		Return(db.CloseDiningSessionTxResult{Session: db.DiningSession{
			ID:         session.ID,
			MerchantID: session.MerchantID,
			Status:     "closed",
		}}, nil)

	scheduler.RunOnce()
}

func TestDineInCheckoutRecoveryScheduler_ContinuesAfterCloseError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	scheduler := NewDineInCheckoutRecoveryScheduler(store)

	first := db.DiningSession{ID: 201, MerchantID: 301, Status: "open"}
	second := db.DiningSession{ID: 202, MerchantID: 302, Status: "open"}

	store.EXPECT().
		ListPaidOpenDineInSessionsForCheckoutRecovery(gomock.Any(), gomock.Any()).
		Return([]db.DiningSession{first, second}, nil)
	gomock.InOrder(
		store.EXPECT().
			CloseDiningSessionTx(gomock.Any(), db.CloseDiningSessionTxParams{
				ID:         first.ID,
				MerchantID: first.MerchantID,
			}).
			Return(db.CloseDiningSessionTxResult{}, errors.New("transient database error")),
		store.EXPECT().
			CloseDiningSessionTx(gomock.Any(), db.CloseDiningSessionTxParams{
				ID:         second.ID,
				MerchantID: second.MerchantID,
			}).
			Return(db.CloseDiningSessionTxResult{Session: db.DiningSession{ID: second.ID, Status: "closed"}}, nil),
	)

	scheduler.RunOnce()
}

func TestDineInCheckoutRecoveryScheduler_RecordsCloseFailureMetrics(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	scheduler := NewDineInCheckoutRecoveryScheduler(store)

	first := db.DiningSession{ID: 301, MerchantID: 401, Status: "open"}
	second := db.DiningSession{ID: 302, MerchantID: 402, Status: "open"}

	beforeSuccess := prometheusCounterValue(t, "dine_in_checkout_recovery_scans_total", map[string]string{"result": "success"})
	beforeListed := prometheusCounterValue(t, "dine_in_checkout_recovery_sessions_total", map[string]string{"result": "listed"})
	beforeClosed := prometheusCounterValue(t, "dine_in_checkout_recovery_sessions_total", map[string]string{"result": "closed"})
	beforeFailed := prometheusCounterValue(t, "dine_in_checkout_recovery_sessions_total", map[string]string{"result": "close_failed"})

	store.EXPECT().
		ListPaidOpenDineInSessionsForCheckoutRecovery(gomock.Any(), gomock.Any()).
		Return([]db.DiningSession{first, second}, nil)
	gomock.InOrder(
		store.EXPECT().
			CloseDiningSessionTx(gomock.Any(), db.CloseDiningSessionTxParams{
				ID:         first.ID,
				MerchantID: first.MerchantID,
			}).
			Return(db.CloseDiningSessionTxResult{}, errors.New("transient database error")),
		store.EXPECT().
			CloseDiningSessionTx(gomock.Any(), db.CloseDiningSessionTxParams{
				ID:         second.ID,
				MerchantID: second.MerchantID,
			}).
			Return(db.CloseDiningSessionTxResult{Session: db.DiningSession{ID: second.ID, Status: "closed"}}, nil),
	)

	scheduler.RunOnce()

	require.Equal(t, beforeSuccess+1, prometheusCounterValue(t, "dine_in_checkout_recovery_scans_total", map[string]string{"result": "success"}))
	require.Equal(t, beforeListed+2, prometheusCounterValue(t, "dine_in_checkout_recovery_sessions_total", map[string]string{"result": "listed"}))
	require.Equal(t, beforeClosed+1, prometheusCounterValue(t, "dine_in_checkout_recovery_sessions_total", map[string]string{"result": "closed"}))
	require.Equal(t, beforeFailed+1, prometheusCounterValue(t, "dine_in_checkout_recovery_sessions_total", map[string]string{"result": "close_failed"}))
}

func TestDineInCheckoutRecoveryScheduler_ListErrorSkipsClose(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	scheduler := NewDineInCheckoutRecoveryScheduler(store)

	store.EXPECT().
		ListPaidOpenDineInSessionsForCheckoutRecovery(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("database unavailable"))

	scheduler.RunOnce()
}

func TestDineInCheckoutRecoveryScheduler_RecordsListFailureMetrics(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	scheduler := NewDineInCheckoutRecoveryScheduler(store)

	beforeListError := prometheusCounterValue(t, "dine_in_checkout_recovery_scans_total", map[string]string{"result": "list_error"})

	store.EXPECT().
		ListPaidOpenDineInSessionsForCheckoutRecovery(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("database unavailable"))

	scheduler.RunOnce()

	require.Equal(t, beforeListError+1, prometheusCounterValue(t, "dine_in_checkout_recovery_scans_total", map[string]string{"result": "list_error"}))
}

func prometheusCounterValue(t *testing.T, name string, labels map[string]string) float64 {
	t.Helper()

	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)

	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.GetMetric() {
			if metric.GetCounter() == nil {
				continue
			}
			matched := true
			for wantName, wantValue := range labels {
				found := false
				for _, label := range metric.GetLabel() {
					if label.GetName() == wantName && label.GetValue() == wantValue {
						found = true
						break
					}
				}
				if !found {
					matched = false
					break
				}
			}
			if !matched {
				continue
			}
			return metric.GetCounter().GetValue()
		}
	}
	return 0
}
