package db

func PaymentOrderRequiresProfitSharing(paymentOrder PaymentOrder) bool {
	return paymentOrder.PaymentChannel == PaymentChannelBaofuAggregate && paymentOrder.RequiresProfitSharing
}

func OrderRequiresProfitSharing(order Order) bool {
	if order.ReservationID.Valid {
		return true
	}

	switch order.OrderType {
	case "dine_in", "takeaway":
		return false
	default:
		return true
	}
}
