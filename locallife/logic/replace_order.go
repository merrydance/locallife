package logic

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"

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
		payOrder, createErr := createReplaceOrderEcommercePayment(ctx, store, ecommerceClient, input.UserID, replaceTx.NewOrder, delta)
		if createErr != nil {
			return ReplaceOrderResult{}, createErr
		}
		result.PaymentOrderID = &payOrder.ID
	} else if delta < 0 {
		refundAmount := -delta
		if refundAmount > 0 && (paymentClient != nil || ecommerceClient != nil) {
			refundAllocations, err := buildReservationRefundAllocations(ctx, store, reservation.ID, refundAmount)
			if err != nil {
				return ReplaceOrderResult{}, err
			}
			allocatedRefundAmount := sumReservationRefundAllocations(refundAllocations)
			if allocatedRefundAmount != refundAmount {
				return ReplaceOrderResult{}, NewRequestError(http.StatusConflict, errors.New("reservation refund funding chain changed, please retry"))
			}
			for _, allocation := range refundAllocations {
				if allocation.RefundAmount <= 0 {
					continue
				}
				refundReason := "订单改菜单退款"
				outRefundNo, err := generateOutRefundNo()
				if err != nil {
					return ReplaceOrderResult{}, fmt.Errorf("generate out refund no: %w", err)
				}

				refundType := allocation.PaymentOrder.PaymentType
				if refundType == paymentTypeNative {
					refundType = paymentTypeMiniProgram
				}
				refundOrder, err := store.CreateRefundOrder(ctx, db.CreateRefundOrderParams{
					PaymentOrderID: allocation.PaymentOrder.ID,
					RefundType:     refundType,
					RefundAmount:   allocation.RefundAmount,
					RefundReason:   pgtype.Text{String: refundReason, Valid: true},
					OutRefundNo:    outRefundNo,
					Status:         "pending",
				})
				if err != nil {
					return ReplaceOrderResult{}, err
				}

				refundStatus, refundID, refundErr := processReplaceOrderRefund(ctx, store, paymentClient, ecommerceClient, oldOrder.MerchantID, allocation.PaymentOrder, outRefundNo, refundReason, allocation.RefundAmount)
				if refundErr != nil {
					if _, dbErr := store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
						log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as failed")
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
	order db.Order,
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
	merchantName := "Order Payment"
	if merchant, err := store.GetMerchant(ctx, order.MerchantID); err == nil && merchant.Name != "" {
		merchantName = merchant.Name + " - Reservation Adjustment"
	}

	var txResult db.CreatePartnerPaymentTxResult
	for attempt := 1; attempt <= outTradeNoMaxRetry; attempt++ {
		outTradeNo, genErr := generateOutTradeNoWithPrefix("RO")
		if genErr != nil {
			return db.PaymentOrder{}, fmt.Errorf("generate out trade no: %w", genErr)
		}
		txResult, err = store.CreatePartnerPaymentTx(ctx, db.CreatePartnerPaymentTxParams{
			UserID:        userID,
			MerchantID:    order.MerchantID,
			OrderID:       order.ID,
			ReservationID: order.ReservationID.Int64,
			BusinessType:  businessTypeOrder,
			Amount:        amount,
			OutTradeNo:    outTradeNo,
			ExpiresAt:     expiresAt,
			Attach:        fmt.Sprintf("order_id:%d", order.ID),
		})
		if err == nil {
			break
		}
		if isOutTradeNoConflict(err) && attempt < outTradeNoMaxRetry {
			continue
		}
		if status, ok := db.IsPartnerPaymentRequestError(err); ok {
			return db.PaymentOrder{}, NewRequestError(status, errors.New(err.Error()))
		}
		return db.PaymentOrder{}, fmt.Errorf("create partner payment: %w", err)
	}

	orderResp, _, err := ecommerceClient.CreatePartnerJSAPIOrder(ctx, &wechatcontracts.PartnerJSAPIOrderRequest{
		SubMchID:      txResult.SubMchID,
		Description:   merchantName,
		OutTradeNo:    txResult.PaymentOrder.OutTradeNo,
		ExpireTime:    expiresAt,
		Attach:        fmt.Sprintf("order_id:%d", order.ID),
		TotalAmount:   amount,
		PayerOpenID:   user.WechatOpenid,
		ProfitSharing: order.ReservationID.Valid || shouldEnableOrderProfitSharing(order.OrderType),
	})
	if err != nil {
		cleanupCtx := context.Background()
		_, _ = store.UpdatePaymentOrderToClosed(cleanupCtx, txResult.PaymentOrder.ID)
		return db.PaymentOrder{}, fmt.Errorf("create partner jsapi order: %w", err)
	}
	if orderResp == nil || orderResp.PrepayID == "" {
		cleanupCtx := context.Background()
		_, _ = store.UpdatePaymentOrderToClosed(cleanupCtx, txResult.PaymentOrder.ID)
		return db.PaymentOrder{}, fmt.Errorf("create partner jsapi order: empty prepay id")
	}

	updatedPayment, err := store.UpdatePaymentOrderPrepayId(ctx, db.UpdatePaymentOrderPrepayIdParams{
		ID:       txResult.PaymentOrder.ID,
		PrepayID: pgtype.Text{String: orderResp.PrepayID, Valid: true},
	})
	if err != nil {
		cleanupCtx := context.Background()
		_, _ = store.UpdatePaymentOrderToFailed(cleanupCtx, txResult.PaymentOrder.ID)
		if closeErr := ecommerceClient.ClosePartnerOrder(cleanupCtx, txResult.PaymentOrder.OutTradeNo, txResult.SubMchID); closeErr != nil {
			log.Warn().Err(closeErr).Str("out_trade_no", txResult.PaymentOrder.OutTradeNo).Msg("close partner order after prepay update failure")
		}
		return db.PaymentOrder{}, fmt.Errorf("update prepay id: %w", err)
	}

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
) (string, string, error) {
	if paymentOrder.PaymentType == "profit_sharing" {
		if ecommerceClient == nil {
			return "", "", errors.New("ecommerce client not configured")
		}
		paymentConfig, err := store.GetMerchantPaymentConfig(ctx, merchantID)
		if err != nil {
			return "", "", fmt.Errorf("get merchant payment config: %w", err)
		}
		refundResp, err := createEcommerceRefundContract(ctx, ecommerceClient, &wechatcontracts.EcommerceRefundRequest{
			SubMchID:    paymentConfig.SubMchID,
			OutTradeNo:  paymentOrder.OutTradeNo,
			OutRefundNo: outRefundNo,
			Reason:      reason,
			Amount: &wechatcontracts.EcommerceRefundRequestAmount{
				Refund:   refundAmount,
				Total:    paymentOrder.Amount,
				Currency: wechatcontracts.EcommerceRefundCurrencyCNY,
			},
		})
		if err != nil {
			return "", "", mapEcommerceRefundCreateError(err)
		}
		return wechatcontracts.EcommerceRefundStatusProcessing, refundResp.RefundID, nil
	}

	if paymentClient == nil {
		return "", "", errors.New("payment client not configured")
	}
	wxRefund, err := createDirectRefundContract(ctx, paymentClient, &wechatcontracts.DirectRefundRequest{
		OutTradeNo:  paymentOrder.OutTradeNo,
		OutRefundNo: outRefundNo,
		Reason:      reason,
		Amount: &wechatcontracts.DirectRefundRequestAmount{
			Refund:   refundAmount,
			Total:    paymentOrder.Amount,
			Currency: wechatcontracts.DirectRefundCurrencyCNY,
		},
	})
	if err != nil {
		return "", "", mapDirectRefundCreateError(err)
	}
	return wxRefund.Status, wxRefund.RefundID, nil
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
