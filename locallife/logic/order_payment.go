package logic

import (
	"errors"
	"net/http"
)

// OrderTotalsInput defines the input for computing totals.
type OrderTotalsInput struct {
	Subtotal            int64
	DiscountAmount      int64
	VoucherAmount       int64
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

// ComputeOrderTotals calculates total amount and balance payment based on inputs.
func ComputeOrderTotals(input OrderTotalsInput) (OrderTotalsResult, error) {
	var result OrderTotalsResult

	totalAmount := input.Subtotal - input.DiscountAmount - input.VoucherAmount + input.DeliveryFee - input.DeliveryFeeDiscount
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
