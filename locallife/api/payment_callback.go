package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/websocket"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
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

const (
	paymentFactBusinessObjectPaymentOrder       = "payment_order"
	paymentFactBusinessObjectRefundOrder        = "refund_order"
	paymentFactBusinessObjectProfitSharingOrder = "profit_sharing_order"
	paymentFactConsumerOrderDomain              = "order_domain"
	paymentFactConsumerClaimRecoveryDomain      = "claim_recovery_domain"
	paymentFactConsumerProfitSharingDomain      = "profit_sharing_domain"
	paymentFactConsumerRiderDepositDomain       = "rider_deposit_domain"
	paymentFactConsumerReservationDomain        = "reservation_domain"
	paymentFactConsumerBaofuVerifyFeeDomain     = "baofu_account_verify_fee_domain"
	paymentFactApplicationTaskUnique            = 30 * time.Second
)

func (server *Server) sendPaymentSuccessNotification(ctx context.Context, paymentOrder db.PaymentOrder, transactionID string, callbackLabel string) {
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	if err := server.SendNotification(ctx, SendNotificationParams{
		UserID:      paymentOrder.UserID,
		Type:        "payment",
		Title:       "支付成功",
		Content:     fmt.Sprintf("您的订单支付已完成，支付金额%s元", fenToYuanString(paymentOrder.Amount, 2)),
		RelatedType: "payment",
		RelatedID:   paymentOrder.ID,
		ExtraData: map[string]any{
			"out_trade_no":   paymentOrder.OutTradeNo,
			"transaction_id": transactionID,
			"amount":         paymentOrder.Amount,
			"business_type":  paymentOrder.BusinessType,
		},
		ExpiresAt:         &expiresAt,
		IgnorePreferences: true,
	}); err != nil {
		log.Error().Err(err).
			Int64("payment_order_id", paymentOrder.ID).
			Str("out_trade_no", paymentOrder.OutTradeNo).
			Str("callback_label", callbackLabel).
			Msg("send payment success notification failed")
	}
}

func (server *Server) enqueueProfitSharingPaymentFactApplication(ctx context.Context, application *db.ExternalPaymentFactApplication) {
	if application == nil || server.taskDistributor == nil {
		return
	}
	distributor, ok := server.taskDistributor.(worker.PaymentFactApplicationTaskDistributor)
	if !ok {
		return
	}
	if err := distributor.DistributeTaskProcessPaymentFactApplication(
		ctx,
		&worker.PaymentFactApplicationPayload{ApplicationID: application.ID},
		asynq.MaxRetry(5),
		asynq.Queue(worker.QueueCritical),
		asynq.Unique(paymentFactApplicationTaskUnique),
	); err != nil {
		log.Warn().Err(err).
			Int64("payment_fact_application_id", application.ID).
			Int64("payment_fact_id", application.FactID).
			Int64("profit_sharing_order_id", application.BusinessObjectID).
			Msg("enqueue profit sharing payment fact application from callback failed; scheduler will retry")
	}
}

func (server *Server) enqueueRiderDepositRefundPaymentFactApplication(ctx context.Context, application *db.ExternalPaymentFactApplication) {
	if application == nil || server.taskDistributor == nil {
		return
	}
	distributor, ok := server.taskDistributor.(worker.PaymentFactApplicationTaskDistributor)
	if !ok {
		return
	}
	if err := distributor.DistributeTaskProcessPaymentFactApplication(
		ctx,
		&worker.PaymentFactApplicationPayload{ApplicationID: application.ID},
		asynq.MaxRetry(5),
		asynq.Queue(worker.QueueCritical),
		asynq.Unique(paymentFactApplicationTaskUnique),
	); err != nil {
		log.Warn().Err(err).
			Int64("payment_fact_application_id", application.ID).
			Int64("payment_fact_id", application.FactID).
			Int64("refund_order_id", application.BusinessObjectID).
			Msg("enqueue rider deposit refund payment fact application from callback failed; scheduler will retry")
	}
}

func (server *Server) enqueueReservationRefundPaymentFactApplication(ctx context.Context, application *db.ExternalPaymentFactApplication) {
	if application == nil || server.taskDistributor == nil {
		return
	}
	distributor, ok := server.taskDistributor.(worker.PaymentFactApplicationTaskDistributor)
	if !ok {
		return
	}
	if err := distributor.DistributeTaskProcessPaymentFactApplication(
		ctx,
		&worker.PaymentFactApplicationPayload{ApplicationID: application.ID},
		asynq.MaxRetry(5),
		asynq.Queue(worker.QueueCritical),
		asynq.Unique(paymentFactApplicationTaskUnique),
	); err != nil {
		log.Warn().Err(err).
			Int64("payment_fact_application_id", application.ID).
			Int64("payment_fact_id", application.FactID).
			Int64("refund_order_id", application.BusinessObjectID).
			Msg("enqueue reservation refund payment fact application from callback failed; scheduler will retry")
	}
}

func (server *Server) enqueueOrderRefundPaymentFactApplication(ctx context.Context, application *db.ExternalPaymentFactApplication) {
	if application == nil || server.taskDistributor == nil {
		return
	}
	distributor, ok := server.taskDistributor.(worker.PaymentFactApplicationTaskDistributor)
	if !ok {
		return
	}
	if err := distributor.DistributeTaskProcessPaymentFactApplication(
		ctx,
		&worker.PaymentFactApplicationPayload{ApplicationID: application.ID},
		asynq.MaxRetry(5),
		asynq.Queue(worker.QueueCritical),
		asynq.Unique(paymentFactApplicationTaskUnique),
	); err != nil {
		log.Warn().Err(err).
			Int64("payment_fact_application_id", application.ID).
			Int64("payment_fact_id", application.FactID).
			Int64("refund_order_id", application.BusinessObjectID).
			Msg("enqueue order refund payment fact application from callback failed; scheduler will retry")
	}
}

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

func writeWechatNotifySuccess(ctx *gin.Context, metricLabel string) {
	_ = metricLabel
	ctx.Status(http.StatusNoContent)
}

func parseClaimPayoutActionIDFromOutBillNo(outBillNo string) (int64, error) {
	trimmed := strings.TrimSpace(outBillNo)
	const prefix = "claimpayout"
	if !strings.HasPrefix(trimmed, prefix) {
		return 0, fmt.Errorf("unsupported claim payout out_bill_no: %s", outBillNo)
	}
	actionID, err := strconv.ParseInt(strings.TrimPrefix(trimmed, prefix), 10, 64)
	if err != nil || actionID <= 0 {
		return 0, fmt.Errorf("invalid claim payout out_bill_no: %s", outBillNo)
	}
	return actionID, nil
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
		writeWechatNotifySuccess(ctx, metricLabel)
		return false
	}

	if existing.CreatedAt.Valid && time.Since(existing.CreatedAt.Time) > notificationClaimStaleWindow {
		log.Warn().
			Str("notification_id", notificationID).
			Str("callback_type", metricLabel).
			Msg("notification claim stale, releasing for retry")
		server.releaseNotificationWithReason(ctx, notificationID, metricLabel, "stale_claim_retry")
		paymentCallbackFailuresTotal.WithLabelValues(metricLabel, "stale_claim_retry").Inc()
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "stale claim, please retry"})
		return false
	}

	paymentCallbackFailuresTotal.WithLabelValues(metricLabel, "inflight_claim").Inc()
	log.Warn().
		Str("notification_id", notificationID).
		Str("callback_type", metricLabel).
		Msg("notification already in processing")
	ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "notification in processing"})
	return false
}

func (server *Server) validateDirectPaymentOwnership(resource *wechatcontracts.DirectPaymentNotificationResource) error {
	if resource == nil {
		return errors.New("payment resource is nil")
	}
	if resource.MchID == "" && resource.AppID == "" {
		return nil
	}
	provider, ok := server.directPaymentClient.(directPaymentOwnershipProvider)
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

func (server *Server) validateDirectRefundOwnership(resource *wechatcontracts.DirectRefundNotificationResource) error {
	if resource == nil || resource.MchID == "" {
		return nil
	}
	provider, ok := server.directPaymentClient.(directPaymentOwnershipProvider)
	if !ok {
		return errors.New("payment client ownership metadata unavailable")
	}
	if resource.MchID != provider.GetMchID() {
		return fmt.Errorf("mchid mismatch")
	}
	return nil
}

func (server *Server) resolveRiderDepositRefundFactObjects(ctx context.Context, outRefundNo string) (db.RefundOrder, db.PaymentOrder, bool, error) {
	if server.paymentFactService == nil {
		return db.RefundOrder{}, db.PaymentOrder{}, false, nil
	}
	refundOrder, err := server.store.GetRefundOrderByOutRefundNo(ctx, outRefundNo)
	if err != nil {
		if !isNotFoundError(err) {
			log.Warn().Err(err).Str("out_refund_no", outRefundNo).Msg("skip rider deposit refund fact because local refund order resolution failed")
		}
		return db.RefundOrder{}, db.PaymentOrder{}, false, nil
	}
	paymentOrder, err := server.store.GetPaymentOrder(ctx, refundOrder.PaymentOrderID)
	if err != nil {
		log.Warn().Err(err).
			Int64("refund_order_id", refundOrder.ID).
			Int64("payment_order_id", refundOrder.PaymentOrderID).
			Str("out_refund_no", outRefundNo).
			Msg("skip rider deposit refund fact because local payment order resolution failed")
		return db.RefundOrder{}, db.PaymentOrder{}, false, nil
	}
	return refundOrder, paymentOrder, paymentOrder.BusinessType == db.ExternalPaymentBusinessOwnerRiderDeposit, nil
}

func (server *Server) recordDirectPaymentCallbackFact(ctx context.Context, notification wechat.PaymentNotification, paymentOrder db.PaymentOrder, resource *wechatcontracts.DirectPaymentNotificationResource) (*db.ExternalPaymentFactApplication, error) {
	if server.paymentFactService == nil || resource == nil {
		return nil, nil
	}
	consumer := ""
	businessOwner := ""
	switch paymentOrder.BusinessType {
	case db.ExternalPaymentBusinessOwnerRiderDeposit:
		consumer = paymentFactConsumerRiderDepositDomain
		businessOwner = db.ExternalPaymentBusinessOwnerRiderDeposit
	case db.ExternalPaymentBusinessOwnerClaimRecovery:
		consumer = paymentFactConsumerClaimRecoveryDomain
		businessOwner = db.ExternalPaymentBusinessOwnerClaimRecovery
	case db.ExternalPaymentBusinessOwnerBaofuVerifyFee:
		consumer = paymentFactConsumerBaofuVerifyFeeDomain
		businessOwner = db.ExternalPaymentBusinessOwnerBaofuVerifyFee
	default:
		return nil, nil
	}
	status := logic.NormalizeDirectPaymentTerminalStatus(resource.TradeState)
	if status == db.ExternalPaymentTerminalStatusUnknown && notification.EventType == "TRANSACTION.SUCCESS" {
		status = db.ExternalPaymentTerminalStatusSuccess
	}
	occurredAt := parseWechatFactTime(resource.SuccessTime)
	result, err := server.paymentFactService.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelDirect,
		Capability:           db.ExternalPaymentCapabilityDirectJSAPIPayment,
		FactSource:           db.ExternalPaymentFactSourceCallback,
		SourceEventID:        paymentFactStringPtr(notification.ID),
		SourceEventType:      paymentFactStringPtr(notification.EventType),
		ExternalObjectType:   db.ExternalPaymentObjectPayment,
		ExternalObjectKey:    resource.OutTradeNo,
		ExternalSecondaryKey: paymentFactStringPtr(resource.TransactionID),
		BusinessOwner:        paymentFactStringPtr(businessOwner),
		BusinessObjectType:   paymentFactStringPtr(paymentFactBusinessObjectPaymentOrder),
		BusinessObjectID:     paymentFactInt64Ptr(paymentOrder.ID),
		UpstreamState:        "SUCCESS",
		TerminalStatus:       status,
		Amount:               paymentFactInt64Ptr(resource.Amount.Total),
		Currency:             "CNY",
		OccurredAt:           occurredAt,
		UpstreamUpdatedAt:    occurredAt,
		RawResource:          directPaymentFactResource(resource),
		DedupeKey:            fmt.Sprintf("wechat:callback:direct_payment:%s", notification.ID),
		Application: &logic.ExternalPaymentFactApplicationTarget{
			Consumer:           consumer,
			BusinessObjectType: paymentFactBusinessObjectPaymentOrder,
			BusinessObjectID:   paymentOrder.ID,
		},
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}

func (server *Server) recordRiderDepositDirectRefundCallbackFact(ctx context.Context, notification wechat.PaymentNotification, refundOrder db.RefundOrder, paymentOrder db.PaymentOrder, resource *wechatcontracts.DirectRefundNotificationResource) (*db.ExternalPaymentFactApplication, error) {
	if server.paymentFactService == nil || paymentOrder.BusinessType != db.ExternalPaymentBusinessOwnerRiderDeposit || resource == nil {
		return nil, nil
	}
	occurredAt := parseWechatFactTime(resource.SuccessTime)
	result, err := server.paymentFactService.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelDirect,
		Capability:           db.ExternalPaymentCapabilityDirectRefund,
		FactSource:           db.ExternalPaymentFactSourceCallback,
		SourceEventID:        paymentFactStringPtr(notification.ID),
		SourceEventType:      paymentFactStringPtr(notification.EventType),
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalObjectKey:    resource.OutRefundNo,
		ExternalSecondaryKey: paymentFactStringPtr(resource.RefundID),
		BusinessOwner:        paymentFactStringPtr(db.ExternalPaymentBusinessOwnerRiderDeposit),
		BusinessObjectType:   paymentFactStringPtr(paymentFactBusinessObjectRefundOrder),
		BusinessObjectID:     paymentFactInt64Ptr(refundOrder.ID),
		UpstreamState:        resource.RefundStatus,
		TerminalStatus:       logic.NormalizeDirectRefundTerminalStatus(resource.RefundStatus),
		Amount:               paymentFactInt64Ptr(resource.Amount.Refund),
		Currency:             "CNY",
		OccurredAt:           occurredAt,
		UpstreamUpdatedAt:    occurredAt,
		RawResource:          directRefundFactResource(resource),
		DedupeKey:            fmt.Sprintf("wechat:callback:direct_refund:%s", notification.ID),
		Application: &logic.ExternalPaymentFactApplicationTarget{
			Consumer:           paymentFactConsumerRiderDepositDomain,
			BusinessObjectType: paymentFactBusinessObjectRefundOrder,
			BusinessObjectID:   refundOrder.ID,
		},
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}

func (server *Server) validateDirectMerchantTransferOwnership(resource *wechatcontracts.DirectMerchantTransferNotificationResource) error {
	if resource == nil || resource.MchID == "" {
		return nil
	}
	if server.transferClient == nil {
		return errors.New("transfer client ownership metadata unavailable")
	}
	if resource.MchID != server.transferClient.GetMchID() {
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
	// 骑手押金无关联订单，但 worker 有独立的直连退款分支
	if paymentOrder.BusinessType == "rider_deposit" {
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

func (server *Server) requireTaskDistributorForNotification(ctx *gin.Context, notificationID, callbackType, responseMessage string) bool {
	if server.taskDistributor != nil {
		return true
	}

	paymentCallbackFailuresTotal.WithLabelValues(callbackType, "task_distributor_missing").Inc()
	log.Error().
		Str("notification_id", notificationID).
		Str("callback_type", callbackType).
		Msg("callback cannot be acknowledged because task distributor is not configured")
	server.releaseNotification(ctx, notificationID, callbackType)
	ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
		Code:    "FAIL",
		Message: responseMessage,
	})
	return false
}

// handleMerchantTransferNotify 处理商家转账回调通知。
// POST /v1/webhooks/wechat-pay/merchant-transfer-notify
func (server *Server) handleMerchantTransferNotify(ctx *gin.Context) {
	if server.transferClient == nil {
		log.Error().Msg("merchant transfer callback received but merchant transfer client is not configured")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "merchant transfer client not configured",
		})
		return
	}

	body, status, err := readWebhookBody(ctx)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("merchant_transfer", "read_body").Inc()
		log.Error().Err(err).Msg("read merchant transfer notification body")
		ctx.JSON(status, wechatPaymentNotifyResponse{Code: "FAIL", Message: "read body failed"})
		return
	}

	signature := ctx.GetHeader("Wechatpay-Signature")
	timestamp := ctx.GetHeader("Wechatpay-Timestamp")
	nonce := ctx.GetHeader("Wechatpay-Nonce")
	serial := ctx.GetHeader("Wechatpay-Serial")
	if err := server.transferClient.VerifyNotificationSignature(signature, timestamp, nonce, serial, string(body)); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("merchant_transfer", "signature").Inc()
		log.Error().Err(err).Msg("invalid wechat signature for merchant transfer notification")
		ctx.JSON(http.StatusUnauthorized, wechatPaymentNotifyResponse{Code: "FAIL", Message: "signature verification failed"})
		return
	}

	var notification wechat.PaymentNotification
	if err := json.Unmarshal(body, &notification); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("merchant_transfer", "parse").Inc()
		log.Error().Err(err).Msg("parse merchant transfer notification")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{Code: "FAIL", Message: "parse notification failed"})
		return
	}

	if notification.EventType != wechatcontracts.DirectMerchantTransferNotifyEventTypeBillFinished {
		log.Info().Str("event_type", notification.EventType).Msg("ignore non-merchant-transfer-finished notification")
		writeWechatNotifySuccess(ctx, "merchant_transfer")
		return
	}

	if !server.tryClaimNotification(ctx, notification, "merchant_transfer") {
		return
	}

	resource, err := server.transferClient.DecryptMerchantTransferNotification(&notification)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("merchant_transfer", "decrypt").Inc()
		log.Error().Err(err).Msg("decrypt merchant transfer notification")
		server.releaseNotification(ctx, notification.ID, "merchant_transfer")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{Code: "FAIL", Message: "decrypt failed"})
		return
	}

	if err := server.validateDirectMerchantTransferOwnership(resource); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("merchant_transfer", "ownership").Inc()
		log.Error().Err(err).
			Str("out_bill_no", resource.OutBillNo).
			Str("mchid", resource.MchID).
			Msg("merchant transfer notification ownership validation failed")
		server.sendAlert(websocket.AlertData{
			AlertType:   websocket.AlertTypeSystemError,
			Level:       websocket.AlertLevelCritical,
			Title:       "商家转账回调归属校验失败",
			Message:     fmt.Sprintf("商家转账回调 out_bill_no=%s 的归属校验失败，mchid=%s。系统已返回 FAIL 等待微信重试，请排查是否存在错商户转账回调。", resource.OutBillNo, resource.MchID),
			RelatedID:   0,
			RelatedType: "behavior_action",
			Extra: map[string]interface{}{
				"out_bill_no":      resource.OutBillNo,
				"transfer_bill_no": resource.TransferBillNo,
				"mchid":            resource.MchID,
			},
		})
		server.releaseNotification(ctx, notification.ID, "merchant_transfer")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "ownership validation failed"})
		return
	}

	actionID, err := parseClaimPayoutActionIDFromOutBillNo(resource.OutBillNo)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("merchant_transfer", "parse_out_bill_no").Inc()
		log.Error().Err(err).Str("out_bill_no", resource.OutBillNo).Msg("resolve claim payout action from merchant transfer notification")
		server.releaseNotification(ctx, notification.ID, "merchant_transfer")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{Code: "FAIL", Message: "invalid out_bill_no"})
		return
	}

	if err := worker.HandleClaimPayoutTransferNotification(ctx, server.store, server.taskDistributor, actionID, resource); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("merchant_transfer", "apply_business").Inc()
		log.Error().Err(err).Int64("behavior_action_id", actionID).Str("out_bill_no", resource.OutBillNo).Msg("apply merchant transfer notification failed")
		server.releaseNotification(ctx, notification.ID, "merchant_transfer")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "apply notification failed"})
		return
	}

	server.markNotificationProcessed(ctx, notification.ID, resource.OutBillNo, resource.TransferBillNo)
	writeWechatNotifySuccess(ctx, "merchant_transfer")
}

// handlePaymentNotify 处理微信支付回调通知
// POST /v1/webhooks/wechat-pay/notify
// 设计原则：快速响应微信，耗时操作放入队列
func (server *Server) handlePaymentNotify(ctx *gin.Context) {
	if server.directPaymentClient == nil {
		log.Error().Msg("payment callback received but payment client is not configured")
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
	serial := ctx.GetHeader("Wechatpay-Serial")

	if err := server.directPaymentClient.VerifyNotificationSignature(signature, timestamp, nonce, serial, string(body)); err != nil {
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
		writeWechatNotifySuccess(ctx, "payment")
		return
	}

	// 🔐 #1: 原子幂等性门 — INSERT ON CONFLICT DO NOTHING 防止并发重复处理
	if !server.tryClaimNotification(ctx, notification, "payment") {
		return
	}

	// 解密通知内容
	resource, err := server.directPaymentClient.DecryptPaymentNotification(&notification)
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
		if !paymentOrder.ProcessedAt.Valid {
			if isSupportedDirectPaymentFactBusinessType(paymentOrder.BusinessType) {
				paymentFactApplication, err := server.recordDirectPaymentCallbackFact(ctx, notification, paymentOrder, resource)
				if err != nil {
					paymentCallbackFailuresTotal.WithLabelValues("payment", "record_payment_fact").Inc()
					log.Error().Err(err).
						Int64("payment_order_id", paymentOrder.ID).
						Str("out_trade_no", resource.OutTradeNo).
						Msg("record direct payment fact failed")
					server.releaseNotification(ctx, notification.ID, "payment")
					ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "payment fact recording failed, please retry"})
					return
				}
				if paymentFactApplication == nil {
					paymentCallbackFailuresTotal.WithLabelValues("payment", "missing_payment_fact_application").Inc()
					log.Error().
						Int64("payment_order_id", paymentOrder.ID).
						Str("out_trade_no", paymentOrder.OutTradeNo).
						Str("business_type", paymentOrder.BusinessType).
						Msg("direct payment callback missing payment fact application")
					server.releaseNotification(ctx, notification.ID, "payment")
					ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
						Code:    "FAIL",
						Message: "payment fact application missing, please retry",
					})
					return
				}
				server.enqueueDirectPaymentFactApplication(ctx, paymentFactApplication)
			} else {
				paymentCallbackFailuresTotal.WithLabelValues("payment", "unsupported_payment_fact_owner").Inc()
				log.Error().
					Int64("payment_order_id", paymentOrder.ID).
					Str("out_trade_no", paymentOrder.OutTradeNo).
					Str("business_type", paymentOrder.BusinessType).
					Str("payment_channel", paymentOrder.PaymentChannel).
					Msg("direct payment order already paid but no payment fact application owner matched")
				server.releaseNotification(ctx, notification.ID, "payment")
				ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
					Code:    "FAIL",
					Message: "unsupported payment business type, please retry",
				})
				return
			}
		}
		log.Info().Int64("id", paymentOrder.ID).Msg("payment order already paid")
		server.markNotificationProcessed(ctx, notification.ID, resource.OutTradeNo, resource.TransactionID)
		writeWechatNotifySuccess(ctx, "payment")
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
		writeWechatNotifySuccess(ctx, "payment")
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
		log.Info().
			Int64("payment_order_id", paymentOrder.ID).
			Str("out_trade_no", resource.OutTradeNo).
			Str("result", mismatchMsg).
			Msg("payment callback acknowledged after amount mismatch handling")
		server.markNotificationProcessed(ctx, notification.ID, resource.OutTradeNo, resource.TransactionID)
		writeWechatNotifySuccess(ctx, "payment")
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
	server.sendPaymentSuccessNotification(ctx, updatedPaymentOrder, resource.TransactionID, "payment")

	// 通知ID已在 tryClaimNotification 中原子写入，无需重复记录

	if isSupportedDirectPaymentFactBusinessType(updatedPaymentOrder.BusinessType) {
		paymentFactApplication, err := server.recordDirectPaymentCallbackFact(ctx, notification, updatedPaymentOrder, resource)
		if err != nil {
			paymentCallbackFailuresTotal.WithLabelValues("payment", "record_payment_fact").Inc()
			log.Error().Err(err).
				Int64("payment_order_id", updatedPaymentOrder.ID).
				Str("out_trade_no", resource.OutTradeNo).
				Msg("record direct payment fact failed")
			server.releaseNotification(ctx, notification.ID, "payment")
			ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "payment fact recording failed, please retry"})
			return
		}
		if paymentFactApplication == nil {
			paymentCallbackFailuresTotal.WithLabelValues("payment", "missing_payment_fact_application").Inc()
			log.Error().
				Int64("payment_order_id", updatedPaymentOrder.ID).
				Str("out_trade_no", updatedPaymentOrder.OutTradeNo).
				Str("business_type", updatedPaymentOrder.BusinessType).
				Msg("direct payment callback missing payment fact application after paid transition")
			server.releaseNotification(ctx, notification.ID, "payment")
			ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
				Code:    "FAIL",
				Message: "payment fact application missing, please retry",
			})
			return
		}
		server.enqueueDirectPaymentFactApplication(ctx, paymentFactApplication)
	} else {
		paymentCallbackFailuresTotal.WithLabelValues("payment", "unsupported_payment_fact_owner").Inc()
		log.Error().
			Int64("payment_order_id", paymentOrder.ID).
			Str("out_trade_no", paymentOrder.OutTradeNo).
			Str("business_type", paymentOrder.BusinessType).
			Str("payment_channel", paymentOrder.PaymentChannel).
			Str("alert_type", "TASK_ENQUEUE_FAILURE").
			Msg("direct payment callback paid transition has no payment fact application owner")
		server.sendAlert(websocket.AlertData{
			AlertType:   websocket.AlertTypeTaskEnqueueFailure,
			Level:       websocket.AlertLevelCritical,
			Title:       "支付事实应用匹配失败",
			Message:     fmt.Sprintf("支付单 %s 支付成功，但业务类型未匹配支付事实应用，系统已返回 FAIL 请求微信重试。", paymentOrder.OutTradeNo),
			RelatedID:   paymentOrder.ID,
			RelatedType: "payment_order",
			Extra: map[string]interface{}{
				"out_trade_no":  paymentOrder.OutTradeNo,
				"business_type": paymentOrder.BusinessType,
			},
		})
		server.releaseNotification(ctx, notification.ID, "payment")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "unsupported payment business type, please retry",
		})
		return
	}

	// 快速返回成功
	writeWechatNotifySuccess(ctx, "payment")
	server.markNotificationProcessed(ctx, notification.ID, resource.OutTradeNo, resource.TransactionID)
}

// handleRefundNotify 处理微信退款回调通知
// POST /v1/webhooks/wechat-pay/refund-notify
func (server *Server) handleRefundNotify(ctx *gin.Context) {
	if server.directPaymentClient == nil {
		log.Error().Msg("refund callback received but payment client is not configured")
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
	serial := ctx.GetHeader("Wechatpay-Serial")

	if err := server.directPaymentClient.VerifyNotificationSignature(signature, timestamp, nonce, serial, string(body)); err != nil {
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
		writeWechatNotifySuccess(ctx, "direct_refund")
		return
	}

	// 🔐 #1: 原子幂等性门
	if !server.tryClaimNotification(ctx, notification, "refund") {
		return
	}

	// 解密通知内容
	resource, err := server.directPaymentClient.DecryptRefundNotification(&notification)
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

	if server.taskDistributor == nil {
		paymentCallbackFailuresTotal.WithLabelValues("refund", "task_distributor_missing").Inc()
		log.Error().
			Str("out_refund_no", resource.OutRefundNo).
			Msg("refund callback cannot be acknowledged because task distributor is not configured")
		server.releaseNotification(ctx, notification.ID, "refund")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "refund result processing unavailable, please retry",
		})
		return
	}

	refundOrder, paymentOrder, shouldRecordFact, err := server.resolveRiderDepositRefundFactObjects(ctx, resource.OutRefundNo)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("refund", "resolve_refund_fact_object").Inc()
		log.Error().Err(err).
			Str("out_refund_no", resource.OutRefundNo).
			Msg("resolve rider deposit refund fact object failed")
		server.releaseNotification(ctx, notification.ID, "refund")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "refund fact object resolution failed, please retry"})
		return
	}
	if shouldRecordFact {
		application, err := server.recordRiderDepositDirectRefundCallbackFact(ctx, notification, refundOrder, paymentOrder, resource)
		if err != nil {
			paymentCallbackFailuresTotal.WithLabelValues("refund", "record_refund_fact").Inc()
			log.Error().Err(err).
				Int64("refund_order_id", refundOrder.ID).
				Str("out_refund_no", resource.OutRefundNo).
				Msg("record rider deposit direct refund fact failed")
			server.releaseNotification(ctx, notification.ID, "refund")
			ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "refund fact recording failed, please retry"})
			return
		}
		server.enqueueRiderDepositRefundPaymentFactApplication(ctx, application)
	}

	if !shouldRecordFact {
		// 将非骑手押金退款结果处理放入旧队列
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
	writeWechatNotifySuccess(ctx, "direct_refund")
	server.markNotificationProcessed(ctx, notification.ID, resource.OutTradeNo, resource.TransactionID)
}
