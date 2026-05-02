package db

func PaymentOrderUsesEcommerceChannel(paymentOrder PaymentOrder) bool {
	return paymentOrder.PaymentChannel == PaymentChannelEcommerce
}

func PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder PaymentOrder) bool {
	return paymentOrder.PaymentChannel == PaymentChannelOrdinaryServiceProvider
}

func PaymentOrderRequiresProfitSharing(paymentOrder PaymentOrder) bool {
	return (paymentOrder.PaymentChannel == PaymentChannelEcommerce || paymentOrder.PaymentChannel == PaymentChannelOrdinaryServiceProvider) && paymentOrder.RequiresProfitSharing
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
