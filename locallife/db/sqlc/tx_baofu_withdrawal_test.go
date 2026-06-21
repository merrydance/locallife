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

func TestCreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxReservesAvailableBalance(t *testing.T) {
	ctx := context.Background()
	ownerID := time.Now().UnixNano()
	binding := createActiveBaofuWithdrawalBindingForTest(t, ctx, BaofuAccountOwnerTypeMerchant, ownerID, BaofuAccountTypeBusiness, BaofuAccountOpeningModeBusinessPublic)

	result, err := testStore.CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTx(ctx, CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxParams{
		WithdrawalOrder: CreateBaofuWithdrawalOrderParams{
			OwnerType:        BaofuAccountOwnerTypeMerchant,
			OwnerID:          ownerID,
			AccountBindingID: binding.ID,
			OutRequestNo:     "WD_RESERVE_" + util.RandomString(16),
			Amount:           1200,
			Status:           BaofuWithdrawalStatusProcessing,
			RawSnapshot:      []byte(`{"state":"submitted"}`),
			IdempotencyKey: pgtype.Text{
				String: "withdraw-reserve-" + util.RandomString(12),
				Valid:  true,
			},
			IdempotencyRequestHash: pgtype.Text{
				String: "sha256:" + util.RandomString(64),
				Valid:  true,
			},
		},
		BusinessOwner:              ExternalPaymentBusinessOwnerMerchantFunds,
		SubmittedAt:                time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC),
		ProviderAvailableAmountFen: 2000,
		ProviderPendingAmountFen:   300,
		ProviderLedgerAmountFen:    2600,
		ProviderFrozenAmountFen:    300,
		ProviderBalanceObservedAt:  time.Date(2026, 6, 21, 9, 59, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.Equal(t, BaofuWithdrawalStatusProcessing, result.WithdrawalOrder.Status)
	require.Equal(t, ExternalPaymentCommandStatusSubmitted, result.SubmittedCommand.CommandStatus)
	require.Equal(t, result.WithdrawalOrder.ID, result.Reservation.WithdrawalOrderID)
	require.Equal(t, BaofuWithdrawalReservationStatusReserved, result.Reservation.Status)
	require.Equal(t, int64(1200), result.Reservation.AmountFen)
	require.Equal(t, int64(1200), result.Guard.ReservedAmountFen)
	require.Equal(t, int64(2000), result.Guard.ProviderAvailableAmountFen)
	require.Equal(t, binding.ID, result.Guard.AccountBindingID)

	loadedReservation, err := testStore.GetBaofuWithdrawalReservationByOrderID(ctx, result.WithdrawalOrder.ID)
	require.NoError(t, err)
	require.Equal(t, result.Reservation.ID, loadedReservation.ID)

	_, err = testStore.CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTx(ctx, CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxParams{
		WithdrawalOrder: CreateBaofuWithdrawalOrderParams{
			OwnerType:        BaofuAccountOwnerTypeMerchant,
			OwnerID:          ownerID,
			AccountBindingID: binding.ID,
			OutRequestNo:     "WD_RESERVE_" + util.RandomString(16),
			Amount:           900,
			Status:           BaofuWithdrawalStatusProcessing,
			RawSnapshot:      []byte(`{"state":"submitted"}`),
			IdempotencyKey: pgtype.Text{
				String: "withdraw-reserve-" + util.RandomString(12),
				Valid:  true,
			},
			IdempotencyRequestHash: pgtype.Text{
				String: "sha256:" + util.RandomString(64),
				Valid:  true,
			},
		},
		BusinessOwner:              ExternalPaymentBusinessOwnerMerchantFunds,
		SubmittedAt:                time.Date(2026, 6, 21, 10, 1, 0, 0, time.UTC),
		ProviderAvailableAmountFen: 2000,
		ProviderBalanceObservedAt:  time.Date(2026, 6, 21, 10, 0, 30, 0, time.UTC),
	})
	require.ErrorIs(t, err, ErrBaofuWithdrawalInsufficientReservedBalance)

	guard, err := testStore.GetBaofuWithdrawalAccountGuardByOwner(ctx, GetBaofuWithdrawalAccountGuardByOwnerParams{
		OwnerType:        BaofuAccountOwnerTypeMerchant,
		OwnerID:          ownerID,
		AccountBindingID: binding.ID,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1200), guard.ReservedAmountFen)
}

func TestApplyBaofuWithdrawalTerminalStatusTxConsumesAndReleasesReservationIdempotently(t *testing.T) {
	ctx := context.Background()
	ownerID := time.Now().UnixNano()
	binding := createActiveBaofuWithdrawalBindingForTest(t, ctx, BaofuAccountOwnerTypeRider, ownerID, BaofuAccountTypePersonal, BaofuAccountOpeningModePersonal)

	createResult, err := testStore.CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTx(ctx, CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxParams{
		WithdrawalOrder: CreateBaofuWithdrawalOrderParams{
			OwnerType:        BaofuAccountOwnerTypeRider,
			OwnerID:          ownerID,
			AccountBindingID: binding.ID,
			OutRequestNo:     "WD_SETTLE_" + util.RandomString(16),
			Amount:           1500,
			Status:           BaofuWithdrawalStatusProcessing,
			RawSnapshot:      []byte(`{"state":"submitted"}`),
			IdempotencyKey: pgtype.Text{
				String: "withdraw-settle-" + util.RandomString(12),
				Valid:  true,
			},
			IdempotencyRequestHash: pgtype.Text{
				String: "sha256:" + util.RandomString(64),
				Valid:  true,
			},
		},
		BusinessOwner:              ExternalPaymentBusinessOwnerRiderIncome,
		SubmittedAt:                time.Date(2026, 6, 21, 10, 2, 0, 0, time.UTC),
		ProviderAvailableAmountFen: 3000,
		ProviderBalanceObservedAt:  time.Date(2026, 6, 21, 10, 1, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	applied, err := testStore.ApplyBaofuWithdrawalTerminalStatusTx(ctx, ApplyBaofuWithdrawalTerminalStatusTxParams{
		WithdrawalOrderID: createResult.WithdrawalOrder.ID,
		Status:            BaofuWithdrawalStatusSucceeded,
		BaofuWithdrawNo:   pgtype.Text{String: "BFWD_SUCCESS_" + util.RandomString(8), Valid: true},
		RawSnapshot:       []byte(`{"state":"succeeded"}`),
	})
	require.NoError(t, err)
	require.True(t, applied.Applied)
	require.Equal(t, BaofuWithdrawalStatusSucceeded, applied.WithdrawalOrder.Status)
	require.Equal(t, BaofuWithdrawalReservationStatusConsumed, applied.Reservation.Status)
	require.Equal(t, int64(0), applied.Guard.ReservedAmountFen)
	require.Equal(t, int64(1500), applied.Guard.ConsumedWithdrawAmountFen)

	replayed, err := testStore.ApplyBaofuWithdrawalTerminalStatusTx(ctx, ApplyBaofuWithdrawalTerminalStatusTxParams{
		WithdrawalOrderID: createResult.WithdrawalOrder.ID,
		Status:            BaofuWithdrawalStatusSucceeded,
		BaofuWithdrawNo:   applied.WithdrawalOrder.BaofuWithdrawNo,
		RawSnapshot:       []byte(`{"state":"succeeded","duplicate":true}`),
	})
	require.NoError(t, err)
	require.False(t, replayed.Applied)
	require.Equal(t, BaofuWithdrawalReservationStatusConsumed, replayed.Reservation.Status)
	require.Equal(t, int64(0), replayed.Guard.ReservedAmountFen)
	require.Equal(t, int64(1500), replayed.Guard.ConsumedWithdrawAmountFen)

	_, err = testStore.ApplyBaofuWithdrawalTerminalStatusTx(ctx, ApplyBaofuWithdrawalTerminalStatusTxParams{
		WithdrawalOrderID: createResult.WithdrawalOrder.ID,
		Status:            BaofuWithdrawalStatusFailed,
		RawSnapshot:       []byte(`{"state":"late_failed"}`),
		ReleaseReason:     pgtype.Text{String: BaofuWithdrawalReservationReleaseReasonFailed, Valid: true},
	})
	require.ErrorIs(t, err, ErrBaofuWithdrawalTerminalReservationMismatch)
}

func TestApplyBaofuWithdrawalTerminalStatusTxReleasesReservedBalanceIdempotently(t *testing.T) {
	ctx := context.Background()
	ownerID := time.Now().UnixNano()
	binding := createActiveBaofuWithdrawalBindingForTest(t, ctx, BaofuAccountOwnerTypeMerchant, ownerID, BaofuAccountTypeBusiness, BaofuAccountOpeningModeBusinessPublic)

	createResult, err := testStore.CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTx(ctx, CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxParams{
		WithdrawalOrder: CreateBaofuWithdrawalOrderParams{
			OwnerType:        BaofuAccountOwnerTypeMerchant,
			OwnerID:          ownerID,
			AccountBindingID: binding.ID,
			OutRequestNo:     "WD_RELEASE_" + util.RandomString(16),
			Amount:           1800,
			Status:           BaofuWithdrawalStatusProcessing,
			RawSnapshot:      []byte(`{"state":"submitted"}`),
			IdempotencyKey: pgtype.Text{
				String: "withdraw-release-" + util.RandomString(12),
				Valid:  true,
			},
			IdempotencyRequestHash: pgtype.Text{
				String: "sha256:" + util.RandomString(64),
				Valid:  true,
			},
		},
		BusinessOwner:              ExternalPaymentBusinessOwnerMerchantFunds,
		SubmittedAt:                time.Date(2026, 6, 21, 11, 0, 0, 0, time.UTC),
		ProviderAvailableAmountFen: 3000,
		ProviderBalanceObservedAt:  time.Date(2026, 6, 21, 10, 59, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.Equal(t, int64(1800), createResult.Guard.ReservedAmountFen)

	applied, err := testStore.ApplyBaofuWithdrawalTerminalStatusTx(ctx, ApplyBaofuWithdrawalTerminalStatusTxParams{
		WithdrawalOrderID: createResult.WithdrawalOrder.ID,
		Status:            BaofuWithdrawalStatusFailed,
		RawSnapshot:       []byte(`{"state":"failed"}`),
		ReleaseReason:     pgtype.Text{String: BaofuWithdrawalReservationReleaseReasonFailed, Valid: true},
	})
	require.NoError(t, err)
	require.True(t, applied.Applied)
	require.Equal(t, BaofuWithdrawalStatusFailed, applied.WithdrawalOrder.Status)
	require.Equal(t, BaofuWithdrawalReservationStatusReleased, applied.Reservation.Status)
	require.Equal(t, BaofuWithdrawalReservationReleaseReasonFailed, applied.Reservation.ReleaseReason.String)
	require.Equal(t, int64(0), applied.Guard.ReservedAmountFen)
	require.Equal(t, int64(0), applied.Guard.ConsumedWithdrawAmountFen)

	replayed, err := testStore.ApplyBaofuWithdrawalTerminalStatusTx(ctx, ApplyBaofuWithdrawalTerminalStatusTxParams{
		WithdrawalOrderID: createResult.WithdrawalOrder.ID,
		Status:            BaofuWithdrawalStatusFailed,
		RawSnapshot:       []byte(`{"state":"failed","duplicate":true}`),
		ReleaseReason:     pgtype.Text{String: BaofuWithdrawalReservationReleaseReasonFailed, Valid: true},
	})
	require.NoError(t, err)
	require.False(t, replayed.Applied)
	require.Equal(t, BaofuWithdrawalReservationStatusReleased, replayed.Reservation.Status)
	require.Equal(t, int64(0), replayed.Guard.ReservedAmountFen)
	require.Equal(t, int64(0), replayed.Guard.ConsumedWithdrawAmountFen)
}

func TestCreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxKeepsNewestProviderBalanceSnapshot(t *testing.T) {
	ctx := context.Background()
	ownerID := time.Now().UnixNano()
	binding := createActiveBaofuWithdrawalBindingForTest(t, ctx, BaofuAccountOwnerTypeMerchant, ownerID, BaofuAccountTypeBusiness, BaofuAccountOpeningModeBusinessPublic)

	_, err := testStore.CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTx(ctx, CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxParams{
		WithdrawalOrder: CreateBaofuWithdrawalOrderParams{
			OwnerType:        BaofuAccountOwnerTypeMerchant,
			OwnerID:          ownerID,
			AccountBindingID: binding.ID,
			OutRequestNo:     "WD_NEWER_" + util.RandomString(16),
			Amount:           400,
			Status:           BaofuWithdrawalStatusProcessing,
			RawSnapshot:      []byte(`{"state":"submitted"}`),
			IdempotencyKey: pgtype.Text{
				String: "withdraw-newer-" + util.RandomString(12),
				Valid:  true,
			},
			IdempotencyRequestHash: pgtype.Text{
				String: "sha256:" + util.RandomString(64),
				Valid:  true,
			},
		},
		BusinessOwner:              ExternalPaymentBusinessOwnerMerchantFunds,
		SubmittedAt:                time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC),
		ProviderAvailableAmountFen: 1000,
		ProviderBalanceObservedAt:  time.Date(2026, 6, 21, 11, 59, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	_, err = testStore.CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTx(ctx, CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxParams{
		WithdrawalOrder: CreateBaofuWithdrawalOrderParams{
			OwnerType:        BaofuAccountOwnerTypeMerchant,
			OwnerID:          ownerID,
			AccountBindingID: binding.ID,
			OutRequestNo:     "WD_OLDER_" + util.RandomString(16),
			Amount:           300,
			Status:           BaofuWithdrawalStatusProcessing,
			RawSnapshot:      []byte(`{"state":"submitted"}`),
			IdempotencyKey: pgtype.Text{
				String: "withdraw-older-" + util.RandomString(12),
				Valid:  true,
			},
			IdempotencyRequestHash: pgtype.Text{
				String: "sha256:" + util.RandomString(64),
				Valid:  true,
			},
		},
		BusinessOwner:              ExternalPaymentBusinessOwnerMerchantFunds,
		SubmittedAt:                time.Date(2026, 6, 21, 12, 1, 0, 0, time.UTC),
		ProviderAvailableAmountFen: 5000,
		ProviderBalanceObservedAt:  time.Date(2026, 6, 21, 11, 58, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	guard, err := testStore.GetBaofuWithdrawalAccountGuardByOwner(ctx, GetBaofuWithdrawalAccountGuardByOwnerParams{
		OwnerType:        BaofuAccountOwnerTypeMerchant,
		OwnerID:          ownerID,
		AccountBindingID: binding.ID,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1000), guard.ProviderAvailableAmountFen)
	require.Equal(t, int64(700), guard.ReservedAmountFen)
	require.Equal(t, time.Date(2026, 6, 21, 11, 59, 0, 0, time.UTC), guard.ProviderBalanceObservedAt.Time.UTC())
}

func TestCreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxRejectsBindingOwnerMismatch(t *testing.T) {
	ctx := context.Background()
	ownerID := time.Now().UnixNano()
	otherOwnerID := ownerID + 1
	otherBinding := createActiveBaofuWithdrawalBindingForTest(t, ctx, BaofuAccountOwnerTypeMerchant, otherOwnerID, BaofuAccountTypeBusiness, BaofuAccountOpeningModeBusinessPublic)

	_, err := testStore.CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTx(ctx, CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxParams{
		WithdrawalOrder: CreateBaofuWithdrawalOrderParams{
			OwnerType:        BaofuAccountOwnerTypeMerchant,
			OwnerID:          ownerID,
			AccountBindingID: otherBinding.ID,
			OutRequestNo:     "WD_SCOPE_" + util.RandomString(16),
			Amount:           400,
			Status:           BaofuWithdrawalStatusProcessing,
			RawSnapshot:      []byte(`{"state":"submitted"}`),
			IdempotencyKey: pgtype.Text{
				String: "withdraw-scope-" + util.RandomString(12),
				Valid:  true,
			},
			IdempotencyRequestHash: pgtype.Text{
				String: "sha256:" + util.RandomString(64),
				Valid:  true,
			},
		},
		BusinessOwner:              ExternalPaymentBusinessOwnerMerchantFunds,
		SubmittedAt:                time.Date(2026, 6, 21, 12, 30, 0, 0, time.UTC),
		ProviderAvailableAmountFen: 1000,
		ProviderBalanceObservedAt:  time.Date(2026, 6, 21, 12, 29, 0, 0, time.UTC),
	})
	require.ErrorIs(t, err, ErrBaofuWithdrawalAccountBindingOwnerMismatch)

	_, guardErr := testStore.GetBaofuWithdrawalAccountGuardByOwner(ctx, GetBaofuWithdrawalAccountGuardByOwnerParams{
		OwnerType:        BaofuAccountOwnerTypeMerchant,
		OwnerID:          ownerID,
		AccountBindingID: otherBinding.ID,
	})
	require.ErrorIs(t, guardErr, ErrRecordNotFound)
}

func createActiveBaofuWithdrawalBindingForTest(t *testing.T, ctx context.Context, ownerType string, ownerID int64, accountType string, openingMode string) BaofuAccountBinding {
	t.Helper()

	binding, err := testStore.UpsertBaofuAccountBinding(ctx, UpsertBaofuAccountBindingParams{
		OwnerType:   ownerType,
		OwnerID:     ownerID,
		AccountType: accountType,
		OpeningMode: openingMode,
		OpenState:   BaofuAccountOpenStateProcessing,
		RawSnapshot: []byte(`{}`),
	})
	require.NoError(t, err)

	active, err := testStore.MarkBaofuAccountBindingActive(ctx, MarkBaofuAccountBindingActiveParams{
		ID:           binding.ID,
		ContractNo:   pgtype.Text{String: "CP_WD_" + util.RandomString(16), Valid: true},
		SharingMerID: pgtype.Text{String: "CP_SHARE_WD_" + util.RandomString(12), Valid: true},
		RawSnapshot:  []byte(`{"state":"active"}`),
	})
	require.NoError(t, err)
	return active
}
