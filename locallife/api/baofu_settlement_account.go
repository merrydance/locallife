package api

import (
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/merrydance/locallife/baofu"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/token"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// getMerchantBaofuSettlementAccount 查询当前商户宝付结算账户开通状态
// @Summary 查询当前商户宝付结算账户
// @Description 商户老板查询宝付二级户开户、微信商户报备与小程序授权目录状态；只返回产品状态、下一步指引和脱敏信息，不暴露宝付上游原始字段
// @Tags 商户财务
// @Produce json
// @Success 200 {object} baofuSettlementAccountResponse
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "仅商户老板可访问"
// @Failure 503 {object} ErrorResponse "宝付结算账户状态暂不可用"
// @Security BearerAuth
// @Router /v1/merchant/settlement-account [get]
func (server *Server) getMerchantBaofuSettlementAccount(ctx *gin.Context) {
	scope, ok := merchantBaofuSettlementAccountScope(ctx)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("merchant not loaded, ensure MerchantStaffMiddleware is applied")))
		return
	}
	server.getBaofuSettlementAccount(ctx, scope)
}

// createMerchantBaofuSettlementAccount 提交或继续当前商户宝付结算账户开户
// @Summary 提交当前商户宝付结算账户开户
// @Description 商户老板提交角色限定的开户资料或继续已有流程；owner_type、account_type、industry_id、qualificationTransSerialNo、platformNo、platformTerminalId 均由服务端控制
// @Tags 商户财务
// @Accept json
// @Produce json
// @Param request body baofuSettlementAccountRequest false "开户资料；空 body 表示继续当前流程"
// @Success 202 {object} baofuSettlementAccountResponse
// @Failure 400 {object} ErrorResponse "请求参数错误或包含服务端控制字段"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "仅商户老板可访问"
// @Failure 503 {object} ErrorResponse "宝付开户服务暂不可用"
// @Security BearerAuth
// @Router /v1/merchant/settlement-account [post]
func (server *Server) createMerchantBaofuSettlementAccount(ctx *gin.Context) {
	scope, ok := merchantBaofuSettlementAccountScope(ctx)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("merchant not loaded, ensure MerchantStaffMiddleware is applied")))
		return
	}
	server.createBaofuSettlementAccount(ctx, scope)
}

// getRiderBaofuSettlementAccount 查询当前骑手宝付结算账户开通状态
// @Summary 查询当前骑手宝付结算账户
// @Description 骑手查询个人宝付二级户开户状态、核验费支付状态和下一步指引；不返回明文身份证、银行卡或宝付上游原始字段
// @Tags 骑手
// @Produce json
// @Success 200 {object} baofuSettlementAccountResponse
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "无骑手权限"
// @Failure 503 {object} ErrorResponse "宝付结算账户状态暂不可用"
// @Security BearerAuth
// @Router /v1/rider/settlement-account [get]
func (server *Server) getRiderBaofuSettlementAccount(ctx *gin.Context) {
	rider, ok := GetRiderFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("rider not loaded, ensure RiderMiddleware is applied")))
		return
	}
	server.getBaofuSettlementAccount(ctx, baofuSettlementAccountScope{
		OwnerType:           db.BaofuAccountOwnerTypeRider,
		OwnerID:             rider.ID,
		OwnerUserID:         rider.UserID,
		AccountType:         db.BaofuAccountTypePersonal,
		Audience:            "rider",
		DefaultProfile:      baofuRiderDefaultProfile(rider),
		DefaultProfileMasks: baofuRiderDefaultProfileMasks(rider),
	})
}

// createRiderBaofuSettlementAccount 提交或继续当前骑手宝付结算账户开户
// @Summary 提交当前骑手宝付结算账户开户
// @Description 骑手提交个人开户资料或继续已有流程；核验费通过微信直连支付给平台后进入宝付开户，服务端控制 owner/account/provider 字段
// @Tags 骑手
// @Accept json
// @Produce json
// @Param request body baofuSettlementAccountRequest false "开户资料；空 body 表示继续当前流程"
// @Success 202 {object} baofuSettlementAccountResponse
// @Failure 400 {object} ErrorResponse "请求参数错误或包含服务端控制字段"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "无骑手权限"
// @Failure 503 {object} ErrorResponse "宝付开户服务暂不可用"
// @Security BearerAuth
// @Router /v1/rider/settlement-account [post]
func (server *Server) createRiderBaofuSettlementAccount(ctx *gin.Context) {
	rider, ok := GetRiderFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("rider not loaded, ensure RiderMiddleware is applied")))
		return
	}
	server.createBaofuSettlementAccount(ctx, baofuSettlementAccountScope{
		OwnerType:           db.BaofuAccountOwnerTypeRider,
		OwnerID:             rider.ID,
		OwnerUserID:         rider.UserID,
		AccountType:         db.BaofuAccountTypePersonal,
		Audience:            "rider",
		DefaultProfile:      baofuRiderDefaultProfile(rider),
		DefaultProfileMasks: baofuRiderDefaultProfileMasks(rider),
	})
}

// getOperatorBaofuSettlementAccount 查询当前运营商宝付结算账户开通状态
// @Summary 查询当前运营商宝付结算账户
// @Description 运营商查询个人宝付二级户开户状态、核验费支付状态和下一步指引；运营商不做微信商户报备或授权目录绑定
// @Tags 运营商财务
// @Produce json
// @Success 200 {object} baofuSettlementAccountResponse
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "无运营商权限"
// @Failure 503 {object} ErrorResponse "宝付结算账户状态暂不可用"
// @Security BearerAuth
// @Router /v1/operators/me/settlement-account [get]
func (server *Server) getOperatorBaofuSettlementAccount(ctx *gin.Context) {
	operator, ok := GetOperatorFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("operator not loaded, ensure LoadOperatorMiddleware is applied")))
		return
	}
	server.getBaofuSettlementAccount(ctx, baofuSettlementAccountScope{
		OwnerType:           db.BaofuAccountOwnerTypeOperator,
		OwnerID:             operator.ID,
		OwnerUserID:         operator.UserID,
		AccountType:         db.BaofuAccountTypePersonal,
		Audience:            "operator",
		DefaultProfile:      baofuOperatorDefaultProfile(operator),
		DefaultProfileMasks: baofuOperatorDefaultProfileMasks(operator),
	})
}

// createOperatorBaofuSettlementAccount 提交或继续当前运营商宝付结算账户开户
// @Summary 提交当前运营商宝付结算账户开户
// @Description 运营商提交个人开户资料或继续已有流程；核验费通过微信直连支付给平台后进入宝付开户，服务端控制 owner/account/provider 字段
// @Tags 运营商财务
// @Accept json
// @Produce json
// @Param request body baofuSettlementAccountRequest false "开户资料；空 body 表示继续当前流程"
// @Success 202 {object} baofuSettlementAccountResponse
// @Failure 400 {object} ErrorResponse "请求参数错误或包含服务端控制字段"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "无运营商权限"
// @Failure 503 {object} ErrorResponse "宝付开户服务暂不可用"
// @Security BearerAuth
// @Router /v1/operators/me/settlement-account [post]
func (server *Server) createOperatorBaofuSettlementAccount(ctx *gin.Context) {
	operator, ok := GetOperatorFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("operator not loaded, ensure LoadOperatorMiddleware is applied")))
		return
	}
	server.createBaofuSettlementAccount(ctx, baofuSettlementAccountScope{
		OwnerType:           db.BaofuAccountOwnerTypeOperator,
		OwnerID:             operator.ID,
		OwnerUserID:         operator.UserID,
		AccountType:         db.BaofuAccountTypePersonal,
		Audience:            "operator",
		DefaultProfile:      baofuOperatorDefaultProfile(operator),
		DefaultProfileMasks: baofuOperatorDefaultProfileMasks(operator),
	})
}

// getPlatformBaofuSettlementAccount 查询平台宝付结算账户开通状态
// @Summary 查询平台宝付结算账户
// @Description 管理员查询平台宝付机构二级户开户状态和下一步指引；平台不做微信商户报备或授权目录绑定
// @Tags 平台财务
// @Produce json
// @Success 200 {object} baofuSettlementAccountResponse
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "无管理员权限"
// @Failure 503 {object} ErrorResponse "宝付结算账户状态暂不可用"
// @Security BearerAuth
// @Router /v1/platform/finance/settlement-account [get]
func (server *Server) getPlatformBaofuSettlementAccount(ctx *gin.Context) {
	server.getBaofuSettlementAccount(ctx, baofuSettlementAccountScope{
		OwnerType:   db.BaofuAccountOwnerTypePlatform,
		OwnerID:     platformBaofuAccountOwnerID,
		AccountType: db.BaofuAccountTypeBusiness,
		Audience:    "platform",
	})
}

// createPlatformBaofuSettlementAccount 提交或继续平台宝付结算账户开户
// @Summary 提交平台宝付结算账户开户
// @Description 管理员提交平台机构开户资料或继续已有流程；平台核验费由平台承担，服务端控制 owner/account/provider 字段
// @Tags 平台财务
// @Accept json
// @Produce json
// @Param request body baofuSettlementAccountRequest false "开户资料；空 body 表示继续当前流程"
// @Success 202 {object} baofuSettlementAccountResponse
// @Failure 400 {object} ErrorResponse "请求参数错误或包含服务端控制字段"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "无管理员权限"
// @Failure 503 {object} ErrorResponse "宝付开户服务暂不可用"
// @Security BearerAuth
// @Router /v1/platform/finance/settlement-account [post]
func (server *Server) createPlatformBaofuSettlementAccount(ctx *gin.Context) {
	server.createBaofuSettlementAccount(ctx, baofuSettlementAccountScope{
		OwnerType:   db.BaofuAccountOwnerTypePlatform,
		OwnerID:     platformBaofuAccountOwnerID,
		AccountType: db.BaofuAccountTypeBusiness,
		Audience:    "platform",
	})
}

func merchantBaofuSettlementAccountScope(ctx *gin.Context) (baofuSettlementAccountScope, bool) {
	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		return baofuSettlementAccountScope{}, false
	}
	return baofuSettlementAccountScope{
		OwnerType:   db.BaofuAccountOwnerTypeMerchant,
		OwnerID:     merchant.ID,
		OwnerUserID: merchant.OwnerUserID,
		AccountType: db.BaofuAccountTypeBusiness,
		Audience:    "merchant",
	}, true
}

func (server *Server) getBaofuSettlementAccount(ctx *gin.Context, scope baofuSettlementAccountScope) {
	resp, err := server.loadBaofuSettlementAccount(ctx, scope)
	if err != nil {
		ctx.JSON(http.StatusServiceUnavailable, loggedBaofuSettlementAccountServerError(ctx, err, scope, "宝付结算账户状态暂不可用，请稍后刷新；如持续失败请联系平台处理", "baofu settlement account status load failed"))
		return
	}
	ctx.JSON(http.StatusOK, resp)
}

func (server *Server) createBaofuSettlementAccount(ctx *gin.Context, scope baofuSettlementAccountScope) {
	req, err := decodeBaofuSettlementAccountRequest(ctx, scope)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New(baofuSettlementAccountDecodeErrorPublicMessage(err))))
		return
	}
	scope, err = baofuSettlementAccountScopeForRequest(scope, req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New(baofuSettlementAccountDecodeErrorPublicMessage(err))))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	profile := req.toOpeningProfileInput()
	if shouldMergeBaofuSettlementAccountProfileDefaults(scope, profile) {
		profile, err = server.baofuSettlementAccountProfileInputWithDefaults(ctx, scope, profile)
		if err != nil {
			if writeBaofuSettlementAccountLogicRequestError(ctx, err, scope) {
				return
			}
			ctx.JSON(http.StatusServiceUnavailable, loggedBaofuSettlementAccountServerError(ctx, err, scope, "宝付开户资料暂不可用，请稍后重试；如持续失败请联系平台处理", "baofu settlement account profile defaults load failed"))
			return
		}
	}
	service := server.newBaofuAccountOnboardingService()
	result, err := service.StartOrRecoverOpening(ctx, logic.BaofuAccountOpeningInput{
		OwnerType:          scope.OwnerType,
		OwnerID:            scope.OwnerID,
		UserID:             authPayload.UserID,
		ClientIP:           ctx.ClientIP(),
		AccountOpeningMode: req.AccountOpeningMode,
		Profile:            profile,
	})
	if err != nil {
		if writeBaofuSettlementAccountLogicRequestError(ctx, err, scope) {
			return
		}
		if isBaofuSettlementAccountServiceNotReady(err) {
			ctx.JSON(http.StatusServiceUnavailable, loggedBaofuSettlementAccountServerError(ctx, err, scope, "宝付结算账户服务未配置，请联系平台处理", baofuSettlementAccountServiceNotReady))
			return
		}
		ctx.JSON(http.StatusServiceUnavailable, loggedBaofuSettlementAccountServerError(ctx, err, scope, "宝付开户服务暂不可用，请稍后重试；如持续失败请联系平台处理", "baofu settlement account start failed"))
		return
	}
	ctx.JSON(http.StatusAccepted, server.baofuSettlementAccountResponseFromResult(scope, result))
}

func baofuSettlementAccountScopeForRequest(scope baofuSettlementAccountScope, req baofuSettlementAccountRequest) (baofuSettlementAccountScope, error) {
	mode := normalizeBaofuSettlementAccountOpeningModeValue(req.AccountOpeningMode)
	if strings.TrimSpace(mode) == "" {
		return scope, nil
	}
	if strings.TrimSpace(scope.OwnerType) != db.BaofuAccountOwnerTypeMerchant {
		return baofuSettlementAccountScope{}, baofuSettlementAccountRoleFieldError{
			Field:     "account_opening_mode",
			Role:      scope.Audience,
			OwnerType: scope.OwnerType,
		}
	}
	switch mode {
	case db.BaofuAccountTypeBusiness, db.BaofuAccountTypePersonal:
		scope.AccountType = mode
		return scope, nil
	default:
		return baofuSettlementAccountScope{}, baofuSettlementAccountRoleFieldError{
			Field:     "account_opening_mode",
			Role:      scope.Audience,
			OwnerType: scope.OwnerType,
		}
	}
}

func writeBaofuSettlementAccountLogicRequestError(ctx *gin.Context, err error, scope baofuSettlementAccountScope) bool {
	var reqErr *logic.RequestError
	if !errors.As(err, &reqErr) {
		return false
	}
	logErr := logic.LoggableError(reqErr)
	_ = ctx.Error(logErr)
	event := log.Error().
		Err(logErr).
		Str("request_id", GetRequestID(ctx)).
		Str("path", ctx.Request.URL.Path).
		Str("method", ctx.Request.Method).
		Int("status", reqErr.Status).
		Str("owner_type", scope.OwnerType).
		Int64("owner_id", scope.OwnerID).
		Str("account_type", scope.AccountType).
		Str("audience", scope.Audience)
	hasProviderContext := false
	if providerContext, ok := logic.BaofuProviderErrorContextFromError(logErr); ok {
		hasProviderContext = true
		event = event.
			Int64("flow_id", providerContext.FlowID).
			Str("owner_type", providerContext.OwnerType).
			Int64("owner_id", providerContext.OwnerID).
			Str("open_trans_serial_no", providerContext.OpenTransSerialNo).
			Str("current_state", providerContext.CurrentState).
			Int64("merchant_report_id", providerContext.MerchantReportID).
			Str("merchant_report_no", providerContext.MerchantReportNo).
			Str("provider_operation", providerContext.ProviderOperation).
			Str("provider_capability", providerContext.ProviderCapability)
	}
	event = writeBaofuSettlementAccountProviderErrorLogFields(event, logErr, hasProviderContext)
	event.Msg("baofu settlement account request rejected")
	ctx.JSON(reqErr.Status, errorResponse(errors.New(logicRequestErrorPublicMessage(reqErr))))
	return true
}

func writeBaofuSettlementAccountProviderErrorLogFields(event *zerolog.Event, err error, hasProviderContext bool) *zerolog.Event {
	var providerErr *baofu.ProviderError
	if !errors.As(err, &providerErr) || providerErr == nil {
		return event
	}
	event = event.
		Str("provider_method", strings.TrimSpace(providerErr.Operation)).
		Str("upstream_code", strings.TrimSpace(providerErr.UpstreamCode)).
		Str("frontend_code", strings.TrimSpace(providerErr.Frontend.Code)).
		Bool("retryable", providerErr.Frontend.Retryable)
	if !hasProviderContext {
		event = event.Str("provider_capability", strings.TrimSpace(providerErr.Capability))
	}
	if providerErr.StatusCode != 0 {
		event = event.Int("http_status", providerErr.StatusCode)
	}
	if cause := errors.Unwrap(providerErr); cause != nil {
		event = event.Str("provider_error_cause", strings.TrimSpace(cause.Error()))
	}
	if sanitized := baofuProviderUpstreamMessageForLog(providerErr.UpstreamMessage); sanitized != "" {
		event = event.Str("upstream_message_sanitized", sanitized)
	}
	return event
}

var (
	baofuProviderIDCardPattern = regexp.MustCompile(`([1-9]\d{5})(\d{8})(\d{3}[0-9Xx])`)
	baofuProviderMobilePattern = regexp.MustCompile(`\b(1[3-9]\d)(\d{4})(\d{4})\b`)
	baofuProviderBankPattern   = regexp.MustCompile(`\b\d{12,25}\b`)
)

func baofuProviderUpstreamMessageForLog(message string) string {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return ""
	}
	sanitized := baofuProviderIDCardPattern.ReplaceAllString(trimmed, `$1********$3`)
	sanitized = baofuProviderMobilePattern.ReplaceAllString(sanitized, `$1****$3`)
	sanitized = baofuProviderBankPattern.ReplaceAllStringFunc(sanitized, func(value string) string {
		if len(value) <= 4 {
			return strings.Repeat("*", len(value))
		}
		return strings.Repeat("*", len(value)-4) + value[len(value)-4:]
	})
	if sanitized == trimmed {
		return ""
	}
	return sanitized
}

func loggedBaofuSettlementAccountServerError(ctx *gin.Context, err error, scope baofuSettlementAccountScope, publicMessage string, logMessage string) ErrorResponse {
	_ = ctx.Error(err)
	event := log.Error().
		Err(err).
		Str("request_id", GetRequestID(ctx)).
		Str("path", ctx.Request.URL.Path).
		Str("method", ctx.Request.Method).
		Str("owner_type", scope.OwnerType).
		Int64("owner_id", scope.OwnerID).
		Str("account_type", scope.AccountType).
		Str("audience", scope.Audience)
	if paymentErr := new(baofuSettlementAccountPaymentLoadError); errors.As(err, paymentErr) {
		event = event.
			Int64("flow_id", paymentErr.FlowID).
			Int64("payment_order_id", paymentErr.PaymentOrderID)
	}
	event.Msg(logMessage)
	return ErrorResponse{Error: publicMessage}
}

func (server *Server) newBaofuAccountOnboardingService() *logic.BaofuAccountOnboardingService {
	service := logic.NewBaofuAccountOnboardingService(server.store, server.baofuAccountClient, server.directPaymentClient, server.dataEncryptor, logic.BaofuAccountOnboardingConfig{
		VerifyFeeFen:      server.config.BaofuAccountVerifyFeeFen,
		IndustryID:        server.config.BaofuBusinessIndustryID,
		CollectMerchantID: server.config.BaofuCollectMerchantID,
	})
	if server.baofuMerchantReportClient != nil {
		service = service.WithMerchantReportContinuation(server.baofuMerchantReportClient, logic.BaofuAccountMerchantReportConfig{
			CollectMerchantID: server.config.BaofuCollectMerchantID,
			CollectTerminalID: server.config.BaofuCollectTerminalID,
			MiniProgramAppID:  server.config.WechatMiniAppID,
			ChannelID:         server.config.BaofuMerchantReportChannelID,
			ChannelName:       server.config.BaofuMerchantReportChannelName,
			Business:          server.config.BaofuMerchantReportBusiness,
		})
	}
	return service
}
