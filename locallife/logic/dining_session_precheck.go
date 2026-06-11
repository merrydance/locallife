package logic

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

// DiningSessionPrecheckInput holds parameters for prechecking a table.
type DiningSessionPrecheckInput struct {
	UserID  int64
	TableID int64
	Now     time.Time
}

// DiningSessionPrecheckResult returns reservation and order context for a table.
type DiningSessionPrecheckResult struct {
	Table              db.Table
	Reservation        *db.TableReservation
	Order              *db.Order
	Reserved           bool
	IsReservationOwner bool
	PaymentMode        *string
	PaidAmount         *int64
}

// PrecheckDiningSession checks whether a table is reserved and exposes reservation/order info.
func PrecheckDiningSession(ctx context.Context, store db.Store, input DiningSessionPrecheckInput) (DiningSessionPrecheckResult, error) {
	var result DiningSessionPrecheckResult

	table, err := store.GetTable(ctx, input.TableID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("table not found"))
		}
		return result, err
	}
	result.Table = table

	activeReservation, err := FindActiveReservationForTable(ctx, store, table.ID, input.Now)
	if err != nil {
		return result, err
	}
	if activeReservation == nil {
		return result, nil
	}

	isMerchant := false
	if m, err := store.GetMerchant(ctx, table.MerchantID); err == nil && m.OwnerUserID == input.UserID {
		isMerchant = true
	} else if err == nil {
		if hasAccess, err := store.CheckUserHasMerchantAccess(ctx, db.CheckUserHasMerchantAccessParams{
			MerchantID: table.MerchantID,
			UserID:     input.UserID,
		}); err == nil && hasAccess {
			isMerchant = true
		}
	}

	if !isCustomerOwnedReservation(*activeReservation, input.UserID) && !isMerchant {
		return result, NewRequestError(http.StatusConflict, errors.New("桌位已被预约，暂时不可用"))
	}

	result.Reserved = true
	result.Reservation = activeReservation
	result.IsReservationOwner = isCustomerOwnedReservation(*activeReservation, input.UserID)

	paymentMode := activeReservation.PaymentMode
	paidAmount := activeReservation.PrepaidAmount
	if paymentMode == paymentModeDeposit {
		paidAmount = activeReservation.DepositAmount
	}
	result.PaymentMode = &paymentMode
	result.PaidAmount = &paidAmount

	order, err := store.GetLatestOrderByReservation(ctx, pgtype.Int8{Int64: activeReservation.ID, Valid: true})
	if err == nil {
		result.Order = &order
	} else if !errors.Is(err, db.ErrRecordNotFound) {
		return result, err
	}

	return result, nil
}
