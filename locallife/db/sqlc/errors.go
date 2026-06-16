package db

import "errors"

// ErrInsufficientDeposit is returned when a rider's available deposit balance
// is insufficient to cover the requested withdrawal amount.
var ErrInsufficientDeposit = errors.New("insufficient deposit balance")

// ErrRiderDepositFrozen is returned when a rider has frozen deposit and cannot
// initiate any withdrawal until the freeze is released.
var ErrRiderDepositFrozen = errors.New("rider deposit is currently frozen")

// ErrRiderAccountNotActivated is returned when a rider is not allowed to start
// a new deposit withdrawal because their account is not active yet.
var ErrRiderAccountNotActivated = errors.New("rider account has not been activated")

// ErrRiderHasActiveDeliveries is returned when a rider still has active
// deliveries and cannot start a new deposit withdrawal.
var ErrRiderHasActiveDeliveries = errors.New("rider has active delivery orders")

// ErrOrderCancellationBlockedByDeliveryState is returned when an order already
// entered a delivery stage where cancellation can no longer release rider
// deposit automatically.
var ErrOrderCancellationBlockedByDeliveryState = errors.New("order cancellation blocked by delivery state")

// ErrMembershipBalanceInsufficient is returned when a membership balance
// mutation would drive the stored balance below zero.
var ErrMembershipBalanceInsufficient = errors.New("insufficient membership balance")

// ErrMembershipAdjustmentIdempotencyConflict is returned when the same
// idempotency key is reused for a different manual membership adjustment.
var ErrMembershipAdjustmentIdempotencyConflict = errors.New("membership adjustment idempotency conflict")

// ErrOrderCreateIdempotencyConflict is returned when the same order-create
// idempotency key is reused for a different request.
var ErrOrderCreateIdempotencyConflict = errors.New("order create idempotency conflict")
