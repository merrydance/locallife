package logic

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"

	"github.com/rs/zerolog/log"
)

const (
	reservationAddonBusiness = "reservation_addon"
	paymentStatusPaid        = "paid"
)

// AddReservationDishesInput describes the add-dishes request.
type AddReservationDishesInput struct {
	UserID          int64
	ReservationID   int64
	Items           []ReservationItemInput
	Now             time.Time
	EcommerceClient wechat.EcommerceClientInterface
	ClientIP        string
}

// AddReservationDishesResult returns the add-dishes outcome.
type AddReservationDishesResult struct {
	Reservation db.TableReservation
	AddedAmount int64
	Payment     *db.PaymentOrder
	PayParams   *wechat.JSAPIPayParams
}

// AddReservationDishes validates and appends reservation items, optionally creating a payment order.
func AddReservationDishes(ctx context.Context, store db.Store, input AddReservationDishesInput) (AddReservationDishesResult, error) {
	var result AddReservationDishesResult

	if len(input.Items) == 0 {
		return result, NewRequestError(http.StatusBadRequest, errors.New("at least one item is required"))
	}

	reservation, err := store.GetTableReservationForUpdate(ctx, input.ReservationID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("reservation not found"))
		}
		return result, err
	}

	if reservation.UserID != input.UserID {
		return result, NewRequestError(http.StatusForbidden, errors.New("you can only add dishes to your own reservation"))
	}
	if reservation.Status != reservationStatusPaid && reservation.Status != reservationStatusConfirmed {
		return result, NewRequestError(http.StatusBadRequest, fmt.Errorf("cannot add dishes to reservation in %s status", reservation.Status))
	}

	validatedItems, addedAmount, err := ValidateReservationItems(ctx, store, reservation.MerchantID, input.Items)
	if err != nil {
		return result, err
	}

	for _, item := range validatedItems {
		var dishID, comboID pgtype.Int8
		if item.DishID != nil {
			dishID = pgtype.Int8{Int64: *item.DishID, Valid: true}
		}
		if item.ComboID != nil {
			comboID = pgtype.Int8{Int64: *item.ComboID, Valid: true}
		}

		_, err := store.CreateReservationItem(ctx, db.CreateReservationItemParams{
			ReservationID: reservation.ID,
			DishID:        dishID,
			ComboID:       comboID,
			Quantity:      item.Quantity,
			UnitPrice:     item.UnitPrice,
			TotalPrice:    item.UnitPrice * int64(item.Quantity),
		})
		if err != nil {
			return result, err
		}
	}

	result.Reservation = reservation
	result.AddedAmount = addedAmount

	if reservation.PaymentMode == paymentModeFull {
		paymentOrder, payParams, err := createReservationAddonPaymentOrder(ctx, store, input.EcommerceClient, reservation, input.UserID, addedAmount, input.Now, input.ClientIP)
		if err != nil {
			return result, err
		}
		result.Payment = &paymentOrder
		result.PayParams = payParams
	}

	return result, nil
}

// ModifyReservationDishesInput describes a modify-dishes request.
type ModifyReservationDishesInput struct {
	UserID          int64
	ReservationID   int64
	Items           []ReservationItemInput
	Now             time.Time
	EcommerceClient wechat.EcommerceClientInterface
	ClientIP        string
}

// ModifyReservationDishesResult returns modify-dishes outcomes.
type ModifyReservationDishesResult struct {
	Reservation     db.TableReservation
	Delta           int64
	Payment         *db.PaymentOrder
	PayParams       *wechat.JSAPIPayParams
	RefundAmount    int64
	RefundInitiated bool
}

// ModifyReservationDishes replaces reservation items and handles payment/refund if needed.
func ModifyReservationDishes(
	ctx context.Context,
	store db.Store,
	ecommerceClient wechat.EcommerceClientInterface,
	input ModifyReservationDishesInput,
) (ModifyReservationDishesResult, error) {
	var result ModifyReservationDishesResult

	if len(input.Items) == 0 {
		return result, NewRequestError(http.StatusBadRequest, errors.New("at least one item is required"))
	}

	reservation, err := store.GetTableReservationForUpdate(ctx, input.ReservationID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("reservation not found"))
		}
		return result, err
	}

	if reservation.UserID != input.UserID {
		return result, NewRequestError(http.StatusForbidden, errors.New("you can only modify your own reservation"))
	}
	if reservation.Status != reservationStatusPaid && reservation.Status != reservationStatusConfirmed && reservation.Status != reservationStatusCheckedIn {
		return result, NewRequestError(http.StatusBadRequest, fmt.Errorf("cannot modify reservation in %s status", reservation.Status))
	}
	if reservation.CookingStartedAt.Valid {
		return result, NewRequestError(http.StatusConflict, errors.New("cooking already started, modification is not allowed"))
	}

	currentTotal, err := store.SumReservationItemsTotal(ctx, reservation.ID)
	if err != nil {
		return result, err
	}

	validatedItems, newTotal, err := ValidateReservationItems(ctx, store, reservation.MerchantID, input.Items)
	if err != nil {
		return result, err
	}

	delta := newTotal - currentTotal

	createItems := make([]db.CreateReservationItemParams, 0, len(validatedItems))
	for _, item := range validatedItems {
		var dishID, comboID pgtype.Int8
		if item.DishID != nil {
			dishID = pgtype.Int8{Int64: *item.DishID, Valid: true}
		}
		if item.ComboID != nil {
			comboID = pgtype.Int8{Int64: *item.ComboID, Valid: true}
		}
		createItems = append(createItems, db.CreateReservationItemParams{
			ReservationID: reservation.ID,
			DishID:        dishID,
			ComboID:       comboID,
			Quantity:      item.Quantity,
			UnitPrice:     item.UnitPrice,
			TotalPrice:    item.UnitPrice * int64(item.Quantity),
		})
	}

	if _, err := store.ReplaceReservationItemsTx(ctx, db.ReplaceReservationItemsTxParams{
		ReservationID: reservation.ID,
		Items:         createItems,
	}); err != nil {
		return result, err
	}

	result.Reservation = reservation
	result.Delta = delta

	if reservation.PaymentMode != paymentModeFull || delta == 0 {
		return result, nil
	}

	if delta > 0 {
		paymentOrder, payParams, err := createReservationAddonPaymentOrder(ctx, store, input.EcommerceClient, reservation, input.UserID, delta, input.Now, input.ClientIP)
		if err != nil {
			return result, err
		}
		result.Payment = &paymentOrder
		result.PayParams = payParams
		return result, nil
	}

	refundAmount := -delta
	if refundAmount > reservation.PrepaidAmount {
		refundAmount = reservation.PrepaidAmount
	}
	if refundAmount <= 0 || ecommerceClient == nil {
		return result, nil
	}

	paymentOrder, err := store.GetLatestPaymentOrderByReservation(ctx, db.GetLatestPaymentOrderByReservationParams{
		ReservationID: pgtype.Int8{Int64: reservation.ID, Valid: true},
		BusinessType:  businessTypeReservation,
	})
	if err != nil {
		return result, err
	}
	if paymentOrder.Status != paymentStatusPaid {
		return result, nil
	}

	outRefundNo, err := generateOutRefundNo()
	if err != nil {
		return result, fmt.Errorf("generate out refund no: %w", err)
	}
	refundOrder, err := store.CreateRefundOrder(ctx, db.CreateRefundOrderParams{
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "partial",
		RefundAmount:   refundAmount,
		RefundReason:   pgtype.Text{String: "Reservation dish change refund", Valid: true},
		OutRefundNo:    outRefundNo,
		Status:         "pending",
	})
	if err != nil {
		return result, err
	}

	wxRefund, err := ecommerceClient.CreateEcommerceRefund(ctx, &wechat.EcommerceRefundRequest{
		SubMchID:    getSingleSubMchID(ctx, store, paymentOrder),
		OutTradeNo:  paymentOrder.OutTradeNo,
		OutRefundNo: outRefundNo,
		Reason:      "Reservation dish change refund",
		RefundAmount: refundAmount,
		TotalAmount:  paymentOrder.Amount,
	})
	if err != nil {
		if _, dbErr := store.UpdateRefundOrderToFailed(ctx, refundOrder.ID); dbErr != nil {
			log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as failed")
		}
		return result, err
	}
	if wxRefund.Status == wechat.RefundStatusSuccess {
		if _, dbErr := store.UpdateRefundOrderToSuccess(ctx, refundOrder.ID); dbErr != nil {
			log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as success")
		}
		if _, dbErr := store.AddReservationPrepaidAmount(ctx, db.AddReservationPrepaidAmountParams{
			ID:            reservation.ID,
			PrepaidAmount: -refundAmount,
		}); dbErr != nil {
			log.Error().Err(dbErr).Int64("reservation_id", reservation.ID).Msg("failed to update reservation prepaid amount")
		}
	}

	result.RefundAmount = refundAmount
	result.RefundInitiated = true
	return result, nil
}

// getSingleSubMchID looks up the sub-merchant ID for an ecommerce refund on a reservation payment order.
func getSingleSubMchID(ctx context.Context, store db.Store, paymentOrder db.PaymentOrder) string {
	if !paymentOrder.ReservationID.Valid {
		return ""
	}
	reservation, err := store.GetTableReservation(ctx, paymentOrder.ReservationID.Int64)
	if err != nil {
		return ""
	}
	cfg, err := store.GetMerchantPaymentConfig(ctx, reservation.MerchantID)
	if err != nil {
		return ""
	}
	return cfg.SubMchID
}

func createReservationAddonPaymentOrder(
	ctx context.Context,
	store db.Store,
	ecommerceClient wechat.EcommerceClientInterface,
	reservation db.TableReservation,
	userID, amount int64,
	now time.Time,
	clientIP string,
) (db.PaymentOrder, *wechat.JSAPIPayParams, error) {
	if amount <= 0 {
		return db.PaymentOrder{}, nil, NewRequestError(http.StatusBadRequest, errors.New("payment amount must be greater than 0"))
	}
	if ecommerceClient == nil {
		return db.PaymentOrder{}, nil, NewRequestError(http.StatusInternalServerError, errors.New("ecommerce client not configured"))
	}

	user, err := store.GetUser(ctx, userID)
	if err != nil {
		return db.PaymentOrder{}, nil, fmt.Errorf("get user: %w", err)
	}
	if user.WechatOpenid == "" {
		return db.PaymentOrder{}, nil, NewRequestError(http.StatusBadRequest, errors.New("wechat openid not found"))
	}

	expiresAt := now.Add(30 * time.Minute)

	combineOutTradeNo, err := generateCombineOutTradeNoForSingle("RA")
	if err != nil {
		return db.PaymentOrder{}, nil, fmt.Errorf("generate combine out trade no: %w", err)
	}

	var txResult db.CreateEcommercePaymentTxResult
	for attempt := 1; attempt <= outTradeNoMaxRetry; attempt++ {
		outTradeNo, genErr := generateOutTradeNoWithPrefix("RA")
		if genErr != nil {
			return db.PaymentOrder{}, nil, fmt.Errorf("generate out trade no: %w", genErr)
		}
		txResult, err = store.CreateEcommercePaymentTx(ctx, db.CreateEcommercePaymentTxParams{
			UserID:            userID,
			MerchantID:        reservation.MerchantID,
			Amount:            amount,
			BusinessType:      reservationAddonBusiness,
			ReservationID:     reservation.ID,
			CombineOutTradeNo: combineOutTradeNo,
			OutTradeNo:        outTradeNo,
			ExpiresAt:         expiresAt,
			Attach:            "",
		})
		if err == nil {
			break
		}
		if isOutTradeNoConflict(err) && attempt < outTradeNoMaxRetry {
			if !sleepWithContext(ctx, outTradeNoRetryBaseBack*time.Duration(attempt)) {
				return db.PaymentOrder{}, nil, NewRequestError(http.StatusRequestTimeout, errors.New("request canceled"))
			}
			continue
		}
		return db.PaymentOrder{}, nil, mapReservationEcommerceError(err)
	}

	combineResp, payParams, err := ecommerceClient.CreateCombineOrder(ctx, &wechat.CombineOrderRequest{
		CombineOutTradeNo: combineOutTradeNo,
		SubOrders: []wechat.SubOrder{
			{
				MchID:       txResult.SubMchID,
				Amount:      amount,
				OutTradeNo:  txResult.PaymentOrder.OutTradeNo,
				Description: "Reservation add-on",
				Attach:      "",
			},
		},
		PayerOpenID: user.WechatOpenid,
		ExpireTime:  expiresAt,
		SceneInfo: &wechat.CombineSceneInfo{
			PayerClientIP: clientIP,
		},
	})
	if err != nil {
		cleanupCtx := context.Background()
		_, _ = store.UpdatePaymentOrderToClosed(cleanupCtx, txResult.PaymentOrder.ID)
		_, _ = store.UpdateCombinedPaymentOrderToClosed(cleanupCtx, txResult.CombinedPaymentOrder.ID)
		return db.PaymentOrder{}, nil, fmt.Errorf("create combine order: %w", err)
	}
	if combineResp == nil || combineResp.PrepayID == "" {
		cleanupCtx := context.Background()
		_, _ = store.UpdatePaymentOrderToClosed(cleanupCtx, txResult.PaymentOrder.ID)
		_, _ = store.UpdateCombinedPaymentOrderToClosed(cleanupCtx, txResult.CombinedPaymentOrder.ID)
		return db.PaymentOrder{}, nil, fmt.Errorf("create combine order: empty prepay id")
	}

	updatedPayment, err := store.UpdatePaymentOrderPrepayId(ctx, db.UpdatePaymentOrderPrepayIdParams{
		ID:       txResult.PaymentOrder.ID,
		PrepayID: pgtype.Text{String: combineResp.PrepayID, Valid: true},
	})
	if err != nil {
		cleanupCtx := context.Background()
		_, _ = store.UpdatePaymentOrderToFailed(cleanupCtx, txResult.PaymentOrder.ID)
		_, _ = store.UpdateCombinedPaymentOrderToFailed(cleanupCtx, txResult.CombinedPaymentOrder.ID)
		return db.PaymentOrder{}, nil, fmt.Errorf("update prepay id: %w", err)
	}

	_, _ = store.UpdateCombinedPaymentOrderPrepay(ctx, db.UpdateCombinedPaymentOrderPrepayParams{
		ID:       txResult.CombinedPaymentOrder.ID,
		PrepayID: pgtype.Text{String: combineResp.PrepayID, Valid: true},
	})

	return updatedPayment, payParams, nil
}
