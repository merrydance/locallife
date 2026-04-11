package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/merrydance/locallife/wechat"
)

const defaultWechatFundAccountType = "BASIC"

type fundBalanceQueryRequest struct {
	AccountType string `form:"account_type"`
	Date        string `form:"date"`
}

type platformAccountBalanceResponse struct {
	AccountType     string `json:"account_type"`
	BalanceDate     string `json:"balance_date,omitempty"`
	AvailableAmount int64  `json:"available_amount"`
	PendingAmount   int64  `json:"pending_amount"`
}

var subMerchantRealtimeAccountTypes = map[string]struct{}{
	"BASIC":     {},
	"FEES":      {},
	"OPERATION": {},
	"DEPOSIT":   {},
}

var subMerchantDayEndAccountTypes = map[string]struct{}{
	"BASIC":   {},
	"DEPOSIT": {},
}

var platformAccountTypes = map[string]struct{}{
	"BASIC":     {},
	"FEES":      {},
	"OPERATION": {},
}

func bindSubMerchantFundBalanceQuery(ctx *gin.Context) (fundBalanceQueryRequest, bool) {
	return bindFundBalanceQuery(ctx, subMerchantRealtimeAccountTypes, subMerchantDayEndAccountTypes)
}

func bindPlatformFundBalanceQuery(ctx *gin.Context) (fundBalanceQueryRequest, bool) {
	return bindFundBalanceQuery(ctx, platformAccountTypes, platformAccountTypes)
}

func bindFundBalanceQuery(ctx *gin.Context, realtimeAllowed, dayEndAllowed map[string]struct{}) (fundBalanceQueryRequest, bool) {
	var req fundBalanceQueryRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return fundBalanceQueryRequest{}, false
	}

	req.AccountType = strings.ToUpper(strings.TrimSpace(req.AccountType))
	if req.AccountType == "" {
		req.AccountType = defaultWechatFundAccountType
	}
	req.Date = strings.TrimSpace(req.Date)

	if req.Date != "" {
		if _, err := time.Parse("2006-01-02", req.Date); err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("date must use YYYY-MM-DD format")))
			return fundBalanceQueryRequest{}, false
		}
		if _, ok := dayEndAllowed[req.AccountType]; !ok {
			ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("account_type %s is not supported for day-end balance", req.AccountType)))
			return fundBalanceQueryRequest{}, false
		}
		return req, true
	}

	if _, ok := realtimeAllowed[req.AccountType]; !ok {
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("account_type %s is not supported", req.AccountType)))
		return fundBalanceQueryRequest{}, false
	}

	return req, true
}

func loadSubMerchantFundBalance(ctx *gin.Context, client wechat.EcommerceClientInterface, subMchID string, query fundBalanceQueryRequest) (*wechat.EcommerceFundBalanceResponse, error) {
	if query.Date != "" {
		return client.QueryEcommerceFundDayEndBalance(ctx, subMchID, query.Date, query.AccountType)
	}
	return client.QueryEcommerceFundBalanceByAccountType(ctx, subMchID, query.AccountType)
}

// getPlatformAccountBalance 查询平台微信支付账户余额
// @Summary 查询平台微信支付账户余额
// @Description 管理员查询平台商户号微信支付账户实时余额；传入 date 时查询指定日期日终余额
// @Tags 平台财务
// @Produce json
// @Param account_type query string false "账户类型，默认 BASIC" Enums(BASIC,OPERATION,FEES)
// @Param date query string false "日终余额日期，格式 YYYY-MM-DD；传入后查询日终余额"
// @Success 200 {object} platformAccountBalanceResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 502 {object} ErrorResponse "微信支付调用失败"
// @Security BearerAuth
// @Router /v1/platform/finance/account/balance [get]
func (server *Server) getPlatformAccountBalance(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(errors.New("ecommerce client not configured")))
		return
	}

	query, ok := bindPlatformFundBalanceQuery(ctx)
	if !ok {
		return
	}

	var (
		balance *wechat.PlatformFundBalanceResponse
		err     error
	)
	if query.Date != "" {
		balance, err = server.ecommerceClient.QueryPlatformFundDayEndBalance(ctx, query.AccountType, query.Date)
	} else {
		balance, err = server.ecommerceClient.QueryPlatformFundBalance(ctx, query.AccountType)
	}
	if err != nil {
		ctx.JSON(http.StatusBadGateway, internalError(ctx, fmt.Errorf("query platform fund balance: %w", err)))
		return
	}

	ctx.JSON(http.StatusOK, platformAccountBalanceResponse{
		AccountType:     query.AccountType,
		BalanceDate:     query.Date,
		AvailableAmount: balance.AvailableAmount,
		PendingAmount:   balance.PendingAmount,
	})
}
