package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/wechat"
	"github.com/merrydance/locallife/worker"
	"github.com/rs/zerolog/log"
)

// ===========================================================================
// 用户投诉处理 API
// 路由：
//   商户：  GET  /v1/merchant/complaints             listMerchantComplaints
//           GET  /v1/merchant/complaints/:id         getMerchantComplaintDetail
//           POST /v1/merchant/complaints/:id/response respondToComplaint
//           POST /v1/merchant/complaints/:id/complete completeComplaint
//   运营商：GET  /v1/operator/complaints             listPendingComplaints
//   Webhook: POST /v1/webhooks/wechat-ecommerce/complaint-notify handleComplaintNotify
// ===========================================================================

// complaintResponse HTTP 投诉单响应体
type complaintResponse struct {
	ID              int64      `json:"id"`
	ComplaintID     string     `json:"complaint_id"`
	ComplaintTime   time.Time  `json:"complaint_time"`
	PayerOpenID     *string    `json:"payer_openid,omitempty"`
	ComplaintDetail string     `json:"complaint_detail"`
	ComplaintState  string     `json:"complaint_state"`
	TransactionID   *string    `json:"transaction_id,omitempty"`
	OutTradeNo      *string    `json:"out_trade_no,omitempty"`
	Amount          int64      `json:"amount"`
	ResponseContent *string    `json:"response_content,omitempty"`
	RespondedAt     *time.Time `json:"responded_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	LastSyncedAt    time.Time  `json:"last_synced_at"`
	WxpayUpdateTime *time.Time `json:"wxpay_update_time,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func toComplaintResponse(c db.WechatComplaint) complaintResponse {
	resp := complaintResponse{
		ID:              c.ID,
		ComplaintID:     c.ComplaintID,
		ComplaintTime:   c.ComplaintTime,
		ComplaintDetail: c.ComplaintDetail,
		ComplaintState:  c.ComplaintState,
		Amount:          c.Amount,
		LastSyncedAt:    c.LastSyncedAt.Time,
		CreatedAt:       c.CreatedAt,
		UpdatedAt:       c.UpdatedAt.Time,
	}
	if c.PayerOpenid.Valid {
		resp.PayerOpenID = &c.PayerOpenid.String
	}
	if c.TransactionID.Valid {
		resp.TransactionID = &c.TransactionID.String
	}
	if c.OutTradeNo.Valid {
		resp.OutTradeNo = &c.OutTradeNo.String
	}
	if c.ResponseContent.Valid {
		resp.ResponseContent = &c.ResponseContent.String
	}
	if c.RespondedAt.Valid {
		t := c.RespondedAt.Time
		resp.RespondedAt = &t
	}
	if c.CompletedAt.Valid {
		t := c.CompletedAt.Time
		resp.CompletedAt = &t
	}
	if c.WxpayUpdateTime.Valid {
		t := c.WxpayUpdateTime.Time
		resp.WxpayUpdateTime = &t
	}
	return resp
}

// ========================= 商户端：查看自己的投诉列表 =========================

// listMerchantComplaints 商户查看自己收到的投诉列表
// GET /v1/merchant/complaints
func (server *Server) listMerchantComplaints(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.getMerchantFromUserID(ctx, authPayload.UserID)
	if err != nil {
		return // getMerchantFromUserID 已写响应
	}

	state := ctx.Query("state") // 可选过滤：PENDING_RESPONSE / PROCESSING / PROCESSED
	pageStr := ctx.Query("page")
	limitStr := ctx.Query("limit")

	page := int32(1)
	limit := int32(20)
	if v, err2 := strconv.Atoi(pageStr); err2 == nil && v > 0 {
		page = int32(v)
	}
	if v, err2 := strconv.Atoi(limitStr); err2 == nil && v > 0 && v <= 100 {
		limit = int32(v)
	}
	offset := (page - 1) * limit

	rows, err := server.store.ListWechatComplaintsByMerchant(ctx, db.ListWechatComplaintsByMerchantParams{
		MerchantID: pgtype.Int8{Int64: merchant.ID, Valid: true},
		Column2:    state, // sqlc 代参数名 Column2，对应 ($2::text IS NULL OR complaint_state = $2)
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := make([]complaintResponse, 0, len(rows))
	for _, r := range rows {
		resp = append(resp, toComplaintResponse(r))
	}
	ctx.JSON(http.StatusOK, gin.H{"complaints": resp, "page": page, "limit": limit})
}

// ========================= 商户端：投诉单详情 =================================

// getMerchantComplaintDetail 商户查看投诉单详情并验证归属
// GET /v1/merchant/complaints/:id
func (server *Server) getMerchantComplaintDetail(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.getMerchantFromUserID(ctx, authPayload.UserID)
	if err != nil {
		return
	}

	complaintID := ctx.Param("id")
	complaint, err := server.store.GetWechatComplaintByComplaintID(ctx, complaintID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("complaint not found")))
		} else {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		}
		return
	}

	// 检查投诉是否属于该商户
	if !complaint.MerchantID.Valid || complaint.MerchantID.Int64 != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("complaint does not belong to your merchant")))
		return
	}

	ctx.JSON(http.StatusOK, toComplaintResponse(complaint))
}

// ========================= 商户端：回复投诉 ====================================

type respondToComplaintRequest struct {
	ResponseContent string `json:"response_content" binding:"required,min=1,max=256"`
	JumpURL         string `json:"jump_url"`
}

// respondToComplaint 商户回复投诉（同时调用微信 API + 更新本地状态）
// POST /v1/merchant/complaints/:id/response
func (server *Server) respondToComplaint(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(errors.New("payment service not configured")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.getMerchantFromUserID(ctx, authPayload.UserID)
	if err != nil {
		return
	}

	complaintID := ctx.Param("id")

	var req respondToComplaintRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 查询并验证投诉归属（FOR UPDATE 防并发）
	complaint, err := server.store.GetWechatComplaintByComplaintIDForUpdate(ctx, complaintID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("complaint not found")))
		} else {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		}
		return
	}
	if !complaint.MerchantID.Valid || complaint.MerchantID.Int64 != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("complaint does not belong to your merchant")))
		return
	}
	if complaint.ComplaintState == string(wechat.ComplaintStateProcessed) {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("complaint is already completed")))
		return
	}

	// 调用微信 API 回复投诉
	if err := server.ecommerceClient.RespondComplaint(ctx, wechat.ComplaintResponseRequest{
		ComplaintID:     complaintID,
		ResponseContent: req.ResponseContent,
		JumpURL:         req.JumpURL,
	}); err != nil {
		log.Error().Err(err).Str("complaint_id", complaintID).Msg("respond complaint: wxpay api failed")
		ctx.JSON(http.StatusBadGateway, errorResponse(errors.New("failed to submit response to WeChat")))
		return
	}

	// 本地持久化回复内容（与上方微信调用成功后才写入，保证一致性）
	updated, err := server.store.UpdateWechatComplaintResponse(ctx, db.UpdateWechatComplaintResponseParams{
		ID:              complaint.ID,
		ResponseContent: pgtype.Text{String: req.ResponseContent, Valid: true},
	})
	if err != nil {
		// 微信已回复成功，本地持久化失败属于降级情形，记录告警但不返回错误给商户
		log.Error().Err(err).Int64("complaint_db_id", complaint.ID).Msg("respond complaint: db update failed after wxpay success")
		ctx.JSON(http.StatusOK, gin.H{"message": "response submitted to WeChat but local DB update failed, will sync later"})
		return
	}

	ctx.JSON(http.StatusOK, toComplaintResponse(updated))
}

// ========================= 商户/运营商：完结投诉 ===============================

// completeComplaint 将投诉标记为已完结（向微信发送完结请求并更新本地状态）
// POST /v1/merchant/complaints/:id/complete （商户端）
// POST /v1/operator/complaints/:id/complete （运营商端，路由复用同一 handler）
func (server *Server) completeComplaint(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(errors.New("payment service not configured")))
		return
	}

	complaintID := ctx.Param("id")

	complaint, err := server.store.GetWechatComplaintByComplaintIDForUpdate(ctx, complaintID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("complaint not found")))
		} else {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		}
		return
	}
	if complaint.ComplaintState == string(wechat.ComplaintStateProcessed) {
		ctx.JSON(http.StatusOK, toComplaintResponse(complaint)) // 已完结，幂等返回 200
		return
	}

	// 调用微信 API 完结投诉
	if err := server.ecommerceClient.CompleteComplaint(ctx, complaintID); err != nil {
		log.Error().Err(err).Str("complaint_id", complaintID).Msg("complete complaint: wxpay api failed")
		ctx.JSON(http.StatusBadGateway, errorResponse(errors.New("failed to complete complaint via WeChat")))
		return
	}

	// 本地标记完结
	updated, err := server.store.UpdateWechatComplaintCompleted(ctx, complaint.ID)
	if err != nil {
		log.Error().Err(err).Int64("complaint_db_id", complaint.ID).Msg("complete complaint: db update failed after wxpay success")
		ctx.JSON(http.StatusOK, gin.H{"message": "complaint completed at WeChat but local DB update failed, will sync"})
		return
	}

	ctx.JSON(http.StatusOK, toComplaintResponse(updated))
}

// ========================= 运营商端：待处理投诉列表 ============================

// listPendingComplaints 运营商查看所有待处理投诉（state=PENDING_RESPONSE 或 PROCESSING）
// GET /v1/operator/complaints
func (server *Server) listPendingComplaints(ctx *gin.Context) {
	pageStr := ctx.Query("page")
	limitStr := ctx.Query("limit")

	page := int32(1)
	limit := int32(20)
	if v, err2 := strconv.Atoi(pageStr); err2 == nil && v > 0 {
		page = int32(v)
	}
	if v, err2 := strconv.Atoi(limitStr); err2 == nil && v > 0 && v <= 100 {
		limit = int32(v)
	}
	offset := (page - 1) * limit

	rows, err := server.store.ListPendingWechatComplaints(ctx, db.ListPendingWechatComplaintsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := make([]complaintResponse, 0, len(rows))
	for _, r := range rows {
		resp = append(resp, toComplaintResponse(r))
	}
	ctx.JSON(http.StatusOK, gin.H{"complaints": resp, "page": page, "limit": limit})
}

// ========================= Webhook：微信投诉通知 ================================

// handleComplaintNotify 处理微信投诉通知回调
// POST /v1/webhooks/wechat-ecommerce/complaint-notify
// 微信在投诉状态变更（新投诉、完结等）时推送此通知
func (server *Server) handleComplaintNotify(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code: "FAIL", Message: "payment client not configured",
		})
		return
	}

	body, status, err := readWebhookBody(ctx)
	if err != nil {
		log.Error().Err(err).Msg("read complaint notification body")
		ctx.JSON(status, wechatPaymentNotifyResponse{Code: "FAIL", Message: "read body failed"})
		return
	}

	// 验证微信签名
	signature := ctx.GetHeader("Wechatpay-Signature")
	timestamp := ctx.GetHeader("Wechatpay-Timestamp")
	nonce := ctx.GetHeader("Wechatpay-Nonce")

	if err := server.ecommerceClient.VerifyNotificationSignature(signature, timestamp, nonce, string(body)); err != nil {
		log.Error().Err(err).Msg("complaint notify: invalid signature")
		ctx.JSON(http.StatusUnauthorized, wechatPaymentNotifyResponse{Code: "FAIL", Message: "signature verification failed"})
		return
	}

	var notification wechat.PaymentNotification
	if err := json.Unmarshal(body, &notification); err != nil {
		log.Error().Err(err).Msg("complaint notify: parse notification failed")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{Code: "FAIL", Message: "parse notification failed"})
		return
	}

	// 幂等检查：通知 ID 已处理则直接返回 200
	exists, err := server.store.CheckNotificationExists(ctx, notification.ID)
	if err != nil {
		log.Error().Err(err).Str("notification_id", notification.ID).Msg("complaint notify: check idempotency failed")
	} else if exists {
		ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{Code: "SUCCESS", Message: "OK"})
		return
	}

	// 仅处理投诉相关事件
	if notification.EventType != "COMPLAINT.STATE_CHANGE" && notification.EventType != "COMPLAINT.CLOSE" {
		log.Info().Str("event_type", notification.EventType).Msg("complaint notify: ignore non-complaint event")
		ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{Code: "SUCCESS", Message: "OK"})
		return
	}

	// 解密通知内容
	complaintNotification, err := server.ecommerceClient.DecryptComplaintNotification(&notification)
	if err != nil {
		log.Error().Err(err).Str("notification_id", notification.ID).Msg("complaint notify: decrypt failed")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{Code: "FAIL", Message: "decrypt failed"})
		return
	}

	log.Info().
		Str("notification_id", notification.ID).
		Str("complaint_id", complaintNotification.ComplaintID).
		Str("action_type", complaintNotification.ActionType).
		Str("state", string(complaintNotification.State)).
		Msg("received complaint notification")

	// 更新本地投诉状态
	_, updateErr := server.store.UpdateWechatComplaintState(ctx, db.UpdateWechatComplaintStateParams{
		ComplaintID:     complaintNotification.ComplaintID,
		ComplaintState:  string(complaintNotification.State),
		WxpayUpdateTime: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if updateErr != nil && !isNotFoundError(updateErr) {
		// 本地无此投诉记录时触发后台同步拉取
		log.Warn().Err(updateErr).Str("complaint_id", complaintNotification.ComplaintID).Msg("complaint notify: db update failed, will enqueue sync")
	}

	// 若本地无此投诉，入队同步任务拉取完整数据
	if isNotFoundError(updateErr) {
		_ = server.taskDistributor.DistributeTaskSyncComplaints(ctx, &worker.SyncComplaintsPayload{
			BeginDate: time.Now().AddDate(0, 0, -1).Format("2006-01-02"),
			EndDate:   time.Now().Format("2006-01-02"),
		})
	}

	// 快速响应微信，返回 200
	ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{Code: "SUCCESS", Message: "OK"})
}
