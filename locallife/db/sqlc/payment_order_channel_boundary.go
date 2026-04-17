package db

func PaymentOrderUsesEcommerceChannel(paymentOrder PaymentOrder) bool {
	return paymentOrder.PaymentChannel == PaymentChannelEcommerce
}

func PaymentOrderRequiresProfitSharing(paymentOrder PaymentOrder) bool {
	return paymentOrder.PaymentChannel == PaymentChannelEcommerce && paymentOrder.RequiresProfitSharing
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
