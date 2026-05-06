package api

import (
	"bytes"
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
	ordinaryserviceprovider "github.com/merrydance/locallife/wechat/ordinaryserviceprovider"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
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

func (server *Server) readVerifiedEcommerceSuccessNotification(ctx *gin.Context, callbackType string) (*wechat.PaymentNotification, bool) {
	if server.ecommerceClient == nil {
		log.Error().Str("callback_type", callbackType).Msg("ecommerce callback received but ecommerce client is not configured")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "ecommerce client not configured",
		})
		return nil, false
	}

	body, status, err := readWebhookBody(ctx)
	if err != nil {
		log.Error().Err(err).Str("callback_type", callbackType).Msg("read ecommerce notification body")
		ctx.JSON(status, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "read body failed",
		})
		return nil, false
	}

	signature := ctx.GetHeader("Wechatpay-Signature")
	timestamp := ctx.GetHeader("Wechatpay-Timestamp")
	nonce := ctx.GetHeader("Wechatpay-Nonce")
	serial := ctx.GetHeader("Wechatpay-Serial")

	if err := server.ecommerceClient.VerifyNotificationSignature(signature, timestamp, nonce, serial, string(body)); err != nil {
		log.Error().Err(err).Str("callback_type", callbackType).Msg("invalid wechat signature for ecommerce notification")
		ctx.JSON(http.StatusUnauthorized, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "signature verification failed",
		})
		return nil, false
	}

	var notification wechat.PaymentNotification
	if err := json.Unmarshal(body, &notification); err != nil {
		log.Error().Err(err).Str("callback_type", callbackType).Msg("parse ecommerce notification")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "parse notification failed",
		})
		return nil, false
	}

	if notification.EventType != "TRANSACTION.SUCCESS" {
		log.Info().Str("callback_type", callbackType).Str("event_type", notification.EventType).Msg("ignore non-success notification")
		writeWechatNotifySuccess(ctx, callbackType)
		return nil, false
	}

	return &notification, true
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

func (server *Server) validateCombineNotifyOwnership(resource *wechatcontracts.CombinePaymentNotification) error {
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

func (server *Server) validatePartnerNotifyOwnership(resource *wechatcontracts.PartnerPaymentNotificationResource) error {
	if server.ecommerceClient == nil {
		return errors.New("ecommerce client not configured")
	}
	if resource.SpMchID != "" && resource.SpMchID != server.ecommerceClient.GetSpMchID() {
		return fmt.Errorf("sp_mchid mismatch")
	}
	if resource.SpAppID != "" && resource.SpAppID != server.ecommerceClient.GetSpAppID() {
		return fmt.Errorf("sp_appid mismatch")
	}
	return nil
}

func (server *Server) validateOrdinaryPaymentNotifyOwnership(resource *ospcontracts.PaymentNotificationPayload) error {
	if server.ordinarySPClient == nil {
		return errors.New("ordinary service provider client not configured")
	}
	if resource == nil {
		return errors.New("ordinary payment resource is nil")
	}
	if resource.SpMchID != "" && resource.SpMchID != server.ordinarySPClient.ServiceProviderMchID() {
		return fmt.Errorf("sp_mchid mismatch")
	}
	if resource.SpAppID != "" && resource.SpAppID != server.ordinarySPClient.ServiceProviderAppID() {
		return fmt.Errorf("sp_appid mismatch")
	}
	return nil
}

func (server *Server) validateOrdinaryCombinePaymentNotifyOwnership(resource *ospcontracts.CombinePaymentNotificationPayload) error {
	if server.ordinarySPClient == nil {
		return errors.New("ordinary service provider client not configured")
	}
	if resource == nil {
		return errors.New("ordinary combine payment resource is nil")
	}
	if resource.CombineMchID != "" && resource.CombineMchID != server.ordinarySPClient.ServiceProviderMchID() {
		return fmt.Errorf("combine_mchid mismatch")
	}
	if resource.CombineAppID != "" && resource.CombineAppID != server.ordinarySPClient.ServiceProviderAppID() {
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

func (server *Server) validateOrdinaryRefundOwnership(resource *ospcontracts.RefundNotificationPayload) error {
	if server.ordinarySPClient == nil {
		return errors.New("ordinary service provider client not configured")
	}
	if resource == nil {
		return errors.New("ordinary refund resource is nil")
	}
	if strings.TrimSpace(resource.SpMchID) == "" {
		return errors.New("sp_mchid missing")
	}
	if resource.SpMchID != server.ordinarySPClient.ServiceProviderMchID() {
		return fmt.Errorf("sp_mchid mismatch")
	}
	return nil
}

func (server *Server) validateOrdinaryRefundLocalOwnership(ctx context.Context, resource *ospcontracts.RefundNotificationPayload, refundOrder db.RefundOrder, paymentOrder db.PaymentOrder) error {
	if resource == nil {
		return errors.New("ordinary refund resource is nil")
	}
	if strings.TrimSpace(resource.OutRefundNo) != strings.TrimSpace(refundOrder.OutRefundNo) {
		return fmt.Errorf("out_refund_no mismatch")
	}
	if strings.TrimSpace(resource.OutTradeNo) != strings.TrimSpace(paymentOrder.OutTradeNo) {
		return fmt.Errorf("out_trade_no mismatch")
	}
	if paymentOrder.TransactionID.Valid && strings.TrimSpace(paymentOrder.TransactionID.String) != "" && strings.TrimSpace(resource.TransactionID) != strings.TrimSpace(paymentOrder.TransactionID.String) {
		return fmt.Errorf("transaction_id mismatch")
	}
	if resource.Amount == nil || resource.Amount.Refund <= 0 {
		return fmt.Errorf("amount.refund missing")
	}
	if refundOrder.RefundAmount > 0 && resource.Amount.Refund != refundOrder.RefundAmount {
		return fmt.Errorf("refund amount mismatch")
	}
	expectedSubMchID, err := server.expectedRefundSubMchID(ctx, paymentOrder)
	if err != nil {
		return err
	}
	if strings.TrimSpace(resource.SubMchID) != expectedSubMchID {
		return fmt.Errorf("sub_mchid mismatch")
	}
	return nil
}

func (server *Server) expectedRefundSubMchID(ctx context.Context, paymentOrder db.PaymentOrder) (string, error) {
	if server.store == nil {
		return "", errors.New("store not configured")
	}
	merchantID := int64(0)
	switch {
	case paymentOrder.OrderID.Valid:
		order, err := server.store.GetOrder(ctx, paymentOrder.OrderID.Int64)
		if err != nil {
			return "", fmt.Errorf("get refund payment order owner: %w", err)
		}
		merchantID = order.MerchantID
	case paymentOrder.ReservationID.Valid:
		reservation, err := server.store.GetTableReservation(ctx, paymentOrder.ReservationID.Int64)
		if err != nil {
			return "", fmt.Errorf("get refund reservation owner: %w", err)
		}
		merchantID = reservation.MerchantID
	default:
		return "", errors.New("refund payment order missing order or reservation owner")
	}
	if merchantID == 0 {
		return "", errors.New("refund payment owner merchant missing")
	}
	paymentConfig, err := server.store.GetMerchantPaymentConfig(ctx, merchantID)
	if err != nil {
		return "", fmt.Errorf("get merchant payment config: %w", err)
	}
	expectedSubMchID := strings.TrimSpace(paymentConfig.SubMchID)
	if expectedSubMchID == "" {
		return "", errors.New("merchant payment config sub_mchid missing")
	}
	return expectedSubMchID, nil
}

func (server *Server) validateProfitSharingOwnership(resource *wechatcontracts.ProfitSharingNotification) error {
	if server.ecommerceClient == nil {
		return errors.New("ecommerce client not configured")
	}
	if resource.SPMchID != "" && resource.SPMchID != server.ecommerceClient.GetSpMchID() {
		return fmt.Errorf("mchid mismatch")
	}
	return nil
}

func (server *Server) validateProfitSharingSubMerchantOwnership(ctx context.Context, resource *wechatcontracts.ProfitSharingNotification, order db.ProfitSharingOrder) error {
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

func paymentFactStringPtr(value string) *string {
	return &value
}

func paymentFactInt64Ptr(value int64) *int64 {
	return &value
}

func parseWechatFactTime(value string) *time.Time {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		log.Warn().Err(err).Str("wechat_time", value).Msg("parse wechat payment fact time failed")
		return nil
	}
	return &parsed
}

func directPaymentFactResource(resource *wechatcontracts.DirectPaymentNotificationResource) []byte {
	if resource == nil {
		return nil
	}
	raw, err := json.Marshal(map[string]any{
		"appid":            resource.AppID,
		"mchid":            resource.MchID,
		"out_trade_no":     resource.OutTradeNo,
		"transaction_id":   resource.TransactionID,
		"trade_type":       resource.TradeType,
		"trade_state":      resource.TradeState,
		"trade_state_desc": resource.TradeStateDesc,
		"amount_total":     resource.Amount.Total,
		"success_time":     resource.SuccessTime,
	})
	if err != nil {
		log.Warn().Err(err).Str("out_trade_no", resource.OutTradeNo).Msg("marshal direct payment fact resource failed")
		return nil
	}
	return raw
}

func directRefundFactResource(resource *wechatcontracts.DirectRefundNotificationResource) []byte {
	if resource == nil {
		return nil
	}
	raw, err := json.Marshal(map[string]any{
		"mchid":          resource.MchID,
		"out_trade_no":   resource.OutTradeNo,
		"transaction_id": resource.TransactionID,
		"out_refund_no":  resource.OutRefundNo,
		"refund_id":      resource.RefundID,
		"refund_status":  resource.RefundStatus,
		"amount_total":   resource.Amount.Total,
		"amount_refund":  resource.Amount.Refund,
		"success_time":   resource.SuccessTime,
	})
	if err != nil {
		log.Warn().Err(err).Str("out_refund_no", resource.OutRefundNo).Msg("marshal direct refund fact resource failed")
		return nil
	}
	return raw
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

func (server *Server) resolveReservationRefundFactObjects(ctx context.Context, outRefundNo string) (db.RefundOrder, db.PaymentOrder, bool, error) {
	if server.paymentFactService == nil {
		return db.RefundOrder{}, db.PaymentOrder{}, false, nil
	}
	refundOrder, err := server.store.GetRefundOrderByOutRefundNo(ctx, outRefundNo)
	if err != nil {
		if !isNotFoundError(err) {
			log.Warn().Err(err).Str("out_refund_no", outRefundNo).Msg("skip reservation refund fact because local refund order resolution failed")
		}
		return db.RefundOrder{}, db.PaymentOrder{}, false, nil
	}
	paymentOrder, err := server.store.GetPaymentOrder(ctx, refundOrder.PaymentOrderID)
	if err != nil {
		log.Warn().Err(err).
			Int64("refund_order_id", refundOrder.ID).
			Int64("payment_order_id", refundOrder.PaymentOrderID).
			Str("out_refund_no", outRefundNo).
			Msg("skip reservation refund fact because local payment order resolution failed")
		return db.RefundOrder{}, db.PaymentOrder{}, false, nil
	}
	shouldRecord := (paymentOrder.PaymentChannel == db.PaymentChannelEcommerce || paymentOrder.PaymentChannel == db.PaymentChannelOrdinaryServiceProvider) && paymentOrder.ReservationID.Valid &&
		(paymentOrder.BusinessType == db.ExternalPaymentBusinessOwnerReservation || paymentOrder.BusinessType == "reservation_addon")
	return refundOrder, paymentOrder, shouldRecord, nil
}

func (server *Server) resolveOrderRefundFactObjects(ctx context.Context, outRefundNo string) (db.RefundOrder, db.PaymentOrder, bool, error) {
	if server.paymentFactService == nil {
		return db.RefundOrder{}, db.PaymentOrder{}, false, nil
	}
	refundOrder, err := server.store.GetRefundOrderByOutRefundNo(ctx, outRefundNo)
	if err != nil {
		if isNotFoundError(err) {
			return db.RefundOrder{}, db.PaymentOrder{}, false, nil
		}
		log.Warn().Err(err).Str("out_refund_no", outRefundNo).Msg("resolve order refund fact failed because local refund order lookup failed")
		return db.RefundOrder{}, db.PaymentOrder{}, false, err
	}
	paymentOrder, err := server.store.GetPaymentOrder(ctx, refundOrder.PaymentOrderID)
	if err != nil {
		log.Warn().Err(err).
			Int64("refund_order_id", refundOrder.ID).
			Int64("payment_order_id", refundOrder.PaymentOrderID).
			Str("out_refund_no", outRefundNo).
			Msg("resolve order refund fact failed because local payment order lookup failed")
		return db.RefundOrder{}, db.PaymentOrder{}, false, err
	}
	shouldRecord := (paymentOrder.PaymentChannel == db.PaymentChannelEcommerce || paymentOrder.PaymentChannel == db.PaymentChannelOrdinaryServiceProvider) && paymentOrder.OrderID.Valid && !paymentOrder.ReservationID.Valid && paymentOrder.BusinessType == db.ExternalPaymentBusinessOwnerOrder
	return refundOrder, paymentOrder, shouldRecord, nil
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

func (server *Server) enqueueRiderDepositPaymentFactApplication(ctx context.Context, application *db.ExternalPaymentFactApplication) {
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
			Int64("payment_order_id", application.BusinessObjectID).
			Msg("enqueue rider deposit payment fact application from callback failed; scheduler will retry")
	}
}

func (server *Server) enqueueClaimRecoveryPaymentFactApplication(ctx context.Context, application *db.ExternalPaymentFactApplication) {
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
			Int64("payment_order_id", application.BusinessObjectID).
			Msg("enqueue claim recovery payment fact application from callback failed; scheduler will retry")
	}
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

func (server *Server) recordReservationEcommerceRefundCallbackFact(ctx context.Context, notification wechat.PaymentNotification, refundOrder db.RefundOrder, paymentOrder db.PaymentOrder, resource *wechat.EcommerceRefundNotification) (*db.ExternalPaymentFactApplication, error) {
	if server.paymentFactService == nil || resource == nil || !paymentOrder.ReservationID.Valid {
		return nil, nil
	}
	status := logic.NormalizeEcommerceRefundTerminalStatus(resource.RefundStatus)
	result, err := server.paymentFactService.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelEcommerce,
		Capability:           db.ExternalPaymentCapabilityEcommerceRefund,
		FactSource:           db.ExternalPaymentFactSourceCallback,
		SourceEventID:        paymentFactStringPtr(notification.ID),
		SourceEventType:      paymentFactStringPtr(notification.EventType),
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalObjectKey:    resource.OutRefundNo,
		ExternalSecondaryKey: paymentFactStringPtr(resource.RefundID),
		BusinessOwner:        paymentFactStringPtr(db.ExternalPaymentBusinessOwnerReservation),
		BusinessObjectType:   paymentFactStringPtr(paymentFactBusinessObjectRefundOrder),
		BusinessObjectID:     paymentFactInt64Ptr(refundOrder.ID),
		UpstreamState:        resource.RefundStatus,
		TerminalStatus:       status,
		Amount:               paymentFactInt64Ptr(resource.Amount.Refund),
		Currency:             "CNY",
		RawResource:          ecommerceRefundFactResource(resource),
		DedupeKey:            fmt.Sprintf("wechat:callback:ecommerce_refund:%s", notification.ID),
		Application: &logic.ExternalPaymentFactApplicationTarget{
			Consumer:           paymentFactConsumerReservationDomain,
			BusinessObjectType: paymentFactBusinessObjectRefundOrder,
			BusinessObjectID:   refundOrder.ID,
		},
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}

func (server *Server) recordOrderEcommerceRefundCallbackFact(ctx context.Context, notification wechat.PaymentNotification, refundOrder db.RefundOrder, paymentOrder db.PaymentOrder, resource *wechat.EcommerceRefundNotification) (*db.ExternalPaymentFactApplication, error) {
	if server.paymentFactService == nil || resource == nil || !paymentOrder.OrderID.Valid || paymentOrder.ReservationID.Valid {
		return nil, nil
	}
	status := logic.NormalizeEcommerceRefundTerminalStatus(resource.RefundStatus)
	result, err := server.paymentFactService.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelEcommerce,
		Capability:           db.ExternalPaymentCapabilityEcommerceRefund,
		FactSource:           db.ExternalPaymentFactSourceCallback,
		SourceEventID:        paymentFactStringPtr(notification.ID),
		SourceEventType:      paymentFactStringPtr(notification.EventType),
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalObjectKey:    resource.OutRefundNo,
		ExternalSecondaryKey: paymentFactStringPtr(resource.RefundID),
		BusinessOwner:        paymentFactStringPtr(db.ExternalPaymentBusinessOwnerOrder),
		BusinessObjectType:   paymentFactStringPtr(paymentFactBusinessObjectRefundOrder),
		BusinessObjectID:     paymentFactInt64Ptr(refundOrder.ID),
		UpstreamState:        resource.RefundStatus,
		TerminalStatus:       status,
		Amount:               paymentFactInt64Ptr(resource.Amount.Refund),
		Currency:             "CNY",
		RawResource:          ecommerceRefundFactResource(resource),
		DedupeKey:            fmt.Sprintf("wechat:callback:ecommerce_refund:%s", notification.ID),
		Application: &logic.ExternalPaymentFactApplicationTarget{
			Consumer:           paymentFactConsumerOrderDomain,
			BusinessObjectType: paymentFactBusinessObjectRefundOrder,
			BusinessObjectID:   refundOrder.ID,
		},
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}

func ecommerceRefundFactResource(resource *wechat.EcommerceRefundNotification) []byte {
	if resource == nil {
		return nil
	}
	raw, err := json.Marshal(map[string]any{
		"sp_mchid":       resource.SpMchID,
		"sub_mchid":      resource.SubMchID,
		"out_trade_no":   resource.OutTradeNo,
		"transaction_id": resource.TransactionID,
		"out_refund_no":  resource.OutRefundNo,
		"refund_id":      resource.RefundID,
		"refund_status":  resource.RefundStatus,
		"amount_refund":  resource.Amount.Refund,
	})
	if err != nil {
		log.Warn().Err(err).Str("out_refund_no", resource.OutRefundNo).Msg("marshal ecommerce refund fact resource failed")
		return nil
	}
	return raw
}

func ordinaryRefundResourceFromEnvelope(envelope *ordinaryserviceprovider.NotificationEnvelope) (*ospcontracts.RefundNotificationPayload, error) {
	if envelope == nil {
		return nil, errors.New("ordinary refund notification envelope is nil")
	}
	data, err := ordinaryNotificationResourceBytes(envelope)
	if err != nil {
		return nil, err
	}
	resource := &ospcontracts.RefundNotificationPayload{}
	if err := json.Unmarshal(data, resource); err != nil {
		return nil, fmt.Errorf("decode ordinary refund notification resource: %w", err)
	}
	resource.SpMchID = strings.TrimSpace(resource.SpMchID)
	resource.SubMchID = strings.TrimSpace(resource.SubMchID)
	resource.OutTradeNo = strings.TrimSpace(resource.OutTradeNo)
	resource.TransactionID = strings.TrimSpace(resource.TransactionID)
	resource.OutRefundNo = strings.TrimSpace(resource.OutRefundNo)
	resource.RefundID = strings.TrimSpace(resource.RefundID)
	resource.RefundStatus = ospcontracts.RefundStatus(strings.TrimSpace(string(resource.RefundStatus)))
	if err := ospcontracts.ValidateRefundNotificationPayload(*resource); err != nil {
		return nil, err
	}
	return resource, nil
}

func ordinaryRefundStatus(resource *ospcontracts.RefundNotificationPayload) string {
	if resource == nil {
		return ""
	}
	return strings.TrimSpace(string(resource.RefundStatus))
}

func ordinaryRefundAmount(resource *ospcontracts.RefundNotificationPayload) int64 {
	if resource == nil || resource.Amount == nil {
		return 0
	}
	return resource.Amount.Refund
}

func (server *Server) recordReservationOrdinaryRefundCallbackFact(ctx context.Context, notification wechat.PaymentNotification, refundOrder db.RefundOrder, paymentOrder db.PaymentOrder, resource *ospcontracts.RefundNotificationPayload) (*db.ExternalPaymentFactApplication, error) {
	if server.paymentFactService == nil || resource == nil || !paymentOrder.ReservationID.Valid {
		return nil, nil
	}
	refundStatus := ordinaryRefundStatus(resource)
	status := logic.NormalizeEcommerceRefundTerminalStatus(refundStatus)
	occurredAt := parseWechatFactTime(resource.SuccessTime)
	result, err := server.paymentFactService.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelOrdinaryServiceProvider,
		Capability:           db.ExternalPaymentCapabilityPartnerRefund,
		FactSource:           db.ExternalPaymentFactSourceCallback,
		SourceEventID:        paymentFactStringPtr(notification.ID),
		SourceEventType:      paymentFactStringPtr(notification.EventType),
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalObjectKey:    resource.OutRefundNo,
		ExternalSecondaryKey: paymentFactStringPtr(resource.RefundID),
		BusinessOwner:        paymentFactStringPtr(db.ExternalPaymentBusinessOwnerReservation),
		BusinessObjectType:   paymentFactStringPtr(paymentFactBusinessObjectRefundOrder),
		BusinessObjectID:     paymentFactInt64Ptr(refundOrder.ID),
		UpstreamState:        refundStatus,
		TerminalStatus:       status,
		Amount:               paymentFactInt64Ptr(ordinaryRefundAmount(resource)),
		Currency:             "CNY",
		OccurredAt:           occurredAt,
		UpstreamUpdatedAt:    occurredAt,
		RawResource:          ordinaryRefundFactResource(resource),
		DedupeKey:            fmt.Sprintf("wechat:callback:ordinary_service_provider_refund:%s", notification.ID),
		Application: &logic.ExternalPaymentFactApplicationTarget{
			Consumer:           paymentFactConsumerReservationDomain,
			BusinessObjectType: paymentFactBusinessObjectRefundOrder,
			BusinessObjectID:   refundOrder.ID,
		},
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}

func (server *Server) recordOrderOrdinaryRefundCallbackFact(ctx context.Context, notification wechat.PaymentNotification, refundOrder db.RefundOrder, paymentOrder db.PaymentOrder, resource *ospcontracts.RefundNotificationPayload) (*db.ExternalPaymentFactApplication, error) {
	if server.paymentFactService == nil || resource == nil || !paymentOrder.OrderID.Valid || paymentOrder.ReservationID.Valid {
		return nil, nil
	}
	refundStatus := ordinaryRefundStatus(resource)
	status := logic.NormalizeEcommerceRefundTerminalStatus(refundStatus)
	occurredAt := parseWechatFactTime(resource.SuccessTime)
	result, err := server.paymentFactService.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelOrdinaryServiceProvider,
		Capability:           db.ExternalPaymentCapabilityPartnerRefund,
		FactSource:           db.ExternalPaymentFactSourceCallback,
		SourceEventID:        paymentFactStringPtr(notification.ID),
		SourceEventType:      paymentFactStringPtr(notification.EventType),
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalObjectKey:    resource.OutRefundNo,
		ExternalSecondaryKey: paymentFactStringPtr(resource.RefundID),
		BusinessOwner:        paymentFactStringPtr(db.ExternalPaymentBusinessOwnerOrder),
		BusinessObjectType:   paymentFactStringPtr(paymentFactBusinessObjectRefundOrder),
		BusinessObjectID:     paymentFactInt64Ptr(refundOrder.ID),
		UpstreamState:        refundStatus,
		TerminalStatus:       status,
		Amount:               paymentFactInt64Ptr(ordinaryRefundAmount(resource)),
		Currency:             "CNY",
		OccurredAt:           occurredAt,
		UpstreamUpdatedAt:    occurredAt,
		RawResource:          ordinaryRefundFactResource(resource),
		DedupeKey:            fmt.Sprintf("wechat:callback:ordinary_service_provider_refund:%s", notification.ID),
		Application: &logic.ExternalPaymentFactApplicationTarget{
			Consumer:           paymentFactConsumerOrderDomain,
			BusinessObjectType: paymentFactBusinessObjectRefundOrder,
			BusinessObjectID:   refundOrder.ID,
		},
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}

func ordinaryRefundFactResource(resource *ospcontracts.RefundNotificationPayload) []byte {
	if resource == nil {
		return nil
	}
	raw, err := json.Marshal(map[string]any{
		"sp_mchid":       resource.SpMchID,
		"sub_mchid":      resource.SubMchID,
		"out_trade_no":   resource.OutTradeNo,
		"transaction_id": resource.TransactionID,
		"out_refund_no":  resource.OutRefundNo,
		"refund_id":      resource.RefundID,
		"refund_status":  ordinaryRefundStatus(resource),
		"amount_refund":  ordinaryRefundAmount(resource),
	})
	if err != nil {
		log.Warn().Err(err).Str("out_refund_no", resource.OutRefundNo).Msg("marshal ordinary refund fact resource failed")
		return nil
	}
	return raw
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

func partnerSubMchIDFromPaymentAttach(paymentOrder db.PaymentOrder) string {
	if !paymentOrder.Attach.Valid {
		return ""
	}
	for _, segment := range strings.Split(strings.TrimSpace(paymentOrder.Attach.String), ";") {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		pair := strings.SplitN(segment, ":", 2)
		if len(pair) != 2 {
			continue
		}
		if strings.TrimSpace(pair[0]) != "sub_mchid" {
			continue
		}
		return strings.TrimSpace(pair[1])
	}
	return ""
}

func (server *Server) resolvePaymentOrderSubMchID(ctx context.Context, paymentOrder db.PaymentOrder) (string, error) {
	if attachSubMchID := partnerSubMchIDFromPaymentAttach(paymentOrder); attachSubMchID != "" {
		return attachSubMchID, nil
	}

	var merchantID int64

	if paymentOrder.OrderID.Valid {
		order, err := server.store.GetOrder(ctx, paymentOrder.OrderID.Int64)
		if err != nil {
			return "", fmt.Errorf("get order for payment order %d: %w", paymentOrder.ID, err)
		}
		merchantID = order.MerchantID
	} else if paymentOrder.ReservationID.Valid {
		reservation, err := server.store.GetTableReservation(ctx, paymentOrder.ReservationID.Int64)
		if err != nil {
			return "", fmt.Errorf("get reservation for payment order %d: %w", paymentOrder.ID, err)
		}
		merchantID = reservation.MerchantID
	} else {
		return "", fmt.Errorf("payment order %d missing order and reservation reference", paymentOrder.ID)
	}

	config, err := server.store.GetMerchantPaymentConfig(ctx, merchantID)
	if err != nil {
		return "", fmt.Errorf("get merchant payment config for payment order %d: %w", paymentOrder.ID, err)
	}
	if config.SubMchID == "" {
		return "", errors.New("merchant payment config sub_mchid missing")
	}

	return config.SubMchID, nil
}

type withdrawStatusSyncSnapshot struct {
	OutRequestNo string
	SubMchID     string
	WithdrawID   string
	Status       string
	Reason       string
}

func (server *Server) syncWithdrawalRecordWithWechat(ctx *gin.Context, record db.WithdrawalRecord, resource *withdrawStatusSyncSnapshot) (db.WithdrawalRecord, error) {
	if resource == nil {
		return record, errors.New("withdraw resource is nil")
	}
	if resource.OutRequestNo == "" {
		return record, errors.New("withdraw out_request_no missing")
	}

	switch record.Channel {
	case merchantWithdrawChannel:
		info := parseMerchantWithdrawAccountInfo(record.AccountInfo)
		if info.OutRequestNo != "" && info.OutRequestNo != resource.OutRequestNo {
			return record, fmt.Errorf("merchant withdraw out_request_no mismatch")
		}
		if info.SubMchID != "" && resource.SubMchID != "" && info.SubMchID != resource.SubMchID {
			return record, fmt.Errorf("merchant withdraw sub_mchid mismatch")
		}
		if resource.WithdrawID != "" && info.WithdrawID != resource.WithdrawID {
			info.WithdrawID = resource.WithdrawID
			if info.OutRequestNo == "" {
				info.OutRequestNo = resource.OutRequestNo
			}
			if info.SubMchID == "" {
				info.SubMchID = resource.SubMchID
			}
			raw, err := json.Marshal(info)
			if err != nil {
				return record, fmt.Errorf("marshal merchant withdraw account info: %w", err)
			}
			record, err = server.persistWithdrawalRecordAccountInfo(ctx, record, raw)
			if err != nil {
				return record, fmt.Errorf("persist merchant withdraw account info: %w", err)
			}
		}
	default:
		return record, fmt.Errorf("unsupported withdrawal channel: %s", record.Channel)
	}

	return record, nil
}

func (server *Server) handlePartnerPaymentNotification(ctx *gin.Context, notification wechat.PaymentNotification, resource *wechatcontracts.PartnerPaymentNotificationResource) {
	server.handlePartnerPaymentNotificationWithOwnership(ctx, notification, resource, "ecommerce_payment", server.validatePartnerNotifyOwnership)
}

func (server *Server) handlePartnerPaymentNotificationWithOwnership(ctx *gin.Context, notification wechat.PaymentNotification, resource *wechatcontracts.PartnerPaymentNotificationResource, callbackLabel string, validateOwnership func(*wechatcontracts.PartnerPaymentNotificationResource) error) {
	if err := validateOwnership(resource); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "ownership").Inc()
		log.Error().Err(err).
			Str("out_trade_no", resource.OutTradeNo).
			Str("sp_mchid", resource.SpMchID).
			Str("sp_appid", resource.SpAppID).
			Msg("partner payment ownership validation failed")
		server.sendAlert(websocket.AlertData{
			AlertType:   websocket.AlertTypeSystemError,
			Level:       websocket.AlertLevelCritical,
			Title:       "收付通单笔支付回调归属校验失败",
			Message:     fmt.Sprintf("收付通单笔回调 out_trade_no=%s 的归属校验失败，sp_mchid=%s, sp_appid=%s。系统已返回 FAIL 等待微信重试，请排查是否存在错服务商或错应用回调。", resource.OutTradeNo, resource.SpMchID, resource.SpAppID),
			RelatedID:   0,
			RelatedType: "payment_order",
			Extra: map[string]interface{}{
				"out_trade_no":   resource.OutTradeNo,
				"transaction_id": resource.TransactionID,
				"sp_mchid":       resource.SpMchID,
				"sp_appid":       resource.SpAppID,
			},
		})
		server.releaseNotification(ctx, notification.ID, callbackLabel)
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "ownership validation failed"})
		return
	}

	log.Info().
		Str("notification_id", notification.ID).
		Str("out_trade_no", resource.OutTradeNo).
		Str("transaction_id", resource.TransactionID).
		Int64("amount", resource.Amount.Total).
		Msg("received partner payment notification")

	paymentOrder, err := server.store.GetPaymentOrderByOutTradeNo(ctx, resource.OutTradeNo)
	if err != nil {
		if isNotFoundError(err) {
			paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "payment_order_not_found").Inc()
			log.Error().Str("out_trade_no", resource.OutTradeNo).Msg("partner payment order not found")
			server.sendAlert(websocket.AlertData{
				AlertType:   websocket.AlertTypeSystemError,
				Level:       websocket.AlertLevelCritical,
				Title:       "收付通单笔回调未找到本地支付单",
				Message:     fmt.Sprintf("收付通单笔回调 out_trade_no=%s, transaction_id=%s 未找到本地 payment_order。系统已返回 FAIL 等待重试，请尽快排查支付单创建与回调时序。", resource.OutTradeNo, resource.TransactionID),
				RelatedID:   0,
				RelatedType: "payment_order",
				Extra: map[string]interface{}{
					"out_trade_no":    resource.OutTradeNo,
					"transaction_id":  resource.TransactionID,
					"business_action": "wechat_retry_expected",
				},
			})
			server.releaseNotification(ctx, notification.ID, callbackLabel)
			ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "payment order not found, please retry"})
			return
		}
		log.Error().Err(err).Str("out_trade_no", resource.OutTradeNo).Msg("get partner payment order")
		paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "query_payment_order").Inc()
		server.releaseNotification(ctx, notification.ID, callbackLabel)
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "internal error"})
		return
	}

	expectedSubMchID, err := server.resolvePaymentOrderSubMchID(ctx, paymentOrder)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "resolve_sub_mchid").Inc()
		log.Error().Err(err).Int64("payment_order_id", paymentOrder.ID).Msg("resolve partner payment sub_mchid failed")
		server.releaseNotification(ctx, notification.ID, callbackLabel)
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "sub merchant resolution failed"})
		return
	}
	if resource.SubMchID != "" && resource.SubMchID != expectedSubMchID {
		paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "sub_mchid_ownership").Inc()
		log.Error().
			Int64("payment_order_id", paymentOrder.ID).
			Str("expected_sub_mchid", expectedSubMchID).
			Str("actual_sub_mchid", resource.SubMchID).
			Msg("partner payment sub_mchid ownership validation failed")
		server.releaseNotification(ctx, notification.ID, callbackLabel)
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "ownership validation failed"})
		return
	}

	if paymentOrder.Status == PaymentStatusPaid {
		if !paymentOrder.ProcessedAt.Valid {
			if paymentOrder.BusinessType == db.ExternalPaymentBusinessOwnerOrder && paymentOrderUsesMainBusinessPaymentChannel(paymentOrder) {
				application, factErr := server.recordOrderPaymentCallbackFactByChannel(ctx, notification, paymentOrder, resource)
				if factErr != nil {
					paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "record_order_payment_fact").Inc()
					log.Error().Err(factErr).
						Int64("payment_order_id", paymentOrder.ID).
						Str("out_trade_no", paymentOrder.OutTradeNo).
						Msg("record order payment callback fact failed")
					server.releaseNotification(ctx, notification.ID, callbackLabel)
					ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "record payment fact failed, please retry"})
					return
				}
				if err := server.enqueueOrderPaymentFactApplication(ctx, application); err != nil {
					paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "reenqueue_order_payment_fact_application").Inc()
					log.Error().Err(err).
						Int64("payment_order_id", paymentOrder.ID).
						Str("out_trade_no", paymentOrder.OutTradeNo).
						Msg("partner payment order already paid but order payment fact application enqueue failed")
					server.releaseNotification(ctx, notification.ID, callbackLabel)
					ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "payment processing enqueue failed, please retry"})
					return
				}
			} else if shouldRecordReservationPaymentFact(paymentOrder) {
				application, factErr := server.recordReservationPaymentCallbackFactByChannel(ctx, notification, paymentOrder, resource)
				if factErr != nil {
					paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "record_reservation_payment_fact").Inc()
					log.Error().Err(factErr).
						Int64("payment_order_id", paymentOrder.ID).
						Str("out_trade_no", paymentOrder.OutTradeNo).
						Msg("record reservation payment callback fact failed")
					server.releaseNotification(ctx, notification.ID, callbackLabel)
					ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "record payment fact failed, please retry"})
					return
				}
				if err := server.enqueueReservationPaymentFactApplication(ctx, application); err != nil {
					paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "reenqueue_reservation_payment_fact_application").Inc()
					log.Error().Err(err).
						Int64("payment_order_id", paymentOrder.ID).
						Str("out_trade_no", paymentOrder.OutTradeNo).
						Msg("partner payment reservation already paid but reservation payment fact application enqueue failed")
					server.releaseNotification(ctx, notification.ID, callbackLabel)
					ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "payment processing enqueue failed, please retry"})
					return
				}
			} else {
				paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "unsupported_payment_fact_owner").Inc()
				log.Error().
					Int64("payment_order_id", paymentOrder.ID).
					Str("out_trade_no", paymentOrder.OutTradeNo).
					Str("business_type", paymentOrder.BusinessType).
					Str("payment_channel", paymentOrder.PaymentChannel).
					Msg("partner payment order already paid but no payment fact application owner matched")
				server.releaseNotification(ctx, notification.ID, callbackLabel)
				ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "unsupported payment business type, please retry"})
				return
			}
		}
		server.markNotificationProcessed(ctx, notification.ID, resource.OutTradeNo, resource.TransactionID)
		writeWechatNotifySuccess(ctx, callbackLabel)
		return
	}

	if paymentOrder.Status == PaymentStatusClosed || paymentOrder.Status == PaymentStatusFailed {
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
				log.Error().Err(enqErr).Int64("payment_order_id", paymentOrder.ID).Msg("failed to enqueue anomaly refund task for partner payment")
				server.sendAlert(websocket.AlertData{
					AlertType: websocket.AlertTypePaymentAmountMismatch,
					Level:     websocket.AlertLevelCritical,
					Title:     "⚠️ 已关闭收付通单笔支付退款任务入队失败 — 需立即人工退款",
					Message: fmt.Sprintf(
						"支付单 %s（ID=%d）处于 %s 状态但微信仍到账 %d 分，自动退款任务入队失败（%v）。交易号: %s。请立即在微信商户平台手动退款 %d 分。",
						resource.OutTradeNo, paymentOrder.ID, paymentOrder.Status, resource.Amount.Total, enqErr, resource.TransactionID, resource.Amount.Total,
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
					Title:     "⚠️ 已关闭收付通单笔支付收到微信付款 — 系统自动退款已触发",
					Message: fmt.Sprintf(
						"支付单 %s（ID=%d）处于 %s 状态但微信到账 %d 分。系统已触发自动退款任务（退款单号: %s），请关注退款结果。",
						resource.OutTradeNo, paymentOrder.ID, paymentOrder.Status, resource.Amount.Total, outRefundNo,
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
				Title:     "⚠️ 已关闭收付通单笔支付收到微信付款 — 需人工退款",
				Message: fmt.Sprintf(
					"支付单 %s（ID=%d）已处于 %s 状态，但微信仍到账 %d 分。任务分发器未配置，无法自动退款。交易号: %s。请在微信商户平台对该交易发起退款，退款金额 %d 分。",
					resource.OutTradeNo, paymentOrder.ID, paymentOrder.Status, resource.Amount.Total, resource.TransactionID, resource.Amount.Total,
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
		server.markNotificationProcessed(ctx, notification.ID, resource.OutTradeNo, resource.TransactionID)
		writeWechatNotifySuccess(ctx, callbackLabel)
		return
	}

	if resource.Amount.Total != paymentOrder.Amount {
		refundReason := "金额异常，系统自动退款"
		refundOrder, outRefundNo, refundRecordErr := server.ensureAmountMismatchRefundRecord(ctx, paymentOrder, resource.Amount.Total, refundReason)
		if refundRecordErr != nil {
			paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "create_mismatch_refund_record").Inc()
			log.Error().Err(refundRecordErr).Int64("payment_order_id", paymentOrder.ID).Msg("create partner mismatch refund record failed")
			server.releaseNotification(ctx, notification.ID, callbackLabel)
			ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "internal error"})
			return
		}
		if _, updateErr := server.store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
			ID:            paymentOrder.ID,
			TransactionID: pgtype.Text{String: resource.TransactionID, Valid: true},
		}); updateErr != nil {
			log.Error().Err(updateErr).Int64("id", paymentOrder.ID).Msg("update partner payment order to paid for mismatch refund failed")
			server.releaseNotification(ctx, notification.ID, callbackLabel)
			ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "internal error"})
			return
		}

		refundPayload, canAutoRefund := buildAmountMismatchRefundPayload(paymentOrder, resource.Amount.Total, refundReason)
		refundEnqueued := false
		if server.taskDistributor != nil && canAutoRefund {
			if enqErr := server.taskDistributor.DistributeTaskProcessRefund(ctx, refundPayload, asynq.MaxRetry(5), asynq.Queue(worker.QueueCritical)); enqErr != nil {
				paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "enqueue_mismatch_refund").Inc()
				server.markRefundOrderFailed(ctx, refundOrder.ID)
				log.Error().Err(enqErr).Int64("payment_order_id", paymentOrder.ID).Msg("partner mismatch refund task enqueue failed")
				server.sendAlert(websocket.AlertData{
					AlertType:   websocket.AlertTypeTaskEnqueueFailure,
					Level:       websocket.AlertLevelCritical,
					Title:       "收付通单笔金额异常退款任务入队失败",
					Message:     fmt.Sprintf("收付通单笔支付单 %s 金额异常后退款任务入队失败，支付单 %d 需要人工处理", resource.OutTradeNo, paymentOrder.ID),
					RelatedID:   paymentOrder.ID,
					RelatedType: "payment_order",
					Extra: map[string]interface{}{
						"out_trade_no":    resource.OutTradeNo,
						"transaction_id":  resource.TransactionID,
						"out_refund_no":   outRefundNo,
						"expected_amount": paymentOrder.Amount,
						"actual_amount":   resource.Amount.Total,
						"error":           enqErr.Error(),
					},
				})
			} else {
				refundEnqueued = true
				server.sendAlert(websocket.AlertData{
					AlertType:   websocket.AlertTypePaymentAmountMismatch,
					Level:       websocket.AlertLevelCritical,
					Title:       "⚠️ 收付通单笔支付金额异常 — 自动退款已触发",
					Message:     fmt.Sprintf("收付通单笔支付单 %s（ID=%d）实收 %d 分与预期 %d 分不符，交易号 %s。系统已自动触发退款，请确认退款结果。", resource.OutTradeNo, paymentOrder.ID, resource.Amount.Total, paymentOrder.Amount, resource.TransactionID),
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
			server.sendAlert(websocket.AlertData{
				AlertType:   websocket.AlertTypePaymentAmountMismatch,
				Level:       websocket.AlertLevelCritical,
				Title:       "⚠️ 收付通单笔支付金额异常 — 无法自动退款，需立即人工处理",
				Message:     fmt.Sprintf("收付通单笔支付单 %s（ID=%d）实收 %d 分与预期 %d 分不符，交易号 %s。系统无法自动触发退款（distributor=%v, can_auto_refund=%v），请在微信商户平台手动退款。", resource.OutTradeNo, paymentOrder.ID, resource.Amount.Total, paymentOrder.Amount, resource.TransactionID, server.taskDistributor != nil, canAutoRefund),
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
		_ = refundEnqueued
		server.markNotificationProcessed(ctx, notification.ID, resource.OutTradeNo, resource.TransactionID)
		writeWechatNotifySuccess(ctx, callbackLabel)
		return
	}

	updatedPaymentOrder, err := server.store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            paymentOrder.ID,
		TransactionID: pgtype.Text{String: resource.TransactionID, Valid: true},
	})
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "update_payment_order").Inc()
		log.Error().Err(err).Int64("id", paymentOrder.ID).Msg("update partner payment order to paid")
		server.releaseNotification(ctx, notification.ID, callbackLabel)
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "update order failed"})
		return
	}

	server.sendPaymentSuccessNotification(ctx, updatedPaymentOrder, resource.TransactionID, callbackLabel)

	if updatedPaymentOrder.BusinessType == db.ExternalPaymentBusinessOwnerOrder && paymentOrderUsesMainBusinessPaymentChannel(updatedPaymentOrder) {
		application, factErr := server.recordOrderPaymentCallbackFactByChannel(ctx, notification, updatedPaymentOrder, resource)
		if factErr != nil {
			paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "record_order_payment_fact").Inc()
			log.Error().Err(factErr).
				Int64("payment_order_id", updatedPaymentOrder.ID).
				Str("out_trade_no", updatedPaymentOrder.OutTradeNo).
				Msg("record order payment callback fact failed")
			server.releaseNotification(ctx, notification.ID, callbackLabel)
			ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "record payment fact failed, please retry"})
			return
		}
		if err := server.enqueueOrderPaymentFactApplication(ctx, application); err != nil {
			paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "enqueue_order_payment_fact_application").Inc()
			log.Error().Err(err).
				Int64("payment_order_id", updatedPaymentOrder.ID).
				Str("out_trade_no", updatedPaymentOrder.OutTradeNo).
				Msg("partner order payment fact application enqueue failed")
			server.releaseNotification(ctx, notification.ID, callbackLabel)
			ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "payment processing enqueue failed, please retry"})
			return
		}
	} else if shouldRecordReservationPaymentFact(updatedPaymentOrder) {
		application, factErr := server.recordReservationPaymentCallbackFactByChannel(ctx, notification, updatedPaymentOrder, resource)
		if factErr != nil {
			paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "record_reservation_payment_fact").Inc()
			log.Error().Err(factErr).
				Int64("payment_order_id", updatedPaymentOrder.ID).
				Str("out_trade_no", updatedPaymentOrder.OutTradeNo).
				Msg("record reservation payment callback fact failed")
			server.releaseNotification(ctx, notification.ID, callbackLabel)
			ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "record payment fact failed, please retry"})
			return
		}
		if err := server.enqueueReservationPaymentFactApplication(ctx, application); err != nil {
			paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "enqueue_reservation_payment_fact_application").Inc()
			log.Error().Err(err).
				Int64("payment_order_id", updatedPaymentOrder.ID).
				Str("out_trade_no", updatedPaymentOrder.OutTradeNo).
				Msg("partner reservation payment fact application enqueue failed")
			server.releaseNotification(ctx, notification.ID, callbackLabel)
			ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "payment processing enqueue failed, please retry"})
			return
		}
	} else {
		paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "unsupported_payment_fact_owner").Inc()
		log.Error().
			Int64("payment_order_id", paymentOrder.ID).
			Str("out_trade_no", paymentOrder.OutTradeNo).
			Str("business_type", paymentOrder.BusinessType).
			Str("payment_channel", paymentOrder.PaymentChannel).
			Msg("partner payment callback paid transition has no payment fact application owner")
		server.releaseNotification(ctx, notification.ID, callbackLabel)
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "unsupported payment business type, please retry"})
		return
	}

	server.markNotificationProcessed(ctx, notification.ID, resource.OutTradeNo, resource.TransactionID)
	writeWechatNotifySuccess(ctx, callbackLabel)
}

// handleEcommerceWithdrawNotify 处理平台收付通提现状态变更通知
// POST /v1/webhooks/wechat-ecommerce/withdraw-notify
func (server *Server) handleEcommerceWithdrawNotify(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		log.Error().Msg("ecommerce withdraw callback received but ecommerce client is not configured")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "ecommerce client not configured",
		})
		return
	}

	body, status, err := readWebhookBody(ctx)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("ecommerce_withdraw", "read_body").Inc()
		log.Error().Err(err).Msg("read ecommerce withdraw notification body")
		ctx.JSON(status, wechatPaymentNotifyResponse{Code: "FAIL", Message: "read body failed"})
		return
	}

	signature := ctx.GetHeader("Wechatpay-Signature")
	timestamp := ctx.GetHeader("Wechatpay-Timestamp")
	nonce := ctx.GetHeader("Wechatpay-Nonce")
	serial := ctx.GetHeader("Wechatpay-Serial")

	if err := server.ecommerceClient.VerifyNotificationSignature(signature, timestamp, nonce, serial, string(body)); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("ecommerce_withdraw", "signature").Inc()
		log.Error().Err(err).Msg("invalid wechat signature for ecommerce withdraw notification")
		ctx.JSON(http.StatusUnauthorized, wechatPaymentNotifyResponse{Code: "FAIL", Message: "signature verification failed"})
		return
	}

	var notification wechat.PaymentNotification
	if err := json.Unmarshal(body, &notification); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("ecommerce_withdraw", "parse").Inc()
		log.Error().Err(err).Msg("parse ecommerce withdraw notification")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{Code: "FAIL", Message: "parse notification failed"})
		return
	}

	if notification.EventType != wechatcontracts.FundManagementNotificationEventType {
		log.Info().Str("event_type", notification.EventType).Msg("ignore non-withdraw ecommerce notification")
		writeWechatNotifySuccess(ctx, "ecommerce_withdraw")
		return
	}

	if !server.tryClaimNotification(ctx, notification, "ecommerce_withdraw") {
		return
	}

	decrypted, err := server.ecommerceClient.DecryptNotificationRaw(&notification)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("ecommerce_withdraw", "decrypt").Inc()
		log.Error().Err(err).Msg("decrypt ecommerce withdraw notification")
		server.releaseNotification(ctx, notification.ID, "ecommerce_withdraw")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{Code: "FAIL", Message: "decrypt failed"})
		return
	}

	var resource wechatcontracts.WithdrawNotificationResource
	if err := json.Unmarshal(decrypted, &resource); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("ecommerce_withdraw", "parse_resource").Inc()
		log.Error().Err(err).Str("decrypted", string(decrypted)).Msg("parse ecommerce withdraw resource")
		server.releaseNotification(ctx, notification.ID, "ecommerce_withdraw")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{Code: "FAIL", Message: "parse resource failed"})
		return
	}
	if err := wechatcontracts.ValidateWithdrawNotificationResource("decrypt ecommerce withdraw notification", &resource); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("ecommerce_withdraw", "contract_validation").Inc()
		log.Error().Err(err).Str("notification_id", notification.ID).Msg("validate ecommerce withdraw resource contract")
		server.releaseNotification(ctx, notification.ID, "ecommerce_withdraw")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{Code: "FAIL", Message: "invalid resource contract"})
		return
	}
	if resource.OutRequestNo == "" {
		paymentCallbackFailuresTotal.WithLabelValues("ecommerce_withdraw", "missing_out_request_no").Inc()
		log.Error().Str("notification_id", notification.ID).Msg("ecommerce withdraw resource missing out_request_no")
		server.releaseNotification(ctx, notification.ID, "ecommerce_withdraw")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{Code: "FAIL", Message: "missing out_request_no"})
		return
	}

	record, err := server.store.GetWithdrawalRecordByOutRequestNo(ctx, pgtype.Text{String: resource.OutRequestNo, Valid: true})
	if err != nil {
		if isNotFoundError(err) {
			paymentCallbackFailuresTotal.WithLabelValues("ecommerce_withdraw", "withdrawal_not_found").Inc()
			log.Error().Str("out_request_no", resource.OutRequestNo).Msg("withdrawal record not found for ecommerce withdraw callback")
			server.releaseNotification(ctx, notification.ID, "ecommerce_withdraw")
			ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "withdrawal record not found, please retry"})
			return
		}
		paymentCallbackFailuresTotal.WithLabelValues("ecommerce_withdraw", "query_withdrawal").Inc()
		log.Error().Err(err).Str("out_request_no", resource.OutRequestNo).Msg("query withdrawal record by out_request_no failed")
		server.releaseNotification(ctx, notification.ID, "ecommerce_withdraw")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "internal error"})
		return
	}

	application, err := server.recordMerchantWithdrawCallbackFact(ctx, notification, record, &resource)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("ecommerce_withdraw", "record_fact").Inc()
		log.Error().Err(err).Int64("withdrawal_record_id", record.ID).Str("out_request_no", resource.OutRequestNo).Msg("record merchant withdraw callback fact failed")
		server.releaseNotification(ctx, notification.ID, "ecommerce_withdraw")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "withdrawal fact record failed"})
		return
	}

	if _, err := server.syncWithdrawalRecordWithWechat(ctx, record, &withdrawStatusSyncSnapshot{
		OutRequestNo: resource.OutRequestNo,
		SubMchID:     resource.SubMchID,
		WithdrawID:   resource.WithdrawID,
		Status:       resource.Status,
		Reason:       resource.Reason,
	}); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("ecommerce_withdraw", "sync_withdrawal").Inc()
		log.Error().Err(err).Int64("withdrawal_record_id", record.ID).Str("out_request_no", resource.OutRequestNo).Msg("sync withdrawal record with ecommerce callback failed")
		server.releaseNotification(ctx, notification.ID, "ecommerce_withdraw")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "withdrawal sync failed"})
		return
	}

	if _, _, err := server.applyMerchantWithdrawFactApplication(ctx, application); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("ecommerce_withdraw", "apply_fact").Inc()
		log.Error().Err(err).Int64("withdrawal_record_id", record.ID).Str("out_request_no", resource.OutRequestNo).Msg("apply merchant withdraw callback fact failed")
		server.releaseNotification(ctx, notification.ID, "ecommerce_withdraw")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "withdrawal sync failed"})
		return
	}

	writeWechatNotifySuccess(ctx, "ecommerce_withdraw")
	server.markNotificationProcessed(ctx, notification.ID, resource.OutRequestNo, resource.WithdrawID)
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

	// 检查是否已处理（幂等）
	if paymentOrder.Status == PaymentStatusPaid {
		if !paymentOrder.ProcessedAt.Valid {
			if paymentOrder.BusinessType == db.ExternalPaymentBusinessOwnerRiderDeposit || paymentOrder.BusinessType == db.ExternalPaymentBusinessOwnerClaimRecovery {
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
				if paymentOrder.BusinessType == db.ExternalPaymentBusinessOwnerRiderDeposit {
					server.enqueueRiderDepositPaymentFactApplication(ctx, paymentFactApplication)
				} else {
					server.enqueueClaimRecoveryPaymentFactApplication(ctx, paymentFactApplication)
				}
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

	if updatedPaymentOrder.BusinessType == db.ExternalPaymentBusinessOwnerRiderDeposit || updatedPaymentOrder.BusinessType == db.ExternalPaymentBusinessOwnerClaimRecovery {
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
		if updatedPaymentOrder.BusinessType == db.ExternalPaymentBusinessOwnerRiderDeposit {
			server.enqueueRiderDepositPaymentFactApplication(ctx, paymentFactApplication)
		} else {
			server.enqueueClaimRecoveryPaymentFactApplication(ctx, paymentFactApplication)
		}
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
		writeWechatNotifySuccess(ctx, "ecommerce_refund")
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
	writeWechatNotifySuccess(ctx, "ecommerce_refund")
	server.markNotificationProcessed(ctx, notification.ID, resource.OutTradeNo, resource.TransactionID)
}

// handleEcommerceRefundNotify 处理平台收付通退款回调通知
// POST /v1/webhooks/wechat-ecommerce/refund-notify
func (server *Server) handleEcommerceRefundNotify(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		log.Error().Msg("ecommerce refund callback received but ecommerce client is not configured")
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
	serial := ctx.GetHeader("Wechatpay-Serial")

	if err := server.ecommerceClient.VerifyNotificationSignature(signature, timestamp, nonce, serial, string(body)); err != nil {
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
		writeWechatNotifySuccess(ctx, "ecommerce_refund")
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

	if !server.requireTaskDistributorForNotification(ctx, notification.ID, "ecommerce_refund", "refund result processing unavailable, please retry") {
		return
	}

	handledByFact := false
	refundOrder, paymentOrder, shouldRecordFact, err := server.resolveReservationRefundFactObjects(ctx, resource.OutRefundNo)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("ecommerce_refund", "resolve_refund_fact_object").Inc()
		log.Error().Err(err).
			Str("out_refund_no", resource.OutRefundNo).
			Msg("resolve reservation refund fact object failed")
		server.releaseNotification(ctx, notification.ID, "ecommerce_refund")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "refund fact object resolution failed, please retry"})
		return
	}
	if shouldRecordFact {
		handledByFact = true
		application, err := server.recordReservationEcommerceRefundCallbackFact(ctx, notification, refundOrder, paymentOrder, resource)
		if err != nil {
			paymentCallbackFailuresTotal.WithLabelValues("ecommerce_refund", "record_refund_fact").Inc()
			log.Error().Err(err).
				Int64("refund_order_id", refundOrder.ID).
				Str("out_refund_no", resource.OutRefundNo).
				Msg("record reservation ecommerce refund fact failed")
			server.releaseNotification(ctx, notification.ID, "ecommerce_refund")
			ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "refund fact recording failed, please retry"})
			return
		}
		server.enqueueReservationRefundPaymentFactApplication(ctx, application)
	}

	if !handledByFact {
		refundOrder, paymentOrder, shouldRecordFact, err = server.resolveOrderRefundFactObjects(ctx, resource.OutRefundNo)
		if err != nil {
			paymentCallbackFailuresTotal.WithLabelValues("ecommerce_refund", "resolve_order_refund_fact_object").Inc()
			log.Error().Err(err).
				Str("out_refund_no", resource.OutRefundNo).
				Msg("resolve order refund fact object failed")
			server.releaseNotification(ctx, notification.ID, "ecommerce_refund")
			ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "refund fact object resolution failed, please retry"})
			return
		}
		if shouldRecordFact {
			handledByFact = true
			application, err := server.recordOrderEcommerceRefundCallbackFact(ctx, notification, refundOrder, paymentOrder, resource)
			if err != nil {
				paymentCallbackFailuresTotal.WithLabelValues("ecommerce_refund", "record_order_refund_fact").Inc()
				log.Error().Err(err).
					Int64("refund_order_id", refundOrder.ID).
					Str("out_refund_no", resource.OutRefundNo).
					Msg("record order ecommerce refund fact failed")
				server.releaseNotification(ctx, notification.ID, "ecommerce_refund")
				ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "refund fact recording failed, please retry"})
				return
			}
			server.enqueueOrderRefundPaymentFactApplication(ctx, application)
		}
	}

	if !handledByFact {
		// 将非预订退款结果处理放入旧队列
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
	writeWechatNotifySuccess(ctx, "ecommerce_refund")
	server.markNotificationProcessed(ctx, notification.ID, resource.OutTradeNo, resource.TransactionID)
}

// handleOrdinaryServiceProviderRefundNotify 处理普通服务商退款回调通知
// POST /v1/webhooks/wechat-ordinary/refund-notify
func (server *Server) handleOrdinaryServiceProviderRefundNotify(ctx *gin.Context) {
	if server.ordinarySPClient == nil {
		log.Error().Msg("ordinary service provider refund callback received but ordinary client is not configured")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "ordinary service provider client not configured"})
		return
	}

	body, status, err := readWebhookBody(ctx)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("ordinary_refund", "read_body").Inc()
		log.Error().Err(err).Msg("read ordinary service provider refund notification body")
		ctx.JSON(status, wechatPaymentNotifyResponse{Code: "FAIL", Message: "read body failed"})
		return
	}
	ctx.Request.Body = io.NopCloser(bytes.NewReader(body))

	envelope, err := server.ordinarySPClient.ParseNotification(ctx, ctx.Request, ordinaryserviceprovider.NotificationTargetRefund)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("ordinary_refund", "parse").Inc()
		log.Error().Err(err).Msg("parse ordinary service provider refund notification")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{Code: "FAIL", Message: "parse notification failed"})
		return
	}
	notification := ordinaryEnvelopeAsPaymentNotification(envelope)

	validEventTypes := map[string]bool{
		"REFUND.SUCCESS":  true,
		"REFUND.ABNORMAL": true,
		"REFUND.CLOSED":   true,
	}
	if !validEventTypes[notification.EventType] {
		log.Info().Str("event_type", notification.EventType).Msg("ignore non-refund ordinary service provider notification")
		writeWechatNotifySuccess(ctx, "ordinary_refund")
		return
	}

	if !server.tryClaimNotification(ctx, notification, "ordinary_refund") {
		return
	}

	resource, err := ordinaryRefundResourceFromEnvelope(envelope)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("ordinary_refund", "decode_resource").Inc()
		log.Error().Err(err).Msg("decode ordinary service provider refund notification resource")
		server.releaseNotification(ctx, notification.ID, "ordinary_refund")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{Code: "FAIL", Message: "decode resource failed"})
		return
	}

	log.Info().
		Str("sp_mchid", resource.SpMchID).
		Str("sub_mchid", resource.SubMchID).
		Str("out_refund_no", resource.OutRefundNo).
		Str("refund_status", ordinaryRefundStatus(resource)).
		Int64("refund_amount", ordinaryRefundAmount(resource)).
		Msg("received ordinary service provider refund notification")

	if err := server.validateOrdinaryRefundOwnership(resource); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("ordinary_refund", "ownership").Inc()
		log.Error().Err(err).
			Str("sp_mchid", resource.SpMchID).
			Str("sub_mchid", resource.SubMchID).
			Msg("ordinary service provider refund notification ownership validation failed")
		server.sendAlert(websocket.AlertData{
			AlertType:   websocket.AlertTypeSystemError,
			Level:       websocket.AlertLevelCritical,
			Title:       "普通服务商退款回调归属校验失败",
			Message:     fmt.Sprintf("普通服务商退款回调 out_refund_no=%s 的归属校验失败，sp_mchid=%s, sub_mchid=%s。系统已返回 FAIL 等待微信重试，请排查是否存在错服务商或错子商户回调。", resource.OutRefundNo, resource.SpMchID, resource.SubMchID),
			RelatedID:   0,
			RelatedType: "refund_order",
			Extra: map[string]interface{}{
				"out_refund_no": resource.OutRefundNo,
				"out_trade_no":  resource.OutTradeNo,
				"sp_mchid":      resource.SpMchID,
				"sub_mchid":     resource.SubMchID,
			},
		})
		server.releaseNotification(ctx, notification.ID, "ordinary_refund")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "ownership validation failed"})
		return
	}

	if !server.requireTaskDistributorForNotification(ctx, notification.ID, "ordinary_refund", "refund result processing unavailable, please retry") {
		return
	}

	handledByFact := false
	refundOrder, paymentOrder, shouldRecordFact, err := server.resolveReservationRefundFactObjects(ctx, resource.OutRefundNo)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("ordinary_refund", "resolve_refund_fact_object").Inc()
		log.Error().Err(err).Str("out_refund_no", resource.OutRefundNo).Msg("resolve ordinary reservation refund fact object failed")
		server.releaseNotification(ctx, notification.ID, "ordinary_refund")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "refund fact object resolution failed, please retry"})
		return
	}
	if shouldRecordFact {
		handledByFact = true
		if err := server.validateOrdinaryRefundLocalOwnership(ctx, resource, refundOrder, paymentOrder); err != nil {
			paymentCallbackFailuresTotal.WithLabelValues("ordinary_refund", "local_ownership").Inc()
			log.Error().Err(err).
				Int64("refund_order_id", refundOrder.ID).
				Int64("payment_order_id", paymentOrder.ID).
				Str("out_refund_no", resource.OutRefundNo).
				Str("out_trade_no", resource.OutTradeNo).
				Str("sub_mchid", resource.SubMchID).
				Msg("ordinary service provider refund notification local ownership validation failed")
			server.releaseNotification(ctx, notification.ID, "ordinary_refund")
			ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "ownership validation failed"})
			return
		}
		application, err := server.recordReservationOrdinaryRefundCallbackFact(ctx, notification, refundOrder, paymentOrder, resource)
		if err != nil {
			paymentCallbackFailuresTotal.WithLabelValues("ordinary_refund", "record_refund_fact").Inc()
			log.Error().Err(err).Int64("refund_order_id", refundOrder.ID).Str("out_refund_no", resource.OutRefundNo).Msg("record ordinary reservation refund fact failed")
			server.releaseNotification(ctx, notification.ID, "ordinary_refund")
			ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "refund fact recording failed, please retry"})
			return
		}
		server.enqueueReservationRefundPaymentFactApplication(ctx, application)
	}

	if !handledByFact {
		refundOrder, paymentOrder, shouldRecordFact, err = server.resolveOrderRefundFactObjects(ctx, resource.OutRefundNo)
		if err != nil {
			paymentCallbackFailuresTotal.WithLabelValues("ordinary_refund", "resolve_order_refund_fact_object").Inc()
			log.Error().Err(err).Str("out_refund_no", resource.OutRefundNo).Msg("resolve ordinary order refund fact object failed")
			server.releaseNotification(ctx, notification.ID, "ordinary_refund")
			ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "refund fact object resolution failed, please retry"})
			return
		}
		if shouldRecordFact {
			handledByFact = true
			if err := server.validateOrdinaryRefundLocalOwnership(ctx, resource, refundOrder, paymentOrder); err != nil {
				paymentCallbackFailuresTotal.WithLabelValues("ordinary_refund", "local_ownership").Inc()
				log.Error().Err(err).
					Int64("refund_order_id", refundOrder.ID).
					Int64("payment_order_id", paymentOrder.ID).
					Str("out_refund_no", resource.OutRefundNo).
					Str("out_trade_no", resource.OutTradeNo).
					Str("sub_mchid", resource.SubMchID).
					Msg("ordinary service provider refund notification local ownership validation failed")
				server.releaseNotification(ctx, notification.ID, "ordinary_refund")
				ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "ownership validation failed"})
				return
			}
			application, err := server.recordOrderOrdinaryRefundCallbackFact(ctx, notification, refundOrder, paymentOrder, resource)
			if err != nil {
				paymentCallbackFailuresTotal.WithLabelValues("ordinary_refund", "record_order_refund_fact").Inc()
				log.Error().Err(err).Int64("refund_order_id", refundOrder.ID).Str("out_refund_no", resource.OutRefundNo).Msg("record ordinary order refund fact failed")
				server.releaseNotification(ctx, notification.ID, "ordinary_refund")
				ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "refund fact recording failed, please retry"})
				return
			}
			server.enqueueOrderRefundPaymentFactApplication(ctx, application)
		}
	}

	if !handledByFact {
		err = server.taskDistributor.DistributeTaskProcessRefundResult(
			ctx,
			&worker.RefundResultPayload{
				OutRefundNo:  resource.OutRefundNo,
				RefundStatus: ordinaryRefundStatus(resource),
				RefundID:     resource.RefundID,
			},
			asynq.MaxRetry(3),
			asynq.Queue(worker.QueueCritical),
		)
		if err != nil {
			paymentCallbackFailuresTotal.WithLabelValues("ordinary_refund", "enqueue").Inc()
			log.Error().Err(err).Str("out_refund_no", resource.OutRefundNo).Msg("failed to enqueue ordinary refund result task")
			server.releaseNotification(ctx, notification.ID, "ordinary_refund")
			ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "enqueue failed, please retry"})
			return
		}
	}

	writeWechatNotifySuccess(ctx, "ordinary_refund")
	server.markNotificationProcessed(ctx, notification.ID, resource.OutTradeNo, resource.TransactionID)
}

func ordinaryEnvelopeAsPaymentNotification(envelope *ordinaryserviceprovider.NotificationEnvelope) wechat.PaymentNotification {
	if envelope == nil {
		return wechat.PaymentNotification{}
	}
	createTime := time.Time{}
	if envelope.CreateTime != nil {
		createTime = *envelope.CreateTime
	}
	return wechat.PaymentNotification{
		ID:           envelope.ID,
		CreateTime:   createTime,
		EventType:    envelope.EventType,
		ResourceType: envelope.ResourceType,
		Summary:      envelope.Summary,
	}
}

// handleProfitSharingNotify 处理平台收付通分账结果回调通知
// POST /v1/webhooks/wechat-ecommerce/profit-sharing-notify
func (server *Server) handleProfitSharingNotify(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		log.Error().Msg("profit sharing callback received but ecommerce client is not configured")
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
	serial := ctx.GetHeader("Wechatpay-Serial")

	if err := server.ecommerceClient.VerifyNotificationSignature(signature, timestamp, nonce, serial, string(body)); err != nil {
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
		writeWechatNotifySuccess(ctx, "profit_sharing")
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
		Str("sp_mch_id", resource.SPMchID).
		Str("sub_mch_id", resource.SubMchID).
		Str("out_order_no", resource.OutOrderNo).
		Str("order_id", resource.OrderID).
		Msg("received profit sharing notification")

	if err := server.validateProfitSharingOwnership(resource); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("profit_sharing", "ownership").Inc()
		log.Error().Err(err).
			Str("sp_mch_id", resource.SPMchID).
			Str("sub_mch_id", resource.SubMchID).
			Msg("profit sharing notification ownership validation failed")
		server.sendAlert(websocket.AlertData{
			AlertType:   websocket.AlertTypeSystemError,
			Level:       websocket.AlertLevelCritical,
			Title:       "分账回调归属校验失败",
			Message:     fmt.Sprintf("分账回调 out_order_no=%s 的归属校验失败，sp_mch_id=%s, sub_mch_id=%s。系统已返回 FAIL 等待微信重试，请排查是否存在错服务商分账回调。", resource.OutOrderNo, resource.SPMchID, resource.SubMchID),
			RelatedID:   0,
			RelatedType: "profit_sharing_order",
			Extra: map[string]interface{}{
				"out_order_no": resource.OutOrderNo,
				"order_id":     resource.OrderID,
				"sp_mch_id":    resource.SPMchID,
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
			writeWechatNotifySuccess(ctx, "profit_sharing")
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
		writeWechatNotifySuccess(ctx, "profit_sharing")
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
			Msg("query profit sharing detail failed")
	}

	if queryErr != nil {
		paymentCallbackFailuresTotal.WithLabelValues("profit_sharing", "query_profit_sharing").Inc()
		server.releaseNotification(ctx, notification.ID, "profit_sharing")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "query profit sharing failed",
		})
		return
	}

	finalResult, finalFailReason := logic.ResolveProfitSharingQueryFinalResult(queryResp)

	paymentFactApplication, err := server.recordProfitSharingCallbackFact(ctx, db.PaymentChannelEcommerce, notification, profitSharingOrder, resource, queryResp, finalResult, finalFailReason)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("profit_sharing", "record_fact").Inc()
		log.Error().Err(err).
			Int64("profit_sharing_order_id", profitSharingOrder.ID).
			Str("out_order_no", resource.OutOrderNo).
			Str("result", finalResult).
			Msg("record profit sharing payment fact failed")
		server.releaseNotification(ctx, notification.ID, "profit_sharing")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "record payment fact failed",
		})
		return
	}
	switch finalResult {
	case "SUCCESS":
		log.Info().
			Int64("profit_sharing_order_id", profitSharingOrder.ID).
			Str("out_order_no", resource.OutOrderNo).
			Msg("profit sharing terminal fact recorded; waiting for fact application consumer")

	case "FAILED", "CLOSED":
		log.Error().
			Int64("profit_sharing_order_id", profitSharingOrder.ID).
			Str("out_order_no", resource.OutOrderNo).
			Str("result", finalResult).
			Str("fail_reason", finalFailReason).
			Msg("profit sharing terminal fact recorded as failed; waiting for fact application consumer")

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
	server.enqueueProfitSharingPaymentFactApplication(ctx, paymentFactApplication)

	// 通知ID已在 tryClaimNotification 中原子写入，无需重复记录

	// 快速返回成功
	writeWechatNotifySuccess(ctx, "profit_sharing")
	server.markNotificationProcessed(ctx, notification.ID, "", resource.TransactionID)
}

// handleEcommercePaymentNotify 处理平台收付通普通支付回调通知
// POST /v1/webhooks/wechat-ecommerce/payment-notify
func (server *Server) handleEcommercePaymentNotify(ctx *gin.Context) {
	notification, ok := server.readVerifiedEcommerceSuccessNotification(ctx, "ecommerce_payment")
	if !ok {
		return
	}

	if !server.tryClaimNotification(ctx, *notification, "ecommerce_payment") {
		return
	}

	resource, err := server.ecommerceClient.DecryptPartnerPaymentNotification(notification)
	if err != nil {
		log.Error().Err(err).Msg("decrypt ecommerce payment notification")
		server.releaseNotification(ctx, notification.ID, "ecommerce_payment")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "decrypt failed",
		})
		return
	}
	if err := wechatcontracts.ValidatePartnerPaymentNotification("decrypt partner payment notification", resource); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("ecommerce_payment", "contract_validation").Inc()
		log.Error().Err(err).
			Str("notification_id", notification.ID).
			Str("out_trade_no", resource.OutTradeNo).
			Str("transaction_id", resource.TransactionID).
			Msg("partner payment notification contract validation failed")
		server.sendAlert(websocket.AlertData{
			AlertType:   websocket.AlertTypeSystemError,
			Level:       websocket.AlertLevelCritical,
			Title:       "收付通单笔支付回调契约校验失败",
			Message:     fmt.Sprintf("收付通单笔支付回调 notification_id=%s 未通过上游契约校验。系统已返回 FAIL 等待微信重试，请核对微信文档与当前通知 contract 是否漂移。", notification.ID),
			RelatedID:   0,
			RelatedType: "payment_order",
			Extra: map[string]interface{}{
				"notification_id": notification.ID,
				"out_trade_no":    resource.OutTradeNo,
				"transaction_id":  resource.TransactionID,
				"reason":          err.Error(),
			},
		})
		server.releaseNotification(ctx, notification.ID, "ecommerce_payment")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "notification contract validation failed",
		})
		return
	}

	server.handlePartnerPaymentNotification(ctx, *notification, resource)
}

// handleOrdinaryServiceProviderPaymentNotify 处理普通服务商单笔支付回调通知
// POST /v1/webhooks/wechat-ordinary/payment-notify
func (server *Server) handleOrdinaryServiceProviderPaymentNotify(ctx *gin.Context) {
	envelope, notification, ok := server.readOrdinarySuccessNotification(ctx, "ordinary_payment", ordinaryserviceprovider.NotificationTargetPayment)
	if !ok {
		return
	}

	if !server.tryClaimNotification(ctx, notification, "ordinary_payment") {
		return
	}

	resource, err := ordinaryPartnerPaymentResourceFromEnvelope(envelope)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("ordinary_payment", "decode_resource").Inc()
		log.Error().Err(err).Str("notification_id", notification.ID).Msg("decode ordinary service provider payment notification resource")
		server.releaseNotification(ctx, notification.ID, "ordinary_payment")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{Code: "FAIL", Message: "decode resource failed"})
		return
	}
	if err := server.validateOrdinaryPaymentNotifyOwnership(resource); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("ordinary_payment", "ownership").Inc()
		log.Error().Err(err).
			Str("notification_id", notification.ID).
			Str("out_trade_no", resource.OutTradeNo).
			Str("sp_mchid", resource.SpMchID).
			Str("sp_appid", resource.SpAppID).
			Msg("ordinary service provider payment notification ownership validation failed")
		server.releaseNotification(ctx, notification.ID, "ordinary_payment")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "ownership validation failed"})
		return
	}

	server.handlePartnerPaymentNotificationWithOwnership(ctx, notification, ordinaryPaymentNotificationToPartnerResource(resource), "ordinary_payment", noopPartnerPaymentOwnershipValidation)
}

// handleOrdinaryServiceProviderCombinePaymentNotify 处理普通服务商合单支付回调通知
// POST /v1/webhooks/wechat-ordinary/combine-notify
func (server *Server) handleOrdinaryServiceProviderCombinePaymentNotify(ctx *gin.Context) {
	envelope, notification, ok := server.readOrdinarySuccessNotification(ctx, "ordinary_combine_payment", ordinaryserviceprovider.NotificationTargetCombinePayment)
	if !ok {
		return
	}

	if !server.tryClaimNotification(ctx, notification, "ordinary_combine_payment") {
		return
	}

	resource, err := ordinaryCombinePaymentResourceFromEnvelope(envelope)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("ordinary_combine_payment", "decode_resource").Inc()
		log.Error().Err(err).Str("notification_id", notification.ID).Msg("decode ordinary service provider combine payment notification resource")
		server.releaseNotification(ctx, notification.ID, "ordinary_combine_payment")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{Code: "FAIL", Message: "decode resource failed"})
		return
	}
	if err := server.validateOrdinaryCombinePaymentNotifyOwnership(resource); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("ordinary_combine_payment", "ownership").Inc()
		log.Error().Err(err).
			Str("notification_id", notification.ID).
			Str("combine_out_trade_no", resource.CombineOutTradeNo).
			Msg("ordinary service provider combine payment notification ownership validation failed")
		server.releaseNotification(ctx, notification.ID, "ordinary_combine_payment")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "ownership validation failed"})
		return
	}

	server.processCombinePaymentNotification(ctx, &notification, ordinaryCombinePaymentNotificationToLegacy(resource), "ordinary_combine_payment", noopCombinePaymentOwnershipValidation)
}

func (server *Server) readOrdinarySuccessNotification(ctx *gin.Context, callbackType string, target ordinaryserviceprovider.NotificationTarget) (*ordinaryserviceprovider.NotificationEnvelope, wechat.PaymentNotification, bool) {
	if server.ordinarySPClient == nil {
		log.Error().Str("callback_type", callbackType).Msg("ordinary service provider callback received but ordinary client is not configured")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "ordinary service provider client not configured"})
		return nil, wechat.PaymentNotification{}, false
	}

	body, status, err := readWebhookBody(ctx)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues(callbackType, "read_body").Inc()
		log.Error().Err(err).Str("callback_type", callbackType).Msg("read ordinary service provider notification body")
		ctx.JSON(status, wechatPaymentNotifyResponse{Code: "FAIL", Message: "read body failed"})
		return nil, wechat.PaymentNotification{}, false
	}
	ctx.Request.Body = io.NopCloser(bytes.NewReader(body))

	envelope, err := server.ordinarySPClient.ParseNotification(ctx, ctx.Request, target)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues(callbackType, "parse").Inc()
		log.Error().Err(err).Str("callback_type", callbackType).Msg("parse ordinary service provider notification")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{Code: "FAIL", Message: "parse notification failed"})
		return nil, wechat.PaymentNotification{}, false
	}
	notification := ordinaryEnvelopeAsPaymentNotification(envelope)
	if notification.EventType != "TRANSACTION.SUCCESS" {
		log.Info().Str("callback_type", callbackType).Str("event_type", notification.EventType).Msg("ignore non-success ordinary service provider notification")
		writeWechatNotifySuccess(ctx, callbackType)
		return nil, wechat.PaymentNotification{}, false
	}
	return envelope, notification, true
}

func ordinaryPartnerPaymentResourceFromEnvelope(envelope *ordinaryserviceprovider.NotificationEnvelope) (*ospcontracts.PaymentNotificationPayload, error) {
	if envelope == nil {
		return nil, errors.New("ordinary payment notification envelope is nil")
	}
	data, err := ordinaryNotificationResourceBytes(envelope)
	if err != nil {
		return nil, err
	}
	resource := &ospcontracts.PaymentNotificationPayload{}
	if err := json.Unmarshal(data, resource); err != nil {
		return nil, fmt.Errorf("decode ordinary payment notification resource: %w", err)
	}
	if err := ospcontracts.ValidatePaymentNotificationPayload(*resource); err != nil {
		return nil, err
	}
	return resource, nil
}

func ordinaryCombinePaymentResourceFromEnvelope(envelope *ordinaryserviceprovider.NotificationEnvelope) (*ospcontracts.CombinePaymentNotificationPayload, error) {
	if envelope == nil {
		return nil, errors.New("ordinary combine payment notification envelope is nil")
	}
	data, err := ordinaryNotificationResourceBytes(envelope)
	if err != nil {
		return nil, err
	}
	resource := &ospcontracts.CombinePaymentNotificationPayload{}
	if err := json.Unmarshal(data, resource); err != nil {
		return nil, fmt.Errorf("decode ordinary combine payment notification resource: %w", err)
	}
	if err := ospcontracts.ValidateCombinePaymentNotificationPayload(*resource); err != nil {
		return nil, err
	}
	return resource, nil
}

func ordinaryPaymentNotificationToPartnerResource(resource *ospcontracts.PaymentNotificationPayload) *wechatcontracts.PartnerPaymentNotificationResource {
	if resource == nil {
		return nil
	}
	converted := &wechatcontracts.PartnerPaymentNotificationResource{
		SpAppID:        resource.SpAppID,
		SpMchID:        resource.SpMchID,
		SubAppID:       resource.SubAppID,
		SubMchID:       resource.SubMchID,
		OutTradeNo:     resource.OutTradeNo,
		TransactionID:  resource.TransactionID,
		TradeType:      resource.TradeType,
		TradeState:     string(resource.TradeState),
		TradeStateDesc: resource.TradeStateDesc,
		BankType:       resource.BankType,
		Attach:         resource.Attach,
		SuccessTime:    resource.SuccessTime,
		Payer: wechatcontracts.PartnerOrderPayerInfo{
			SpOpenID:  resource.Payer.SpOpenID,
			SubOpenID: resource.Payer.SubOpenID,
		},
		PromotionDetail: ordinaryPaymentPromotionDetails(resource.PromotionDetail),
	}
	if resource.Amount != nil {
		converted.Amount = wechatcontracts.PartnerOrderQueryAmount{
			Total:         resource.Amount.Total,
			PayerTotal:    resource.Amount.PayerTotal,
			Currency:      string(resource.Amount.Currency),
			PayerCurrency: string(resource.Amount.PayerCurrency),
		}
	}
	if resource.SceneInfo != nil {
		converted.SceneInfo = &wechatcontracts.PartnerOrderQuerySceneInfo{DeviceID: resource.SceneInfo.DeviceID}
	}
	return converted
}

func ordinaryPaymentPromotionDetails(details []ospcontracts.PaymentPromotionDetail) []wechatcontracts.PartnerPromotionDetail {
	if len(details) == 0 {
		return nil
	}
	converted := make([]wechatcontracts.PartnerPromotionDetail, 0, len(details))
	for _, detail := range details {
		converted = append(converted, wechatcontracts.PartnerPromotionDetail{
			CouponID:            detail.CouponID,
			Name:                detail.Name,
			Scope:               detail.Scope,
			Type:                detail.Type,
			Amount:              detail.Amount,
			StockID:             detail.StockID,
			WechatpayContribute: detail.WechatpayContribute,
			MerchantContribute:  detail.MerchantContribute,
			OtherContribute:     detail.OtherContribute,
			Currency:            detail.Currency,
			GoodsDetail:         ordinaryPaymentPromotionGoodsDetails(detail.GoodsDetail),
		})
	}
	return converted
}

func ordinaryPaymentPromotionGoodsDetails(details []ospcontracts.PaymentPromotionGoodsDetail) []wechatcontracts.PartnerPromotionGoodsDetail {
	if len(details) == 0 {
		return nil
	}
	converted := make([]wechatcontracts.PartnerPromotionGoodsDetail, 0, len(details))
	for _, detail := range details {
		converted = append(converted, wechatcontracts.PartnerPromotionGoodsDetail{
			GoodsID:        detail.GoodsID,
			Quantity:       detail.Quantity,
			UnitPrice:      detail.UnitPrice,
			DiscountAmount: detail.DiscountAmount,
			GoodsRemark:    detail.GoodsRemark,
		})
	}
	return converted
}

func ordinaryCombinePaymentNotificationToLegacy(resource *ospcontracts.CombinePaymentNotificationPayload) *wechatcontracts.CombinePaymentNotification {
	if resource == nil {
		return nil
	}
	converted := &wechatcontracts.CombinePaymentNotification{
		CombineAppID:      resource.CombineAppID,
		CombineMchID:      resource.CombineMchID,
		CombineOutTradeNo: resource.CombineOutTradeNo,
		SubOrders:         ordinaryCombinePaymentSubOrdersToLegacy(resource.SubOrders),
		CombinePayerInfo:  &wechatcontracts.CombinePaymentNotificationPayerInfo{OpenID: resource.CombinePayerInfo.OpenID},
	}
	if resource.SceneInfo != nil {
		converted.SceneInfo = &wechatcontracts.CombinePaymentNotificationSceneInfo{DeviceID: resource.SceneInfo.DeviceID}
	}
	return converted
}

func ordinaryCombinePaymentSubOrdersToLegacy(orders []ospcontracts.CombineOrderState) []wechatcontracts.CombinePaymentNotificationSubOrder {
	converted := make([]wechatcontracts.CombinePaymentNotificationSubOrder, 0, len(orders))
	for _, order := range orders {
		item := wechatcontracts.CombinePaymentNotificationSubOrder{
			MchID:           order.MchID,
			SubMchID:        order.SubMchID,
			SubAppID:        order.SubAppID,
			OutTradeNo:      order.OutTradeNo,
			TransactionID:   order.TransactionID,
			TradeType:       order.TradeType,
			TradeState:      string(order.TradeState),
			BankType:        order.BankType,
			Attach:          order.Attach,
			PromotionDetail: ordinaryPaymentPromotionDetails(order.PromotionDetail),
			SuccessTime:     order.SuccessTime,
		}
		item.Amount.TotalAmount = order.Amount.TotalAmount
		item.Amount.PayerAmount = order.Amount.PayerAmount
		item.Amount.Currency = string(order.Amount.Currency)
		item.Amount.PayerCurrency = string(order.Amount.PayerCurrency)
		item.Amount.SettlementRate = order.Amount.SettlementRate
		converted = append(converted, item)
	}
	return converted
}

func noopPartnerPaymentOwnershipValidation(*wechatcontracts.PartnerPaymentNotificationResource) error {
	return nil
}

func noopCombinePaymentOwnershipValidation(*wechatcontracts.CombinePaymentNotification) error {
	return nil
}

func ordinaryNotificationResourceBytes(envelope *ordinaryserviceprovider.NotificationEnvelope) ([]byte, error) {
	if strings.TrimSpace(envelope.Plaintext) != "" {
		return []byte(envelope.Plaintext), nil
	}
	data, err := json.Marshal(envelope.Decoded)
	if err != nil {
		return nil, fmt.Errorf("marshal ordinary notification decoded resource: %w", err)
	}
	return data, nil
}

// handleCombinePaymentNotify 处理平台收付通合单支付回调通知
// POST /v1/webhooks/wechat-ecommerce/combine-notify
func (server *Server) handleCombinePaymentNotify(ctx *gin.Context) {
	notification, ok := server.readVerifiedEcommerceSuccessNotification(ctx, "combine_payment")
	if !ok {
		return
	}

	// 🔐 #1: 原子幂等性门
	if !server.tryClaimNotification(ctx, *notification, "combine_payment") {
		return
	}

	resource, err := server.ecommerceClient.DecryptCombinePaymentNotification(notification)
	if err != nil {
		log.Error().Err(err).Msg("decrypt combine payment notification")
		server.releaseNotification(ctx, notification.ID, "combine_payment")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "decrypt failed",
		})
		return
	}
	if err := wechatcontracts.ValidateCombinePaymentNotification("decrypt combine payment notification", resource); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("combine_payment", "contract_validation").Inc()
		log.Error().Err(err).
			Str("notification_id", notification.ID).
			Str("combine_out_trade_no", resource.CombineOutTradeNo).
			Msg("combine payment notification contract validation failed")
		server.sendAlert(websocket.AlertData{
			AlertType:   websocket.AlertTypeSystemError,
			Level:       websocket.AlertLevelCritical,
			Title:       "合单支付回调契约校验失败",
			Message:     fmt.Sprintf("合单支付回调 notification_id=%s 未通过上游契约校验。系统已返回 FAIL 等待微信重试，请核对微信文档与当前通知 contract 是否漂移。", notification.ID),
			RelatedID:   0,
			RelatedType: "combined_payment_order",
			Extra: map[string]interface{}{
				"notification_id":      notification.ID,
				"combine_out_trade_no": resource.CombineOutTradeNo,
				"reason":               err.Error(),
			},
		})
		server.releaseNotification(ctx, notification.ID, "combine_payment")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "notification contract validation failed",
		})
		return
	}

	server.processCombinePaymentNotification(ctx, notification, resource, "combine_payment", server.validateCombineNotifyOwnership)
}

func (server *Server) processCombinePaymentNotification(ctx *gin.Context, notification *wechat.PaymentNotification, resource *wechatcontracts.CombinePaymentNotification, callbackLabel string, validateOwnership func(*wechatcontracts.CombinePaymentNotification) error) {
	log.Info().
		Str("combine_out_trade_no", resource.CombineOutTradeNo).
		Int("sub_orders_count", len(resource.SubOrders)).
		Msg("received combine payment notification")

	if err := validateOwnership(resource); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "ownership").Inc()
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
		server.releaseNotification(ctx, notification.ID, callbackLabel)
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "ownership validation failed",
		})
		return
	}

	combinedOrder, err := server.store.GetCombinedPaymentOrderByOutTradeNo(ctx, resource.CombineOutTradeNo)
	if err != nil {
		paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "combined_order_not_found").Inc()
		log.Error().Err(err).
			Str("combine_out_trade_no", resource.CombineOutTradeNo).
			Msg("get combined payment order failed before processing sub orders")
		if isNotFoundError(err) {
			server.sendAlert(websocket.AlertData{
				AlertType:   websocket.AlertTypeSystemError,
				Level:       websocket.AlertLevelCritical,
				Title:       "合单回调未找到本地主合单",
				Message:     fmt.Sprintf("合单回调 combine_out_trade_no=%s 未找到本地 combined_payment_order。系统已返回 FAIL 等待微信重试，请尽快排查主合单创建链路。", resource.CombineOutTradeNo),
				RelatedID:   0,
				RelatedType: "combined_payment_order",
				Extra: map[string]interface{}{
					"combine_out_trade_no": resource.CombineOutTradeNo,
					"business_action":      "wechat_retry_expected",
				},
			})
		}
		server.releaseNotification(ctx, notification.ID, callbackLabel)
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "get combined payment order failed",
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

		if !paymentOrder.CombinedPaymentID.Valid || paymentOrder.CombinedPaymentID.Int64 != combinedOrder.ID {
			paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "combined_order_mismatch").Inc()
			log.Error().
				Int64("payment_order_id", paymentOrder.ID).
				Str("out_trade_no", subOrder.OutTradeNo).
				Str("combine_out_trade_no", resource.CombineOutTradeNo).
				Msg("combine sub-order does not belong to the notified combined payment")
			server.sendAlert(websocket.AlertData{
				AlertType:   websocket.AlertTypeSystemError,
				Level:       websocket.AlertLevelCritical,
				Title:       "合单回调子单归属校验失败",
				Message:     fmt.Sprintf("合单回调 combine_out_trade_no=%s 的子单 %s 归属校验失败，本地 payment_order=%d 不属于该主合单。系统已返回 FAIL 等待微信重试，请尽快排查。", resource.CombineOutTradeNo, subOrder.OutTradeNo, paymentOrder.ID),
				RelatedID:   paymentOrder.ID,
				RelatedType: "payment_order",
				Extra: map[string]interface{}{
					"combine_out_trade_no":      resource.CombineOutTradeNo,
					"out_trade_no":              subOrder.OutTradeNo,
					"payment_order_id":          paymentOrder.ID,
					"payment_combined_order_id": paymentOrder.CombinedPaymentID.Int64,
					"expected_combined_id":      combinedOrder.ID,
				},
			})
			failedOrders = append(failedOrders, subOrder.OutTradeNo)
			continue
		}

		// 检查是否已处理（幂等）
		if paymentOrder.Status == PaymentStatusPaid {
			if paymentOrder.ProcessedAt.Valid {
				successCount++
				continue
			}
			if paymentOrder.BusinessType == db.ExternalPaymentBusinessOwnerOrder && paymentOrderUsesMainBusinessPaymentChannel(paymentOrder) {
				application, factErr := server.recordCombinedOrderPaymentCallbackFactByChannel(ctx, notification, combinedOrder, paymentOrder, subOrder)
				if factErr != nil {
					paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "record_order_payment_fact").Inc()
					log.Error().Err(factErr).
						Int64("payment_order_id", paymentOrder.ID).
						Str("out_trade_no", subOrder.OutTradeNo).
						Msg("record combined order payment callback fact failed")
					failedOrders = append(failedOrders, subOrder.OutTradeNo)
					continue
				}
				if enqErr := server.enqueueOrderPaymentFactApplication(ctx, application); enqErr != nil {
					paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "reenqueue_order_payment_fact_application").Inc()
					log.Error().Err(enqErr).
						Int64("payment_order_id", paymentOrder.ID).
						Str("out_trade_no", subOrder.OutTradeNo).
						Msg("combine sub-order already paid but order payment fact application enqueue failed")
					failedOrders = append(failedOrders, subOrder.OutTradeNo)
					continue
				}
			} else if shouldRecordReservationPaymentFact(paymentOrder) {
				application, factErr := server.recordCombinedReservationPaymentCallbackFactByChannel(ctx, notification, combinedOrder, paymentOrder, subOrder)
				if factErr != nil {
					paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "record_reservation_payment_fact").Inc()
					log.Error().Err(factErr).
						Int64("payment_order_id", paymentOrder.ID).
						Str("out_trade_no", subOrder.OutTradeNo).
						Msg("record combined reservation payment callback fact failed")
					failedOrders = append(failedOrders, subOrder.OutTradeNo)
					continue
				}
				if enqErr := server.enqueueReservationPaymentFactApplication(ctx, application); enqErr != nil {
					paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "reenqueue_reservation_payment_fact_application").Inc()
					log.Error().Err(enqErr).
						Int64("payment_order_id", paymentOrder.ID).
						Str("out_trade_no", subOrder.OutTradeNo).
						Msg("combine sub-order already paid but reservation payment fact application enqueue failed")
					failedOrders = append(failedOrders, subOrder.OutTradeNo)
					continue
				}
			} else {
				paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "unsupported_payment_fact_owner").Inc()
				log.Error().
					Int64("payment_order_id", paymentOrder.ID).
					Str("out_trade_no", subOrder.OutTradeNo).
					Str("business_type", paymentOrder.BusinessType).
					Str("payment_channel", paymentOrder.PaymentChannel).
					Msg("combine sub-order already paid but no payment fact application owner matched")
				failedOrders = append(failedOrders, subOrder.OutTradeNo)
				continue
			}
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
			if server.taskDistributor == nil {
				paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "anomaly_refund_distributor_missing").Inc()
				server.sendAlert(websocket.AlertData{
					AlertType:   websocket.AlertTypePaymentAmountMismatch,
					Level:       websocket.AlertLevelCritical,
					Title:       "合单异常到账无法入队退款",
					Message:     fmt.Sprintf("合单子订单 %s 已处于 %s 状态但微信仍到账，任务分发器未配置，系统已返回 FAIL 等待微信重试，请立即排查退款链路。", subOrder.OutTradeNo, paymentOrder.Status),
					RelatedID:   paymentOrder.ID,
					RelatedType: "payment_order",
					Extra: map[string]interface{}{
						"combine_out_trade_no": resource.CombineOutTradeNo,
						"out_trade_no":         subOrder.OutTradeNo,
						"transaction_id":       subOrder.TransactionID,
						"payment_status":       paymentOrder.Status,
					},
				})
				failedOrders = append(failedOrders, subOrder.OutTradeNo)
				continue
			}

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
				paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "enqueue_anomaly_refund_task").Inc()
				log.Error().Err(enqErr).
					Int64("payment_order_id", paymentOrder.ID).
					Msg("failed to enqueue anomaly refund task for combine sub-order")
				server.sendAlert(websocket.AlertData{
					AlertType:   websocket.AlertTypePaymentAmountMismatch,
					Level:       websocket.AlertLevelCritical,
					Title:       "合单异常到账退款任务入队失败",
					Message:     fmt.Sprintf("合单子订单 %s 已处于 %s 状态但微信仍到账，异常退款任务入队失败。系统已返回 FAIL 等待微信重试，请立即排查。", subOrder.OutTradeNo, paymentOrder.Status),
					RelatedID:   paymentOrder.ID,
					RelatedType: "payment_order",
					Extra: map[string]interface{}{
						"combine_out_trade_no": resource.CombineOutTradeNo,
						"out_trade_no":         subOrder.OutTradeNo,
						"transaction_id":       subOrder.TransactionID,
						"payment_status":       paymentOrder.Status,
						"error":                enqErr.Error(),
					},
				})
				failedOrders = append(failedOrders, subOrder.OutTradeNo)
				continue
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
				paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "create_mismatch_refund_record").Inc()
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
					paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "enqueue_mismatch_refund_task").Inc()
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

		if paymentOrder.BusinessType == db.ExternalPaymentBusinessOwnerOrder && paymentOrderUsesMainBusinessPaymentChannel(paymentOrder) {
			application, factErr := server.recordCombinedOrderPaymentCallbackFactByChannel(ctx, notification, combinedOrder, paymentOrder, subOrder)
			if factErr != nil {
				paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "record_order_payment_fact").Inc()
				log.Error().Err(factErr).
					Int64("payment_order_id", paymentOrder.ID).
					Str("out_trade_no", subOrder.OutTradeNo).
					Msg("record combined order payment callback fact failed")
				failedOrders = append(failedOrders, subOrder.OutTradeNo)
				continue
			}
			if enqErr := server.enqueueOrderPaymentFactApplication(ctx, application); enqErr != nil {
				paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "enqueue_order_payment_fact_application").Inc()
				log.Error().Err(enqErr).
					Int64("payment_order_id", paymentOrder.ID).
					Str("out_trade_no", subOrder.OutTradeNo).
					Msg("combine order payment fact application enqueue failed")
				failedOrders = append(failedOrders, subOrder.OutTradeNo)
				continue
			}
		} else if shouldRecordReservationPaymentFact(paymentOrder) {
			application, factErr := server.recordCombinedReservationPaymentCallbackFactByChannel(ctx, notification, combinedOrder, paymentOrder, subOrder)
			if factErr != nil {
				paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "record_reservation_payment_fact").Inc()
				log.Error().Err(factErr).
					Int64("payment_order_id", paymentOrder.ID).
					Str("out_trade_no", subOrder.OutTradeNo).
					Msg("record combined reservation payment callback fact failed")
				failedOrders = append(failedOrders, subOrder.OutTradeNo)
				continue
			}
			if enqErr := server.enqueueReservationPaymentFactApplication(ctx, application); enqErr != nil {
				paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "enqueue_reservation_payment_fact_application").Inc()
				log.Error().Err(enqErr).
					Int64("payment_order_id", paymentOrder.ID).
					Str("out_trade_no", subOrder.OutTradeNo).
					Msg("combine reservation payment fact application enqueue failed")
				failedOrders = append(failedOrders, subOrder.OutTradeNo)
				continue
			}
		} else {
			paymentCallbackFailuresTotal.WithLabelValues(callbackLabel, "unsupported_payment_fact_owner").Inc()
			log.Error().
				Int64("payment_order_id", paymentOrder.ID).
				Str("out_trade_no", subOrder.OutTradeNo).
				Str("business_type", paymentOrder.BusinessType).
				Str("payment_channel", paymentOrder.PaymentChannel).
				Msg("combine payment callback paid transition has no payment fact application owner")
			server.sendAlert(websocket.AlertData{
				AlertType:   websocket.AlertTypeTaskEnqueueFailure,
				Level:       websocket.AlertLevelCritical,
				Title:       "合单支付事实应用匹配失败",
				Message:     fmt.Sprintf("合单子订单 %s 已支付，但业务类型未匹配支付事实应用，系统已返回 FAIL 请求微信重试。", subOrder.OutTradeNo),
				RelatedID:   paymentOrder.ID,
				RelatedType: "payment_order",
				Extra: map[string]interface{}{
					"out_trade_no":   subOrder.OutTradeNo,
					"transaction_id": subOrder.TransactionID,
					"business_type":  paymentOrder.BusinessType,
				},
			})
			failedOrders = append(failedOrders, subOrder.OutTradeNo)
			continue
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
		writeWechatNotifySuccess(ctx, callbackLabel)
		server.markNotificationProcessed(ctx, notification.ID, resource.CombineOutTradeNo, "")
		return
	}

	// ✅ 更新合单主单状态为已支付，保证对账、恢复扫描、报表的 paid_at 正常落库。
	// 合单层级无单一 transaction_id（每子单各持一个），主单 transaction_id 置 NULL 合规。
	if combinedOrder.Status == PaymentStatusPaid {
		writeWechatNotifySuccess(ctx, callbackLabel)
		server.markNotificationProcessed(ctx, notification.ID, resource.CombineOutTradeNo, "")
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
		server.releaseNotification(ctx, notification.ID, callbackLabel)
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

	writeWechatNotifySuccess(ctx, callbackLabel)
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
		log.Error().Msg("applyment callback received but ecommerce client is not configured")
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
	serial := ctx.GetHeader("Wechatpay-Serial")

	if err := server.ecommerceClient.VerifyNotificationSignature(signature, timestamp, nonce, serial, string(body)); err != nil {
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
		writeWechatNotifySuccess(ctx, "applyment")
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

	decrypted, err := server.ecommerceClient.DecryptNotificationRaw(&paymentNotification)
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
			writeWechatNotifySuccess(ctx, "applyment")
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
	if applyment.SubjectType != "merchant" {
		log.Warn().
			Int64("applyment_id", applyment.ID).
			Str("subject_type", applyment.SubjectType).
			Msg("ignore non-merchant applyment subject type")
		server.markNotificationProcessed(ctx, notification.ID, "", "")
		writeWechatNotifySuccess(ctx, "applyment")
		return
	}

	// 映射状态；未知上游状态时保持本地状态，避免查询/回调入口写入不一致语义。
	mappedStatus := mapApplymentStateToDBStatus(resource.ApplymentState)
	newStatus := resolveApplymentCallbackStatus(applyment.Status, resource.ApplymentState)
	if mappedStatus == "finish" && resource.SubMchID == "" {
		newStatus = "submitted"
	}
	shouldActivate := newStatus == "finish" && resource.SubMchID != ""
	shouldRoutePendingFact := newStatus == "account_need_verify" || newStatus == "to_be_confirmed" || newStatus == "to_be_signed"
	shouldRouteTerminalFact := newStatus == "rejected" || newStatus == "frozen" || newStatus == "canceled"

	// merchant applyment 的 FINISH 终态改由 payment fact/application owner path 驱动。
	if shouldActivate {
		if !server.requireTaskDistributorForNotification(ctx, notification.ID, "applyment", "applyment activation processing unavailable, please retry") {
			return
		}
		application, factErr := worker.RecordApplymentActivatedCallbackFact(ctx, server.store, applyment, map[string]any{
			"applyment_id":    resource.ApplymentID,
			"out_request_no":  resource.OutRequestNo,
			"applyment_state": resource.ApplymentState,
			"sub_mchid":       resource.SubMchID,
		}, notification.ID, notification.EventType, resource.SubMchID)
		if factErr != nil {
			log.Error().Err(factErr).
				Int64("applyment_id", applyment.ID).
				Str("out_request_no", resource.OutRequestNo).
				Msg("record applyment activation callback fact failed")
			server.releaseNotification(ctx, notification.ID, "applyment")
			ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "internal error"})
			return
		}
		if err := worker.EnqueueApplymentPaymentFactApplication(ctx, server.taskDistributor, application); err != nil {
			paymentCallbackFailuresTotal.WithLabelValues("applyment", "enqueue_fact_application").Inc()
			log.Error().Err(err).
				Int64("applyment_id", applyment.ID).
				Str("out_request_no", resource.OutRequestNo).
				Msg("enqueue applyment activation fact application failed")
			server.releaseNotification(ctx, notification.ID, "applyment")
			ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "enqueue failed, please retry"})
			return
		}
		writeWechatNotifySuccess(ctx, "applyment")
		server.markNotificationProcessed(ctx, notification.ID, "", "")
		return
	}
	if shouldRoutePendingFact {
		if !server.requireTaskDistributorForNotification(ctx, notification.ID, "applyment", "applyment result processing unavailable, please retry") {
			return
		}
		application, factErr := worker.RecordApplymentPendingCallbackFact(ctx, server.store, applyment, map[string]any{
			"applyment_id":    resource.ApplymentID,
			"out_request_no":  resource.OutRequestNo,
			"applyment_state": resource.ApplymentState,
			"sub_mchid":       resource.SubMchID,
		}, notification.ID, notification.EventType, resource.ApplymentState, resource.SubMchID)
		if factErr != nil {
			log.Error().Err(factErr).
				Int64("applyment_id", applyment.ID).
				Str("out_request_no", resource.OutRequestNo).
				Str("applyment_state", resource.ApplymentState).
				Msg("record applyment pending callback fact failed")
			server.releaseNotification(ctx, notification.ID, "applyment")
			ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "internal error"})
			return
		}
		if err := worker.EnqueueApplymentPaymentFactApplication(ctx, server.taskDistributor, application); err != nil {
			paymentCallbackFailuresTotal.WithLabelValues("applyment", "enqueue_fact_application").Inc()
			log.Error().Err(err).
				Int64("applyment_id", applyment.ID).
				Str("out_request_no", resource.OutRequestNo).
				Str("applyment_state", resource.ApplymentState).
				Msg("enqueue applyment pending fact application failed")
			server.releaseNotification(ctx, notification.ID, "applyment")
			ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "enqueue failed, please retry"})
			return
		}
		writeWechatNotifySuccess(ctx, "applyment")
		server.markNotificationProcessed(ctx, notification.ID, "", "")
		return
	}
	if shouldRouteTerminalFact {
		if !server.requireTaskDistributorForNotification(ctx, notification.ID, "applyment", "applyment result processing unavailable, please retry") {
			return
		}
		application, factErr := worker.RecordApplymentTerminalCallbackFact(ctx, server.store, applyment, map[string]any{
			"applyment_id":    resource.ApplymentID,
			"out_request_no":  resource.OutRequestNo,
			"applyment_state": resource.ApplymentState,
			"sub_mchid":       resource.SubMchID,
		}, notification.ID, notification.EventType, resource.ApplymentState, resource.SubMchID)
		if factErr != nil {
			log.Error().Err(factErr).
				Int64("applyment_id", applyment.ID).
				Str("out_request_no", resource.OutRequestNo).
				Str("applyment_state", resource.ApplymentState).
				Msg("record applyment terminal callback fact failed")
			server.releaseNotification(ctx, notification.ID, "applyment")
			ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "internal error"})
			return
		}
		if err := worker.EnqueueApplymentPaymentFactApplication(ctx, server.taskDistributor, application); err != nil {
			paymentCallbackFailuresTotal.WithLabelValues("applyment", "enqueue_fact_application").Inc()
			log.Error().Err(err).
				Int64("applyment_id", applyment.ID).
				Str("out_request_no", resource.OutRequestNo).
				Str("applyment_state", resource.ApplymentState).
				Msg("enqueue applyment terminal fact application failed")
			server.releaseNotification(ctx, notification.ID, "applyment")
			ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "enqueue failed, please retry"})
			return
		}
		writeWechatNotifySuccess(ctx, "applyment")
		server.markNotificationProcessed(ctx, notification.ID, "", "")
		return
	}

	{
		nextSubMchID := applyment.SubMchID
		if resource.SubMchID != "" {
			nextSubMchID = pgtype.Text{String: resource.SubMchID, Valid: true}
		}
		// 更新进件状态
		_, err = server.store.UpdateEcommerceApplymentStatus(ctx, db.UpdateEcommerceApplymentStatusParams{
			ID:                 applyment.ID,
			ApplymentID:        applyment.ApplymentID,
			Status:             newStatus,
			RejectReason:       pgtype.Text{},     // 如果有驳回原因需要主动查询
			SignUrl:            applyment.SignUrl, // 回调资源不带签约字段，保留已落库值避免被清空
			SignState:          applyment.SignState,
			LegalValidationUrl: applyment.LegalValidationUrl,
			AccountValidation:  applyment.AccountValidation,
			SubMchID:           nextSubMchID,
		})
		if err != nil {
			log.Error().Err(err).Int64("applyment_id", applyment.ID).Msg("update applyment status")
		}

	}

	// 通知ID已在 tryClaimApplymentNotification 中原子写入，无需重复记录

	if !server.requireTaskDistributorForNotification(ctx, notification.ID, "applyment", "applyment result processing unavailable, please retry") {
		return
	}

	// 📤 异步处理：发送通知 + 添加分账接收方
	if err := server.taskDistributor.DistributeTaskProcessApplymentResult(
		ctx,
		&worker.ApplymentResultPayload{
			ApplymentID:     applyment.ID,
			OutRequestNo:    resource.OutRequestNo,
			ApplymentState:  resource.ApplymentState,
			ApplymentStatus: newStatus,
			SignState:       strings.TrimSpace(applyment.SignState.String),
			SubMchID:        resource.SubMchID,
			SubjectType:     applyment.SubjectType,
			SubjectID:       applyment.SubjectID,
		},
		asynq.MaxRetry(3),
		asynq.Queue(worker.QueueDefault),
	); err != nil {
		paymentCallbackFailuresTotal.WithLabelValues("applyment", "enqueue").Inc()
		log.Error().Err(err).
			Int64("applyment_id", applyment.ID).
			Str("out_request_no", resource.OutRequestNo).
			Msg("enqueue applyment follow-up task failed")
		server.releaseNotification(ctx, notification.ID, "applyment")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "enqueue failed, please retry",
		})
		return
	}

	writeWechatNotifySuccess(ctx, "applyment")
	server.markNotificationProcessed(ctx, notification.ID, "", "")
}

// mapApplymentStateToDBStatus 将微信进件状态映射为数据库状态
func mapApplymentStateToDBStatus(wechatState string) string {
	return logic.MapWechatApplymentStateToStatus(wechatState)
}

func resolveApplymentCallbackStatus(currentStatus, wechatState string) string {
	if strings.TrimSpace(wechatState) == "NEED_SIGN" {
		switch strings.TrimSpace(currentStatus) {
		case "to_be_signed", "signing":
			return currentStatus
		}
	}
	return logic.ResolveWechatApplymentStatus(currentStatus, wechatState, "")
}

// handleOrderSettlementNotify 处理微信订单结算事件通知
// POST /v1/webhooks/wechat-miniprogram/settlement-notify
//
// 微信在用户确认收货（或 T+2 自动确认）后推送 trade_manage_order_settlement 事件。
// settlement_time 字段非空代表资金已实际结算，此时触发分账。
func (server *Server) handleOrderSettlementNotify(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		log.Error().Msg("settlement callback received but ecommerce client is not configured")
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
	serial := ctx.GetHeader("Wechatpay-Serial")

	if err := server.ecommerceClient.VerifyNotificationSignature(signature, timestamp, nonce, serial, string(body)); err != nil {
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
		writeWechatNotifySuccess(ctx, "settlement")
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
		writeWechatNotifySuccess(ctx, "settlement")
		return
	}

	// 通过子单 out_trade_no 找到对应的业务订单。收付通单笔支付不会写 combined sub order，
	// 这里需要先查合单子单，再回退到 payment_order。
	subOrder, err := server.store.GetCombinedPaymentSubOrderByOutTradeNo(ctx, resource.MerchantTradeNo)
	if err != nil {
		if isNotFoundError(err) {
			po, paymentErr := server.store.GetPaymentOrderByOutTradeNo(ctx, resource.MerchantTradeNo)
			if paymentErr != nil {
				if isNotFoundError(paymentErr) {
					log.Warn().Str("merchant_trade_no", resource.MerchantTradeNo).Msg("settlement payment order not found, skip")
					server.markNotificationProcessed(ctx, notification.ID, resource.MerchantTradeNo, resource.TransactionID)
					writeWechatNotifySuccess(ctx, "settlement")
					return
				}
				paymentCallbackFailuresTotal.WithLabelValues("settlement", "query_payment_order").Inc()
				log.Error().Err(paymentErr).Str("merchant_trade_no", resource.MerchantTradeNo).Msg("get payment order for settlement fallback")
				server.releaseNotification(ctx, notification.ID, "settlement")
				ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "internal error"})
				return
			}

			if po.Status != "paid" || !db.PaymentOrderUsesEcommerceChannel(po) || !db.PaymentOrderRequiresProfitSharing(po) || !po.OrderID.Valid {
				log.Info().
					Int64("payment_order_id", po.ID).
					Str("status", po.Status).
					Str("payment_type", po.PaymentType).
					Str("payment_channel", po.PaymentChannel).
					Msg("settlement fallback: payment order not eligible for profit sharing, skip")
				server.markNotificationProcessed(ctx, notification.ID, resource.MerchantTradeNo, resource.TransactionID)
				writeWechatNotifySuccess(ctx, "settlement")
				return
			}

			if !server.requireTaskDistributorForNotification(ctx, notification.ID, "settlement", "profit sharing dispatch unavailable, please retry") {
				return
			}

			err = server.taskDistributor.DistributeTaskProcessProfitSharing(
				ctx,
				&worker.ProfitSharingPayload{
					PaymentOrderID: po.ID,
					OrderID:        po.OrderID.Int64,
				},
				asynq.MaxRetry(5),
				asynq.ProcessIn(2*time.Second),
				asynq.Queue(worker.QueueCritical),
			)
			if err != nil {
				paymentCallbackFailuresTotal.WithLabelValues("settlement", "enqueue_profit_sharing_task").Inc()
				log.Error().Err(err).Int64("payment_order_id", po.ID).Str("merchant_trade_no", resource.MerchantTradeNo).Msg("settlement fallback profit sharing task enqueue failed")
				server.sendAlert(websocket.AlertData{
					AlertType:   websocket.AlertTypeTaskEnqueueFailure,
					Level:       websocket.AlertLevelCritical,
					Title:       "结算分账任务入队失败",
					Message:     fmt.Sprintf("支付单 %d 已进入结算事件，但分账任务入队失败，需要人工介入确认商户结算", po.ID),
					RelatedID:   po.ID,
					RelatedType: "payment_order",
					Extra: map[string]interface{}{
						"merchant_trade_no": resource.MerchantTradeNo,
						"out_trade_no":      po.OutTradeNo,
						"order_id":          po.OrderID.Int64,
						"payment_order_id":  po.ID,
						"error":             err.Error(),
					},
				})
				server.releaseNotification(ctx, notification.ID, "settlement")
				ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "enqueue failed, please retry"})
				return
			}

			server.markNotificationProcessed(ctx, notification.ID, resource.MerchantTradeNo, resource.TransactionID)
			writeWechatNotifySuccess(ctx, "settlement")
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
			writeWechatNotifySuccess(ctx, "settlement")
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

	if po.Status != "paid" || !db.PaymentOrderUsesEcommerceChannel(po) || !db.PaymentOrderRequiresProfitSharing(po) {
		log.Info().
			Int64("payment_order_id", po.ID).
			Str("status", po.Status).
			Str("payment_type", po.PaymentType).
			Str("payment_channel", po.PaymentChannel).
			Msg("settlement: payment order not eligible for profit sharing, skip")
		server.markNotificationProcessed(ctx, notification.ID, subOrder.OutTradeNo, resource.TransactionID)
		writeWechatNotifySuccess(ctx, "settlement")
		return
	}

	// 通知ID已在 tryClaimNotification 中原子写入，无需重复记录

	if !server.requireTaskDistributorForNotification(ctx, notification.ID, "settlement", "profit sharing dispatch unavailable, please retry") {
		return
	}

	// 📤 派发分账任务
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
		server.releaseNotification(ctx, notification.ID, "settlement")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{Code: "FAIL", Message: "enqueue failed, please retry"})
		return
	}

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

	server.markNotificationProcessed(ctx, notification.ID, subOrder.OutTradeNo, resource.TransactionID)
	writeWechatNotifySuccess(ctx, "settlement")
}
