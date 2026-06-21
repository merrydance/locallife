package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type CreateBaofuWithdrawalOrderWithSubmittedCommandTxParams struct {
	WithdrawalOrder CreateBaofuWithdrawalOrderParams
	BusinessOwner   string
	SubmittedAt     time.Time
}

type CreateBaofuWithdrawalOrderWithSubmittedCommandTxResult struct {
	WithdrawalOrder  BaofuWithdrawalOrder
	SubmittedCommand ExternalPaymentCommand
}

type CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxParams struct {
	WithdrawalOrder            CreateBaofuWithdrawalOrderParams
	BusinessOwner              string
	SubmittedAt                time.Time
	ProviderAvailableAmountFen int64
	ProviderPendingAmountFen   int64
	ProviderLedgerAmountFen    int64
	ProviderFrozenAmountFen    int64
	ProviderBalanceObservedAt  time.Time
}

type CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxResult struct {
	WithdrawalOrder  BaofuWithdrawalOrder
	Reservation      BaofuWithdrawalReservation
	Guard            BaofuWithdrawalAccountGuard
	SubmittedCommand ExternalPaymentCommand
}

type ApplyBaofuWithdrawalTerminalStatusTxParams struct {
	WithdrawalOrderID int64
	Status            string
	BaofuWithdrawNo   pgtype.Text
	RawSnapshot       []byte
	ReleaseReason     pgtype.Text
}

type ApplyBaofuWithdrawalTerminalStatusTxResult struct {
	WithdrawalOrder BaofuWithdrawalOrder
	Reservation     BaofuWithdrawalReservation
	Guard           BaofuWithdrawalAccountGuard
	Applied         bool
}

func (store *SQLStore) CreateBaofuWithdrawalOrderWithSubmittedCommandTx(ctx context.Context, arg CreateBaofuWithdrawalOrderWithSubmittedCommandTxParams) (CreateBaofuWithdrawalOrderWithSubmittedCommandTxResult, error) {
	var result CreateBaofuWithdrawalOrderWithSubmittedCommandTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		withdrawalOrder, err := q.CreateBaofuWithdrawalOrder(ctx, arg.WithdrawalOrder)
		if err != nil {
			return err
		}
		result.WithdrawalOrder = withdrawalOrder

		command, err := createBaofuWithdrawSubmittedCommand(ctx, q, withdrawalOrder, arg.BusinessOwner, arg.SubmittedAt)
		if err != nil {
			return err
		}
		result.SubmittedCommand = command
		return nil
	})

	return result, err
}

func (store *SQLStore) CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTx(ctx context.Context, arg CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxParams) (CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxResult, error) {
	var result CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTxResult

	submittedAt := normalizedBaofuWithdrawalTxTime(arg.SubmittedAt)
	observedAt := normalizedBaofuWithdrawalTxTime(arg.ProviderBalanceObservedAt)
	if arg.ProviderBalanceObservedAt.IsZero() {
		observedAt = submittedAt
	}

	err := store.execTx(ctx, func(q *Queries) error {
		if err := verifyBaofuWithdrawalAccountBindingOwner(ctx, q, arg.WithdrawalOrder.OwnerType, arg.WithdrawalOrder.OwnerID, arg.WithdrawalOrder.AccountBindingID); err != nil {
			return err
		}

		if _, err := q.UpsertBaofuWithdrawalAccountGuardBalance(ctx, UpsertBaofuWithdrawalAccountGuardBalanceParams{
			OwnerType:                  arg.WithdrawalOrder.OwnerType,
			OwnerID:                    arg.WithdrawalOrder.OwnerID,
			AccountBindingID:           arg.WithdrawalOrder.AccountBindingID,
			ProviderAvailableAmountFen: arg.ProviderAvailableAmountFen,
			ProviderPendingAmountFen:   arg.ProviderPendingAmountFen,
			ProviderLedgerAmountFen:    arg.ProviderLedgerAmountFen,
			ProviderFrozenAmountFen:    arg.ProviderFrozenAmountFen,
			ProviderBalanceObservedAt:  pgtype.Timestamptz{Time: observedAt, Valid: true},
		}); err != nil {
			return fmt.Errorf("upsert baofu withdrawal guard balance: %w", err)
		}

		guard, err := q.GetBaofuWithdrawalAccountGuardByOwnerForUpdate(ctx, GetBaofuWithdrawalAccountGuardByOwnerForUpdateParams{
			OwnerType:        arg.WithdrawalOrder.OwnerType,
			OwnerID:          arg.WithdrawalOrder.OwnerID,
			AccountBindingID: arg.WithdrawalOrder.AccountBindingID,
		})
		if err != nil {
			return fmt.Errorf("lock baofu withdrawal guard: %w", err)
		}

		withdrawalOrder, err := q.CreateBaofuWithdrawalOrder(ctx, arg.WithdrawalOrder)
		if err != nil {
			return fmt.Errorf("create baofu withdrawal order: %w", err)
		}
		result.WithdrawalOrder = withdrawalOrder

		reservation, err := q.CreateBaofuWithdrawalReservation(ctx, CreateBaofuWithdrawalReservationParams{
			WithdrawalOrderID: withdrawalOrder.ID,
			OwnerType:         withdrawalOrder.OwnerType,
			OwnerID:           withdrawalOrder.OwnerID,
			AccountBindingID:  withdrawalOrder.AccountBindingID,
			AmountFen:         withdrawalOrder.Amount,
			ReservedAt:        submittedAt,
		})
		if err != nil {
			return fmt.Errorf("create baofu withdrawal reservation: %w", err)
		}
		result.Reservation = reservation

		guard, err = q.ReserveBaofuWithdrawalAccountGuardAmount(ctx, ReserveBaofuWithdrawalAccountGuardAmountParams{
			ID:        guard.ID,
			AmountFen: withdrawalOrder.Amount,
		})
		if err != nil {
			if errors.Is(err, ErrRecordNotFound) {
				return ErrBaofuWithdrawalInsufficientReservedBalance
			}
			return fmt.Errorf("reserve baofu withdrawal guard: %w", err)
		}
		result.Guard = guard

		command, err := createBaofuWithdrawSubmittedCommand(ctx, q, withdrawalOrder, arg.BusinessOwner, submittedAt)
		if err != nil {
			return err
		}
		result.SubmittedCommand = command
		return nil
	})

	return result, err
}

func (store *SQLStore) ApplyBaofuWithdrawalTerminalStatusTx(ctx context.Context, arg ApplyBaofuWithdrawalTerminalStatusTxParams) (ApplyBaofuWithdrawalTerminalStatusTxResult, error) {
	var result ApplyBaofuWithdrawalTerminalStatusTxResult

	if !isTerminalBaofuWithdrawalOrderStatus(arg.Status) {
		return result, ErrBaofuWithdrawalTerminalReservationMismatch
	}

	settledAt := pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	releaseReason := arg.ReleaseReason
	if strings.TrimSpace(releaseReason.String) == "" {
		releaseReason = pgtype.Text{String: defaultBaofuWithdrawalReleaseReason(arg.Status), Valid: true}
	}

	err := store.execTx(ctx, func(q *Queries) error {
		withdrawalOrder, err := q.GetBaofuWithdrawalOrderForUpdate(ctx, arg.WithdrawalOrderID)
		if err != nil {
			return fmt.Errorf("lock baofu withdrawal order: %w", err)
		}
		result.WithdrawalOrder = withdrawalOrder

		if err := verifyBaofuWithdrawalAccountBindingOwner(ctx, q, withdrawalOrder.OwnerType, withdrawalOrder.OwnerID, withdrawalOrder.AccountBindingID); err != nil {
			return err
		}

		reservation, err := q.GetBaofuWithdrawalReservationByOrderIDForUpdate(ctx, withdrawalOrder.ID)
		if err != nil {
			return fmt.Errorf("lock baofu withdrawal reservation: %w", err)
		}
		result.Reservation = reservation

		if !baofuWithdrawalReservationMatchesOrder(withdrawalOrder, reservation) {
			return ErrBaofuWithdrawalTerminalReservationMismatch
		}

		guard, err := q.GetBaofuWithdrawalAccountGuardByOwnerForUpdate(ctx, GetBaofuWithdrawalAccountGuardByOwnerForUpdateParams{
			OwnerType:        withdrawalOrder.OwnerType,
			OwnerID:          withdrawalOrder.OwnerID,
			AccountBindingID: withdrawalOrder.AccountBindingID,
		})
		if err != nil {
			return fmt.Errorf("lock baofu withdrawal guard: %w", err)
		}
		result.Guard = guard

		if isTerminalBaofuWithdrawalOrderStatus(withdrawalOrder.Status) {
			if withdrawalOrder.Status == arg.Status && baofuWithdrawalTerminalMatchesReservation(withdrawalOrder.Status, reservation.Status) {
				result.Applied = false
				return nil
			}
			return ErrBaofuWithdrawalTerminalReservationMismatch
		}

		if withdrawalOrder.Status != BaofuWithdrawalStatusProcessing || reservation.Status != BaofuWithdrawalReservationStatusReserved {
			return ErrBaofuWithdrawalTerminalReservationMismatch
		}

		withdrawalOrder, err = q.UpdateBaofuWithdrawalOrderStatus(ctx, UpdateBaofuWithdrawalOrderStatusParams{
			ID:              withdrawalOrder.ID,
			Status:          arg.Status,
			BaofuWithdrawNo: arg.BaofuWithdrawNo,
			RawSnapshot:     arg.RawSnapshot,
		})
		if err != nil {
			if errors.Is(err, ErrRecordNotFound) {
				return ErrBaofuWithdrawalTerminalReservationMismatch
			}
			return fmt.Errorf("update baofu withdrawal terminal status: %w", err)
		}
		result.WithdrawalOrder = withdrawalOrder

		switch arg.Status {
		case BaofuWithdrawalStatusSucceeded:
			reservation, err = q.ConsumeBaofuWithdrawalReservation(ctx, ConsumeBaofuWithdrawalReservationParams{
				ID:         reservation.ID,
				ConsumedAt: settledAt,
			})
			if err != nil {
				if errors.Is(err, ErrRecordNotFound) {
					return ErrBaofuWithdrawalTerminalReservationMismatch
				}
				return fmt.Errorf("consume baofu withdrawal reservation: %w", err)
			}
			guard, err = q.ConsumeBaofuWithdrawalAccountGuardAmount(ctx, ConsumeBaofuWithdrawalAccountGuardAmountParams{
				ID:        guard.ID,
				AmountFen: reservation.AmountFen,
			})
			if err != nil {
				if errors.Is(err, ErrRecordNotFound) {
					return ErrBaofuWithdrawalTerminalReservationMismatch
				}
				return fmt.Errorf("consume baofu withdrawal guard: %w", err)
			}
		case BaofuWithdrawalStatusFailed, BaofuWithdrawalStatusReturned:
			reservation, err = q.ReleaseBaofuWithdrawalReservation(ctx, ReleaseBaofuWithdrawalReservationParams{
				ID:            reservation.ID,
				ReleaseReason: releaseReason,
				ReleasedAt:    settledAt,
			})
			if err != nil {
				if errors.Is(err, ErrRecordNotFound) {
					return ErrBaofuWithdrawalTerminalReservationMismatch
				}
				return fmt.Errorf("release baofu withdrawal reservation: %w", err)
			}
			guard, err = q.ReleaseBaofuWithdrawalAccountGuardAmount(ctx, ReleaseBaofuWithdrawalAccountGuardAmountParams{
				ID:        guard.ID,
				AmountFen: reservation.AmountFen,
			})
			if err != nil {
				if errors.Is(err, ErrRecordNotFound) {
					return ErrBaofuWithdrawalTerminalReservationMismatch
				}
				return fmt.Errorf("release baofu withdrawal guard: %w", err)
			}
		default:
			return ErrBaofuWithdrawalTerminalReservationMismatch
		}

		result.Reservation = reservation
		result.Guard = guard
		result.Applied = true
		return nil
	})

	return result, err
}

func verifyBaofuWithdrawalAccountBindingOwner(ctx context.Context, q *Queries, ownerType string, ownerID int64, accountBindingID int64) error {
	binding, err := q.GetBaofuAccountBinding(ctx, accountBindingID)
	if err != nil {
		return fmt.Errorf("get baofu withdrawal account binding: %w", err)
	}
	if binding.OwnerType != ownerType || binding.OwnerID != ownerID {
		return ErrBaofuWithdrawalAccountBindingOwnerMismatch
	}
	return nil
}

func createBaofuWithdrawSubmittedCommand(ctx context.Context, q *Queries, withdrawalOrder BaofuWithdrawalOrder, businessOwner string, submittedAt time.Time) (ExternalPaymentCommand, error) {
	submittedAt = normalizedBaofuWithdrawalTxTime(submittedAt)
	command, err := q.CreateExternalPaymentCommand(ctx, CreateExternalPaymentCommandParams{
		Provider:           ExternalPaymentProviderBaofu,
		Channel:            PaymentChannelBaofuAggregate,
		Capability:         ExternalPaymentCapabilityBaofuWithdraw,
		CommandType:        ExternalPaymentCommandTypeCreateBaofuWithdraw,
		BusinessOwner:      strings.TrimSpace(businessOwner),
		BusinessObjectType: pgtype.Text{String: "baofu_withdrawal_order", Valid: true},
		BusinessObjectID:   pgtype.Int8{Int64: withdrawalOrder.ID, Valid: true},
		ExternalObjectType: ExternalPaymentObjectWithdraw,
		ExternalObjectKey:  strings.TrimSpace(withdrawalOrder.OutRequestNo),
		CommandStatus:      ExternalPaymentCommandStatusSubmitted,
		SubmittedAt:        submittedAt,
		ResponseSnapshot:   baofuWithdrawSubmittedCommandSnapshot(withdrawalOrder),
	})
	if err != nil {
		return ExternalPaymentCommand{}, fmt.Errorf("create baofu withdrawal submitted command: %w", err)
	}
	return command, nil
}

func normalizedBaofuWithdrawalTxTime(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC()
	}
	return value.UTC()
}

func isTerminalBaofuWithdrawalOrderStatus(status string) bool {
	switch status {
	case BaofuWithdrawalStatusSucceeded, BaofuWithdrawalStatusFailed, BaofuWithdrawalStatusReturned:
		return true
	default:
		return false
	}
}

func baofuWithdrawalTerminalMatchesReservation(orderStatus string, reservationStatus string) bool {
	switch orderStatus {
	case BaofuWithdrawalStatusSucceeded:
		return reservationStatus == BaofuWithdrawalReservationStatusConsumed
	case BaofuWithdrawalStatusFailed, BaofuWithdrawalStatusReturned:
		return reservationStatus == BaofuWithdrawalReservationStatusReleased
	default:
		return false
	}
}

func baofuWithdrawalReservationMatchesOrder(order BaofuWithdrawalOrder, reservation BaofuWithdrawalReservation) bool {
	return reservation.WithdrawalOrderID == order.ID &&
		reservation.OwnerType == order.OwnerType &&
		reservation.OwnerID == order.OwnerID &&
		reservation.AccountBindingID == order.AccountBindingID &&
		reservation.AmountFen == order.Amount
}

func defaultBaofuWithdrawalReleaseReason(status string) string {
	switch status {
	case BaofuWithdrawalStatusFailed:
		return BaofuWithdrawalReservationReleaseReasonFailed
	case BaofuWithdrawalStatusReturned:
		return BaofuWithdrawalReservationReleaseReasonReturned
	default:
		return BaofuWithdrawalReservationReleaseReasonRejected
	}
}

func baofuWithdrawSubmittedCommandSnapshot(order BaofuWithdrawalOrder) []byte {
	payload := map[string]any{
		"provider":                  ExternalPaymentProviderBaofu,
		"operation":                 "create_baofu_withdraw",
		"state":                     "submitted",
		"dispatch_mode":             "async_worker",
		"baofu_withdrawal_order_id": order.ID,
		"owner_type":                order.OwnerType,
		"owner_id":                  order.OwnerID,
		"out_request_no":            strings.TrimSpace(order.OutRequestNo),
		"amount":                    order.Amount,
		"status":                    order.Status,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return []byte(`{}`)
	}
	return data
}
