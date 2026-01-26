package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

// OpenDiningSessionTxParams bundles inputs for opening a dining session transactionally.
type OpenDiningSessionTxParams struct {
	TableID                int64
	MerchantID             int64
	UserID                 int64
	ReservationID          pgtype.Int8
	ImportReservationItems bool
	ActivateOrder          *ActivateOrderInput
}

// CloseDiningSessionTxParams bundles inputs for closing a dining session transactionally.
type CloseDiningSessionTxParams struct {
	ID         int64
	MerchantID int64
}

// CloseDiningSessionTxResult captures the side effects of the transaction.
type CloseDiningSessionTxResult struct {
	Session DiningSession
}

// ActivateOrderInput describes an order update when binding to a session.
type ActivateOrderInput struct {
	OrderID              int64
	Status               string
	NewFulfillmentStatus pgtype.Text
}

// OpenDiningSessionTxResult captures the side effects of the transaction.
type OpenDiningSessionTxResult struct {
	Session        DiningSession
	BillingGroup   BillingGroup
	CartID         *int64
	ImportedItems  int
	ActivatedOrder *Order
}

// OpenDiningSessionTx performs creation of the dining session, optional cart import,
// order activation, and reservation check-in atomically.
func (store *SQLStore) OpenDiningSessionTx(ctx context.Context, arg OpenDiningSessionTxParams) (OpenDiningSessionTxResult, error) {
	var result OpenDiningSessionTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// 1) Create dining session
		result.Session, err = q.CreateDiningSession(ctx, CreateDiningSessionParams{
			MerchantID:    arg.MerchantID,
			TableID:       arg.TableID,
			ReservationID: arg.ReservationID,
			UserID:        arg.UserID,
			ActiveOrderID: pgtype.Int8{Valid: false},
			Status:        "open",
		})
		if err != nil {
			return fmt.Errorf("create dining session: %w", err)
		}

		// 1.1) Create default billing group and owner membership
		result.BillingGroup, err = q.CreateBillingGroup(ctx, CreateBillingGroupParams{
			DiningSessionID: result.Session.ID,
			Status:          "open",
			IsDefault:       true,
			TotalAmount:     0,
			PaidAmount:      0,
		})
		if err != nil {
			return fmt.Errorf("create default billing group: %w", err)
		}

		if _, err := q.CreateBillingGroupMember(ctx, CreateBillingGroupMemberParams{
			BillingGroupID: result.BillingGroup.ID,
			UserID:         arg.UserID,
			Role:           "owner",
		}); err != nil {
			return fmt.Errorf("create billing group member: %w", err)
		}

		// 2) Import reservation items into a dine-in cart if needed
		if arg.ImportReservationItems && arg.ReservationID.Valid {
			cart, err := q.CreateCart(ctx, CreateCartParams{
				UserID:        arg.UserID,
				MerchantID:    arg.MerchantID,
				OrderType:     "dine_in",
				TableID:       pgtype.Int8{Int64: arg.TableID, Valid: true},
				ReservationID: arg.ReservationID,
			})
			if err != nil {
				return fmt.Errorf("create cart: %w", err)
			}

			if err := q.ClearCart(ctx, cart.ID); err != nil {
				return fmt.Errorf("clear cart: %w", err)
			}

			items, err := q.ListReservationItems(ctx, arg.ReservationID.Int64)
			if err != nil {
				return fmt.Errorf("list reservation items: %w", err)
			}

			for _, it := range items {
				var dishID, comboID pgtype.Int8
				if it.DishID.Valid {
					dishID = pgtype.Int8{Int64: it.DishID.Int64, Valid: true}
				}
				if it.ComboID.Valid {
					comboID = pgtype.Int8{Int64: it.ComboID.Int64, Valid: true}
				}
				if _, err := q.AddCartItem(ctx, AddCartItemParams{
					CartID:         cart.ID,
					DishID:         dishID,
					ComboID:        comboID,
					Quantity:       it.Quantity,
					Customizations: nil,
				}); err != nil {
					return fmt.Errorf("add cart item: %w", err)
				}
				result.ImportedItems++
			}

			result.CartID = &cart.ID
		}

		// 3) Activate reservation order and bind as active order if requested
		if arg.ActivateOrder != nil {
			updatedOrder, err := q.UpdateOrderStatus(ctx, UpdateOrderStatusParams{
				Status:            arg.ActivateOrder.Status,
				FulfillmentStatus: arg.ActivateOrder.NewFulfillmentStatus,
				ID:                arg.ActivateOrder.OrderID,
			})
			if err != nil {
				return fmt.Errorf("update order status: %w", err)
			}

			result.Session.ActiveOrderID = pgtype.Int8{Int64: updatedOrder.ID, Valid: true}
			result.ActivatedOrder = &updatedOrder

			if _, err := q.UpdateDiningSessionActiveOrder(ctx, UpdateDiningSessionActiveOrderParams{
				ID:            result.Session.ID,
				ActiveOrderID: pgtype.Int8{Int64: updatedOrder.ID, Valid: true},
			}); err != nil {
				return fmt.Errorf("update dining session active order: %w", err)
			}
		}

		// 4) Mark reservation checked in if applicable
		if arg.ReservationID.Valid {
			if _, err := q.UpdateReservationToCheckedIn(ctx, arg.ReservationID.Int64); err != nil {
				return fmt.Errorf("update reservation to checked in: %w", err)
			}
		}

		// 5) Update table status to occupied
		_, err = q.UpdateTableStatus(ctx, UpdateTableStatusParams{
			ID:                   arg.TableID,
			Status:               "occupied",
			CurrentReservationID: arg.ReservationID,
		})
		if err != nil {
			return fmt.Errorf("update table status to occupied: %w", err)
		}

		return nil
	})

	return result, err
}

// CloseDiningSessionTx performs closure of the dining session, updates table status,
// and closes all associated billing groups.
func (store *SQLStore) CloseDiningSessionTx(ctx context.Context, arg CloseDiningSessionTxParams) (CloseDiningSessionTxResult, error) {
	var result CloseDiningSessionTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		session, err := closeDiningSessionInternal(ctx, q, arg.ID, arg.MerchantID)
		if err != nil {
			return err
		}
		result.Session = session
		return nil
	})

	return result, err
}

func closeDiningSessionInternal(ctx context.Context, q *Queries, sessionID int64, merchantID int64) (DiningSession, error) {
	// 1) Get dining session
	session, err := q.GetDiningSession(ctx, sessionID)
	if err != nil {
		return DiningSession{}, fmt.Errorf("get dining session: %w", err)
	}

	if session.MerchantID != merchantID {
		return DiningSession{}, fmt.Errorf("dining session does not belong to this merchant")
	}

	if session.Status == "closed" {
		return session, nil
	}

	// 2) Close dining session
	session, err = q.CloseDiningSession(ctx, sessionID)
	if err != nil {
		return DiningSession{}, fmt.Errorf("close dining session: %w", err)
	}

	// 3) Close all billing groups associated with this session
	groups, err := q.ListBillingGroupsBySession(ctx, sessionID)
	if err != nil {
		return DiningSession{}, fmt.Errorf("list billing groups: %w", err)
	}

	for _, group := range groups {
		if group.Status != "closed" {
			_, err = q.UpdateBillingGroupStatus(ctx, UpdateBillingGroupStatusParams{
				ID:     group.ID,
				Status: "closed",
			})
			if err != nil {
				return DiningSession{}, fmt.Errorf("close billing group %d: %w", group.ID, err)
			}
		}
	}

	// 4) Update table status to available
	table, err := q.GetTable(ctx, session.TableID)
	if err != nil {
		return DiningSession{}, fmt.Errorf("get table: %w", err)
	}

	// If the table was occupied for a specific reservation, and that reservation is now done,
	// we should clear CurrentReservationID if it matches Session.ReservationID.
	newReservationID := table.CurrentReservationID
	if session.ReservationID.Valid && table.CurrentReservationID.Valid && session.ReservationID.Int64 == table.CurrentReservationID.Int64 {
		newReservationID = pgtype.Int8{Valid: false}
	}

	_, err = q.UpdateTableStatus(ctx, UpdateTableStatusParams{
		ID:                   session.TableID,
		Status:               "available",
		CurrentReservationID: newReservationID,
	})
	if err != nil {
		return DiningSession{}, fmt.Errorf("update table status to available: %w", err)
	}

	// 5) Update reservation status to 'completed'
	if session.ReservationID.Valid {
		if _, err := q.db.Exec(ctx, `UPDATE table_reservations SET status = 'completed', completed_at = now(), updated_at = now() WHERE id = $1`, session.ReservationID.Int64); err != nil {
			return DiningSession{}, fmt.Errorf("update reservation to completed: %w", err)
		}
	}

	return session, nil
}
