package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/wechat"
)

type subMerchantLimitationRecoverySpecificationResponse struct {
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

type accountLimitationsResponse struct {
	AccountStatus          string                                               `json:"account_status"`
	StatusDesc             string                                               `json:"status_desc,omitempty"`
	MchID                  string                                               `json:"mchid,omitempty"`
	LimitedFunctions       []string                                             `json:"limited_functions,omitempty"`
	OtherLimitedFunctions  string                                               `json:"other_limited_functions,omitempty"`
	RecoverySpecifications []subMerchantLimitationRecoverySpecificationResponse `json:"recovery_specifications,omitempty"`
}

type platformSubMerchantLimitationsResponse struct {
	MchID                  string                                               `json:"mchid"`
	LimitedFunctions       []string                                             `json:"limited_functions,omitempty"`
	OtherLimitedFunctions  string                                               `json:"other_limited_functions,omitempty"`
	RecoverySpecifications []subMerchantLimitationRecoverySpecificationResponse `json:"recovery_specifications,omitempty"`
}

func mapSubMerchantLimitationRecoverySpecifications(items []wechat.SubMerchantLimitationRecoverySpecification) []subMerchantLimitationRecoverySpecificationResponse {
	if len(items) == 0 {
		return nil
	}

	result := make([]subMerchantLimitationRecoverySpecificationResponse, 0, len(items))
	for _, item := range items {
		result = append(result, subMerchantLimitationRecoverySpecificationResponse{
			LimitationCaseID:         item.LimitationCaseID,
			LimitationReasonType:     item.LimitationReasonType,
			LimitationReason:         item.LimitationReason,
			LimitationReasonDescribe: item.LimitationReasonDescribe,
			RelateLimitations:        item.RelateLimitations,
			OtherRelateLimitations:   item.OtherRelateLimitations,
			RecoverWay:               item.RecoverWay,
			RecoverWayParam:          item.RecoverWayParam,
			RecoverHelpURL:           item.RecoverHelpURL,
			LimitationActionType:     item.LimitationActionType,
			LimitationStartDate:      item.LimitationStartDate,
			LimitationDate:           item.LimitationDate,
		})
	}

	return result
}

func newAccountLimitationsResponse(accountStatus, statusDesc string, wxResp *wechat.SubMerchantLimitationsResponse) accountLimitationsResponse {
	resp := accountLimitationsResponse{
		AccountStatus: accountStatus,
		StatusDesc:    statusDesc,
	}
	if wxResp == nil {
		return resp
	}

	resp.StatusDesc = ""
	resp.MchID = wxResp.MchID
	resp.LimitedFunctions = wxResp.LimitedFunctions
	resp.OtherLimitedFunctions = wxResp.OtherLimitedFunctions
	resp.RecoverySpecifications = mapSubMerchantLimitationRecoverySpecifications(wxResp.RecoverySpecifications)
	return resp
}

func newPlatformSubMerchantLimitationsResponse(wxResp *wechat.SubMerchantLimitationsResponse) platformSubMerchantLimitationsResponse {
	return platformSubMerchantLimitationsResponse{
		MchID:                  wxResp.MchID,
		LimitedFunctions:       wxResp.LimitedFunctions,
		OtherLimitedFunctions:  wxResp.OtherLimitedFunctions,
		RecoverySpecifications: mapSubMerchantLimitationRecoverySpecifications(wxResp.RecoverySpecifications),
	}
}

// getMerchantAccountLimitations 查询商户自己的收付通子商户管控情况。
// @Summary 查询商户收付通管控情况
// @Description 商户查询自己名下收付通子商户的管控情况；未开通或未激活时返回 account_status 和 status_desc。
// @Tags 商户财务
// @Produce json
// @Success 200 {object} accountLimitationsResponse
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 404 {object} ErrorResponse "商户不存在"
// @Failure 502 {object} ErrorResponse "微信支付下游异常"
// @Failure 503 {object} ErrorResponse "微信客户端未配置"
// @Security BearerAuth
// @Router /v1/merchant/finance/account/limitations [get]
func (server *Server) getMerchantAccountLimitations(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(errors.New("ecommerce client not configured")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	_, paymentConfig, accountStatus, statusDesc, err := server.getFinanceViewerPaymentConfigState(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if paymentConfig == nil || accountStatus != "active" {
		ctx.JSON(http.StatusOK, newAccountLimitationsResponse(accountStatus, statusDesc, nil))
		return
	}

	wxResp, err := server.ecommerceClient.QuerySubMerchantLimitations(ctx, paymentConfig.SubMchID)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, internalError(ctx, fmt.Errorf("query merchant limitations: %w", err)))
		return
	}

	ctx.JSON(http.StatusOK, newAccountLimitationsResponse("active", "", wxResp))
}

// getOperatorAccountLimitations 查询运营商自己的收付通子商户管控情况。
// @Summary 查询运营商收付通管控情况
// @Description 运营商查询自己名下收付通子商户的管控情况；未开通时返回 account_status 和 status_desc。
// @Tags 运营商财务
// @Produce json
// @Success 200 {object} accountLimitationsResponse
// @Failure 403 {object} ErrorResponse "运营商未激活或无权限"
// @Failure 502 {object} ErrorResponse "微信支付下游异常"
// @Failure 503 {object} ErrorResponse "微信客户端未配置"
// @Security BearerAuth
// @Router /v1/operators/me/finance/account/limitations [get]
func (server *Server) getOperatorAccountLimitations(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(errors.New("ecommerce client not configured")))
		return
	}

	operator := ctx.MustGet(operatorKey).(db.Operator)
	if operator.Status != "active" {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("operator is not active")))
		return
	}

	accountStatus, statusDesc := getOperatorFinanceAccountStatus(operator)
	if accountStatus != "active" {
		ctx.JSON(http.StatusOK, newAccountLimitationsResponse(accountStatus, statusDesc, nil))
		return
	}

	wxResp, err := server.ecommerceClient.QuerySubMerchantLimitations(ctx, operator.SubMchID.String)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, internalError(ctx, fmt.Errorf("query operator limitations: %w", err)))
		return
	}

	ctx.JSON(http.StatusOK, newAccountLimitationsResponse("active", "", wxResp))
}

// getPlatformSubMerchantLimitations 查询平台侧指定子商户的收付通管控情况。
// @Summary 查询子商户收付通管控情况
// @Description 平台管理员按 sub_mch_id 查询微信支付平台收付通子商户的管控情况。
// @Tags 平台财务
// @Produce json
// @Param sub_mch_id path string true "子商户号"
// @Success 200 {object} platformSubMerchantLimitationsResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 502 {object} ErrorResponse "微信支付下游异常"
// @Failure 503 {object} ErrorResponse "微信客户端未配置"
// @Security BearerAuth
// @Router /v1/platform/finance/wechat-ecommerce/merchant-limitations/{sub_mch_id} [get]
func (server *Server) getPlatformSubMerchantLimitations(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(errors.New("ecommerce client not configured")))
		return
	}

	subMchID := strings.TrimSpace(ctx.Param("sub_mch_id"))
	if subMchID == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("sub_mch_id is required")))
		return
	}

	wxResp, err := server.ecommerceClient.QuerySubMerchantLimitations(ctx, subMchID)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, internalError(ctx, fmt.Errorf("query platform sub merchant limitations: %w", err)))
		return
	}

	ctx.JSON(http.StatusOK, newPlatformSubMerchantLimitationsResponse(wxResp))
}
