package merchantreport

import (
	"context"
	"errors"
	"strings"

	"github.com/merrydance/locallife/baofu"
	"github.com/merrydance/locallife/baofu/merchantreport/contracts"
)

type Client struct {
	root *baofu.Client
}

func NewClient(root *baofu.Client) *Client {
	return &Client{root: root}
}

func (c *Client) SubmitWechatReport(ctx context.Context, req contracts.WechatMerchantReportRequest) (*contracts.MerchantReportResult, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	var result contracts.MerchantReportResult
	if err := c.root.PostMerchantReport(ctx, "merchant_report", req, &result); err != nil {
		return nil, err
	}
	if err := result.ValidateMerchantReportResponseForRequest(req); err != nil {
		return nil, baofu.NewProviderContractError("merchant_report", err)
	}
	if err := merchantReportBusinessFailureError("merchant_report", result.ResultCode, result.ErrorCode, result.ErrorMessage); err != nil {
		return nil, err
	}
	result = result.Normalized()
	return &result, nil
}

func (c *Client) QueryReport(ctx context.Context, req contracts.MerchantReportQueryRequest) (*contracts.MerchantReportResult, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	var result contracts.MerchantReportResult
	if err := c.root.PostMerchantReport(ctx, "merchant_report_query", req, &result); err != nil {
		return nil, err
	}
	if err := result.ValidateMerchantReportQueryResponseForRequest(req); err != nil {
		return nil, baofu.NewProviderContractError("merchant_report_query", err)
	}
	if err := merchantReportBusinessFailureError("merchant_report_query", result.ResultCode, result.ErrorCode, result.ErrorMessage); err != nil {
		return nil, err
	}
	result = result.Normalized()
	return &result, nil
}

func (c *Client) BindSubConfig(ctx context.Context, req contracts.BindSubConfigRequest) (*contracts.BindSubConfigResult, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	var result contracts.BindSubConfigResult
	if err := c.root.PostMerchantReport(ctx, "bind_sub_config", req, &result); err != nil {
		return nil, err
	}
	if err := result.ValidateBindSubConfigResponseForRequest(req); err != nil {
		return nil, baofu.NewProviderContractError("bind_sub_config", err)
	}
	if err := merchantReportBusinessFailureError("bind_sub_config", result.ResultCode, result.ErrorCode, result.ErrorMessage); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) validate() error {
	if c == nil || c.root == nil {
		return errors.New("baofu merchant report client is not configured")
	}
	return nil
}

func merchantReportBusinessFailureError(operation, resultCode, errorCode, errorMessage string) error {
	normalizedResultCode := strings.ToUpper(strings.TrimSpace(resultCode))
	normalizedErrorCode := strings.ToUpper(strings.TrimSpace(errorCode))
	if normalizedResultCode == "FAIL" {
		code := strings.TrimSpace(errorCode)
		if code == "" {
			code = normalizedResultCode
		}
		return baofu.NewProviderBusinessError(operation, code, errorMessage)
	}
	if normalizedResultCode == "SUCCESS" && normalizedErrorCode != "" && normalizedErrorCode != "SUCCESS" {
		return baofu.NewProviderBusinessError(operation, errorCode, errorMessage)
	}
	return nil
}
