package logic

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/baofu"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"

	"github.com/rs/zerolog/log"
)

// ReplaceOrderInput defines the input for replacing a reservation order.
type ReplaceOrderInput struct {
	UserID   int64
	OrderID  int64
	Items    []OrderItemInput
	Notes    string
	ClientIP string
}

// ReplaceOrderResult reports the replacement outcome.
type ReplaceOrderResult struct {
	NewOrder        db.Order
	Delta           int64
	PaymentOrderID  *int64
	RefundInitiated bool
}

// ReplaceReservationOrder replaces a full-payment reservation order with new items.
func ReplaceReservationOrder(
	ctx context.Context,
	store db.Store,
	input ReplaceOrderInput,
	normalize NormalizeDishCustomizationsFunc,
) (ReplaceOrderResult, error) {
	return ReplaceReservationOrderWithBaofu(ctx, store, nil, input, normalize)
}

func ReplaceReservationOrderWithBaofu(
	ctx context.Context,
	store db.Store,
	paymentFacade PaymentFacade,
	input ReplaceOrderInput,
	normalize NormalizeDishCustomizationsFunc,
) (ReplaceOrderResult, error) {
	oldOrder, err := store.GetOrderForUpdate(ctx, input.OrderID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return ReplaceOrderResult{}, NewRequestError(http.StatusNotFound, errors.New("order not found"))
		}
		return ReplaceOrderResult{}, err
	}

	if oldOrder.UserID != input.UserID {
		return ReplaceOrderResult{}, NewRequestError(http.StatusForbidden, errors.New("order does not belong to you"))
	}
	if oldOrder.OrderType != "reservation" {
		return ReplaceOrderResult{}, NewRequestError(http.StatusBadRequest, errors.New("only reservation orders can be replaced"))
	}
	if oldOrder.Status != "paid" {
		return ReplaceOrderResult{}, NewRequestError(http.StatusConflict, errors.New("order must be paid before replacement"))
	}
	if oldOrder.ReplacedByOrderID.Valid {
		return ReplaceOrderResult{}, NewRequestError(http.StatusConflict, errors.New("order already replaced"))
	}
	if !oldOrder.ReservationID.Valid {
		return ReplaceOrderResult{}, NewRequestError(http.StatusBadRequest, errors.New("order missing reservation"))
	}

	reservation, err := store.GetTableReservation(ctx, oldOrder.ReservationID.Int64)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return ReplaceOrderResult{}, NewRequestError(http.StatusNotFound, errors.New("reservation not found"))
		}
		return ReplaceOrderResult{}, err
	}
	if reservation.UserID != input.UserID {
		return ReplaceOrderResult{}, NewRequestError(http.StatusForbidden, errors.New("reservation does not belong to you"))
	}
	if reservation.PaymentMode != "full" {
		return ReplaceOrderResult{}, NewRequestError(http.StatusBadRequest, errors.New("only full-payment reservations support replacement"))
	}
	if reservation.Status != "paid" && reservation.Status != "confirmed" && reservation.Status != "checked_in" {
		return ReplaceOrderResult{}, NewRequestError(http.StatusConflict, errors.New("reservation is not ready for replacement"))
	}

	session, err := store.GetActiveDiningSessionByReservation(ctx, pgtype.Int8{Int64: reservation.ID, Valid: true})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return ReplaceOrderResult{}, NewRequestError(http.StatusConflict, errors.New("no active dining session for reservation"))
		}
		return ReplaceOrderResult{}, err
	}
	if session.UserID != input.UserID {
		return ReplaceOrderResult{}, NewRequestError(http.StatusForbidden, errors.New("dining session does not belong to you"))
	}

	subtotal, items, err := CalculateOrderItems(ctx, store, reservation.MerchantID, input.Items, normalize)
	if err != nil {
		return ReplaceOrderResult{}, NewRequestError(http.StatusBadRequest, err)
	}

	discountAmount := int64(0)
	if bestAmount, err := GetBestDiscountAmount(ctx, store, reservation.MerchantID, subtotal); err == nil {
		discountAmount = bestAmount
	}

	newTotal := subtotal - discountAmount
	if newTotal < 0 {
		newTotal = 0
	}

	delta := newTotal - oldOrder.TotalAmount
	newStatus := "paid"
	newFulfillment := "pending_kitchen"
	if delta > 0 {
		newStatus = "pending"
		newFulfillment = "scheduled"
		if paymentFacade == nil {
			return ReplaceOrderResult{}, fmt.Errorf("baofu payment facade not configured")
		}
	}

	var refundAllocations []reservationRefundAllocation
	if delta < 0 {
		refundAmount := -delta
		if refundAmount > 0 {
			refundAllocations, err = buildReservationRefundAllocations(ctx, store, reservation.ID, refundAmount)
			if err != nil {
				return ReplaceOrderResult{}, err
			}
			allocatedRefundAmount := sumReservationRefundAllocations(refundAllocations)
			if allocatedRefundAmount != refundAmount {
				return ReplaceOrderResult{}, NewRequestError(http.StatusConflict, errors.New("预订退款资金链路已变化，请刷新后重试"))
			}
			// 退款路径已统一到 Baofu，实际退款在后续创建退款单时校验。
		}
	}

	orderNo, err := generateOrderNo()
	if err != nil {
		return ReplaceOrderResult{}, fmt.Errorf("generate order no: %w", err)
	}
	createArgs := db.CreateOrderParams{
		OrderNo:             orderNo,
		UserID:              input.UserID,
		MerchantID:          reservation.MerchantID,
		OrderType:           "dine_in",
		TableID:             pgtype.Int8{Int64: reservation.TableID, Valid: true},
		ReservationID:       pgtype.Int8{Int64: reservation.ID, Valid: true},
		DeliveryFee:         0,
		Subtotal:            subtotal,
		DiscountAmount:      discountAmount,
		DeliveryFeeDiscount: 0,
		TotalAmount:         newTotal,
		Status:              newStatus,
		FulfillmentStatus:   newFulfillment,
	}
	if input.Notes != "" {
		createArgs.Notes = pgtype.Text{String: input.Notes, Valid: true}
	}

	replaceTx, err := store.ReplaceOrderTx(ctx, db.ReplaceOrderTxParams{
		CreateOrderParams: createArgs,
		Items:             items,
		OldOrderID:        oldOrder.ID,
		CancelReason:      "replaced by new order",
	})
	if err != nil {
		return ReplaceOrderResult{}, err
	}

	result := ReplaceOrderResult{
		NewOrder: replaceTx.NewOrder,
		Delta:    delta,
	}

	if delta > 0 {
		payOrder, createErr := createReplaceOrderBaofuPayment(ctx, store, paymentFacade, input.UserID, replaceTx.NewOrder, delta, input.ClientIP)
		if createErr != nil {
			return ReplaceOrderResult{}, createErr
		}
		result.PaymentOrderID = &payOrder.ID
	} else if delta < 0 {
		refundAmount := -delta
		if refundAmount > 0 {
			for _, allocation := range refundAllocations {
				if allocation.RefundAmount <= 0 {
					continue
				}
				refundReason := "订单改菜单退款"
				outRefundNo, err := generateOutRefundNo()
				if err != nil {
					return ReplaceOrderResult{}, fmt.Errorf("generate out refund no: %w", err)
				}

				refundOrder, err := store.CreateRefundOrder(ctx, db.CreateRefundOrderParams{
					PaymentOrderID: allocation.PaymentOrder.ID,
					RefundType:     paymentTypeProfitSharing,
					RefundAmount:   allocation.RefundAmount,
					RefundReason:   pgtype.Text{String: refundReason, Valid: true},
					OutRefundNo:    outRefundNo,
					Status:         "pending",
				})
				if err != nil {
					return ReplaceOrderResult{}, err
				}

				refundStatus, refundID, refundErr := processReplaceOrderRefundWithBaofu(ctx, store, paymentFacade, oldOrder.MerchantID, allocation.PaymentOrder, outRefundNo, refundReason, allocation.RefundAmount)
				if refundErr != nil {
					if _, dbErr := store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
						log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as failed")
					} else {
						recordReplaceReservationRefundCommandRejected(ctx, store, allocation.PaymentOrder, refundOrder, outRefundNo, refundErr)
					}
					return ReplaceOrderResult{}, refundErr
				}
				switch refundStatus {
				case wechatcontracts.DirectRefundStatusSuccess:
					if _, dbErr := store.UpdateRefundOrderToSuccess(ctx, refundOrder.ID); dbErr != nil {
						log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as success")
					}
				case wechatcontracts.DirectRefundStatusProcessing:
					if _, dbErr := store.UpdateRefundOrderToProcessing(ctx, db.UpdateRefundOrderToProcessingParams{ID: refundOrder.ID, RefundID: pgtype.Text{String: refundID, Valid: refundID != ""}}); dbErr != nil {
						log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as processing")
					}
					recordReplaceReservationRefundCommandAccepted(ctx, store, allocation.PaymentOrder, refundOrder, outRefundNo, refundID)
				}
				result.RefundInitiated = true
			}
		}
	}

	return result, nil
}

func createReplaceOrderBaofuPayment(
	ctx context.Context,
	store db.Store,
	paymentFacade PaymentFacade,
	userID int64,
	order db.Order,
	amount int64,
	clientIP string,
) (db.PaymentOrder, error) {
	if paymentFacade == nil {
		return db.PaymentOrder{}, fmt.Errorf("baofu payment facade not configured")
	}
	clientIP = strings.TrimSpace(clientIP)
	if clientIP == "" {
		log.Error().
			Int64("user_id", userID).
			Int64("order_id", order.ID).
			Str("operation", "replace_order_baofu_payment").
			Str("reason", "missing_client_ip").
			Msg("replace order baofu payment missing client ip")
		return db.PaymentOrder{}, NewRequestError(http.StatusBadRequest, errors.New("支付环境信息缺失，请刷新页面后重试"))
	}

	user, err := store.GetUser(ctx, userID)
	if err != nil {
		return db.PaymentOrder{}, fmt.Errorf("get user: %w", err)
	}
	if user.WechatOpenid == "" {
		return db.PaymentOrder{}, NewRequestError(http.StatusBadRequest, errors.New("wechat openid not found"))
	}

	result, err := paymentFacade.CreatePaymentOrder(ctx, CreatePaymentOrderInput{
		UserID:       userID,
		OrderID:      order.ID,
		PaymentType:  paymentTypeMiniProgram,
		BusinessType: businessTypeOrder,
		ClientIP:     clientIP,
		Amount:       amount,
	})
	if err != nil {
		return db.PaymentOrder{}, err
	}
	return result.PaymentOrder, nil
}

func markReplaceReservationPaymentOrderFailedForCleanup(ctx context.Context, store db.Store, paymentOrderID int64) {
	if _, err := store.UpdatePaymentOrderToFailed(ctx, paymentOrderID); err != nil {
		log.Error().Err(err).
			Int64("payment_order_id", paymentOrderID).
			Msg("failed to mark replace reservation payment order failed after prepay update failure")
	}
}

func processReplaceOrderRefundWithBaofu(
	ctx context.Context,
	store db.Store,
	paymentFacade PaymentFacade,
	merchantID int64,
	paymentOrder db.PaymentOrder,
	outRefundNo string,
	reason string,
	refundAmount int64,
) (string, string, error) {
	if paymentFacade == nil {
		return "", "", fmt.Errorf("baofu payment facade not configured")
	}
	refundOrder, err := store.GetRefundOrderByOutRefundNo(ctx, outRefundNo)
	if err != nil {
		return "", "", err
	}
	_ = merchantID
	req := aggregatecontracts.RefundBeforeShareRequest{
		OutTradeNo:      strings.TrimSpace(outRefundNo),
		NotifyURL:       paymentFacade.BaofuRefundNotifyURL(),
		RefundAmountFen: refundAmount,
		TotalAmountFen:  refundAmount,
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
		log.Error().
			Err(LoggableError(err)).
			Int64("payment_order_id", paymentOrder.ID).
			Int64("refund_order_id", refundOrder.ID).
			Str("out_refund_no", outRefundNo).
			Str("origin_out_trade_no", strings.TrimSpace(paymentOrder.OutTradeNo)).
			Bool("origin_trade_no_present", strings.TrimSpace(req.OriginTradeNo) != "").
			Int64("refund_amount", refundAmount).
			Str("operation", "replace_order_baofu_refund").
			Msg("replace order baofu refund create failed")
		return "", "", mapBaofuRefundCreateError(err)
	}
	if baofuRefund == nil || strings.TrimSpace(baofuRefund.TradeNo) == "" {
		err := errors.New("baofu refund returned empty result")
		log.Error().
			Err(err).
			Int64("payment_order_id", paymentOrder.ID).
			Int64("refund_order_id", refundOrder.ID).
			Str("out_refund_no", outRefundNo).
			Str("origin_out_trade_no", strings.TrimSpace(paymentOrder.OutTradeNo)).
			Bool("origin_trade_no_present", strings.TrimSpace(req.OriginTradeNo) != "").
			Int64("refund_amount", refundAmount).
			Str("operation", "replace_order_baofu_refund").
			Msg("replace order baofu refund response invalid")
		return "", "", NewRequestErrorWithCause(http.StatusBadGateway, errors.New("退款提交失败，请稍后重试或联系平台处理"), err)
	}
	if strings.EqualFold(strings.TrimSpace(baofuRefund.ResultCode), aggregatecontracts.BusinessResultCodeFail) {
		err := baofuRefundRejectedRequestError(baofuRefund)
		log.Error().
			Err(LoggableError(err)).
			Int64("payment_order_id", paymentOrder.ID).
			Int64("refund_order_id", refundOrder.ID).
			Str("out_refund_no", outRefundNo).
			Str("origin_out_trade_no", strings.TrimSpace(paymentOrder.OutTradeNo)).
			Bool("origin_trade_no_present", strings.TrimSpace(req.OriginTradeNo) != "").
			Int64("refund_amount", refundAmount).
			Str("result_code", strings.TrimSpace(baofuRefund.ResultCode)).
			Str("refund_state", strings.TrimSpace(baofuRefund.RefundState)).
			Str("error_code", strings.TrimSpace(baofuRefund.ErrorCode)).
			Str("operation", "replace_order_baofu_refund").
			Msg("replace order baofu refund rejected by provider")
		return "", "", err
	}
	refundID := strings.TrimSpace(baofuRefund.TradeNo)
	return string(wechatcontracts.DirectRefundStatusProcessing), refundID, nil
}

func baofuRefundRejectedRequestError(refundResp *aggregatecontracts.RefundResult) error {
	upstreamCode := strings.TrimSpace(refundResp.ErrorCode)
	if upstreamCode == "" {
		upstreamCode = strings.TrimSpace(refundResp.ResultCode)
	}
	cause := &baofu.ProviderError{
		Operation:       "order_refund",
		Capability:      db.ExternalPaymentCapabilityBaofuRefund,
		UpstreamCode:    upstreamCode,
		UpstreamMessage: strings.TrimSpace(refundResp.ErrorMessage),
	}
	return NewRequestErrorWithCause(http.StatusBadGateway, errors.New("退款提交失败，请稍后重试或联系平台处理"), cause)
}

func recordReplaceReservationRefundCommandAccepted(ctx context.Context, store db.Store, paymentOrder db.PaymentOrder, refundOrder db.RefundOrder, outRefundNo string, refundID string) {
	paymentCommandSvc := NewPaymentCommandService(store)
	_, err := paymentCommandSvc.RecordExternalPaymentCommand(ctx, dbReplaceReservationRefundCommandInput(
		paymentOrder,
		refundOrder,
		outRefundNo,
		db.ExternalPaymentCommandStatusAccepted,
		stringPtrIfNotEmpty(refundID),
		nil,
		nil,
		replaceReservationRefundCommandSnapshot(map[string]string{
			"out_refund_no": outRefundNo,
			"refund_id":     refundID,
		}),
	))
	if err != nil {
		log.Error().Err(err).
			Int64("refund_order_id", refundOrder.ID).
			Str("out_refund_no", outRefundNo).
			Str("payment_channel", paymentOrder.PaymentChannel).
			Msg("record replace reservation refund command accepted failed")
	}
}

func recordReplaceReservationRefundCommandRejected(ctx context.Context, store db.Store, paymentOrder db.PaymentOrder, refundOrder db.RefundOrder, outRefundNo string, refundErr error) {
	paymentCommandSvc := NewPaymentCommandService(store)
	errorCode, errorMessage := refundCommandErrorFields(refundErr)
	_, err := paymentCommandSvc.RecordExternalPaymentCommand(ctx, dbReplaceReservationRefundCommandInput(
		paymentOrder,
		refundOrder,
		outRefundNo,
		db.ExternalPaymentCommandStatusRejected,
		nil,
		errorCode,
		errorMessage,
		replaceReservationRefundCommandSnapshot(map[string]string{
			"out_refund_no": outRefundNo,
			"error_code":    stringValue(errorCode),
			"error_message": stringValue(errorMessage),
		}),
	))
	if err != nil {
		log.Error().Err(err).
			Int64("refund_order_id", refundOrder.ID).
			Str("out_refund_no", outRefundNo).
			Str("payment_channel", paymentOrder.PaymentChannel).
			Msg("record replace reservation refund command rejected failed")
	}
}

func dbReplaceReservationRefundCommandInput(
	paymentOrder db.PaymentOrder,
	refundOrder db.RefundOrder,
	outRefundNo string,
	commandStatus string,
	externalSecondaryKey *string,
	lastErrorCode *string,
	lastErrorMessage *string,
	responseSnapshot []byte,
) RecordExternalPaymentCommandInput {
	businessObjectType := "refund_order"
	businessObjectID := refundOrder.ID
	return RecordExternalPaymentCommandInput{
		Provider:             db.ExternalPaymentProviderBaofu,
		Channel:              paymentOrder.PaymentChannel,
		Capability:           db.ExternalPaymentCapabilityBaofuRefund,
		CommandType:          db.ExternalPaymentCommandTypeCreateRefund,
		BusinessOwner:        db.ExternalPaymentBusinessOwnerReservation,
		BusinessObjectType:   &businessObjectType,
		BusinessObjectID:     &businessObjectID,
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalObjectKey:    outRefundNo,
		ExternalSecondaryKey: externalSecondaryKey,
		CommandStatus:        commandStatus,
		LastErrorCode:        lastErrorCode,
		LastErrorMessage:     lastErrorMessage,
		ResponseSnapshot:     responseSnapshot,
	}
}

func refundCommandErrorFields(err error) (*string, *string) {
	loggableErr := LoggableError(err)
	var providerErr *baofu.ProviderError
	if errors.As(loggableErr, &providerErr) {
		if code := strings.TrimSpace(providerErr.UpstreamCode); code != "" {
			return stringPtrIfNotEmpty(code), stringPtrIfNotEmpty(baofu.BaofuCommandMessage(code, providerErr.UpstreamMessage))
		}
	}
	if loggableErr == nil {
		return nil, nil
	}
	return nil, stringPtrIfNotEmpty(strings.TrimSpace(loggableErr.Error()))
}

func replaceReservationRefundCommandSnapshot(values map[string]string) []byte {
	filtered := make(map[string]string, len(values))
	for key, value := range values {
		if value != "" {
			filtered[key] = value
		}
	}
	if len(filtered) == 0 {
		return []byte(`{}`)
	}
	data, err := json.Marshal(filtered)
	if err != nil {
		return []byte(`{}`)
	}
	return data
}

func generateOrderNo() (string, error) {
	now := time.Now()
	dateStr := now.Format("20060102150405")

	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto/rand.Read failed: %w", err)
	}
	randomNum := fmt.Sprintf("%06d", int(b[0])*10000+int(b[1])*100+int(b[2]))

	return dateStr + randomNum[:6], nil
}
