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
	CreateRefund(ctx context.Context, req contracts.RefundBeforeShareRequest) (*contracts.RefundResult, error)
	QueryRefund(ctx context.Context, req contracts.RefundQueryRequest) (*contracts.RefundResult, error)
	CloseOrder(ctx context.Context, req contracts.OrderCloseRequest) (*contracts.OrderCloseResult, error)
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
	environment := c.root.Config().Environment
	if environment == baofu.BaofuEnvironmentSandbox {
		req = req.WithoutSubMchID()
	}
	if err := req.ValidateForEnvironment(environment); err != nil {
		return nil, err
	}
	var result contracts.UnifiedOrderResult
	if err := c.root.PostAggregatePay(ctx, "unified_order", req, &result); err != nil {
		return nil, err
	}
	if err := result.ValidateUnifiedOrderResponseForRequest(req); err != nil {
		return nil, baofu.NewProviderContractError("unified_order", err)
	}
	return &result, nil
}

func (c *HTTPClient) QueryPayment(ctx context.Context, req contracts.PaymentQueryRequest) (*contracts.UnifiedOrderResult, error) {
	if err := c.validate("order_query"); err != nil {
		return nil, err
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	var result contracts.UnifiedOrderResult
	if err := c.root.PostAggregatePay(ctx, "order_query", req, &result); err != nil {
		return nil, err
	}
	if err := result.ValidateOrderQueryResponseForRequest(req); err != nil {
		return nil, baofu.NewProviderContractError("order_query", err)
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
	if err := result.ValidateShareAfterPayResponseForRequest(req); err != nil {
		return nil, baofu.NewProviderContractError("share_after_pay", err)
	}
	return &result, nil
}

func (c *HTTPClient) QueryProfitSharing(ctx context.Context, req contracts.ShareQueryRequest) (*contracts.ShareResult, error) {
	if err := c.validate("share_query"); err != nil {
		return nil, err
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	var result contracts.ShareResult
	if err := c.root.PostAggregatePay(ctx, "share_query", req, &result); err != nil {
		return nil, err
	}
	if err := result.ValidateShareQueryResponseForRequest(req); err != nil {
		return nil, baofu.NewProviderContractError("share_query", err)
	}
	return &result, nil
}

func (c *HTTPClient) CreateRefund(ctx context.Context, req contracts.RefundBeforeShareRequest) (*contracts.RefundResult, error) {
	if err := c.validate("order_refund"); err != nil {
		return nil, err
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	var result contracts.RefundResult
	if err := c.root.PostAggregatePay(ctx, "order_refund", req, &result); err != nil {
		return nil, err
	}
	if err := result.ValidateOrderRefundResponseForRequest(req); err != nil {
		return nil, baofu.NewProviderContractError("order_refund", err)
	}
	return &result, nil
}

func (c *HTTPClient) QueryRefund(ctx context.Context, req contracts.RefundQueryRequest) (*contracts.RefundResult, error) {
	if err := c.validate("refund_query"); err != nil {
		return nil, err
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	var result contracts.RefundResult
	if err := c.root.PostAggregatePay(ctx, "refund_query", req, &result); err != nil {
		return nil, err
	}
	if err := result.ValidateRefundQueryResponseForRequest(req); err != nil {
		return nil, baofu.NewProviderContractError("refund_query", err)
	}
	return &result, nil
}

func (c *HTTPClient) CloseOrder(ctx context.Context, req contracts.OrderCloseRequest) (*contracts.OrderCloseResult, error) {
	if err := c.validate("order_close"); err != nil {
		return nil, err
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	var result contracts.OrderCloseResult
	if err := c.root.PostAggregatePay(ctx, "order_close", req, &result); err != nil {
		return nil, err
	}
	if err := result.ValidateOrderCloseResponseForRequest(req); err != nil {
		return nil, baofu.NewProviderContractError("order_close", err)
	}
	return &result, nil
}

func (c *HTTPClient) validate(operation string) error {
	if c == nil || c.root == nil {
		return errors.New("baofu aggregatepay client is not configured for " + operation)
	}
	return nil
}
