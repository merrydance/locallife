package account

import (
	"context"
	"errors"

	"github.com/merrydance/locallife/baofu"
	"github.com/merrydance/locallife/baofu/account/contracts"
)

type Client struct {
	root *baofu.Client
}

func NewClient(root *baofu.Client) *Client {
	return &Client{root: root}
}

func (c *Client) OpenAccount(ctx context.Context, req contracts.OpenAccountRequest) (*contracts.AccountResult, error) {
	if c == nil || c.root == nil {
		return nil, errors.New("baofu account client is not configured")
	}
	return nil, errors.New("baofu account open transport is not implemented")
}

func (c *Client) QueryAccount(ctx context.Context, req contracts.QueryAccountRequest) (*contracts.AccountResult, error) {
	if c == nil || c.root == nil {
		return nil, errors.New("baofu account client is not configured")
	}
	return nil, errors.New("baofu account query transport is not implemented")
}

func (c *Client) QueryBalance(ctx context.Context, req contracts.BalanceQueryRequest) (*contracts.BalanceResult, error) {
	if c == nil || c.root == nil {
		return nil, errors.New("baofu account client is not configured")
	}
	return nil, errors.New("baofu balance query transport is not implemented")
}

func (c *Client) CreateWithdraw(ctx context.Context, req contracts.WithdrawRequest) (*contracts.WithdrawResult, error) {
	if c == nil || c.root == nil {
		return nil, errors.New("baofu account client is not configured")
	}
	return nil, errors.New("baofu withdraw transport is not implemented")
}

func (c *Client) QueryWithdraw(ctx context.Context, req contracts.WithdrawQueryRequest) (*contracts.WithdrawResult, error) {
	if c == nil || c.root == nil {
		return nil, errors.New("baofu account client is not configured")
	}
	return nil, errors.New("baofu withdraw query transport is not implemented")
}
