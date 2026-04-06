package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
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
	AccountStatus string                 `json:"account_status"`
	StatusDesc    string                 `json:"status_desc,omitempty"`
	Account       *settlementAccountInfo `json:"account,omitempty"`
}

type operatorSettlementAccountResponse struct {
	AccountStatus string                 `json:"account_status"`
	StatusDesc    string                 `json:"status_desc,omitempty"`
	Account       *settlementAccountInfo `json:"account,omitempty"`
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
