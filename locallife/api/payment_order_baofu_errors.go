package api

import (
	"errors"
	"strings"

	"github.com/merrydance/locallife/baofu"
	"github.com/merrydance/locallife/logic"
)

func isBaofuPaymentServiceNotConfigured(err error) bool {
	return errors.Is(err, logic.ErrBaofuPaymentServiceNotConfigured)
}

func isBaofuProviderPaymentError(err error) bool {
	if err == nil {
		return false
	}
	var providerErr *baofu.ProviderError
	if !errors.As(err, &providerErr) {
		return false
	}
	operation := strings.ToLower(strings.TrimSpace(providerErr.Operation))
	return strings.Contains(operation, "order") || strings.Contains(operation, "payment")
}
