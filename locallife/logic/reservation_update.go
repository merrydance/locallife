package logic

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

// MerchantUpdateReservationInput describes merchant updates to a reservation.
type MerchantUpdateReservationInput struct {
	OperatorUserID  int64
	ReservationID   int64
	TableID         *int64
	ReservationDate *time.Time
	ReservationTime *time.Time
	GuestCount      *int16
	ContactName     *string
	ContactPhone    *string
	Notes           *string
}

// MerchantUpdateReservation updates reservation details with conflict checks.
func MerchantUpdateReservation(ctx context.Context, store db.Store, input MerchantUpdateReservationInput) (db.TableReservation, error) {
	merchant, err := resolveMerchantForUser(ctx, store, input.OperatorUserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.TableReservation{}, NewRequestError(http.StatusForbidden, errors.New("not a merchant"))
		}
		return db.TableReservation{}, err
	}

	updateParams := db.UpdateReservationParams{ID: input.ReservationID}

	if input.TableID != nil {
		updateParams.TableID = pgtype.Int8{Int64: *input.TableID, Valid: true}
	}
	if input.ReservationDate != nil {
		updateParams.ReservationDate = pgtype.Date{Time: *input.ReservationDate, Valid: true}
	}
	if input.ReservationTime != nil {
		updateParams.ReservationTime = pgtype.Time{
			Microseconds: int64(input.ReservationTime.Hour()*3600+input.ReservationTime.Minute()*60) * 1000000,
			Valid:        true,
		}
	}
	if input.GuestCount != nil {
		updateParams.GuestCount = pgtype.Int2{Int16: *input.GuestCount, Valid: true}
	}
	if input.ContactName != nil {
		updateParams.ContactName = pgtype.Text{String: *input.ContactName, Valid: true}
	}
	if input.ContactPhone != nil {
		updateParams.ContactPhone = pgtype.Text{String: *input.ContactPhone, Valid: true}
	}
	if input.Notes != nil {
		updateParams.Notes = pgtype.Text{String: *input.Notes, Valid: true}
	}

	updatedReservation, err := store.UpdateReservationTx(ctx, db.UpdateReservationTxParams{
		MerchantID:  merchant.ID,
		Reservation: updateParams,
	})
	if err != nil {
		if reqErr := mapReservationUpdateMutationError(err); reqErr != nil {
			return db.TableReservation{}, reqErr
		}
		return db.TableReservation{}, err
	}

	return updatedReservation, nil
}

func mapReservationUpdateMutationError(err error) error {
	switch {
	case errors.Is(err, db.ErrRecordNotFound):
		return NewRequestError(http.StatusNotFound, errors.New("reservation not found"))
	case errors.Is(err, db.ErrTableNotFoundForReservation):
		return NewRequestError(http.StatusNotFound, errors.New("table not found"))
	case errors.Is(err, db.ErrReservationMerchantMismatch):
		return NewRequestError(http.StatusForbidden, errors.New("reservation does not belong to your merchant"))
	case errors.Is(err, db.ErrReservationTerminalState):
		return NewRequestError(http.StatusConflict, errors.New("cannot modify completed, cancelled or expired reservations"))
	case errors.Is(err, db.ErrReservationTimeConflict):
		return NewRequestError(http.StatusConflict, errors.New("time slot is already reserved"))
	default:
		return mapReservationTableMutationError(err)
	}
}
