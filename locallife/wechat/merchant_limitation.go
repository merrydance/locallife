package wechat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const merchantLimitationsURL = "/v3/mch-operation-manage/merchant-limitations/sub-mchid/%s"

// SubMerchantLimitationsResponse 二级商户管控情况。
type SubMerchantLimitationsResponse struct {
	MchID                  string                                       `json:"mchid"`
	LimitedFunctions       []string                                     `json:"limited_functions,omitempty"`
	OtherLimitedFunctions  string                                       `json:"other_limited_functions,omitempty"`
	RecoverySpecifications []SubMerchantLimitationRecoverySpecification `json:"recovery_specifications,omitempty"`
}

// SubMerchantLimitationRecoverySpecification 单个管控原因及解脱路径。
type SubMerchantLimitationRecoverySpecification struct {
	LimitationCaseID         string   `json:"limitation_case_id,omitempty"`
	LimitationReasonType     string   `json:"limitation_reason_type,omitempty"`
	LimitationReason         string   `json:"limitation_reason,omitempty"`
	LimitationReasonDescribe string   `json:"limitation_reason_describe,omitempty"`
	RelateLimitations        []string `json:"relate_limitations,omitempty"`
	OtherRelateLimitations   string   `json:"other_relate_limitations,omitempty"`
	RecoverWay               string   `json:"recover_way,omitempty"`
	RecoverWayParam          string   `json:"recover_way_param,omitempty"`
	RecoverHelpURL           string   `json:"recover_help_url,omitempty"`
	LimitationActionType     string   `json:"limitation_action_type,omitempty"`
	LimitationStartDate      string   `json:"limitation_start_date,omitempty"`
	LimitationDate           string   `json:"limitation_date,omitempty"`
}

// QuerySubMerchantLimitations 查询特约商户/二级商户管控情况。
func (c *EcommerceClient) QuerySubMerchantLimitations(ctx context.Context, subMchID string) (*SubMerchantLimitationsResponse, error) {
	subMchID = strings.TrimSpace(subMchID)
	if subMchID == "" {
		return nil, fmt.Errorf("query sub merchant limitations: sub_mchid is required")
	}

	requestURL := fmt.Sprintf(merchantLimitationsURL, url.PathEscape(subMchID))
	respBody, err := c.doRequest(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("query sub merchant limitations: %w", err)
	}

	var resp SubMerchantLimitationsResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal sub merchant limitations: %w", err)
	}

	if resp.MchID == "" {
		resp.MchID = subMchID
	}

	return &resp, nil
}
