package db

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type CreateReservationPositiveAdjustmentPaymentTxParams struct {
	ReservationID         int64
	UserID                int64
	MerchantID            int64
	ExpectedCurrentAmount int64
	TargetTotal           int64
	DeltaAmount           int64
	Items                 []CreateReservationItemParams
	OutTradeNo            string
	ExpiresAt             time.Time
	Attach                string
}

type CreateReservationPositiveAdjustmentPaymentTxResult struct {
	Reservation  TableReservation
	Adjustment   ReservationAdjustment
	Items        []ReservationAdjustmentItem
	Holds        []ReservationAdjustmentInventoryHold
	PaymentOrder PaymentOrder
	SubMchID     string
}

type ApplyPaidReservationAdjustmentTxParams struct {
	PaymentOrderID int64
}

type ApplyPaidReservationAdjustmentTxResult struct {
	PaymentOrder PaymentOrder
	Adjustment   ReservationAdjustment
	Reservation  TableReservation
	Processed    bool
}

type CloseReservationAdjustmentForPaymentTxParams struct {
	PaymentOrderID int64
	Status         string
	Reason         string
}

type CloseReservationAdjustmentForPaymentTxResult struct {
	PaymentOrder PaymentOrder
	Adjustment   ReservationAdjustment
	Closed       bool
}

func (store *SQLStore) CreateReservationPositiveAdjustmentPaymentTx(
	ctx context.Context,
	arg CreateReservationPositiveAdjustmentPaymentTxParams,
) (CreateReservationPositiveAdjustmentPaymentTxResult, error) {
	var result CreateReservationPositiveAdjustmentPaymentTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		reservation, err := q.GetTableReservationForUpdate(ctx, arg.ReservationID)
		if err != nil {
			if errors.Is(err, ErrRecordNotFound) {
				return &requestError{statusCode: http.StatusNotFound, err: errors.New("reservation not found")}
			}
			return fmt.Errorf("lock reservation: %w", err)
		}
		result.Reservation = reservation

		if !isCustomerOwnedReservation(reservation, arg.UserID) {
			return &requestError{statusCode: http.StatusForbidden, err: fmt.Errorf("reservation %d does not belong to user", arg.ReservationID)}
		}
		if arg.MerchantID > 0 && reservation.MerchantID != arg.MerchantID {
			return &requestError{statusCode: http.StatusConflict, err: fmt.Errorf("reservation %d merchant changed", arg.ReservationID)}
		}
		if reservation.PaymentMode != "full" {
			return &requestError{statusCode: http.StatusBadRequest, err: fmt.Errorf("reservation %d payment mode changed", arg.ReservationID)}
		}
		if reservation.Status != "paid" && reservation.Status != "confirmed" && reservation.Status != "checked_in" {
			return &requestError{statusCode: http.StatusBadRequest, err: fmt.Errorf("reservation %d status is %s, expect paid/confirmed/checked_in", arg.ReservationID, reservation.Status)}
		}
		if reservation.CookingStartedAt.Valid {
			return &requestError{statusCode: http.StatusConflict, err: errors.New("reservation cooking already started")}
		}
		if arg.DeltaAmount <= 0 || arg.TargetTotal-arg.ExpectedCurrentAmount != arg.DeltaAmount {
			return &requestError{statusCode: http.StatusBadRequest, err: errors.New("positive reservation adjustment amount is invalid")}
		}
		currentTotal, err := q.SumReservationItemsTotal(ctx, arg.ReservationID)
		if err != nil {
			return fmt.Errorf("sum reservation items total: %w", err)
		}
		if currentTotal != arg.ExpectedCurrentAmount {
			return &requestError{statusCode: http.StatusConflict, err: errors.New("预订菜品金额已变化，请刷新后重试")}
		}
		if currentTotal+arg.DeltaAmount != arg.TargetTotal {
			return &requestError{statusCode: http.StatusBadRequest, err: errors.New("positive reservation adjustment target total is invalid")}
		}
		if err := validateReservationAdjustmentTargetItems(arg.Items, arg.TargetTotal); err != nil {
			return err
		}
		if _, err := q.GetActiveReservationAdjustmentByReservation(ctx, arg.ReservationID); err == nil {
			return &requestError{statusCode: http.StatusConflict, err: errors.New("reservation has active adjustment")}
		} else if !errors.Is(err, ErrRecordNotFound) {
			return fmt.Errorf("get active reservation adjustment: %w", err)
		}

		paymentConfig, err := q.GetMerchantPaymentConfig(ctx, reservation.MerchantID)
		if err != nil {
			return fmt.Errorf("get merchant payment config for merchant %d: %w", reservation.MerchantID, err)
		}
		if paymentConfig.Status != MerchantPaymentConfigStatusActive || paymentConfig.SubMchID == "" {
			return &requestError{statusCode: http.StatusBadRequest, err: fmt.Errorf("merchant %d payment config invalid or inactive", reservation.MerchantID)}
		}
		result.SubMchID = paymentConfig.SubMchID

		adjustment, err := q.CreateReservationAdjustment(ctx, CreateReservationAdjustmentParams{
			ReservationID: reservation.ID,
			UserID:        reservation.UserID,
			MerchantID:    reservation.MerchantID,
			Direction:     ReservationAdjustmentDirectionPositive,
			Status:        ReservationAdjustmentStatusCreatingPayment,
			CurrentTotal:  currentTotal,
			TargetTotal:   arg.TargetTotal,
			DeltaAmount:   arg.DeltaAmount,
		})
		if err != nil {
			return fmt.Errorf("create reservation adjustment: %w", err)
		}
		result.Adjustment = adjustment

		result.Items = make([]ReservationAdjustmentItem, 0, len(arg.Items))
		for index, item := range arg.Items {
			created, err := q.CreateReservationAdjustmentItem(ctx, CreateReservationAdjustmentItemParams{
				AdjustmentID: adjustment.ID,
				DishID:       item.DishID,
				ComboID:      item.ComboID,
				Quantity:     item.Quantity,
				UnitPrice:    item.UnitPrice,
				TotalPrice:   item.TotalPrice,
				Position:     int32(index),
			})
			if err != nil {
				return fmt.Errorf("create reservation adjustment item: %w", err)
			}
			result.Items = append(result.Items, created)
		}

		if err := createReservationAdjustmentInventoryHolds(ctx, q, reservation, adjustment.ID, arg.Items, arg.ExpiresAt, &result); err != nil {
			return err
		}

		attach := arg.Attach
		if attach != "" {
			attach = attach + ";sub_mchid:" + paymentConfig.SubMchID
		}
		paymentOrder, err := q.CreatePaymentOrder(ctx, CreatePaymentOrderParams{
			ReservationID:         pgtype.Int8{Int64: reservation.ID, Valid: true},
			UserID:                reservation.UserID,
			PaymentType:           "miniprogram",
			PaymentChannel:        PaymentChannelBaofuAggregate,
			RequiresProfitSharing: true,
			BusinessType:          "reservation_addon",
			Amount:                arg.DeltaAmount,
			OutTradeNo:            arg.OutTradeNo,
			ExpiresAt:             pgtype.Timestamptz{Time: arg.ExpiresAt, Valid: true},
			Attach:                textIfNotEmpty(attach),
		})
		if err != nil {
			return fmt.Errorf("create reservation adjustment payment order: %w", err)
		}
		result.PaymentOrder = paymentOrder

		adjustment, err = q.LinkReservationAdjustmentPaymentOrder(ctx, LinkReservationAdjustmentPaymentOrderParams{
			ID:             adjustment.ID,
			PaymentOrderID: pgtype.Int8{Int64: paymentOrder.ID, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("link reservation adjustment payment order: %w", err)
		}
		result.Adjustment = adjustment
		return nil
	})

	return result, err
}

func validateReservationAdjustmentTargetItems(items []CreateReservationItemParams, targetTotal int64) error {
	var itemsTotal int64
	for _, item := range items {
		if item.Quantity <= 0 {
			return &requestError{statusCode: http.StatusBadRequest, err: errors.New("positive reservation adjustment item quantity is invalid")}
		}
		if item.UnitPrice < 0 || item.TotalPrice < 0 || item.TotalPrice != item.UnitPrice*int64(item.Quantity) {
			return &requestError{statusCode: http.StatusBadRequest, err: errors.New("positive reservation adjustment item amount is invalid")}
		}
		itemsTotal += item.TotalPrice
	}
	if itemsTotal != targetTotal {
		return &requestError{statusCode: http.StatusBadRequest, err: errors.New("positive reservation adjustment items total is invalid")}
	}
	return nil
}

func (store *SQLStore) ApplyPaidReservationAdjustmentTx(
	ctx context.Context,
	arg ApplyPaidReservationAdjustmentTxParams,
) (ApplyPaidReservationAdjustmentTxResult, error) {
	var result ApplyPaidReservationAdjustmentTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		paymentOrder, err := q.GetPaymentOrderForUpdate(ctx, arg.PaymentOrderID)
		if err != nil {
			return fmt.Errorf("get payment order: %w", err)
		}
		applyResult, err := applyPaidReservationAdjustmentWithQueries(ctx, q, paymentOrder)
		if err != nil {
			return err
		}
		result = applyResult
		return nil
	})

	return result, err
}

func applyPaidReservationAdjustmentWithQueries(
	ctx context.Context,
	q *Queries,
	paymentOrder PaymentOrder,
) (ApplyPaidReservationAdjustmentTxResult, error) {
	var result ApplyPaidReservationAdjustmentTxResult
	result.PaymentOrder = paymentOrder
	if paymentOrder.Status != "paid" {
		return result, nil
	}

	adjustment, err := q.GetReservationAdjustmentByPaymentOrderForUpdate(ctx, pgtype.Int8{Int64: paymentOrder.ID, Valid: true})
	if err != nil {
		return result, fmt.Errorf("get reservation adjustment by payment order: %w", err)
	}
	result.Adjustment = adjustment
	if paymentOrder.ProcessedAt.Valid && adjustment.Status == ReservationAdjustmentStatusApplied {
		return result, nil
	}
	if adjustment.Status == ReservationAdjustmentStatusApplied {
		processedOrder, err := q.UpdatePaymentOrderProcessedAt(ctx, paymentOrder.ID)
		if err != nil && !errors.Is(err, ErrRecordNotFound) {
			return result, fmt.Errorf("mark payment order processed: %w", err)
		}
		if err == nil {
			result.PaymentOrder = processedOrder
			result.Processed = true
		}
		return result, nil
	}
	if _, err := q.MarkReservationAdjustmentApplying(ctx, adjustment.ID); err != nil {
		return result, fmt.Errorf("mark reservation adjustment applying: %w", err)
	}

	reservation, err := q.GetTableReservationForUpdate(ctx, adjustment.ReservationID)
	if err != nil {
		return result, fmt.Errorf("lock reservation: %w", err)
	}
	result.Reservation = reservation
	if reservation.Status != "paid" && reservation.Status != "confirmed" && reservation.Status != "checked_in" {
		return result, fmt.Errorf("reservation %d status is %s, cannot apply paid adjustment", reservation.ID, reservation.Status)
	}
	if reservation.CookingStartedAt.Valid {
		return result, fmt.Errorf("reservation %d cooking already started, cannot apply paid adjustment", reservation.ID)
	}

	if _, err := q.GetReservationPaymentByPaymentOrderID(ctx, paymentOrder.ID); err != nil {
		if !errors.Is(err, ErrRecordNotFound) {
			return result, fmt.Errorf("get reservation addon payment by payment order: %w", err)
		}
		if _, err := q.CreateReservationPayment(ctx, CreateReservationPaymentParams{
			ReservationID:  reservation.ID,
			PaymentOrderID: paymentOrder.ID,
			Amount:         paymentOrder.Amount,
			Type:           "addon",
		}); err != nil {
			return result, fmt.Errorf("create reservation addon payment: %w", err)
		}
		reservation, err = q.AddReservationPrepaidAmount(ctx, AddReservationPrepaidAmountParams{
			ID:            reservation.ID,
			PrepaidAmount: paymentOrder.Amount,
		})
		if err != nil {
			return result, fmt.Errorf("add reservation prepaid amount: %w", err)
		}
		result.Reservation = reservation
	}

	items, err := q.ListReservationAdjustmentItems(ctx, adjustment.ID)
	if err != nil {
		return result, fmt.Errorf("list reservation adjustment items: %w", err)
	}
	if err := q.DeleteReservationItems(ctx, reservation.ID); err != nil {
		return result, fmt.Errorf("delete reservation items: %w", err)
	}
	for _, item := range items {
		if _, err := q.CreateReservationItem(ctx, CreateReservationItemParams{
			ReservationID: reservation.ID,
			DishID:        item.DishID,
			ComboID:       item.ComboID,
			Quantity:      item.Quantity,
			UnitPrice:     item.UnitPrice,
			TotalPrice:    item.TotalPrice,
		}); err != nil {
			return result, fmt.Errorf("create reservation item from adjustment: %w", err)
		}
	}

	holds, err := q.ListReservationAdjustmentInventoryHoldsForUpdate(ctx, adjustment.ID)
	if err != nil {
		return result, fmt.Errorf("list reservation adjustment inventory holds: %w", err)
	}
	for _, hold := range holds {
		if hold.Status != ReservationAdjustmentHoldStatusHeld {
			continue
		}
		if _, err := q.MarkReservationAdjustmentHoldConverted(ctx, hold.ID); err != nil {
			return result, fmt.Errorf("mark reservation adjustment hold converted: %w", err)
		}
	}
	if err := applyReservationAdjustmentInventoryWithQueries(ctx, q, reservation, items, holds); err != nil {
		return result, err
	}

	adjustment, err = q.MarkReservationAdjustmentApplied(ctx, adjustment.ID)
	if err != nil {
		return result, fmt.Errorf("mark reservation adjustment applied: %w", err)
	}
	result.Adjustment = adjustment

	processedOrder, err := q.UpdatePaymentOrderProcessedAt(ctx, paymentOrder.ID)
	if err != nil {
		return result, fmt.Errorf("mark payment order processed: %w", err)
	}
	result.PaymentOrder = processedOrder
	result.Processed = true
	return result, nil
}

func (store *SQLStore) CloseReservationAdjustmentForPaymentTx(
	ctx context.Context,
	arg CloseReservationAdjustmentForPaymentTxParams,
) (CloseReservationAdjustmentForPaymentTxResult, error) {
	var result CloseReservationAdjustmentForPaymentTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		paymentOrder, err := q.GetPaymentOrderForUpdate(ctx, arg.PaymentOrderID)
		if err != nil {
			return fmt.Errorf("get payment order: %w", err)
		}
		result.PaymentOrder = paymentOrder
		adjustment, err := q.GetReservationAdjustmentByPaymentOrderForUpdate(ctx, pgtype.Int8{Int64: paymentOrder.ID, Valid: true})
		if err != nil {
			return fmt.Errorf("get reservation adjustment by payment order: %w", err)
		}
		result.Adjustment = adjustment
		switch adjustment.Status {
		case ReservationAdjustmentStatusApplied,
			ReservationAdjustmentStatusClosed,
			ReservationAdjustmentStatusFailed,
			ReservationAdjustmentStatusExpired:
			return nil
		}
		if err := releaseReservationAdjustmentHolds(ctx, q, adjustment.ID); err != nil {
			return err
		}
		reason := pgtype.Text{String: arg.Reason, Valid: arg.Reason != ""}
		switch arg.Status {
		case ReservationAdjustmentStatusFailed:
			adjustment, err = q.MarkReservationAdjustmentFailed(ctx, MarkReservationAdjustmentFailedParams{ID: adjustment.ID, FailureReason: reason})
		case ReservationAdjustmentStatusExpired:
			adjustment, err = q.MarkReservationAdjustmentExpired(ctx, MarkReservationAdjustmentExpiredParams{ID: adjustment.ID, CloseReason: reason})
		default:
			adjustment, err = q.MarkReservationAdjustmentClosed(ctx, MarkReservationAdjustmentClosedParams{ID: adjustment.ID, CloseReason: reason})
		}
		if err != nil {
			return fmt.Errorf("mark reservation adjustment terminal: %w", err)
		}
		result.Adjustment = adjustment
		result.Closed = true
		return nil
	})

	return result, err
}

func applyReservationAdjustmentInventoryWithQueries(
	ctx context.Context,
	q *Queries,
	reservation TableReservation,
	items []ReservationAdjustmentItem,
	holds []ReservationAdjustmentInventoryHold,
) error {
	target := map[int64]int32{}
	for _, item := range items {
		if item.DishID.Valid {
			target[item.DishID.Int64] += int32(item.Quantity)
		}
	}
	held := map[int64]int32{}
	for _, hold := range holds {
		if hold.Status == ReservationAdjustmentHoldStatusHeld {
			held[hold.DishID] += hold.Quantity
		}
	}

	existing, err := q.ListReservationInventoryByReservation(ctx, reservation.ID)
	if err != nil {
		return fmt.Errorf("list reservation inventory: %w", err)
	}
	existingMap := make(map[int64]int32, len(existing))
	for _, entry := range existing {
		existingMap[entry.DishID] = entry.Quantity
	}

	for dishID, desired := range target {
		current := existingMap[dishID]
		delta := desired - current
		if delta > 0 && held[dishID] < delta {
			return fmt.Errorf("reservation adjustment inventory hold is insufficient for dish %d", dishID)
		}
		if delta < 0 {
			if err := releaseReservedInventoryIfTracked(ctx, q, ReleaseReservedInventoryParams{
				MerchantID:       reservation.MerchantID,
				DishID:           dishID,
				Date:             reservation.ReservationDate,
				ReservedQuantity: -delta,
			}); err != nil {
				return fmt.Errorf("release reservation adjustment reduced inventory: %w", err)
			}
		}
		if _, err := q.UpsertReservationInventory(ctx, UpsertReservationInventoryParams{
			ReservationID: reservation.ID,
			DishID:        dishID,
			Quantity:      desired,
		}); err != nil {
			return fmt.Errorf("upsert reservation adjustment inventory: %w", err)
		}
		delete(existingMap, dishID)
	}

	for dishID, quantity := range existingMap {
		if quantity > 0 {
			if err := releaseReservedInventoryIfTracked(ctx, q, ReleaseReservedInventoryParams{
				MerchantID:       reservation.MerchantID,
				DishID:           dishID,
				Date:             reservation.ReservationDate,
				ReservedQuantity: quantity,
			}); err != nil {
				return fmt.Errorf("release removed reservation adjustment inventory: %w", err)
			}
		}
		if err := q.DeleteReservationInventoryByDish(ctx, DeleteReservationInventoryByDishParams{
			ReservationID: reservation.ID,
			DishID:        dishID,
		}); err != nil {
			return fmt.Errorf("delete removed reservation adjustment inventory: %w", err)
		}
	}
	return nil
}

func createReservationAdjustmentInventoryHolds(
	ctx context.Context,
	q *Queries,
	reservation TableReservation,
	adjustmentID int64,
	targetItems []CreateReservationItemParams,
	expiresAt time.Time,
	result *CreateReservationPositiveAdjustmentPaymentTxResult,
) error {
	target := map[int64]int32{}
	for _, item := range targetItems {
		if item.DishID.Valid {
			target[item.DishID.Int64] += int32(item.Quantity)
		}
	}
	existing, err := q.ListReservationInventoryByReservation(ctx, reservation.ID)
	if err != nil {
		return fmt.Errorf("list reservation inventory: %w", err)
	}
	for _, entry := range existing {
		if target[entry.DishID] <= entry.Quantity {
			delete(target, entry.DishID)
			continue
		}
		target[entry.DishID] -= entry.Quantity
	}
	for dishID, delta := range target {
		if delta <= 0 {
			continue
		}
		if _, err := q.ReserveInventory(ctx, ReserveInventoryParams{
			MerchantID:       reservation.MerchantID,
			DishID:           dishID,
			Date:             reservation.ReservationDate,
			ReservedQuantity: delta,
		}); err != nil {
			if !errors.Is(err, ErrRecordNotFound) {
				return fmt.Errorf("reserve inventory for reservation adjustment: %w", err)
			}
			if _, getErr := q.GetDailyInventory(ctx, GetDailyInventoryParams{
				MerchantID: reservation.MerchantID,
				DishID:     dishID,
				Date:       reservation.ReservationDate,
			}); getErr == nil {
				return &requestError{statusCode: http.StatusConflict, err: errors.New("reservation adjustment inventory is insufficient")}
			} else if !errors.Is(getErr, ErrRecordNotFound) {
				return fmt.Errorf("get daily inventory: %w", getErr)
			}
		}
		hold, err := q.CreateReservationAdjustmentInventoryHold(ctx, CreateReservationAdjustmentInventoryHoldParams{
			AdjustmentID:    adjustmentID,
			MerchantID:      reservation.MerchantID,
			DishID:          dishID,
			ReservationDate: reservation.ReservationDate,
			Quantity:        delta,
			ExpiresAt:       expiresAt,
		})
		if err != nil {
			return fmt.Errorf("create reservation adjustment inventory hold: %w", err)
		}
		result.Holds = append(result.Holds, hold)
	}
	return nil
}

func releaseReservationAdjustmentHolds(ctx context.Context, q *Queries, adjustmentID int64) error {
	holds, err := q.ListReservationAdjustmentInventoryHoldsForUpdate(ctx, adjustmentID)
	if err != nil {
		return fmt.Errorf("list reservation adjustment inventory holds: %w", err)
	}
	for _, hold := range holds {
		if hold.Status != ReservationAdjustmentHoldStatusHeld {
			continue
		}
		if err := releaseReservedInventoryIfTracked(ctx, q, ReleaseReservedInventoryParams{
			MerchantID:       hold.MerchantID,
			DishID:           hold.DishID,
			Date:             hold.ReservationDate,
			ReservedQuantity: hold.Quantity,
		}); err != nil {
			return fmt.Errorf("release reservation adjustment inventory hold: %w", err)
		}
		if _, err := q.MarkReservationAdjustmentHoldReleased(ctx, hold.ID); err != nil {
			return fmt.Errorf("mark reservation adjustment hold released: %w", err)
		}
	}
	return nil
}

func textIfNotEmpty(value string) pgtype.Text {
	if value == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: value, Valid: true}
}
