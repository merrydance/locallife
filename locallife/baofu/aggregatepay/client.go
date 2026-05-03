package aggregatepay

import (
	"context"

	"github.com/merrydance/locallife/baofu/aggregatepay/contracts"
)

type Client interface {
	CreateUnifiedOrder(ctx context.Context, req contracts.UnifiedOrderRequest) (*contracts.UnifiedOrderResult, error)
}
