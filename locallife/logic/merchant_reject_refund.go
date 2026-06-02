package logic

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/rs/zerolog/log"
)

// MerchantRejectRefundInput defines the input for refunding a rejected order.
type MerchantRejectRefundInput struct {
	MerchantID int64 // 商户ID，收付通退款路径需要用于获取 SubMchID
	OrderID    int64
	Reason     string
}

// MerchantRejectRefundResult captures refund processing details.
type MerchantRejectRefundResult struct {
	PaymentOrder *db.PaymentOrder
	RefundOrder  *db.RefundOrder
	Submission   MerchantRefundSubmission
}

// ProcessMerchantRejectRefund handles full refund for a merchant-rejected order.
func ProcessMerchantRejectRefund(
	ctx context.Context,
	store db.Store,
	paymentFacade PaymentFacade,
	input MerchantRejectRefundInput,
) (MerchantRejectRefundResult, error) {
	var result MerchantRejectRefundResult

	paymentOrder, err := store.GetLatestPaymentOrderByOrder(ctx, db.GetLatestPaymentOrderByOrderParams{
		OrderID:      pgtype.Int8{Int64: input.OrderID, Valid: true},
		BusinessType: "order",
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			result.Submission = MerchantRefundSubmission{
				Status:  MerchantRefundSubmissionStatusNotNeeded,
				Message: "订单未找到已支付支付单，无需发起退款。",
			}
			return result, nil
		}
		return result, err
	}
	if paymentOrder.Status != "paid" {
		paymentOrders, listErr := store.GetPaymentOrdersByOrder(ctx, pgtype.Int8{Int64: input.OrderID, Valid: true})
		if listErr != nil {
			return result, listErr
		}

		foundPaid := false
		for _, candidate := range paymentOrders {
			if candidate.BusinessType == "order" && candidate.Status == "paid" {
				paymentOrder = candidate
				foundPaid = true
				break
			}
		}

		if !foundPaid {
			result.Submission = MerchantRefundSubmission{
				Status:  MerchantRefundSubmissionStatusNotNeeded,
				Message: "订单未找到已支付支付单，无需发起退款。",
			}
			return result, nil
		}
	}
	result.PaymentOrder = &paymentOrder
	if !paymentOrderUsesBaofuAggregateChannel(paymentOrder) {
		return result, mainBusinessBaofuOnlyError("处理商户拒单退款")
	}

	reason := fmt.Sprintf("商户拒单：%s", input.Reason)
	outRefundNo, err := generateOutRefundNo()
	if err != nil {
		return result, fmt.Errorf("generate out refund no: %w", err)
	}

	txResult, err := store.CreateRefundOrderTx(ctx, db.CreateRefundOrderTxParams{
		PaymentOrderID: paymentOrder.ID,
		RefundType:     paymentTypeProfitSharing,
		RefundAmount:   paymentOrder.Amount,
		RefundReason:   reason,
		OutRefundNo:    outRefundNo,
	})
	if err != nil {
		if _, ok := db.IsRefundRequestError(err); ok {
			return result, fmt.Errorf("refund validation: %w", err)
		}
		return result, err
	}
	refundOrder := txResult.RefundOrder
	result.RefundOrder = &refundOrder
	if err := processMerchantRejectBaofuRefund(ctx, store, paymentFacade, paymentOrder, refundOrder, reason); err != nil {
		latest := refundOrder
		if loaded, getErr := store.GetRefundOrder(ctx, refundOrder.ID); getErr == nil {
			latest = loaded
			result.RefundOrder = &latest
		}
		result.Submission = merchantRejectRefundSubmissionForError(latest, err)
		return result, err
	}
	latest := refundOrder
	if loaded, getErr := store.GetRefundOrder(ctx, refundOrder.ID); getErr == nil {
		latest = loaded
		result.RefundOrder = &latest
	}
	result.Submission = MerchantRefundSubmission{
		Status:      MerchantRefundSubmissionStatusAccepted,
		Message:     "退款申请已提交，系统会继续同步退款结果。",
		RefundOrder: &latest,
	}
	return result, nil
}

func merchantRejectRefundSubmissionForError(refundOrder db.RefundOrder, err error) MerchantRefundSubmission {
	status := MerchantRefundSubmissionStatusManualRequired
	message := "订单已取消，但退款提交失败，请联系平台处理。"
	if refundOrder.Status == "pending" {
		status = MerchantRefundSubmissionStatusPendingRecovery
		message = "订单已取消，退款提交暂未确认，系统会稍后自动重试。"
	}
	if refundOrder.Status == "failed" {
		message = "订单已取消，但退款提交被支付通道拒绝，请联系平台处理。"
	}

	var reqErr *RequestError
	if errors.As(err, &reqErr) && reqErr.Err != nil {
		if reqErr.Status == http.StatusServiceUnavailable && refundOrder.Status == "pending" {
			message = "订单已取消，退款通道暂不可用，系统会稍后自动重试。"
		}
		if reqErr.Status == http.StatusServiceUnavailable && refundOrder.Status == "failed" {
			message = "订单已取消，但退款通道暂不可用，请联系平台处理。"
		}
	}

	return MerchantRefundSubmission{
		Status:      status,
		Message:     message,
		RefundOrder: &refundOrder,
	}
}

func processMerchantRejectBaofuRefund(
	ctx context.Context,
	store db.Store,
	paymentFacade PaymentFacade,
	paymentOrder db.PaymentOrder,
	refundOrder db.RefundOrder,
	reason string,
) error {
	if paymentFacade == nil {
		configErr := errors.New("baofu payment facade not configured")
		log.Error().
			Err(configErr).
			Int64("payment_order_id", paymentOrder.ID).
			Int64("refund_order_id", refundOrder.ID).
			Msg("baofu refund facade missing for merchant reject refund")
		return NewRequestErrorWithCause(http.StatusServiceUnavailable, errors.New("宝付退款通道未配置，请联系平台处理"), configErr)
	}

	req := aggregatecontracts.RefundBeforeShareRequest{
		OutTradeNo:      strings.TrimSpace(refundOrder.OutRefundNo),
		NotifyURL:       paymentFacade.BaofuRefundNotifyURL(),
		RefundAmountFen: refundOrder.RefundAmount,
		TotalAmountFen:  refundOrder.RefundAmount,
		TransactionTime: time.Now().UTC().Format("20060102150405"),
		RefundReason:    strings.TrimSpace(reason),
	}
	if paymentOrder.TransactionID.Valid && strings.TrimSpace(paymentOrder.TransactionID.String) != "" {
		req.OriginTradeNo = strings.TrimSpace(paymentOrder.TransactionID.String)
	} else {
		req.OriginOutTradeNo = strings.TrimSpace(paymentOrder.OutTradeNo)
	}

	baofuRefund, err := paymentFacade.CreateBaofuRefund(ctx, req)
	if err != nil {
		disposition := ClassifyBaofuRefundCreateFailure(err)
		if dbErr := applyBaofuRefundCreateFailureDisposition(ctx, store, refundOrder, "", disposition); dbErr != nil {
			return dbErr
		}
		recordBaofuRefundCommand(ctx, store, paymentOrder, refundOrder, nil, disposition.CommandStatus, err)
		if disposition.MarkProcessing {
			return nil
		}
		return mapBaofuRefundCreateError(err)
	}
	refundID := ""
	if baofuRefund != nil {
		refundID = strings.TrimSpace(baofuRefund.TradeNo)
		if refundID == "" {
			refundID = strings.TrimSpace(baofuRefund.OutTradeNo)
		}
	}
	if baofuRefund == nil {
		err := errors.New("baofu refund returned empty result")
		if _, dbErr := store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
			log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark baofu merchant reject refund as failed")
		}
		recordBaofuRefundCommand(ctx, store, paymentOrder, refundOrder, nil, db.ExternalPaymentCommandStatusRejected, err)
		return err
	}
	if strings.EqualFold(strings.TrimSpace(baofuRefund.ResultCode), aggregatecontracts.BusinessResultCodeFail) {
		providerErr := BaofuRefundCreateProviderResultError(baofuRefund)
		disposition := ClassifyBaofuRefundCreateFailure(providerErr)
		if dbErr := applyBaofuRefundCreateFailureDisposition(ctx, store, refundOrder, refundID, disposition); dbErr != nil {
			return dbErr
		}
		recordBaofuRefundCommand(ctx, store, paymentOrder, refundOrder, baofuRefund, disposition.CommandStatus, nil)
		if disposition.MarkProcessing {
			return nil
		}
		return mapBaofuRefundCreateError(providerErr)
	}
	if _, dbErr := store.UpdateRefundOrderToProcessing(ctx, db.UpdateRefundOrderToProcessingParams{
		ID:       refundOrder.ID,
		RefundID: pgtype.Text{String: refundID, Valid: refundID != ""},
	}); dbErr != nil {
		log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as processing")
	}
	recordBaofuRefundCommand(ctx, store, paymentOrder, refundOrder, baofuRefund, db.ExternalPaymentCommandStatusAccepted, nil)
	return nil
}
