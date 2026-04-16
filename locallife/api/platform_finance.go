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

type platformFundFlowBillDownloadQuery struct {
	BillDate    string `form:"bill_date" binding:"required"`
	AccountType string `form:"account_type"`
	TarType     string `form:"tar_type"`
}

type platformProfitSharingBillDownloadQuery struct {
	BillDate string `form:"bill_date" binding:"required"`
	SubMchID string `form:"sub_mchid"`
	TarType  string `form:"tar_type"`
}

type platformBillDownloadResponse struct {
	BillDate    string `json:"bill_date"`
	AccountType string `json:"account_type,omitempty"`
	SubMchID    string `json:"sub_mch_id,omitempty"`
	TarType     string `json:"tar_type,omitempty"`
	HashType    string `json:"hash_type"`
	HashValue   string `json:"hash_value"`
	DownloadURL string `json:"download_url"`
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

func normalizeWechatBillDate(value string) (time.Time, string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, "", errors.New("bill_date is required")
	}

	billDate, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, "", errors.New("bill_date must use YYYY-MM-DD format")
	}

	return billDate, billDate.Format("2006-01-02"), nil
}

func normalizeWechatBillTarType(value string) (string, error) {
	return wechatcontracts.NormalizeFundManagementTarType(value)
}

func bindPlatformFundFlowBillDownloadQuery(ctx *gin.Context) (time.Time, platformFundFlowBillDownloadQuery, bool) {
	var req platformFundFlowBillDownloadQuery
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return time.Time{}, platformFundFlowBillDownloadQuery{}, false
	}

	billDate, normalizedDate, err := normalizeWechatBillDate(req.BillDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return time.Time{}, platformFundFlowBillDownloadQuery{}, false
	}
	req.BillDate = normalizedDate

	req.AccountType, err = wechatcontracts.NormalizeFundManagementPlatformAccountType(req.AccountType)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return time.Time{}, platformFundFlowBillDownloadQuery{}, false
	}

	req.TarType, err = normalizeWechatBillTarType(req.TarType)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return time.Time{}, platformFundFlowBillDownloadQuery{}, false
	}

	return billDate, req, true
}

func bindPlatformProfitSharingBillDownloadQuery(ctx *gin.Context) (time.Time, platformProfitSharingBillDownloadQuery, bool) {
	var req platformProfitSharingBillDownloadQuery
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return time.Time{}, platformProfitSharingBillDownloadQuery{}, false
	}

	billDate, normalizedDate, err := normalizeWechatBillDate(req.BillDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return time.Time{}, platformProfitSharingBillDownloadQuery{}, false
	}
	req.BillDate = normalizedDate
	req.SubMchID = strings.TrimSpace(req.SubMchID)
	req.TarType, err = normalizeWechatBillTarType(req.TarType)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return time.Time{}, platformProfitSharingBillDownloadQuery{}, false
	}

	return billDate, req, true
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

func respondFundFlowBillDownloadError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, wechat.ErrBillNotReady):
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("微信资金账单生成中，请稍后重试")))
	case errors.Is(err, wechat.ErrBillNotFound):
		ctx.JSON(http.StatusNotFound, errorResponse(errors.New("所选日期暂无微信资金账单")))
	default:
		var contractErr *wechatcontracts.FundManagementContractError
		if errors.As(err, &contractErr) {
			ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, fmt.Errorf("get fund flow bill download url: %w", err), "微信资金账单返回数据异常，请稍后重试", "fund flow bill download url contract invalid"))
			return
		}

		var wxErr *wechat.WechatPayError
		if errors.As(err, &wxErr) && wechaterrorcodes.FundManagementCodeEquals(wxErr.Code, wechaterrorcodes.FundManagementCodeNoAuth) {
			ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, fmt.Errorf("get fund flow bill download url: %w", err), "微信侧暂无资金账单下载权限，请联系管理员检查收付通配置", "fund flow bill download permission denied"))
			return
		}

		ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, fmt.Errorf("get fund flow bill download url: %w", err), "微信资金账单下载地址获取失败，请稍后重试", "fund flow bill download url failed"))
	}
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

// getPlatformFundFlowBillDownloadURL 获取平台资金账单下载地址
// @Summary 获取平台资金账单下载地址
// @Description 管理员调用微信支付资金账单接口，获取平台商户号账单下载地址
// @Tags 平台财务
// @Produce json
// @Param bill_date query string true "账单日期，格式 YYYY-MM-DD"
// @Param account_type query string false "账户类型，默认 BASIC" Enums(BASIC,OPERATION,FEES)
// @Param tar_type query string false "压缩类型，默认 GZIP" Enums(GZIP)
// @Success 200 {object} platformBillDownloadResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 502 {object} ErrorResponse "微信支付调用失败"
// @Failure 503 {object} ErrorResponse "微信支付未配置"
// @Security BearerAuth
// @Router /v1/platform/finance/bills/fund-flow/download-url [get]
func (server *Server) getPlatformFundFlowBillDownloadURL(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(errors.New("ecommerce client not configured")))
		return
	}

	billDate, req, ok := bindPlatformFundFlowBillDownloadQuery(ctx)
	if !ok {
		return
	}

	resp, err := server.ecommerceClient.GetFundFlowBillDownloadURL(ctx, billDate, req.AccountType, req.TarType)
	if err != nil {
		respondFundFlowBillDownloadError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, platformBillDownloadResponse{
		BillDate:    req.BillDate,
		AccountType: req.AccountType,
		TarType:     req.TarType,
		HashType:    resp.HashType,
		HashValue:   resp.HashValue,
		DownloadURL: resp.DownloadURL,
	})
}

// getPlatformProfitSharingBillDownloadURL 获取平台分账账单下载地址
// @Summary 获取平台分账账单下载地址
// @Description 管理员调用微信支付分账账单接口，获取平台或指定子商户分账账单下载地址
// @Tags 平台财务
// @Produce json
// @Param bill_date query string true "账单日期，格式 YYYY-MM-DD"
// @Param sub_mchid query string false "指定子商户号；不传则获取服务商下全部分账账单"
// @Param tar_type query string false "压缩类型，默认 GZIP" Enums(GZIP)
// @Success 200 {object} platformBillDownloadResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 502 {object} ErrorResponse "微信支付调用失败"
// @Failure 503 {object} ErrorResponse "微信支付未配置"
// @Security BearerAuth
// @Router /v1/platform/finance/bills/profit-sharing/download-url [get]
func (server *Server) getPlatformProfitSharingBillDownloadURL(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(errors.New("ecommerce client not configured")))
		return
	}

	billDate, req, ok := bindPlatformProfitSharingBillDownloadQuery(ctx)
	if !ok {
		return
	}

	resp, err := server.ecommerceClient.GetProfitSharingBillDownloadURL(ctx, billDate, req.SubMchID, req.TarType)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, internalError(ctx, fmt.Errorf("get profit sharing bill download url: %w", err)))
		return
	}

	ctx.JSON(http.StatusOK, platformBillDownloadResponse{
		BillDate:    req.BillDate,
		SubMchID:    req.SubMchID,
		TarType:     req.TarType,
		HashType:    resp.HashType,
		HashValue:   resp.HashValue,
		DownloadURL: resp.DownloadURL,
	})
}
