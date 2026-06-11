package logic

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

// OrderSessionInput defines the order context for session validation.
type OrderSessionInput struct {
	UserID         int64
	MerchantID     int64
	OrderType      string
	TableID        *int64
	ReservationID  *int64
	BillingGroupID *int64
}

// OrderSessionResult contains validated session and reservation info.
type OrderSessionResult struct {
	Reservation    *db.TableReservation
	DiningSession  *db.DiningSession
	BillingGroupID *int64
	TableID        *int64
}

// ValidateOrderSessionAndBilling validates reservation/dining session and billing group rules.
func ValidateOrderSessionAndBilling(ctx context.Context, store db.Store, input OrderSessionInput) (OrderSessionResult, error) {
	var result OrderSessionResult

	switch input.OrderType {
	case "reservation":
		if input.ReservationID == nil {
			return result, NewRequestError(http.StatusBadRequest, errors.New("reservation_id is required"))
		}

		reservation, err := store.GetTableReservation(ctx, *input.ReservationID)
		if err != nil {
			if errors.Is(err, db.ErrRecordNotFound) {
				return result, NewRequestError(http.StatusNotFound, errors.New("reservation not found"))
			}
			return result, err
		}

		if !isCustomerOwnedReservation(reservation, input.UserID) {
			return result, NewRequestError(http.StatusForbidden, errors.New("reservation does not belong to you"))
		}
		if reservation.MerchantID != input.MerchantID {
			return result, NewRequestError(http.StatusBadRequest, errors.New("reservation does not belong to this merchant"))
		}
		if reservation.PaymentMode == paymentModeDeposit && reservation.Status == reservationStatusPending {
			return result, NewRequestError(http.StatusConflict, errors.New("reservation deposit is not paid"))
		}
		if reservation.Status != "pending" && reservation.Status != "paid" && reservation.Status != "confirmed" && reservation.Status != "checked_in" {
			return result, NewRequestError(http.StatusConflict, errors.New("reservation is in an invalid state"))
		}
		if input.TableID != nil && reservation.TableID != *input.TableID {
			return result, NewRequestError(http.StatusBadRequest, errors.New("table does not match reservation"))
		}

		result.Reservation = &reservation
		tableID := reservation.TableID
		result.TableID = &tableID

		session, err := store.GetActiveDiningSessionByReservation(ctx, pgtype.Int8{Int64: reservation.ID, Valid: true})
		if err == nil {
			if session.TableID != reservation.TableID {
				return result, NewRequestError(http.StatusConflict, errors.New("dining session table mismatch"))
			}
			if session.MerchantID != input.MerchantID {
				return result, NewRequestError(http.StatusBadRequest, errors.New("dining session merchant mismatch"))
			}
			result.DiningSession = &session
		} else if !errors.Is(err, db.ErrRecordNotFound) {
			return result, err
		}

	case "dine_in":
		if input.TableID == nil {
			return result, NewRequestError(http.StatusBadRequest, errors.New("table_id is required"))
		}

		session, err := store.GetActiveDiningSessionByTable(ctx, *input.TableID)
		if err != nil {
			if errors.Is(err, db.ErrRecordNotFound) {
				return result, NewRequestError(http.StatusConflict, errors.New("no active dining session"))
			}
			return result, err
		}
		if session.MerchantID != input.MerchantID {
			return result, NewRequestError(http.StatusBadRequest, errors.New("dining session merchant mismatch"))
		}
		result.DiningSession = &session

		if input.ReservationID != nil {
			if !session.ReservationID.Valid || session.ReservationID.Int64 != *input.ReservationID {
				return result, NewRequestError(http.StatusConflict, errors.New("dining session reservation mismatch"))
			}
		}

		var sessionReservation *db.TableReservation
		if session.ReservationID.Valid {
			reservation, err := store.GetTableReservation(ctx, session.ReservationID.Int64)
			if err != nil {
				if errors.Is(err, db.ErrRecordNotFound) {
					return result, NewRequestError(http.StatusNotFound, errors.New("reservation not found"))
				}
				return result, err
			}
			if reservation.MerchantID != input.MerchantID {
				return result, NewRequestError(http.StatusBadRequest, errors.New("reservation does not belong to this merchant"))
			}
			if reservation.TableID != *input.TableID {
				return result, NewRequestError(http.StatusBadRequest, errors.New("table does not match reservation"))
			}
			if !isOnlineReservationSource(reservation.Source) {
				return result, NewRequestError(http.StatusForbidden, errors.New("reservation does not belong to you"))
			}
			sessionReservation = &reservation
		}

		if input.ReservationID != nil {
			reservation := sessionReservation
			if reservation == nil {
				loadedReservation, err := store.GetTableReservation(ctx, *input.ReservationID)
				if err != nil {
					if errors.Is(err, db.ErrRecordNotFound) {
						return result, NewRequestError(http.StatusNotFound, errors.New("reservation not found"))
					}
					return result, err
				}
				reservation = &loadedReservation
			}
			if !isCustomerOwnedReservation(*reservation, input.UserID) {
				return result, NewRequestError(http.StatusForbidden, errors.New("reservation does not belong to you"))
			}
			if reservation.MerchantID != input.MerchantID {
				return result, NewRequestError(http.StatusBadRequest, errors.New("reservation does not belong to this merchant"))
			}
			if reservation.TableID != *input.TableID {
				return result, NewRequestError(http.StatusBadRequest, errors.New("table does not match reservation"))
			}
			if reservation.PaymentMode != "deposit" {
				return result, NewRequestError(http.StatusConflict, errors.New("reservation is not in deposit mode"))
			}
			if reservation.Status != "paid" && reservation.Status != "confirmed" && reservation.Status != "checked_in" {
				return result, NewRequestError(http.StatusConflict, errors.New("reservation is not ready for dining"))
			}

			result.Reservation = reservation
		}
	}

	if input.BillingGroupID != nil && input.OrderType != "dine_in" && input.OrderType != "reservation" {
		return result, NewRequestError(http.StatusBadRequest, errors.New("billing group is only allowed for dine-in or reservation"))
	}

	if input.OrderType == "dine_in" || input.OrderType == "reservation" {
		if result.DiningSession == nil {
			if input.OrderType == "dine_in" {
				return result, NewRequestError(http.StatusConflict, errors.New("no active dining session for billing"))
			}
			if input.BillingGroupID != nil {
				return result, NewRequestError(http.StatusBadRequest, errors.New("billing group requires active session"))
			}
		} else {
			var bg db.BillingGroup
			if input.BillingGroupID != nil {
				value, err := store.GetBillingGroup(ctx, *input.BillingGroupID)
				if err != nil {
					if errors.Is(err, db.ErrRecordNotFound) {
						return result, NewRequestError(http.StatusNotFound, errors.New("billing group not found"))
					}
					return result, err
				}
				bg = value
			} else {
				value, err := store.GetDefaultBillingGroupBySession(ctx, result.DiningSession.ID)
				if err != nil {
					if errors.Is(err, db.ErrRecordNotFound) {
						return result, NewRequestError(http.StatusConflict, errors.New("default billing group not found"))
					}
					return result, err
				}
				bg = value
			}

			if bg.DiningSessionID != result.DiningSession.ID {
				return result, NewRequestError(http.StatusConflict, errors.New("billing group does not belong to dining session"))
			}
			if bg.Status == "closed" {
				return result, NewRequestError(http.StatusConflict, errors.New("billing group is closed"))
			}
			if _, err := store.GetActiveBillingGroupMember(ctx, db.GetActiveBillingGroupMemberParams{
				BillingGroupID: bg.ID,
				UserID:         input.UserID,
			}); err != nil {
				if errors.Is(err, db.ErrRecordNotFound) {
					return result, NewRequestError(http.StatusForbidden, errors.New("not a member of billing group"))
				}
				return result, err
			}

			if input.BillingGroupID == nil {
				bgID := bg.ID
				result.BillingGroupID = &bgID
			}
		}
	}

	return result, nil
}
