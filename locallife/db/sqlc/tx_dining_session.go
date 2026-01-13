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

// ActivateOrderInput describes an order update when binding to a session.
type ActivateOrderInput struct {
	OrderID              int64
	Status               string
	NewFulfillmentStatus pgtype.Text
}

// OpenDiningSessionTxResult captures the side effects of the transaction.
type OpenDiningSessionTxResult struct {
	Session        DiningSession
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

		return nil
	})

	return result, err
}
