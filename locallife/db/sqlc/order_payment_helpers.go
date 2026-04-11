package db

import "fmt"

const (
	orderPaymentMethodWechat  = "wechat"
	orderPaymentMethodBalance = "balance"
)

// OrderRemainingPayableAmount returns the remaining external payment amount after membership balance deduction.
func OrderRemainingPayableAmount(order Order) (int64, error) {
	if order.BalancePaid < 0 {
		return 0, fmt.Errorf("order %d balance_paid is negative", order.ID)
	}
	if order.BalancePaid > order.TotalAmount {
		return 0, fmt.Errorf("order %d balance_paid exceeds total_amount", order.ID)
	}
	return order.TotalAmount - order.BalancePaid, nil
}

func normalizeOrderPaymentMethod(method string) string {
	switch method {
	case orderPaymentMethodBalance:
		return orderPaymentMethodBalance
	case "", orderPaymentMethodWechat:
		return orderPaymentMethodWechat
	default:
		return orderPaymentMethodWechat
	}
}
