package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"

	"github.com/merrydance/locallife/baofu"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
)

const paymentTypeProfitSharing = "profit_sharing"

type RefundService struct {
	store         db.Store
	paymentFacade PaymentFacade
	taskScheduler TaskScheduler
	clock         Clock
	idGenerator   IDGenerator
}

func NewRefundService(
	store db.Store,
	paymentFacade PaymentFacade,
	taskScheduler TaskScheduler,
	clock Clock,
	idGenerator IDGenerator,
) *RefundService {
	if clock == nil {
		clock = SystemClock{}
	}
	if idGenerator == nil {
		idGenerator = DefaultIDGenerator{}
	}

	return &RefundService{
		store:         store,
		paymentFacade: paymentFacade,
		taskScheduler: taskScheduler,
		clock:         clock,
		idGenerator:   idGenerator,
	}
}

// maybeMarkPaymentOrderRefunded 仅在累计退款额 >= 支付金额时才将支付单标记为 refunded，
// 避免部分退款错误终结支付单。
func (s *RefundService) maybeMarkPaymentOrderRefunded(ctx context.Context, paymentOrderID int64, paymentAmount int64) {
	totalSuccessfulRefunded, err := s.store.GetTotalSuccessfulRefundedByPaymentOrder(ctx, paymentOrderID)
	if err != nil {
		log.Error().Err(err).Int64("payment_order_id", paymentOrderID).Msg("failed to get total successful refunded amount")
		return
	}
	if totalSuccessfulRefunded >= paymentAmount {
		if _, dbErr := s.store.UpdatePaymentOrderToRefunded(ctx, paymentOrderID); dbErr != nil {
			log.Error().Err(dbErr).Int64("payment_order_id", paymentOrderID).Msg("failed to mark payment order as refunded")
		}
	} else {
		log.Info().
			Int64("payment_order_id", paymentOrderID).
			Int64("total_successful_refunded", totalSuccessfulRefunded).
			Int64("payment_amount", paymentAmount).
			Msg("partial refund: payment order not yet fully refunded")
	}
}

func (s *RefundService) CreateRefundOrder(ctx context.Context, input CreateRefundOrderInput) (CreateRefundOrderResult, error) {
	merchant, err := resolveMerchantForUser(ctx, s.store, input.ActorUserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return CreateRefundOrderResult{}, NewRequestError(http.StatusForbidden, errors.New("you are not a merchant"))
		}
		return CreateRefundOrderResult{}, err
	}

	paymentOrder, err := s.store.GetPaymentOrder(ctx, input.PaymentOrderID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return CreateRefundOrderResult{}, NewRequestError(http.StatusNotFound, errors.New("payment order not found"))
		}
		return CreateRefundOrderResult{}, err
	}

	if paymentOrder.Status != "paid" {
		return CreateRefundOrderResult{}, NewRequestError(http.StatusBadRequest, errors.New("payment order is not paid"))
	}

	if !paymentOrder.OrderID.Valid {
		return CreateRefundOrderResult{}, NewRequestError(http.StatusBadRequest, errors.New("payment order has no associated order"))
	}

	order, err := s.store.GetOrder(ctx, paymentOrder.OrderID.Int64)
	if err != nil {
		return CreateRefundOrderResult{}, err
	}

	if order.MerchantID != merchant.ID {
		return CreateRefundOrderResult{}, NewRequestError(http.StatusForbidden, errors.New("order does not belong to your merchant"))
	}

	if input.RefundAmount > paymentOrder.Amount {
		return CreateRefundOrderResult{}, NewRequestError(http.StatusBadRequest, errors.New("refund amount exceeds payment amount"))
	}

	if !paymentOrderUsesBaofuAggregateChannel(paymentOrder) {
		return CreateRefundOrderResult{}, mainBusinessBaofuOnlyError("发起退款")
	}

	guard, err := s.store.GetBaofuPaymentOrderRefundGuardForUpdate(ctx, paymentOrder.ID)
	if err != nil {
		return CreateRefundOrderResult{}, fmt.Errorf("get baofu refund guard: %w", err)
	}
	if guard.HasStartedProfitSharing {
		return CreateRefundOrderResult{}, NewRequestError(http.StatusBadRequest, errors.New("订单已进入结算分账流程，不支持退款"))
	}
	outRefundNo, err := s.idGenerator.OutRefundNo(s.clock.Now())
	if err != nil {
		return CreateRefundOrderResult{}, fmt.Errorf("generate out refund no: %w", err)
	}
	txResult, err := s.store.CreateRefundOrderTx(ctx, db.CreateRefundOrderTxParams{
		PaymentOrderID: input.PaymentOrderID,
		RefundType:     input.RefundType,
		RefundAmount:   input.RefundAmount,
		RefundReason:   input.RefundReason,
		OutRefundNo:    outRefundNo,
	})
	if err != nil {
		if statusCode, ok := db.IsRefundRequestError(err); ok {
			return CreateRefundOrderResult{}, NewRequestError(statusCode, errors.Unwrap(err))
		}
		return CreateRefundOrderResult{}, fmt.Errorf("create baofu refund order: %w", err)
	}
	refundOrder := txResult.RefundOrder
	if err := s.processBaofuPreShareRefund(ctx, paymentOrder, refundOrder, input); err != nil {
		return CreateRefundOrderResult{}, err
	}
	if latest, getErr := s.store.GetRefundOrder(ctx, refundOrder.ID); getErr == nil {
		refundOrder = latest
	}
	return CreateRefundOrderResult{RefundOrder: refundOrder}, nil
}

func (s *RefundService) GetRefundOrder(ctx context.Context, input GetRefundOrderInput) (GetRefundOrderResult, error) {
	refundOrder, err := s.store.GetRefundOrder(ctx, input.RefundID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return GetRefundOrderResult{}, NewRequestError(http.StatusNotFound, errors.New("refund order not found"))
		}
		return GetRefundOrderResult{}, err
	}

	paymentOrder, err := s.store.GetPaymentOrder(ctx, refundOrder.PaymentOrderID)
	if err != nil {
		return GetRefundOrderResult{}, err
	}

	if paymentOrder.UserID == input.ActorUserID {
		return GetRefundOrderResult{RefundOrder: refundOrder}, nil
	}

	if paymentOrder.OrderID.Valid {
		order, getOrderErr := s.store.GetOrder(ctx, paymentOrder.OrderID.Int64)
		if getOrderErr == nil {
			merchant, getMerchantErr := resolveMerchantForUser(ctx, s.store, input.ActorUserID)
			if getMerchantErr == nil && order.MerchantID == merchant.ID {
				return GetRefundOrderResult{RefundOrder: refundOrder}, nil
			}
		}
	}

	return GetRefundOrderResult{}, NewRequestError(http.StatusForbidden, errors.New("access denied"))
}

func (s *RefundService) ListRefundOrdersByPayment(ctx context.Context, input ListRefundOrdersByPaymentInput) (ListRefundOrdersByPaymentResult, error) {
	paymentOrder, err := s.store.GetPaymentOrder(ctx, input.PaymentOrderID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return ListRefundOrdersByPaymentResult{}, NewRequestError(http.StatusNotFound, errors.New("payment order not found"))
		}
		return ListRefundOrdersByPaymentResult{}, err
	}

	if paymentOrder.UserID != input.ActorUserID {
		if paymentOrder.OrderID.Valid {
			order, getOrderErr := s.store.GetOrder(ctx, paymentOrder.OrderID.Int64)
			if getOrderErr != nil {
				return ListRefundOrdersByPaymentResult{}, getOrderErr
			}
			merchant, getMerchantErr := resolveMerchantForUser(ctx, s.store, input.ActorUserID)
			if getMerchantErr != nil || order.MerchantID != merchant.ID {
				return ListRefundOrdersByPaymentResult{}, NewRequestError(http.StatusForbidden, errors.New("access denied"))
			}
		} else {
			return ListRefundOrdersByPaymentResult{}, NewRequestError(http.StatusForbidden, errors.New("access denied"))
		}
	}

	refundOrders, err := s.store.ListRefundOrdersByPaymentOrder(ctx, input.PaymentOrderID)
	if err != nil {
		return ListRefundOrdersByPaymentResult{}, err
	}

	return ListRefundOrdersByPaymentResult{RefundOrders: refundOrders}, nil
}

func (s *RefundService) ListProfitSharingReturnsByRefund(ctx context.Context, input ListProfitSharingReturnsByRefundInput) (ListProfitSharingReturnsByRefundResult, error) {
	merchant, err := resolveMerchantForUser(ctx, s.store, input.ActorUserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return ListProfitSharingReturnsByRefundResult{}, NewRequestError(http.StatusForbidden, errors.New("you are not a merchant"))
		}
		return ListProfitSharingReturnsByRefundResult{}, err
	}

	refundOrder, err := s.store.GetRefundOrder(ctx, input.RefundID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return ListProfitSharingReturnsByRefundResult{}, NewRequestError(http.StatusNotFound, errors.New("refund order not found"))
		}
		return ListProfitSharingReturnsByRefundResult{}, err
	}

	paymentOrder, err := s.store.GetPaymentOrder(ctx, refundOrder.PaymentOrderID)
	if err != nil {
		return ListProfitSharingReturnsByRefundResult{}, err
	}
	if !paymentOrder.OrderID.Valid {
		return ListProfitSharingReturnsByRefundResult{}, NewRequestError(http.StatusBadRequest, errors.New("refund order has no associated order"))
	}

	order, err := s.store.GetOrder(ctx, paymentOrder.OrderID.Int64)
	if err != nil {
		return ListProfitSharingReturnsByRefundResult{}, err
	}
	if order.MerchantID != merchant.ID {
		return ListProfitSharingReturnsByRefundResult{}, NewRequestError(http.StatusForbidden, errors.New("refund order does not belong to your merchant"))
	}

	returns, err := s.store.ListProfitSharingReturnsByRefundOrder(ctx, refundOrder.ID)
	if err != nil {
		return ListProfitSharingReturnsByRefundResult{}, err
	}

	return ListProfitSharingReturnsByRefundResult{Returns: returns}, nil
}

func (s *RefundService) processBaofuPreShareRefund(ctx context.Context, paymentOrder db.PaymentOrder, refundOrder db.RefundOrder, input CreateRefundOrderInput) error {
	if s.paymentFacade == nil {
		if _, dbErr := s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
			log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark baofu refund as failed")
		}
		return fmt.Errorf("baofu aggregate client: not configured")
	}

	req := aggregatecontracts.RefundBeforeShareRequest{
		OutTradeNo:      strings.TrimSpace(refundOrder.OutRefundNo),
		NotifyURL:       s.paymentFacade.BaofuRefundNotifyURL(),
		RefundAmountFen: input.RefundAmount,
		TotalAmountFen:  input.RefundAmount,
		TransactionTime: s.clock.Now().UTC().Format("20060102150405"),
		RefundReason:    strings.TrimSpace(input.RefundReason),
	}
	if paymentOrder.TransactionID.Valid && strings.TrimSpace(paymentOrder.TransactionID.String) != "" {
		req.OriginTradeNo = strings.TrimSpace(paymentOrder.TransactionID.String)
	} else {
		req.OriginOutTradeNo = strings.TrimSpace(paymentOrder.OutTradeNo)
	}
	refundResp, err := s.paymentFacade.CreateBaofuRefund(ctx, req)
	if err != nil {
		if _, dbErr := s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
			log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark baofu refund as failed")
		}
		recordBaofuRefundCommand(ctx, s.store, paymentOrder, refundOrder, nil, db.ExternalPaymentCommandStatusRejected, err)
		return mapBaofuRefundCreateError(err)
	}
	if refundResp == nil {
		err := errors.New("baofu refund returned empty result")
		if _, dbErr := s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
			log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark baofu refund as failed")
		}
		recordBaofuRefundCommand(ctx, s.store, paymentOrder, refundOrder, nil, db.ExternalPaymentCommandStatusRejected, err)
		return err
	}

	refundID := strings.TrimSpace(refundResp.TradeNo)
	switch strings.ToUpper(strings.TrimSpace(refundResp.ResultCode)) {
	case aggregatecontracts.BusinessResultCodeSuccess:
		if _, dbErr := s.store.UpdateRefundOrderToProcessing(ctx, db.UpdateRefundOrderToProcessingParams{
			ID:       refundOrder.ID,
			RefundID: pgtype.Text{String: refundID, Valid: refundID != ""},
		}); dbErr != nil {
			return fmt.Errorf("update baofu refund order to processing: %w", dbErr)
		}
		recordBaofuRefundCommand(ctx, s.store, paymentOrder, refundOrder, refundResp, db.ExternalPaymentCommandStatusAccepted, nil)
	case aggregatecontracts.BusinessResultCodeFail:
		if _, dbErr := s.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
			return fmt.Errorf("update baofu refund order to failed: %w", dbErr)
		}
		recordBaofuRefundCommand(ctx, s.store, paymentOrder, refundOrder, refundResp, db.ExternalPaymentCommandStatusRejected, nil)
		return NewRequestError(http.StatusBadGateway, errors.New("宝付退款受理失败，请稍后重试或联系平台处理"))
	default:
		if _, dbErr := s.store.UpdateRefundOrderToProcessing(ctx, db.UpdateRefundOrderToProcessingParams{
			ID:       refundOrder.ID,
			RefundID: pgtype.Text{String: refundID, Valid: refundID != ""},
		}); dbErr != nil {
			return fmt.Errorf("update baofu refund order to processing: %w", dbErr)
		}
		recordBaofuRefundCommand(ctx, s.store, paymentOrder, refundOrder, refundResp, db.ExternalPaymentCommandStatusUnknown, nil)
	}
	return nil
}

func mapBaofuRefundCreateError(err error) error {
	if err == nil {
		return nil
	}
	if message := strings.ToLower(err.Error()); strings.Contains(message, "baofu") && strings.Contains(message, "not configured") {
		return NewRequestErrorWithCause(http.StatusServiceUnavailable, errors.New("宝付退款通道未配置，请联系平台处理"), err)
	}
	var providerErr *baofu.ProviderError
	if !errors.As(err, &providerErr) {
		return err
	}
	classified := baofu.ClassifyBaofuError(providerErr.UpstreamCode, providerErr.UpstreamMessage)
	status := http.StatusBadGateway
	switch classified.Category {
	case baofu.BaofuErrorCategoryUserActionRequired:
		status = http.StatusBadRequest
	case baofu.BaofuErrorCategoryPlatformConfiguration:
		status = http.StatusServiceUnavailable
	case baofu.BaofuErrorCategoryRetryable:
		status = http.StatusServiceUnavailable
	case baofu.BaofuErrorCategoryManualReview:
		status = http.StatusBadGateway
	}
	return NewRequestErrorWithCause(status, errors.New(classified.PublicMessage), err)
}

func recordBaofuRefundCommand(ctx context.Context, store db.Store, paymentOrder db.PaymentOrder, refundOrder db.RefundOrder, refundResp *aggregatecontracts.RefundResult, status string, cause error) {
	snapshot := buildBaofuRefundCommandSnapshot(refundOrder, refundResp, cause)
	var secondary pgtype.Text
	var errorCode pgtype.Text
	var errorMessage pgtype.Text
	if refundResp != nil {
		if tradeNo := strings.TrimSpace(refundResp.TradeNo); tradeNo != "" {
			secondary = pgtype.Text{String: tradeNo, Valid: true}
		}
		if code := strings.TrimSpace(refundResp.ErrorCode); code != "" {
			errorCode = pgtype.Text{String: code, Valid: true}
		}
		if message := baofuRefundCommandErrorMessage(refundResp.ErrorCode, refundResp.ErrorMessage, cause); message != "" {
			errorMessage = pgtype.Text{String: message, Valid: true}
		}
	} else {
		var providerErr *baofu.ProviderError
		if errors.As(cause, &providerErr) {
			if code := strings.TrimSpace(providerErr.UpstreamCode); code != "" {
				errorCode = pgtype.Text{String: code, Valid: true}
			}
		}
		if message := baofuRefundCommandErrorMessage("", "", cause); message != "" {
			errorMessage = pgtype.Text{String: message, Valid: true}
		}
	}
	arg := db.CreateExternalPaymentCommandParams{
		Provider:             db.ExternalPaymentProviderBaofu,
		Channel:              db.PaymentChannelBaofuAggregate,
		Capability:           db.ExternalPaymentCapabilityBaofuRefund,
		CommandType:          db.ExternalPaymentCommandTypeCreateRefund,
		BusinessOwner:        refundServiceBaofuRefundBusinessOwner(paymentOrder),
		BusinessObjectType:   pgtype.Text{String: "refund_order", Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: refundOrder.ID, Valid: true},
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalObjectKey:    strings.TrimSpace(refundOrder.OutRefundNo),
		ExternalSecondaryKey: secondary,
		CommandStatus:        status,
		SubmittedAt:          time.Now().UTC(),
		ResponseSnapshot:     snapshot,
	}
	if status == db.ExternalPaymentCommandStatusAccepted {
		arg.AcceptedAt = pgtype.Timestamptz{Time: arg.SubmittedAt, Valid: true}
	}
	if status == db.ExternalPaymentCommandStatusRejected {
		arg.RejectedAt = pgtype.Timestamptz{Time: arg.SubmittedAt, Valid: true}
	}
	if errorCode.Valid {
		arg.LastErrorCode = errorCode
	}
	if errorMessage.Valid {
		arg.LastErrorMessage = errorMessage
	}
	_, err := store.CreateExternalPaymentCommand(ctx, arg)
	if err != nil {
		log.Error().Err(err).Int64("refund_order_id", refundOrder.ID).Msg("failed to record baofu refund command")
	}
}

func baofuRefundCommandErrorMessage(errorCode string, upstreamMessage string, cause error) string {
	if trimmedCode := strings.TrimSpace(errorCode); trimmedCode != "" || strings.TrimSpace(upstreamMessage) != "" {
		return strings.TrimSpace(baofu.BaofuCommandMessage(trimmedCode, upstreamMessage))
	}
	var providerErr *baofu.ProviderError
	if errors.As(cause, &providerErr) {
		return strings.TrimSpace(baofu.BaofuCommandMessage(providerErr.UpstreamCode, providerErr.UpstreamMessage))
	}
	if cause != nil {
		return strings.TrimSpace(cause.Error())
	}
	return ""
}

func refundServiceBaofuRefundBusinessOwner(paymentOrder db.PaymentOrder) string {
	if paymentOrder.BusinessType == businessTypeReservation || paymentOrder.BusinessType == reservationAddonBusiness || paymentOrder.ReservationID.Valid {
		return db.ExternalPaymentBusinessOwnerReservation
	}
	return db.ExternalPaymentBusinessOwnerOrder
}

func buildBaofuRefundCommandSnapshot(refundOrder db.RefundOrder, refundResp *aggregatecontracts.RefundResult, cause error) []byte {
	snapshot := struct {
		Provider     string `json:"provider"`
		Operation    string `json:"operation"`
		OutRefundNo  string `json:"out_refund_no"`
		RefundTrade  string `json:"refund_trade_no,omitempty"`
		RefundState  string `json:"refund_state,omitempty"`
		ResultCode   string `json:"result_code,omitempty"`
		ErrorCode    string `json:"error_code,omitempty"`
		ErrorPresent bool   `json:"error_present,omitempty"`
	}{
		Provider:    db.ExternalPaymentProviderBaofu,
		Operation:   "order_refund",
		OutRefundNo: strings.TrimSpace(refundOrder.OutRefundNo),
	}
	if refundResp != nil {
		snapshot.RefundTrade = strings.TrimSpace(refundResp.TradeNo)
		snapshot.RefundState = strings.TrimSpace(refundResp.RefundState)
		snapshot.ResultCode = strings.TrimSpace(refundResp.ResultCode)
		snapshot.ErrorCode = strings.TrimSpace(refundResp.ErrorCode)
		if strings.EqualFold(strings.TrimSpace(refundResp.ResultCode), aggregatecontracts.BusinessResultCodeFail) ||
			strings.TrimSpace(refundResp.ErrorCode) != "" ||
			strings.TrimSpace(refundResp.ErrorMessage) != "" {
			snapshot.ErrorPresent = true
		}
	}
	if cause != nil {
		snapshot.ErrorPresent = true
		var providerErr *baofu.ProviderError
		if errors.As(cause, &providerErr) {
			snapshot.ErrorCode = strings.TrimSpace(providerErr.UpstreamCode)
		}
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return []byte(`{"provider":"baofu","operation":"order_refund"}`)
	}
	return raw
}
