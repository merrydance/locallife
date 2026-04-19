package wechat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const violationNotificationURL = "/v3/merchant-risk-manage/violation-notifications"

// ViolationNotificationConfigRequest 商户违规通知回调地址请求。
type ViolationNotificationConfigRequest struct {
	NotifyURL *string `json:"notify_url,omitempty"`
}

// ViolationNotificationConfigResponse 商户违规通知回调地址响应。
type ViolationNotificationConfigResponse struct {
	NotifyURL *string `json:"notify_url,omitempty"`
}

// ViolationNotificationResource 商户违规通知解密后的资源对象。
type ViolationNotificationResource struct {
	SubMchID          string    `json:"sub_mchid"`
	CompanyName       string    `json:"company_name"`
	RecordID          string    `json:"record_id"`
	PunishPlan        string    `json:"punish_plan"`
	PunishTime        time.Time `json:"punish_time"`
	PunishDescription string    `json:"punish_description"`
	RiskType          string    `json:"risk_type"`
	RiskDescription   string    `json:"risk_description"`
}

// QueryViolationNotification 查询商户违规通知回调地址。
func (c *EcommerceClient) QueryViolationNotification(ctx context.Context) (*ViolationNotificationConfigResponse, error) {
	respBody, err := c.doRequest(ctx, http.MethodGet, violationNotificationURL, nil)
	if err != nil {
		return nil, fmt.Errorf("query violation notification: %w", err)
	}

	var result ViolationNotificationConfigResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse violation notification query response: %w", err)
	}

	return &result, nil
}

// CreateViolationNotification 创建商户违规通知回调地址。
func (c *EcommerceClient) CreateViolationNotification(ctx context.Context, req *ViolationNotificationConfigRequest) (*ViolationNotificationConfigResponse, error) {
	normalized, err := c.normalizeViolationNotificationConfig(req)
	if err != nil {
		return nil, err
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, violationNotificationURL, normalized)
	if err != nil {
		return nil, fmt.Errorf("create violation notification: %w", err)
	}

	var result ViolationNotificationConfigResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse violation notification create response: %w", err)
	}

	return &result, nil
}

// UpdateViolationNotification 修改商户违规通知回调地址。
func (c *EcommerceClient) UpdateViolationNotification(ctx context.Context, req *ViolationNotificationConfigRequest) (*ViolationNotificationConfigResponse, error) {
	normalized, err := c.normalizeViolationNotificationConfig(req)
	if err != nil {
		return nil, err
	}

	respBody, err := c.doRequest(ctx, http.MethodPut, violationNotificationURL, normalized)
	if err != nil {
		return nil, fmt.Errorf("update violation notification: %w", err)
	}

	var result ViolationNotificationConfigResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse violation notification update response: %w", err)
	}

	return &result, nil
}

// DeleteViolationNotification 删除商户违规通知回调地址。
func (c *EcommerceClient) DeleteViolationNotification(ctx context.Context) error {
	if _, err := c.doRequest(ctx, http.MethodDelete, violationNotificationURL, nil); err != nil {
		return fmt.Errorf("delete violation notification: %w", err)
	}
	return nil
}

// DecryptViolationNotification 解密商户违规通知。
func (c *EcommerceClient) DecryptViolationNotification(notification *PaymentNotification) (*ViolationNotificationResource, error) {
	raw, err := c.DecryptNotificationRaw(notification)
	if err != nil {
		return nil, fmt.Errorf("decrypt violation notification: %w", err)
	}

	var result ViolationNotificationResource
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse violation notification: %w", err)
	}

	return &result, nil
}

func (c *EcommerceClient) normalizeViolationNotificationConfig(req *ViolationNotificationConfigRequest) (*ViolationNotificationConfigRequest, error) {
	if req == nil {
		req = &ViolationNotificationConfigRequest{}
	}

	resolvedURL := c.violationNotifyURL
	if req.NotifyURL != nil {
		resolvedURL = strings.TrimSpace(*req.NotifyURL)
	}
	if resolvedURL == "" {
		return nil, fmt.Errorf("violation notification notify_url is required")
	}

	return &ViolationNotificationConfigRequest{NotifyURL: &resolvedURL}, nil
}
