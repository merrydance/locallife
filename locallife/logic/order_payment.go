package logic

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

// OrderTotalsInput defines the input for computing totals.
type OrderTotalsInput struct {
	Subtotal            int64
	DiscountAmount      int64
	VoucherAmount       int64
	PackagingFee        int64
	DeliveryFee         int64
	DeliveryFeeDiscount int64
	DepositDeduction    int64
	MembershipBalance   int64
	UseBalance          bool
}

// OrderTotalsResult describes computed totals.
type OrderTotalsResult struct {
	TotalAmount int64
	BalancePaid int64
}

// ResolveReservationDepositDeduction returns the actual paid deposit that can be deducted from a reservation order.
func ResolveReservationDepositDeduction(ctx context.Context, store db.Store, reservation *db.TableReservation) (int64, error) {
	if reservation == nil || reservation.PaymentMode != paymentModeDeposit {
		return 0, nil
	}

	paymentOrder, err := store.GetLatestPaymentOrderByReservation(ctx, db.GetLatestPaymentOrderByReservationParams{
		ReservationID: pgtype.Int8{Int64: reservation.ID, Valid: true},
		BusinessType:  businessTypeReservation,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return 0, NewRequestError(http.StatusConflict, errors.New("reservation deposit payment record not found"))
		}
		return 0, err
	}
	if paymentOrder.Status != paymentStatusPaid {
		return 0, NewRequestError(http.StatusConflict, errors.New("reservation deposit payment is not settled"))
	}

	return paymentOrder.Amount, nil
}

// ComputeOrderTotals calculates total amount and balance payment based on inputs.
func ComputeOrderTotals(input OrderTotalsInput) (OrderTotalsResult, error) {
	var result OrderTotalsResult

	totalAmount := input.Subtotal - input.DiscountAmount - input.VoucherAmount + input.PackagingFee + input.DeliveryFee - input.DeliveryFeeDiscount
	if totalAmount < 0 {
		totalAmount = 0
	}

	if input.DepositDeduction > 0 {
		if input.DepositDeduction > totalAmount {
			input.DepositDeduction = totalAmount
		}
		totalAmount -= input.DepositDeduction
	}

	balancePaid := int64(0)
	if input.UseBalance {
		if input.MembershipBalance <= 0 {
			return result, NewRequestError(http.StatusBadRequest, errors.New("insufficient membership balance"))
		}
		if input.MembershipBalance >= totalAmount {
			balancePaid = totalAmount
		} else {
			balancePaid = input.MembershipBalance
		}
	}

	result.TotalAmount = totalAmount
	result.BalancePaid = balancePaid
	return result, nil
}
