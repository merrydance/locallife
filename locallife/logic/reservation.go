package logic

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/util"

	"github.com/rs/zerolog/log"
)

const (
	reservationStatusPending   = "pending"
	reservationStatusPaid      = "paid"
	reservationStatusConfirmed = "confirmed"
	reservationStatusCheckedIn = "checked_in"
	reservationStatusCompleted = "completed"
	reservationStatusCancelled = "cancelled"
	reservationStatusExpired   = "expired"
	reservationStatusNoShow    = "no_show"
)

const (
	paymentModeDeposit = "deposit"
	paymentModeFull    = "full"
)

const (
	defaultPaymentTimeoutMinutes = 30
	defaultRefundDeadlineHours   = 2
	defaultDepositAmount         = 10000
)

const (
	reservationActionConfirm      = "confirm"
	reservationActionComplete     = "complete"
	reservationActionCancel       = "cancel"
	reservationActionNoShow       = "no_show"
	reservationActionCheckIn      = "check_in"
	reservationActionStartCooking = "start_cooking"
)

// ReservationItemInput describes a dish or combo in a reservation request.
type ReservationItemInput struct {
	DishID   *int64
	ComboID  *int64
	Quantity int16
}

// ValidatedReservationItem holds validated pricing for reservation items.
type ValidatedReservationItem struct {
	DishID    *int64
	ComboID   *int64
	Quantity  int16
	UnitPrice int64
}

// CreateReservationInput contains required data to create a reservation.
type CreateReservationInput struct {
	UserID              int64
	TableID             int64
	ReservationDate     time.Time
	ReservationTime     time.Time
	GuestCount          int16
	ContactName         string
	ContactPhone        string
	PaymentMode         string
	Notes               string
	Items               []ReservationItemInput
	Now                 time.Time
	PaymentTimeoutMins  int
	RefundDeadlineHours int
	DefaultDeposit      int64
}

// CreateReservationResult returns the created reservation.
type CreateReservationResult struct {
	Reservation db.TableReservation
}

// MerchantCreateReservationInput contains required data to create a merchant reservation.
type MerchantCreateReservationInput struct {
	OperatorUserID  int64
	MerchantID      int64
	TableID         int64
	ReservationDate time.Time
	ReservationTime time.Time
	GuestCount      int16
	ContactName     string
	ContactPhone    string
	Source          string
	Notes           string
	Now             time.Time
}

// ReservationStatusUpdateResult returns the updated reservation and prior status.
type ReservationStatusUpdateResult struct {
	Reservation    db.TableReservation
	PreviousStatus string
	RefundEligible bool
}

// ReservationRefundPolicy defines reservation cancel refund percentages.
type ReservationRefundPolicy struct {
	UserBeforeDeadlinePercent     int
	UserAfterDeadlinePercent      int
	MerchantBeforeDeadlinePercent int
	MerchantAfterDeadlinePercent  int
}

func normalizeRefundPercent(value int) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func (policy ReservationRefundPolicy) normalize() ReservationRefundPolicy {
	return ReservationRefundPolicy{
		UserBeforeDeadlinePercent:     normalizeRefundPercent(policy.UserBeforeDeadlinePercent),
		UserAfterDeadlinePercent:      normalizeRefundPercent(policy.UserAfterDeadlinePercent),
		MerchantBeforeDeadlinePercent: normalizeRefundPercent(policy.MerchantBeforeDeadlinePercent),
		MerchantAfterDeadlinePercent:  normalizeRefundPercent(policy.MerchantAfterDeadlinePercent),
	}
}

func (policy ReservationRefundPolicy) refundPercent(isMerchant bool, beforeDeadline bool) int {
	if isMerchant {
		if beforeDeadline {
			return policy.MerchantBeforeDeadlinePercent
		}
		return policy.MerchantAfterDeadlinePercent
	}
	if beforeDeadline {
		return policy.UserBeforeDeadlinePercent
	}
	return policy.UserAfterDeadlinePercent
}

// CreateReservation creates a customer reservation with validations and conflict checks.
func CreateReservation(ctx context.Context, store db.Store, input CreateReservationInput) (CreateReservationResult, error) {
	var result CreateReservationResult

	if input.PaymentTimeoutMins <= 0 {
		input.PaymentTimeoutMins = defaultPaymentTimeoutMinutes
	}
	if input.RefundDeadlineHours <= 0 {
		input.RefundDeadlineHours = defaultRefundDeadlineHours
	}
	if input.DefaultDeposit <= 0 {
		input.DefaultDeposit = defaultDepositAmount
	}

	if input.PaymentMode != paymentModeDeposit && input.PaymentMode != paymentModeFull {
		return result, NewRequestError(http.StatusBadRequest, errors.New("invalid payment mode"))
	}

	reservationDateTime := time.Date(
		input.ReservationDate.Year(), input.ReservationDate.Month(), input.ReservationDate.Day(),
		input.ReservationTime.Hour(), input.ReservationTime.Minute(), 0, 0, input.ReservationDate.Location(),
	)
	if reservationDateTime.Before(input.Now) {
		return result, NewRequestError(http.StatusBadRequest, errors.New("所选时段已过，请选择其他时间"))
	}

	table, err := store.GetTable(ctx, input.TableID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("table not found"))
		}
		return result, err
	}

	if table.TableType != db.TableTypeRoom {
		return result, NewRequestError(http.StatusBadRequest, errors.New("只有包间可以预订"))
	}
	if table.Status == db.TableStatusDisabled {
		return result, NewRequestError(http.StatusConflict, errors.New("table is disabled and cannot be reserved"))
	}
	if input.GuestCount > table.Capacity {
		return result, NewRequestError(http.StatusBadRequest, errors.New("guest count exceeds table capacity"))
	}

	conflict, err := CheckReservationConflict(ctx, store, input.TableID, table.MerchantID, input.ReservationDate, input.ReservationTime, 0)
	if err != nil {
		return result, err
	}
	if conflict {
		return result, NewRequestError(http.StatusConflict, errors.New("该时间段已被预订，请选择其他时间"))
	}

	pgDate := pgtype.Date{Time: input.ReservationDate, Valid: true}
	pgTime := pgtype.Time{
		Microseconds: int64(input.ReservationTime.Hour()*3600+input.ReservationTime.Minute()*60) * 1000000,
		Valid:        true,
	}

	var depositAmount int64
	var prepaidAmount int64
	var validatedItems []ValidatedReservationItem

	switch input.PaymentMode {
	case paymentModeDeposit:
		if table.MinimumSpend.Valid && table.MinimumSpend.Int64 > 0 {
			depositAmount = table.MinimumSpend.Int64
		} else {
			depositAmount = input.DefaultDeposit
		}
	case paymentModeFull:
		if len(input.Items) > 0 {
			validatedItems, prepaidAmount, err = ValidateReservationItems(ctx, store, table.MerchantID, input.Items)
			if err != nil {
				return result, err
			}
			if table.MinimumSpend.Valid && prepaidAmount < table.MinimumSpend.Int64 {
				return result, NewRequestError(http.StatusBadRequest, fmt.Errorf("预点菜品金额 %d 分未达到包间最低消费 %d 分", prepaidAmount, table.MinimumSpend.Int64))
			}
		}
	}

	paymentDeadline := input.Now.Add(time.Duration(input.PaymentTimeoutMins) * time.Minute)
	refundDeadline := reservationDateTime.Add(-time.Duration(input.RefundDeadlineHours) * time.Hour)

	arg := db.CreateTableReservationParams{
		TableID:         input.TableID,
		UserID:          input.UserID,
		MerchantID:      table.MerchantID,
		ReservationDate: pgDate,
		ReservationTime: pgTime,
		GuestCount:      input.GuestCount,
		ContactName:     input.ContactName,
		ContactPhone:    input.ContactPhone,
		PaymentMode:     input.PaymentMode,
		DepositAmount:   depositAmount,
		PrepaidAmount:   prepaidAmount,
		RefundDeadline:  refundDeadline,
		PaymentDeadline: paymentDeadline,
		Status:          reservationStatusPending,
	}
	if input.Notes != "" {
		arg.Notes = pgtype.Text{String: input.Notes, Valid: true}
	}

	txArg := db.CreateReservationTxParams{
		CreateTableReservationParams: arg,
		DefaultDepositAmount:         input.DefaultDeposit,
		AfterLock: func(ctx context.Context, q *db.Queries) error {
			existingReservations, err := q.ListReservationsByTableAndDate(ctx, db.ListReservationsByTableAndDateParams{
				TableID:         input.TableID,
				ReservationDate: pgDate,
			})
			if err != nil {
				return fmt.Errorf("failed to list existing reservations: %w", err)
			}

			config := buildTimeSlotConfigFromQueries(ctx, q, table.MerchantID, input.ReservationDate)
			newDateTime := time.Date(input.ReservationDate.Year(), input.ReservationDate.Month(), input.ReservationDate.Day(), input.ReservationTime.Hour(), input.ReservationTime.Minute(), 0, 0, input.ReservationDate.Location())

			for _, r := range existingReservations {
				if r.Status == reservationStatusCancelled || r.Status == reservationStatusExpired || r.Status == reservationStatusNoShow {
					continue
				}
				if !r.ReservationTime.Valid {
					continue
				}

				existingTime := util.CombineDateAndTime(r.ReservationDate.Time, r.ReservationTime.Microseconds)
				if util.AreReservationsConflictingWithConfig(newDateTime, existingTime, config) {
					return fmt.Errorf("该时间段刚刚被抢订，请选择其他时间")
				}
			}
			return nil
		},
	}

	if len(validatedItems) > 0 {
		txArg.Items = make([]db.ReservationItemInput, len(validatedItems))
		for i, item := range validatedItems {
			txArg.Items[i] = db.ReservationItemInput{
				DishID:    item.DishID,
				ComboID:   item.ComboID,
				Quantity:  item.Quantity,
				UnitPrice: item.UnitPrice,
			}
		}
	}

	created, err := store.CreateReservationTx(ctx, txArg)
	if err != nil {
		if isReservationConflictError(err) {
			return result, NewRequestError(http.StatusConflict, errors.New("该时间段刚刚被抢订，请选择其他时间"))
		}
		if reqErr := mapReservationTableMutationError(err); reqErr != nil {
			return result, reqErr
		}
		return result, err
	}

	result.Reservation = created.Reservation
	return result, nil
}

// MerchantCreateReservation creates a reservation on behalf of a merchant.
func MerchantCreateReservation(ctx context.Context, store db.Store, input MerchantCreateReservationInput) (db.TableReservation, error) {
	reservationDateTime := time.Date(
		input.ReservationDate.Year(), input.ReservationDate.Month(), input.ReservationDate.Day(),
		input.ReservationTime.Hour(), input.ReservationTime.Minute(), 0, 0, input.ReservationDate.Location(),
	)
	if reservationDateTime.IsZero() {
		return db.TableReservation{}, NewRequestError(http.StatusBadRequest, errors.New("invalid reservation time"))
	}

	table, err := store.GetTable(ctx, input.TableID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.TableReservation{}, NewRequestError(http.StatusNotFound, errors.New("table not found"))
		}
		return db.TableReservation{}, err
	}
	if table.MerchantID != input.MerchantID {
		return db.TableReservation{}, NewRequestError(http.StatusForbidden, errors.New("table does not belong to your merchant"))
	}
	if table.Status == db.TableStatusDisabled {
		return db.TableReservation{}, NewRequestError(http.StatusConflict, errors.New("table is disabled and cannot be reserved"))
	}
	if input.GuestCount > table.Capacity {
		return db.TableReservation{}, NewRequestError(http.StatusBadRequest, errors.New("guest count exceeds table capacity"))
	}

	conflict, err := CheckReservationConflict(ctx, store, input.TableID, input.MerchantID, input.ReservationDate, input.ReservationTime, 0)
	if err != nil {
		return db.TableReservation{}, err
	}
	if conflict {
		return db.TableReservation{}, NewRequestError(http.StatusConflict, errors.New("该时间段已被预订，请选择其他时间"))
	}

	pgDate := pgtype.Date{Time: input.ReservationDate, Valid: true}
	pgTime := pgtype.Time{
		Microseconds: int64(input.ReservationTime.Hour()*3600+input.ReservationTime.Minute()*60) * 1000000,
		Valid:        true,
	}

	source := strings.TrimSpace(input.Source)
	if source == "" {
		source = "merchant"
	}

	txArg := db.CreateMerchantReservationTxParams{
		CreateTableReservationByMerchantParams: db.CreateTableReservationByMerchantParams{
			TableID:         input.TableID,
			UserID:          input.OperatorUserID,
			MerchantID:      input.MerchantID,
			ReservationDate: pgDate,
			ReservationTime: pgTime,
			GuestCount:      input.GuestCount,
			ContactName:     input.ContactName,
			ContactPhone:    input.ContactPhone,
			PaymentMode:     paymentModeDeposit,
			DepositAmount:   0,
			PrepaidAmount:   0,
			RefundDeadline:  input.Now,
			PaymentDeadline: input.Now.Add(365 * 24 * time.Hour),
			Notes:           pgtype.Text{String: input.Notes, Valid: input.Notes != ""},
			Source:          pgtype.Text{String: source, Valid: true},
		},
		AfterLock: func(ctx context.Context, q *db.Queries) error {
			existingReservations, err := q.ListReservationsByTableAndDate(ctx, db.ListReservationsByTableAndDateParams{
				TableID:         input.TableID,
				ReservationDate: pgDate,
			})
			if err != nil {
				return fmt.Errorf("failed to list existing reservations: %w", err)
			}

			config := buildTimeSlotConfigFromQueries(ctx, q, input.MerchantID, input.ReservationDate)
			newDateTime := time.Date(input.ReservationDate.Year(), input.ReservationDate.Month(), input.ReservationDate.Day(), input.ReservationTime.Hour(), input.ReservationTime.Minute(), 0, 0, input.ReservationDate.Location())

			for _, r := range existingReservations {
				if r.Status == reservationStatusCancelled || r.Status == reservationStatusExpired || r.Status == reservationStatusNoShow {
					continue
				}
				if !r.ReservationTime.Valid {
					continue
				}

				existingTime := util.CombineDateAndTime(r.ReservationDate.Time, r.ReservationTime.Microseconds)
				if util.AreReservationsConflictingWithConfig(newDateTime, existingTime, config) {
					return fmt.Errorf("该时间段刚刚被抢订，请选择其他时间")
				}
			}
			return nil
		},
	}

	reservation, err := store.CreateMerchantReservationTx(ctx, txArg)
	if err != nil {
		if isReservationConflictError(err) {
			return db.TableReservation{}, NewRequestError(http.StatusConflict, errors.New("该时间段刚刚被抢订，请选择其他时间"))
		}
		if reqErr := mapReservationTableMutationError(err); reqErr != nil {
			return db.TableReservation{}, reqErr
		}
		return db.TableReservation{}, err
	}

	return reservation, nil
}

// ConfirmReservation confirms a paid reservation as merchant.
func ConfirmReservation(ctx context.Context, store db.Store, userID, reservationID int64) (ReservationStatusUpdateResult, error) {
	merchant, err := resolveMerchantForUser(ctx, store, userID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return ReservationStatusUpdateResult{}, NewRequestError(http.StatusForbidden, errors.New("not a merchant"))
		}
		return ReservationStatusUpdateResult{}, err
	}

	reservation, err := store.GetTableReservationForUpdate(ctx, reservationID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return ReservationStatusUpdateResult{}, NewRequestError(http.StatusNotFound, errors.New("reservation not found"))
		}
		return ReservationStatusUpdateResult{}, err
	}

	if reservation.MerchantID != merchant.ID {
		return ReservationStatusUpdateResult{}, NewRequestError(http.StatusForbidden, errors.New("reservation does not belong to your merchant"))
	}
	suspension, err := GetTakeoutSuspension(ctx, store, merchant.ID)
	if err != nil {
		return ReservationStatusUpdateResult{}, err
	}
	if suspension != nil {
		return ReservationStatusUpdateResult{}, NewRequestError(http.StatusForbidden, errors.New("商户预订接单已暂停"))
	}
	if !isReservationStatusAllowed(reservation.Status, reservationActionConfirm) {
		return ReservationStatusUpdateResult{}, NewRequestError(http.StatusConflict, errors.New("reservation is not paid"))
	}

	result, err := store.ConfirmReservationTx(ctx, db.ConfirmReservationTxParams{
		ReservationID: reservationID,
		TableID:       reservation.TableID,
	})
	if err != nil {
		return ReservationStatusUpdateResult{}, err
	}

	return ReservationStatusUpdateResult{Reservation: result.Reservation, PreviousStatus: reservation.Status}, nil
}

// CompleteReservation marks a reservation as completed and releases the table.
func CompleteReservation(ctx context.Context, store db.Store, taskScheduler TaskScheduler, userID, reservationID int64) (ReservationStatusUpdateResult, error) {
	merchant, err := resolveMerchantForUser(ctx, store, userID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return ReservationStatusUpdateResult{}, NewRequestError(http.StatusForbidden, errors.New("not a merchant"))
		}
		return ReservationStatusUpdateResult{}, err
	}

	reservation, err := store.GetTableReservationForUpdate(ctx, reservationID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return ReservationStatusUpdateResult{}, NewRequestError(http.StatusNotFound, errors.New("reservation not found"))
		}
		return ReservationStatusUpdateResult{}, err
	}

	if reservation.MerchantID != merchant.ID {
		return ReservationStatusUpdateResult{}, NewRequestError(http.StatusForbidden, errors.New("reservation does not belong to your merchant"))
	}
	if !isReservationStatusAllowed(reservation.Status, reservationActionComplete) {
		return ReservationStatusUpdateResult{}, NewRequestError(http.StatusConflict, errors.New("reservation is not confirmed or checked in"))
	}
	if err := ensureNoActiveReservationAdjustment(ctx, store, reservation.ID); err != nil {
		return ReservationStatusUpdateResult{}, err
	}

	var currentReservationID pgtype.Int8
	if table, err := store.GetTable(ctx, reservation.TableID); err == nil {
		currentReservationID = table.CurrentReservationID
	}

	result, err := store.CompleteReservationTx(ctx, db.CompleteReservationTxParams{
		ReservationID:        reservationID,
		TableID:              reservation.TableID,
		CurrentReservationID: currentReservationID,
	})
	if err != nil {
		if statusCode, ok := db.IsRefundRequestError(err); ok {
			return ReservationStatusUpdateResult{}, NewRequestError(statusCode, errors.Unwrap(err))
		}
		return ReservationStatusUpdateResult{}, err
	}

	scheduleBaofuProfitSharingForCompletedReservation(ctx, store, taskScheduler, merchant, result.Reservation)

	return ReservationStatusUpdateResult{Reservation: result.Reservation, PreviousStatus: reservation.Status}, nil
}

// CancelReservation cancels a reservation and optionally prepares refund info.
func CancelReservation(
	ctx context.Context,
	store db.Store,
	userID,
	reservationID int64,
	reason string,
	refundPolicy ReservationRefundPolicy,
	now time.Time,
) (ReservationStatusUpdateResult, error) {
	reservation, err := store.GetTableReservationForUpdate(ctx, reservationID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return ReservationStatusUpdateResult{}, NewRequestError(http.StatusNotFound, errors.New("reservation not found"))
		}
		return ReservationStatusUpdateResult{}, err
	}

	isOwner := reservation.UserID == userID
	isMerchant := false
	if !isOwner {
		merchant, err := resolveMerchantForUser(ctx, store, userID)
		if err != nil && !errors.Is(err, db.ErrRecordNotFound) {
			return ReservationStatusUpdateResult{}, err
		}
		if err == nil && merchant.ID == reservation.MerchantID {
			isMerchant = true
		}
	}

	if !isOwner && !isMerchant {
		return ReservationStatusUpdateResult{}, NewRequestError(http.StatusForbidden, errors.New("not authorized to cancel this reservation"))
	}
	if !isReservationStatusAllowed(reservation.Status, reservationActionCancel) {
		return ReservationStatusUpdateResult{}, NewRequestError(http.StatusConflict, errors.New("预约状态不允许取消"))
	}
	if err := ensureNoActiveReservationAdjustment(ctx, store, reservation.ID); err != nil {
		return ReservationStatusUpdateResult{}, err
	}

	refundPolicy = refundPolicy.normalize()
	refundEligible := false
	refundPercent := 0
	if reservation.Status == reservationStatusPaid || reservation.Status == reservationStatusConfirmed {
		refundEligible = now.Before(reservation.RefundDeadline)
		refundPercent = refundPolicy.refundPercent(isMerchant, refundEligible)
		if !refundEligible && refundPercent <= 0 {
			return ReservationStatusUpdateResult{}, NewRequestError(http.StatusConflict, errors.New("退款截止时间已过"))
		}
	}

	var refundPaymentOrder *db.PaymentOrder
	refundAmount := int64(0)
	if (reservation.Status == reservationStatusPaid || reservation.Status == reservationStatusConfirmed) && refundPercent > 0 {
		paymentOrder, err := store.GetLatestPaymentOrderByReservation(ctx, db.GetLatestPaymentOrderByReservationParams{
			ReservationID: pgtype.Int8{Int64: reservation.ID, Valid: true},
			BusinessType:  businessTypeReservation,
		})
		if err != nil {
			if !errors.Is(err, db.ErrRecordNotFound) {
				return ReservationStatusUpdateResult{}, fmt.Errorf("get latest reservation payment order for refund: %w", err)
			}
		} else if paymentOrder.Status == paymentStatusPaid {
			refundAmount = paymentOrder.Amount * int64(refundPercent) / 100
			if refundAmount <= 0 {
				refundEligible = false
			} else {
				if paymentOrder.PaymentChannel != db.PaymentChannelBaofuAggregate {
					return ReservationStatusUpdateResult{}, NewRequestError(
						http.StatusConflict,
						fmt.Errorf("预定支付单支付通道 %s 已废弃，无法发起自动退款，请联系平台处理", paymentOrder.PaymentChannel),
					)
				}
				refundPaymentOrder = &paymentOrder
			}
		}
	}

	var currentReservationID pgtype.Int8
	if table, err := store.GetTable(ctx, reservation.TableID); err == nil {
		currentReservationID = table.CurrentReservationID
	}

	result, err := store.CancelReservationTx(ctx, db.CancelReservationTxParams{
		ReservationID:        reservationID,
		TableID:              reservation.TableID,
		CancelReason:         reason,
		CurrentReservationID: currentReservationID,
		ReleaseInventory:     true,
	})
	if err != nil {
		if statusCode, ok := db.IsRefundRequestError(err); ok {
			return ReservationStatusUpdateResult{}, NewRequestError(statusCode, errors.Unwrap(err))
		}
		return ReservationStatusUpdateResult{}, err
	}

	if refundPaymentOrder != nil && refundAmount > 0 {
		paymentOrder := *refundPaymentOrder
		outRefundNo, err := generateOutRefundNo()
		if err != nil {
			return ReservationStatusUpdateResult{}, fmt.Errorf("generate out refund no: %w", err)
		}

		refundType := refundTypeForPaymentOrder(paymentOrder)

		txResult, createErr := store.CreateRefundOrderTx(ctx, db.CreateRefundOrderTxParams{
			PaymentOrderID: paymentOrder.ID,
			RefundType:     refundType,
			RefundAmount:   refundAmount,
			RefundReason:   "预定取消退款",
			OutRefundNo:    outRefundNo,
		})
		if createErr != nil {
			if _, ok := db.IsRefundRequestError(createErr); ok {
				log.Warn().Err(createErr).Int64("payment_order_id", paymentOrder.ID).Msg("reservation refund validation failed")
			} else {
				log.Error().Err(createErr).Int64("payment_order_id", paymentOrder.ID).Msg("create reservation refund order tx failed")
			}
		} else if txResult.RefundOrder.ID > 0 {
			log.Info().
				Int64("refund_order_id", txResult.RefundOrder.ID).
				Int64("payment_order_id", paymentOrder.ID).
				Str("out_refund_no", outRefundNo).
				Msg("reservation cancel refund order created; async recovery will submit baofu refund")
		}
	}

	return ReservationStatusUpdateResult{
		Reservation:    result.Reservation,
		PreviousStatus: reservation.Status,
		RefundEligible: refundPercent > 0,
	}, nil
}

// MarkReservationNoShow marks a reservation as no-show and releases inventory.
func MarkReservationNoShow(ctx context.Context, store db.Store, userID, reservationID int64) (ReservationStatusUpdateResult, error) {
	merchant, err := resolveMerchantForUser(ctx, store, userID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return ReservationStatusUpdateResult{}, NewRequestError(http.StatusForbidden, errors.New("not a merchant"))
		}
		return ReservationStatusUpdateResult{}, err
	}

	reservation, err := store.GetTableReservationForUpdate(ctx, reservationID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return ReservationStatusUpdateResult{}, NewRequestError(http.StatusNotFound, errors.New("reservation not found"))
		}
		return ReservationStatusUpdateResult{}, err
	}

	if reservation.MerchantID != merchant.ID {
		return ReservationStatusUpdateResult{}, NewRequestError(http.StatusForbidden, errors.New("reservation does not belong to your merchant"))
	}
	if !isReservationStatusAllowed(reservation.Status, reservationActionNoShow) {
		return ReservationStatusUpdateResult{}, NewRequestError(http.StatusConflict, errors.New("only paid or confirmed reservations can be marked as no-show"))
	}
	if err := ensureNoActiveReservationAdjustment(ctx, store, reservation.ID); err != nil {
		return ReservationStatusUpdateResult{}, err
	}

	var currentReservationID pgtype.Int8
	if table, err := store.GetTable(ctx, reservation.TableID); err == nil {
		currentReservationID = table.CurrentReservationID
	}

	result, err := store.MarkNoShowTx(ctx, db.MarkNoShowTxParams{
		ReservationID:        reservationID,
		TableID:              reservation.TableID,
		CurrentReservationID: currentReservationID,
	})
	if err != nil {
		if statusCode, ok := db.IsRefundRequestError(err); ok {
			return ReservationStatusUpdateResult{}, NewRequestError(statusCode, errors.Unwrap(err))
		}
		return ReservationStatusUpdateResult{}, err
	}

	if err := store.ReleaseReservationInventoryTx(ctx, db.ReleaseReservationInventoryTxParams{
		ReservationID: reservation.ID,
	}); err != nil {
		return ReservationStatusUpdateResult{}, err
	}

	return ReservationStatusUpdateResult{Reservation: result.Reservation, PreviousStatus: reservation.Status}, nil
}

// CheckInReservation marks a reservation as checked in.
func CheckInReservation(ctx context.Context, store db.Store, userID, reservationID int64, now time.Time, earlyMinutes, lateMinutes int) (ReservationStatusUpdateResult, error) {
	reservation, err := store.GetTableReservationForUpdate(ctx, reservationID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return ReservationStatusUpdateResult{}, NewRequestError(http.StatusNotFound, errors.New("reservation not found"))
		}
		return ReservationStatusUpdateResult{}, err
	}

	isOwner := reservation.UserID == userID
	isMerchant := false
	if !isOwner {
		merchant, err := resolveMerchantForUser(ctx, store, userID)
		if err != nil && !errors.Is(err, db.ErrRecordNotFound) {
			return ReservationStatusUpdateResult{}, err
		}
		if err == nil && merchant.ID == reservation.MerchantID {
			isMerchant = true
		}
	}

	if !isOwner && !isMerchant {
		return ReservationStatusUpdateResult{}, NewRequestError(http.StatusForbidden, errors.New("not authorized to check in this reservation"))
	}
	if !isReservationStatusAllowed(reservation.Status, reservationActionCheckIn) {
		return ReservationStatusUpdateResult{}, NewRequestError(http.StatusConflict, errors.New("only paid or confirmed reservations can be checked in"))
	}

	if reservation.ReservationDate.Valid && reservation.ReservationTime.Valid {
		hours := reservation.ReservationTime.Microseconds / 1000000 / 3600
		minutes := (reservation.ReservationTime.Microseconds / 1000000 % 3600) / 60
		reservationDateTime := time.Date(
			reservation.ReservationDate.Time.Year(),
			reservation.ReservationDate.Time.Month(),
			reservation.ReservationDate.Time.Day(),
			int(hours), int(minutes), 0, 0, time.Local,
		)

		earlyLimit := reservationDateTime.Add(-time.Duration(earlyMinutes) * time.Minute)
		lateLimit := reservationDateTime.Add(time.Duration(lateMinutes) * time.Minute)

		if now.Before(earlyLimit) {
			return ReservationStatusUpdateResult{}, NewRequestError(http.StatusBadRequest, fmt.Errorf(
				"尚未到签到时间，请在预订时间前%d分钟内签到",
				earlyMinutes,
			))
		}
		if now.After(lateLimit) {
			return ReservationStatusUpdateResult{}, NewRequestError(http.StatusBadRequest, fmt.Errorf(
				"已超过签到时间%d分钟，请联系商户",
				lateMinutes,
			))
		}
	}

	updated, err := store.UpdateReservationToCheckedIn(ctx, reservationID)
	if err != nil {
		return ReservationStatusUpdateResult{}, err
	}

	return ReservationStatusUpdateResult{Reservation: updated, PreviousStatus: reservation.Status}, nil
}

// StartCookingReservation marks reservation as cooking started.
func StartCookingReservation(ctx context.Context, store db.Store, userID, reservationID int64) (ReservationStatusUpdateResult, error) {
	reservation, err := store.GetTableReservationForUpdate(ctx, reservationID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return ReservationStatusUpdateResult{}, NewRequestError(http.StatusNotFound, errors.New("reservation not found"))
		}
		return ReservationStatusUpdateResult{}, err
	}

	isOwner := reservation.UserID == userID
	isMerchant := false
	if !isOwner {
		merchant, err := resolveMerchantForUser(ctx, store, userID)
		if err != nil && !errors.Is(err, db.ErrRecordNotFound) {
			return ReservationStatusUpdateResult{}, err
		}
		if err == nil && merchant.ID == reservation.MerchantID {
			isMerchant = true
		}
	}

	if !isOwner && !isMerchant {
		return ReservationStatusUpdateResult{}, NewRequestError(http.StatusForbidden, errors.New("not authorized to start cooking for this reservation"))
	}
	if !isReservationStatusAllowed(reservation.Status, reservationActionStartCooking) {
		return ReservationStatusUpdateResult{}, NewRequestError(http.StatusConflict, errors.New("only confirmed or checked-in reservations can start cooking"))
	}
	if err := ensureNoActiveReservationAdjustment(ctx, store, reservation.ID); err != nil {
		return ReservationStatusUpdateResult{}, err
	}

	updated, err := store.UpdateReservationCookingStarted(ctx, reservationID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return ReservationStatusUpdateResult{}, NewRequestError(http.StatusConflict, errors.New("预订存在待支付改菜补差单，请先完成或关闭支付单"))
		}
		return ReservationStatusUpdateResult{}, err
	}

	return ReservationStatusUpdateResult{Reservation: updated, PreviousStatus: reservation.Status}, nil
}

// ValidateReservationItems validates reservation items and returns pricing totals.
func ValidateReservationItems(ctx context.Context, store db.Store, merchantID int64, items []ReservationItemInput) ([]ValidatedReservationItem, int64, error) {
	var total int64
	validated := make([]ValidatedReservationItem, 0, len(items))

	for _, item := range items {
		if item.DishID == nil && item.ComboID == nil {
			return nil, 0, NewRequestError(http.StatusBadRequest, errors.New("每个菜品项必须指定 dish_id 或 combo_id"))
		}
		if item.DishID != nil && item.ComboID != nil {
			return nil, 0, NewRequestError(http.StatusBadRequest, errors.New("每个菜品项只能指定 dish_id 或 combo_id 之一"))
		}

		var unitPrice int64
		if item.DishID != nil {
			dish, err := store.GetDish(ctx, *item.DishID)
			if err != nil {
				if errors.Is(err, db.ErrRecordNotFound) {
					return nil, 0, NewRequestError(http.StatusBadRequest, fmt.Errorf("菜品 %d 不存在", *item.DishID))
				}
				return nil, 0, err
			}
			if dish.MerchantID != merchantID {
				return nil, 0, NewRequestError(http.StatusBadRequest, fmt.Errorf("菜品 %s 不属于该商户", dish.Name))
			}
			if !dish.IsOnline {
				return nil, 0, NewRequestError(http.StatusBadRequest, fmt.Errorf("菜品 %s 已下架", dish.Name))
			}
			if !dish.IsAvailable {
				return nil, 0, NewRequestError(http.StatusBadRequest, fmt.Errorf("菜品 %s 暂不可售", dish.Name))
			}
			unitPrice = dish.Price
		}

		if item.ComboID != nil {
			combo, err := store.GetComboSet(ctx, *item.ComboID)
			if err != nil {
				if errors.Is(err, db.ErrRecordNotFound) {
					return nil, 0, NewRequestError(http.StatusBadRequest, fmt.Errorf("套餐 %d 不存在", *item.ComboID))
				}
				return nil, 0, err
			}
			if combo.MerchantID != merchantID {
				return nil, 0, NewRequestError(http.StatusBadRequest, fmt.Errorf("套餐 %s 不属于该商户", combo.Name))
			}
			if !combo.IsOnline {
				return nil, 0, NewRequestError(http.StatusBadRequest, fmt.Errorf("套餐 %s 已下架", combo.Name))
			}
			if err := validateComboChildDishesOrderable(ctx, store, combo.ID, combo.Name); err != nil {
				return nil, 0, err
			}
			unitPrice = combo.ComboPrice
		}

		validated = append(validated, ValidatedReservationItem{
			DishID:    item.DishID,
			ComboID:   item.ComboID,
			Quantity:  item.Quantity,
			UnitPrice: unitPrice,
		})
		total += unitPrice * int64(item.Quantity)
	}

	return validated, total, nil
}

// CheckReservationConflict checks for reservation time conflicts on a table.
func CheckReservationConflict(ctx context.Context, store db.Store, tableID int64, merchantID int64, date time.Time, newTime time.Time, excludeID int64) (bool, error) {
	pgDate := pgtype.Date{Time: date, Valid: true}
	reservations, err := store.ListReservationsByTableAndDate(ctx, db.ListReservationsByTableAndDateParams{
		TableID:         tableID,
		ReservationDate: pgDate,
	})
	if err != nil {
		return false, err
	}

	config := buildTimeSlotConfig(ctx, store, merchantID, date)

	for _, r := range reservations {
		if r.ID == excludeID {
			continue
		}
		if r.Status == reservationStatusCancelled || r.Status == reservationStatusExpired || r.Status == reservationStatusNoShow {
			continue
		}
		if !r.ReservationTime.Valid {
			continue
		}

		existingTime := util.CombineDateAndTime(r.ReservationDate.Time, r.ReservationTime.Microseconds)
		newDateTime := time.Date(date.Year(), date.Month(), date.Day(), newTime.Hour(), newTime.Minute(), 0, 0, date.Location())
		if util.AreReservationsConflictingWithConfig(newDateTime, existingTime, config) {
			return true, nil
		}
	}

	return false, nil
}

func buildTimeSlotConfig(ctx context.Context, store db.Store, merchantID int64, date time.Time) util.TimeSlotConfig {
	config := util.DefaultConfig
	businessHours, err := store.ListMerchantBusinessHours(ctx, merchantID)
	if err != nil {
		return config
	}

	return buildTimeSlotConfigFromBusinessHours(businessHours, date, config)
}

func buildTimeSlotConfigFromQueries(ctx context.Context, q *db.Queries, merchantID int64, date time.Time) util.TimeSlotConfig {
	config := util.DefaultConfig
	businessHours, err := q.ListMerchantBusinessHours(ctx, merchantID)
	if err != nil {
		return config
	}

	return buildTimeSlotConfigFromBusinessHours(businessHours, date, config)
}

func buildTimeSlotConfigFromBusinessHours(businessHours []db.MerchantBusinessHour, date time.Time, config util.TimeSlotConfig) util.TimeSlotConfig {
	dayOfWeek := int32(date.Weekday())
	var todayHours []db.MerchantBusinessHour
	for _, bh := range businessHours {
		if bh.SpecialDate.Valid && bh.SpecialDate.Time.Format("2006-01-02") == date.Format("2006-01-02") {
			todayHours = append(todayHours, bh)
		}
	}
	if len(todayHours) == 0 {
		for _, bh := range businessHours {
			if !bh.SpecialDate.Valid && bh.DayOfWeek == dayOfWeek {
				todayHours = append(todayHours, bh)
			}
		}
	}

	if len(todayHours) > 0 {
		h1 := todayHours[0]
		config.LunchStart = int(h1.OpenTime.Microseconds/1000000/3600*100) + int(h1.OpenTime.Microseconds/1000000%3600/60)
		config.LunchEnd = int(h1.CloseTime.Microseconds/1000000/3600*100) + int(h1.CloseTime.Microseconds/1000000%3600/60)
		config.DinnerStart = 0
		config.DinnerEnd = 0

		if len(todayHours) > 1 {
			h2 := todayHours[1]
			config.DinnerStart = int(h2.OpenTime.Microseconds/1000000/3600*100) + int(h2.OpenTime.Microseconds/1000000%3600/60)
			config.DinnerEnd = int(h2.CloseTime.Microseconds/1000000/3600*100) + int(h2.CloseTime.Microseconds/1000000%3600/60)
		} else if config.LunchStart >= 1500 {
			config.DinnerStart = config.LunchStart
			config.DinnerEnd = config.LunchEnd
			config.LunchStart = 0
			config.LunchEnd = 0
		}
	}

	return config
}

func isReservationStatusAllowed(status string, action string) bool {
	switch action {
	case reservationActionConfirm:
		return status == reservationStatusPaid
	case reservationActionComplete:
		return status == reservationStatusConfirmed || status == reservationStatusCheckedIn
	case reservationActionCancel:
		return status == reservationStatusPending || status == reservationStatusPaid || status == reservationStatusConfirmed
	case reservationActionNoShow:
		return status == reservationStatusPaid || status == reservationStatusConfirmed
	case reservationActionCheckIn:
		return status == reservationStatusPaid || status == reservationStatusConfirmed
	case reservationActionStartCooking:
		return status == reservationStatusConfirmed || status == reservationStatusCheckedIn
	default:
		return false
	}
}

func isReservationConflictError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "刚刚被抢订")
}

func mapReservationTableMutationError(err error) error {
	switch {
	case errors.Is(err, db.ErrRecordNotFound):
		return NewRequestError(http.StatusNotFound, errors.New("table not found"))
	case errors.Is(err, db.ErrTableDisabledForReservation):
		return NewRequestError(http.StatusConflict, errors.New("table is disabled and cannot be reserved"))
	case errors.Is(err, db.ErrTableMerchantMismatchForReservation):
		return NewRequestError(http.StatusForbidden, errors.New("table does not belong to your merchant"))
	case errors.Is(err, db.ErrTableTypeNotReservable):
		return NewRequestError(http.StatusBadRequest, errors.New("只有包间可以预订"))
	case errors.Is(err, db.ErrReservationGuestCountExceedsCapacity):
		return NewRequestError(http.StatusBadRequest, errors.New("guest count exceeds table capacity"))
	case errors.Is(err, db.ErrReservationMinimumSpendNotMet):
		return NewRequestError(http.StatusBadRequest, errors.New("预点菜品金额未达到包间最低消费，请刷新后重试"))
	default:
		return nil
	}
}
