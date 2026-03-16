package db

import "errors"

// ErrInsufficientDeposit is returned when a rider's available deposit balance
// is insufficient to cover the requested withdrawal amount.
var ErrInsufficientDeposit = errors.New("insufficient deposit balance")
