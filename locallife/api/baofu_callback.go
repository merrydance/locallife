package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	baofunotification "github.com/merrydance/locallife/baofu/account/notification"
	baofuaggregatenotification "github.com/merrydance/locallife/baofu/aggregatepay/notification"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/rs/zerolog/log"
)

type baofuAccountNotificationParser interface {
	ParseOpenAccountNotification(body []byte) (*baofunotification.AccountNotification, error)
}

type baofuAggregatePaymentNotificationParser interface {
	ParsePaymentNotification(body []byte) (*baofuaggregatenotification.PaymentNotification, error)
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
	fact, err := server.recordBaofuAccountOpenCallbackFact(ctx.Request.Context(), notification)
	if err != nil {
		log.Error().Err(err).
			Str("out_request_no", strings.TrimSpace(notification.OutRequestNo)).
			Str("contract_no", strings.TrimSpace(notification.ContractNo)).
			Msg("persist baofu account callback fact failed")
		ctx.JSON(http.StatusInternalServerError, baofuCallbackResponse{Code: "FAIL", Message: "persist callback failed"})
		return
	}
	log.Info().
		Int64("payment_fact_id", fact.ID).
		Str("out_request_no", strings.TrimSpace(notification.OutRequestNo)).
		Str("baofu_open_state", strings.TrimSpace(notification.OpenState)).
		Msg("baofu account callback fact persisted")
	ctx.JSON(http.StatusOK, baofuCallbackResponse{Code: "SUCCESS", Message: "OK"})
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
	paymentOrder, err := server.store.GetPaymentOrderByOutTradeNo(ctx.Request.Context(), strings.TrimSpace(notification.Fact.OutTradeNo))
	if err != nil {
		log.Error().Err(err).Str("out_trade_no", strings.TrimSpace(notification.Fact.OutTradeNo)).Msg("load baofu payment order for callback failed")
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
	ctx.JSON(http.StatusOK, baofuCallbackResponse{Code: "SUCCESS", Message: "OK"})
}

func (server *Server) recordBaofuAccountOpenCallbackFact(ctx context.Context, notification *baofunotification.AccountNotification) (db.ExternalPaymentFact, error) {
	if notification == nil {
		return db.ExternalPaymentFact{}, fmt.Errorf("baofu account callback notification is required")
	}
	outRequestNo := strings.TrimSpace(notification.OutRequestNo)
	if outRequestNo == "" {
		return db.ExternalPaymentFact{}, fmt.Errorf("baofu account callback out request no is required")
	}
	upstreamState := strings.TrimSpace(notification.UpstreamState)
	terminalStatus, isTerminal := baofuAccountTerminalStatus(notification.OpenState)
	observedAt := baofuObservedAt(notification)
	return server.store.CreateExternalPaymentFact(ctx, db.CreateExternalPaymentFactParams{
		Provider:             db.ExternalPaymentProviderBaofu,
		Channel:              db.PaymentChannelBaofuAggregate,
		Capability:           db.ExternalPaymentCapabilityBaofuAccount,
		FactSource:           db.ExternalPaymentFactSourceCallback,
		SourceEventID:        pgtype.Text{String: outRequestNo, Valid: true},
		SourceEventType:      pgtype.Text{String: "BAOFU_ACCOUNT_OPEN", Valid: true},
		ExternalObjectType:   "baofu_account",
		ExternalObjectKey:    outRequestNo,
		ExternalSecondaryKey: baofuText(strings.TrimSpace(notification.ContractNo)),
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerApplyment, Valid: true},
		BusinessObjectType:   pgtype.Text{},
		BusinessObjectID:     pgtype.Int8{},
		UpstreamState:        upstreamState,
		TerminalStatus:       terminalStatus,
		IsTerminal:           isTerminal,
		Currency:             "CNY",
		OccurredAt:           pgtype.Timestamptz{Time: observedAt, Valid: true},
		ObservedAt:           time.Now().UTC(),
		RawResource:          notification.Raw,
		DedupeKey:            fmt.Sprintf("baofu:callback:account:%s:%s", outRequestNo, upstreamState),
		ProcessingStatus:     db.ExternalPaymentFactProcessingStatusReceived,
	})
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
