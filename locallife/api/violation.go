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

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
)

const (
	violationCallbackType = "violation"
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
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return listPlatformWechatMerchantViolationsQuery{}, false
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Limit > 100 {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("limit must be between 1 and 100")))
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

func bindOptionalViolationNotificationConfigRequest(ctx *gin.Context) (platformViolationNotificationConfigRequest, bool) {
	var req platformViolationNotificationConfigRequest
	if ctx.Request.ContentLength == 0 {
		return req, true
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return platformViolationNotificationConfigRequest{}, false
	}
	if req.NotifyURL != nil {
		trimmed := strings.TrimSpace(*req.NotifyURL)
		req.NotifyURL = &trimmed
	}
	return req, true
}

// getPlatformViolationNotificationConfig 查询平台侧当前违规通知回调配置。
// @Summary 查询商户违规通知回调地址
// @Description 管理员查询微信支付平台收付通商户违规通知回调地址；未配置时返回 configured=false
// @Tags 平台财务
// @Produce json
// @Success 200 {object} platformViolationNotificationConfigResponse
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 503 {object} ErrorResponse "支付服务未配置"
// @Failure 502 {object} ErrorResponse "微信支付调用失败"
// @Security BearerAuth
// @Router /v1/platform/finance/wechat-ecommerce/violation-notification [get]
func (server *Server) getPlatformViolationNotificationConfig(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		err := errors.New("ecommerce client not configured")
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, "ecommerce client not configured", "platform violation notification query ecommerce client not configured"))
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
		ctx.JSON(http.StatusBadGateway, internalError(ctx, fmt.Errorf("query violation notification config: %w", err)))
		return
	}

	if config != nil && config.NotifyURL != nil && strings.TrimSpace(*config.NotifyURL) != "" {
		response.Configured = true
		response.NotifyURL = config.NotifyURL
	}

	ctx.JSON(http.StatusOK, response)
}

// createPlatformViolationNotificationConfig 创建平台侧违规通知回调配置。
// @Summary 创建商户违规通知回调地址
// @Description 管理员在微信支付平台收付通侧创建商户违规通知回调地址；未传 notify_url 时使用当前服务配置
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
func (server *Server) createPlatformViolationNotificationConfig(ctx *gin.Context) {
	server.upsertPlatformViolationNotificationConfig(ctx, http.MethodPost)
}

// updatePlatformViolationNotificationConfig 更新平台侧违规通知回调配置。
// @Summary 更新商户违规通知回调地址
// @Description 管理员更新微信支付平台收付通商户违规通知回调地址；未传 notify_url 时使用当前服务配置
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
func (server *Server) updatePlatformViolationNotificationConfig(ctx *gin.Context) {
	server.upsertPlatformViolationNotificationConfig(ctx, http.MethodPut)
}

func (server *Server) upsertPlatformViolationNotificationConfig(ctx *gin.Context, method string) {
	if server.ecommerceClient == nil {
		err := errors.New("ecommerce client not configured")
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, "ecommerce client not configured", "platform violation notification upsert ecommerce client not configured"))
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
		ctx.JSON(http.StatusBadGateway, internalError(ctx, fmt.Errorf("%s violation notification config: %w", strings.ToLower(method), err)))
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

// deletePlatformViolationNotificationConfig 删除平台侧违规通知回调配置。
// @Summary 删除商户违规通知回调地址
// @Description 管理员删除微信支付平台收付通商户违规通知回调地址；若微信侧已不存在则幂等返回 204
// @Tags 平台财务
// @Success 204 "删除成功"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 503 {object} ErrorResponse "支付服务未配置"
// @Failure 502 {object} ErrorResponse "微信支付调用失败"
// @Security BearerAuth
// @Router /v1/platform/finance/wechat-ecommerce/violation-notification [delete]
func (server *Server) deletePlatformViolationNotificationConfig(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		err := errors.New("ecommerce client not configured")
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, "ecommerce client not configured", "platform violation notification delete ecommerce client not configured"))
		return
	}

	err := server.ecommerceClient.DeleteViolationNotification(ctx)
	if err != nil {
		if statusCode, ok := wechatPayStatusCode(err); ok && statusCode == http.StatusNotFound {
			ctx.Status(http.StatusNoContent)
			return
		}
		ctx.JSON(http.StatusBadGateway, internalError(ctx, fmt.Errorf("delete violation notification config: %w", err)))
		return
	}

	ctx.Status(http.StatusNoContent)
}

// listPlatformWechatMerchantViolations 查询平台侧违规记录列表。
// @Summary 查询商户违规记录列表
// @Description 管理员分页查看微信支付平台收付通商户违规记录，支持按 merchant_id、sub_mch_id、event_type、risk_type 过滤
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
// @Description 管理员按 record_id 查询单条微信支付平台收付通商户违规记录
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
func (server *Server) getPlatformWechatMerchantViolation(ctx *gin.Context) {
	recordID := strings.TrimSpace(ctx.Param("record_id"))
	if recordID == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("record_id is required")))
		return
	}

	record, err := server.store.GetWechatMerchantViolationByRecordID(ctx, recordID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("violation record not found")))
		} else {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		}
		return
	}

	ctx.JSON(http.StatusOK, toPlatformWechatMerchantViolationResponse(record))
}
