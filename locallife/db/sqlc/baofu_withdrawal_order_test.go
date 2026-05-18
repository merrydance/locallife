package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func createRandomBaofuWithdrawalOrder(t *testing.T) BaofuWithdrawalOrder {
	t.Helper()
	ownerID := time.Now().UnixNano()
	binding, err := testStore.UpsertBaofuAccountBinding(context.Background(), UpsertBaofuAccountBindingParams{
		OwnerType:   BaofuAccountOwnerTypeRider,
		OwnerID:     ownerID,
		AccountType: BaofuAccountTypePersonal,
		OpenState:   BaofuAccountOpenStateProcessing,
		RawSnapshot: []byte(`{}`),
	})
	require.NoError(t, err)

	order, err := testStore.CreateBaofuWithdrawalOrder(context.Background(), CreateBaofuWithdrawalOrderParams{
		OwnerType:        BaofuAccountOwnerTypeRider,
		OwnerID:          ownerID,
		AccountBindingID: binding.ID,
		OutRequestNo:     "WD_GUARD_" + util.RandomString(16),
		Amount:           1000,
		Status:           BaofuWithdrawalStatusProcessing,
		RawSnapshot:      []byte(`{"state":"processing"}`),
	})
	require.NoError(t, err)
	require.Equal(t, BaofuWithdrawalStatusProcessing, order.Status)
	return order
}

func TestUpdateBaofuWithdrawalOrderStatusDoesNotRegressTerminal(t *testing.T) {
	order := createRandomBaofuWithdrawalOrder(t)

	succeeded, err := testStore.UpdateBaofuWithdrawalOrderStatus(context.Background(), UpdateBaofuWithdrawalOrderStatusParams{
		ID:              order.ID,
		Status:          BaofuWithdrawalStatusSucceeded,
		BaofuWithdrawNo: pgtype.Text{String: "BFWD_" + util.RandomString(12), Valid: true},
		RawSnapshot:     []byte(`{"state":"succeeded"}`),
	})
	require.NoError(t, err)
	require.Equal(t, BaofuWithdrawalStatusSucceeded, succeeded.Status)
	require.True(t, succeeded.FinishedAt.Valid)

	_, err = testStore.UpdateBaofuWithdrawalOrderStatus(context.Background(), UpdateBaofuWithdrawalOrderStatusParams{
		ID:              order.ID,
		Status:          BaofuWithdrawalStatusFailed,
		BaofuWithdrawNo: pgtype.Text{String: "BFWD_LATE_FAIL", Valid: true},
		RawSnapshot:     []byte(`{"state":"failed"}`),
	})
	require.ErrorIs(t, err, ErrRecordNotFound)

	current, err := testStore.GetBaofuWithdrawalOrder(context.Background(), order.ID)
	require.NoError(t, err)
	require.Equal(t, BaofuWithdrawalStatusSucceeded, current.Status)
	require.Equal(t, succeeded.BaofuWithdrawNo.String, current.BaofuWithdrawNo.String)
}
