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
	reservationRefundReason  = "Reservation dish change refund"
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
	TaskScheduler   TaskScheduler
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

type reservationRefundAllocation struct {
	PaymentOrder db.PaymentOrder
	RefundAmount int64
}

// ModifyReservationDishes replaces reservation items and handles payment/refund if needed.
func ModifyReservationDishes(
	ctx context.Context,
	store db.Store,
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

	var refundAllocations []reservationRefundAllocation
	if reservation.PaymentMode == paymentModeFull && delta < 0 {
		refundAllocations, err = buildReservationRefundAllocations(ctx, store, reservation.ID, minInt64(-delta, reservation.PrepaidAmount))
		if err != nil {
			return result, err
		}
	}

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

	refundAmount := sumReservationRefundAllocations(refundAllocations)
	if refundAmount <= 0 {
		return result, nil
	}
	for _, allocation := range refundAllocations {
		outRefundNo, genErr := generateOutRefundNo()
		if genErr != nil {
			return result, fmt.Errorf("generate out refund no: %w", genErr)
		}

		refundType := allocation.PaymentOrder.PaymentType
		if refundType == paymentTypeNative {
			refundType = paymentTypeMiniProgram
		}

		if _, createErr := store.CreateRefundOrderTx(ctx, db.CreateRefundOrderTxParams{
			PaymentOrderID: allocation.PaymentOrder.ID,
			RefundType:     refundType,
			RefundAmount:   allocation.RefundAmount,
			RefundReason:   reservationRefundReason,
			OutRefundNo:    outRefundNo,
		}); createErr != nil {
			if statusCode, ok := db.IsRefundRequestError(createErr); ok {
				return result, NewRequestError(statusCode, errors.Unwrap(createErr))
			}
			return result, fmt.Errorf("create reservation dish change refund order: %w", createErr)
		}

		if input.TaskScheduler == nil {
			log.Error().
				Int64("payment_order_id", allocation.PaymentOrder.ID).
				Str("out_refund_no", outRefundNo).
				Msg("reservation dish change refund task scheduler not configured; relying on recovery scheduler")
			continue
		}

		scheduleErr := input.TaskScheduler.ScheduleProcessRefund(ctx, ProcessRefundTaskInput{
			PaymentOrderID: allocation.PaymentOrder.ID,
			ReservationID:  reservation.ID,
			RefundAmount:   allocation.RefundAmount,
			Reason:         reservationRefundReason,
			OutRefundNo:    outRefundNo,
		})
		if scheduleErr != nil {
			log.Error().
				Err(scheduleErr).
				Int64("payment_order_id", allocation.PaymentOrder.ID).
				Str("out_refund_no", outRefundNo).
				Msg("failed to enqueue reservation dish change refund task; pending recovery remains available")
		}
	}

	result.RefundAmount = refundAmount
	result.RefundInitiated = true
	return result, nil
}

func buildReservationRefundAllocations(
	ctx context.Context,
	store db.Store,
	reservationID int64,
	desiredRefundAmount int64,
) ([]reservationRefundAllocation, error) {
	if desiredRefundAmount <= 0 {
		return nil, nil
	}

	paymentOrders, err := store.GetPaymentOrdersByReservation(ctx, pgtype.Int8{Int64: reservationID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("get payment orders by reservation: %w", err)
	}

	allocations := make([]reservationRefundAllocation, 0)
	remainingRefund := desiredRefundAmount
	for _, paymentOrder := range paymentOrders {
		if remainingRefund <= 0 {
			break
		}
		if paymentOrder.Status != paymentStatusPaid {
			continue
		}
		if paymentOrder.BusinessType != businessTypeReservation && paymentOrder.BusinessType != reservationAddonBusiness {
			continue
		}

		occupiedAmount, occupiedErr := store.GetTotalRefundedByPaymentOrder(ctx, paymentOrder.ID)
		if occupiedErr != nil {
			return nil, fmt.Errorf("get occupied refund amount by payment order %d: %w", paymentOrder.ID, occupiedErr)
		}

		availableAmount := paymentOrder.Amount - occupiedAmount
		if availableAmount <= 0 {
			continue
		}

		allocationAmount := minInt64(remainingRefund, availableAmount)
		allocations = append(allocations, reservationRefundAllocation{
			PaymentOrder: paymentOrder,
			RefundAmount: allocationAmount,
		})
		remainingRefund -= allocationAmount
	}

	return allocations, nil
}

func sumReservationRefundAllocations(allocations []reservationRefundAllocation) int64 {
	var total int64
	for _, allocation := range allocations {
		total += allocation.RefundAmount
	}
	return total
}

func minInt64(left, right int64) int64 {
	if left < right {
		return left
	}
	return right
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
