package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/websocket"
	"github.com/merrydance/locallife/wechat"
	"github.com/merrydance/locallife/worker"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
)

// ==================== 微信支付回调 ====================

// wechatPaymentNotifyResponse 微信支付回调响应
type wechatPaymentNotifyResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
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
	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		log.Error().Err(err).Msg("read payment notification body")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{
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
		log.Error().Err(err).Str("body", string(body)).Msg("parse payment notification")
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

	// 🔐 P0-2: 幂等性检查 - 检查通知ID是否已处理
	exists, err := server.store.CheckNotificationExists(ctx, notification.ID)
	if err != nil {
		log.Error().Err(err).Str("notification_id", notification.ID).Msg("check notification existence")
		// 查询失败不应拒绝通知，继续处理
	} else if exists {
		log.Info().Str("notification_id", notification.ID).Msg("notification already processed, return success")
		ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{
			Code:    "SUCCESS",
			Message: "OK",
		})
		return
	}

	// 解密通知内容
	resource, err := server.paymentClient.DecryptPaymentNotification(&notification)
	if err != nil {
		log.Error().Err(err).Msg("decrypt payment notification")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "decrypt failed",
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
		if errors.Is(err, pgx.ErrNoRows) {
			log.Error().Str("out_trade_no", resource.OutTradeNo).Msg("payment order not found")
			// 订单不存在，返回成功避免微信重试
			ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{
				Code:    "SUCCESS",
				Message: "order not found, ignored",
			})
			return
		}
		log.Error().Err(err).Str("out_trade_no", resource.OutTradeNo).Msg("get payment order")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "internal error",
		})
		return
	}

	// 检查是否已处理（幂等）
	if paymentOrder.Status == PaymentStatusPaid {
		log.Info().Int64("id", paymentOrder.ID).Msg("payment order already paid")
		ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{
			Code:    "SUCCESS",
			Message: "OK",
		})
		return
	}

	// ✅ P0-2: 验证支付金额是否匹配
	if resource.Amount.Total != paymentOrder.Amount {
		log.Error().
			Int64("expected_amount", paymentOrder.Amount).
			Int64("actual_amount", resource.Amount.Total).
			Str("out_trade_no", resource.OutTradeNo).
			Msg("⚠️ payment amount mismatch - possible attack or system error")
		// 返回成功避免微信重试，但标记需要人工审核
		ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{
			Code:    "SUCCESS",
			Message: "amount mismatch, manual review required",
		})
		return
	}

	// 更新支付订单状态为已支付（这是核心操作，必须同步完成）
	updatedPaymentOrder, err := server.store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            paymentOrder.ID,
		TransactionID: pgtype.Text{String: resource.TransactionID, Valid: true},
	})
	if err != nil {
		log.Error().Err(err).Int64("id", paymentOrder.ID).Msg("update payment order to paid")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "update order failed",
		})
		return
	}

	// 📢 M14: 异步发送支付成功通知（避免阻塞微信回调响应）
	if server.taskDistributor != nil {
		expiresAt := time.Now().Add(7 * 24 * time.Hour)
		_ = server.taskDistributor.DistributeTaskSendNotification(
			ctx,
			&worker.SendNotificationPayload{
				UserID:      updatedPaymentOrder.UserID,
				Type:        "payment",
				Title:       "支付成功",
				Content:     fmt.Sprintf("您的订单支付已完成，支付金额%.2f元", float64(updatedPaymentOrder.Amount)/100),
				RelatedType: "payment",
				RelatedID:   updatedPaymentOrder.ID,
				ExtraData: map[string]any{
					"out_trade_no":   updatedPaymentOrder.OutTradeNo,
					"transaction_id": resource.TransactionID,
					"amount":         updatedPaymentOrder.Amount,
					"business_type":  updatedPaymentOrder.BusinessType,
				},
				ExpiresAt: &expiresAt,
			},
			asynq.Queue(worker.QueueDefault),
		)
	}

	// 🔐 P0-2: 记录通知ID，防止重复处理
	_, err = server.store.CreateWechatNotification(ctx, db.CreateWechatNotificationParams{
		ID:            notification.ID,
		EventType:     notification.EventType,
		ResourceType:  pgtype.Text{String: notification.ResourceType, Valid: true},
		Summary:       pgtype.Text{String: notification.Summary, Valid: true},
		OutTradeNo:    pgtype.Text{String: resource.OutTradeNo, Valid: true},
		TransactionID: pgtype.Text{String: resource.TransactionID, Valid: true},
	})
	if err != nil {
		log.Error().Err(err).Str("notification_id", notification.ID).Msg("⚠️ failed to record notification ID")
		// 记录失败不影响业务处理，继续
	}

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
			if server.wsHub != nil {
				server.wsHub.SendAlert(websocket.AlertData{
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
		}
	} else {
		log.Warn().Msg("task distributor not configured, skip async processing")
	}

	// 快速返回成功
	ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{
		Code:    "SUCCESS",
		Message: "OK",
	})
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
	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		log.Error().Err(err).Msg("read refund notification body")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{
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

	// 🔐 P0-2: 幂等性检查
	exists, err := server.store.CheckNotificationExists(ctx, notification.ID)
	if err != nil {
		log.Error().Err(err).Str("notification_id", notification.ID).Msg("check notification existence")
	} else if exists {
		log.Info().Str("notification_id", notification.ID).Msg("refund notification already processed")
		ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{
			Code:    "SUCCESS",
			Message: "OK",
		})
		return
	}

	// 解密通知内容
	resource, err := server.paymentClient.DecryptRefundNotification(&notification)
	if err != nil {
		log.Error().Err(err).Msg("decrypt refund notification")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "decrypt failed",
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
			log.Error().Err(err).
				Str("out_refund_no", resource.OutRefundNo).
				Msg("failed to enqueue refund result task")
		}
	}

	// 🔐 P0-2: 记录通知ID
	_, err = server.store.CreateWechatNotification(ctx, db.CreateWechatNotificationParams{
		ID:            notification.ID,
		EventType:     notification.EventType,
		ResourceType:  pgtype.Text{String: notification.ResourceType, Valid: true},
		Summary:       pgtype.Text{String: notification.Summary, Valid: true},
		OutTradeNo:    pgtype.Text{String: resource.OutRefundNo, Valid: true},
		TransactionID: pgtype.Text{String: resource.RefundID, Valid: true},
	})
	if err != nil {
		log.Warn().Err(err).Str("notification_id", notification.ID).Msg("failed to record notification")
	}

	// 快速返回成功
	ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{
		Code:    "SUCCESS",
		Message: "OK",
	})
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
	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		log.Error().Err(err).Msg("read ecommerce refund notification body")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{
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

	// 🔐 P0-2: 幂等性检查
	exists, err := server.store.CheckNotificationExists(ctx, notification.ID)
	if err != nil {
		log.Error().Err(err).Str("notification_id", notification.ID).Msg("check notification existence")
	} else if exists {
		log.Info().Str("notification_id", notification.ID).Msg("ecommerce refund notification already processed")
		ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{
			Code:    "SUCCESS",
			Message: "OK",
		})
		return
	}

	// 解密通知内容（使用平台收付通专用解密方法）
	resource, err := server.ecommerceClient.DecryptEcommerceRefundNotification(&notification)
	if err != nil {
		log.Error().Err(err).Msg("decrypt ecommerce refund notification")
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
			log.Error().Err(err).
				Str("out_refund_no", resource.OutRefundNo).
				Str("sp_mchid", resource.SpMchID).
				Str("sub_mchid", resource.SubMchID).
				Msg("⚠️ CRITICAL: failed to enqueue ecommerce refund result task - manual intervention may be required")
		}
	}

	// 🔐 P0-2: 记录通知ID
	_, err = server.store.CreateWechatNotification(ctx, db.CreateWechatNotificationParams{
		ID:            notification.ID,
		EventType:     notification.EventType,
		ResourceType:  pgtype.Text{String: notification.ResourceType, Valid: true},
		Summary:       pgtype.Text{String: notification.Summary, Valid: true},
		OutTradeNo:    pgtype.Text{String: resource.OutRefundNo, Valid: true},
		TransactionID: pgtype.Text{String: resource.RefundID, Valid: true},
	})
	if err != nil {
		log.Warn().Err(err).Str("notification_id", notification.ID).Msg("failed to record notification")
	}

	// 快速返回成功
	ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{
		Code:    "SUCCESS",
		Message: "OK",
	})
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
	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		log.Error().Err(err).Msg("read profit sharing notification body")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{
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

	// 🔐 P0-2: 幂等性检查
	exists, err := server.store.CheckNotificationExists(ctx, notification.ID)
	if err != nil {
		log.Error().Err(err).Str("notification_id", notification.ID).Msg("check notification existence")
	} else if exists {
		log.Info().Str("notification_id", notification.ID).Msg("profit sharing notification already processed")
		ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{
			Code:    "SUCCESS",
			Message: "OK",
		})
		return
	}

	// 解密通知内容
	resource, err := server.ecommerceClient.DecryptProfitSharingNotification(&notification)
	if err != nil {
		log.Error().Err(err).Msg("decrypt profit sharing notification")
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

	// 查询分账订单
	profitSharingOrder, err := server.store.GetProfitSharingOrderByOutOrderNo(ctx, resource.OutOrderNo)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Error().Str("out_order_no", resource.OutOrderNo).Msg("profit sharing order not found")
			// 订单不存在，返回成功避免微信重试
			ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{
				Code:    "SUCCESS",
				Message: "order not found, ignored",
			})
			return
		}
		log.Error().Err(err).Str("out_order_no", resource.OutOrderNo).Msg("get profit sharing order")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "internal error",
		})
		return
	}

	// 检查是否已处理（幂等）
	if profitSharingOrder.Status == "finished" || profitSharingOrder.Status == "failed" {
		log.Info().Int64("id", profitSharingOrder.ID).Str("status", profitSharingOrder.Status).Msg("profit sharing order already processed")
		ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{
			Code:    "SUCCESS",
			Message: "OK",
		})
		return
	}

	// 根据分账结果更新订单状态
	switch resource.Receiver.Result {
	case "SUCCESS":
		_, err = server.store.UpdateProfitSharingOrderToFinished(ctx, profitSharingOrder.ID)
		if err != nil {
			log.Error().Err(err).Int64("id", profitSharingOrder.ID).Msg("update profit sharing order to finished")
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

	case "CLOSED", "FAILED":
		_, err = server.store.UpdateProfitSharingOrderToFailed(ctx, profitSharingOrder.ID)
		if err != nil {
			log.Error().Err(err).Int64("id", profitSharingOrder.ID).Msg("update profit sharing order to failed")
		}
		log.Error().
			Int64("profit_sharing_order_id", profitSharingOrder.ID).
			Str("out_order_no", resource.OutOrderNo).
			Str("result", resource.Receiver.Result).
			Str("fail_reason", resource.Receiver.FailReason).
			Msg("⚠️ profit sharing failed - manual review required")

	default:
		log.Warn().
			Str("result", resource.Receiver.Result).
			Str("out_order_no", resource.OutOrderNo).
			Msg("unknown profit sharing result")
	}

	// 🔐 P0-2: 记录通知ID
	_, err = server.store.CreateWechatNotification(ctx, db.CreateWechatNotificationParams{
		ID:            notification.ID,
		EventType:     notification.EventType,
		ResourceType:  pgtype.Text{String: notification.ResourceType, Valid: true},
		Summary:       pgtype.Text{String: notification.Summary, Valid: true},
		OutTradeNo:    pgtype.Text{String: resource.OutOrderNo, Valid: true},
		TransactionID: pgtype.Text{String: resource.OrderID, Valid: true},
	})
	if err != nil {
		log.Warn().Err(err).Str("notification_id", notification.ID).Msg("failed to record profit sharing notification")
	}

	// 📤 异步处理：发送分账结果通知
	if server.taskDistributor != nil {
		_ = server.taskDistributor.DistributeTaskProcessProfitSharingResult(
			ctx,
			&worker.ProfitSharingResultPayload{
				ProfitSharingOrderID: profitSharingOrder.ID,
				OutOrderNo:           resource.OutOrderNo,
				Result:               resource.Receiver.Result,
				FailReason:           resource.Receiver.FailReason,
				MerchantID:           profitSharingOrder.MerchantID,
			},
			asynq.MaxRetry(3),
			asynq.Queue(worker.QueueDefault),
		)
	}

	// 快速返回成功
	ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{
		Code:    "SUCCESS",
		Message: "OK",
	})
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
	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		log.Error().Err(err).Msg("read combine payment notification body")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{
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

	// 🔐 P0-2: 幂等性检查
	exists, err := server.store.CheckNotificationExists(ctx, notification.ID)
	if err != nil {
		log.Error().Err(err).Str("notification_id", notification.ID).Msg("check notification existence")
	} else if exists {
		log.Info().Str("notification_id", notification.ID).Msg("combine payment notification already processed")
		ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{
			Code:    "SUCCESS",
			Message: "OK",
		})
		return
	}

	// 解密通知内容
	resource, err := server.ecommerceClient.DecryptCombinePaymentNotification(&notification)
	if err != nil {
		log.Error().Err(err).Msg("decrypt combine payment notification")
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

	// ✅ P0-1: 修复合单支付错误处理
	var failedOrders []string
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
			if errors.Is(err, pgx.ErrNoRows) {
				log.Error().Str("out_trade_no", subOrder.OutTradeNo).Msg("payment order not found")
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
			_ = server.taskDistributor.DistributeTaskProcessPaymentSuccess(
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
		}
	}

	// 如果有失败的订单，返回FAIL触发微信重试
	if len(failedOrders) > 0 {
		log.Error().
			Strs("failed_orders", failedOrders).
			Int("success_count", successCount).
			Int("failed_count", len(failedOrders)).
			Msg("some sub orders failed to process")

		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: fmt.Sprintf("%d orders failed", len(failedOrders)),
		})
		return
	}

	log.Info().
		Int("success_count", successCount).
		Msg("all sub orders processed successfully")

	// 🔐 P0-2: 记录通知ID
	_, err = server.store.CreateWechatNotification(ctx, db.CreateWechatNotificationParams{
		ID:            notification.ID,
		EventType:     notification.EventType,
		ResourceType:  pgtype.Text{String: notification.ResourceType, Valid: true},
		Summary:       pgtype.Text{String: notification.Summary, Valid: true},
		OutTradeNo:    pgtype.Text{String: resource.CombineOutTradeNo, Valid: true},
		TransactionID: pgtype.Text{Valid: false}, // 合单支付没有单一transaction_id
	})
	if err != nil {
		log.Warn().Err(err).Str("notification_id", notification.ID).Msg("failed to record notification")
	}

	ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{
		Code:    "SUCCESS",
		Message: "OK",
	})
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
	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		log.Error().Err(err).Msg("read applyment notification body")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{
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
		log.Error().Err(err).Str("body", string(body)).Msg("parse applyment notification")
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

	// 🔐 P0-2: 幂等性检查 - 检查通知ID是否已处理
	exists, err := server.store.CheckNotificationExists(ctx, notification.ID)
	if err != nil {
		log.Error().Err(err).Str("notification_id", notification.ID).Msg("check notification existence")
		// 查询失败不应拒绝通知，继续处理
	} else if exists {
		log.Info().Str("notification_id", notification.ID).Msg("applyment notification already processed, return success")
		ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{
			Code:    "SUCCESS",
			Message: "OK",
		})
		return
	}

	// 构造 PaymentNotification 结构以复用现有解密逻辑
	var paymentNotification wechat.PaymentNotification
	if err := json.Unmarshal(body, &paymentNotification); err != nil {
		log.Error().Err(err).Str("body", string(body)).Msg("parse payment notification for decryption")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "parse notification failed",
		})
		return
	}

	// 使用 paymentClient 解密
	if server.paymentClient == nil {
		log.Error().Msg("payment client not configured for decryption")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "payment client not configured",
		})
		return
	}

	decrypted, err := server.paymentClient.DecryptNotificationRaw(&paymentNotification)
	if err != nil {
		log.Error().Err(err).Msg("decrypt applyment notification")
		ctx.JSON(http.StatusBadRequest, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "decrypt failed",
		})
		return
	}

	var resource ApplymentStateChangeResource
	if err := json.Unmarshal(decrypted, &resource); err != nil {
		log.Error().Err(err).Str("decrypted", string(decrypted)).Msg("parse applyment resource")
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
		if errors.Is(err, pgx.ErrNoRows) {
			log.Warn().Str("out_request_no", resource.OutRequestNo).Msg("applyment record not found")
			ctx.JSON(http.StatusOK, wechatPaymentNotifyResponse{
				Code:    "SUCCESS",
				Message: "OK",
			})
			return
		}
		log.Error().Err(err).Msg("get applyment record")
		ctx.JSON(http.StatusInternalServerError, wechatPaymentNotifyResponse{
			Code:    "FAIL",
			Message: "internal error",
		})
		return
	}

	// 映射状态
	newStatus := mapApplymentStateToDBStatus(resource.ApplymentState)

	// 如果有二级商户号，说明开户成功
	if resource.SubMchID != "" {
		// 更新进件记录
		_, err = server.store.UpdateEcommerceApplymentSubMchID(ctx, db.UpdateEcommerceApplymentSubMchIDParams{
			ID:       applyment.ID,
			SubMchID: pgtype.Text{String: resource.SubMchID, Valid: true},
		})
		if err != nil {
			log.Error().Err(err).Int64("applyment_id", applyment.ID).Msg("update applyment sub_mch_id")
		}

		// 根据主体类型更新对应的二级商户号
		switch applyment.SubjectType {
		case "merchant":
			// 更新商户的支付配置
			_, err = server.store.UpdateMerchantPaymentConfig(ctx, db.UpdateMerchantPaymentConfigParams{
				MerchantID: applyment.SubjectID,
				SubMchID:   pgtype.Text{String: resource.SubMchID, Valid: true},
				Status:     pgtype.Text{String: "active", Valid: true},
			})
			if err != nil {
				log.Error().Err(err).Int64("merchant_id", applyment.SubjectID).Msg("update merchant payment config sub_mch_id")
			}
			// 更新商户状态为 active
			_, err = server.store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{
				ID:     applyment.SubjectID,
				Status: "active",
			})
			if err != nil {
				log.Error().Err(err).Int64("merchant_id", applyment.SubjectID).Msg("update merchant status")
			}
		case "rider":
			// 更新骑手的二级商户号
			_, err = server.store.UpdateRiderSubMchID(ctx, db.UpdateRiderSubMchIDParams{
				ID:       applyment.SubjectID,
				SubMchID: pgtype.Text{String: resource.SubMchID, Valid: true},
			})
			if err != nil {
				log.Error().Err(err).Int64("rider_id", applyment.SubjectID).Msg("update rider sub_mch_id")
			}
			// 更新骑手状态为 active
			_, err = server.store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{
				ID:     applyment.SubjectID,
				Status: "active",
			})
			if err != nil {
				log.Error().Err(err).Int64("rider_id", applyment.SubjectID).Msg("update rider status")
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
	}

	// 🔐 P0-2: 记录通知ID，防止重复处理
	_, err = server.store.CreateWechatNotification(ctx, db.CreateWechatNotificationParams{
		ID:            notification.ID,
		EventType:     notification.EventType,
		ResourceType:  pgtype.Text{String: notification.ResourceType, Valid: true},
		Summary:       pgtype.Text{String: notification.Summary, Valid: true},
		OutTradeNo:    pgtype.Text{String: resource.OutRequestNo, Valid: true},
		TransactionID: pgtype.Text{Valid: false},
	})
	if err != nil {
		log.Warn().Err(err).Str("notification_id", notification.ID).Msg("failed to record applyment notification")
	}

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
