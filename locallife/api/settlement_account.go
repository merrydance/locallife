package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
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

const settlementApplicationNoMaxLength = 64

func validateSettlementApplicationNo(applicationNo string) error {
	normalized := strings.TrimSpace(applicationNo)
	if normalized == "" {
		return errors.New("application_no is required")
	}
	if utf8.RuneCountInString(normalized) > settlementApplicationNoMaxLength {
		return fmt.Errorf("application_no must not exceed %d characters", settlementApplicationNoMaxLength)
	}
	return nil
}

// modifySettlementAccountRequest 修改结算账户请求体
type modifySettlementAccountRequest struct {
	AccountType    string `json:"account_type" binding:"required,oneof=ACCOUNT_TYPE_BUSINESS ACCOUNT_TYPE_PRIVATE"`
	AccountBank    string `json:"account_bank" binding:"required,max=128"`
	NeedBankBranch bool   `json:"need_bank_branch"`
	BankName       string `json:"bank_name" binding:"omitempty,max=128"`
	BankBranchID   string `json:"bank_branch_id" binding:"omitempty,max=128"`
	AccountNumber  string `json:"account_number" binding:"required,numeric,max=32"`
	AccountName    string `json:"account_name" binding:"omitempty,max=128"`
}

func (req *modifySettlementAccountRequest) normalize() {
	req.AccountType = strings.TrimSpace(req.AccountType)
	req.AccountBank = strings.TrimSpace(req.AccountBank)
	req.BankName = strings.TrimSpace(req.BankName)
	req.BankBranchID = strings.TrimSpace(req.BankBranchID)
	req.AccountNumber = strings.TrimSpace(req.AccountNumber)
	req.AccountName = strings.TrimSpace(req.AccountName)
}

func (req modifySettlementAccountRequest) validateResolvedSelection(needBankBranch bool) error {
	if needBankBranch && req.BankBranchID == "" && req.BankName == "" {
		return errSettlementBankBranchRequired
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
		err := errors.New("ecommerce client not configured")
		_ = ctx.Error(err)
		log.Error().
			Err(err).
			Str("request_id", GetRequestID(ctx)).
			Str("path", ctx.Request.URL.Path).
			Str("method", ctx.Request.Method).
			Msg("merchant settlement account query rejected because ecommerce client is not configured")
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(errors.New("ecommerce client not configured")))
		return
	}

	var query settlementAccountQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		_ = ctx.Error(err)
		log.Warn().
			Err(err).
			Str("request_id", GetRequestID(ctx)).
			Str("path", ctx.Request.URL.Path).
			Str("method", ctx.Request.Method).
			Msg("merchant settlement account query rejected invalid query parameters")
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
// @Description 商户修改自己的收付通结算银行账户。account_number 传入明文后由服务端加密；account_name 仅在需要修改开户名称时传明文，未传时保持当前开户名称不变。
// @Tags 商户财务
// @Accept json
// @Produce json
// @Param body body modifySettlementAccountRequest true "修改结算账户请求"
// @Success 200 {object} modifySettlementAccountApplicationResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "微信签名失败"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 404 {object} ErrorResponse "商户不存在"
// @Failure 429 {object} ErrorResponse "请求过于频繁"
// @Failure 422 {object} ErrorResponse "商户收付通账户未激活"
// @Failure 500 {object} ErrorResponse "加密失败"
// @Failure 503 {object} ErrorResponse "微信客户端未配置"
// @Security BearerAuth
// @Router /v1/merchant/finance/account/settlement-account [post]
func (server *Server) modifyMerchantSettlementAccount(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		respondSettlementClientError(ctx, http.StatusServiceUnavailable, "modify_settlement_account", "merchant", 0, "", "", ErrSettlementServiceUnavailable)
		return
	}

	var req modifySettlementAccountRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		respondSettlementClientError(ctx, http.StatusBadRequest, "modify_settlement_account", "merchant", 0, "", "", err)
		return
	}
	req.normalize()

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
	subMchID := ""
	if paymentConfig != nil {
		subMchID = paymentConfig.SubMchID
	}
	if paymentConfig == nil || accountStatus != "active" {
		respondSettlementClientError(ctx, http.StatusUnprocessableEntity, "modify_settlement_account", "merchant", merchant.ID, subMchID, "", ErrSettlementAccountInactive)
		return
	}
	if err := server.validateMerchantSettlementAccountScope(ctx, merchant.ID, req.AccountType); err != nil {
		if isNotFoundError(err) {
			respondSettlementClientError(ctx, http.StatusBadRequest, "modify_settlement_account", "merchant", merchant.ID, paymentConfig.SubMchID, "", errSettlementMerchantApplymentNotFound)
			return
		}
		if errors.Is(err, ErrMerchantApplymentOrganizationUnsupported) || errors.Is(err, ErrApplymentEnterprisePublicAccountRequired) {
			respondSettlementClientError(ctx, http.StatusBadRequest, "modify_settlement_account", "merchant", merchant.ID, paymentConfig.SubMchID, "", err)
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	bankOption, err := server.resolveSettlementBankOption(ctx.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrSettlementAccountBankUnsupported) {
			respondSettlementClientError(ctx, http.StatusBadRequest, "modify_settlement_account", "merchant", merchant.ID, paymentConfig.SubMchID, "", err)
			return
		}
		status, resp := settlementWechatErrorResponse(ctx, "resolve_settlement_bank", "merchant", merchant.ID, paymentConfig.SubMchID, "", err)
		ctx.JSON(status, resp)
		return
	}
	if err := req.validateResolvedSelection(bankOption.NeedBankBranch); err != nil {
		respondSettlementClientError(ctx, http.StatusBadRequest, "modify_settlement_account", "merchant", merchant.ID, paymentConfig.SubMchID, "", err)
		return
	}
	req.AccountBank = bankOption.AccountBank

	// 加密银行账号（必填敏感字段）
	encryptedAccountNumber, err := server.ecommerceClient.EncryptSensitiveData(req.AccountNumber)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("encrypt account number: %w", err)))
		return
	}

	// 开户名称仅在需要修改时才加密并下发给微信。
	encryptedAccountName := ""
	if req.AccountName != "" {
		encryptedAccountName, err = server.ecommerceClient.EncryptSensitiveData(req.AccountName)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("encrypt account name: %w", err)))
			return
		}
	}

	wxResp, err := server.ecommerceClient.ModifySubMerchantSettlement(ctx, paymentConfig.SubMchID, &wechatcontracts.ModifySubMerchantSettlementRequest{
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
	AccountName      string `json:"account_name" binding:"required"`
	AccountType      string `json:"account_type" binding:"required" enums:"ACCOUNT_TYPE_BUSINESS,ACCOUNT_TYPE_PRIVATE"`
	AccountBank      string `json:"account_bank" binding:"required"`
	BankName         string `json:"bank_name,omitempty"`
	BankBranchID     string `json:"bank_branch_id,omitempty"`
	AccountNumber    string `json:"account_number" binding:"required"`
	VerifyResult     string `json:"verify_result" binding:"required" enums:"AUDIT_SUCCESS,AUDITING,AUDIT_FAIL"`
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
// @Param account_number_rule query string false "银行账号展示规则（默认 ACCOUNT_NUMBER_RULE_MASK_V1）" Enums(ACCOUNT_NUMBER_RULE_MASK_V1,ACCOUNT_NUMBER_RULE_MASK_V2)
// @Success 200 {object} settlementApplicationResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "微信签名失败"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 404 {object} ErrorResponse "商户或申请单不存在"
// @Failure 429 {object} ErrorResponse "请求过于频繁"
// @Failure 422 {object} ErrorResponse "商户收付通账户未激活"
// @Failure 500 {object} ErrorResponse "内部错误"
// @Failure 503 {object} ErrorResponse "微信客户端未配置"
// @Security BearerAuth
// @Router /v1/merchant/finance/account/settlement-account/applications/{application_no} [get]
func (server *Server) getMerchantSettlementApplication(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		respondSettlementClientError(ctx, http.StatusServiceUnavailable, "query_settlement_application", "merchant", 0, "", "", ErrSettlementServiceUnavailable)
		return
	}

	applicationNo := ctx.Param("application_no")
	if err := validateSettlementApplicationNo(applicationNo); err != nil {
		respondSettlementClientError(ctx, http.StatusBadRequest, "query_settlement_application", "merchant", 0, "", applicationNo, err)
		return
	}

	var query settlementAccountQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		respondSettlementClientError(ctx, http.StatusBadRequest, "query_settlement_application", "merchant", 0, "", applicationNo, err)
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, paymentConfig, accountStatus, _, err := server.getFinanceViewerPaymentConfigState(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			respondSettlementClientError(ctx, http.StatusNotFound, "query_settlement_application", "merchant", 0, "", applicationNo, ErrMerchantNotFound)
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	subMchID := ""
	if paymentConfig != nil {
		subMchID = paymentConfig.SubMchID
	}

	if paymentConfig == nil || accountStatus != "active" {
		respondSettlementClientError(ctx, http.StatusUnprocessableEntity, "query_settlement_application", "merchant", merchant.ID, subMchID, applicationNo, ErrSettlementAccountInactive)
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
