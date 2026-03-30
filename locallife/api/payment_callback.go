package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/websocket"
	"github.com/merrydance/locallife/wechat"
	"github.com/merrydance/locallife/worker"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
)

// ==================== 微信支付回调 ====================

// wechatPaymentNotifyResponse 微信支付回调响应
type wechatPaymentNotifyResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

const maxWebhookBodyBytes int64 = 1 << 20 // 1MB
const notificationClaimStaleWindow = 2 * time.Minute
const notificationReleaseReasonProcessingFailed = "processing_failed"

type directPaymentOwnershipProvider interface {
	GetMchID() string
	GetAppID() string
}

func readWebhookBody(ctx *gin.Context) ([]byte, int, error) {
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxWebhookBodyBytes)
	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return nil, http.StatusRequestEntityTooLarge, err
		}
		return nil, http.StatusBadRequest, err
	}
	return body, 0, nil
}

func (server *Server) markNotificationProcessed(ctx context.Context, notificationID, outTradeNo, transactionID string) {
	sqlStore, ok := server.store.(*db.SQLStore)
	if !ok {
		return
	}
	if err := sqlStore.MarkWechatNotificationProcessed(ctx, notificationID, outTradeNo, transactionID); err != nil {
		log.Error().Err(err).Str("notification_id", notificationID).Msg("mark notification processed failed")
	}
}

func (server *Server) handleDuplicateClaimedNotification(ctx *gin.Context, notificationID, metricLabel string) bool {
	existing, err := server.store.GetWechatNotification(ctx, notificationID)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues(metricLabel, "duplicate_claim_lookup").Inc()
		log.Error().Err(err).Str("notification_id", notificationID).Str("callback_type", metricLabel).Msg("duplicate notification lookup failed")
		server.sendAlert(websocket.AlertData{
			AlertType:   websocket.AlertTypeSystemError,
			Level:       websocket.AlertLevelCritical,
			Title:       "微信重复回调状态查询失败",
			Message:     fmt.Sprintf("微信回调 notification_id=%s 在重复认领后查询本地状态失败，callback_type=%s。系统已返回 FAIL 等待微信重试，请尽快排查通知表与回调处理链。", notificationID, metricLabel),
			RelatedID:   0,
			RelatedType: "wechat_notification",
			Extra: map[string]interface{}{
				"notification_id": notificationID,
				"callback_type":   metricLabel,
			},
		})
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "notification status lookup failed"})
		return false
	}

	if existing.ProcessedAt.Valid {
		ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{Code: "SUCCESS", Message: "OK"})
		return false
	}

	if existing.CreatedAt.Valid && time.Since(existing.CreatedAt.Time) > notificationClaimStaleWindow {
		server.releaseNotificationWithReason(ctx, notificationID, metricLabel, "stale_claim_retry")
		paymentCallbackFailuresTotal.WithLabelValues(metricLabel, "stale_claim_retry").Inc()
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "stale claim, please retry"})
		return false
	}

	paymentCallbackFailuresTotal.WithLabelValues(metricLabel, "inflight_claim").Inc()
	ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "notification in processing"})
	return false
}

func (server *Server) validateCombineNotifyOwnership(resource *wechat.CombinePaymentNotification) error {
	if server.ecommerceClient == nil {
		return errors.New("ecommerce client not configured")
	}
	if resource.CombineMchID != "" && resource.CombineMchID != server.ecommerceClient.GetSpMchID() {
		return fmt.Errorf("combine_mchid mismatch")
	}
	if resource.CombineAppID != "" && resource.CombineAppID != server.ecommerceClient.GetSpAppID() {
		return fmt.Errorf("combine_appid mismatch")
	}
	return nil
}

func (server *Server) validateEcommerceRefundOwnership(resource *wechat.EcommerceRefundNotification) error {
	if server.ecommerceClient == nil {
		return errors.New("ecommerce client not configured")
	}
	if resource.SpMchID != "" && resource.SpMchID != server.ecommerceClient.GetSpMchID() {
		return fmt.Errorf("sp_mchid mismatch")
	}
	return nil
}

func (server *Server) validateProfitSharingOwnership(resource *wechat.ProfitSharingNotification) error {
	if server.ecommerceClient == nil {
		return errors.New("ecommerce client not configured")
	}
	if resource.MchID != "" && resource.MchID != server.ecommerceClient.GetSpMchID() {
		return fmt.Errorf("mchid mismatch")
	}
	return nil
}

func (server *Server) validateProfitSharingSubMerchantOwnership(ctx context.Context, resource *wechat.ProfitSharingNotification, order db.ProfitSharingOrder) error {
	if resource == nil {
		return errors.New("profit sharing resource is nil")
	}
	if resource.SubMchID == "" {
		return errors.New("sub_mchid missing")
	}
	if order.MerchantID == 0 {
		return errors.New("profit sharing order missing merchant_id")
	}
	paymentConfig, err := server.store.GetMerchantPaymentConfig(ctx, order.MerchantID)
	if err != nil {
		return fmt.Errorf("get merchant payment config: %w", err)
	}
	if paymentConfig.SubMchID == "" {
		return errors.New("merchant payment config sub_mchid missing")
	}
	if resource.SubMchID != paymentConfig.SubMchID {
		return fmt.Errorf("sub_mchid mismatch")
	}
	return nil
}

func (server *Server) validateDirectPaymentOwnership(resource *wechat.PaymentNotificationResource) error {
	if resource == nil {
		return errors.New("payment resource is nil")
	}
	if resource.MchID == "" && resource.AppID == "" {
		return nil
	}
	provider, ok := server.paymentClient.(directPaymentOwnershipProvider)
	if !ok {
		return errors.New("payment client ownership metadata unavailable")
	}
	if resource.MchID != "" && resource.MchID != provider.GetMchID() {
		return fmt.Errorf("mchid mismatch")
	}
	if resource.AppID != "" && resource.AppID != provider.GetAppID() {
		return fmt.Errorf("appid mismatch")
	}
	return nil
}

func (server *Server) validateDirectRefundOwnership(resource *wechat.RefundNotificationResource) error {
	if resource == nil || resource.MchID == "" {
		return nil
	}
	provider, ok := server.paymentClient.(directPaymentOwnershipProvider)
	if !ok {
		return errors.New("payment client ownership metadata unavailable")
	}
	if resource.MchID != provider.GetMchID() {
		return fmt.Errorf("mchid mismatch")
	}
	return nil
}

func buildAmountMismatchRefundOutRefundNo(paymentOrder db.PaymentOrder) string {
	switch {
	case paymentOrder.OrderID.Valid:
		return fmt.Sprintf("RF%d_%d", paymentOrder.ID, paymentOrder.OrderID.Int64)
	case paymentOrder.ReservationID.Valid:
		return fmt.Sprintf("RF%d_R%d", paymentOrder.ID, paymentOrder.ReservationID.Int64)
	case paymentOrder.BusinessType == "membership_recharge":
		return fmt.Sprintf("RFM%d_M", paymentOrder.ID)
	case paymentOrder.BusinessType == "rider_deposit":
		return fmt.Sprintf("RFM%d_D", paymentOrder.ID)
	default:
		return fmt.Sprintf("RFM%d", paymentOrder.ID)
	}
}

func (server *Server) ensureAmountMismatchRefundRecord(ctx context.Context, paymentOrder db.PaymentOrder, refundAmount int64, reason string) (db.RefundOrder, string, error) {
	outRefundNo := buildAmountMismatchRefundOutRefundNo(paymentOrder)
	refundOrder, err := server.store.CreateRefundOrder(ctx, db.CreateRefundOrderParams{
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "amount_mismatch",
		RefundAmount:   refundAmount,
		RefundReason:   pgtype.Text{String: reason, Valid: reason != ""},
		OutRefundNo:    outRefundNo,
		Status:         "pending",
	})
	if err != nil {
		if db.ErrorCode(err) == db.UniqueViolation {
			existing, lookupErr := server.store.GetRefundOrderByOutRefundNo(ctx, outRefundNo)
			if lookupErr != nil {
				return db.RefundOrder{}, outRefundNo, fmt.Errorf("lookup existing amount mismatch refund record: %w", lookupErr)
			}
			return existing, outRefundNo, nil
		}
		return db.RefundOrder{}, outRefundNo, fmt.Errorf("create amount mismatch refund record: %w", err)
	}
	return refundOrder, outRefundNo, nil
}

func buildAmountMismatchRefundPayload(paymentOrder db.PaymentOrder, refundAmount int64, reason string) (*worker.PayloadProcessRefund, bool) {
	payload := &worker.PayloadProcessRefund{
		PaymentOrderID: paymentOrder.ID,
		RefundAmount:   refundAmount,
		Reason:         reason,
	}
	if paymentOrder.OrderID.Valid {
		payload.OrderID = paymentOrder.OrderID.Int64
		return payload, true
	}
	if paymentOrder.ReservationID.Valid {
		payload.ReservationID = paymentOrder.ReservationID.Int64
		return payload, true
	}
	return nil, false
}

func (server *Server) markRefundOrderFailed(ctx context.Context, refundOrderID int64) {
	if refundOrderID == 0 {
		return
	}
	if _, err := server.store.UpdateRefundOrderToFailed(ctx, refundOrderID); err != nil {
		log.Error().Err(err).Int64("refund_order_id", refundOrderID).Msg("mark refund order failed")
	}
}

// tryClaimNotification 原子性地尝试认领通知（INSERT ON CONFLICT DO NOTHING）。
// 返回 false 时已写入 HTTP 响应，调用方直接 return。
func (server *Server) tryClaimNotification(ctx *gin.Context, n wechat.PaymentNotification, metricLabel string) bool {
	claimed, err := server.store.TryClaimWechatNotification(ctx, db.CreateWechatNotificationParams{
		ID:           n.ID,
		EventType:    n.EventType,
		ResourceType: pgtype.Text{String: n.ResourceType, Valid: n.ResourceType != ""},
		Summary:      pgtype.Text{String: n.Summary, Valid: n.Summary != ""},
	})
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues(metricLabel, "claim_notification").Inc()
		log.Error().Err(err).Str("notification_id", n.ID).Msg("claim notification failed")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "internal error"})
		return false
	}
	if !claimed {
		log.Info().Str("notification_id", n.ID).Msg("notification already claimed")
		return server.handleDuplicateClaimedNotification(ctx, n.ID, metricLabel)
	}
	return true
}

// tryClaimApplymentNotification 与 tryClaimNotification 相同，适用于进件通知类型。
func (server *Server) tryClaimApplymentNotification(ctx *gin.Context, n ApplymentStateNotification) bool {
	claimed, err := server.store.TryClaimWechatNotification(ctx, db.CreateWechatNotificationParams{
		ID:           n.ID,
		EventType:    n.EventType,
		ResourceType: pgtype.Text{String: n.ResourceType, Valid: n.ResourceType != ""},
		Summary:      pgtype.Text{String: n.Summary, Valid: n.Summary != ""},
	})
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("applyment", "claim_notification").Inc()
		log.Error().Err(err).Str("notification_id", n.ID).Msg("claim applyment notification failed")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "internal error"})
		return false
	}
	if !claimed {
		log.Info().Str("notification_id", n.ID).Msg("applyment notification already claimed")
		return server.handleDuplicateClaimedNotification(ctx, n.ID, "applyment")
	}
	return true
}

func normalizeNotificationMetricLabel(callbackType string) string {
	callbackType = strings.TrimSpace(callbackType)
	if callbackType == "" {
		return "unknown"
	}
	return callbackType
}

func normalizeNotificationReleaseReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return notificationReleaseReasonProcessingFailed
	}
	return reason
}

// releaseNotification 释放一个已认领但业务逻辑失败的通知占位，允许微信下次重试。
func (server *Server) releaseNotification(ctx context.Context, id string, callbackType ...string) {
	label := ""
	if len(callbackType) > 0 {
		label = callbackType[0]
	}
	server.releaseNotificationWithReason(ctx, id, label, notificationReleaseReasonProcessingFailed)
}

func (server *Server) releaseNotificationWithReason(ctx context.Context, id, callbackType, reason string) {
	callbackType = normalizeNotificationMetricLabel(callbackType)
	reason = normalizeNotificationReleaseReason(reason)
	if err := server.store.ReleaseWechatNotificationClaim(ctx, id); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues(callbackType, "release_claim_"+reason).Inc()
		log.Error().Err(err).Str("notification_id", id).Str("callback_type", callbackType).Str("reason", reason).Msg("release notification claim failed - recovery scheduler will handle")
		server.sendAlert(websocket.AlertData{
			AlertType:   websocket.AlertTypeSystemError,
			Level:       websocket.AlertLevelCritical,
			Title:       "微信通知占位释放失败",
			Message:     fmt.Sprintf("微信通知 %s 在释放 claim 时失败，callback_type=%s, reason=%s。后续重试可能被阻塞，需要尽快排查 notification 表与回调处理链。", id, callbackType, reason),
			RelatedID:   0,
			RelatedType: "wechat_notification",
			Extra: map[string]interface{}{
				"notification_id": id,
				"callback_type":   callbackType,
				"reason":          reason,
				"error":           err.Error(),
			},
		})
	}
}

// handlePaymentNotify 处理微信支付回调通知
// POST /v1/webhooks/wechat-pay/notify
// 设计原则：快速响应微信，耗时操作放入队列
func (server *Server) handlePaymentNotify(ctx *gin.Context) {
	if server.paymentClient == nil {
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "payment client not configured",
		})
		return
	}

	// 读取请求体
	body, status, err := readWebhookBody(ctx)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("payment", "read_body").Inc()
		log.Error().Err(err).Msg("read payment notification body")
		ctx.JSON(status, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "read body failed",
		})
		return
	}

	// 🔒 验证微信签名（关键安全步骤）
	signature := ctx.GetHeader("Wechatpay-Signature")
	timestamp := ctx.GetHeader("Wechatpay-Timestamp")
	nonce := ctx.GetHeader("Wechatpay-Nonce")

	if err := server.paymentClient.VerifyNotificationSignature(signature, timestamp, nonce, string(body)); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("payment", "signature").Inc()
		log.Error().
			Err(err).
			Str("signature", signature).
			Str("timestamp", timestamp).
			Msg("⚠️ invalid wechat signature - possible fake notification")
		ctx.JSON(http.StatusUnauthorized, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "signature verification failed",
		})
		return
	}

	// 解析通知
	var notification wechat.PaymentNotification
	if err := json.Unmarshal(body, &notification); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("payment", "parse").Inc()
		log.Error().Err(err).Msg("parse payment notification")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "parse notification failed",
		})
		return
	}

	// 检查通知类型
	if notification.EventType != "TRANSACTION.SUCCESS" {
		log.Info().Str("event_type", notification.EventType).Msg("ignore non-success notification")
		ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{
			Code:    "SUCCESS",
			Message: "OK",
		})
		return
	}

	// 🔐 #1: 原子幂等性门 — INSERT ON CONFLICT DO NOTHING 防止并发重复处理
	if !server.tryClaimNotification(ctx, notification, "payment") {
		return
	}

	// 解密通知内容
	resource, err := server.paymentClient.DecryptPaymentNotification(&notification)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("payment", "decrypt").Inc()
		log.Error().Err(err).Msg("decrypt payment notification")
		server.releaseNotification(ctx, notification.ID, "payment")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "decrypt failed",
		})
		return
	}

	if err := server.validateDirectPaymentOwnership(resource); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("payment", "ownership").Inc()
		log.Error().Err(err).
			Str("out_trade_no", resource.OutTradeNo).
			Str("mchid", resource.MchID).
			Str("appid", resource.AppID).
			Msg("payment notification ownership validation failed")
		server.sendAlert(websocket.AlertData{
			AlertType:   websocket.AlertTypeSystemError,
			Level:       websocket.AlertLevelCritical,
			Title:       "直连支付回调归属校验失败",
			Message:     fmt.Sprintf("直连支付回调 out_trade_no=%s 的归属校验失败，mchid=%s, appid=%s。系统已返回 FAIL 等待微信重试，请排查是否存在错商户或错应用回调。", resource.OutTradeNo, resource.MchID, resource.AppID),
			RelatedID:   0,
			RelatedType: "payment_order",
			Extra: map[string]interface{}{
				"out_trade_no":   resource.OutTradeNo,
				"transaction_id": resource.TransactionID,
				"mchid":          resource.MchID,
				"appid":          resource.AppID,
			},
		})
		server.releaseNotification(ctx, notification.ID, "payment")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "ownership validation failed",
		})
		return
	}

	log.Info().
		Str("notification_id", notification.ID).
		Str("out_trade_no", resource.OutTradeNo).
		Str("transaction_id", resource.TransactionID).
		Int64("amount", resource.Amount.Total).
		Msg("received payment notification")

	// 查询支付订单
	paymentOrder, err := server.store.GetPaymentOrderByOutTradeNo(ctx, resource.OutTradeNo)
	if err != nil {
		if isNotFoundError(err) {
			paymentCallbackFailuresTotal.WithLabelValues("payment", "payment_order_not_found").Inc()
			log.Error().Str("out_trade_no", resource.OutTradeNo).Msg("payment order not found")
			server.sendAlert(websocket.AlertData{
				AlertType:   websocket.AlertTypeSystemError,
				Level:       websocket.AlertLevelCritical,
				Title:       "支付回调未找到本地支付单",
				Message:     fmt.Sprintf("微信支付回调 out_trade_no=%s, transaction_id=%s 未找到本地 payment_order。系统已返回 FAIL 等待重试，请尽快排查支付单创建与回调时序。", resource.OutTradeNo, resource.TransactionID),
				RelatedID:   0,
				RelatedType: "payment_order",
				Extra: map[string]interface{}{
					"out_trade_no":    resource.OutTradeNo,
					"transaction_id":  resource.TransactionID,
					"business_action": "wechat_retry_expected",
				},
			})
			server.releaseNotification(ctx, notification.ID, "payment")
			ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
				Code:    "FAIL",
				Message: "payment order not found, please retry",
			})
			return
		}
		log.Error().Err(err).Str("out_trade_no", resource.OutTradeNo).Msg("get payment order")
		paymentCallbackFailuresTotal.WithLabelValues("payment", "query_payment_order").Inc()
		server.releaseNotification(ctx, notification.ID, "payment")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "internal error",
		})
		return
	}
	// 检查是否已处理（幂等）
	if paymentOrder.Status == PaymentStatusPaid {
		log.Info().Int64("id", paymentOrder.ID).Msg("payment order already paid")
		server.markNotificationProcessed(ctx, notification.ID, resource.OutTradeNo, resource.TransactionID)
		ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{
			Code:    "SUCCESS",
			Message: "OK",
		})
		return
	}

	// 🚨 检测已关闭/失败订单收到支付回调：用户在关单窗口内完成了支付。
	// 约束说明：UpdatePaymentOrderToPaid 的 SQL 限定 status='pending'，对 closed/failed 订单
	// 无法走正常 paid→refund 链路。此处通过 TaskProcessAnomalyRefund 异步任务直接用
	// TransactionID 发起微信退款，绕过本地状态约束。
	// 通知占位永久保留，作为事故流水记录。
	if paymentOrder.Status == PaymentStatusClosed || paymentOrder.Status == PaymentStatusFailed {
		log.Error().
			Str("out_trade_no", resource.OutTradeNo).
			Str("payment_status", paymentOrder.Status).
			Int64("amount_fen", resource.Amount.Total).
			Str("transaction_id", resource.TransactionID).
			Int64("payment_order_id", paymentOrder.ID).
			Msg("⚠️ CRITICAL: payment received for closed/failed order — auto refund initiated")

		outRefundNo := fmt.Sprintf("CRF%d", paymentOrder.ID)
		if server.taskDistributor != nil {
			enqErr := server.taskDistributor.DistributeTaskProcessAnomalyRefund(ctx,
				&worker.PayloadProcessAnomalyRefund{
					PaymentOrderID: paymentOrder.ID,
					TransactionID:  resource.TransactionID,
					RefundAmount:   resource.Amount.Total,
					OutRefundNo:    outRefundNo,
				},
				asynq.MaxRetry(5),
				asynq.Queue(worker.QueueCritical),
			)
			if enqErr != nil {
				log.Error().Err(enqErr).
					Int64("payment_order_id", paymentOrder.ID).
					Msg("failed to enqueue anomaly refund task")
				server.sendAlert(websocket.AlertData{
					AlertType: websocket.AlertTypePaymentAmountMismatch,
					Level:     websocket.AlertLevelCritical,
					Title:     "⚠️ 已关闭订单退款任务入队失败 — 需立即人工退款",
					Message: fmt.Sprintf(
						"支付单 %s（ID=%d）处于 %s 状态但微信到账 %d 分，自动退款任务入队失败（%v）。"+
							"交易号: %s。请立即在微信商户平台手动退款 %d 分。",
						resource.OutTradeNo, paymentOrder.ID, paymentOrder.Status,
						resource.Amount.Total, enqErr, resource.TransactionID, resource.Amount.Total,
					),
					RelatedID:   paymentOrder.ID,
					RelatedType: "payment_order",
					Extra: map[string]interface{}{
						"out_trade_no":    resource.OutTradeNo,
						"transaction_id":  resource.TransactionID,
						"amount_fen":      resource.Amount.Total,
						"payment_status":  paymentOrder.Status,
						"out_refund_no":   outRefundNo,
						"action_required": "manual_refund_via_wechat_dashboard",
					},
				})
			} else {
				server.sendAlert(websocket.AlertData{
					AlertType: websocket.AlertTypePaymentAmountMismatch,
					Level:     websocket.AlertLevelWarning,
					Title:     "⚠️ 已关闭订单收到微信付款 — 系统自动退款已触发",
					Message: fmt.Sprintf(
						"支付单 %s（ID=%d）处于 %s 状态但微信到账 %d 分。"+
							"系统已触发自动退款任务（退款单号: %s），请关注退款结果。",
						resource.OutTradeNo, paymentOrder.ID, paymentOrder.Status,
						resource.Amount.Total, outRefundNo,
					),
					RelatedID:   paymentOrder.ID,
					RelatedType: "payment_order",
					Extra: map[string]interface{}{
						"out_trade_no":   resource.OutTradeNo,
						"transaction_id": resource.TransactionID,
						"amount_fen":     resource.Amount.Total,
						"payment_status": paymentOrder.Status,
						"out_refund_no":  outRefundNo,
					},
				})
			}
		} else {
			server.sendAlert(websocket.AlertData{
				AlertType: websocket.AlertTypePaymentAmountMismatch,
				Level:     websocket.AlertLevelCritical,
				Title:     "⚠️ 已关闭订单收到微信付款 — 需人工退款",
				Message: fmt.Sprintf(
					"支付单 %s（ID=%d）已处于 %s 状态，但微信仍到账 %d 分。"+
						"任务分发器未配置，无法自动退款。交易号: %s。请在微信商户平台对该交易发起退款，退款金额 %d 分。",
					resource.OutTradeNo, paymentOrder.ID, paymentOrder.Status,
					resource.Amount.Total, resource.TransactionID, resource.Amount.Total,
				),
				RelatedID:   paymentOrder.ID,
				RelatedType: "payment_order",
				Extra: map[string]interface{}{
					"out_trade_no":    resource.OutTradeNo,
					"transaction_id":  resource.TransactionID,
					"amount_fen":      resource.Amount.Total,
					"payment_status":  paymentOrder.Status,
					"action_required": "manual_refund_via_wechat_dashboard",
				},
			})
		}
		// 通知占位已在 tryClaimNotification 中写入，此处不 release —— 以永久记录该事件，防止重复告警。
		server.markNotificationProcessed(ctx, notification.ID, resource.OutTradeNo, resource.TransactionID)
		ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{Code: "SUCCESS", Message: "OK"})
		return
	}

	// ✅ P0-2: 验证支付金额是否匹配
	if resource.Amount.Total != paymentOrder.Amount {
		log.Error().
			Int64("expected_amount", paymentOrder.Amount).
			Int64("actual_amount", resource.Amount.Total).
			Str("out_trade_no", resource.OutTradeNo).
			Msg("⚠️ payment amount mismatch detected")
		refundReason := "金额异常，系统自动退款"
		refundOrder, outRefundNo, refundRecordErr := server.ensureAmountMismatchRefundRecord(ctx, paymentOrder, resource.Amount.Total, refundReason)
		if refundRecordErr != nil {
			paymentCallbackFailuresTotal.WithLabelValues("payment", "create_mismatch_refund_record").Inc()
			log.Error().Err(refundRecordErr).Int64("payment_order_id", paymentOrder.ID).Msg("create amount mismatch refund record failed")
			server.releaseNotification(ctx, notification.ID, "payment")
			ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "internal error"})
			return
		}
		// 先标记为 paid（记录实际到账）再触发退款，确保退款链路可正常反查交易流水
		if _, updateErr := server.store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
			ID:            paymentOrder.ID,
			TransactionID: pgtype.Text{String: resource.TransactionID, Valid: true},
		}); updateErr != nil {
			log.Error().Err(updateErr).Int64("id", paymentOrder.ID).Msg("update payment order to paid for mismatch refund failed")
			server.releaseNotification(ctx, notification.ID, "payment")
			ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "internal error"})
			return
		}

		// 尝试入队退款任务。告警在结果确认后发送，确保标题与实际结果一致。
		refundEnqueued := false
		refundPayload, canAutoRefund := buildAmountMismatchRefundPayload(paymentOrder, resource.Amount.Total, refundReason)
		if server.taskDistributor != nil && canAutoRefund {
			if enqErr := server.taskDistributor.DistributeTaskProcessRefund(
				ctx,
				refundPayload,
				asynq.MaxRetry(5),
				asynq.Queue(worker.QueueCritical),
			); enqErr != nil {
				paymentCallbackFailuresTotal.WithLabelValues("payment", "enqueue_mismatch_refund").Inc()
				server.markRefundOrderFailed(ctx, refundOrder.ID)
				log.Error().Err(enqErr).Int64("payment_order_id", paymentOrder.ID).Msg("⚠️ CRITICAL: mismatch refund task enqueue failed - manual intervention required")
				server.sendAlert(websocket.AlertData{
					AlertType: websocket.AlertTypeTaskEnqueueFailure,
					Level:     websocket.AlertLevelCritical,
					Title:     "⚠️ 金额异常退款任务入队失败 — 需立即人工退款",
					Message: fmt.Sprintf(
						"支付单 %s（ID=%d）金额异常已记录为 paid，退款任务入队失败。"+
							"交易号 %s，金额 %d 分。请在微信商户平台手动退款。",
						paymentOrder.OutTradeNo, paymentOrder.ID, resource.TransactionID, resource.Amount.Total),
					RelatedID:   paymentOrder.ID,
					RelatedType: "payment_order",
					Extra: map[string]interface{}{
						"out_trade_no":    paymentOrder.OutTradeNo,
						"transaction_id":  resource.TransactionID,
						"amount_fen":      resource.Amount.Total,
						"out_refund_no":   outRefundNo,
						"action_required": "manual_refund_via_wechat_dashboard",
						"error":           enqErr.Error(),
					},
				})
			} else {
				refundEnqueued = true
				server.sendAlert(websocket.AlertData{
					AlertType: websocket.AlertTypePaymentAmountMismatch,
					Level:     websocket.AlertLevelCritical,
					Title:     "⚠️ 支付金额异常 — 自动退款已触发",
					Message: fmt.Sprintf(
						"支付单 %s（ID=%d）实收 %d 分与预期 %d 分不符，交易号 %s。"+
							"系统已自动触发退款，请确认退款结果。",
						resource.OutTradeNo, paymentOrder.ID, resource.Amount.Total, paymentOrder.Amount, resource.TransactionID),
					RelatedID:   paymentOrder.ID,
					RelatedType: "payment_order",
					Extra: map[string]interface{}{
						"out_trade_no":    resource.OutTradeNo,
						"transaction_id":  resource.TransactionID,
						"out_refund_no":   outRefundNo,
						"expected_amount": paymentOrder.Amount,
						"actual_amount":   resource.Amount.Total,
					},
				})
			}
		} else {
			server.markRefundOrderFailed(ctx, refundOrder.ID)
			// taskDistributor 未配置或缺少可自动退款的业务定位信息，无法自动退款
			log.Error().
				Bool("distributor_nil", server.taskDistributor == nil).
				Bool("can_auto_refund", canAutoRefund).
				Int64("payment_order_id", paymentOrder.ID).
				Msg("⚠️ CRITICAL: amount mismatch cannot trigger auto-refund - manual intervention required")
			server.sendAlert(websocket.AlertData{
				AlertType: websocket.AlertTypePaymentAmountMismatch,
				Level:     websocket.AlertLevelCritical,
				Title:     "⚠️ 支付金额异常 — 无法自动退款，需立即人工处理",
				Message: fmt.Sprintf(
					"支付单 %s（ID=%d）实收 %d 分与预期 %d 分不符，交易号 %s。"+
						"系统无法自动触发退款（distributor=%v, can_auto_refund=%v），请在微信商户平台手动退款。",
					resource.OutTradeNo, paymentOrder.ID, resource.Amount.Total, paymentOrder.Amount, resource.TransactionID,
					server.taskDistributor != nil, canAutoRefund),
				RelatedID:   paymentOrder.ID,
				RelatedType: "payment_order",
				Extra: map[string]interface{}{
					"out_trade_no":    resource.OutTradeNo,
					"transaction_id":  resource.TransactionID,
					"out_refund_no":   outRefundNo,
					"expected_amount": paymentOrder.Amount,
					"actual_amount":   resource.Amount.Total,
					"action_required": "manual_refund_via_wechat_dashboard",
				},
			})
		}
		mismatchMsg := "amount mismatch, manual refund required"
		if refundEnqueued {
			mismatchMsg = "amount mismatch, auto-refund triggered"
		}
		server.markNotificationProcessed(ctx, notification.ID, resource.OutTradeNo, resource.TransactionID)
		ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{Code: "SUCCESS", Message: mismatchMsg})
		return
	}

	// 更新支付订单状态为已支付（这是核心操作，必须同步完成）
	updatedPaymentOrder, err := server.store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            paymentOrder.ID,
		TransactionID: pgtype.Text{String: resource.TransactionID, Valid: true},
	})
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("payment", "update_payment_order").Inc()
		log.Error().Err(err).Int64("id", paymentOrder.ID).Msg("update payment order to paid")
		server.releaseNotification(ctx, notification.ID, "payment")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "update order failed",
		})
		return
	}

	// 📢 M14: 异步发送支付成功通知（避免阻塞微信回调响应）
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	_ = server.SendNotification(ctx, SendNotificationParams{
		UserID:      updatedPaymentOrder.UserID,
		Type:        "payment",
		Title:       "支付成功",
		Content:     fmt.Sprintf("您的订单支付已完成，支付金额%s元", fenToYuanString(updatedPaymentOrder.Amount, 2)),
		RelatedType: "payment",
		RelatedID:   updatedPaymentOrder.ID,
		ExtraData: map[string]any{
			"out_trade_no":   updatedPaymentOrder.OutTradeNo,
			"transaction_id": resource.TransactionID,
			"amount":         updatedPaymentOrder.Amount,
			"business_type":  updatedPaymentOrder.BusinessType,
		},
		ExpiresAt:         &expiresAt,
		IgnorePreferences: true,
	})

	// 通知ID已在 tryClaimNotification 中原子写入，无需重复记录

	// 将后续业务逻辑放入队列异步处理
	// 这样可以快速响应微信，避免超时
	if server.taskDistributor != nil {
		err = server.taskDistributor.DistributeTaskProcessPaymentSuccess(
			ctx,
			&worker.PaymentSuccessPayload{
				PaymentOrderID: paymentOrder.ID,
				TransactionID:  resource.TransactionID,
				BusinessType:   paymentOrder.BusinessType,
			},
			asynq.MaxRetry(3),
			asynq.ProcessIn(1*time.Second), // 1秒后处理，确保数据库事务提交
			asynq.Queue(worker.QueueCritical),
		)
		if err != nil {
			// ⚠️ P2-5: 任务入队失败，需要补偿机制处理
			// 支付状态已更新为paid，但后续业务逻辑未执行
			// 建议：定时任务扫描已paid但未处理的订单
			log.Error().Err(err).
				Int64("payment_order_id", paymentOrder.ID).
				Str("out_trade_no", paymentOrder.OutTradeNo).
				Str("alert_type", "TASK_ENQUEUE_FAILURE").
				Msg("⚠️ ALERT: payment success task enqueue failed - manual intervention may be required")
			// 发送告警给平台运营人员
			server.sendAlert(websocket.AlertData{
				AlertType:   websocket.AlertTypeTaskEnqueueFailure,
				Level:       websocket.AlertLevelCritical,
				Title:       "支付成功任务入队失败",
				Message:     fmt.Sprintf("支付单 %s 支付成功，但后续业务任务入队失败，需要人工介入处理", paymentOrder.OutTradeNo),
				RelatedID:   paymentOrder.ID,
				RelatedType: "payment_order",
				Extra: map[string]interface{}{
					"out_trade_no": paymentOrder.OutTradeNo,
					"error":        err.Error(),
				},
			})
		}
	} else {
		log.Warn().Msg("task distributor not configured, skip async processing")
	}

	// 快速返回成功
	ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{
		Code:    "SUCCESS",
		Message: "OK",
	})
	server.markNotificationProcessed(ctx, notification.ID, resource.OutTradeNo, resource.TransactionID)
}

// handleRefundNotify 处理微信退款回调通知
// POST /v1/webhooks/wechat-pay/refund-notify
func (server *Server) handleRefundNotify(ctx *gin.Context) {
	if server.paymentClient == nil {
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "payment client not configured",
		})
		return
	}

	// 读取请求体用于验签
	body, status, err := readWebhookBody(ctx)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("refund", "read_body").Inc()
		log.Error().Err(err).Msg("read refund notification body")
		ctx.JSON(status, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "read body failed",
		})
		return
	}

	// 🔒 验证微信签名
	signature := ctx.GetHeader("Wechatpay-Signature")
	timestamp := ctx.GetHeader("Wechatpay-Timestamp")
	nonce := ctx.GetHeader("Wechatpay-Nonce")

	if err := server.paymentClient.VerifyNotificationSignature(signature, timestamp, nonce, string(body)); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("refund", "signature").Inc()
		log.Error().Err(err).Msg("⚠️ invalid wechat signature for refund notification")
		ctx.JSON(http.StatusUnauthorized, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "signature verification failed",
		})
		return
	}

	// 解析通知
	var notification wechat.PaymentNotification
	if err := json.Unmarshal(body, &notification); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("refund", "parse").Inc()
		log.Error().Err(err).Msg("parse refund notification")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "parse notification failed",
		})
		return
	}

	// 检查通知类型
	validEventTypes := map[string]bool{
		"REFUND.SUCCESS":  true,
		"REFUND.ABNORMAL": true,
		"REFUND.CLOSED":   true,
	}
	if !validEventTypes[notification.EventType] {
		log.Info().Str("event_type", notification.EventType).Msg("ignore non-refund notification")
		ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{
			Code:    "SUCCESS",
			Message: "OK",
		})
		return
	}

	// 🔐 #1: 原子幂等性门
	if !server.tryClaimNotification(ctx, notification, "refund") {
		return
	}

	// 解密通知内容
	resource, err := server.paymentClient.DecryptRefundNotification(&notification)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("refund", "decrypt").Inc()
		log.Error().Err(err).Msg("decrypt refund notification")
		server.releaseNotification(ctx, notification.ID, "refund")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "decrypt failed",
		})
		return
	}

	if err := server.validateDirectRefundOwnership(resource); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("refund", "ownership").Inc()
		log.Error().Err(err).
			Str("out_refund_no", resource.OutRefundNo).
			Str("mchid", resource.MchID).
			Msg("refund notification ownership validation failed")
		server.sendAlert(websocket.AlertData{
			AlertType:   websocket.AlertTypeSystemError,
			Level:       websocket.AlertLevelCritical,
			Title:       "直连退款回调归属校验失败",
			Message:     fmt.Sprintf("直连退款回调 out_refund_no=%s 的归属校验失败，mchid=%s。系统已返回 FAIL 等待微信重试，请排查是否存在错商户退款回调。", resource.OutRefundNo, resource.MchID),
			RelatedID:   0,
			RelatedType: "refund_order",
			Extra: map[string]interface{}{
				"out_refund_no":  resource.OutRefundNo,
				"out_trade_no":   resource.OutTradeNo,
				"transaction_id": resource.TransactionID,
				"mchid":          resource.MchID,
			},
		})
		server.releaseNotification(ctx, notification.ID, "refund")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "ownership validation failed",
		})
		return
	}

	log.Info().
		Str("notification_id", notification.ID).
		Str("out_refund_no", resource.OutRefundNo).
		Str("refund_status", resource.RefundStatus).
		Int64("refund_amount", resource.Amount.Refund).
		Msg("received refund notification")

	// 将退款结果处理放入队列
	if server.taskDistributor != nil {
		err = server.taskDistributor.DistributeTaskProcessRefundResult(
			ctx,
			&worker.RefundResultPayload{
				OutRefundNo:  resource.OutRefundNo,
				RefundStatus: resource.RefundStatus,
				RefundID:     resource.RefundID,
			},
			asynq.MaxRetry(3),
			asynq.Queue(worker.QueueCritical),
		)
		if err != nil {
			paymentCallbackFailuresTotal.WithLabelValues("refund", "enqueue").Inc()
			log.Error().Err(err).
				Str("out_refund_no", resource.OutRefundNo).
				Msg("failed to enqueue refund result task")
			server.releaseNotification(ctx, notification.ID, "refund")
			ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
				Code:    "FAIL",
				Message: "enqueue failed, please retry",
			})
			return
		}
	}

	// 通知ID已在 tryClaimNotification 中原子写入，无需重复记录

	// 快速返回成功
	ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{
		Code:    "SUCCESS",
		Message: "OK",
	})
	server.markNotificationProcessed(ctx, notification.ID, resource.OutTradeNo, resource.TransactionID)
}

// handleEcommerceRefundNotify 处理平台收付通退款回调通知
// POST /v1/webhooks/wechat-ecommerce/refund-notify
func (server *Server) handleEcommerceRefundNotify(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "ecommerce client not configured",
		})
		return
	}

	// 读取请求体用于验签
	body, status, err := readWebhookBody(ctx)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("ecommerce_refund", "read_body").Inc()
		log.Error().Err(err).Msg("read ecommerce refund notification body")
		ctx.JSON(status, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "read body failed",
		})
		return
	}

	// 🔒 验证微信签名
	signature := ctx.GetHeader("Wechatpay-Signature")
	timestamp := ctx.GetHeader("Wechatpay-Timestamp")
	nonce := ctx.GetHeader("Wechatpay-Nonce")

	if err := server.ecommerceClient.VerifyNotificationSignature(signature, timestamp, nonce, string(body)); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("ecommerce_refund", "signature").Inc()
		log.Error().Err(err).Msg("⚠️ invalid wechat signature for ecommerce refund notification")
		ctx.JSON(http.StatusUnauthorized, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "signature verification failed",
		})
		return
	}

	// 解析通知
	var notification wechat.PaymentNotification
	if err := json.Unmarshal(body, &notification); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("ecommerce_refund", "parse").Inc()
		log.Error().Err(err).Msg("parse ecommerce refund notification")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "parse notification failed",
		})
		return
	}

	// 检查通知类型
	validEventTypes := map[string]bool{
		"REFUND.SUCCESS":  true,
		"REFUND.ABNORMAL": true,
		"REFUND.CLOSED":   true,
	}
	if !validEventTypes[notification.EventType] {
		log.Info().Str("event_type", notification.EventType).Msg("ignore non-refund notification")
		ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{
			Code:    "SUCCESS",
			Message: "OK",
		})
		return
	}

	// 🔐 #1: 原子幂等性门
	if !server.tryClaimNotification(ctx, notification, "ecommerce_refund") {
		return
	}

	// 解密通知内容（使用平台收付通专用解密方法）
	resource, err := server.ecommerceClient.DecryptEcommerceRefundNotification(&notification)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("ecommerce_refund", "decrypt").Inc()
		log.Error().Err(err).Msg("decrypt ecommerce refund notification")
		server.releaseNotification(ctx, notification.ID, "ecommerce_refund")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "decrypt failed",
		})
		return
	}

	log.Info().
		Str("sp_mchid", resource.SpMchID).
		Str("sub_mchid", resource.SubMchID).
		Str("out_refund_no", resource.OutRefundNo).
		Str("refund_status", resource.RefundStatus).
		Int64("refund_amount", resource.Amount.Refund).
		Msg("received ecommerce refund notification")

	if err := server.validateEcommerceRefundOwnership(resource); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("ecommerce_refund", "ownership").Inc()
		log.Error().Err(err).
			Str("sp_mchid", resource.SpMchID).
			Str("sub_mchid", resource.SubMchID).
			Msg("ecommerce refund notification ownership validation failed")
		server.sendAlert(websocket.AlertData{
			AlertType:   websocket.AlertTypeSystemError,
			Level:       websocket.AlertLevelCritical,
			Title:       "收付通退款回调归属校验失败",
			Message:     fmt.Sprintf("收付通退款回调 out_refund_no=%s 的归属校验失败，sp_mchid=%s, sub_mchid=%s。系统已返回 FAIL 等待微信重试，请排查是否存在错服务商或错子商户回调。", resource.OutRefundNo, resource.SpMchID, resource.SubMchID),
			RelatedID:   0,
			RelatedType: "refund_order",
			Extra: map[string]interface{}{
				"out_refund_no": resource.OutRefundNo,
				"out_trade_no":  resource.OutTradeNo,
				"sp_mchid":      resource.SpMchID,
				"sub_mchid":     resource.SubMchID,
			},
		})
		server.releaseNotification(ctx, notification.ID, "ecommerce_refund")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "ownership validation failed",
		})
		return
	}

	// 将退款结果处理放入队列（复用现有退款结果处理逻辑）
	if server.taskDistributor != nil {
		err = server.taskDistributor.DistributeTaskProcessRefundResult(
			ctx,
			&worker.RefundResultPayload{
				OutRefundNo:  resource.OutRefundNo,
				RefundStatus: resource.RefundStatus,
				RefundID:     resource.RefundID,
			},
			asynq.MaxRetry(3),
			asynq.Queue(worker.QueueCritical),
		)
		if err != nil {
			paymentCallbackFailuresTotal.WithLabelValues("ecommerce_refund", "enqueue").Inc()
			log.Error().Err(err).
				Str("out_refund_no", resource.OutRefundNo).
				Str("sp_mchid", resource.SpMchID).
				Str("sub_mchid", resource.SubMchID).
				Msg("⚠️ CRITICAL: failed to enqueue ecommerce refund result task - manual intervention may be required")
			server.releaseNotification(ctx, notification.ID, "ecommerce_refund")
			ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
				Code:    "FAIL",
				Message: "enqueue failed, please retry",
			})
			return
		}
	}

	// 通知ID已在 tryClaimNotification 中原子写入，无需重复记录

	// 快速返回成功
	ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{
		Code:    "SUCCESS",
		Message: "OK",
	})
	server.markNotificationProcessed(ctx, notification.ID, resource.OutTradeNo, resource.TransactionID)
}

// handleProfitSharingNotify 处理平台收付通分账结果回调通知
// POST /v1/webhooks/wechat-ecommerce/profit-sharing-notify
func (server *Server) handleProfitSharingNotify(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "ecommerce client not configured",
		})
		return
	}

	// 读取请求体用于验签
	body, status, err := readWebhookBody(ctx)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("profit_sharing", "read_body").Inc()
		log.Error().Err(err).Msg("read profit sharing notification body")
		ctx.JSON(status, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "read body failed",
		})
		return
	}

	// 🔒 验证微信签名
	signature := ctx.GetHeader("Wechatpay-Signature")
	timestamp := ctx.GetHeader("Wechatpay-Timestamp")
	nonce := ctx.GetHeader("Wechatpay-Nonce")

	if err := server.ecommerceClient.VerifyNotificationSignature(signature, timestamp, nonce, string(body)); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("profit_sharing", "signature").Inc()
		log.Error().Err(err).Msg("⚠️ invalid wechat signature for profit sharing notification")
		ctx.JSON(http.StatusUnauthorized, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "signature verification failed",
		})
		return
	}

	// 解析通知
	var notification wechat.PaymentNotification
	if err := json.Unmarshal(body, &notification); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("profit_sharing", "parse").Inc()
		log.Error().Err(err).Msg("parse profit sharing notification")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "parse notification failed",
		})
		return
	}

	// 检查通知类型（分账通知事件类型）
	validEventTypes := map[string]bool{
		"TRANSACTION.SUCCESS": true, // 分账成功
	}
	if !validEventTypes[notification.EventType] {
		log.Info().Str("event_type", notification.EventType).Msg("ignore non-profit-sharing notification")
		ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{
			Code:    "SUCCESS",
			Message: "OK",
		})
		return
	}

	// 🔐 #1: 原子幂等性门
	if !server.tryClaimNotification(ctx, notification, "profit_sharing") {
		return
	}

	// 解密通知内容
	resource, err := server.ecommerceClient.DecryptProfitSharingNotification(&notification)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("profit_sharing", "decrypt").Inc()
		log.Error().Err(err).Msg("decrypt profit sharing notification")
		server.releaseNotification(ctx, notification.ID, "profit_sharing")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "decrypt failed",
		})
		return
	}

	log.Info().
		Str("mch_id", resource.MchID).
		Str("sub_mch_id", resource.SubMchID).
		Str("out_order_no", resource.OutOrderNo).
		Str("order_id", resource.OrderID).
		Str("receiver_result", resource.Receiver.Result).
		Int64("receiver_amount", resource.Receiver.Amount).
		Msg("received profit sharing notification")

	if err := server.validateProfitSharingOwnership(resource); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("profit_sharing", "ownership").Inc()
		log.Error().Err(err).
			Str("mch_id", resource.MchID).
			Str("sub_mch_id", resource.SubMchID).
			Msg("profit sharing notification ownership validation failed")
		server.sendAlert(websocket.AlertData{
			AlertType:   websocket.AlertTypeSystemError,
			Level:       websocket.AlertLevelCritical,
			Title:       "分账回调归属校验失败",
			Message:     fmt.Sprintf("分账回调 out_order_no=%s 的归属校验失败，mch_id=%s, sub_mch_id=%s。系统已返回 FAIL 等待微信重试，请排查是否存在错服务商分账回调。", resource.OutOrderNo, resource.MchID, resource.SubMchID),
			RelatedID:   0,
			RelatedType: "profit_sharing_order",
			Extra: map[string]interface{}{
				"out_order_no": resource.OutOrderNo,
				"order_id":     resource.OrderID,
				"mch_id":       resource.MchID,
				"sub_mch_id":   resource.SubMchID,
			},
		})
		server.releaseNotification(ctx, notification.ID, "profit_sharing")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "ownership validation failed",
		})
		return
	}

	// 查询分账订单
	profitSharingOrder, err := server.store.GetProfitSharingOrderByOutOrderNo(ctx, resource.OutOrderNo)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("profit_sharing", "query_profit_sharing_order").Inc()
		if isNotFoundError(err) {
			log.Error().Str("out_order_no", resource.OutOrderNo).Msg("profit sharing order not found")
			// 订单不存在，返回成功避免微信重试
			server.markNotificationProcessed(ctx, notification.ID, "", resource.TransactionID)
			ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{
				Code:    "SUCCESS",
				Message: "order not found, ignored",
			})
			return
		}
		log.Error().Err(err).Str("out_order_no", resource.OutOrderNo).Msg("get profit sharing order")
		server.releaseNotification(ctx, notification.ID, "profit_sharing")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "internal error",
		})
		return
	}

	// 检查是否已处理（幂等）
	if profitSharingOrder.Status == "finished" || profitSharingOrder.Status == "failed" {
		log.Info().Int64("id", profitSharingOrder.ID).Str("status", profitSharingOrder.Status).Msg("profit sharing order already processed")
		server.markNotificationProcessed(ctx, notification.ID, "", resource.TransactionID)
		ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{
			Code:    "SUCCESS",
			Message: "OK",
		})
		return
	}

	if err := server.validateProfitSharingSubMerchantOwnership(ctx, resource, profitSharingOrder); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("profit_sharing", "sub_merchant_ownership").Inc()
		log.Error().Err(err).
			Int64("profit_sharing_order_id", profitSharingOrder.ID).
			Int64("merchant_id", profitSharingOrder.MerchantID).
			Str("out_order_no", resource.OutOrderNo).
			Str("sub_mch_id", resource.SubMchID).
			Msg("profit sharing notification sub merchant ownership validation failed")
		server.sendAlert(websocket.AlertData{
			AlertType:   websocket.AlertTypeSystemError,
			Level:       websocket.AlertLevelCritical,
			Title:       "分账回调子商户归属校验失败",
			Message:     fmt.Sprintf("分账回调 out_order_no=%s 的子商户归属校验失败，sub_mch_id=%s。系统已返回 FAIL 等待微信重试，请排查本地商户支付配置与分账单归属是否一致。", resource.OutOrderNo, resource.SubMchID),
			RelatedID:   profitSharingOrder.ID,
			RelatedType: "profit_sharing_order",
			Extra: map[string]interface{}{
				"profit_sharing_order_id": profitSharingOrder.ID,
				"merchant_id":             profitSharingOrder.MerchantID,
				"out_order_no":            resource.OutOrderNo,
				"order_id":                resource.OrderID,
				"sub_mch_id":              resource.SubMchID,
			},
		})
		server.releaseNotification(ctx, notification.ID, "profit_sharing")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "ownership validation failed",
		})
		return
	}

	queryResp, queryErr := server.ecommerceClient.QueryProfitSharing(ctx, resource.SubMchID, resource.TransactionID, resource.OutOrderNo)
	if queryErr != nil {
		log.Warn().Err(queryErr).
			Str("out_order_no", resource.OutOrderNo).
			Msg("query profit sharing detail failed, fallback to single receiver event")
	}

	finalResult := strings.ToUpper(resource.Receiver.Result)
	finalFailReason := resource.Receiver.FailReason

	if queryErr == nil {
		allSuccess := strings.ToUpper(queryResp.Status) == "FINISHED"
		hasFailed := false
		failedReasons := make([]string, 0)

		for _, receiver := range queryResp.Receivers {
			result := strings.ToUpper(receiver.Result)
			switch result {
			case "SUCCESS":
				// pass
			case "FAILED", "CLOSED":
				hasFailed = true
				if receiver.FailReason != "" {
					failedReasons = append(failedReasons, receiver.FailReason)
				}
				allSuccess = false
			default:
				allSuccess = false
			}
		}

		switch {
		case hasFailed:
			finalResult = "FAILED"
			if len(failedReasons) > 0 {
				finalFailReason = strings.Join(failedReasons, ";")
			}
		case allSuccess:
			finalResult = "SUCCESS"
		default:
			finalResult = "PROCESSING"
		}
	}

	switch finalResult {
	case "SUCCESS":
		_, err = server.store.UpdateProfitSharingOrderToFinished(ctx, profitSharingOrder.ID)
		if err != nil {
			paymentCallbackFailuresTotal.WithLabelValues("profit_sharing", "update_profit_sharing_order").Inc()
			log.Error().Err(err).Int64("id", profitSharingOrder.ID).Msg("update profit sharing order to finished")
			server.releaseNotification(ctx, notification.ID, "profit_sharing")
			ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
				Code:    "FAIL",
				Message: "update order failed",
			})
			return
		}
		log.Info().
			Int64("profit_sharing_order_id", profitSharingOrder.ID).
			Str("out_order_no", resource.OutOrderNo).
			Msg("profit sharing completed successfully")

	case "FAILED", "CLOSED":
		_, err = server.store.UpdateProfitSharingOrderToFailed(ctx, profitSharingOrder.ID)
		if err != nil {
			paymentCallbackFailuresTotal.WithLabelValues("profit_sharing", "update_profit_sharing_order").Inc()
			log.Error().Err(err).Int64("id", profitSharingOrder.ID).Msg("update profit sharing order to failed")
		}
		log.Error().
			Int64("profit_sharing_order_id", profitSharingOrder.ID).
			Str("out_order_no", resource.OutOrderNo).
			Str("result", finalResult).
			Str("fail_reason", finalFailReason).
			Msg("⚠️ profit sharing failed - manual review required")

	case "PROCESSING":
		log.Info().
			Int64("profit_sharing_order_id", profitSharingOrder.ID).
			Str("out_order_no", resource.OutOrderNo).
			Msg("profit sharing still processing")

	default:
		log.Warn().
			Str("result", finalResult).
			Str("out_order_no", resource.OutOrderNo).
			Msg("unknown profit sharing result")
	}

	// 通知ID已在 tryClaimNotification 中原子写入，无需重复记录

	// 📤 CB-3: 异步处理分账结果通知（错误记录日志+告警，notification claim 已写入不需 release）
	if server.taskDistributor != nil {
		if enqErr := server.taskDistributor.DistributeTaskProcessProfitSharingResult(
			ctx,
			&worker.ProfitSharingResultPayload{
				ProfitSharingOrderID: profitSharingOrder.ID,
				OutOrderNo:           resource.OutOrderNo,
				Result:               finalResult,
				FailReason:           finalFailReason,
				MerchantID:           profitSharingOrder.MerchantID,
			},
			asynq.MaxRetry(3),
			asynq.Queue(worker.QueueDefault),
		); enqErr != nil {
			paymentCallbackFailuresTotal.WithLabelValues("profit_sharing", "enqueue_result_task").Inc()
			log.Error().Err(enqErr).
				Int64("profit_sharing_order_id", profitSharingOrder.ID).
				Str("out_order_no", resource.OutOrderNo).
				Msg("⚠️ failed to enqueue profit sharing result task")
			server.sendAlert(websocket.AlertData{
				AlertType:   websocket.AlertTypeTaskEnqueueFailure,
				Level:       websocket.AlertLevelCritical,
				Title:       "分账结果任务入队失败",
				Message:     fmt.Sprintf("分账单 %s 结果 %s 处理完成，但后续通知任务入队失败，需人工确认处理结果", resource.OutOrderNo, finalResult),
				RelatedID:   profitSharingOrder.ID,
				RelatedType: "profit_sharing_order",
				Extra:       map[string]interface{}{"out_order_no": resource.OutOrderNo, "result": finalResult, "error": enqErr.Error()},
			})
		}
	}

	// 快速返回成功
	ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{
		Code:    "SUCCESS",
		Message: "OK",
	})
	server.markNotificationProcessed(ctx, notification.ID, "", resource.TransactionID)
}

// handleCombinePaymentNotify 处理平台收付通合单支付回调通知
// POST /v1/webhooks/wechat-ecommerce/notify
func (server *Server) handleCombinePaymentNotify(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "ecommerce client not configured",
		})
		return
	}

	// 读取请求体用于验签
	body, status, err := readWebhookBody(ctx)
	if err != nil {
		log.Error().Err(err).Msg("read combine payment notification body")
		ctx.JSON(status, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "read body failed",
		})
		return
	}

	// 🔒 验证微信签名
	signature := ctx.GetHeader("Wechatpay-Signature")
	timestamp := ctx.GetHeader("Wechatpay-Timestamp")
	nonce := ctx.GetHeader("Wechatpay-Nonce")

	// 验证签名（EcommerceClient继承自PaymentClient，拥有此方法）
	if err := server.ecommerceClient.VerifyNotificationSignature(signature, timestamp, nonce, string(body)); err != nil {
		log.Error().Err(err).Msg("⚠️ invalid wechat signature for combine payment notification")
		ctx.JSON(http.StatusUnauthorized, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "signature verification failed",
		})
		return
	}

	// 解析通知
	var notification wechat.PaymentNotification
	if err := json.Unmarshal(body, &notification); err != nil {
		log.Error().Err(err).Msg("parse combine payment notification")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "parse notification failed",
		})
		return
	}

	// 检查通知类型
	if notification.EventType != "TRANSACTION.SUCCESS" {
		log.Info().Str("event_type", notification.EventType).Msg("ignore non-success notification")
		ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{
			Code:    "SUCCESS",
			Message: "OK",
		})
		return
	}

	// 🔐 #1: 原子幂等性门
	if !server.tryClaimNotification(ctx, notification, "combine_payment") {
		return
	}

	// 解密通知内容
	resource, err := server.ecommerceClient.DecryptCombinePaymentNotification(&notification)
	if err != nil {
		log.Error().Err(err).Msg("decrypt combine payment notification")
		server.releaseNotification(ctx, notification.ID, "combine_payment")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "decrypt failed",
		})
		return
	}

	log.Info().
		Str("combine_out_trade_no", resource.CombineOutTradeNo).
		Int("sub_orders_count", len(resource.SubOrders)).
		Msg("received combine payment notification")

	if err := server.validateCombineNotifyOwnership(resource); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("combine_payment", "ownership").Inc()
		log.Error().Err(err).
			Str("combine_out_trade_no", resource.CombineOutTradeNo).
			Msg("combine payment ownership validation failed")
		server.sendAlert(websocket.AlertData{
			AlertType:   websocket.AlertTypeSystemError,
			Level:       websocket.AlertLevelCritical,
			Title:       "合单回调归属校验失败",
			Message:     fmt.Sprintf("合单回调 combine_out_trade_no=%s 的归属校验失败，combine_mchid=%s, combine_appid=%s。系统已返回 FAIL 等待微信重试，请排查是否存在错服务商或错应用合单回调。", resource.CombineOutTradeNo, resource.CombineMchID, resource.CombineAppID),
			RelatedID:   0,
			RelatedType: "combined_payment_order",
			Extra: map[string]interface{}{
				"combine_out_trade_no": resource.CombineOutTradeNo,
				"combine_mchid":        resource.CombineMchID,
				"combine_appid":        resource.CombineAppID,
			},
		})
		server.releaseNotification(ctx, notification.ID, "combine_payment")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "ownership validation failed",
		})
		return
	}

	// ✅ P0-1: 修复合单支付错误处理
	var failedOrders []string
	exceptionalCount := 0
	successCount := 0

	// 处理每个子订单
	for _, subOrder := range resource.SubOrders {
		if subOrder.TradeState != "SUCCESS" {
			log.Warn().
				Str("out_trade_no", subOrder.OutTradeNo).
				Str("trade_state", subOrder.TradeState).
				Msg("sub order not success")
			continue
		}

		// 查询支付订单
		paymentOrder, err := server.store.GetPaymentOrderByOutTradeNo(ctx, subOrder.OutTradeNo)
		if err != nil {
			if isNotFoundError(err) {
				log.Error().Str("out_trade_no", subOrder.OutTradeNo).Msg("payment order not found")
				server.sendAlert(websocket.AlertData{
					AlertType:   websocket.AlertTypeSystemError,
					Level:       websocket.AlertLevelCritical,
					Title:       "合单回调未找到本地子支付单",
					Message:     fmt.Sprintf("合单回调 combine_out_trade_no=%s 的子单 %s 未找到本地 payment_order。系统将返回 FAIL 等待微信重试，请排查子支付单创建与回调时序。", resource.CombineOutTradeNo, subOrder.OutTradeNo),
					RelatedID:   0,
					RelatedType: "payment_order",
					Extra: map[string]interface{}{
						"combine_out_trade_no": resource.CombineOutTradeNo,
						"out_trade_no":         subOrder.OutTradeNo,
						"transaction_id":       subOrder.TransactionID,
						"business_action":      "wechat_retry_expected",
					},
				})
				failedOrders = append(failedOrders, subOrder.OutTradeNo)
				continue
			}
			log.Error().Err(err).Str("out_trade_no", subOrder.OutTradeNo).Msg("get payment order failed")
			failedOrders = append(failedOrders, subOrder.OutTradeNo)
			continue
		}

		// 检查是否已处理（幂等）
		if paymentOrder.Status == PaymentStatusPaid {
			successCount++
			continue
		}

		// 🚨 检测已关闭/失败子单收到支付回调：与单笔支付回调保持一致的异常到账保护
		if paymentOrder.Status == PaymentStatusClosed || paymentOrder.Status == PaymentStatusFailed {
			log.Error().
				Str("out_trade_no", subOrder.OutTradeNo).
				Str("payment_status", paymentOrder.Status).
				Str("transaction_id", subOrder.TransactionID).
				Int64("payment_order_id", paymentOrder.ID).
				Msg("⚠️ CRITICAL: combine sub-order payment received for closed/failed order — auto refund initiated")

			outRefundNo := fmt.Sprintf("CRF%d", paymentOrder.ID)
			if server.taskDistributor != nil {
				if enqErr := server.taskDistributor.DistributeTaskProcessAnomalyRefund(ctx,
					&worker.PayloadProcessAnomalyRefund{
						PaymentOrderID: paymentOrder.ID,
						TransactionID:  subOrder.TransactionID,
						RefundAmount:   subOrder.Amount.TotalAmount,
						OutRefundNo:    outRefundNo,
					},
					asynq.MaxRetry(5),
					asynq.Queue(worker.QueueCritical),
				); enqErr != nil {
					log.Error().Err(enqErr).
						Int64("payment_order_id", paymentOrder.ID).
						Msg("failed to enqueue anomaly refund task for combine sub-order")
				}
			}
			exceptionalCount++ // 异常到账子单不应推动主合单进入 paid
			continue
		}

		// ✅ P1-1: 验证子订单支付金额是否匹配（与单笔支付回调保持一致）
		if subOrder.Amount.TotalAmount != paymentOrder.Amount {
			log.Error().
				Int64("expected_amount", paymentOrder.Amount).
				Int64("actual_amount", subOrder.Amount.TotalAmount).
				Str("out_trade_no", subOrder.OutTradeNo).
				Int64("payment_order_id", paymentOrder.ID).
				Msg("⚠️ combine sub-order amount mismatch detected")

			refundReason := "合单子订单金额异常，系统自动退款"
			refundOrder, outRefundNo, refundRecordErr := server.ensureAmountMismatchRefundRecord(ctx, paymentOrder, subOrder.Amount.TotalAmount, refundReason)
			if refundRecordErr != nil {
				paymentCallbackFailuresTotal.WithLabelValues("combine_payment", "create_mismatch_refund_record").Inc()
				log.Error().Err(refundRecordErr).Int64("payment_order_id", paymentOrder.ID).Msg("create combine sub-order mismatch refund record failed")
				failedOrders = append(failedOrders, subOrder.OutTradeNo)
				continue
			}

			// 先落退款记录，再标记为 paid（记录实际到账）再触发退款
			if _, updateErr := server.store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
				ID:            paymentOrder.ID,
				TransactionID: pgtype.Text{String: subOrder.TransactionID, Valid: true},
			}); updateErr != nil {
				log.Error().Err(updateErr).Int64("id", paymentOrder.ID).Msg("update combine sub-order to paid for mismatch refund failed")
				failedOrders = append(failedOrders, subOrder.OutTradeNo)
				continue
			}

			refundPayload, canAutoRefund := buildAmountMismatchRefundPayload(paymentOrder, subOrder.Amount.TotalAmount, refundReason)
			if server.taskDistributor != nil && canAutoRefund {
				if enqErr := server.taskDistributor.DistributeTaskProcessRefund(
					ctx,
					refundPayload,
					asynq.MaxRetry(5),
					asynq.Queue(worker.QueueCritical),
				); enqErr != nil {
					paymentCallbackFailuresTotal.WithLabelValues("combine_payment", "enqueue_mismatch_refund_task").Inc()
					server.markRefundOrderFailed(ctx, refundOrder.ID)
					log.Error().Err(enqErr).Int64("payment_order_id", paymentOrder.ID).Str("out_trade_no", subOrder.OutTradeNo).Msg("combine sub-order mismatch refund task enqueue failed")
					server.sendAlert(websocket.AlertData{
						AlertType:   websocket.AlertTypeTaskEnqueueFailure,
						Level:       websocket.AlertLevelCritical,
						Title:       "合单金额异常退款任务入队失败",
						Message:     fmt.Sprintf("合单子订单 %s 金额异常后退款任务入队失败，支付单 %d 需要人工处理", subOrder.OutTradeNo, paymentOrder.ID),
						RelatedID:   paymentOrder.ID,
						RelatedType: "payment_order",
						Extra: map[string]interface{}{
							"out_trade_no":    subOrder.OutTradeNo,
							"transaction_id":  subOrder.TransactionID,
							"out_refund_no":   outRefundNo,
							"expected_amount": paymentOrder.Amount,
							"actual_amount":   subOrder.Amount.TotalAmount,
							"error":           enqErr.Error(),
						},
					})
				}
			} else {
				server.markRefundOrderFailed(ctx, refundOrder.ID)
			}

			server.sendAlert(websocket.AlertData{
				AlertType:   websocket.AlertTypePaymentAmountMismatch,
				Level:       websocket.AlertLevelCritical,
				Title:       "⚠️ 合单子订单金额异常",
				Message:     fmt.Sprintf("合单子订单 %s（ID=%d）实收 %d 分与预期 %d 分不符，交易号 %s", subOrder.OutTradeNo, paymentOrder.ID, subOrder.Amount.TotalAmount, paymentOrder.Amount, subOrder.TransactionID),
				RelatedID:   paymentOrder.ID,
				RelatedType: "payment_order",
				Extra: map[string]interface{}{
					"out_trade_no":    subOrder.OutTradeNo,
					"transaction_id":  subOrder.TransactionID,
					"out_refund_no":   outRefundNo,
					"expected_amount": paymentOrder.Amount,
					"actual_amount":   subOrder.Amount.TotalAmount,
				},
			})
			exceptionalCount++
			continue
		}

		// 更新支付订单状态
		_, err = server.store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
			ID:            paymentOrder.ID,
			TransactionID: pgtype.Text{String: subOrder.TransactionID, Valid: true},
		})
		if err != nil {
			log.Error().Err(err).Int64("id", paymentOrder.ID).Msg("update payment order to paid failed")
			failedOrders = append(failedOrders, subOrder.OutTradeNo)
			continue
		}

		successCount++

		// 分发异步任务处理后续业务
		if server.taskDistributor != nil {
			enqErr := server.taskDistributor.DistributeTaskProcessPaymentSuccess(
				ctx,
				&worker.PaymentSuccessPayload{
					PaymentOrderID: paymentOrder.ID,
					TransactionID:  subOrder.TransactionID,
					BusinessType:   paymentOrder.BusinessType,
				},
				asynq.MaxRetry(3),
				asynq.ProcessIn(1*time.Second),
				asynq.Queue(worker.QueueCritical),
			)
			if enqErr != nil {
				paymentCallbackFailuresTotal.WithLabelValues("combine_payment", "enqueue_payment_success_task").Inc()
				log.Error().Err(enqErr).
					Int64("payment_order_id", paymentOrder.ID).
					Str("out_trade_no", subOrder.OutTradeNo).
					Msg("combine payment success task enqueue failed")
				server.sendAlert(websocket.AlertData{
					AlertType:   websocket.AlertTypeTaskEnqueueFailure,
					Level:       websocket.AlertLevelCritical,
					Title:       "合单支付成功任务入队失败",
					Message:     fmt.Sprintf("合单子订单 %s 已支付，但后续业务任务入队失败，需要人工确认订单推进状态", subOrder.OutTradeNo),
					RelatedID:   paymentOrder.ID,
					RelatedType: "payment_order",
					Extra: map[string]interface{}{
						"out_trade_no":   subOrder.OutTradeNo,
						"transaction_id": subOrder.TransactionID,
						"business_type":  paymentOrder.BusinessType,
						"error":          enqErr.Error(),
					},
				})
			}
		}
	}

	// 如果有失败的订单，返回FAIL触发微信重试
	if len(failedOrders) > 0 {
		log.Error().
			Strs("failed_orders", failedOrders).
			Int("success_count", successCount).
			Int("failed_count", len(failedOrders)).
			Msg("some sub orders failed to process")
		server.releaseNotification(ctx, notification.ID, "combine_payment")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: fmt.Sprintf("%d orders failed", len(failedOrders)),
		})
		return
	}

	log.Info().
		Int("success_count", successCount).
		Int("exceptional_count", exceptionalCount).
		Msg("all sub orders processed successfully")

	if exceptionalCount > 0 {
		log.Warn().
			Int("success_count", successCount).
			Int("exceptional_count", exceptionalCount).
			Str("combine_out_trade_no", resource.CombineOutTradeNo).
			Msg("combined payment contains exceptional sub orders, skip marking main order paid")
		ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{
			Code:    "SUCCESS",
			Message: "OK",
		})
		server.markNotificationProcessed(ctx, notification.ID, resource.CombineOutTradeNo, "")
		return
	}

	// ✅ 更新合单主单状态为已支付，保证对账、恢复扫描、报表的 paid_at 正常落库。
	// 合单层级无单一 transaction_id（每子单各持一个），主单 transaction_id 置 NULL 合规。
	combinedOrder, err := server.store.GetCombinedPaymentOrderByOutTradeNo(ctx, resource.CombineOutTradeNo)
	if err != nil {
		log.Error().Err(err).
			Str("combine_out_trade_no", resource.CombineOutTradeNo).
			Msg("get combined payment order failed after sub orders paid")
		if isNotFoundError(err) {
			server.sendAlert(websocket.AlertData{
				AlertType:   websocket.AlertTypeSystemError,
				Level:       websocket.AlertLevelCritical,
				Title:       "合单回调未找到本地主合单",
				Message:     fmt.Sprintf("合单回调 combine_out_trade_no=%s 的所有子单已处理，但未找到 combined_payment_order。系统已返回 FAIL 等待微信重试，请尽快排查主合单创建链路。", resource.CombineOutTradeNo),
				RelatedID:   0,
				RelatedType: "combined_payment_order",
				Extra: map[string]interface{}{
					"combine_out_trade_no": resource.CombineOutTradeNo,
					"business_action":      "wechat_retry_expected",
				},
			})
		}
		server.releaseNotification(ctx, notification.ID, "combine_payment")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "get combined payment order failed",
		})
		return
	}

	if _, err := server.store.UpdateCombinedPaymentOrderToPaid(ctx, db.UpdateCombinedPaymentOrderToPaidParams{
		ID:            combinedOrder.ID,
		TransactionID: pgtype.Text{Valid: false},
	}); err != nil {
		log.Error().Err(err).
			Int64("combined_payment_id", combinedOrder.ID).
			Str("combine_out_trade_no", resource.CombineOutTradeNo).
			Msg("update combined payment order to paid failed")
		server.releaseNotification(ctx, notification.ID, "combine_payment")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "update combined payment order failed",
		})
		return
	}

	log.Info().
		Int64("combined_payment_id", combinedOrder.ID).
		Str("combine_out_trade_no", resource.CombineOutTradeNo).
		Msg("combined payment order marked as paid")

	// 通知ID已在 tryClaimNotification 中原子写入，无需重复记录

	ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{
		Code:    "SUCCESS",
		Message: "OK",
	})
	server.markNotificationProcessed(ctx, notification.ID, resource.CombineOutTradeNo, "")
}

// ==================== 二级商户进件状态回调 ====================

// ApplymentStateNotification 进件状态变更通知
type ApplymentStateNotification struct {
	ID           string `json:"id"`            // 通知ID
	CreateTime   string `json:"create_time"`   // 通知创建时间
	EventType    string `json:"event_type"`    // 通知类型: APPLYMENT_STATE.CHANGE
	ResourceType string `json:"resource_type"` // 资源类型: encrypt-resource
	Resource     struct {
		Algorithm      string `json:"algorithm"`       // 加密算法
		Ciphertext     string `json:"ciphertext"`      // 加密数据
		Nonce          string `json:"nonce"`           // 随机串
		AssociatedData string `json:"associated_data"` // 附加数据
		OriginalType   string `json:"original_type"`   // 原始类型
	} `json:"resource"`
	Summary string `json:"summary"` // 回调摘要
}

// ApplymentStateChangeResource 进件状态变更资源数据
type ApplymentStateChangeResource struct {
	ApplymentID    int64  `json:"applyment_id"`    // 微信支付申请单号
	OutRequestNo   string `json:"out_request_no"`  // 业务申请编号
	ApplymentState string `json:"applyment_state"` // 申请状态
	SubMchID       string `json:"sub_mchid"`       // 特约商户号（开户成功后返回）
}

// handleApplymentStateNotify 处理二级商户进件状态变更通知
// POST /v1/webhooks/wechat-ecommerce/applyment-notify
func (server *Server) handleApplymentStateNotify(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "ecommerce client not configured",
		})
		return
	}

	// 读取请求体
	body, status, err := readWebhookBody(ctx)
	if err != nil {
		log.Error().Err(err).Msg("read applyment notification body")
		ctx.JSON(status, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "read body failed",
		})
		return
	}

	// 验证微信签名
	signature := ctx.GetHeader("Wechatpay-Signature")
	timestamp := ctx.GetHeader("Wechatpay-Timestamp")
	nonce := ctx.GetHeader("Wechatpay-Nonce")

	if err := server.ecommerceClient.VerifyNotificationSignature(signature, timestamp, nonce, string(body)); err != nil {
		log.Error().
			Err(err).
			Str("signature", signature).
			Str("timestamp", timestamp).
			Msg("⚠️ invalid wechat applyment signature")
		ctx.JSON(http.StatusUnauthorized, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "signature verification failed",
		})
		return
	}

	// 解析通知
	var notification ApplymentStateNotification
	if err := json.Unmarshal(body, &notification); err != nil {
		log.Error().Err(err).Msg("parse applyment notification")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "parse notification failed",
		})
		return
	}

	// 检查通知类型
	if notification.EventType != "APPLYMENT_STATE.CHANGE" {
		log.Info().Str("event_type", notification.EventType).Msg("ignore non-applyment notification")
		ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{
			Code:    "SUCCESS",
			Message: "OK",
		})
		return
	}

	// 🔐 #1: 原子幂等性门
	if !server.tryClaimApplymentNotification(ctx, notification) {
		return
	}

	// 构造 PaymentNotification 结构以复用现有解密逻辑
	var paymentNotification wechat.PaymentNotification
	if err := json.Unmarshal(body, &paymentNotification); err != nil {
		log.Error().Err(err).Msg("parse payment notification for decryption")
		server.releaseNotification(ctx, notification.ID, "applyment")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "parse notification failed",
		})
		return
	}

	// 使用 paymentClient 解密
	if server.paymentClient == nil {
		log.Error().Msg("payment client not configured for decryption")
		server.releaseNotification(ctx, notification.ID, "applyment")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "payment client not configured",
		})
		return
	}

	decrypted, err := server.paymentClient.DecryptNotificationRaw(&paymentNotification)
	if err != nil {
		log.Error().Err(err).Msg("decrypt applyment notification")
		server.releaseNotification(ctx, notification.ID, "applyment")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "decrypt failed",
		})
		return
	}

	var resource ApplymentStateChangeResource
	if err := json.Unmarshal(decrypted, &resource); err != nil {
		log.Error().Err(err).Str("decrypted", string(decrypted)).Msg("parse applyment resource")
		server.releaseNotification(ctx, notification.ID, "applyment")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "parse resource failed",
		})
		return
	}

	log.Info().
		Int64("applyment_id", resource.ApplymentID).
		Str("out_request_no", resource.OutRequestNo).
		Str("applyment_state", resource.ApplymentState).
		Str("sub_mch_id", resource.SubMchID).
		Msg("received applyment state change notification")

	// 查找进件记录
	applyment, err := server.store.GetEcommerceApplymentByOutRequestNo(ctx, resource.OutRequestNo)
	if err != nil {
		if isNotFoundError(err) {
			log.Warn().Str("out_request_no", resource.OutRequestNo).Msg("applyment record not found")
			server.markNotificationProcessed(ctx, notification.ID, "", "")
			ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{
				Code:    "SUCCESS",
				Message: "OK",
			})
			return
		}
		log.Error().Err(err).Msg("get applyment record")
		server.releaseNotification(ctx, notification.ID, "applyment")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "internal error",
		})
		return
	}
	if applyment.SubjectType != "merchant" && applyment.SubjectType != "operator" {
		log.Warn().
			Int64("applyment_id", applyment.ID).
			Str("subject_type", applyment.SubjectType).
			Msg("ignore unsupported applyment subject type")
		server.markNotificationProcessed(ctx, notification.ID, "", "")
		ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{Code: "SUCCESS", Message: "OK"})
		return
	}

	// 映射状态
	newStatus := mapApplymentStateToDBStatus(resource.ApplymentState)
	if newStatus == "finish" && resource.SubMchID == "" {
		newStatus = "submitted"
	}

	// 如果有二级商户号，说明开户成功
	if resource.SubMchID != "" {
		// 🔐 CB-4: 三步进件激活更新在单个事务中原子完成（UpdateEcommerceApplymentSubMchID +
		// UpdateMerchantPaymentConfig + UpdateMerchantStatus），任意失败整体回滚。
		if txErr := server.store.ApplymentSubMchActivationTx(ctx, db.ApplymentSubMchActivationTxParams{
			ApplymentID: applyment.ID,
			SubjectType: applyment.SubjectType,
			SubjectID:   applyment.SubjectID,
			SubMchID:    resource.SubMchID,
		}); txErr != nil {
			log.Error().Err(txErr).
				Int64("applyment_id", applyment.ID).
				Str("subject_type", applyment.SubjectType).
				Msg("applyment activation tx failed")
			server.releaseNotification(ctx, notification.ID, "applyment")
			ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "internal error"})
			return
		}
		// 非商户主体的额外更新（不在激活事务内，但各自幂等安全）
		switch applyment.SubjectType {
		case "operator":
			if _, err = server.store.UpdateOperatorSubMchID(ctx, db.UpdateOperatorSubMchIDParams{
				ID:       applyment.SubjectID,
				SubMchID: pgtype.Text{String: resource.SubMchID, Valid: true},
			}); err != nil {
				log.Error().Err(err).Int64("operator_id", applyment.SubjectID).Msg("update operator sub_mch_id")
			}
			// 绑卡成功，恢复到 active（清除 bindbank_submitted 瞬时状态）
			if _, err = server.store.UpdateOperatorStatus(ctx, db.UpdateOperatorStatusParams{
				ID:     applyment.SubjectID,
				Status: "active",
			}); err != nil {
				log.Error().Err(err).Int64("operator_id", applyment.SubjectID).Msg("reset operator status to active after sub_mch_id binding")
			}
		}
	} else {
		// 更新进件状态
		_, err = server.store.UpdateEcommerceApplymentStatus(ctx, db.UpdateEcommerceApplymentStatusParams{
			ID:           applyment.ID,
			Status:       newStatus,
			RejectReason: pgtype.Text{}, // 如果有驳回原因需要主动查询
		})
		if err != nil {
			log.Error().Err(err).Int64("applyment_id", applyment.ID).Msg("update applyment status")
		}

		if applyment.SubjectType == "operator" && (newStatus == "rejected" || newStatus == "canceled") {
			// 绑卡被拒/撤销，回到 active（绑卡是可选步骤，不阻塞运营）
			_, err = server.store.UpdateOperatorStatus(ctx, db.UpdateOperatorStatusParams{
				ID:     applyment.SubjectID,
				Status: "active",
			})
			if err != nil {
				log.Error().Err(err).Int64("operator_id", applyment.SubjectID).Msg("reset operator status to active after bindbank rejection")
			}
		}
	}

	// 通知ID已在 tryClaimApplymentNotification 中原子写入，无需重复记录

	// 📤 异步处理：发送通知 + 添加分账接收方
	if server.taskDistributor != nil {
		_ = server.taskDistributor.DistributeTaskProcessApplymentResult(
			ctx,
			&worker.ApplymentResultPayload{
				ApplymentID:    applyment.ID,
				OutRequestNo:   resource.OutRequestNo,
				ApplymentState: resource.ApplymentState,
				SubMchID:       resource.SubMchID,
				SubjectType:    applyment.SubjectType,
				SubjectID:      applyment.SubjectID,
			},
			asynq.MaxRetry(3),
			asynq.Queue(worker.QueueDefault),
		)
	}

	ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{
		Code:    "SUCCESS",
		Message: "OK",
	})
	server.markNotificationProcessed(ctx, notification.ID, "", "")
}

// mapApplymentStateToDBStatus 将微信进件状态映射为数据库状态
func mapApplymentStateToDBStatus(wechatState string) string {
	switch wechatState {
	case "APPLYMENT_STATE_EDITTING":
		return "editing"
	case "APPLYMENT_STATE_AUDITING":
		return "auditing"
	case "APPLYMENT_STATE_REJECTED":
		return "rejected"
	case "APPLYMENT_STATE_TO_BE_CONFIRMED":
		return "to_be_confirmed"
	case "APPLYMENT_STATE_TO_BE_SIGNED":
		return "to_be_signed"
	case "APPLYMENT_STATE_SIGNING":
		return "signing"
	case "APPLYMENT_STATE_FINISHED":
		return "finish"
	case "APPLYMENT_STATE_CANCELED":
		return "canceled"
	default:
		return wechatState
	}
}

// handleOrderSettlementNotify 处理微信订单结算事件通知
// POST /v1/webhooks/wechat-miniprogram/settlement-notify
//
// 微信在用户确认收货（或 T+2 自动确认）后推送 trade_manage_order_settlement 事件。
// settlement_time 字段非空代表资金已实际结算，此时触发分账。
func (server *Server) handleOrderSettlementNotify(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "ecommerce client not configured",
		})
		return
	}

	// 读取请求体用于验签
	body, status, err := readWebhookBody(ctx)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("settlement", "read_body").Inc()
		log.Error().Err(err).Msg("read settlement notification body")
		ctx.JSON(status, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "read body failed",
		})
		return
	}

	// 🔒 验证微信支付签名
	signature := ctx.GetHeader("Wechatpay-Signature")
	timestamp := ctx.GetHeader("Wechatpay-Timestamp")
	nonce := ctx.GetHeader("Wechatpay-Nonce")

	if err := server.ecommerceClient.VerifyNotificationSignature(signature, timestamp, nonce, string(body)); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("settlement", "signature").Inc()
		log.Error().
			Err(err).
			Str("signature", signature).
			Msg("⚠️ invalid signature for settlement notification")
		ctx.JSON(http.StatusUnauthorized, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "signature verification failed",
		})
		return
	}

	// 解析通知外层
	var notification wechat.PaymentNotification
	if err := json.Unmarshal(body, &notification); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("settlement", "parse").Inc()
		log.Error().Err(err).Msg("parse settlement notification")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "parse notification failed",
		})
		return
	}

	if notification.EventType != "trade_manage_order_settlement" {
		log.Info().Str("event_type", notification.EventType).Msg("ignore non-settlement notification")
		ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{Code: "SUCCESS", Message: "OK"})
		return
	}

	// 🔐 #1: 原子幂等性门
	if !server.tryClaimNotification(ctx, notification, "settlement") {
		return
	}

	// 解密通知体
	resource, err := server.ecommerceClient.DecryptSettlementNotification(&notification)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("settlement", "decrypt").Inc()
		log.Error().Err(err).Msg("decrypt settlement notification")
		server.releaseNotification(ctx, notification.ID, "settlement")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "decrypt failed",
		})
		return
	}

	log.Info().
		Str("transaction_id", resource.TransactionID).
		Str("merchant_trade_no", resource.MerchantTradeNo).
		Str("settlement_time", resource.SettlementTime).
		Int("confirm_method", resource.ConfirmReceiveMethod).
		Msg("received settlement notification")

	// settlement_time 为空，说明仅是状态通知，资金未实际结算，跳过分账
	if resource.SettlementTime == "" {
		log.Info().Str("merchant_trade_no", resource.MerchantTradeNo).Msg("settlement_time empty, no actual settlement yet")
		server.markNotificationProcessed(ctx, notification.ID, resource.MerchantTradeNo, resource.TransactionID)
		ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{Code: "SUCCESS", Message: "OK"})
		return
	}

	// 通过子单 out_trade_no 找到对应的业务订单
	subOrder, err := server.store.GetCombinedPaymentSubOrderByOutTradeNo(ctx, resource.MerchantTradeNo)
	if err != nil {
		if isNotFoundError(err) {
			// 未知子单，可能是非 profit_sharing 订单或旧数据，忽略
			log.Warn().Str("merchant_trade_no", resource.MerchantTradeNo).Msg("combined sub order not found, skip settlement")
			server.markNotificationProcessed(ctx, notification.ID, resource.MerchantTradeNo, resource.TransactionID)
			ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{Code: "SUCCESS", Message: "OK"})
			return
		}
		paymentCallbackFailuresTotal.WithLabelValues("settlement", "query_sub_order").Inc()
		log.Error().Err(err).Str("merchant_trade_no", resource.MerchantTradeNo).Msg("get combined sub order")
		server.releaseNotification(ctx, notification.ID, "settlement")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "internal error",
		})
		return
	}

	// 🔐 #12: 通过子单的 OutTradeNo 精确查找对应的支付订单，而不是 GetLatestPaymentOrderByOrder
	// （后者可能返回同一业务订单的更新支付单，导致分账任务使用错误的支付订单ID）
	po, err := server.store.GetPaymentOrderByOutTradeNo(ctx, subOrder.OutTradeNo)
	if err != nil {
		if isNotFoundError(err) {
			log.Warn().Str("out_trade_no", subOrder.OutTradeNo).Msg("payment order not found for settlement, skip")
			server.markNotificationProcessed(ctx, notification.ID, subOrder.OutTradeNo, resource.TransactionID)
			ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{Code: "SUCCESS", Message: "OK"})
			return
		}
		paymentCallbackFailuresTotal.WithLabelValues("settlement", "query_payment_order").Inc()
		log.Error().Err(err).Str("out_trade_no", subOrder.OutTradeNo).Msg("get payment order for settlement")
		server.releaseNotification(ctx, notification.ID, "settlement")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "internal error",
		})
		return
	}

	if po.Status != "paid" || po.PaymentType != "profit_sharing" {
		log.Info().
			Int64("payment_order_id", po.ID).
			Str("status", po.Status).
			Str("type", po.PaymentType).
			Msg("settlement: payment order not eligible for profit sharing, skip")
		server.markNotificationProcessed(ctx, notification.ID, subOrder.OutTradeNo, resource.TransactionID)
		ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{Code: "SUCCESS", Message: "OK"})
		return
	}

	// 通知ID已在 tryClaimNotification 中原子写入，无需重复记录

	// 📤 派发分账任务
	if server.taskDistributor != nil {
		err = server.taskDistributor.DistributeTaskProcessProfitSharing(
			ctx,
			&worker.ProfitSharingPayload{
				PaymentOrderID: po.ID,
				OrderID:        subOrder.OrderID,
			},
			asynq.MaxRetry(5),
			asynq.ProcessIn(2*time.Second), // 短暂延迟确保数据库一致性
			asynq.Queue(worker.QueueCritical),
		)
		if err != nil {
			paymentCallbackFailuresTotal.WithLabelValues("settlement", "enqueue_profit_sharing_task").Inc()
			log.Error().Err(err).
				Int64("payment_order_id", po.ID).
				Str("merchant_trade_no", resource.MerchantTradeNo).
				Msg("⚠️ ALERT: settlement profit sharing task enqueue failed")
			server.sendAlert(websocket.AlertData{
				AlertType:   websocket.AlertTypeTaskEnqueueFailure,
				Level:       websocket.AlertLevelCritical,
				Title:       "结算分账任务入队失败",
				Message:     fmt.Sprintf("支付单 %d 已进入结算事件，但分账任务入队失败，需要人工介入确认商户结算", po.ID),
				RelatedID:   po.ID,
				RelatedType: "payment_order",
				Extra: map[string]interface{}{
					"merchant_trade_no": resource.MerchantTradeNo,
					"out_trade_no":      subOrder.OutTradeNo,
					"order_id":          subOrder.OrderID,
					"payment_order_id":  po.ID,
					"error":             err.Error(),
				},
			})
		} else {
			log.Info().
				Int64("payment_order_id", po.ID).
				Int64("order_id", subOrder.OrderID).
				Str("confirm_method", func() string {
					if resource.ConfirmReceiveMethod == 2 {
						return "T+2 auto"
					}
					return "user"
				}()).
				Msg("profit sharing task dispatched via settlement event")
		}
	}

	server.markNotificationProcessed(ctx, notification.ID, subOrder.OutTradeNo, resource.TransactionID)
	ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{Code: "SUCCESS", Message: "OK"})
}
