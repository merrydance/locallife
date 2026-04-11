package wechat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// 微信平台收付通补差 API 端点
const (
	subsidyCreateURL = "/v3/ecommerce/subsidies/create"
	subsidyReturnURL = "/v3/ecommerce/subsidies/return"
	subsidyCancelURL = "/v3/ecommerce/subsidies/cancel"
)

// SubsidyRequest 创建补差请求
type SubsidyRequest struct {
	// SubMchID 二级商户号
	SubMchID string
	// TransactionID 微信支付订单号（子单 transaction_id）
	TransactionID string
	// PayerAmount 用户实际支付金额（分）
	PayerAmount int64
	// Amount 补差金额（分），补差金额 + PayerAmount = 商品原价
	Amount int64
	// Description 补差说明（最长80字）
	Description string
	// OutSubsidyNo 商户补差单号（全局唯一，用于幂等）
	OutSubsidyNo string
}

// SubsidyResponse 创建补差响应（微信仅返回 HTTP 200，无 body）
// 成功时微信不返回 subsidy_id，out_subsidy_no 即为幂等标识
type SubsidyResponse struct {
	// SubsidyID 微信补差单号（部分版本文档有记录，实际可能为空）
	SubsidyID string `json:"subsidy_id"`
}

// SubsidyReturnRequest 补差退回请求（退款时使用）
type SubsidyReturnRequest struct {
	// SubMchID 二级商户号
	SubMchID string
	// TransactionID 原支付订单的微信支付订单号
	TransactionID string
	// OutReturnNo 商户退回单号（全局唯一）
	OutReturnNo string
	// Amount 退回金额（分），不超过原补差金额
	Amount int64
	// Description 退回说明（最长80字）
	Description string
}

// SubsidyReturnResponse 补差退回响应
type SubsidyReturnResponse struct {
	// ReturnID 微信退回单号
	ReturnID string `json:"return_id"`
	// OutReturnNo 商户退回单号（回显）
	OutReturnNo string `json:"out_return_no"`
}

// SubsidyCancelRequest 取消补差请求
type SubsidyCancelRequest struct {
	// SubMchID 二级商户号
	SubMchID string
	// TransactionID 微信支付订单号
	TransactionID string
	// Description 取消原因（最长80字）
	Description string
}

// CreateSubsidy 向二级商户发起补差（平台出资）
// 对应 POST /v3/ecommerce/subsidies/create
//
// 调用时机：分账完结前，通常在订单确认完成后调用。
// 补差成功后资金立即进入二级商户账户，不受分账冻结影响。
func (c *EcommerceClient) CreateSubsidy(ctx context.Context, req SubsidyRequest) (*SubsidyResponse, error) {
	if req.Amount <= 0 {
		return nil, fmt.Errorf("subsidy amount must be positive, got %d", req.Amount)
	}
	if req.SubMchID == "" || req.TransactionID == "" || req.OutSubsidyNo == "" {
		return nil, fmt.Errorf("SubMchID, TransactionID, OutSubsidyNo are required")
	}

	body := map[string]interface{}{
		"appid":           c.spAppID,
		"sp_mchid":        c.spMchID,
		"sub_mchid":       req.SubMchID,
		"transaction_id":  req.TransactionID,
		"payer_amount":    req.PayerAmount,
		"amount":          req.Amount,
		"description":     req.Description,
		"out_subsidy_no":  req.OutSubsidyNo,
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, subsidyCreateURL, body)
	if err != nil {
		return nil, fmt.Errorf("create subsidy (out_subsidy_no=%s): %w", req.OutSubsidyNo, err)
	}

	var result SubsidyResponse
	// 微信此接口在成功时可能返回空 body，unmarshal 容错
	if len(respBody) > 2 {
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, fmt.Errorf("parse create subsidy response: %w", err)
		}
	}
	return &result, nil
}

// ReturnSubsidy 退回补差（退款时平台从二级商户账户回收补差款）
// 对应 POST /v3/ecommerce/subsidies/return
//
// 调用时机：退款前，与 CreateProfitSharingReturn 类似，需先退回补差再退款。
func (c *EcommerceClient) ReturnSubsidy(ctx context.Context, req SubsidyReturnRequest) (*SubsidyReturnResponse, error) {
	if req.Amount <= 0 {
		return nil, fmt.Errorf("return amount must be positive, got %d", req.Amount)
	}
	if req.SubMchID == "" || req.TransactionID == "" || req.OutReturnNo == "" {
		return nil, fmt.Errorf("SubMchID, TransactionID, OutReturnNo are required")
	}

	body := map[string]interface{}{
		"sp_mchid":       c.spMchID,
		"sub_mchid":      req.SubMchID,
		"transaction_id": req.TransactionID,
		"out_return_no":  req.OutReturnNo,
		"amount":         req.Amount,
		"description":    req.Description,
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, subsidyReturnURL, body)
	if err != nil {
		return nil, fmt.Errorf("return subsidy (out_return_no=%s): %w", req.OutReturnNo, err)
	}

	var result SubsidyReturnResponse
	if len(respBody) > 2 {
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, fmt.Errorf("parse return subsidy response: %w", err)
		}
	}
	return &result, nil
}

// CancelSubsidy 取消补差（尚未分账前可取消）
// 对应 POST /v3/ecommerce/subsidies/cancel
func (c *EcommerceClient) CancelSubsidy(ctx context.Context, req SubsidyCancelRequest) error {
	if req.SubMchID == "" || req.TransactionID == "" {
		return fmt.Errorf("SubMchID and TransactionID are required")
	}

	body := map[string]interface{}{
		"sp_mchid":       c.spMchID,
		"sub_mchid":      req.SubMchID,
		"transaction_id": req.TransactionID,
		"description":    req.Description,
	}

	_, err := c.doRequest(ctx, http.MethodPost, subsidyCancelURL, body)
	if err != nil {
		return fmt.Errorf("cancel subsidy (transaction_id=%s): %w", req.TransactionID, err)
	}
	return nil
}
