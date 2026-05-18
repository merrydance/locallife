package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestBaofuAccountBindingLifecycle(t *testing.T) {
	ctx := context.Background()
	ownerID := time.Now().UnixNano()
	outRequestNo := "OPEN" + util.RandomString(18)

	binding, err := testStore.UpsertBaofuAccountBinding(ctx, UpsertBaofuAccountBindingParams{
		OwnerType:             BaofuAccountOwnerTypeRider,
		OwnerID:               ownerID,
		AccountType:           BaofuAccountTypePersonal,
		LoginNo:               pgtype.Text{String: "rider-login", Valid: true},
		OpenState:             BaofuAccountOpenStateProcessing,
		LastOpenTransSerialNo: pgtype.Text{String: outRequestNo, Valid: true},
		RawSnapshot:           []byte(`{"status":"processing"}`),
	})
	require.NoError(t, err)
	require.Equal(t, BaofuAccountOpenStateProcessing, binding.OpenState)
	require.Equal(t, outRequestNo, binding.LastOpenTransSerialNo.String)

	active, err := testStore.MarkBaofuAccountBindingActive(ctx, MarkBaofuAccountBindingActiveParams{
		ID:           binding.ID,
		ContractNo:   pgtype.Text{String: "CP" + util.RandomString(16), Valid: true},
		SharingMerID: pgtype.Text{String: "CP_SHARE" + util.RandomString(8), Valid: true},
		RawSnapshot:  []byte(`{"status":"active"}`),
	})
	require.NoError(t, err)
	require.Equal(t, BaofuAccountOpenStateActive, active.OpenState)
	require.True(t, active.ContractNo.Valid)
	require.True(t, active.SharingMerID.Valid)

	byOwner, err := testStore.GetBaofuAccountBindingByOwner(ctx, GetBaofuAccountBindingByOwnerParams{
		OwnerType: BaofuAccountOwnerTypeRider,
		OwnerID:   ownerID,
	})
	require.NoError(t, err)
	require.Equal(t, active.ID, byOwner.ID)

	byContract, err := testStore.GetBaofuAccountBindingByContractNo(ctx, active.ContractNo)
	require.NoError(t, err)
	require.Equal(t, active.ID, byContract.ID)
}

func TestMarkBaofuAccountBindingActiveRequiresReceiver(t *testing.T) {
	binding, err := testStore.UpsertBaofuAccountBinding(context.Background(), UpsertBaofuAccountBindingParams{
		OwnerType:   BaofuAccountOwnerTypeMerchant,
		OwnerID:     time.Now().UnixNano(),
		AccountType: BaofuAccountTypeBusiness,
		OpenState:   BaofuAccountOpenStateProcessing,
		RawSnapshot: []byte(`{}`),
	})
	require.NoError(t, err)

	_, err = testStore.MarkBaofuAccountBindingActive(context.Background(), MarkBaofuAccountBindingActiveParams{
		ID:          binding.ID,
		RawSnapshot: []byte(`{}`),
	})
	require.Error(t, err)
	require.Equal(t, "23514", ErrorCode(err))
}

func TestMarkBaofuAccountBindingActiveRejectsContractOnlyReceiver(t *testing.T) {
	binding, err := testStore.UpsertBaofuAccountBinding(context.Background(), UpsertBaofuAccountBindingParams{
		OwnerType:   BaofuAccountOwnerTypeMerchant,
		OwnerID:     time.Now().UnixNano(),
		AccountType: BaofuAccountTypeBusiness,
		OpenState:   BaofuAccountOpenStateProcessing,
		RawSnapshot: []byte(`{}`),
	})
	require.NoError(t, err)

	_, err = testStore.MarkBaofuAccountBindingActive(context.Background(), MarkBaofuAccountBindingActiveParams{
		ID:          binding.ID,
		ContractNo:  pgtype.Text{String: "CONTRACT_ONLY", Valid: true},
		RawSnapshot: []byte(`{}`),
	})
	require.Error(t, err)
	require.Equal(t, "23514", ErrorCode(err))
}
