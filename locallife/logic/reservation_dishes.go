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
	UserID                      int64
	ReservationID               int64
	Items                       []ReservationItemInput
	RejectLegacyPackagingDishes bool
	Now                         time.Time
	PaymentFacade               PaymentFacade
	ClientIP                    string
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

	if !isCustomerOwnedReservation(reservation, input.UserID) {
		return result, NewRequestError(http.StatusForbidden, errors.New("you can only add dishes to your own reservation"))
	}
	if reservation.Status != reservationStatusPaid && reservation.Status != reservationStatusConfirmed {
		return result, NewRequestError(http.StatusBadRequest, fmt.Errorf("cannot add dishes to reservation in %s status", reservation.Status))
	}
	if reservation.CookingStartedAt.Valid {
		return result, NewRequestError(http.StatusConflict, errors.New("cooking already started, modification is not allowed"))
	}
	if err := ensureNoActiveReservationAdjustment(ctx, store, reservation.ID); err != nil {
		return result, err
	}

	validatedItems, addedAmount, err := ValidateReservationItems(ctx, store, reservation.MerchantID, input.Items, ValidateReservationItemsOptions{
		RejectLegacyPackagingDishes: input.RejectLegacyPackagingDishes,
	})
	if err != nil {
		return result, err
	}

	if reservation.PaymentMode == paymentModeFull {
		currentTotal, err := store.SumReservationItemsTotal(ctx, reservation.ID)
		if err != nil {
			return result, err
		}
		targetItems, err := buildAddReservationDishesTargetItems(ctx, store, reservation.ID, validatedItems)
		if err != nil {
			return result, err
		}
		result.Reservation = reservation
		result.AddedAmount = addedAmount
		paymentOrder, payParams, err := createReservationAdjustmentPaymentOrder(
			ctx,
			input.PaymentFacade,
			reservation,
			input.UserID,
			currentTotal,
			currentTotal+addedAmount,
			addedAmount,
			targetItems,
			input.Now,
			input.ClientIP,
		)
		if err != nil {
			return result, err
		}
		result.Payment = &paymentOrder
		result.PayParams = payParams
		return result, nil
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

	return result, nil
}

// ModifyReservationDishesInput describes a modify-dishes request.
type ModifyReservationDishesInput struct {
	UserID                      int64
	ReservationID               int64
	Items                       []ReservationItemInput
	RejectLegacyPackagingDishes bool
	Now                         time.Time
	PaymentFacade               PaymentFacade
	ClientIP                    string
	TaskScheduler               TaskScheduler
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

func buildAddReservationDishesTargetItems(
	ctx context.Context,
	store db.Store,
	reservationID int64,
	addedItems []ValidatedReservationItem,
) ([]db.CreateReservationItemParams, error) {
	currentItems, err := store.GetReservationItemsByReservation(ctx, reservationID)
	if err != nil {
		return nil, fmt.Errorf("get current reservation items: %w", err)
	}
	targetItems := make([]db.CreateReservationItemParams, 0, len(currentItems)+len(addedItems))
	for _, item := range currentItems {
		targetItems = append(targetItems, db.CreateReservationItemParams{
			ReservationID: reservationID,
			DishID:        item.DishID,
			ComboID:       item.ComboID,
			Quantity:      item.Quantity,
			UnitPrice:     item.UnitPrice,
			TotalPrice:    item.TotalPrice,
		})
	}
	for _, item := range addedItems {
		var dishID, comboID pgtype.Int8
		if item.DishID != nil {
			dishID = pgtype.Int8{Int64: *item.DishID, Valid: true}
		}
		if item.ComboID != nil {
			comboID = pgtype.Int8{Int64: *item.ComboID, Valid: true}
		}
		targetItems = append(targetItems, db.CreateReservationItemParams{
			ReservationID: reservationID,
			DishID:        dishID,
			ComboID:       comboID,
			Quantity:      item.Quantity,
			UnitPrice:     item.UnitPrice,
			TotalPrice:    item.UnitPrice * int64(item.Quantity),
		})
	}
	return targetItems, nil
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

	if !isCustomerOwnedReservation(reservation, input.UserID) {
		return result, NewRequestError(http.StatusForbidden, errors.New("you can only modify your own reservation"))
	}
	if reservation.Status != reservationStatusPaid && reservation.Status != reservationStatusConfirmed && reservation.Status != reservationStatusCheckedIn {
		return result, NewRequestError(http.StatusBadRequest, fmt.Errorf("cannot modify reservation in %s status", reservation.Status))
	}
	if reservation.CookingStartedAt.Valid {
		return result, NewRequestError(http.StatusConflict, errors.New("cooking already started, modification is not allowed"))
	}
	if err := ensureNoActiveReservationAdjustment(ctx, store, reservation.ID); err != nil {
		return result, err
	}

	currentTotal, err := store.SumReservationItemsTotal(ctx, reservation.ID)
	if err != nil {
		return result, err
	}

	validatedItems, newTotal, err := ValidateReservationItems(ctx, store, reservation.MerchantID, input.Items, ValidateReservationItemsOptions{
		RejectLegacyPackagingDishes: input.RejectLegacyPackagingDishes,
	})
	if err != nil {
		return result, err
	}

	delta := newTotal - currentTotal

	var refundAllocations []reservationRefundAllocation
	if reservation.PaymentMode == paymentModeFull && delta < 0 {
		requiredRefundAmount := minInt64(-delta, reservation.PrepaidAmount)
		refundAllocations, err = buildReservationRefundAllocations(ctx, store, reservation.ID, requiredRefundAmount)
		if err != nil {
			return result, err
		}
		if sumReservationRefundAllocations(refundAllocations) != requiredRefundAmount {
			return result, NewRequestError(http.StatusConflict, errors.New("预订退款资金链路已变化，请刷新后重试"))
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

	result.Reservation = reservation
	result.Delta = delta

	if reservation.PaymentMode == paymentModeFull && delta > 0 {
		paymentOrder, payParams, err := createReservationAdjustmentPaymentOrder(
			ctx,
			input.PaymentFacade,
			reservation,
			input.UserID,
			currentTotal,
			newTotal,
			delta,
			createItems,
			input.Now,
			input.ClientIP,
		)
		if err != nil {
			return result, err
		}
		result.Payment = &paymentOrder
		result.PayParams = payParams
		return result, nil
	}

	refundAmount := sumReservationRefundAllocations(refundAllocations)
	refundOrderParams := make([]db.CreateRefundOrderTxParams, 0, len(refundAllocations))
	refundScheduleInputs := make([]ProcessRefundTaskInput, 0, len(refundAllocations))
	if reservation.PaymentMode == paymentModeFull && delta < 0 && refundAmount > 0 {
		for _, allocation := range refundAllocations {
			if allocation.RefundAmount <= 0 {
				continue
			}
			outRefundNo, genErr := generateOutRefundNo()
			if genErr != nil {
				return result, fmt.Errorf("generate out refund no: %w", genErr)
			}

			refundType := refundTypeForPaymentOrder(allocation.PaymentOrder)
			refundOrderParams = append(refundOrderParams, db.CreateRefundOrderTxParams{
				PaymentOrderID: allocation.PaymentOrder.ID,
				RefundType:     refundType,
				RefundAmount:   allocation.RefundAmount,
				RefundReason:   reservationRefundReason,
				OutRefundNo:    outRefundNo,
			})
			refundScheduleInputs = append(refundScheduleInputs, ProcessRefundTaskInput{
				PaymentOrderID: allocation.PaymentOrder.ID,
				ReservationID:  reservation.ID,
				RefundAmount:   allocation.RefundAmount,
				Reason:         reservationRefundReason,
				OutRefundNo:    outRefundNo,
			})
		}
	}

	if len(refundOrderParams) > 0 {
		if _, err := store.ReplaceReservationItemsWithRefundOrdersTx(ctx, db.ReplaceReservationItemsWithRefundOrdersTxParams{
			ReservationID:         reservation.ID,
			ExpectedCurrentAmount: currentTotal,
			Items:                 createItems,
			RefundOrders:          refundOrderParams,
		}); err != nil {
			if statusCode, ok := db.IsRefundRequestError(err); ok {
				return result, NewRequestError(statusCode, errors.Unwrap(err))
			}
			return result, fmt.Errorf("replace reservation dishes with refund orders: %w", err)
		}
	} else if _, err := store.ReplaceReservationItemsTx(ctx, db.ReplaceReservationItemsTxParams{
		ReservationID:         reservation.ID,
		ExpectedCurrentAmount: currentTotal,
		Items:                 createItems,
	}); err != nil {
		if statusCode, ok := db.IsRefundRequestError(err); ok {
			return result, NewRequestError(statusCode, errors.Unwrap(err))
		}
		return result, err
	}

	if reservation.PaymentMode != paymentModeFull || delta == 0 {
		return result, nil
	}

	if refundAmount <= 0 {
		return result, nil
	}
	for _, scheduleInput := range refundScheduleInputs {
		if input.TaskScheduler == nil {
			log.Error().
				Int64("payment_order_id", scheduleInput.PaymentOrderID).
				Str("out_refund_no", scheduleInput.OutRefundNo).
				Msg("reservation dish change refund task scheduler not configured; relying on recovery scheduler")
			continue
		}

		scheduleErr := input.TaskScheduler.ScheduleProcessRefund(ctx, scheduleInput)
		if scheduleErr != nil {
			log.Error().
				Err(scheduleErr).
				Int64("payment_order_id", scheduleInput.PaymentOrderID).
				Str("out_refund_no", scheduleInput.OutRefundNo).
				Msg("failed to enqueue reservation dish change refund task; pending recovery remains available")
		}
	}

	result.RefundAmount = refundAmount
	result.RefundInitiated = true
	return result, nil
}

func createReservationAdjustmentPaymentOrder(
	ctx context.Context,
	paymentFacade PaymentFacade,
	reservation db.TableReservation,
	userID, currentTotal, targetTotal, amount int64,
	items []db.CreateReservationItemParams,
	now time.Time,
	clientIP string,
) (db.PaymentOrder, *wechat.JSAPIPayParams, error) {
	if amount <= 0 {
		return db.PaymentOrder{}, nil, NewRequestError(http.StatusBadRequest, errors.New("payment amount must be greater than 0"))
	}
	if paymentFacade == nil {
		return db.PaymentOrder{}, nil, fmt.Errorf("baofu payment facade not configured")
	}
	expiresAt := now.Add(30 * time.Minute)
	if now.IsZero() {
		expiresAt = time.Now().Add(30 * time.Minute)
	}
	baofuResult, err := paymentFacade.CreateReservationAdjustmentPaymentOrder(ctx, CreateReservationAdjustmentPaymentInput{
		UserID:        userID,
		ReservationID: reservation.ID,
		MerchantID:    reservation.MerchantID,
		Items:         items,
		CurrentTotal:  currentTotal,
		TargetTotal:   targetTotal,
		DeltaAmount:   amount,
		ClientIP:      clientIP,
		Now:           now,
		ExpiresAt:     expiresAt,
	})
	if err != nil {
		return db.PaymentOrder{}, nil, err
	}
	return baofuResult.PaymentOrder, baofuResult.PayParams, nil
}

func ensureNoActiveReservationAdjustment(ctx context.Context, store db.Store, reservationID int64) error {
	adjustment, err := store.GetActiveReservationAdjustmentByReservation(ctx, reservationID)
	if err == nil {
		if adjustment.PaymentOrderID.Valid {
			return NewRequestError(http.StatusConflict, fmt.Errorf("预订存在待支付改菜补差单，请先完成或关闭支付单 %d", adjustment.PaymentOrderID.Int64))
		}
		return NewRequestError(http.StatusConflict, errors.New("预订存在进行中的改菜补差单，请刷新后重试"))
	}
	if errors.Is(err, db.ErrRecordNotFound) {
		return nil
	}
	return fmt.Errorf("get active reservation adjustment: %w", err)
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
		if paymentOrder.BusinessType != businessTypeReservation &&
			paymentOrder.BusinessType != reservationAddonBusiness &&
			!(paymentOrder.BusinessType == businessTypeOrder && paymentOrder.ReservationID.Valid) {
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
