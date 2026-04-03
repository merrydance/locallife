package logic

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"

	"github.com/rs/zerolog/log"
)

// ReplaceOrderInput defines the input for replacing a reservation order.
type ReplaceOrderInput struct {
	UserID  int64
	OrderID int64
	Items   []OrderItemInput
	Notes   string
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
	paymentClient wechat.PaymentClientInterface,
	ecommerceClient wechat.EcommerceClientInterface,
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
		payOrder, createErr := createReplaceOrderEcommercePayment(ctx, store, ecommerceClient, input.UserID, replaceTx.NewOrder.ID, delta)
		if createErr != nil {
			return ReplaceOrderResult{}, createErr
		}
		result.PaymentOrderID = &payOrder.ID
	} else if delta < 0 {
		refundAmount := -delta
		if refundAmount > 0 && (paymentClient != nil || ecommerceClient != nil) {
			paymentOrder, err := store.GetLatestPaymentOrderByOrder(ctx, db.GetLatestPaymentOrderByOrderParams{
				OrderID:      pgtype.Int8{Int64: oldOrder.ID, Valid: true},
				BusinessType: "order",
			})
			if err != nil {
				return ReplaceOrderResult{}, err
			}
			if paymentOrder.Status == "paid" {
				refundReason := "订单改菜单退款"
				outRefundNo, err := generateOutRefundNo()
				if err != nil {
					return ReplaceOrderResult{}, fmt.Errorf("generate out refund no: %w", err)
				}
				refundOrder, err := store.CreateRefundOrder(ctx, db.CreateRefundOrderParams{
					PaymentOrderID: paymentOrder.ID,
					RefundType:     "partial",
					RefundAmount:   refundAmount,
					RefundReason:   pgtype.Text{String: refundReason, Valid: true},
					OutRefundNo:    outRefundNo,
					Status:         "pending",
				})
				if err != nil {
					return ReplaceOrderResult{}, err
				}

				refundStatus, refundErr := processReplaceOrderRefund(ctx, store, paymentClient, ecommerceClient, oldOrder.MerchantID, paymentOrder, outRefundNo, refundReason, refundAmount)
				if refundErr != nil {
					if _, dbErr := store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
						log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as failed")
					}
					return ReplaceOrderResult{}, refundErr
				}
				switch refundStatus {
				case wechat.RefundStatusSuccess:
					if _, dbErr := store.UpdateRefundOrderToSuccess(ctx, refundOrder.ID); dbErr != nil {
						log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as success")
					}
				case wechat.RefundStatusProcessing:
					if _, dbErr := store.UpdateRefundOrderToProcessing(ctx, db.UpdateRefundOrderToProcessingParams{ID: refundOrder.ID}); dbErr != nil {
						log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as processing")
					}
				}
				result.RefundInitiated = true
			}
		}
	}

	return result, nil
}

func createReplaceOrderEcommercePayment(
	ctx context.Context,
	store db.Store,
	ecommerceClient wechat.EcommerceClientInterface,
	userID int64,
	orderID int64,
	amount int64,
) (db.PaymentOrder, error) {
	if ecommerceClient == nil {
		return db.PaymentOrder{}, fmt.Errorf("ecommerce client: not configured")
	}

	user, err := store.GetUser(ctx, userID)
	if err != nil {
		return db.PaymentOrder{}, fmt.Errorf("get user: %w", err)
	}
	if user.WechatOpenid == "" {
		return db.PaymentOrder{}, NewRequestError(http.StatusBadRequest, errors.New("wechat openid not found"))
	}

	expiresAt := time.Now().Add(30 * time.Minute)
	combineOutTradeNo, err := generateCombineOutTradeNoForSingle("RC")
	if err != nil {
		return db.PaymentOrder{}, fmt.Errorf("generate combine out trade no: %w", err)
	}

	txResult, err := store.CreateCombinedPaymentTx(ctx, db.CreateCombinedPaymentTxParams{
		UserID:            userID,
		OrderIDs:          []int64{orderID},
		CombineOutTradeNo: combineOutTradeNo,
		ExpiresAt:         expiresAt,
	})
	if err != nil {
		if mapped := mapCombinedPaymentError(err); mapped != nil {
			return db.PaymentOrder{}, mapped
		}
		return db.PaymentOrder{}, fmt.Errorf("create combined payment: %w", err)
	}
	if len(txResult.OrderInfos) == 0 {
		return db.PaymentOrder{}, fmt.Errorf("create combined payment: empty order infos")
	}

	info := txResult.OrderInfos[0]
	description := "Order Payment"
	if info.Merchant.Name != "" {
		description = info.Merchant.Name + " - Order Payment"
	}

	combineResp, _, err := ecommerceClient.CreateCombineOrder(ctx, &wechat.CombineOrderRequest{
		CombineOutTradeNo: combineOutTradeNo,
		SubOrders: []wechat.SubOrder{{
			SubMchID:    info.PaymentConfig.SubMchID,
			Amount:      amount,
			OutTradeNo:  info.PaymentOrder.OutTradeNo,
			Description: description,
			Attach:      info.PaymentOrder.Attach.String,
		}},
		PayerOpenID: user.WechatOpenid,
		ExpireTime:  expiresAt,
	})
	if err != nil {
		cleanupCtx := context.Background()
		_, _ = store.UpdatePaymentOrderToClosed(cleanupCtx, info.PaymentOrder.ID)
		_, _ = store.UpdateCombinedPaymentOrderToClosed(cleanupCtx, txResult.CombinedPaymentOrder.ID)
		return db.PaymentOrder{}, fmt.Errorf("create combine order: %w", err)
	}
	if combineResp == nil || strings.TrimSpace(combineResp.PrepayID) == "" {
		cleanupCtx := context.Background()
		_, _ = store.UpdatePaymentOrderToClosed(cleanupCtx, info.PaymentOrder.ID)
		_, _ = store.UpdateCombinedPaymentOrderToClosed(cleanupCtx, txResult.CombinedPaymentOrder.ID)
		return db.PaymentOrder{}, fmt.Errorf("create combine order: empty prepay id")
	}

	updatedPayment, err := store.UpdatePaymentOrderPrepayId(ctx, db.UpdatePaymentOrderPrepayIdParams{
		ID:       info.PaymentOrder.ID,
		PrepayID: pgtype.Text{String: combineResp.PrepayID, Valid: true},
	})
	if err != nil {
		cleanupCtx := context.Background()
		_, _ = store.UpdatePaymentOrderToFailed(cleanupCtx, info.PaymentOrder.ID)
		_, _ = store.UpdateCombinedPaymentOrderToFailed(cleanupCtx, txResult.CombinedPaymentOrder.ID)
		if closeErr := ecommerceClient.CloseCombineOrder(cleanupCtx, combineOutTradeNo, []wechat.SubOrderClose{{
			SubMchID:   info.PaymentConfig.SubMchID,
			OutTradeNo: info.PaymentOrder.OutTradeNo,
		}}); closeErr != nil {
			log.Warn().Err(closeErr).Str("combine_out_trade_no", combineOutTradeNo).Msg("close combine order after prepay update failure")
		}
		return db.PaymentOrder{}, fmt.Errorf("update prepay id: %w", err)
	}

	_, _ = store.UpdateCombinedPaymentOrderPrepay(ctx, db.UpdateCombinedPaymentOrderPrepayParams{
		ID:       txResult.CombinedPaymentOrder.ID,
		PrepayID: pgtype.Text{String: combineResp.PrepayID, Valid: true},
	})

	return updatedPayment, nil
}

func processReplaceOrderRefund(
	ctx context.Context,
	store db.Store,
	paymentClient wechat.PaymentClientInterface,
	ecommerceClient wechat.EcommerceClientInterface,
	merchantID int64,
	paymentOrder db.PaymentOrder,
	outRefundNo string,
	reason string,
	refundAmount int64,
) (string, error) {
	if paymentOrder.PaymentType == "profit_sharing" {
		if ecommerceClient == nil {
			return "", errors.New("ecommerce client not configured")
		}
		paymentConfig, err := store.GetMerchantPaymentConfig(ctx, merchantID)
		if err != nil {
			return "", fmt.Errorf("get merchant payment config: %w", err)
		}
		refundResp, err := ecommerceClient.CreateEcommerceRefund(ctx, &wechat.EcommerceRefundRequest{
			SubMchID:     paymentConfig.SubMchID,
			OutTradeNo:   paymentOrder.OutTradeNo,
			OutRefundNo:  outRefundNo,
			Reason:       reason,
			RefundAmount: refundAmount,
			TotalAmount:  paymentOrder.Amount,
		})
		if err != nil {
			return "", err
		}
		return refundResp.Status, nil
	}

	if paymentClient == nil {
		return "", errors.New("payment client not configured")
	}
	wxRefund, err := paymentClient.CreateRefund(ctx, &wechat.RefundRequest{
		OutTradeNo:   paymentOrder.OutTradeNo,
		OutRefundNo:  outRefundNo,
		Reason:       reason,
		RefundAmount: refundAmount,
		TotalAmount:  paymentOrder.Amount,
	})
	if err != nil {
		return "", err
	}
	return wxRefund.Status, nil
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
