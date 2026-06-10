package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestMerchantOpenStatusScheduler_RunOnce(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().AutoCloseMerchants(gomock.Any()).DoAndReturn(
		func(ctx context.Context) ([]int64, error) {
			require.NotNil(t, ctx)
			return nil, nil
		},
	)
	store.EXPECT().SyncMerchantOpenStatusByBusinessHours(gomock.Any()).DoAndReturn(
		func(ctx context.Context) ([]int64, error) {
			require.NotNil(t, ctx)
			return []int64{1001}, nil
		},
	)
	store.EXPECT().ClearExpiredMerchantManualOpenStatusOverrides(gomock.Any()).DoAndReturn(
		func(ctx context.Context) (int64, error) {
			require.NotNil(t, ctx)
			return int64(0), nil
		},
	)
	store.EXPECT().GetMerchantIsOpen(gomock.Any(), int64(1001)).DoAndReturn(
		func(ctx context.Context, merchantID int64) (db.GetMerchantIsOpenRow, error) {
			require.NotNil(t, ctx)
			require.Equal(t, int64(1001), merchantID)
			return db.GetMerchantIsOpenRow{ID: merchantID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}}, nil
		},
	)

	publisher := &testMerchantStatusChangePublisher{}

	s := NewMerchantOpenStatusScheduler(store, publisher)
	s.RunOnce()
	require.Equal(t, int64(1001), publisher.merchantID)
	require.True(t, publisher.isOpen)
	require.Equal(t, "business_hours", publisher.source)
}

type testMerchantStatusChangePublisher struct {
	merchantID int64
	isOpen     bool
	source     string
}

func (p *testMerchantStatusChangePublisher) PublishMerchantStatusChange(_ context.Context, merchantID int64, isOpen bool, _ *time.Time, source string) error {
	p.merchantID = merchantID
	p.isOpen = isOpen
	p.source = source
	return nil
}

func TestMerchantOpenStatusScheduler_RunOnceAutoClosesExpiredManualStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().AutoCloseMerchants(gomock.Any()).DoAndReturn(
		func(ctx context.Context) ([]int64, error) {
			require.NotNil(t, ctx)
			return []int64{1002}, nil
		},
	)
	store.EXPECT().GetMerchantIsOpen(gomock.Any(), int64(1002)).DoAndReturn(
		func(ctx context.Context, merchantID int64) (db.GetMerchantIsOpenRow, error) {
			require.NotNil(t, ctx)
			require.Equal(t, int64(1002), merchantID)
			return db.GetMerchantIsOpenRow{ID: merchantID, IsOpen: false}, nil
		},
	)
	store.EXPECT().SyncMerchantOpenStatusByBusinessHours(gomock.Any()).Return(nil, nil)
	store.EXPECT().ClearExpiredMerchantManualOpenStatusOverrides(gomock.Any()).Return(int64(0), nil)

	publisher := &testMerchantStatusChangePublisher{}

	s := NewMerchantOpenStatusScheduler(store, publisher)
	s.RunOnce()
	require.Equal(t, int64(1002), publisher.merchantID)
	require.False(t, publisher.isOpen)
	require.Equal(t, "auto_close", publisher.source)
}
