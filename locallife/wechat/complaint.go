package wechat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// 微信支付用户投诉 v2 API 端点
const (
	complaintsListURL    = "/v3/merchant-service/complaints-v2"
	complaintDetailURL   = "/v3/merchant-service/complaints-v2/%s"
	complaintResponseURL = "/v3/merchant-service/complaints-v2/%s/response"
	complaintCompleteURL = "/v3/merchant-service/complaints-v2/%s/complete"
)

// ComplaintState 投诉状态
type ComplaintState string

const (
	ComplaintStatePendingResponse ComplaintState = "PENDING_RESPONSE" // 待回复
	ComplaintStateProcessing      ComplaintState = "PROCESSING"       // 处理中
	ComplaintStateProcessed       ComplaintState = "PROCESSED"        // 已完结
)

// ComplaintDetail 投诉单详情（微信侧返回）
type ComplaintDetail struct {
	// ComplaintID 投诉单号（全局唯一）
	ComplaintID string `json:"complaint_id"`
	// ComplaintTime 投诉时间
	ComplaintTime time.Time `json:"complaint_time"`
	// PayerOpenID 投诉人 openid（微信可能不返回）
	PayerOpenID string `json:"payer_openid"`
	// ComplaintDetail 投诉详情
	ComplaintDetail string `json:"complaint_detail"`
	// ComplaintState 投诉状态
	ComplaintState ComplaintState `json:"complaint_state"`
	// TransactionID 相关微信支付订单号
	TransactionID string `json:"transaction_id"`
	// SubMchID 二级商户号（收付通平台场景，微信返回字段名为 sub_mchid）
	SubMchID string `json:"sub_mchid"`
	// ComplaintOrderInfo 关联订单信息列表
	ComplaintOrderInfo []ComplaintOrderInfo `json:"complaint_order_info"`
	// PayerComplaintFullInfo 是否已付款完整投诉
	PayerComplaintFullInfo bool `json:"payer_complaint_full_info"`
	// Amount 订单金额（分）
	Amount int64 `json:"complaint_amount"`
	// UpdateTime 更新时间
	UpdateTime time.Time `json:"update_time"`
}

// ComplaintOrderInfo 投诉关联的订单信息
type ComplaintOrderInfo struct {
	TransactionID string `json:"transaction_id"` // 微信支付订单号
	OutTradeNo    string `json:"out_trade_no"`   // 商户订单号
	Amount        int64  `json:"amount"`          // 订单金额（分）
}

// ListComplaintsRequest 查询投诉列表请求参数
type ListComplaintsRequest struct {
	// BeginDate 查询开始日期（格式 "2006-01-02"）
	BeginDate string
	// EndDate 查询结束日期（格式 "2006-01-02"）
	EndDate string
	// ComplaintState 投诉状态过滤（空表示不过滤）
	ComplaintState ComplaintState
	// SubMchID 二级商户号（收付通场景；空表示查全部）
	SubMchID string
	// Limit 分页大小（1-50，默认5）
	Limit int
	// Offset 分页偏移（≥0）
	Offset int
}

// ListComplaintsResponse 查询投诉列表响应
type ListComplaintsResponse struct {
	// Data 投诉单列表
	Data []ComplaintDetail `json:"data"`
	// TotalCount 总记录数
	TotalCount int `json:"total_count"`
	// Limit 本次请求的 limit
	Limit int `json:"limit"`
	// Offset 本次请求的 offset
	Offset int `json:"offset"`
}

// ComplaintResponseRequest 回复投诉请求
type ComplaintResponseRequest struct {
	// ComplaintID 投诉单号
	ComplaintID string
	// ResponseContent 回复内容（最长256字）
	ResponseContent string
	// JumpURL 跳转链接（可选）
	JumpURL string
}

// ListComplaints 查询投诉单列表（支持分页 + 状态过滤）
// 对应 GET /v3/merchant-service/complaints-v2
func (c *EcommerceClient) ListComplaints(ctx context.Context, req ListComplaintsRequest) (*ListComplaintsResponse, error) {
	if req.Limit <= 0 {
		req.Limit = 5
	}
	if req.Limit > 50 {
		req.Limit = 50
	}

	apiPath := fmt.Sprintf("%s?limit=%d&offset=%d", complaintsListURL, req.Limit, req.Offset)
	if req.BeginDate != "" {
		apiPath += "&begin_date=" + req.BeginDate
	}
	if req.EndDate != "" {
		apiPath += "&end_date=" + req.EndDate
	}
	if req.ComplaintState != "" {
		apiPath += "&complaint_state=" + string(req.ComplaintState)
	}
	if req.SubMchID != "" {
		apiPath += "&sub_mchid=" + req.SubMchID
	}

	respBody, err := c.doRequest(ctx, http.MethodGet, apiPath, nil)
	if err != nil {
		return nil, fmt.Errorf("list complaints: %w", err)
	}

	var result ListComplaintsResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse list complaints response: %w", err)
	}
	return &result, nil
}

// GetComplaintDetail 查询投诉单详情
// 对应 GET /v3/merchant-service/complaints-v2/{complaint_id}
func (c *EcommerceClient) GetComplaintDetail(ctx context.Context, complaintID string) (*ComplaintDetail, error) {
	apiPath := fmt.Sprintf(complaintDetailURL, complaintID)
	respBody, err := c.doRequest(ctx, http.MethodGet, apiPath, nil)
	if err != nil {
		return nil, fmt.Errorf("get complaint detail (complaint_id=%s): %w", complaintID, err)
	}

	var result ComplaintDetail
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse complaint detail: %w", err)
	}
	return &result, nil
}

// RespondComplaint 回复投诉
// 对应 POST /v3/merchant-service/complaints-v2/{complaint_id}/response
// 回复成功后投诉状态将变为 PROCESSING
func (c *EcommerceClient) RespondComplaint(ctx context.Context, req ComplaintResponseRequest) error {
	body := map[string]interface{}{
		"complainted_mchid": c.spMchID,
		"response_content":  req.ResponseContent,
	}
	if req.JumpURL != "" {
		body["jump_url"] = req.JumpURL
	}
	apiPath := fmt.Sprintf(complaintResponseURL, req.ComplaintID)
	_, err := c.doRequest(ctx, http.MethodPost, apiPath, body)
	if err != nil {
		return fmt.Errorf("respond complaint (complaint_id=%s): %w", req.ComplaintID, err)
	}
	return nil
}

// CompleteComplaint 完结投诉
// 对应 POST /v3/merchant-service/complaints-v2/{complaint_id}/complete
// 仅当问题已解决且用户不再追加投诉时调用
func (c *EcommerceClient) CompleteComplaint(ctx context.Context, complaintID string) error {
	body := map[string]interface{}{
		"complainted_mchid": c.spMchID,
	}
	apiPath := fmt.Sprintf(complaintCompleteURL, complaintID)
	_, err := c.doRequest(ctx, http.MethodPost, apiPath, body)
	if err != nil {
		return fmt.Errorf("complete complaint (complaint_id=%s): %w", complaintID, err)
	}
	return nil
}

// ComplaintNotification 微信推送的投诉通知（Webhook）
type ComplaintNotification struct {
	// ComplaintID 投诉单号
	ComplaintID string `json:"complaint_id"`
	// ActionType 事件类型：COMPLAINT_CLOSE（投诉完结）/ COMPLAINT_STATE_CHANGE（状态变更）
	ActionType string `json:"action_type"`
	// ComplaintDetail 最新投诉摘要
	ComplaintDetail string `json:"complaint_detail"`
	// State 最新投诉状态
	State ComplaintState `json:"state"`
	// ComplaintOrderInfo 关联订单（可能为空数组）
	ComplaintOrderInfo []ComplaintOrderInfo `json:"complaint_order_info"`
	// ServiceOrderInfo 服务单信息（一般不关注）
	ServiceOrderInfo []interface{} `json:"service_order_info"`
	// PolicyID 投诉触发的处理策略 ID
	PolicyID string `json:"policy_id"`
	// SubMchID 子商户号（收付通场景）
	SubMchID string `json:"sub_mchid"`
}

// DecryptComplaintNotification 解密投诉通知
func (c *EcommerceClient) DecryptComplaintNotification(notification *PaymentNotification) (*ComplaintNotification, error) {
	raw, err := c.DecryptNotificationRaw(notification)
	if err != nil {
		return nil, fmt.Errorf("decrypt complaint notification: %w", err)
	}
	var result ComplaintNotification
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse complaint notification: %w", err)
	}
	return &result, nil
}
