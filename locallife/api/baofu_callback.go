package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	baofunotification "github.com/merrydance/locallife/baofu/account/notification"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	baofuaggregatenotification "github.com/merrydance/locallife/baofu/aggregatepay/notification"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/worker"
	"github.com/rs/zerolog/log"
)

type baofuAccountNotificationParser interface {
	ParseOpenAccountNotification(body []byte) (*baofunotification.AccountNotification, error)
	ParseWithdrawNotification(body []byte) (*baofunotification.WithdrawNotification, error)
}

type baofuAggregatePaymentNotificationParser interface {
	ParsePaymentNotification(body []byte) (*baofuaggregatenotification.PaymentNotification, error)
	ParseShareNotification(body []byte) (*baofuaggregatenotification.ShareNotification, error)
	ParseRefundNotification(body []byte) (*baofuaggregatenotification.RefundNotification, error)
}

type baofuCallbackResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (server *Server) SetBaofuAccountNotificationParserForTest(parser baofuAccountNotificationParser) {
	server.baofuAccountNotificationParser = parser
}

func (server *Server) SetBaofuAggregatePaymentNotificationParserForTest(parser baofuAggregatePaymentNotificationParser) {
	server.baofuPaymentNotificationParser = parser
}

func (server *Server) handleBaofuAccountOpenNotify(ctx *gin.Context) {
	if server.baofuAccountNotificationParser == nil {
		log.Error().Msg("baofu account callback received but parser is not configured")
		ctx.JSON(http.StatusServiceUnavailable, baofuCallbackResponse{Code: "FAIL", Message: "baofu account callback service unavailable"})
		return
	}
	body, status, err := readWebhookBody(ctx)
	if err != nil {
		log.Error().Err(err).Msg("read baofu account callback body failed")
		ctx.JSON(status, baofuCallbackResponse{Code: "FAIL", Message: "read callback body failed"})
		return
	}
	body = baofuAccountCallbackPayload(ctx, body)
	notification, err := server.baofuAccountNotificationParser.ParseOpenAccountNotification(body)
	if err != nil {
		log.Error().Err(err).Msg("parse baofu account callback failed")
		ctx.JSON(http.StatusUnauthorized, baofuCallbackResponse{Code: "FAIL", Message: "callback verification failed"})
		return
	}
	if notification == nil {
		log.Error().Msg("parse baofu account callback returned empty notification")
		ctx.JSON(http.StatusBadRequest, baofuCallbackResponse{Code: "FAIL", Message: "callback content invalid"})
		return
	}
	if err := server.validateBaofuCollectNotificationIdentity(notification.MemberID, notification.TerminalID); err != nil {
		log.Error().Err(err).Msg("baofu account callback identity mismatch")
		ctx.JSON(http.StatusUnauthorized, baofuCallbackResponse{Code: "FAIL", Message: "callback verification failed"})
		return
	}
	fact, err := server.recordBaofuAccountOpenCallbackFact(ctx.Request.Context(), notification)
	if err != nil {
		log.Error().Err(err).
			Str("out_request_no", strings.TrimSpace(notification.OutRequestNo)).
			Str("contract_no_mask", maskedBaofuIdentifier(notification.ContractNo)).
			Msg("persist baofu account callback fact failed")
		ctx.JSON(http.StatusInternalServerError, baofuCallbackResponse{Code: "FAIL", Message: "persist callback failed"})
		return
	}
	log.Info().
		Int64("payment_fact_id", fact.ID).
		Str("out_request_no", strings.TrimSpace(notification.OutRequestNo)).
		Str("baofu_open_state", strings.TrimSpace(notification.OpenState)).
		Msg("baofu account callback fact persisted")
	ctx.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(baofunotification.AccountNotificationACK()))
}

func (server *Server) handleBaofuWithdrawNotify(ctx *gin.Context) {
	if server.baofuAccountNotificationParser == nil {
		log.Error().Msg("baofu withdraw callback received but parser is not configured")
		ctx.JSON(http.StatusServiceUnavailable, baofuCallbackResponse{Code: "FAIL", Message: "baofu withdraw callback service unavailable"})
		return
	}
	if server.taskDistributor == nil {
		log.Error().Msg("baofu withdraw callback received but task distributor is not configured")
		ctx.JSON(http.StatusServiceUnavailable, baofuCallbackResponse{Code: "FAIL", Message: "baofu withdraw callback service unavailable"})
		return
	}
	body, status, err := readWebhookBody(ctx)
	if err != nil {
		log.Error().Err(err).Msg("read baofu withdraw callback body failed")
		ctx.JSON(status, baofuCallbackResponse{Code: "FAIL", Message: "read callback body failed"})
		return
	}
	body = baofuAccountCallbackPayload(ctx, body)
	notification, err := server.baofuAccountNotificationParser.ParseWithdrawNotification(body)
	if err != nil {
		log.Error().Err(err).Msg("parse baofu withdraw callback failed")
		ctx.JSON(http.StatusUnauthorized, baofuCallbackResponse{Code: "FAIL", Message: "callback verification failed"})
		return
	}
	if notification == nil {
		log.Error().Msg("parse baofu withdraw callback returned empty notification")
		ctx.JSON(http.StatusBadRequest, baofuCallbackResponse{Code: "FAIL", Message: "callback content invalid"})
		return
	}
	if err := server.validateBaofuPayoutNotificationIdentity(notification.MemberID, notification.TerminalID); err != nil {
		log.Error().Err(err).Msg("baofu withdraw callback identity mismatch")
		ctx.JSON(http.StatusUnauthorized, baofuCallbackResponse{Code: "FAIL", Message: "callback verification failed"})
		return
	}
	outRequestNo := strings.TrimSpace(notification.TransSerialNo)
	if outRequestNo == "" {
		log.Error().Msg("baofu withdraw callback missing trans serial no")
		ctx.JSON(http.StatusBadRequest, baofuCallbackResponse{Code: "FAIL", Message: "callback content invalid"})
		return
	}
	withdrawalOrder, err := server.store.GetBaofuWithdrawalOrderByOutRequestNo(ctx.Request.Context(), outRequestNo)
	if err != nil {
		log.Error().Err(err).Str("out_request_no", outRequestNo).Msg("load baofu withdrawal order for callback failed")
		ctx.JSON(http.StatusInternalServerError, baofuCallbackResponse{Code: "FAIL", Message: "persist callback failed"})
		return
	}
	if err := server.taskDistributor.DistributeTaskProcessBaofuWithdrawalFactApplication(ctx.Request.Context(), &worker.BaofuWithdrawalFactApplicationPayload{
		WithdrawalOrderID: withdrawalOrder.ID,
		UpstreamState:     strings.TrimSpace(notification.UpstreamState),
		BaofuWithdrawNo:   strings.TrimSpace(notification.BaofuWithdrawNo),
		RawSnapshot:       notification.Raw,
	}, asynq.MaxRetry(5), asynq.Queue(worker.QueueCritical), asynq.Unique(30*time.Second)); err != nil {
		log.Error().Err(err).Int64("baofu_withdrawal_order_id", withdrawalOrder.ID).Str("out_request_no", outRequestNo).Msg("enqueue baofu withdrawal callback fact application failed")
		ctx.JSON(http.StatusInternalServerError, baofuCallbackResponse{Code: "FAIL", Message: "persist callback failed"})
		return
	}
	log.Info().
		Int64("baofu_withdrawal_order_id", withdrawalOrder.ID).
		Str("out_request_no", outRequestNo).
		Str("baofu_withdraw_state", strings.TrimSpace(notification.UpstreamState)).
		Msg("baofu withdraw callback fact application enqueued")
	ctx.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(baofunotification.AccountNotificationACK()))
}

func baofuAccountCallbackPayload(ctx *gin.Context, body []byte) []byte {
	if strings.TrimSpace(string(body)) != "" {
		return body
	}
	if ctx == nil || ctx.Request == nil || ctx.Request.URL == nil {
		return body
	}
	if rawQuery := strings.TrimSpace(ctx.Request.URL.RawQuery); rawQuery != "" {
		return []byte(rawQuery)
	}
	return body
}

func (server *Server) handleBaofuPaymentNotify(ctx *gin.Context) {
	if server.baofuPaymentNotificationParser == nil {
		log.Error().Msg("baofu payment callback received but parser is not configured")
		ctx.JSON(http.StatusServiceUnavailable, baofuCallbackResponse{Code: "FAIL", Message: "baofu payment callback service unavailable"})
		return
	}
	body, status, err := readWebhookBody(ctx)
	if err != nil {
		log.Error().Err(err).Msg("read baofu payment callback body failed")
		ctx.JSON(status, baofuCallbackResponse{Code: "FAIL", Message: "read callback body failed"})
		return
	}
	notification, err := server.baofuPaymentNotificationParser.ParsePaymentNotification(body)
	if err != nil {
		log.Error().Err(err).Msg("parse baofu payment callback failed")
		ctx.JSON(http.StatusUnauthorized, baofuCallbackResponse{Code: "FAIL", Message: "callback verification failed"})
		return
	}
	if notification == nil {
		log.Error().Msg("parse baofu payment callback returned empty notification")
		ctx.JSON(http.StatusBadRequest, baofuCallbackResponse{Code: "FAIL", Message: "callback content invalid"})
		return
	}
	if err := server.validateBaofuCollectCallbackIdentityPresent(notification.Fact.MerchantID, notification.Fact.TerminalID); err != nil {
		log.Error().Err(err).Msg("baofu payment callback identity mismatch")
		ctx.JSON(http.StatusUnauthorized, baofuCallbackResponse{Code: "FAIL", Message: "callback verification failed"})
		return
	}
	paymentOrder, err := server.loadBaofuPaymentOrderForCallback(ctx.Request.Context(), notification)
	if err != nil {
		log.Error().Err(err).
			Str("out_trade_no", strings.TrimSpace(notification.Fact.OutTradeNo)).
			Str("baofu_trade_no", strings.TrimSpace(notification.Fact.TradeNo)).
			Msg("load baofu payment order for callback failed")
		ctx.JSON(http.StatusInternalServerError, baofuCallbackResponse{Code: "FAIL", Message: "persist callback failed"})
		return
	}
	service := logic.NewBaofuPaymentService(server.store, nil, logic.BaofuPaymentServiceConfig{})
	result, err := service.RecordPaymentFact(ctx.Request.Context(), logic.RecordBaofuPaymentFactInput{
		PaymentOrder:    paymentOrder,
		Fact:            notification.Fact,
		FactSource:      db.ExternalPaymentFactSourceCallback,
		SourceEventID:   strings.TrimSpace(notification.NotifyID),
		SourceEventType: strings.TrimSpace(notification.NotifyType),
		OccurredAt:      notification.OccurredAt,
	})
	if err != nil {
		log.Error().Err(err).Str("out_trade_no", strings.TrimSpace(notification.Fact.OutTradeNo)).Msg("persist baofu payment callback fact failed")
		ctx.JSON(http.StatusInternalServerError, baofuCallbackResponse{Code: "FAIL", Message: "persist callback failed"})
		return
	}
	if err := server.enqueueOrderPaymentFactApplication(ctx.Request.Context(), result.Application); err != nil {
		log.Warn().Err(err).Int64("payment_fact_id", result.Fact.ID).Msg("enqueue baofu payment fact application failed; scheduler will retry")
	}
	log.Info().
		Int64("payment_fact_id", result.Fact.ID).
		Str("out_trade_no", strings.TrimSpace(notification.Fact.OutTradeNo)).
		Str("baofu_payment_state", strings.TrimSpace(notification.Fact.TransactionState)).
		Msg("baofu payment callback fact persisted")
	ctx.Data(http.StatusOK, "text/plain; charset=utf-8", []byte("OK"))
}

func (server *Server) loadBaofuPaymentOrderForCallback(ctx context.Context, notification *baofuaggregatenotification.PaymentNotification) (db.PaymentOrder, error) {
	if notification == nil {
		return db.PaymentOrder{}, fmt.Errorf("baofu payment callback notification is required")
	}
	if outTradeNo := strings.TrimSpace(notification.Fact.OutTradeNo); outTradeNo != "" {
		return server.store.GetPaymentOrderByOutTradeNo(ctx, outTradeNo)
	}
	if tradeNo := strings.TrimSpace(notification.Fact.TradeNo); tradeNo != "" {
		paymentOrder, err := server.store.GetPaymentOrderByTransactionId(ctx, pgtype.Text{String: tradeNo, Valid: true})
		if err == nil || !errors.Is(err, db.ErrRecordNotFound) {
			return paymentOrder, err
		}
		outTradeNo, queryErr := server.queryBaofuPaymentOutTradeNoForCallback(ctx, notification)
		if queryErr != nil {
			return db.PaymentOrder{}, fmt.Errorf("query baofu payment callback tradeNo %q: %w", tradeNo, queryErr)
		}
		if strings.TrimSpace(outTradeNo) != "" {
			return server.store.GetPaymentOrderByOutTradeNo(ctx, outTradeNo)
		}
		return db.PaymentOrder{}, err
	}
	return db.PaymentOrder{}, fmt.Errorf("baofu payment callback outTradeNo or tradeNo is required")
}

func (server *Server) queryBaofuPaymentOutTradeNoForCallback(ctx context.Context, notification *baofuaggregatenotification.PaymentNotification) (string, error) {
	if server.baofuAggregateClient == nil || notification == nil {
		return "", db.ErrRecordNotFound
	}
	tradeNo := strings.TrimSpace(notification.Fact.TradeNo)
	if tradeNo == "" {
		return "", db.ErrRecordNotFound
	}
	merchantID, terminalID, err := server.baofuCollectIdentityForCallback(notification.Fact.MerchantID, notification.Fact.TerminalID)
	if err != nil {
		return "", err
	}
	queryResult, err := server.baofuAggregateClient.QueryPayment(ctx, aggregatecontracts.PaymentQueryRequest{
		MerchantID: merchantID,
		TerminalID: terminalID,
		TradeNo:    tradeNo,
	})
	if err != nil {
		return "", err
	}
	if queryResult == nil {
		return "", db.ErrRecordNotFound
	}
	return strings.TrimSpace(queryResult.OutTradeNo), nil
}

func (server *Server) handleBaofuShareNotify(ctx *gin.Context) {
	if server.baofuPaymentNotificationParser == nil {
		log.Error().Msg("baofu share callback received but parser is not configured")
		ctx.JSON(http.StatusServiceUnavailable, baofuCallbackResponse{Code: "FAIL", Message: "baofu share callback service unavailable"})
		return
	}
	body, status, err := readWebhookBody(ctx)
	if err != nil {
		log.Error().Err(err).Msg("read baofu share callback body failed")
		ctx.JSON(status, baofuCallbackResponse{Code: "FAIL", Message: "read callback body failed"})
		return
	}
	notification, err := server.baofuPaymentNotificationParser.ParseShareNotification(body)
	if err != nil {
		log.Error().Err(err).Msg("parse baofu share callback failed")
		ctx.JSON(http.StatusUnauthorized, baofuCallbackResponse{Code: "FAIL", Message: "callback verification failed"})
		return
	}
	if notification == nil {
		log.Error().Msg("parse baofu share callback returned empty notification")
		ctx.JSON(http.StatusBadRequest, baofuCallbackResponse{Code: "FAIL", Message: "callback content invalid"})
		return
	}
	if err := server.validateBaofuCollectCallbackIdentityPresent(notification.Fact.MerchantID, notification.Fact.TerminalID); err != nil {
		log.Error().Err(err).Msg("baofu share callback identity mismatch")
		ctx.JSON(http.StatusUnauthorized, baofuCallbackResponse{Code: "FAIL", Message: "callback verification failed"})
		return
	}
	profitSharingOrder, err := server.loadBaofuProfitSharingOrderForCallback(ctx.Request.Context(), notification)
	if err != nil {
		log.Error().Err(err).Str("out_order_no", strings.TrimSpace(notification.Fact.OutTradeNo)).Msg("load baofu profit sharing order for callback failed")
		ctx.JSON(http.StatusInternalServerError, baofuCallbackResponse{Code: "FAIL", Message: "persist callback failed"})
		return
	}
	service := logic.NewBaofuProfitSharingService(server.store)
	result, err := service.RecordShareFact(ctx.Request.Context(), logic.RecordBaofuShareFactInput{
		ProfitSharingOrder: profitSharingOrder,
		Fact:               notification.Fact,
		FactSource:         db.ExternalPaymentFactSourceCallback,
		SourceEventID:      strings.TrimSpace(notification.NotifyID),
		SourceEventType:    strings.TrimSpace(notification.NotifyType),
		OccurredAt:         notification.OccurredAt,
	})
	if err != nil {
		log.Error().Err(err).Str("out_order_no", strings.TrimSpace(notification.Fact.OutTradeNo)).Msg("persist baofu share callback fact failed")
		ctx.JSON(http.StatusInternalServerError, baofuCallbackResponse{Code: "FAIL", Message: "persist callback failed"})
		return
	}
	if err := server.enqueueOrderPaymentFactApplication(ctx.Request.Context(), result.Application); err != nil {
		log.Warn().Err(err).Int64("payment_fact_id", result.Fact.ID).Msg("enqueue baofu share fact application failed; scheduler will retry")
	}
	log.Info().
		Int64("payment_fact_id", result.Fact.ID).
		Str("out_order_no", strings.TrimSpace(notification.Fact.OutTradeNo)).
		Str("baofu_share_state", strings.TrimSpace(notification.Fact.TransactionState)).
		Msg("baofu share callback fact persisted")
	ctx.Data(http.StatusOK, "text/plain; charset=utf-8", []byte("OK"))
}

func (server *Server) loadBaofuProfitSharingOrderForCallback(ctx context.Context, notification *baofuaggregatenotification.ShareNotification) (db.ProfitSharingOrder, error) {
	if notification == nil {
		return db.ProfitSharingOrder{}, fmt.Errorf("baofu share callback notification is required")
	}
	if outOrderNo := strings.TrimSpace(notification.Fact.OutTradeNo); outOrderNo != "" {
		return server.store.GetProfitSharingOrderByOutOrderNo(ctx, outOrderNo)
	}
	tradeNo := strings.TrimSpace(notification.Fact.TradeNo)
	if tradeNo == "" {
		return db.ProfitSharingOrder{}, fmt.Errorf("baofu share callback outTradeNo or tradeNo is required")
	}
	outOrderNo, err := server.queryBaofuShareOutOrderNoForCallback(ctx, notification)
	if err != nil {
		return db.ProfitSharingOrder{}, fmt.Errorf("query baofu share callback tradeNo %q: %w", tradeNo, err)
	}
	if strings.TrimSpace(outOrderNo) == "" {
		return db.ProfitSharingOrder{}, db.ErrRecordNotFound
	}
	return server.store.GetProfitSharingOrderByOutOrderNo(ctx, outOrderNo)
}

func (server *Server) queryBaofuShareOutOrderNoForCallback(ctx context.Context, notification *baofuaggregatenotification.ShareNotification) (string, error) {
	if server.baofuAggregateClient == nil || notification == nil {
		return "", db.ErrRecordNotFound
	}
	tradeNo := strings.TrimSpace(notification.Fact.TradeNo)
	if tradeNo == "" {
		return "", db.ErrRecordNotFound
	}
	merchantID, terminalID, err := server.baofuCollectIdentityForCallback(notification.Fact.MerchantID, notification.Fact.TerminalID)
	if err != nil {
		return "", err
	}
	queryResult, err := server.baofuAggregateClient.QueryProfitSharing(ctx, aggregatecontracts.ShareQueryRequest{
		MerchantID: merchantID,
		TerminalID: terminalID,
		TradeNo:    tradeNo,
	})
	if err != nil {
		return "", err
	}
	if queryResult == nil {
		return "", db.ErrRecordNotFound
	}
	return strings.TrimSpace(queryResult.OutTradeNo), nil
}

func (server *Server) baofuCollectIdentityForCallback(notificationMerchantID string, notificationTerminalID string) (string, string, error) {
	configuredMerchantID := strings.TrimSpace(server.config.BaofuCollectMerchantID)
	configuredTerminalID := strings.TrimSpace(server.config.BaofuCollectTerminalID)
	notificationMerchantID = strings.TrimSpace(notificationMerchantID)
	notificationTerminalID = strings.TrimSpace(notificationTerminalID)
	if configuredMerchantID != "" && notificationMerchantID != "" && notificationMerchantID != configuredMerchantID {
		return "", "", fmt.Errorf("baofu callback merId does not match configured collect merchant")
	}
	if configuredTerminalID != "" && notificationTerminalID != "" && notificationTerminalID != configuredTerminalID {
		return "", "", fmt.Errorf("baofu callback terId does not match configured collect terminal")
	}
	return firstNonEmptyTrimmed(configuredMerchantID, notificationMerchantID), firstNonEmptyTrimmed(configuredTerminalID, notificationTerminalID), nil
}

func (server *Server) validateBaofuCollectCallbackIdentityPresent(notificationMerchantID string, notificationTerminalID string) error {
	notificationMerchantID = strings.TrimSpace(notificationMerchantID)
	notificationTerminalID = strings.TrimSpace(notificationTerminalID)
	if notificationMerchantID == "" {
		return fmt.Errorf("baofu callback merId is required")
	}
	if notificationTerminalID == "" {
		return fmt.Errorf("baofu callback terId is required")
	}
	_, _, err := server.baofuCollectIdentityForCallback(notificationMerchantID, notificationTerminalID)
	return err
}

func (server *Server) handleBaofuRefundNotify(ctx *gin.Context) {
	if server.baofuPaymentNotificationParser == nil {
		log.Error().Msg("baofu refund callback received but parser is not configured")
		ctx.JSON(http.StatusServiceUnavailable, baofuCallbackResponse{Code: "FAIL", Message: "baofu refund callback service unavailable"})
		return
	}
	body, status, err := readWebhookBody(ctx)
	if err != nil {
		log.Error().Err(err).Msg("read baofu refund callback body failed")
		ctx.JSON(status, baofuCallbackResponse{Code: "FAIL", Message: "read callback body failed"})
		return
	}
	notification, err := server.baofuPaymentNotificationParser.ParseRefundNotification(body)
	if err != nil {
		log.Error().Err(err).Msg("parse baofu refund callback failed")
		ctx.JSON(http.StatusUnauthorized, baofuCallbackResponse{Code: "FAIL", Message: "callback verification failed"})
		return
	}
	if notification == nil {
		log.Error().Msg("parse baofu refund callback returned empty notification")
		ctx.JSON(http.StatusBadRequest, baofuCallbackResponse{Code: "FAIL", Message: "callback content invalid"})
		return
	}
	if err := server.validateBaofuCollectCallbackIdentityPresent(notification.Fact.MerchantID, notification.Fact.TerminalID); err != nil {
		log.Error().Err(err).Msg("baofu refund callback identity mismatch")
		ctx.JSON(http.StatusUnauthorized, baofuCallbackResponse{Code: "FAIL", Message: "callback verification failed"})
		return
	}
	refundOrder, err := server.store.GetRefundOrderByOutRefundNo(ctx.Request.Context(), strings.TrimSpace(notification.Fact.OutTradeNo))
	if err != nil {
		log.Error().Err(err).Str("out_refund_no", strings.TrimSpace(notification.Fact.OutTradeNo)).Msg("load baofu refund order for callback failed")
		ctx.JSON(http.StatusInternalServerError, baofuCallbackResponse{Code: "FAIL", Message: "persist callback failed"})
		return
	}
	paymentOrder, err := server.store.GetPaymentOrder(ctx.Request.Context(), refundOrder.PaymentOrderID)
	if err != nil {
		log.Error().Err(err).
			Int64("refund_order_id", refundOrder.ID).
			Int64("payment_order_id", refundOrder.PaymentOrderID).
			Msg("load baofu refund payment order for callback failed")
		ctx.JSON(http.StatusInternalServerError, baofuCallbackResponse{Code: "FAIL", Message: "persist callback failed"})
		return
	}
	application, err := server.recordBaofuRefundCallbackFact(ctx.Request.Context(), notification, refundOrder, paymentOrder)
	if err != nil {
		log.Error().Err(err).
			Int64("refund_order_id", refundOrder.ID).
			Str("out_refund_no", strings.TrimSpace(notification.Fact.OutTradeNo)).
			Msg("persist baofu refund callback fact failed")
		ctx.JSON(http.StatusInternalServerError, baofuCallbackResponse{Code: "FAIL", Message: "persist callback failed"})
		return
	}
	switch application.Consumer {
	case paymentFactConsumerReservationDomain:
		server.enqueueReservationRefundPaymentFactApplication(ctx.Request.Context(), application)
	default:
		server.enqueueOrderRefundPaymentFactApplication(ctx.Request.Context(), application)
	}
	log.Info().
		Int64("refund_order_id", refundOrder.ID).
		Str("out_refund_no", strings.TrimSpace(notification.Fact.OutTradeNo)).
		Str("baofu_refund_state", strings.TrimSpace(notification.Fact.TransactionState)).
		Msg("baofu refund callback fact persisted")
	ctx.Data(http.StatusOK, "text/plain; charset=utf-8", []byte("OK"))
}

func (server *Server) recordBaofuRefundCallbackFact(ctx context.Context, notification *baofuaggregatenotification.RefundNotification, refundOrder db.RefundOrder, paymentOrder db.PaymentOrder) (*db.ExternalPaymentFactApplication, error) {
	if server.paymentFactService == nil {
		return nil, fmt.Errorf("baofu refund callback payment fact service is not configured")
	}
	if notification == nil {
		return nil, fmt.Errorf("baofu refund callback notification is required")
	}
	if paymentOrder.PaymentChannel != db.PaymentChannelBaofuAggregate {
		return nil, fmt.Errorf("baofu refund callback payment order %d has channel %q", paymentOrder.ID, paymentOrder.PaymentChannel)
	}
	owner, consumer, err := baofuRefundFactOwnerAndConsumer(paymentOrder)
	if err != nil {
		return nil, err
	}
	amount := notification.Fact.SuccessAmountFen
	if amount <= 0 {
		amount = refundOrder.RefundAmount
	}
	upstreamState := baofuRefundFactUpstreamState(notification.Fact)
	result, err := server.paymentFactService.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderBaofu,
		Channel:              db.PaymentChannelBaofuAggregate,
		Capability:           db.ExternalPaymentCapabilityBaofuRefund,
		FactSource:           db.ExternalPaymentFactSourceCallback,
		SourceEventID:        paymentFactStringPtr(strings.TrimSpace(notification.NotifyID)),
		SourceEventType:      paymentFactStringPtr(strings.TrimSpace(notification.NotifyType)),
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalObjectKey:    strings.TrimSpace(notification.Fact.OutTradeNo),
		ExternalSecondaryKey: paymentFactStringPtr(strings.TrimSpace(notification.Fact.TradeNo)),
		BusinessOwner:        paymentFactStringPtr(owner),
		BusinessObjectType:   paymentFactStringPtr(paymentFactBusinessObjectRefundOrder),
		BusinessObjectID:     paymentFactInt64Ptr(refundOrder.ID),
		UpstreamState:        upstreamState,
		TerminalStatus:       notification.TerminalStatus,
		Amount:               paymentFactInt64Ptr(amount),
		Currency:             "CNY",
		OccurredAt:           baofuNotificationTimePtr(notification.OccurredAt),
		UpstreamUpdatedAt:    baofuNotificationTimePtr(notification.OccurredAt),
		RawResource:          notification.Raw,
		DedupeKey:            baofuRefundCallbackDedupeKey(notification),
		Application: &logic.ExternalPaymentFactApplicationTarget{
			Consumer:           consumer,
			BusinessObjectType: paymentFactBusinessObjectRefundOrder,
			BusinessObjectID:   refundOrder.ID,
		},
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}

func baofuRefundFactOwnerAndConsumer(paymentOrder db.PaymentOrder) (owner string, consumer string, err error) {
	if paymentOrder.OrderID.Valid && !paymentOrder.ReservationID.Valid && paymentOrder.BusinessType == db.ExternalPaymentBusinessOwnerOrder {
		return db.ExternalPaymentBusinessOwnerOrder, paymentFactConsumerOrderDomain, nil
	}
	if paymentOrder.ReservationID.Valid && (paymentOrder.BusinessType == db.ExternalPaymentBusinessOwnerReservation || paymentOrder.BusinessType == "reservation_addon") {
		return db.ExternalPaymentBusinessOwnerReservation, paymentFactConsumerReservationDomain, nil
	}
	return "", "", fmt.Errorf("baofu refund callback payment order %d has unsupported business target", paymentOrder.ID)
}

func baofuNotificationTimePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	utc := value.UTC()
	return &utc
}

func baofuRefundCallbackDedupeKey(notification *baofuaggregatenotification.RefundNotification) string {
	outRefundNo := strings.TrimSpace(notification.Fact.OutTradeNo)
	sourceEventID := strings.TrimSpace(notification.NotifyID)
	if sourceEventID == "" {
		sourceEventID = baofuRefundFactUpstreamState(notification.Fact)
	}
	return fmt.Sprintf("baofu:callback:refund:%s:%s", outRefundNo, sourceEventID)
}

func baofuRefundFactUpstreamState(fact aggregatecontracts.RefundFact) string {
	if upstreamState := strings.TrimSpace(fact.TransactionState); upstreamState != "" {
		return upstreamState
	}
	return strings.TrimSpace(fact.ResultCode)
}

func (server *Server) validateBaofuPayoutNotificationIdentity(memberID string, terminalID string) error {
	configuredMerchantID := strings.TrimSpace(server.config.BaofuPayoutMerchantID)
	configuredTerminalID := strings.TrimSpace(server.config.BaofuPayoutTerminalID)
	memberID = strings.TrimSpace(memberID)
	terminalID = strings.TrimSpace(terminalID)
	if configuredMerchantID != "" && memberID != configuredMerchantID {
		return fmt.Errorf("baofu account callback member_id does not match configured payout merchant")
	}
	if configuredTerminalID != "" && terminalID != configuredTerminalID {
		return fmt.Errorf("baofu account callback terminal_id does not match configured payout terminal")
	}
	return nil
}

func (server *Server) validateBaofuCollectNotificationIdentity(memberID string, terminalID string) error {
	configuredMerchantID := strings.TrimSpace(server.config.BaofuCollectMerchantID)
	configuredTerminalID := strings.TrimSpace(server.config.BaofuCollectTerminalID)
	memberID = strings.TrimSpace(memberID)
	terminalID = strings.TrimSpace(terminalID)
	if configuredMerchantID != "" && memberID != configuredMerchantID {
		return fmt.Errorf("baofu account callback member_id does not match configured collect merchant")
	}
	if configuredTerminalID != "" && terminalID != configuredTerminalID {
		return fmt.Errorf("baofu account callback terminal_id does not match configured collect terminal")
	}
	return nil
}

func baofuAccountTerminalStatus(openState string) (terminalStatus string, isTerminal bool) {
	switch strings.TrimSpace(openState) {
	case db.BaofuAccountOpenStateActive:
		return db.ExternalPaymentTerminalStatusSuccess, true
	case db.BaofuAccountOpenStateFailed:
		return db.ExternalPaymentTerminalStatusFailed, true
	case db.BaofuAccountOpenStateProcessing:
		return db.ExternalPaymentTerminalStatusProcessing, false
	default:
		return db.ExternalPaymentTerminalStatusUnknown, false
	}
}

func baofuObservedAt(notification *baofunotification.AccountNotification) time.Time {
	if notification != nil && !notification.OccurredAt.IsZero() {
		return notification.OccurredAt.UTC()
	}
	return time.Now().UTC()
}

func baofuText(value string) pgtype.Text {
	value = strings.TrimSpace(value)
	return pgtype.Text{String: value, Valid: value != ""}
}
