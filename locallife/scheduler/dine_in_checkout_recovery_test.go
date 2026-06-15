package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
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
