package db

type RiderDepositAvailability struct {
	DeliveryFrozenDeposit      int64
	WithdrawalProcessingAmount int64
	AvailableDeposit           int64
}

func CalculateRiderDepositAvailability(rider Rider, withdrawalProcessingAmount int64) RiderDepositAvailability {
	deliveryFrozenDeposit := rider.FrozenDeposit - withdrawalProcessingAmount
	if deliveryFrozenDeposit < 0 {
		deliveryFrozenDeposit = 0
	}

	availableDeposit := rider.DepositAmount - deliveryFrozenDeposit - withdrawalProcessingAmount
	if availableDeposit < 0 {
		availableDeposit = 0
	}

	return RiderDepositAvailability{
		DeliveryFrozenDeposit:      deliveryFrozenDeposit,
		WithdrawalProcessingAmount: withdrawalProcessingAmount,
		AvailableDeposit:           availableDeposit,
	}
}
