package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
)

var (
	ErrDiningSessionNotOpen   = errors.New("dining session is not open")
	ErrTargetTableDisabled    = errors.New("target table is disabled")
	ErrTargetTableOccupied    = errors.New("target table is occupied")
	ErrTargetTableReserved    = errors.New("target table is reserved")
	ErrReservationMismatch    = errors.New("reservation mismatch")
	ErrDiningSessionNotFound  = errors.New("dining session not found")
	ErrDiningSessionTableSame = errors.New("target table is same as current")
)

type TransferDiningSessionTableTxParams struct {
	SessionID      int64
	ToTableID      int64
	OperatorUserID int64
	Reason         pgtype.Text
}

type TransferDiningSessionTableTxResult struct {
	Session   DiningSession
	FromTable Table
	ToTable   Table
}

// TransferDiningSessionTableTx transfers an open dining session to another table within a transaction.
func (store *SQLStore) TransferDiningSessionTableTx(ctx context.Context, arg TransferDiningSessionTableTxParams) (TransferDiningSessionTableTxResult, error) {
	var result TransferDiningSessionTableTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var session DiningSession
		row := q.db.QueryRow(ctx, `SELECT id, merchant_id, table_id, reservation_id, user_id, active_order_id, status, opened_at, closed_at, created_at, updated_at
			FROM dining_sessions WHERE id = $1 FOR UPDATE`, arg.SessionID)
		if err := row.Scan(
			&session.ID,
			&session.MerchantID,
			&session.TableID,
			&session.ReservationID,
			&session.UserID,
			&session.ActiveOrderID,
			&session.Status,
			&session.OpenedAt,
			&session.ClosedAt,
			&session.CreatedAt,
			&session.UpdatedAt,
		); err != nil {
			if errors.Is(err, ErrRecordNotFound) {
				return ErrDiningSessionNotFound
			}
			return fmt.Errorf("get dining session for update: %w", err)
		}

		if session.Status != "open" {
			return ErrDiningSessionNotOpen
		}
		if session.TableID == arg.ToTableID {
			return ErrDiningSessionTableSame
		}

		fromTable, err := q.GetTableForUpdate(ctx, session.TableID)
		if err != nil {
			return fmt.Errorf("get from table: %w", err)
		}
		toTable, err := q.GetTableForUpdate(ctx, arg.ToTableID)
		if err != nil {
			return fmt.Errorf("get to table: %w", err)
		}
		if fromTable.MerchantID != session.MerchantID || toTable.MerchantID != session.MerchantID {
			return fmt.Errorf("table does not belong to session merchant")
		}
		if toTable.Status == "disabled" {
			return ErrTargetTableDisabled
		}

		if existing, err := q.GetActiveDiningSessionByTable(ctx, toTable.ID); err == nil {
			if existing.ID != session.ID {
				return ErrTargetTableOccupied
			}
		} else if !errors.Is(err, ErrRecordNotFound) {
			return fmt.Errorf("check active dining session: %w", err)
		}

		if session.ReservationID.Valid {
			res, err := q.GetTableReservationForUpdate(ctx, session.ReservationID.Int64)
			if err != nil {
				return fmt.Errorf("get reservation for update: %w", err)
			}
			if res.MerchantID != session.MerchantID {
				return ErrReservationMismatch
			}
			if res.TableID != fromTable.ID {
				return ErrReservationMismatch
			}

			// Check if target table has a conflicting reservation
			if toTable.CurrentReservationID.Valid && toTable.CurrentReservationID.Int64 != res.ID {
				targetRes, err := q.GetTableReservation(ctx, toTable.CurrentReservationID.Int64)
				if err == nil {
					resStart := util.CombineDateAndTime(targetRes.ReservationDate.Time, targetRes.ReservationTime.Microseconds)
					if util.IsConflictWithReservation(time.Now(), resStart) {
						return ErrTargetTableReserved
					}
				} else {
					return ErrTargetTableReserved
				}
			}
			if toTable.Status == "reserved" && (!toTable.CurrentReservationID.Valid || toTable.CurrentReservationID.Int64 != res.ID) {
				// Similar check for status
				if toTable.CurrentReservationID.Valid {
					targetRes, err := q.GetTableReservation(ctx, toTable.CurrentReservationID.Int64)
					if err == nil {
						resStart := util.CombineDateAndTime(targetRes.ReservationDate.Time, targetRes.ReservationTime.Microseconds)
						if util.IsConflictWithReservation(time.Now(), resStart) {
							return ErrTargetTableReserved
						}
					} else {
						return ErrTargetTableReserved
					}
				} else {
					return ErrTargetTableReserved
				}
			}
		} else {
			if toTable.Status == "reserved" || toTable.CurrentReservationID.Valid {
				return ErrTargetTableReserved
			}
		}

		if _, err := q.db.Exec(ctx, `UPDATE dining_sessions SET table_id = $1, updated_at = now() WHERE id = $2`, toTable.ID, session.ID); err != nil {
			return fmt.Errorf("update dining session table: %w", err)
		}

		if session.ReservationID.Valid {
			if _, err := q.db.Exec(ctx, `UPDATE table_reservations SET table_id = $1, updated_at = now() WHERE id = $2`, toTable.ID, session.ReservationID.Int64); err != nil {
				return fmt.Errorf("update reservation table: %w", err)
			}
		}

		updatedFrom, err := q.UpdateTableStatus(ctx, UpdateTableStatusParams{
			ID:                   fromTable.ID,
			Status:               "available",
			CurrentReservationID: pgtype.Int8{Valid: false},
		})
		if err != nil {
			return fmt.Errorf("update from table status: %w", err)
		}

		// [Change] The target table is now occupied, but it doesn't "adopt" the reservation record
		// to avoid displaying information that was specific to the original reserved table.
		newToResID := toTable.CurrentReservationID
		if session.ReservationID.Valid {
			newToResID = session.ReservationID
		}

		updatedTo, err := q.UpdateTableStatus(ctx, UpdateTableStatusParams{
			ID:                   toTable.ID,
			Status:               "occupied",
			CurrentReservationID: newToResID,
		})
		if err != nil {
			return fmt.Errorf("update to table status: %w", err)
		}

		billingGroups, err := q.ListBillingGroupsBySession(ctx, session.ID)
		if err != nil {
			return fmt.Errorf("list billing groups: %w", err)
		}
		for _, bg := range billingGroups {
			orders, err := q.ListBillingGroupOrdersByGroup(ctx, bg.ID)
			if err != nil {
				return fmt.Errorf("list billing group orders: %w", err)
			}
			for _, o := range orders {
				if _, err := q.db.Exec(ctx, `UPDATE orders SET table_id = $1, updated_at = now() WHERE id = $2`, toTable.ID, o.OrderID); err != nil {
					return fmt.Errorf("update order table: %w", err)
				}
			}
		}

		if _, err := q.db.Exec(ctx, `INSERT INTO table_transfer_logs
			(merchant_id, dining_session_id, reservation_id, from_table_id, to_table_id, operator_user_id, reason)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			session.MerchantID,
			session.ID,
			session.ReservationID,
			fromTable.ID,
			toTable.ID,
			arg.OperatorUserID,
			arg.Reason,
		); err != nil {
			return fmt.Errorf("insert transfer log: %w", err)
		}

		updatedSession, err := q.GetDiningSession(ctx, session.ID)
		if err != nil {
			return fmt.Errorf("get updated dining session: %w", err)
		}

		result.Session = updatedSession
		result.FromTable = updatedFrom
		result.ToTable = updatedTo
		return nil
	})

	return result, err
}
