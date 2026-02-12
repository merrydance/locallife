package logic

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

// EnsureReservationSingleActiveOrder blocks duplicate deposit-mode orders.
func EnsureReservationSingleActiveOrder(ctx context.Context, store db.Store, reservationID int64) error {
	existing, err := store.GetLatestOrderByReservation(ctx, pgtype.Int8{Int64: reservationID, Valid: true})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if existing.Status != "cancelled" && !existing.ReplacedByOrderID.Valid {
		return NewRequestError(http.StatusConflict, errors.New("reservation already has an active order"))
	}
	return nil
}

// BindDiningSessionActiveOrder updates the active order on a dining session.
func BindDiningSessionActiveOrder(ctx context.Context, store db.Store, sessionID, orderID int64) error {
	_, err := store.UpdateDiningSessionActiveOrder(ctx, db.UpdateDiningSessionActiveOrderParams{
		ID:            sessionID,
		ActiveOrderID: pgtype.Int8{Int64: orderID, Valid: true},
	})
	return err
}

// ClearDiningOrderCart removes a dine-in or reservation cart after order placement.
type ClearDiningOrderCartInput struct {
	UserID        int64
	MerchantID    int64
	OrderType     string
	TableID       *int64
	ReservationID *int64
}

// ClearDiningOrderCart removes cart rows for dine-in/reservation flows.
func ClearDiningOrderCart(ctx context.Context, store db.Store, input ClearDiningOrderCartInput) error {
	tableID := pgtype.Int8{}
	if input.TableID != nil {
		tableID = pgtype.Int8{Int64: *input.TableID, Valid: true}
	}
	reservationID := pgtype.Int8{}
	if input.ReservationID != nil {
		reservationID = pgtype.Int8{Int64: *input.ReservationID, Valid: true}
	}

	cart, err := store.GetCartByUserAndMerchant(ctx, db.GetCartByUserAndMerchantParams{
		UserID:        input.UserID,
		MerchantID:    input.MerchantID,
		OrderType:     input.OrderType,
		TableID:       tableID,
		ReservationID: reservationID,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	return store.ClearCart(ctx, cart.ID)
}
