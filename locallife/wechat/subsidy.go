package wechat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
)

// 微信平台收付通补差 API 端点
const (
	subsidyCreateURL = "/v3/ecommerce/subsidies/create"
	subsidyReturnURL = "/v3/ecommerce/subsidies/return"
	subsidyCancelURL = "/v3/ecommerce/subsidies/cancel"
)

// CreateSubsidy 向二级商户发起补差（平台出资）
// 对应 POST /v3/ecommerce/subsidies/create
//
// 调用时机：分账完结前，通常在订单确认完成后调用。
// 补差成功后资金立即进入二级商户账户，不受分账冻结影响。
func (c *EcommerceClient) CreateSubsidy(ctx context.Context, req wechatcontracts.SubsidyRequest) (*wechatcontracts.SubsidyResponse, error) {
	if err := wechatcontracts.ValidateSubsidyRequest(req); err != nil {
		return nil, err
	}

	body := map[string]interface{}{
		"sub_mchid":      req.SubMchID,
		"transaction_id": req.TransactionID,
		"amount":         req.Amount,
		"description":    req.Description,
		"out_subsidy_no": req.OutSubsidyNo,
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, subsidyCreateURL, body)
	if err != nil {
		return nil, fmt.Errorf("create subsidy (out_subsidy_no=%s): %w", req.OutSubsidyNo, err)
	}

	var result wechatcontracts.SubsidyResponse
	// 微信此接口在成功时可能返回空 body，unmarshal 容错
	if len(respBody) > 2 {
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, fmt.Errorf("parse create subsidy response: %w", err)
		}
	}
	if err := wechatcontracts.ValidateSubsidyCreateResponse("create subsidy", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ReturnSubsidy 退回补差（退款时平台从二级商户账户回收补差款）
// 对应 POST /v3/ecommerce/subsidies/return
//
// 调用时机：退款前，与 CreateProfitSharingReturn 类似，需先退回补差再退款。
func (c *EcommerceClient) ReturnSubsidy(ctx context.Context, req wechatcontracts.SubsidyReturnRequest) (*wechatcontracts.SubsidyReturnResponse, error) {
	if err := wechatcontracts.ValidateSubsidyReturnRequest(req); err != nil {
		return nil, err
	}

	body := map[string]interface{}{
		"sub_mchid":      req.SubMchID,
		"out_order_no":   req.OutOrderNo,
		"transaction_id": req.TransactionID,
		"amount":         req.Amount,
		"description":    req.Description,
	}
	if strings.TrimSpace(req.RefundID) != "" {
		body["refund_id"] = req.RefundID
	}
	if strings.TrimSpace(req.SubsidyID) != "" {
		body["subsidy_id"] = req.SubsidyID
	}
	if len(req.From) > 0 {
		body["from"] = req.From
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, subsidyReturnURL, body)
	if err != nil {
		return nil, fmt.Errorf("return subsidy (out_order_no=%s): %w", req.OutOrderNo, err)
	}

	var result wechatcontracts.SubsidyReturnResponse
	if len(respBody) > 2 {
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, fmt.Errorf("parse return subsidy response: %w", err)
		}
	}
	if err := wechatcontracts.ValidateSubsidyReturnResponse("return subsidy", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CancelSubsidy 取消补差（尚未分账前可取消）
// 对应 POST /v3/ecommerce/subsidies/cancel
func (c *EcommerceClient) CancelSubsidy(ctx context.Context, req wechatcontracts.SubsidyCancelRequest) (*wechatcontracts.SubsidyCancelResponse, error) {
	if err := wechatcontracts.ValidateSubsidyCancelRequest(req); err != nil {
		return nil, err
	}

	body := map[string]interface{}{
		"sub_mchid":      req.SubMchID,
		"transaction_id": req.TransactionID,
		"description":    req.Description,
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, subsidyCancelURL, body)
	if err != nil {
		return nil, fmt.Errorf("cancel subsidy (transaction_id=%s): %w", req.TransactionID, err)
	}

	var result wechatcontracts.SubsidyCancelResponse
	if len(respBody) > 2 {
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, fmt.Errorf("parse cancel subsidy response: %w", err)
		}
	}
	if err := wechatcontracts.ValidateSubsidyCancelResponse("cancel subsidy", &result); err != nil {
		return nil, err
	}
	return &result, nil
}
