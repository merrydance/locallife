package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestListBaofuWithdrawalReservationDriftsFindsProcessingOrderWithoutReservedReservation(t *testing.T) {
	ctx := context.Background()
	ownerID := time.Now().UnixNano()
	binding := createActiveBaofuWithdrawalBindingForTest(t, ctx, BaofuAccountOwnerTypeMerchant, ownerID, BaofuAccountTypeBusiness, BaofuAccountOpeningModeBusinessPublic)

	order, err := testStore.CreateBaofuWithdrawalOrder(ctx, CreateBaofuWithdrawalOrderParams{
		OwnerType:        BaofuAccountOwnerTypeMerchant,
		OwnerID:          ownerID,
		AccountBindingID: binding.ID,
		OutRequestNo:     "WD_DRIFT_MISSING_" + util.RandomString(12),
		Amount:           1200,
		Status:           BaofuWithdrawalStatusProcessing,
		RawSnapshot:      []byte(`{"state":"legacy_processing"}`),
	})
	require.NoError(t, err)

	rows, err := testStore.ListBaofuWithdrawalReservationDrifts(ctx, ListBaofuWithdrawalReservationDriftsParams{
		OwnerType:  pgtype.Text{String: BaofuAccountOwnerTypeMerchant, Valid: true},
		OwnerID:    pgtype.Int8{Int64: ownerID, Valid: true},
		LimitCount: 20,
	})
	require.NoError(t, err)

	requireBaofuWithdrawalReservationDrift(t, rows, BaofuWithdrawalReservationDriftTypeProcessingMissingReservedReservation, order.ID, 0, 0)
}

func TestListBaofuWithdrawalReservationDriftsFindsTerminalOrderWithReservedReservation(t *testing.T) {
	ctx := context.Background()
	ownerID := time.Now().UnixNano()
	binding := createActiveBaofuWithdrawalBindingForTest(t, ctx, BaofuAccountOwnerTypeRider, ownerID, BaofuAccountTypePersonal, BaofuAccountOpeningModePersonal)

	created, err := testStore.CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTx(ctx, CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxParams{
		WithdrawalOrder: CreateBaofuWithdrawalOrderParams{
			OwnerType:        BaofuAccountOwnerTypeRider,
			OwnerID:          ownerID,
			AccountBindingID: binding.ID,
			OutRequestNo:     "WD_DRIFT_TERMINAL_" + util.RandomString(12),
			Amount:           1500,
			Status:           BaofuWithdrawalStatusProcessing,
			RawSnapshot:      []byte(`{"state":"submitted"}`),
			IdempotencyKey: pgtype.Text{
				String: "withdraw-drift-terminal-" + util.RandomString(12),
				Valid:  true,
			},
			IdempotencyRequestHash: pgtype.Text{
				String: "sha256:" + util.RandomString(64),
				Valid:  true,
			},
		},
		BusinessOwner:              ExternalPaymentBusinessOwnerRiderIncome,
		SubmittedAt:                time.Date(2026, 6, 21, 13, 0, 0, 0, time.UTC),
		ProviderAvailableAmountFen: 3000,
		ProviderBalanceObservedAt:  time.Date(2026, 6, 21, 12, 59, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	_, err = testStore.UpdateBaofuWithdrawalOrderStatus(ctx, UpdateBaofuWithdrawalOrderStatusParams{
		ID:              created.WithdrawalOrder.ID,
		Status:          BaofuWithdrawalStatusSucceeded,
		BaofuWithdrawNo: pgtype.Text{String: "BFWD_DRIFT_" + util.RandomString(10), Valid: true},
		RawSnapshot:     []byte(`{"state":"legacy_terminal"}`),
	})
	require.NoError(t, err)

	rows, err := testStore.ListBaofuWithdrawalReservationDrifts(ctx, ListBaofuWithdrawalReservationDriftsParams{
		OwnerType:  pgtype.Text{String: BaofuAccountOwnerTypeRider, Valid: true},
		OwnerID:    pgtype.Int8{Int64: ownerID, Valid: true},
		LimitCount: 20,
	})
	require.NoError(t, err)

	requireBaofuWithdrawalReservationDrift(t, rows, BaofuWithdrawalReservationDriftTypeTerminalReservedReservation, created.WithdrawalOrder.ID, created.Reservation.ID, 0)
}

func TestListBaofuWithdrawalReservationDriftsFindsGuardReservedMismatch(t *testing.T) {
	ctx := context.Background()
	ownerID := time.Now().UnixNano()
	binding := createActiveBaofuWithdrawalBindingForTest(t, ctx, BaofuAccountOwnerTypeOperator, ownerID, BaofuAccountTypeBusiness, BaofuAccountOpeningModeBusinessPublic)

	guard, err := testStore.UpsertBaofuWithdrawalAccountGuardBalance(ctx, UpsertBaofuWithdrawalAccountGuardBalanceParams{
		OwnerType:                  BaofuAccountOwnerTypeOperator,
		OwnerID:                    ownerID,
		AccountBindingID:           binding.ID,
		ProviderAvailableAmountFen: 4000,
		ProviderPendingAmountFen:   0,
		ProviderLedgerAmountFen:    4000,
		ProviderFrozenAmountFen:    0,
		ProviderBalanceObservedAt:  pgtype.Timestamptz{Time: time.Date(2026, 6, 21, 13, 30, 0, 0, time.UTC), Valid: true},
	})
	require.NoError(t, err)
	guard, err = testStore.ReserveBaofuWithdrawalAccountGuardAmount(ctx, ReserveBaofuWithdrawalAccountGuardAmountParams{
		ID:        guard.ID,
		AmountFen: 700,
	})
	require.NoError(t, err)

	rows, err := testStore.ListBaofuWithdrawalReservationDrifts(ctx, ListBaofuWithdrawalReservationDriftsParams{
		OwnerType:  pgtype.Text{String: BaofuAccountOwnerTypeOperator, Valid: true},
		OwnerID:    pgtype.Int8{Int64: ownerID, Valid: true},
		LimitCount: 20,
	})
	require.NoError(t, err)

	requireBaofuWithdrawalReservationDrift(t, rows, BaofuWithdrawalReservationDriftTypeGuardReservedMismatch, 0, 0, guard.ID)
}

func requireBaofuWithdrawalReservationDrift(t *testing.T, rows []ListBaofuWithdrawalReservationDriftsRow, driftType string, orderID int64, reservationID int64, guardID int64) {
	t.Helper()

	for _, row := range rows {
		if row.DriftType != driftType {
			continue
		}
		if orderID > 0 && row.WithdrawalOrderID != orderID {
			continue
		}
		if reservationID > 0 && row.ReservationID != reservationID {
			continue
		}
		if guardID > 0 && row.GuardID != guardID {
			continue
		}
		return
	}
	require.Failf(t, "missing baofu withdrawal reservation drift", "type=%s order_id=%d reservation_id=%d guard_id=%d rows=%+v", driftType, orderID, reservationID, guardID, rows)
}
