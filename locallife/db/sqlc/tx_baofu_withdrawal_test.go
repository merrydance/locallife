package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestCreateBaofuWithdrawalOrderWithSubmittedCommandTxStoresOrderAndSubmittedCommandAtomically(t *testing.T) {
	ctx := context.Background()
	ownerID := time.Now().UnixNano()
	binding, err := testStore.UpsertBaofuAccountBinding(ctx, UpsertBaofuAccountBindingParams{
		OwnerType:   BaofuAccountOwnerTypeMerchant,
		OwnerID:     ownerID,
		AccountType: BaofuAccountTypeBusiness,
		OpeningMode: BaofuAccountOpeningModeBusinessPublic,
		OpenState:   BaofuAccountOpenStateProcessing,
		RawSnapshot: []byte(`{}`),
	})
	require.NoError(t, err)

	result, err := testStore.CreateBaofuWithdrawalOrderWithSubmittedCommandTx(ctx, CreateBaofuWithdrawalOrderWithSubmittedCommandTxParams{
		WithdrawalOrder: CreateBaofuWithdrawalOrderParams{
			OwnerType:        BaofuAccountOwnerTypeMerchant,
			OwnerID:          ownerID,
			AccountBindingID: binding.ID,
			OutRequestNo:     "WD_TX_" + util.RandomString(16),
			Amount:           1200,
			Status:           BaofuWithdrawalStatusProcessing,
			RawSnapshot:      []byte(`{"state":"submitted"}`),
			IdempotencyKey: pgtype.Text{
				String: "withdraw-tx-" + util.RandomString(12),
				Valid:  true,
			},
			IdempotencyRequestHash: pgtype.Text{
				String: "sha256:" + util.RandomString(64),
				Valid:  true,
			},
		},
		BusinessOwner: ExternalPaymentBusinessOwnerMerchantFunds,
		SubmittedAt:   time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.Equal(t, BaofuWithdrawalStatusProcessing, result.WithdrawalOrder.Status)
	require.Equal(t, ExternalPaymentCommandStatusSubmitted, result.SubmittedCommand.CommandStatus)
	require.Equal(t, ExternalPaymentProviderBaofu, result.SubmittedCommand.Provider)
	require.Equal(t, PaymentChannelBaofuAggregate, result.SubmittedCommand.Channel)
	require.Equal(t, ExternalPaymentCapabilityBaofuWithdraw, result.SubmittedCommand.Capability)
	require.Equal(t, result.WithdrawalOrder.ID, result.SubmittedCommand.BusinessObjectID.Int64)
	require.Equal(t, result.WithdrawalOrder.OutRequestNo, result.SubmittedCommand.ExternalObjectKey)
	require.False(t, result.SubmittedCommand.SubmittedAt.IsZero())

	loaded, err := testStore.GetExternalPaymentCommandByExternalObject(ctx, GetExternalPaymentCommandByExternalObjectParams{
		Provider:           ExternalPaymentProviderBaofu,
		Channel:            PaymentChannelBaofuAggregate,
		Capability:         ExternalPaymentCapabilityBaofuWithdraw,
		CommandType:        ExternalPaymentCommandTypeCreateBaofuWithdraw,
		ExternalObjectType: ExternalPaymentObjectWithdraw,
		ExternalObjectKey:  result.WithdrawalOrder.OutRequestNo,
	})
	require.NoError(t, err)
	require.Equal(t, ExternalPaymentCommandStatusSubmitted, loaded.CommandStatus)
	require.Equal(t, result.SubmittedCommand.ID, loaded.ID)
}
