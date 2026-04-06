package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/wechat"
)

func isWechatPayNotFoundError(err error) bool {
	var wxErr *wechat.WechatPayError
	return errors.As(err, &wxErr) && wxErr.StatusCode == http.StatusNotFound
}

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
	AccountStatus string                 `json:"account_status"`
	StatusDesc    string                 `json:"status_desc,omitempty"`
	Account       *settlementAccountInfo `json:"account,omitempty"`
}

type operatorSettlementAccountResponse struct {
	AccountStatus string                 `json:"account_status"`
	StatusDesc    string                 `json:"status_desc,omitempty"`
	Account       *settlementAccountInfo `json:"account,omitempty"`
}

// modifySettlementAccountRequest 修改结算账户请求体
type modifySettlementAccountRequest struct {
	AccountType   string `json:"account_type" binding:"required"`
	AccountBank   string `json:"account_bank" binding:"required"`
	BankName      string `json:"bank_name"`
	BankBranchID  string `json:"bank_branch_id"`
	AccountNumber string `json:"account_number" binding:"required"`
	AccountName   string `json:"account_name"`
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
		ctx.JSON(http.StatusOK, merchantSettlementAccountResponse{
			AccountStatus: accountStatus,
			StatusDesc:    statusDesc,
		})
		return
	}

	wxResp, err := server.ecommerceClient.QuerySubMerchantSettlement(ctx, paymentConfig.SubMchID, "")
	if err != nil {
		ctx.JSON(http.StatusBadGateway, internalError(ctx, fmt.Errorf("query settlement account: %w", err)))
		return
	}

	ctx.JSON(http.StatusOK, merchantSettlementAccountResponse{
		AccountStatus: "active",
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

// ========================= 运营商侧接口 =========================

// getOperatorSettlementAccount 查询运营商结算账户信息
// @Summary 查询运营商结算账户
// @Description 运营商查询自己的收付通结算账户（银行账户）信息，商户号从认证 session 取
// @Tags 运营商财务
// @Produce json
// @Success 200 {object} operatorSettlementAccountResponse
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 503 {object} ErrorResponse "微信客户端未配置"
// @Security BearerAuth
// @Router /v1/operators/me/finance/account/settlement-account [get]
func (server *Server) getOperatorSettlementAccount(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(errors.New("ecommerce client not configured")))
		return
	}

	operator := ctx.MustGet(operatorKey).(db.Operator)

	// 与余额接口（operator_finance.go）保持一致：仅 active 的运营商可查询财务账户
	if operator.Status != "active" {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("operator is not active")))
		return
	}

	accountStatus, statusDesc := getOperatorFinanceAccountStatus(operator)
	if accountStatus != "active" {
		ctx.JSON(http.StatusOK, operatorSettlementAccountResponse{
			AccountStatus: accountStatus,
			StatusDesc:    statusDesc,
		})
		return
	}

	wxResp, err := server.ecommerceClient.QuerySubMerchantSettlement(ctx, operator.SubMchID.String, "")
	if err != nil {
		ctx.JSON(http.StatusBadGateway, internalError(ctx, fmt.Errorf("query settlement account: %w", err)))
		return
	}

	ctx.JSON(http.StatusOK, operatorSettlementAccountResponse{
		AccountStatus: "active",
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

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	_, paymentConfig, accountStatus, _, err := server.getFinanceViewerPaymentConfigState(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if paymentConfig == nil || accountStatus != "active" {
		ctx.JSON(http.StatusUnprocessableEntity, errorResponse(errors.New("merchant payment account is not active")))
		return
	}

	// 加密银行账号（必填敏感字段）
	encryptedAccountNumber, err := server.ecommerceClient.EncryptSensitiveData(req.AccountNumber)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("encrypt account number: %w", err)))
		return
	}

	// 加密开户名称（选填敏感字段，非空才加密）
	var encryptedAccountName string
	if req.AccountName != "" {
		encryptedAccountName, err = server.ecommerceClient.EncryptSensitiveData(req.AccountName)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("encrypt account name: %w", err)))
			return
		}
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
		ctx.JSON(http.StatusBadGateway, internalError(ctx, fmt.Errorf("modify settlement account: %w", err)))
		return
	}

	ctx.JSON(http.StatusOK, modifySettlementAccountApplicationResponse{
		ApplicationNo: wxResp.ApplicationNo,
	})
}

// ========================= 运营商侧修改接口 =========================

// modifyOperatorSettlementAccount 修改运营商结算账户
// @Summary 修改运营商结算账户
// @Description 运营商修改自己的收付通结算银行账户。account_number 和 account_name 传入明文，服务端负责加密后转发给微信支付。
// @Tags 运营商财务
// @Accept json
// @Produce json
// @Param body body modifySettlementAccountRequest true "修改结算账户请求"
// @Success 200 {object} modifySettlementAccountApplicationResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 403 {object} ErrorResponse "运营商未激活或无权限"
// @Failure 422 {object} ErrorResponse "运营商收付通账户未激活"
// @Failure 500 {object} ErrorResponse "加密失败"
// @Failure 502 {object} ErrorResponse "微信支付下游异常"
// @Failure 503 {object} ErrorResponse "微信客户端未配置"
// @Security BearerAuth
// @Router /v1/operators/me/finance/account/settlement-account [post]
func (server *Server) modifyOperatorSettlementAccount(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(errors.New("ecommerce client not configured")))
		return
	}

	var req modifySettlementAccountRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	operator := ctx.MustGet(operatorKey).(db.Operator)

	if operator.Status != "active" {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("operator is not active")))
		return
	}

	accountStatus, _ := getOperatorFinanceAccountStatus(operator)
	if accountStatus != "active" {
		ctx.JSON(http.StatusUnprocessableEntity, errorResponse(errors.New("operator payment account is not active")))
		return
	}

	// 加密银行账号（必填敏感字段）
	encryptedAccountNumber, err := server.ecommerceClient.EncryptSensitiveData(req.AccountNumber)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("encrypt account number: %w", err)))
		return
	}

	// 加密开户名称（选填敏感字段，非空才加密）
	var encryptedAccountName string
	if req.AccountName != "" {
		encryptedAccountName, err = server.ecommerceClient.EncryptSensitiveData(req.AccountName)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("encrypt account name: %w", err)))
			return
		}
	}

	wxResp, err := server.ecommerceClient.ModifySubMerchantSettlement(ctx, operator.SubMchID.String, &wechat.ModifySubMerchantSettlementRequest{
		AccountType:   req.AccountType,
		AccountBank:   req.AccountBank,
		BankName:      req.BankName,
		BankBranchID:  req.BankBranchID,
		AccountNumber: encryptedAccountNumber,
		AccountName:   encryptedAccountName,
	})
	if err != nil {
		ctx.JSON(http.StatusBadGateway, internalError(ctx, fmt.Errorf("modify settlement account: %w", err)))
		return
	}

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
	accountNumberRule := ctx.Query("account_number_rule")

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	_, paymentConfig, accountStatus, _, err := server.getFinanceViewerPaymentConfigState(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("merchant not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if paymentConfig == nil || accountStatus != "active" {
		ctx.JSON(http.StatusUnprocessableEntity, errorResponse(errors.New("merchant payment account is not active")))
		return
	}

	wxResp, err := server.ecommerceClient.QuerySubMerchantSettlementApplication(ctx, paymentConfig.SubMchID, applicationNo, accountNumberRule)
	if err != nil {
		if isWechatPayNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("settlement application not found")))
			return
		}
		ctx.JSON(http.StatusBadGateway, internalError(ctx, fmt.Errorf("query settlement application: %w", err)))
		return
	}

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

// ========================= 运营商侧申请查询 =========================

// getOperatorSettlementApplication 查询运营商结算账户修改申请状态
// @Summary 查询运营商结算账户修改申请状态
// @Description 运营商查询自己的结算账户修改申请审核结果。
// @Tags 运营商财务
// @Produce json
// @Param application_no path string true "修改结算账户申请单号"
// @Param account_number_rule query string false "银行账号展示规则（默认 ACCOUNT_NUMBER_RULE_MASK_V1）"
// @Success 200 {object} settlementApplicationResponse
// @Failure 403 {object} ErrorResponse "运营商未激活或无权限"
// @Failure 404 {object} ErrorResponse "申请单不存在"
// @Failure 422 {object} ErrorResponse "运营商收付通账户未激活"
// @Failure 502 {object} ErrorResponse "微信支付下游异常"
// @Failure 503 {object} ErrorResponse "微信客户端未配置"
// @Security BearerAuth
// @Router /v1/operators/me/finance/account/settlement-account/applications/{application_no} [get]
func (server *Server) getOperatorSettlementApplication(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(errors.New("ecommerce client not configured")))
		return
	}

	applicationNo := ctx.Param("application_no")
	if applicationNo == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("application_no is required")))
		return
	}
	accountNumberRule := ctx.Query("account_number_rule")

	operator := ctx.MustGet(operatorKey).(db.Operator)

	if operator.Status != "active" {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("operator is not active")))
		return
	}

	accountStatus, _ := getOperatorFinanceAccountStatus(operator)
	if accountStatus != "active" {
		ctx.JSON(http.StatusUnprocessableEntity, errorResponse(errors.New("operator payment account is not active")))
		return
	}

	wxResp, err := server.ecommerceClient.QuerySubMerchantSettlementApplication(ctx, operator.SubMchID.String, applicationNo, accountNumberRule)
	if err != nil {
		if isWechatPayNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("settlement application not found")))
			return
		}
		ctx.JSON(http.StatusBadGateway, internalError(ctx, fmt.Errorf("query settlement application: %w", err)))
		return
	}

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
