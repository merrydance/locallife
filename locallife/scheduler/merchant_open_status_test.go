package scheduler

import (
	"context"
	"testing"

	mockdb "github.com/merrydance/locallife/db/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestMerchantOpenStatusScheduler_RunOnce(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().SyncMerchantOpenStatusByBusinessHours(gomock.Any()).DoAndReturn(
		func(ctx context.Context) ([]int64, error) {
			require.NotNil(t, ctx)
			return []int64{1001}, nil
		},
	)

	s := NewMerchantOpenStatusScheduler(store)
	s.RunOnce()
}
