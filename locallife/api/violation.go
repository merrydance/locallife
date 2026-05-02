package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	ordinaryserviceprovider "github.com/merrydance/locallife/wechat/ordinaryserviceprovider"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
)

const (
	violationCallbackType = "violation"
)

const ordinaryViolationCallbackType = "ordinary_violation"
const recoverWayVerifyInactiveMerchantIdentity = "VERIFY_INACTIVE_MERCHANT_IDENTITY"

const (
	ordinaryViolationServiceUnavailableMessage = "普通服务商商户管控能力未完成配置，请联系平台管理员检查微信支付配置后重试"
	platformViolationServiceUnavailableMessage = "微信支付商户管控能力未完成配置，请联系平台管理员检查微信支付配置后重试"
	ordinaryViolationQueryUnavailableMessage   = "普通服务商处置通知配置查询失败，请稍后重试或联系平台管理员检查微信支付配置"
	ordinaryViolationUpsertUnavailableMessage  = "普通服务商处置通知配置失败，请稍后重试或联系平台管理员检查微信支付配置"
	ordinaryViolationDeleteUnavailableMessage  = "普通服务商处置通知配置删除失败，请稍后重试或联系平台管理员检查微信支付配置"
	platformViolationQueryUnavailableMessage   = "微信商户处置通知配置查询失败，请稍后重试或联系平台管理员检查微信支付配置"
	platformViolationUpsertUnavailableMessage  = "微信商户处置通知配置失败，请稍后重试或联系平台管理员检查微信支付配置"
	platformViolationDeleteUnavailableMessage  = "微信商户处置通知配置删除失败，请稍后重试或联系平台管理员检查微信支付配置"
)

type platformViolationNotificationConfigRequest struct {
	NotifyURL *string `json:"notify_url" binding:"omitempty,url"`
}

type platformViolationNotificationConfigResponse struct {
	Configured         bool    `json:"configured"`
	NotifyURL          *string `json:"notify_url,omitempty"`
	EffectiveNotifyURL string  `json:"effective_notify_url"`
}

type platformWechatMerchantViolationResponse struct {
	ID                   int64      `json:"id"`
	RecordID             string     `json:"record_id"`
	SubMchID             string     `json:"sub_mch_id"`
	MerchantID           *int64     `json:"merchant_id,omitempty"`
	CompanyName          string     `json:"company_name"`
	EventType            string     `json:"event_type"`
	RiskType             string     `json:"risk_type"`
	RiskDescription      string     `json:"risk_description"`
	PunishPlan           string     `json:"punish_plan"`
	PunishTime           *time.Time `json:"punish_time,omitempty"`
	PunishDescription    string     `json:"punish_description"`
	LatestNotificationID string     `json:"latest_notification_id"`
	LatestNotifyTime     time.Time  `json:"latest_notify_time"`
	LastReceivedAt       time.Time  `json:"last_received_at"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            *time.Time `json:"updated_at,omitempty"`
}

type listPlatformWechatMerchantViolationsQuery struct {
	MerchantID int64  `form:"merchant_id"`
	SubMchID   string `form:"sub_mch_id"`
	EventType  string `form:"event_type"`
	RiskType   string `form:"risk_type"`
	Page       int32  `form:"page"`
	Limit      int32  `form:"limit"`
}

type listPlatformWechatMerchantViolationsResponse struct {
	Violations []platformWechatMerchantViolationResponse `json:"violations"`
	Page       int32                                     `json:"page"`
	Limit      int32                                     `json:"limit"`
	Total      int64                                     `json:"total"`
}

type ordinaryMerchantLimitationDiagnosticResponse struct {
	SubMchID                            string                                     `json:"sub_mch_id"`
	MchID                               string                                     `json:"mch_id,omitempty"`
	LimitedFunctions                    []string                                   `json:"limited_functions"`
	OtherLimitedFunctions               string                                     `json:"other_limited_functions,omitempty"`
	RecoverySpecifications              []ordinaryMerchantRecoverySpecificationDTO `json:"recovery_specifications"`
	Limitations                         []ordinaryMerchantLimitationDTO            `json:"limitations"`
	CanVerifyInactiveMerchantIdentity   bool                                       `json:"can_verify_inactive_merchant_identity"`
	InactiveMerchantIdentityActionGuide string                                     `json:"inactive_merchant_identity_action_guide,omitempty"`
	MerchantControlActionGuide          string                                     `json:"merchant_control_action_guide"`
}

type ordinaryMerchantRecoverySpecificationDTO struct {
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

type ordinaryMerchantLimitationDTO struct {
	Capability     string   `json:"capability,omitempty"`
	Limited        bool     `json:"limited"`
	Reason         string   `json:"reason,omitempty"`
	RecoverActions []string `json:"recover_actions,omitempty"`
}

type createInactiveMerchantIdentityVerificationRequest struct {
	BusinessCode string `json:"business_code"`
}

type inactiveMerchantIdentityVerificationResponse struct {
	SubMchID       string `json:"sub_mch_id"`
	VerificationID string `json:"verification_id,omitempty"`
	VerifyID       string `json:"verify_id,omitempty"`
	State          string `json:"state,omitempty"`
	FailReason     string `json:"fail_reason,omitempty"`
	Reason         string `json:"reason,omitempty"`
	CreateTime     string `json:"create_time,omitempty"`
	FinishTime     string `json:"finish_time,omitempty"`
	ActionGuide    string `json:"action_guide"`
}

// handleViolationNotify 处理微信商户违规通知回调。
// POST /v1/webhooks/wechat-ecommerce/violation-notify
func (server *Server) handleViolationNotify(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		log.Error().Msg("violation callback received but ecommerce client is not configured")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "ecommerce client not configured",
		})
		return
	}

	body, status, err := readWebhookBody(ctx)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues(violationCallbackType, "read_body").Inc()
		log.Error().Err(err).Msg("read violation notification body")
		ctx.JSON(status, wechatPaymentNotifyResponse{Code: "FAIL", Message: "read body failed"})
		return
	}

	signature := ctx.GetHeader("Wechatpay-Signature")
	timestamp := ctx.GetHeader("Wechatpay-Timestamp")
	nonce := ctx.GetHeader("Wechatpay-Nonce")
	serial := ctx.GetHeader("Wechatpay-Serial")
	if err := server.ecommerceClient.VerifyNotificationSignature(signature, timestamp, nonce, serial, string(body)); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues(violationCallbackType, "signature").Inc()
		log.Error().Err(err).Msg("violation notify: invalid signature")
		ctx.JSON(http.StatusUnauthorized, wechatPaymentNotifyResponse{Code: "FAIL", Message: "signature verification failed"})
		return
	}

	var notification wechat.PaymentNotification
	if err := json.Unmarshal(body, &notification); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues(violationCallbackType, "parse").Inc()
		log.Error().Err(err).Msg("violation notify: parse notification failed")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{Code: "FAIL", Message: "parse notification failed"})
		return
	}

	if !server.tryClaimNotification(ctx, notification, violationCallbackType) {
		return
	}

	if !isSupportedViolationEventType(notification.EventType) {
		log.Info().Str("event_type", notification.EventType).Msg("violation notify: ignore unsupported event")
		server.markNotificationProcessed(ctx, notification.ID, "", "")
		writeWechatNotifySuccess(ctx, violationCallbackType)
		return
	}

	resource, err := server.ecommerceClient.DecryptViolationNotification(&notification)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues(violationCallbackType, "decrypt").Inc()
		log.Error().Err(err).Str("notification_id", notification.ID).Msg("violation notify: decrypt failed")
		server.releaseNotification(ctx, notification.ID, violationCallbackType)
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{Code: "FAIL", Message: "decrypt failed"})
		return
	}

	log.Info().
		Str("notification_id", notification.ID).
		Str("event_type", notification.EventType).
		Str("sub_mchid", resource.SubMchID).
		Str("record_id", resource.RecordID).
		Str("risk_type", resource.RiskType).
		Msg("received merchant violation notification")

	merchantID, err := server.resolveViolationMerchantID(ctx, resource.SubMchID)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues(violationCallbackType, "resolve_merchant").Inc()
		log.Error().Err(err).Str("notification_id", notification.ID).Str("sub_mchid", resource.SubMchID).Msg("violation notify: resolve merchant failed")
		server.releaseNotification(ctx, notification.ID, violationCallbackType)
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "resolve merchant failed"})
		return
	}

	if _, err := server.store.UpsertWechatMerchantViolation(ctx, buildViolationUpsertParams(notification, resource, merchantID)); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues(violationCallbackType, "persist").Inc()
		log.Error().Err(err).Str("notification_id", notification.ID).Str("record_id", resource.RecordID).Msg("violation notify: persist violation failed")
		server.releaseNotification(ctx, notification.ID, violationCallbackType)
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "persist violation failed"})
		return
	}

	server.markNotificationProcessed(ctx, notification.ID, "", "")
	writeWechatNotifySuccess(ctx, violationCallbackType)
}

func (server *Server) handleOrdinaryServiceProviderViolationNotify(ctx *gin.Context) {
	if server.ordinarySPClient == nil {
		log.Error().Msg("ordinary service provider violation callback received but client is not configured")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "ordinary service provider client not configured",
		})
		return
	}

	envelope, err := server.ordinarySPClient.ParseNotification(ctx, ctx.Request, ordinaryserviceprovider.NotificationTargetMerchantViolation)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues(ordinaryViolationCallbackType, "parse").Inc()
		log.Error().Err(err).Msg("ordinary service provider violation notify: parse notification failed")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{Code: "FAIL", Message: "parse notification failed"})
		return
	}
	notification := ordinaryViolationPaymentNotification(envelope)
	if !server.tryClaimNotification(ctx, notification, ordinaryViolationCallbackType) {
		return
	}

	if !isSupportedViolationEventType(notification.EventType) {
		log.Info().Str("event_type", notification.EventType).Msg("ordinary service provider violation notify: ignore unsupported event")
		server.markNotificationProcessed(ctx, notification.ID, "", "")
		writeWechatNotifySuccess(ctx, ordinaryViolationCallbackType)
		return
	}

	resource, err := ordinaryViolationResourceFromEnvelope(envelope)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues(ordinaryViolationCallbackType, "parse_resource").Inc()
		log.Error().Err(err).Str("notification_id", notification.ID).Msg("ordinary service provider violation notify: parse resource failed")
		server.releaseNotification(ctx, notification.ID, ordinaryViolationCallbackType)
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{Code: "FAIL", Message: "parse resource failed"})
		return
	}

	merchantID, err := server.resolveViolationMerchantID(ctx, resource.SubMchID)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues(ordinaryViolationCallbackType, "resolve_merchant").Inc()
		log.Error().Err(err).Str("notification_id", notification.ID).Str("sub_mchid", resource.SubMchID).Msg("ordinary service provider violation notify: resolve merchant failed")
		server.releaseNotification(ctx, notification.ID, ordinaryViolationCallbackType)
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "resolve merchant failed"})
		return
	}

	if _, err := server.store.UpsertWechatMerchantViolation(ctx, buildViolationUpsertParams(notification, resource, merchantID)); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues(ordinaryViolationCallbackType, "persist").Inc()
		log.Error().Err(err).Str("notification_id", notification.ID).Str("record_id", resource.RecordID).Msg("ordinary service provider violation notify: persist violation failed")
		server.releaseNotification(ctx, notification.ID, ordinaryViolationCallbackType)
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "persist violation failed"})
		return
	}

	server.markNotificationProcessed(ctx, notification.ID, "", "")
	writeWechatNotifySuccess(ctx, ordinaryViolationCallbackType)
}

func ordinaryViolationPaymentNotification(envelope *ordinaryserviceprovider.NotificationEnvelope) wechat.PaymentNotification {
	if envelope == nil {
		return wechat.PaymentNotification{}
	}
	createTime := time.Now().UTC()
	if envelope.CreateTime != nil {
		createTime = *envelope.CreateTime
	}
	return wechat.PaymentNotification{
		ID:           strings.TrimSpace(envelope.ID),
		EventType:    strings.TrimSpace(envelope.EventType),
		ResourceType: strings.TrimSpace(envelope.ResourceType),
		Summary:      strings.TrimSpace(envelope.Summary),
		CreateTime:   createTime,
	}
}

func ordinaryViolationResourceFromEnvelope(envelope *ordinaryserviceprovider.NotificationEnvelope) (*wechat.ViolationNotificationResource, error) {
	if envelope == nil {
		return nil, errors.New("ordinary violation notification envelope is nil")
	}
	payload := map[string]any{}
	if strings.TrimSpace(envelope.Plaintext) != "" {
		if err := json.Unmarshal([]byte(envelope.Plaintext), &payload); err != nil {
			return nil, fmt.Errorf("decode ordinary violation plaintext: %w", err)
		}
	} else {
		payload = envelope.Decoded
	}
	resource := &wechat.ViolationNotificationResource{
		SubMchID:          stringFromMap(payload, "sub_mchid"),
		RecordID:          stringFromMap(payload, "record_id"),
		CompanyName:       stringFromMap(payload, "company_name"),
		RiskType:          stringFromMap(payload, "risk_type"),
		RiskDescription:   stringFromMap(payload, "risk_description"),
		PunishPlan:        stringFromMap(payload, "punish_plan"),
		PunishDescription: stringFromMap(payload, "punish_description"),
	}
	if punishTime := stringFromMap(payload, "punish_time"); punishTime != "" {
		parsed, err := time.Parse(time.RFC3339, punishTime)
		if err != nil {
			return nil, fmt.Errorf("ordinary violation notification invalid punish_time: %w", err)
		}
		resource.PunishTime = parsed
	}
	if resource.RecordID == "" {
		return nil, errors.New("ordinary violation notification missing record_id")
	}
	if resource.SubMchID == "" {
		return nil, errors.New("ordinary violation notification missing sub_mchid")
	}
	return resource, nil
}

func stringFromMap(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	raw, ok := values[key]
	if !ok || raw == nil {
		return ""
	}
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func isSupportedViolationEventType(eventType string) bool {
	switch eventType {
	case "VIOLATION.PUNISH", "VIOLATION.INTERCEPT", "VIOLATION.APPEAL":
		return true
	default:
		return false
	}
}

func buildViolationUpsertParams(notification wechat.PaymentNotification, resource *wechat.ViolationNotificationResource, merchantID pgtype.Int8) db.UpsertWechatMerchantViolationParams {
	params := db.UpsertWechatMerchantViolationParams{
		RecordID:             strings.TrimSpace(resource.RecordID),
		SubMchID:             strings.TrimSpace(resource.SubMchID),
		MerchantID:           merchantID,
		CompanyName:          strings.TrimSpace(resource.CompanyName),
		EventType:            notification.EventType,
		RiskType:             strings.TrimSpace(resource.RiskType),
		RiskDescription:      strings.TrimSpace(resource.RiskDescription),
		PunishPlan:           strings.TrimSpace(resource.PunishPlan),
		PunishDescription:    strings.TrimSpace(resource.PunishDescription),
		LatestNotificationID: notification.ID,
		LatestNotifyTime:     notification.CreateTime,
	}
	if !resource.PunishTime.IsZero() {
		params.PunishTime = pgtype.Timestamptz{Time: resource.PunishTime, Valid: true}
	}
	return params
}

func (server *Server) resolveViolationMerchantID(ctx *gin.Context, subMchID string) (pgtype.Int8, error) {
	subMchID = strings.TrimSpace(subMchID)
	if subMchID == "" {
		return pgtype.Int8{}, nil
	}

	paymentConfig, err := server.store.GetMerchantPaymentConfigBySubMchID(ctx, subMchID)
	if err != nil {
		if isNotFoundError(err) {
			return pgtype.Int8{}, nil
		}
		return pgtype.Int8{}, err
	}

	return pgtype.Int8{Int64: paymentConfig.MerchantID, Valid: true}, nil
}

func toPlatformWechatMerchantViolationResponse(record db.WechatMerchantViolation) platformWechatMerchantViolationResponse {
	resp := platformWechatMerchantViolationResponse{
		ID:                   record.ID,
		RecordID:             record.RecordID,
		SubMchID:             record.SubMchID,
		CompanyName:          record.CompanyName,
		EventType:            record.EventType,
		RiskType:             record.RiskType,
		RiskDescription:      record.RiskDescription,
		PunishPlan:           record.PunishPlan,
		PunishDescription:    record.PunishDescription,
		LatestNotificationID: record.LatestNotificationID,
		LatestNotifyTime:     record.LatestNotifyTime,
		LastReceivedAt:       record.LastReceivedAt,
		CreatedAt:            record.CreatedAt,
	}
	if record.MerchantID.Valid {
		resp.MerchantID = &record.MerchantID.Int64
	}
	if record.PunishTime.Valid {
		t := record.PunishTime.Time
		resp.PunishTime = &t
	}
	if record.UpdatedAt.Valid {
		t := record.UpdatedAt.Time
		resp.UpdatedAt = &t
	}
	return resp
}

func bindPlatformWechatMerchantViolationsQuery(ctx *gin.Context) (listPlatformWechatMerchantViolationsQuery, bool) {
	var req listPlatformWechatMerchantViolationsQuery
	if err := ctx.ShouldBindQuery(&req); err != nil {
		respondViolationClientError(ctx, err, "查询条件格式无效，请检查分页和筛选条件后重试", "list_wechat_merchant_violations")
		return listPlatformWechatMerchantViolationsQuery{}, false
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Limit > 100 {
		respondViolationClientError(ctx, errors.New("limit must be between 1 and 100"), "每页最多查询 100 条违规记录，请缩小分页范围后重试", "list_wechat_merchant_violations")
		return listPlatformWechatMerchantViolationsQuery{}, false
	}
	req.SubMchID = strings.TrimSpace(req.SubMchID)
	req.EventType = strings.ToUpper(strings.TrimSpace(req.EventType))
	req.RiskType = strings.ToUpper(strings.TrimSpace(req.RiskType))
	return req, true
}

func toOptionalInt8(value int64) pgtype.Int8 {
	if value <= 0 {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: value, Valid: true}
}

func toOptionalText(value string) pgtype.Text {
	value = strings.TrimSpace(value)
	if value == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: value, Valid: true}
}

func wechatPayStatusCode(err error) (int, bool) {
	var wxErr *wechat.WechatPayError
	if errors.As(err, &wxErr) && wxErr != nil {
		return wxErr.StatusCode, true
	}
	return 0, false
}

func ordinaryProviderStatusCode(err error) (int, bool) {
	var providerErr *ordinaryserviceprovider.ProviderError
	if errors.As(err, &providerErr) && providerErr != nil {
		return providerErr.StatusCode, true
	}
	return 0, false
}

func bindOptionalViolationNotificationConfigRequest(ctx *gin.Context) (platformViolationNotificationConfigRequest, bool) {
	var req platformViolationNotificationConfigRequest
	if ctx.Request.ContentLength == 0 {
		return req, true
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		respondViolationClientError(ctx, err, "处置通知回调地址格式无效，请填写 HTTPS 回调地址后重试", "bind_violation_notification_config")
		return platformViolationNotificationConfigRequest{}, false
	}
	if req.NotifyURL != nil {
		trimmed := strings.TrimSpace(*req.NotifyURL)
		req.NotifyURL = &trimmed
	}
	return req, true
}

func isOrdinaryServiceProviderViolationRoute(ctx *gin.Context) bool {
	if ctx == nil || ctx.Request == nil || ctx.Request.URL == nil {
		return false
	}
	return strings.Contains(ctx.Request.URL.Path, "/wechat-ordinary/")
}

func respondViolationClientError(ctx *gin.Context, err error, publicMessage string, operation string) {
	_ = ctx.Error(err)
	log.Warn().
		Err(err).
		Str("request_id", GetRequestID(ctx)).
		Str("operation", operation).
		Str("path", ctx.Request.URL.Path).
		Msg("wechat merchant violation request rejected")
	ctx.JSON(http.StatusBadRequest, errorResponse(errors.New(publicMessage)))
}

// getPlatformViolationNotificationConfig 查询平台侧当前违规通知回调配置。
// @Summary 查询商户违规通知回调地址
// @Description 管理员查询微信支付平台收付通或普通服务商商户处置通知回调地址；未配置时返回 configured=false
// @Tags 平台财务
// @Produce json
// @Success 200 {object} platformViolationNotificationConfigResponse
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 503 {object} ErrorResponse "支付服务未配置"
// @Failure 502 {object} ErrorResponse "微信支付调用失败"
// @Security BearerAuth
// @Router /v1/platform/finance/wechat-ecommerce/violation-notification [get]
// @Router /v1/platform/finance/wechat-ordinary/violation-notification [get]
func (server *Server) getPlatformViolationNotificationConfig(ctx *gin.Context) {
	if isOrdinaryServiceProviderViolationRoute(ctx) {
		if server.ordinarySPClient == nil {
			err := errors.New("ordinary service provider client not configured")
			ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, ordinaryViolationServiceUnavailableMessage, "ordinary violation notification query client not configured"))
			return
		}
		server.getOrdinaryServiceProviderViolationNotificationConfig(ctx)
		return
	}
	if server.ecommerceClient == nil {
		err := errors.New("ecommerce client not configured")
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, platformViolationServiceUnavailableMessage, "platform violation notification query ecommerce client not configured"))
		return
	}

	response := platformViolationNotificationConfigResponse{
		Configured:         false,
		EffectiveNotifyURL: server.config.EffectiveWechatEcommerceViolationNotifyURL(),
	}

	config, err := server.ecommerceClient.QueryViolationNotification(ctx)
	if err != nil {
		if statusCode, ok := wechatPayStatusCode(err); ok && statusCode == http.StatusNotFound {
			ctx.JSON(http.StatusOK, response)
			return
		}
		ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, fmt.Errorf("query violation notification config: %w", err), platformViolationQueryUnavailableMessage, "query platform violation notification config failed"))
		return
	}

	if config != nil && config.NotifyURL != nil && strings.TrimSpace(*config.NotifyURL) != "" {
		response.Configured = true
		response.NotifyURL = config.NotifyURL
	}

	ctx.JSON(http.StatusOK, response)
}

func (server *Server) getOrdinaryServiceProviderViolationNotificationConfig(ctx *gin.Context) {
	response := platformViolationNotificationConfigResponse{
		Configured:         false,
		EffectiveNotifyURL: server.config.EffectiveWechatOrdinaryViolationNotifyURL(),
	}
	config, err := server.ordinarySPClient.QueryViolationNotificationConfig(ctx)
	if err != nil {
		if statusCode, ok := ordinaryProviderStatusCode(err); ok && statusCode == http.StatusNotFound {
			ctx.JSON(http.StatusOK, response)
			return
		}
		ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, fmt.Errorf("query ordinary service provider violation notification config: %w", err), ordinaryViolationQueryUnavailableMessage, "query ordinary service provider violation notification config failed"))
		return
	}
	if config != nil && strings.TrimSpace(config.NotifyURL) != "" {
		response.Configured = true
		response.NotifyURL = stringPtrIfNotEmpty(config.NotifyURL)
	}
	ctx.JSON(http.StatusOK, response)
}

// createPlatformViolationNotificationConfig 创建平台侧违规通知回调配置。
// @Summary 创建商户违规通知回调地址
// @Description 管理员在微信支付平台收付通或普通服务商侧创建商户处置通知回调地址；未传 notify_url 时使用当前服务配置
// @Tags 平台财务
// @Accept json
// @Produce json
// @Param request body platformViolationNotificationConfigRequest false "回调地址配置"
// @Success 200 {object} platformViolationNotificationConfigResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 503 {object} ErrorResponse "支付服务未配置"
// @Failure 502 {object} ErrorResponse "微信支付调用失败"
// @Security BearerAuth
// @Router /v1/platform/finance/wechat-ecommerce/violation-notification [post]
// @Router /v1/platform/finance/wechat-ordinary/violation-notification [post]
func (server *Server) createPlatformViolationNotificationConfig(ctx *gin.Context) {
	server.upsertPlatformViolationNotificationConfig(ctx, http.MethodPost)
}

// updatePlatformViolationNotificationConfig 更新平台侧违规通知回调配置。
// @Summary 更新商户违规通知回调地址
// @Description 管理员更新微信支付平台收付通或普通服务商商户处置通知回调地址；未传 notify_url 时使用当前服务配置
// @Tags 平台财务
// @Accept json
// @Produce json
// @Param request body platformViolationNotificationConfigRequest false "回调地址配置"
// @Success 200 {object} platformViolationNotificationConfigResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 503 {object} ErrorResponse "支付服务未配置"
// @Failure 502 {object} ErrorResponse "微信支付调用失败"
// @Security BearerAuth
// @Router /v1/platform/finance/wechat-ecommerce/violation-notification [put]
// @Router /v1/platform/finance/wechat-ordinary/violation-notification [put]
func (server *Server) updatePlatformViolationNotificationConfig(ctx *gin.Context) {
	server.upsertPlatformViolationNotificationConfig(ctx, http.MethodPut)
}

func (server *Server) upsertPlatformViolationNotificationConfig(ctx *gin.Context, method string) {
	if isOrdinaryServiceProviderViolationRoute(ctx) {
		if server.ordinarySPClient == nil {
			err := errors.New("ordinary service provider client not configured")
			ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, ordinaryViolationServiceUnavailableMessage, "ordinary violation notification upsert client not configured"))
			return
		}
		server.upsertOrdinaryServiceProviderViolationNotificationConfig(ctx, method)
		return
	}
	if server.ecommerceClient == nil {
		err := errors.New("ecommerce client not configured")
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, platformViolationServiceUnavailableMessage, "platform violation notification upsert ecommerce client not configured"))
		return
	}

	req, ok := bindOptionalViolationNotificationConfigRequest(ctx)
	if !ok {
		return
	}

	var (
		resp *wechat.ViolationNotificationConfigResponse
		err  error
	)
	wechatReq := &wechat.ViolationNotificationConfigRequest{NotifyURL: req.NotifyURL}
	if method == http.MethodPost {
		resp, err = server.ecommerceClient.CreateViolationNotification(ctx, wechatReq)
	} else {
		resp, err = server.ecommerceClient.UpdateViolationNotification(ctx, wechatReq)
	}
	if err != nil {
		ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, fmt.Errorf("%s violation notification config: %w", strings.ToLower(method), err), platformViolationUpsertUnavailableMessage, "upsert platform violation notification config failed"))
		return
	}

	configuredResp := platformViolationNotificationConfigResponse{
		Configured:         resp != nil && resp.NotifyURL != nil && strings.TrimSpace(*resp.NotifyURL) != "",
		NotifyURL:          nil,
		EffectiveNotifyURL: server.config.EffectiveWechatEcommerceViolationNotifyURL(),
	}
	if resp != nil {
		configuredResp.NotifyURL = resp.NotifyURL
	}
	ctx.JSON(http.StatusOK, configuredResp)
}

func (server *Server) upsertOrdinaryServiceProviderViolationNotificationConfig(ctx *gin.Context, method string) {
	req, ok := bindOptionalViolationNotificationConfigRequest(ctx)
	if !ok {
		return
	}
	notifyURL := server.config.EffectiveWechatOrdinaryViolationNotifyURL()
	if req.NotifyURL != nil {
		notifyURL = strings.TrimSpace(*req.NotifyURL)
	}
	if notifyURL == "" {
		err := errors.New("ordinary service provider violation notify url not configured")
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, "普通服务商处置通知回调地址未配置，请先配置后再重试", "ordinary service provider violation notify url missing"))
		return
	}

	var (
		resp *ospcontracts.ViolationNotificationConfigResponse
		err  error
	)
	wechatReq := ospcontracts.ViolationNotificationConfigRequest{NotifyURL: notifyURL}
	if method == http.MethodPost {
		resp, err = server.ordinarySPClient.CreateViolationNotificationConfig(ctx, wechatReq)
	} else {
		resp, err = server.ordinarySPClient.UpdateViolationNotificationConfig(ctx, wechatReq)
	}
	if err != nil {
		ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, fmt.Errorf("%s ordinary service provider violation notification config: %w", strings.ToLower(method), err), ordinaryViolationUpsertUnavailableMessage, "upsert ordinary service provider violation notification config failed"))
		return
	}

	configuredResp := platformViolationNotificationConfigResponse{
		Configured:         resp != nil && strings.TrimSpace(resp.NotifyURL) != "",
		EffectiveNotifyURL: server.config.EffectiveWechatOrdinaryViolationNotifyURL(),
	}
	if resp != nil {
		configuredResp.NotifyURL = stringPtrIfNotEmpty(resp.NotifyURL)
	}
	ctx.JSON(http.StatusOK, configuredResp)
}

// deletePlatformViolationNotificationConfig 删除平台侧违规通知回调配置。
// @Summary 删除商户违规通知回调地址
// @Description 管理员删除微信支付平台收付通或普通服务商商户处置通知回调地址；若微信侧已不存在则幂等返回 204
// @Tags 平台财务
// @Success 204 "删除成功"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 503 {object} ErrorResponse "支付服务未配置"
// @Failure 502 {object} ErrorResponse "微信支付调用失败"
// @Security BearerAuth
// @Router /v1/platform/finance/wechat-ecommerce/violation-notification [delete]
// @Router /v1/platform/finance/wechat-ordinary/violation-notification [delete]
func (server *Server) deletePlatformViolationNotificationConfig(ctx *gin.Context) {
	if isOrdinaryServiceProviderViolationRoute(ctx) {
		if server.ordinarySPClient == nil {
			err := errors.New("ordinary service provider client not configured")
			ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, ordinaryViolationServiceUnavailableMessage, "ordinary violation notification delete client not configured"))
			return
		}
		err := server.ordinarySPClient.DeleteViolationNotificationConfig(ctx)
		if err != nil {
			if statusCode, ok := ordinaryProviderStatusCode(err); ok && statusCode == http.StatusNotFound {
				ctx.Status(http.StatusNoContent)
				return
			}
			ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, fmt.Errorf("delete ordinary service provider violation notification config: %w", err), ordinaryViolationDeleteUnavailableMessage, "delete ordinary service provider violation notification config failed"))
			return
		}
		ctx.Status(http.StatusNoContent)
		return
	}
	if server.ecommerceClient == nil {
		err := errors.New("ecommerce client not configured")
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, platformViolationServiceUnavailableMessage, "platform violation notification delete ecommerce client not configured"))
		return
	}

	err := server.ecommerceClient.DeleteViolationNotification(ctx)
	if err != nil {
		if statusCode, ok := wechatPayStatusCode(err); ok && statusCode == http.StatusNotFound {
			ctx.Status(http.StatusNoContent)
			return
		}
		ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, fmt.Errorf("delete violation notification config: %w", err), platformViolationDeleteUnavailableMessage, "delete platform violation notification config failed"))
		return
	}

	ctx.Status(http.StatusNoContent)
}

func (server *Server) queryOrdinaryMerchantLimitation(ctx *gin.Context, subMchID string) (*ospcontracts.MerchantLimitationQueryResponse, error) {
	if server.ordinarySPClient == nil {
		return nil, errors.New("ordinary service provider client not configured")
	}
	return server.ordinarySPClient.QueryMerchantLimitation(ctx, ospcontracts.MerchantLimitationQueryRequest{SubMchID: subMchID})
}

func bindOrdinarySubMchIDParam(ctx *gin.Context) (string, bool) {
	subMchID := strings.TrimSpace(ctx.Param("sub_mch_id"))
	if subMchID == "" {
		respondViolationClientError(ctx, errors.New("sub_mch_id is required"), "特约商户号不能为空，请选择商户后重试", "bind_ordinary_sub_mch_id")
		return "", false
	}
	return subMchID, true
}

func merchantLimitedFunctionsToStrings(values []ospcontracts.MerchantLimitedFunction) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(string(value)) != "" {
			out = append(out, strings.TrimSpace(string(value)))
		}
	}
	return out
}

func merchantRecoverySpecificationsToDTO(values []ospcontracts.MerchantRecoverySpecification) []ordinaryMerchantRecoverySpecificationDTO {
	out := make([]ordinaryMerchantRecoverySpecificationDTO, 0, len(values))
	for _, value := range values {
		out = append(out, ordinaryMerchantRecoverySpecificationDTO{
			LimitationCaseID:         strings.TrimSpace(value.LimitationCaseID),
			LimitationReasonType:     strings.TrimSpace(value.LimitationReasonType),
			LimitationReason:         strings.TrimSpace(value.LimitationReason),
			LimitationReasonDescribe: strings.TrimSpace(value.LimitationReasonDescribe),
			RelateLimitations:        merchantLimitedFunctionsToStrings(value.RelateLimitations),
			OtherRelateLimitations:   strings.TrimSpace(value.OtherRelateLimitations),
			RecoverWay:               strings.TrimSpace(value.RecoverWay),
			RecoverWayParam:          strings.TrimSpace(value.RecoverWayParam),
			RecoverHelpURL:           strings.TrimSpace(value.RecoverHelpURL),
			LimitationActionType:     strings.TrimSpace(value.LimitationActionType),
			LimitationStartDate:      strings.TrimSpace(value.LimitationStartDate),
			LimitationDate:           strings.TrimSpace(value.LimitationDate),
		})
	}
	return out
}

func merchantLimitationsToDTO(values []ospcontracts.MerchantLimitation) []ordinaryMerchantLimitationDTO {
	out := make([]ordinaryMerchantLimitationDTO, 0, len(values))
	for _, value := range values {
		out = append(out, ordinaryMerchantLimitationDTO{
			Capability:     strings.TrimSpace(value.Capability),
			Limited:        value.Limited,
			Reason:         strings.TrimSpace(value.Reason),
			RecoverActions: value.RecoverActions,
		})
	}
	return out
}

func hasInactiveMerchantIdentityRecoverWay(specs []ospcontracts.MerchantRecoverySpecification) bool {
	for _, spec := range specs {
		if strings.EqualFold(strings.TrimSpace(spec.RecoverWay), recoverWayVerifyInactiveMerchantIdentity) {
			return true
		}
	}
	return false
}

func toOrdinaryMerchantLimitationDiagnosticResponse(subMchID string, resp *ospcontracts.MerchantLimitationQueryResponse) ordinaryMerchantLimitationDiagnosticResponse {
	result := ordinaryMerchantLimitationDiagnosticResponse{
		SubMchID:                   subMchID,
		LimitedFunctions:           []string{},
		RecoverySpecifications:     []ordinaryMerchantRecoverySpecificationDTO{},
		Limitations:                []ordinaryMerchantLimitationDTO{},
		MerchantControlActionGuide: "微信未返回商户管控限制；如支付、退款或分账仍失败，请联系平台核对普通服务商特约商户配置。",
	}
	if resp == nil {
		return result
	}
	if strings.TrimSpace(resp.SubMchID) != "" {
		result.SubMchID = strings.TrimSpace(resp.SubMchID)
	}
	result.MchID = strings.TrimSpace(resp.MchID)
	result.LimitedFunctions = merchantLimitedFunctionsToStrings(resp.LimitedFunctions)
	result.OtherLimitedFunctions = strings.TrimSpace(resp.OtherLimitedFunctions)
	result.RecoverySpecifications = merchantRecoverySpecificationsToDTO(resp.RecoverySpecifications)
	result.Limitations = merchantLimitationsToDTO(resp.Limitations)
	result.CanVerifyInactiveMerchantIdentity = hasInactiveMerchantIdentityRecoverWay(resp.RecoverySpecifications)
	if len(result.LimitedFunctions) > 0 || len(result.RecoverySpecifications) > 0 || len(result.Limitations) > 0 {
		result.MerchantControlActionGuide = "微信返回商户管控限制；请查看恢复说明中的帮助链接或微信支付商户平台指引，处理完成前不要重复发起受限支付能力。"
	}
	if result.CanVerifyInactiveMerchantIdentity {
		result.InactiveMerchantIdentityActionGuide = "微信返回不活跃商户身份核实解脱路径；平台管理员可发起身份核实，完成前前端应提示商户等待平台处理。"
	}
	return result
}

// getOrdinaryMerchantLimitationDiagnostic 查询普通服务商特约商户管控诊断。
// @Summary 查询普通服务商商户管控诊断
// @Description 管理员按特约商户号查询微信普通服务商商户管控能力、原因和恢复动作
// @Tags 平台财务
// @Produce json
// @Param sub_mch_id path string true "特约商户号"
// @Success 200 {object} ordinaryMerchantLimitationDiagnosticResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 503 {object} ErrorResponse "支付服务未配置"
// @Failure 502 {object} ErrorResponse "微信支付调用失败"
// @Security BearerAuth
// @Router /v1/platform/finance/wechat-ordinary/merchant-limitations/{sub_mch_id} [get]
func (server *Server) getOrdinaryMerchantLimitationDiagnostic(ctx *gin.Context) {
	subMchID, ok := bindOrdinarySubMchIDParam(ctx)
	if !ok {
		return
	}
	resp, err := server.queryOrdinaryMerchantLimitation(ctx, subMchID)
	if err != nil {
		if isEcommerceClientNotConfigured(err) {
			ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, ordinaryViolationServiceUnavailableMessage, "query ordinary merchant limitation client not configured"))
			return
		}
		ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, err, "微信商户管控查询失败，请稍后重试或联系平台检查普通服务商配置", "query ordinary merchant limitation failed"))
		return
	}
	ctx.JSON(http.StatusOK, toOrdinaryMerchantLimitationDiagnosticResponse(subMchID, resp))
}

func toInactiveMerchantIdentityVerificationResponse(subMchID string, createResp *ospcontracts.InactiveMerchantIdentityVerificationCreateResponse, queryResp *ospcontracts.InactiveMerchantIdentityVerificationQueryResponse) inactiveMerchantIdentityVerificationResponse {
	resp := inactiveMerchantIdentityVerificationResponse{
		SubMchID:    strings.TrimSpace(subMchID),
		ActionGuide: "不活跃商户身份核实已提交，请等待微信支付处理；完成前前端应提示商户等待平台通知，不要重复提交。",
	}
	if createResp != nil {
		resp.VerificationID = strings.TrimSpace(createResp.VerificationID)
		resp.VerifyID = strings.TrimSpace(createResp.VerifyID)
	}
	if queryResp != nil {
		if strings.TrimSpace(queryResp.SubMchID) != "" {
			resp.SubMchID = strings.TrimSpace(queryResp.SubMchID)
		}
		resp.VerificationID = strings.TrimSpace(queryResp.VerificationID)
		resp.VerifyID = strings.TrimSpace(queryResp.VerifyID)
		resp.State = strings.TrimSpace(string(queryResp.State))
		resp.FailReason = strings.TrimSpace(queryResp.FailReason)
		resp.Reason = strings.TrimSpace(queryResp.Reason)
		resp.CreateTime = strings.TrimSpace(queryResp.CreateTime)
		resp.FinishTime = strings.TrimSpace(queryResp.FinishTime)
		resp.ActionGuide = "请根据微信支付返回的身份核实状态处理；处理中请等待，成功后可重试受限能力，失败时请按失败原因联系平台继续处理。"
	}
	return resp
}

// createInactiveMerchantIdentityVerification 发起不活跃商户身份核实。
// @Summary 发起不活跃商户身份核实
// @Description 仅当商户管控诊断返回 VERIFY_INACTIVE_MERCHANT_IDENTITY 解脱路径时，管理员可发起身份核实
// @Tags 平台财务
// @Accept json
// @Produce json
// @Param sub_mch_id path string true "特约商户号"
// @Param request body createInactiveMerchantIdentityVerificationRequest false "业务申请编号"
// @Success 200 {object} inactiveMerchantIdentityVerificationResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 409 {object} ErrorResponse "当前商户不允许发起身份核实"
// @Failure 503 {object} ErrorResponse "支付服务未配置"
// @Failure 502 {object} ErrorResponse "微信支付调用失败"
// @Security BearerAuth
// @Router /v1/platform/finance/wechat-ordinary/merchant-limitations/{sub_mch_id}/inactive-identity-verifications [post]
func (server *Server) createInactiveMerchantIdentityVerification(ctx *gin.Context) {
	subMchID, ok := bindOrdinarySubMchIDParam(ctx)
	if !ok {
		return
	}
	var req createInactiveMerchantIdentityVerificationRequest
	if ctx.Request.ContentLength > 0 {
		if err := ctx.ShouldBindJSON(&req); err != nil {
			respondViolationClientError(ctx, err, "身份核实请求格式无效，请检查填写内容后重试", "create_inactive_merchant_identity_verification")
			return
		}
		req.BusinessCode = strings.TrimSpace(req.BusinessCode)
	}
	limitation, err := server.queryOrdinaryMerchantLimitation(ctx, subMchID)
	if err != nil {
		if isEcommerceClientNotConfigured(err) {
			ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, ordinaryViolationServiceUnavailableMessage, "inactive merchant identity verification limitation query client not configured"))
			return
		}
		ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, err, "微信商户管控查询失败，暂不能发起身份核实，请稍后重试", "inactive merchant identity verification limitation query failed"))
		return
	}
	if limitation == nil || !hasInactiveMerchantIdentityRecoverWay(limitation.RecoverySpecifications) {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("当前商户未返回不活跃商户身份核实解脱路径，请先按商户管控诊断中的恢复指引处理")))
		return
	}
	resp, err := server.ordinarySPClient.CreateInactiveMerchantIdentityVerification(ctx, ospcontracts.InactiveMerchantIdentityVerificationCreateRequest{
		SubMchID:     subMchID,
		BusinessCode: req.BusinessCode,
	})
	if err != nil {
		ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, err, "微信不活跃商户身份核实提交失败，请稍后重试或联系平台检查普通服务商配置", "create inactive merchant identity verification failed"))
		return
	}
	ctx.JSON(http.StatusOK, toInactiveMerchantIdentityVerificationResponse(subMchID, resp, nil))
}

// getInactiveMerchantIdentityVerification 查询不活跃商户身份核实状态。
// @Summary 查询不活跃商户身份核实状态
// @Description 管理员查询普通服务商不活跃商户身份核实状态
// @Tags 平台财务
// @Produce json
// @Param sub_mch_id path string true "特约商户号"
// @Param verification_id path string true "身份核实单号"
// @Success 200 {object} inactiveMerchantIdentityVerificationResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 503 {object} ErrorResponse "支付服务未配置"
// @Failure 502 {object} ErrorResponse "微信支付调用失败"
// @Security BearerAuth
// @Router /v1/platform/finance/wechat-ordinary/merchant-limitations/{sub_mch_id}/inactive-identity-verifications/{verification_id} [get]
func (server *Server) getInactiveMerchantIdentityVerification(ctx *gin.Context) {
	subMchID, ok := bindOrdinarySubMchIDParam(ctx)
	if !ok {
		return
	}
	verificationID := strings.TrimSpace(ctx.Param("verification_id"))
	if verificationID == "" {
		respondViolationClientError(ctx, errors.New("verification_id is required"), "身份核实单号不能为空，请刷新后重试", "bind_inactive_merchant_identity_verification_id")
		return
	}
	if server.ordinarySPClient == nil {
		err := errors.New("ordinary service provider client not configured")
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, ordinaryViolationServiceUnavailableMessage, "query inactive merchant identity verification client not configured"))
		return
	}
	resp, err := server.ordinarySPClient.QueryInactiveMerchantIdentityVerification(ctx, ospcontracts.InactiveMerchantIdentityVerificationQueryRequest{
		SubMchID:       subMchID,
		VerificationID: verificationID,
	})
	if err != nil {
		ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, err, "微信不活跃商户身份核实状态查询失败，请稍后重试或联系平台检查普通服务商配置", "query inactive merchant identity verification failed"))
		return
	}
	ctx.JSON(http.StatusOK, toInactiveMerchantIdentityVerificationResponse(subMchID, nil, resp))
}

// listPlatformWechatMerchantViolations 查询平台侧违规记录列表。
// @Summary 查询商户违规记录列表
// @Description 管理员分页查看微信支付平台收付通或普通服务商商户处置记录，支持按 merchant_id、sub_mch_id、event_type、risk_type 过滤
// @Tags 平台财务
// @Produce json
// @Param merchant_id query int false "商户ID"
// @Param sub_mch_id query string false "二级商户号"
// @Param event_type query string false "违规事件类型"
// @Param risk_type query string false "风险类型"
// @Param page query int false "页码，默认 1"
// @Param limit query int false "每页数量，默认 20，最大 100"
// @Success 200 {object} listPlatformWechatMerchantViolationsResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /v1/platform/finance/wechat-ecommerce/violations [get]
// @Router /v1/platform/finance/wechat-ordinary/violations [get]
func (server *Server) listPlatformWechatMerchantViolations(ctx *gin.Context) {
	req, ok := bindPlatformWechatMerchantViolationsQuery(ctx)
	if !ok {
		return
	}

	listParams := db.ListWechatMerchantViolationsParams{
		MerchantID: toOptionalInt8(req.MerchantID),
		SubMchID:   toOptionalText(req.SubMchID),
		EventType:  toOptionalText(req.EventType),
		RiskType:   toOptionalText(req.RiskType),
		Limit:      req.Limit,
		Offset:     (req.Page - 1) * req.Limit,
	}
	countParams := db.CountWechatMerchantViolationsParams{
		MerchantID: toOptionalInt8(req.MerchantID),
		SubMchID:   toOptionalText(req.SubMchID),
		EventType:  toOptionalText(req.EventType),
		RiskType:   toOptionalText(req.RiskType),
	}

	total, err := server.store.CountWechatMerchantViolations(ctx, countParams)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	rows, err := server.store.ListWechatMerchantViolations(ctx, listParams)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	violations := make([]platformWechatMerchantViolationResponse, 0, len(rows))
	for _, row := range rows {
		violations = append(violations, toPlatformWechatMerchantViolationResponse(row))
	}

	ctx.JSON(http.StatusOK, listPlatformWechatMerchantViolationsResponse{
		Violations: violations,
		Page:       req.Page,
		Limit:      req.Limit,
		Total:      total,
	})
}

// getPlatformWechatMerchantViolation 获取平台侧单条违规记录详情。
// @Summary 获取商户违规记录详情
// @Description 管理员按 record_id 查询单条微信支付平台收付通或普通服务商商户处置记录
// @Tags 平台财务
// @Produce json
// @Param record_id path string true "微信侧违规记录ID"
// @Success 200 {object} platformWechatMerchantViolationResponse
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 404 {object} ErrorResponse "违规记录不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /v1/platform/finance/wechat-ecommerce/violations/{record_id} [get]
// @Router /v1/platform/finance/wechat-ordinary/violations/{record_id} [get]
func (server *Server) getPlatformWechatMerchantViolation(ctx *gin.Context) {
	recordID := strings.TrimSpace(ctx.Param("record_id"))
	if recordID == "" {
		respondViolationClientError(ctx, errors.New("record_id is required"), "违规记录编号不能为空，请刷新后重试", "bind_wechat_merchant_violation_record_id")
		return
	}

	record, err := server.store.GetWechatMerchantViolationByRecordID(ctx, recordID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("未找到该违规记录，请刷新列表后重试")))
		} else {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		}
		return
	}

	ctx.JSON(http.StatusOK, toPlatformWechatMerchantViolationResponse(record))
}
