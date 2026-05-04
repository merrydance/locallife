package aggregatepay

import (
	"context"
	"errors"

	"github.com/merrydance/locallife/baofu"
	"github.com/merrydance/locallife/baofu/aggregatepay/contracts"
)

type Client interface {
	CreateUnifiedOrder(ctx context.Context, req contracts.UnifiedOrderRequest) (*contracts.UnifiedOrderResult, error)
	QueryPayment(ctx context.Context, req contracts.PaymentQueryRequest) (*contracts.UnifiedOrderResult, error)
	CreateProfitSharing(ctx context.Context, req contracts.ShareAfterPayRequest) (*contracts.ShareResult, error)
	QueryProfitSharing(ctx context.Context, req contracts.ShareQueryRequest) (*contracts.ShareResult, error)
}

type HTTPClient struct {
	root *baofu.Client
}

func NewClient(root *baofu.Client) *HTTPClient {
	return &HTTPClient{root: root}
}

func (c *HTTPClient) CreateUnifiedOrder(ctx context.Context, req contracts.UnifiedOrderRequest) (*contracts.UnifiedOrderResult, error) {
	if err := c.validate("unified_order"); err != nil {
		return nil, err
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	var result contracts.UnifiedOrderResult
	if err := c.root.PostAggregatePay(ctx, "unified_order", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *HTTPClient) QueryPayment(ctx context.Context, req contracts.PaymentQueryRequest) (*contracts.UnifiedOrderResult, error) {
	if err := c.validate("order_query"); err != nil {
		return nil, err
	}
	var result contracts.UnifiedOrderResult
	if err := c.root.PostAggregatePay(ctx, "order_query", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *HTTPClient) CreateProfitSharing(ctx context.Context, req contracts.ShareAfterPayRequest) (*contracts.ShareResult, error) {
	if err := c.validate("share_after_pay"); err != nil {
		return nil, err
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	var result contracts.ShareResult
	if err := c.root.PostAggregatePay(ctx, "share_after_pay", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *HTTPClient) QueryProfitSharing(ctx context.Context, req contracts.ShareQueryRequest) (*contracts.ShareResult, error) {
	if err := c.validate("share_query"); err != nil {
		return nil, err
	}
	var result contracts.ShareResult
	if err := c.root.PostAggregatePay(ctx, "share_query", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *HTTPClient) validate(operation string) error {
	if c == nil || c.root == nil {
		return errors.New("baofu aggregatepay client is not configured for " + operation)
	}
	return nil
}
