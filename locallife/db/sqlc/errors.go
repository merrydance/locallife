package db

import "errors"

// ErrInsufficientDeposit is returned when a rider's available deposit balance
// is insufficient to cover the requested withdrawal amount.
var ErrInsufficientDeposit = errors.New("insufficient deposit balance")

// ErrRiderDepositFrozen is returned when a rider has frozen deposit and cannot
// initiate any withdrawal until the freeze is released.
var ErrRiderDepositFrozen = errors.New("rider deposit is currently frozen")

// ErrOrderCancellationBlockedByDeliveryState is returned when an order already
// entered a delivery stage where cancellation can no longer release rider
// deposit automatically.
var ErrOrderCancellationBlockedByDeliveryState = errors.New("order cancellation blocked by delivery state")
