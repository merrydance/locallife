package aggregatepay

import (
	"context"

	"github.com/merrydance/locallife/baofu/aggregatepay/contracts"
)

type Client interface {
	CreateUnifiedOrder(ctx context.Context, req contracts.UnifiedOrderRequest) (*contracts.UnifiedOrderResult, error)
	QueryPayment(ctx context.Context, req contracts.PaymentQueryRequest) (*contracts.UnifiedOrderResult, error)
	CreateProfitSharing(ctx context.Context, req contracts.ShareAfterPayRequest) (*contracts.ShareResult, error)
	QueryProfitSharing(ctx context.Context, req contracts.ShareQueryRequest) (*contracts.ShareResult, error)
}
