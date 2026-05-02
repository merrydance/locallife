package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	wechaterrorcodes "github.com/merrydance/locallife/wechat/errorcodes"
)

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

type fundBalanceAccountTypeNormalizer func(string) (string, error)

func bindSubMerchantFundBalanceQuery(ctx *gin.Context) (fundBalanceQueryRequest, bool) {
	return bindFundBalanceQuery(ctx, wechatcontracts.NormalizeFundManagementSubMerchantRealtimeAccountType, wechatcontracts.NormalizeFundManagementSubMerchantDayEndAccountType)
}

func bindPlatformFundBalanceQuery(ctx *gin.Context) (fundBalanceQueryRequest, bool) {
	return bindFundBalanceQuery(ctx, wechatcontracts.NormalizeFundManagementPlatformAccountType, wechatcontracts.NormalizeFundManagementPlatformAccountType)
}

func bindFundBalanceQuery(ctx *gin.Context, normalizeRealtimeAccountType, normalizeDayEndAccountType fundBalanceAccountTypeNormalizer) (fundBalanceQueryRequest, bool) {
	var req fundBalanceQueryRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return fundBalanceQueryRequest{}, false
	}

	req.Date = strings.TrimSpace(req.Date)

	if req.Date != "" {
		if _, err := time.Parse("2006-01-02", req.Date); err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("date must use YYYY-MM-DD format")))
			return fundBalanceQueryRequest{}, false
		}
		accountType, err := normalizeDayEndAccountType(req.AccountType)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return fundBalanceQueryRequest{}, false
		}
		req.AccountType = accountType
		return req, true
	}

	accountType, err := normalizeRealtimeAccountType(req.AccountType)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return fundBalanceQueryRequest{}, false
	}
	req.AccountType = accountType

	return req, true
}

func loadSubMerchantFundBalance(ctx *gin.Context, client wechat.EcommerceClientInterface, subMchID string, query fundBalanceQueryRequest) (*wechat.EcommerceFundBalanceResponse, error) {
	if query.Date != "" {
		return client.QueryEcommerceFundDayEndBalance(ctx, subMchID, query.Date, query.AccountType)
	}
	return client.QueryEcommerceFundBalanceByAccountType(ctx, subMchID, query.AccountType)
}

func respondFundBalanceQueryError(ctx *gin.Context, operation string, err error) {
	var contractErr *wechatcontracts.FundManagementContractError
	if errors.As(err, &contractErr) {
		ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, fmt.Errorf("%s: %w", operation, err), "微信资金账户返回数据异常，请稍后重试", operation+": upstream contract invalid"))
		return
	}

	var wxErr *wechat.WechatPayError
	if errors.As(err, &wxErr) && wechaterrorcodes.FundManagementCodeEquals(wxErr.Code, wechaterrorcodes.FundManagementCodeNoAuth) {
		ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, fmt.Errorf("%s: %w", operation, err), "微信侧暂无该账户查询权限，请联系管理员检查收付通配置", operation+": permission denied"))
		return
	}

	ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, fmt.Errorf("%s: %w", operation, err), "微信资金账户查询失败，请稍后重试", operation+": upstream failed"))
}

// getPlatformAccountBalance 查询平台微信支付账户余额
// @Summary 查询平台微信支付账户余额
// @Description 管理员查询历史平台收付通商户号微信支付账户实时余额；普通服务商模式不支持平台内余额查询，应前往微信支付商户平台/商家助手处理
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
		err := errors.New("ecommerce client not configured")
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, "平台收付通资金查询服务未配置；普通服务商模式请前往微信支付商户平台/商家助手处理资金操作", "platform account balance ecommerce client not configured"))
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
		respondFundBalanceQueryError(ctx, "query platform fund balance", err)
		return
	}

	ctx.JSON(http.StatusOK, platformAccountBalanceResponse{
		AccountType:     query.AccountType,
		BalanceDate:     query.Date,
		AvailableAmount: balance.AvailableAmount,
		PendingAmount:   balance.PendingAmount,
	})
}
