package db

import (
	"context"
	"fmt"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ==================== 取消预定事务 ====================

// CancelReservationTxParams contains the input parameters for cancelling a reservation
type CancelReservationTxParams struct {
	ReservationID int64
	TableID       int64
	CancelReason  string
	// 用于检查是否需要释放桌台
	CurrentReservationID pgtype.Int8
}

// CancelReservationTxResult contains the result of the cancel reservation transaction
type CancelReservationTxResult struct {
	Reservation  TableReservation
	TableUpdated bool
}

// CancelReservationTx cancels a reservation and releases the table in a single transaction:
// 1. Update reservation status to cancelled
// 2. Release table if it was assigned to this reservation
func (store *SQLStore) CancelReservationTx(ctx context.Context, arg CancelReservationTxParams) (CancelReservationTxResult, error) {
	var result CancelReservationTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// 1. 更新预定状态为已取消
		var cancelReason pgtype.Text
		if arg.CancelReason != "" {
			cancelReason = pgtype.Text{String: arg.CancelReason, Valid: true}
		}

		result.Reservation, err = q.UpdateReservationToCancelled(ctx, UpdateReservationToCancelledParams{
			ID:           arg.ReservationID,
			CancelReason: cancelReason,
		})
		if err != nil {
			return fmt.Errorf("update reservation to cancelled: %w", err)
		}

		// 2. 如果桌台当前预定是此预定，释放桌台
		if arg.CurrentReservationID.Valid && arg.CurrentReservationID.Int64 == arg.ReservationID {
			_, err = q.UpdateTableStatus(ctx, UpdateTableStatusParams{
				ID:                   arg.TableID,
				Status:               "available",
				CurrentReservationID: pgtype.Int8{Valid: false},
			})
			if err != nil {
				return fmt.Errorf("update table status: %w", err)
			}
			result.TableUpdated = true
		}

		return nil
	})

	return result, err
}

// ==================== 标记未到店事务 ====================

// MarkNoShowTxParams contains the input parameters for marking a reservation as no-show
type MarkNoShowTxParams struct {
	ReservationID        int64
	TableID              int64
	CurrentReservationID pgtype.Int8
}

// MarkNoShowTxResult contains the result of the mark no-show transaction
type MarkNoShowTxResult struct {
	Reservation  TableReservation
	TableUpdated bool
}

// MarkNoShowTx marks a reservation as no-show and releases the table in a single transaction:
// 1. Update reservation status to no_show
// 2. Release table if it was assigned to this reservation
func (store *SQLStore) MarkNoShowTx(ctx context.Context, arg MarkNoShowTxParams) (MarkNoShowTxResult, error) {
	var result MarkNoShowTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// 1. 更新预定状态为未到店
		result.Reservation, err = q.UpdateReservationToNoShow(ctx, arg.ReservationID)
		if err != nil {
			return fmt.Errorf("update reservation to no_show: %w", err)
		}

		// 2. 如果桌台当前预定是此预定，释放桌台
		if arg.CurrentReservationID.Valid && arg.CurrentReservationID.Int64 == arg.ReservationID {
			_, err = q.UpdateTableStatus(ctx, UpdateTableStatusParams{
				ID:                   arg.TableID,
				Status:               "available",
				CurrentReservationID: pgtype.Int8{Valid: false},
			})
			if err != nil {
				return fmt.Errorf("update table status: %w", err)
			}
			result.TableUpdated = true
		}

		return nil
	})

	return result, err
}

// ==================== 确认预定事务 ====================

// ConfirmReservationTxParams contains the input parameters for confirming a reservation
type ConfirmReservationTxParams struct {
	ReservationID int64
	TableID       int64
}

// ConfirmReservationTxResult contains the result of the confirm reservation transaction
type ConfirmReservationTxResult struct {
	Reservation TableReservation
	Table       Table
}

// ConfirmReservationTx confirms a reservation and updates the table status in a single transaction:
// 1. Update reservation status to confirmed
// 2. Update table to reserved with current reservation ID
func (store *SQLStore) ConfirmReservationTx(ctx context.Context, arg ConfirmReservationTxParams) (ConfirmReservationTxResult, error) {
	var result ConfirmReservationTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// 1. 更新预定状态为已确认
		result.Reservation, err = q.UpdateReservationToConfirmed(ctx, arg.ReservationID)
		if err != nil {
			return fmt.Errorf("update reservation to confirmed: %w", err)
		}

		// 2. 更新桌台状态为已预定
		result.Table, err = q.UpdateTableStatus(ctx, UpdateTableStatusParams{
			ID:                   arg.TableID,
			Status:               "reserved",
			CurrentReservationID: pgtype.Int8{Int64: arg.ReservationID, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("update table status: %w", err)
		}

		return nil
	})

	return result, err
}

// ==================== 完成预定事务 ====================

// CompleteReservationTxParams contains the input parameters for completing a reservation
type CompleteReservationTxParams struct {
	ReservationID        int64
	TableID              int64
	CurrentReservationID pgtype.Int8
}

// CompleteReservationTxResult contains the result of the complete reservation transaction
type CompleteReservationTxResult struct {
	Reservation  TableReservation
	TableUpdated bool
}

// CompleteReservationTx completes a reservation and releases the table in a single transaction:
// 1. Update reservation status to completed
// 2. Release table if it was assigned to this reservation
func (store *SQLStore) CompleteReservationTx(ctx context.Context, arg CompleteReservationTxParams) (CompleteReservationTxResult, error) {
	var result CompleteReservationTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// 1. 更新预定状态为已完成
		result.Reservation, err = q.UpdateReservationToCompleted(ctx, arg.ReservationID)
		if err != nil {
			return fmt.Errorf("update reservation to completed: %w", err)
		}

		// 2. 如果桌台当前预定是此预定，释放桌台
		if arg.CurrentReservationID.Valid && arg.CurrentReservationID.Int64 == arg.ReservationID {
			_, err = q.UpdateTableStatus(ctx, UpdateTableStatusParams{
				ID:                   arg.TableID,
				Status:               "available",
				CurrentReservationID: pgtype.Int8{Valid: false},
			})
			if err != nil {
				return fmt.Errorf("update table status: %w", err)
			}
			result.TableUpdated = true
		}

		return nil
	})

	return result, err
}

// ==================== 预订菜品替换事务 ====================

// ReplaceReservationItemsTxParams contains the input parameters for replacing reservation items
type ReplaceReservationItemsTxParams struct {
	ReservationID int64
	Items         []CreateReservationItemParams
}

// ReplaceReservationItemsTxResult contains the result of replacing reservation items
type ReplaceReservationItemsTxResult struct {
	Items       []ReservationItem
	TotalAmount int64
}

// ReplaceReservationItemsTx replaces all reservation items in a single transaction
func (store *SQLStore) ReplaceReservationItemsTx(ctx context.Context, arg ReplaceReservationItemsTxParams) (ReplaceReservationItemsTxResult, error) {
	var result ReplaceReservationItemsTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		if err := q.DeleteReservationItems(ctx, arg.ReservationID); err != nil {
			return fmt.Errorf("delete reservation items: %w", err)
		}

		result.Items = make([]ReservationItem, 0, len(arg.Items))
		for _, item := range arg.Items {
			created, err := q.CreateReservationItem(ctx, item)
			if err != nil {
				return fmt.Errorf("create reservation item: %w", err)
			}
			result.Items = append(result.Items, created)
			result.TotalAmount += created.TotalPrice
		}

		return nil
	})

	return result, err
}

// ==================== 预订库存同步事务 ====================

// SyncReservationInventoryTxParams contains the input parameters for syncing reservation inventory
type SyncReservationInventoryTxParams struct {
	ReservationID int64
}

// SyncReservationInventoryTxResult captures the sync result
type SyncReservationInventoryTxResult struct {
	Reservation TableReservation
}

// SyncReservationInventoryTx syncs reserved inventory with current reservation items
func (store *SQLStore) SyncReservationInventoryTx(ctx context.Context, arg SyncReservationInventoryTxParams) (SyncReservationInventoryTxResult, error) {
	var result SyncReservationInventoryTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		reservation, err := syncReservationInventoryWithQueries(ctx, q, arg.ReservationID)
		if err != nil {
			return err
		}
		result.Reservation = reservation
		return nil
	})

	return result, err
}

func syncReservationInventoryWithQueries(ctx context.Context, q *Queries, reservationID int64) (TableReservation, error) {
	reservation, err := q.GetTableReservation(ctx, reservationID)
	if err != nil {
		return reservation, fmt.Errorf("get reservation: %w", err)
	}

	items, err := q.ListReservationDishSummary(ctx, reservationID)
	if err != nil {
		return reservation, fmt.Errorf("list reservation dish summary: %w", err)
	}
	existing, err := q.ListReservationInventoryByReservation(ctx, reservationID)
	if err != nil {
		return reservation, fmt.Errorf("list reservation inventory: %w", err)
	}

	existingMap := make(map[int64]int32, len(existing))
	for _, e := range existing {
		existingMap[e.DishID] = e.Quantity
	}

	for _, it := range items {
		if !it.DishID.Valid {
			continue
		}
		dishID := it.DishID.Int64
		desired := it.Quantity
		current := existingMap[dishID]
		delta := desired - current

		if delta > 0 {
			_, err := q.ReserveInventory(ctx, ReserveInventoryParams{
				MerchantID: reservation.MerchantID,
				DishID:     dishID,
				Date:       reservation.ReservationDate,
				ReservedQuantity: delta,
			})
			if err != nil {
				if !errors.Is(err, pgx.ErrNoRows) {
					return reservation, fmt.Errorf("reserve inventory: %w", err)
				}
				// No inventory record means unlimited; otherwise insufficient stock
				if _, getErr := q.GetDailyInventory(ctx, GetDailyInventoryParams{
					MerchantID: reservation.MerchantID,
					DishID:     dishID,
					Date:       reservation.ReservationDate,
				}); getErr != nil && !errors.Is(getErr, pgx.ErrNoRows) {
					return reservation, fmt.Errorf("get daily inventory: %w", getErr)
				}
				if _, getErr := q.GetDailyInventory(ctx, GetDailyInventoryParams{
					MerchantID: reservation.MerchantID,
					DishID:     dishID,
					Date:       reservation.ReservationDate,
				}); getErr == nil {
					return reservation, fmt.Errorf("insufficient inventory for reservation")
				}
			}
		} else if delta < 0 {
			_, err := q.ReleaseReservedInventory(ctx, ReleaseReservedInventoryParams{
				MerchantID: reservation.MerchantID,
				DishID:     dishID,
				Date:       reservation.ReservationDate,
				ReservedQuantity: -delta,
			})
			if err != nil {
				return reservation, fmt.Errorf("release reserved inventory: %w", err)
			}
		}

		if _, err := q.UpsertReservationInventory(ctx, UpsertReservationInventoryParams{
			ReservationID: reservationID,
			DishID:        dishID,
			Quantity:      desired,
		}); err != nil {
			return reservation, fmt.Errorf("upsert reservation inventory: %w", err)
		}
		delete(existingMap, dishID)
	}

	for dishID, qty := range existingMap {
		if qty <= 0 {
			if err := q.DeleteReservationInventoryByDish(ctx, DeleteReservationInventoryByDishParams{
				ReservationID: reservationID,
				DishID:        dishID,
			}); err != nil {
				return reservation, fmt.Errorf("delete reservation inventory: %w", err)
			}
			continue
		}
		if _, err := q.ReleaseReservedInventory(ctx, ReleaseReservedInventoryParams{
			MerchantID: reservation.MerchantID,
			DishID:     dishID,
			Date:       reservation.ReservationDate,
			ReservedQuantity: qty,
		}); err != nil {
			return reservation, fmt.Errorf("release reserved inventory: %w", err)
		}
		if err := q.DeleteReservationInventoryByDish(ctx, DeleteReservationInventoryByDishParams{
			ReservationID: reservationID,
			DishID:        dishID,
		}); err != nil {
			return reservation, fmt.Errorf("delete reservation inventory: %w", err)
		}
	}

	return reservation, nil
}

// ==================== 预订库存释放事务 ====================

// ReleaseReservationInventoryTxParams contains the input parameters for releasing reservation inventory
type ReleaseReservationInventoryTxParams struct {
	ReservationID int64
}

// ReleaseReservationInventoryTx releases all reserved inventory for a reservation
func (store *SQLStore) ReleaseReservationInventoryTx(ctx context.Context, arg ReleaseReservationInventoryTxParams) error {
	return store.execTx(ctx, func(q *Queries) error {
		reservation, err := q.GetTableReservation(ctx, arg.ReservationID)
		if err != nil {
			return fmt.Errorf("get reservation: %w", err)
		}

		entries, err := q.ListReservationInventoryByReservation(ctx, arg.ReservationID)
		if err != nil {
			return fmt.Errorf("list reservation inventory: %w", err)
		}

		for _, e := range entries {
			if e.Quantity <= 0 {
				continue
			}
			if _, err := q.ReleaseReservedInventory(ctx, ReleaseReservedInventoryParams{
				MerchantID: reservation.MerchantID,
				DishID:     e.DishID,
				Date:       reservation.ReservationDate,
				ReservedQuantity: e.Quantity,
			}); err != nil {
				return fmt.Errorf("release reserved inventory: %w", err)
			}
			if err := q.DeleteReservationInventoryByDish(ctx, DeleteReservationInventoryByDishParams{
				ReservationID: arg.ReservationID,
				DishID:        e.DishID,
			}); err != nil {
				return fmt.Errorf("delete reservation inventory: %w", err)
			}
		}

		return nil
	})
}

// ==================== 创建预定事务 ====================

// ReservationItemInput 预定菜品项输入
type ReservationItemInput struct {
	DishID    *int64
	ComboID   *int64
	Quantity  int16
	UnitPrice int64
}

// CreateReservationTxParams contains the input parameters for creating a reservation
type CreateReservationTxParams struct {
	CreateTableReservationParams
	Items []ReservationItemInput // 全款模式的预点菜品
}

// CreateReservationTxResult contains the result of the create reservation transaction
type CreateReservationTxResult struct {
	Reservation TableReservation
	Items       []ReservationItem
}

// CreateReservationTx creates a reservation with optional items in a single transaction:
// 1. Create the reservation
// 2. Create reservation items (for full payment mode)
func (store *SQLStore) CreateReservationTx(ctx context.Context, arg CreateReservationTxParams) (CreateReservationTxResult, error) {
	var result CreateReservationTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// 1. 创建预定
		result.Reservation, err = q.CreateTableReservation(ctx, arg.CreateTableReservationParams)
		if err != nil {
			return fmt.Errorf("create reservation: %w", err)
		}

		// 2. 创建预定菜品明细（如果有）
		if len(arg.Items) > 0 {
			result.Items = make([]ReservationItem, len(arg.Items))
			for i, item := range arg.Items {
				totalPrice := item.UnitPrice * int64(item.Quantity)

				var dishID, comboID pgtype.Int8
				if item.DishID != nil {
					dishID = pgtype.Int8{Int64: *item.DishID, Valid: true}
				}
				if item.ComboID != nil {
					comboID = pgtype.Int8{Int64: *item.ComboID, Valid: true}
				}

				result.Items[i], err = q.CreateReservationItem(ctx, CreateReservationItemParams{
					ReservationID: result.Reservation.ID,
					DishID:        dishID,
					ComboID:       comboID,
					Quantity:      item.Quantity,
					UnitPrice:     item.UnitPrice,
					TotalPrice:    totalPrice,
				})
				if err != nil {
					return fmt.Errorf("create reservation item %d: %w", i, err)
				}
			}
		}

		return nil
	})

	return result, err
}
