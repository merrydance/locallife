package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/wechat"
	"github.com/rs/zerolog/log"
)

// ========================= 响应类型 =========================

type settlementAccountInfo struct {
	AccountType      string `json:"account_type"`
	AccountBank      string `json:"account_bank"`
	BankName         string `json:"bank_name,omitempty"`
	BankBranchID     string `json:"bank_branch_id,omitempty"`
	AccountNumber    string `json:"account_number"`
	VerifyResult     string `json:"verify_result"`
	VerifyFailReason string `json:"verify_fail_reason,omitempty"`
}

type merchantSettlementAccountResponse struct {
	AccountStatus       string                 `json:"account_status"`
	StatusDesc          string                 `json:"status_desc,omitempty"`
	LatestApplicationNo string                 `json:"latest_application_no,omitempty"`
	Account             *settlementAccountInfo `json:"account,omitempty"`
}

type settlementAccountQuery struct {
	AccountNumberRule string `form:"account_number_rule" binding:"omitempty,oneof=ACCOUNT_NUMBER_RULE_MASK_V1 ACCOUNT_NUMBER_RULE_MASK_V2"`
}

// modifySettlementAccountRequest 修改结算账户请求体
type modifySettlementAccountRequest struct {
	AccountType    string `json:"account_type" binding:"required,oneof=ACCOUNT_TYPE_BUSINESS ACCOUNT_TYPE_PRIVATE"`
	AccountBank    string `json:"account_bank" binding:"required,max=128"`
	NeedBankBranch bool   `json:"need_bank_branch"`
	BankName       string `json:"bank_name" binding:"omitempty,max=128"`
	BankBranchID   string `json:"bank_branch_id" binding:"omitempty,max=128"`
	AccountNumber  string `json:"account_number" binding:"required,numeric,max=32"`
	AccountName    string `json:"account_name" binding:"required,max=128"`
}

func (req *modifySettlementAccountRequest) normalize() {
	req.AccountType = strings.TrimSpace(req.AccountType)
	req.AccountBank = strings.TrimSpace(req.AccountBank)
	req.BankName = strings.TrimSpace(req.BankName)
	req.BankBranchID = strings.TrimSpace(req.BankBranchID)
	req.AccountNumber = strings.TrimSpace(req.AccountNumber)
	req.AccountName = strings.TrimSpace(req.AccountName)
}

func (req modifySettlementAccountRequest) validateSelection() error {
	if req.AccountName == "" {
		return fmt.Errorf("account_name is required")
	}
	return nil
}

func (req modifySettlementAccountRequest) validateResolvedSelection(needBankBranch bool) error {
	if needBankBranch && req.BankBranchID == "" && req.BankName == "" {
		return fmt.Errorf("bank_branch_id or bank_name is required when bank branch selection is needed")
	}
	return nil
}

func pickSettlementBankOption(options []applymentBankOption, accountBank string) (applymentBankOption, bool) {
	target := strings.TrimSpace(accountBank)
	if target == "" {
		return applymentBankOption{}, false
	}

	pickConsistent := func(matches []applymentBankOption) (applymentBankOption, bool) {
		if len(matches) == 0 {
			return applymentBankOption{}, false
		}
		candidate := matches[0]
		for _, option := range matches[1:] {
			if option.NeedBankBranch != candidate.NeedBankBranch {
				return applymentBankOption{}, false
			}
		}
		return candidate, true
	}

	accountBankMatches := make([]applymentBankOption, 0, len(options))
	for _, option := range options {
		if strings.EqualFold(strings.TrimSpace(option.AccountBank), target) {
			accountBankMatches = append(accountBankMatches, option)
		}
	}
	if option, ok := pickConsistent(accountBankMatches); ok {
		return option, true
	}

	aliasMatches := make([]applymentBankOption, 0, len(options))
	for _, option := range options {
		if strings.EqualFold(strings.TrimSpace(option.BankAlias), target) {
			aliasMatches = append(aliasMatches, option)
		}
	}
	return pickConsistent(aliasMatches)
}

func (server *Server) resolveSettlementBankOption(ctx context.Context, req modifySettlementAccountRequest) (*applymentBankOption, error) {
	if req.AccountType == "ACCOUNT_TYPE_PRIVATE" && req.AccountNumber != "" {
		matches, _, err := server.searchApplymentBanksByAccountNumber(ctx, req.AccountNumber)
		if err != nil {
			return nil, fmt.Errorf("search settlement bank by account number: %w", err)
		}
		if option, ok := pickSettlementBankOption(matches, req.AccountBank); ok {
			return &option, nil
		}
		if len(matches) == 1 {
			return &matches[0], nil
		}
	}

	banks, _, err := server.loadApplymentBanks(ctx, req.AccountType)
	if err != nil {
		return nil, fmt.Errorf("load settlement banks: %w", err)
	}
	if option, ok := pickSettlementBankOption(banks, req.AccountBank); ok {
		return &option, nil
	}

	return nil, ErrSettlementAccountBankUnsupported
}

func settlementWechatErrorResponse(ctx *gin.Context, operation string, subjectType string, subjectID int64, subMchID string, applicationNo string, err error) (int, ErrorResponse) {
	_ = ctx.Error(err)

	var wxErr *wechat.WechatPayError
	if errors.As(err, &wxErr) {
		evt := log.Error()
		if wxErr.StatusCode < http.StatusInternalServerError {
			evt = log.Warn()
		}
		evt.
			Err(err).
			Str("request_id", GetRequestID(ctx)).
			Str("operation", operation).
			Str("subject_type", subjectType).
			Int64("subject_id", subjectID).
			Str("sub_mch_id", strings.TrimSpace(subMchID)).
			Str("application_no", strings.TrimSpace(applicationNo)).
			Int("wechat_status_code", wxErr.StatusCode).
			Str("wechat_error_code", strings.TrimSpace(wxErr.Code)).
			Str("wechat_error_message", strings.TrimSpace(wxErr.Message)).
			Str("wechat_error_detail", strings.TrimSpace(wxErr.Detail)).
			Msg("wechat settlement request failed")

		if operation == "query_settlement_application" && wxErr.StatusCode == http.StatusNotFound {
			return http.StatusNotFound, errorResponse(ErrSettlementApplicationNotFound)
		}
		if wxErr.StatusCode == http.StatusBadRequest || wxErr.StatusCode == http.StatusUnauthorized || wxErr.StatusCode == http.StatusForbidden || wxErr.StatusCode == http.StatusUnprocessableEntity {
			switch strings.TrimSpace(wxErr.Code) {
			case "PARAM_ERROR":
				return http.StatusBadGateway, errorResponse(ErrSettlementWechatParamError)
			case "INVALID_REQUEST":
				return http.StatusBadGateway, errorResponse(ErrSettlementWechatInvalidRequest)
			case "SIGN_ERROR":
				return http.StatusBadGateway, errorResponse(ErrSettlementWechatSignError)
			default:
				return http.StatusBadGateway, errorResponse(ErrSettlementWechatRequestRejected)
			}
		}
		return http.StatusBadGateway, errorResponse(ErrSettlementWechatServiceUnavailable)
	}

	log.Error().
		Err(err).
		Str("request_id", GetRequestID(ctx)).
		Str("operation", operation).
		Str("subject_type", subjectType).
		Int64("subject_id", subjectID).
		Str("sub_mch_id", strings.TrimSpace(subMchID)).
		Str("application_no", strings.TrimSpace(applicationNo)).
		Msg("wechat settlement request failed")

	return http.StatusBadGateway, errorResponse(ErrSettlementWechatServiceUnavailable)
}

func (server *Server) validateMerchantSettlementAccountScope(ctx *gin.Context, merchantID int64, accountType string) error {
	applyment, err := server.store.GetLatestEcommerceApplymentBySubject(ctx, db.GetLatestEcommerceApplymentBySubjectParams{
		SubjectType: "merchant",
		SubjectID:   merchantID,
	})
	if err != nil {
		return fmt.Errorf("get latest merchant applyment: %w", err)
	}
	return validateMerchantApplymentScope(strings.TrimSpace(applyment.OrganizationType), strings.TrimSpace(accountType))
}

// modifySettlementAccountApplicationResponse 修改结算账户申请成功应答
type modifySettlementAccountApplicationResponse struct {
	ApplicationNo string `json:"application_no"`
}

// ========================= 商户侧接口 =========================

// getMerchantSettlementAccount 查询商户结算账户信息
// @Summary 查询商户结算账户
// @Description 商户查询自己的收付通结算账户（银行账户）信息，商户号从认证 session 取
// @Tags 商户财务
// @Produce json
// @Param account_number_rule query string false "银行账号展示规则（默认 ACCOUNT_NUMBER_RULE_MASK_V1）"
// @Success 200 {object} merchantSettlementAccountResponse
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 503 {object} ErrorResponse "微信客户端未配置"
// @Security BearerAuth
// @Router /v1/merchant/finance/account/settlement-account [get]
func (server *Server) getMerchantSettlementAccount(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(errors.New("ecommerce client not configured")))
		return
	}

	var query settlementAccountQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, paymentConfig, accountStatus, statusDesc, err := server.getFinanceViewerPaymentConfigState(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrMerchantNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	latestApplicationNo := ""
	if paymentConfig != nil {
		latestApplicationNo = pgTextValue(paymentConfig.LatestSettlementApplicationNo)
	}

	if paymentConfig == nil || accountStatus != "active" {
		ctx.JSON(http.StatusOK, merchantSettlementAccountResponse{
			AccountStatus:       accountStatus,
			StatusDesc:          statusDesc,
			LatestApplicationNo: latestApplicationNo,
		})
		return
	}

	wxResp, err := server.ecommerceClient.QuerySubMerchantSettlement(ctx, paymentConfig.SubMchID, query.AccountNumberRule)
	if err != nil {
		status, resp := settlementWechatErrorResponse(ctx, "query_settlement_account", "merchant", merchant.ID, paymentConfig.SubMchID, "", fmt.Errorf("query settlement account: %w", err))
		ctx.JSON(status, resp)
		return
	}

	statusDesc = buildSettlementAccountStatusDesc(wxResp.VerifyResult, wxResp.VerifyFailReason)
	logSettlementAccountQuerySuccess("merchant", merchant.ID, paymentConfig.SubMchID, wxResp.VerifyResult, latestApplicationNo, wxResp.VerifyFailReason != "")

	ctx.JSON(http.StatusOK, merchantSettlementAccountResponse{
		AccountStatus:       "active",
		StatusDesc:          statusDesc,
		LatestApplicationNo: latestApplicationNo,
		Account: &settlementAccountInfo{
			AccountType:      wxResp.AccountType,
			AccountBank:      wxResp.AccountBank,
			BankName:         wxResp.BankName,
			BankBranchID:     wxResp.BankBranchID,
			AccountNumber:    wxResp.AccountNumber,
			VerifyResult:     wxResp.VerifyResult,
			VerifyFailReason: wxResp.VerifyFailReason,
		},
	})
}

// ========================= 商户侧修改接口 =========================

// modifyMerchantSettlementAccount 修改商户结算账户
// @Summary 修改商户结算账户
// @Description 商户修改自己的收付通结算银行账户。account_number 和 account_name 传入明文，服务端负责加密后转发给微信支付。
// @Tags 商户财务
// @Accept json
// @Produce json
// @Param body body modifySettlementAccountRequest true "修改结算账户请求"
// @Success 200 {object} modifySettlementAccountApplicationResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 404 {object} ErrorResponse "商户不存在"
// @Failure 422 {object} ErrorResponse "商户收付通账户未激活"
// @Failure 500 {object} ErrorResponse "加密失败"
// @Failure 502 {object} ErrorResponse "微信支付下游异常"
// @Failure 503 {object} ErrorResponse "微信客户端未配置"
// @Security BearerAuth
// @Router /v1/merchant/finance/account/settlement-account [post]
func (server *Server) modifyMerchantSettlementAccount(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(errors.New("ecommerce client not configured")))
		return
	}

	var req modifySettlementAccountRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	req.normalize()
	if err := req.validateSelection(); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, paymentConfig, accountStatus, _, err := server.getFinanceViewerPaymentConfigState(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrMerchantNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if paymentConfig == nil || accountStatus != "active" {
		ctx.JSON(http.StatusUnprocessableEntity, errorResponse(errors.New("merchant payment account is not active")))
		return
	}
	if err := server.validateMerchantSettlementAccountScope(ctx, merchant.ID, req.AccountType); err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("merchant applyment not found")))
			return
		}
		if errors.Is(err, ErrMerchantApplymentOrganizationUnsupported) || errors.Is(err, ErrApplymentEnterprisePublicAccountRequired) {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	bankOption, err := server.resolveSettlementBankOption(ctx.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrSettlementAccountBankUnsupported) {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}
		status, resp := settlementWechatErrorResponse(ctx, "resolve_settlement_bank", "merchant", merchant.ID, paymentConfig.SubMchID, "", err)
		ctx.JSON(status, resp)
		return
	}
	if err := req.validateResolvedSelection(bankOption.NeedBankBranch); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	req.AccountBank = bankOption.AccountBank

	// 加密银行账号（必填敏感字段）
	encryptedAccountNumber, err := server.ecommerceClient.EncryptSensitiveData(req.AccountNumber)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("encrypt account number: %w", err)))
		return
	}

	// 加密开户名称（必填敏感字段）
	encryptedAccountName, err := server.ecommerceClient.EncryptSensitiveData(req.AccountName)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("encrypt account name: %w", err)))
		return
	}

	wxResp, err := server.ecommerceClient.ModifySubMerchantSettlement(ctx, paymentConfig.SubMchID, &wechat.ModifySubMerchantSettlementRequest{
		AccountType:   req.AccountType,
		AccountBank:   req.AccountBank,
		BankName:      req.BankName,
		BankBranchID:  req.BankBranchID,
		AccountNumber: encryptedAccountNumber,
		AccountName:   encryptedAccountName,
	})
	if err != nil {
		status, resp := settlementWechatErrorResponse(ctx, "modify_settlement_account", "merchant", merchant.ID, paymentConfig.SubMchID, "", fmt.Errorf("modify settlement account: %w", err))
		ctx.JSON(status, resp)
		return
	}

	submittedAt := time.Now()
	if err := server.updateMerchantSettlementApplicationTracking(ctx, merchant.ID, wxResp.ApplicationNo, &submittedAt); err != nil {
		log.Error().Err(err).
			Int64("merchant_id", merchant.ID).
			Str("sub_mch_id", paymentConfig.SubMchID).
			Str("application_no", wxResp.ApplicationNo).
			Msg("persist latest merchant settlement application failed")
	}
	logSettlementModifySuccess("merchant", merchant.ID, paymentConfig.SubMchID, wxResp.ApplicationNo)

	ctx.JSON(http.StatusOK, modifySettlementAccountApplicationResponse{
		ApplicationNo: wxResp.ApplicationNo,
	})
}

// ========================= 申请状态查询类型 =========================

// settlementApplicationResponse 结算账户修改申请状态响应
type settlementApplicationResponse struct {
	AccountName      string `json:"account_name"`
	AccountType      string `json:"account_type"`
	AccountBank      string `json:"account_bank"`
	BankName         string `json:"bank_name,omitempty"`
	BankBranchID     string `json:"bank_branch_id,omitempty"`
	AccountNumber    string `json:"account_number"`
	VerifyResult     string `json:"verify_result"`
	VerifyFailReason string `json:"verify_fail_reason,omitempty"`
	VerifyFinishTime string `json:"verify_finish_time,omitempty"`
}

// ========================= 商户侧申请查询 =========================

// getMerchantSettlementApplication 查询商户结算账户修改申请状态
// @Summary 查询商户结算账户修改申请状态
// @Description 商户查询自己的结算账户修改申请审核结果。
// @Tags 商户财务
// @Produce json
// @Param application_no path string true "修改结算账户申请单号"
// @Param account_number_rule query string false "银行账号展示规则（默认 ACCOUNT_NUMBER_RULE_MASK_V1）"
// @Success 200 {object} settlementApplicationResponse
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 404 {object} ErrorResponse "商户或申请单不存在"
// @Failure 422 {object} ErrorResponse "商户收付通账户未激活"
// @Failure 500 {object} ErrorResponse "内部错误"
// @Failure 502 {object} ErrorResponse "微信支付下游异常"
// @Failure 503 {object} ErrorResponse "微信客户端未配置"
// @Security BearerAuth
// @Router /v1/merchant/finance/account/settlement-account/applications/{application_no} [get]
func (server *Server) getMerchantSettlementApplication(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(errors.New("ecommerce client not configured")))
		return
	}

	applicationNo := ctx.Param("application_no")
	if applicationNo == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("application_no is required")))
		return
	}

	var query settlementAccountQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, paymentConfig, accountStatus, _, err := server.getFinanceViewerPaymentConfigState(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrMerchantNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if paymentConfig == nil || accountStatus != "active" {
		ctx.JSON(http.StatusUnprocessableEntity, errorResponse(errors.New("merchant payment account is not active")))
		return
	}

	wxResp, err := server.ecommerceClient.QuerySubMerchantSettlementApplication(ctx, paymentConfig.SubMchID, applicationNo, query.AccountNumberRule)
	if err != nil {
		status, resp := settlementWechatErrorResponse(ctx, "query_settlement_application", "merchant", merchant.ID, paymentConfig.SubMchID, applicationNo, fmt.Errorf("query settlement application: %w", err))
		ctx.JSON(status, resp)
		return
	}

	if err := server.updateMerchantSettlementApplicationTracking(ctx, merchant.ID, applicationNo, nil); err != nil {
		log.Error().Err(err).
			Int64("merchant_id", merchant.ID).
			Str("sub_mch_id", paymentConfig.SubMchID).
			Str("application_no", applicationNo).
			Msg("persist queried merchant settlement application failed")
	}
	logSettlementApplicationQuerySuccess("merchant", merchant.ID, paymentConfig.SubMchID, applicationNo, wxResp.VerifyResult, wxResp.VerifyFailReason != "")

	ctx.JSON(http.StatusOK, settlementApplicationResponse{
		AccountName:      wxResp.AccountName,
		AccountType:      wxResp.AccountType,
		AccountBank:      wxResp.AccountBank,
		BankName:         wxResp.BankName,
		BankBranchID:     wxResp.BankBranchID,
		AccountNumber:    wxResp.AccountNumber,
		VerifyResult:     wxResp.VerifyResult,
		VerifyFailReason: wxResp.VerifyFailReason,
		VerifyFinishTime: wxResp.VerifyFinishTime,
	})
}
