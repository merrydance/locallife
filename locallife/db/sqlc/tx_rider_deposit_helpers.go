package db

func orderFreezeAmount(order Order) int64 {
	if order.FinalAmount.Valid && order.FinalAmount.Int64 > 0 {
		return order.FinalAmount.Int64
	}
	return order.TotalAmount
}

func deliveryBlocksOrderCancellation(status string) bool {
	switch status {
	case "picked", "delivering", "delivered", "completed":
		return true
	default:
		return false
	}
}

func deliveryRequiresRiderDepositUnfreeze(status string) bool {
	switch status {
	case "assigned", "picking":
		return true
	default:
		return false
	}
}
