package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
)

// ==================== 取消预定事务 ====================

// CancelReservationTxParams contains the input parameters for cancelling a reservation
type CancelReservationTxParams struct {
	ReservationID int64
	TableID       int64
	CancelReason  string
	// 用于检查是否需要释放桌台
	CurrentReservationID pgtype.Int8
	// P1-029 修复：是否在事务中同时释放库存
	ReleaseInventory bool
}

// CancelReservationTxResult contains the result of the cancel reservation transaction
type CancelReservationTxResult struct {
	Reservation  TableReservation
	TableUpdated bool
}

// CancelReservationTx cancels a reservation and releases the table in a single transaction:
// 1. Update reservation status to cancelled
// 2. Release table if it was assigned to this reservation
// 3. P1-029 修复：释放已预订的库存（同一事务内，确保原子性）
func (store *SQLStore) CancelReservationTx(ctx context.Context, arg CancelReservationTxParams) (CancelReservationTxResult, error) {
	var result CancelReservationTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error
		if err := ensureNoActiveReservationAdjustmentWithQueries(ctx, q, arg.ReservationID); err != nil {
			return err
		}

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

		// 3. P1-029 修复：在同一事务中释放已预订的库存
		if arg.ReleaseInventory {
			reservation, err := q.GetTableReservation(ctx, arg.ReservationID)
			if err != nil {
				return fmt.Errorf("get reservation for inventory release: %w", err)
			}

			entries, err := q.ListReservationInventoryByReservation(ctx, arg.ReservationID)
			if err != nil {
				return fmt.Errorf("list reservation inventory: %w", err)
			}

			for _, e := range entries {
				if e.Quantity <= 0 {
					continue
				}
				if err := releaseReservedInventoryIfTracked(ctx, q, ReleaseReservedInventoryParams{
					MerchantID:       reservation.MerchantID,
					DishID:           e.DishID,
					Date:             reservation.ReservationDate,
					ReservedQuantity: e.Quantity,
				}); err != nil {
					return fmt.Errorf("release reserved inventory for dish %d: %w", e.DishID, err)
				}
				if err := q.DeleteReservationInventoryByDish(ctx, DeleteReservationInventoryByDishParams{
					ReservationID: arg.ReservationID,
					DishID:        e.DishID,
				}); err != nil {
					return fmt.Errorf("delete reservation inventory for dish %d: %w", e.DishID, err)
				}
			}
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
		if err := ensureNoActiveReservationAdjustmentWithQueries(ctx, q, arg.ReservationID); err != nil {
			return err
		}

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

		// 3. P1-064 记录未到店风险行为。线下/电话客户不能归因到录入预约的员工账号。
		decisionUserID := pgtype.Int8{Int64: result.Reservation.UserID, Valid: true}
		traceSummary := "User failed to show up for reservation"
		if isOfflineReservationSource(result.Reservation.Source) {
			decisionUserID = pgtype.Int8{}
			traceSummary = "Offline customer failed to show up for reservation"
		}
		_, err = q.CreateBehaviorDecision(ctx, CreateBehaviorDecisionParams{
			OrderID:            pgtype.Int8{}, // null
			ReservationID:      pgtype.Int8{Int64: arg.ReservationID, Valid: true},
			UserID:             decisionUserID,
			MerchantID:         pgtype.Int8{Int64: result.Reservation.MerchantID, Valid: true},
			RiderID:            pgtype.Int8{},
			DecisionVersion:    "v1",
			ReasonCodes:        []string{"reservation_no_show"},
			ResponsibleParty:   "user",
			CompensationSource: "unknown",
			DecisionStatus:     "decided",
			TraceSummary:       pgtype.Text{String: traceSummary, Valid: true},
			FactSnapshot:       buildReservationNoShowFactSnapshot(result.Reservation),
		})
		if err != nil {
			return fmt.Errorf("create behavior decision: %w", err)
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

// ConfirmReservationTx confirms a reservation without occupying the table:
// 1. Update reservation status to confirmed
// 2. Return the current table snapshot for callers that need it
func (store *SQLStore) ConfirmReservationTx(ctx context.Context, arg ConfirmReservationTxParams) (ConfirmReservationTxResult, error) {
	var result ConfirmReservationTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// 1. 更新预定状态为已确认
		result.Reservation, err = q.UpdateReservationToConfirmed(ctx, arg.ReservationID)
		if err != nil {
			return fmt.Errorf("update reservation to confirmed: %w", err)
		}

		// 2. 返回当前桌台快照；占桌动作由后续到店/开台流程负责
		result.Table, err = q.GetTable(ctx, arg.TableID)
		if err != nil {
			return fmt.Errorf("get table: %w", err)
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
		if err := ensureNoActiveReservationAdjustmentWithQueries(ctx, q, arg.ReservationID); err != nil {
			return err
		}

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
	ReservationID         int64
	ExpectedCurrentAmount int64
	Items                 []CreateReservationItemParams
}

// ReplaceReservationItemsTxResult contains the result of replacing reservation items
type ReplaceReservationItemsTxResult struct {
	Items       []ReservationItem
	TotalAmount int64
}

type ReplaceReservationItemsWithRefundOrdersTxParams struct {
	ReservationID         int64
	ExpectedCurrentAmount int64
	Items                 []CreateReservationItemParams
	RefundOrders          []CreateRefundOrderTxParams
}

type ReplaceReservationItemsWithRefundOrdersTxResult struct {
	Items        []ReservationItem
	TotalAmount  int64
	RefundOrders []RefundOrder
}

// ReplaceReservationItemsTx replaces all reservation items in a single transaction
func (store *SQLStore) ReplaceReservationItemsTx(ctx context.Context, arg ReplaceReservationItemsTxParams) (ReplaceReservationItemsTxResult, error) {
	var result ReplaceReservationItemsTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		if _, err := q.GetTableReservationForUpdate(ctx, arg.ReservationID); err != nil {
			return fmt.Errorf("lock reservation: %w", err)
		}
		if _, err := q.GetActiveReservationAdjustmentByReservation(ctx, arg.ReservationID); err == nil {
			return &requestError{statusCode: http.StatusConflict, err: errors.New("预订存在待支付改菜补差单，请先完成或关闭支付单")}
		} else if !errors.Is(err, ErrRecordNotFound) {
			return fmt.Errorf("get active reservation adjustment: %w", err)
		}
		currentTotal, err := q.SumReservationItemsTotal(ctx, arg.ReservationID)
		if err != nil {
			return fmt.Errorf("sum reservation items total: %w", err)
		}
		if currentTotal != arg.ExpectedCurrentAmount {
			return &requestError{statusCode: http.StatusConflict, err: errors.New("预订菜品金额已变化，请刷新后重试")}
		}

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
		if _, err := syncReservationInventoryWithQueries(ctx, q, arg.ReservationID); err != nil {
			return fmt.Errorf("sync reservation inventory after replacing items: %w", err)
		}

		return nil
	})

	return result, err
}

func (store *SQLStore) ReplaceReservationItemsWithRefundOrdersTx(ctx context.Context, arg ReplaceReservationItemsWithRefundOrdersTxParams) (ReplaceReservationItemsWithRefundOrdersTxResult, error) {
	var result ReplaceReservationItemsWithRefundOrdersTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		if _, err := q.GetTableReservationForUpdate(ctx, arg.ReservationID); err != nil {
			return fmt.Errorf("lock reservation: %w", err)
		}
		if _, err := q.GetActiveReservationAdjustmentByReservation(ctx, arg.ReservationID); err == nil {
			return &requestError{statusCode: http.StatusConflict, err: errors.New("预订存在待支付改菜补差单，请先完成或关闭支付单")}
		} else if !errors.Is(err, ErrRecordNotFound) {
			return fmt.Errorf("get active reservation adjustment: %w", err)
		}
		currentTotal, err := q.SumReservationItemsTotal(ctx, arg.ReservationID)
		if err != nil {
			return fmt.Errorf("sum reservation items total: %w", err)
		}
		if currentTotal != arg.ExpectedCurrentAmount {
			return &requestError{statusCode: http.StatusConflict, err: errors.New("预订菜品金额已变化，请刷新后重试")}
		}

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

		result.RefundOrders = make([]RefundOrder, 0, len(arg.RefundOrders))
		for _, refundArg := range arg.RefundOrders {
			refundResult, err := createRefundOrderWithGuard(ctx, q, refundArg)
			if err != nil {
				return err
			}
			result.RefundOrders = append(result.RefundOrders, refundResult.RefundOrder)
		}
		if _, err := syncReservationInventoryWithQueries(ctx, q, arg.ReservationID); err != nil {
			return fmt.Errorf("sync reservation inventory after replacing items with refund orders: %w", err)
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
				MerchantID:       reservation.MerchantID,
				DishID:           dishID,
				Date:             reservation.ReservationDate,
				ReservedQuantity: delta,
			})
			if err != nil {
				if !errors.Is(err, ErrRecordNotFound) {
					return reservation, fmt.Errorf("reserve inventory: %w", err)
				}
				// No inventory record means unlimited; otherwise insufficient stock
				if _, getErr := q.GetDailyInventory(ctx, GetDailyInventoryParams{
					MerchantID: reservation.MerchantID,
					DishID:     dishID,
					Date:       reservation.ReservationDate,
				}); getErr != nil && !errors.Is(getErr, ErrRecordNotFound) {
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
			err := releaseReservedInventoryIfTracked(ctx, q, ReleaseReservedInventoryParams{
				MerchantID:       reservation.MerchantID,
				DishID:           dishID,
				Date:             reservation.ReservationDate,
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
		if err := releaseReservedInventoryIfTracked(ctx, q, ReleaseReservedInventoryParams{
			MerchantID:       reservation.MerchantID,
			DishID:           dishID,
			Date:             reservation.ReservationDate,
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
			if err := releaseReservedInventoryIfTracked(ctx, q, ReleaseReservedInventoryParams{
				MerchantID:       reservation.MerchantID,
				DishID:           e.DishID,
				Date:             reservation.ReservationDate,
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

func releaseReservedInventoryIfTracked(ctx context.Context, q *Queries, arg ReleaseReservedInventoryParams) error {
	if _, err := q.ReleaseReservedInventory(ctx, arg); err != nil && !errors.Is(err, ErrRecordNotFound) {
		return err
	}
	return nil
}

func ensureNoActiveReservationAdjustmentWithQueries(ctx context.Context, q *Queries, reservationID int64) error {
	if _, err := q.GetTableReservationForUpdate(ctx, reservationID); err != nil {
		return fmt.Errorf("lock reservation for active adjustment guard: %w", err)
	}
	if _, err := q.GetActiveReservationAdjustmentByReservation(ctx, reservationID); err == nil {
		return &requestError{statusCode: http.StatusConflict, err: errors.New("预订存在待支付改菜补差单，请先完成或关闭支付单")}
	} else if !errors.Is(err, ErrRecordNotFound) {
		return fmt.Errorf("get active reservation adjustment: %w", err)
	}
	return nil
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
	Items                []ReservationItemInput // 全款模式的预点菜品
	AfterLock            func(context.Context, *Queries) error
	DefaultDepositAmount int64
}

// CreateReservationTxResult contains the result of the create reservation transaction
type CreateReservationTxResult struct {
	Reservation TableReservation
	Items       []ReservationItem
}

type CreateMerchantReservationTxParams struct {
	CreateTableReservationByMerchantParams
	AfterLock func(context.Context, *Queries) error
}

type UpdateReservationTxParams struct {
	MerchantID  int64
	Reservation UpdateReservationParams
	OperatorID  pgtype.Int8
}

const (
	reservationPaymentModeDeposit = "deposit"
	reservationPaymentModeFull    = "full"
)

func isOfflineReservationSource(source pgtype.Text) bool {
	normalized := strings.TrimSpace(source.String)
	return source.Valid && normalized != "" && normalized != ReservationSourceOnline
}

func normalizedOfflineReservationSource(source pgtype.Text) string {
	switch strings.TrimSpace(source.String) {
	case ReservationSourcePhone:
		return ReservationSourcePhone
	case ReservationSourceWalkin:
		return ReservationSourceWalkin
	default:
		return ReservationSourceMerchant
	}
}

func isCustomerOwnedReservation(reservation TableReservation, userID int64) bool {
	return reservation.UserID == userID && !isOfflineReservationSource(reservation.Source)
}

func buildReservationNoShowFactSnapshot(reservation TableReservation) []byte {
	if !isOfflineReservationSource(reservation.Source) {
		return nil
	}

	snapshot := map[string]any{
		"customer_identity_type": "offline_customer",
		"reservation_source":     reservation.Source.String,
	}
	if reservation.OfflineCustomerID.Valid {
		snapshot["offline_customer_id"] = reservation.OfflineCustomerID.Int64
	}
	if reservation.CreatedByUserID.Valid {
		snapshot["created_by_user_id"] = reservation.CreatedByUserID.Int64
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return nil
	}
	return payload
}

func isReservationTerminalForUpdate(status string) bool {
	return status == ReservationStatusCompleted ||
		status == ReservationStatusCancelled ||
		status == ReservationStatusExpired
}

func ensureLockedTableReservableForReservation(table Table, merchantID int64, guestCount int16) error {
	if table.MerchantID != merchantID {
		return ErrTableMerchantMismatchForReservation
	}
	if table.TableType != TableTypeRoom {
		return ErrTableTypeNotReservable
	}
	if table.Status == TableStatusDisabled {
		return ErrTableDisabledForReservation
	}
	if guestCount > table.Capacity {
		return ErrReservationGuestCountExceedsCapacity
	}
	return nil
}

func applyLockedTableReservationPricing(table Table, arg *CreateTableReservationParams, defaultDepositAmount int64) error {
	switch arg.PaymentMode {
	case reservationPaymentModeDeposit:
		if table.MinimumSpend.Valid && table.MinimumSpend.Int64 > 0 {
			arg.DepositAmount = table.MinimumSpend.Int64
			return nil
		}
		if defaultDepositAmount > 0 {
			arg.DepositAmount = defaultDepositAmount
		}
	case reservationPaymentModeFull:
		if table.MinimumSpend.Valid && arg.PrepaidAmount < table.MinimumSpend.Int64 {
			return ErrReservationMinimumSpendNotMet
		}
	}
	return nil
}

func validateLockedTableMinimumSpendForReservation(table Table, reservation TableReservation) error {
	if !table.MinimumSpend.Valid || table.MinimumSpend.Int64 <= 0 {
		return nil
	}
	if isOfflineReservationSource(reservation.Source) {
		return nil
	}

	switch reservation.PaymentMode {
	case reservationPaymentModeDeposit:
		if reservation.DepositAmount < table.MinimumSpend.Int64 {
			return ErrReservationMinimumSpendNotMet
		}
	case reservationPaymentModeFull:
		if reservation.PrepaidAmount < table.MinimumSpend.Int64 {
			return ErrReservationMinimumSpendNotMet
		}
	}
	return nil
}

func updateReservationTargetChanged(current TableReservation, update UpdateReservationParams) bool {
	if update.TableID.Valid && update.TableID.Int64 != current.TableID {
		return true
	}
	if update.ReservationDate.Valid && !update.ReservationDate.Time.Equal(current.ReservationDate.Time) {
		return true
	}
	if update.ReservationTime.Valid && (!current.ReservationTime.Valid || update.ReservationTime.Microseconds != current.ReservationTime.Microseconds) {
		return true
	}
	return false
}

func reservationContactChanged(current TableReservation, update UpdateReservationParams) bool {
	return update.ContactName.Valid && strings.TrimSpace(update.ContactName.String) != current.ContactName ||
		update.ContactPhone.Valid && strings.TrimSpace(update.ContactPhone.String) != current.ContactPhone
}

func reservationTimeSlotConfigFromQueries(ctx context.Context, q *Queries, merchantID int64, date time.Time) util.TimeSlotConfig {
	config := util.DefaultConfig
	businessHours, err := q.ListMerchantBusinessHours(ctx, merchantID)
	if err != nil {
		return config
	}

	dayOfWeek := int32(date.Weekday())
	var todayHours []MerchantBusinessHour
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

	if len(todayHours) == 0 {
		return config
	}

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

	return config
}

func ensureUpdatedReservationHasNoConflict(ctx context.Context, q *Queries, reservation TableReservation, tableID int64, date time.Time, reservationTime pgtype.Time) error {
	if !reservationTime.Valid {
		return nil
	}

	existingReservations, err := q.ListReservationsByTableAndDate(ctx, ListReservationsByTableAndDateParams{
		TableID:         tableID,
		ReservationDate: pgtype.Date{Time: date, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("list existing reservations: %w", err)
	}

	config := reservationTimeSlotConfigFromQueries(ctx, q, reservation.MerchantID, date)
	newDateTime := util.CombineDateAndTime(date, reservationTime.Microseconds)
	for _, r := range existingReservations {
		if r.ID == reservation.ID {
			continue
		}
		if r.Status == ReservationStatusCancelled || r.Status == ReservationStatusExpired || r.Status == ReservationStatusNoShow {
			continue
		}
		if !r.ReservationTime.Valid {
			continue
		}
		existingTime := util.CombineDateAndTime(r.ReservationDate.Time, r.ReservationTime.Microseconds)
		if util.AreReservationsConflictingWithConfig(newDateTime, existingTime, config) {
			return ErrReservationTimeConflict
		}
	}
	return nil
}

func (store *SQLStore) UpdateReservationTx(ctx context.Context, arg UpdateReservationTxParams) (TableReservation, error) {
	var result TableReservation

	err := store.execTx(ctx, func(q *Queries) error {
		reservation, err := q.GetTableReservationForUpdate(ctx, arg.Reservation.ID)
		if err != nil {
			return fmt.Errorf("lock reservation: %w", err)
		}
		if reservation.MerchantID != arg.MerchantID {
			return ErrReservationMerchantMismatch
		}
		if isReservationTerminalForUpdate(reservation.Status) {
			return ErrReservationTerminalState
		}

		targetTableID := reservation.TableID
		if arg.Reservation.TableID.Valid {
			targetTableID = arg.Reservation.TableID.Int64
		}
		targetDate := reservation.ReservationDate.Time
		if arg.Reservation.ReservationDate.Valid {
			targetDate = arg.Reservation.ReservationDate.Time
		}
		targetTime := reservation.ReservationTime
		if arg.Reservation.ReservationTime.Valid {
			targetTime = arg.Reservation.ReservationTime
		}
		targetGuestCount := reservation.GuestCount
		if arg.Reservation.GuestCount.Valid {
			targetGuestCount = arg.Reservation.GuestCount.Int16
		}

		if arg.Reservation.TableID.Valid ||
			arg.Reservation.ReservationDate.Valid ||
			arg.Reservation.ReservationTime.Valid ||
			arg.Reservation.GuestCount.Valid {
			table, err := q.GetTableForUpdate(ctx, targetTableID)
			if err != nil {
				if errors.Is(err, ErrRecordNotFound) {
					return ErrTableNotFoundForReservation
				}
				return fmt.Errorf("lock table: %w", err)
			}
			if err := ensureLockedTableReservableForReservation(table, reservation.MerchantID, targetGuestCount); err != nil {
				return err
			}
			if err := validateLockedTableMinimumSpendForReservation(table, reservation); err != nil {
				return err
			}
		}

		if updateReservationTargetChanged(reservation, arg.Reservation) {
			if err := ensureUpdatedReservationHasNoConflict(ctx, q, reservation, targetTableID, targetDate, targetTime); err != nil {
				return err
			}
		}

		if isOfflineReservationSource(reservation.Source) && reservationContactChanged(reservation, arg.Reservation) {
			contactName := reservation.ContactName
			if arg.Reservation.ContactName.Valid {
				contactName = strings.TrimSpace(arg.Reservation.ContactName.String)
				arg.Reservation.ContactName = pgtype.Text{String: contactName, Valid: true}
			}
			contactPhone := reservation.ContactPhone
			if arg.Reservation.ContactPhone.Valid {
				contactPhone = strings.TrimSpace(arg.Reservation.ContactPhone.String)
				arg.Reservation.ContactPhone = pgtype.Text{String: contactPhone, Valid: true}
			}
			if contactName == "" || contactPhone == "" {
				return ErrReservationInvalidOfflineCustomerContact
			}

			operatorID := arg.OperatorID
			if !operatorID.Valid {
				operatorID = reservation.CreatedByUserID
			}
			offlineCustomer, err := q.UpsertMerchantOfflineCustomer(ctx, UpsertMerchantOfflineCustomerParams{
				MerchantID:      reservation.MerchantID,
				ContactName:     contactName,
				ContactPhone:    contactPhone,
				Source:          normalizedOfflineReservationSource(reservation.Source),
				CreatedByUserID: operatorID,
			})
			if err != nil {
				return fmt.Errorf("upsert reservation offline customer: %w", err)
			}
			arg.Reservation.OfflineCustomerID = pgtype.Int8{Int64: offlineCustomer.ID, Valid: true}
		}

		result, err = q.UpdateReservation(ctx, arg.Reservation)
		if err != nil {
			return fmt.Errorf("update reservation: %w", err)
		}

		return nil
	})

	return result, err
}

// CreateReservationTx creates a reservation with optional items in a single transaction:
// 1. Create the reservation
// 2. Create reservation items (for full payment mode)
func (store *SQLStore) CreateReservationTx(ctx context.Context, arg CreateReservationTxParams) (CreateReservationTxResult, error) {
	var result CreateReservationTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// 0. 锁定桌台，防止并发预订（P0-002 修复）
		table, err := q.GetTableForUpdate(ctx, arg.CreateTableReservationParams.TableID)
		if err != nil {
			return fmt.Errorf("get table check lock: %w", err)
		}
		if err := ensureLockedTableReservableForReservation(table, arg.MerchantID, arg.GuestCount); err != nil {
			return err
		}
		if err := applyLockedTableReservationPricing(table, &arg.CreateTableReservationParams, arg.DefaultDepositAmount); err != nil {
			return err
		}

		// 0.1 执行自定义校验 (如再次检查冲突)
		if arg.AfterLock != nil {
			if err := arg.AfterLock(ctx, q); err != nil {
				return err
			}
		}

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

func (store *SQLStore) CreateMerchantReservationTx(ctx context.Context, arg CreateMerchantReservationTxParams) (TableReservation, error) {
	var result TableReservation

	err := store.execTx(ctx, func(q *Queries) error {
		table, err := q.GetTableForUpdate(ctx, arg.TableID)
		if err != nil {
			return fmt.Errorf("get table check lock: %w", err)
		}
		if err := ensureLockedTableReservableForReservation(table, arg.MerchantID, arg.GuestCount); err != nil {
			return err
		}

		if arg.AfterLock != nil {
			if err := arg.AfterLock(ctx, q); err != nil {
				return err
			}
		}

		source := arg.Source
		if !source.Valid || strings.TrimSpace(source.String) == "" {
			source = pgtype.Text{String: ReservationSourceMerchant, Valid: true}
		} else {
			source = pgtype.Text{String: normalizedOfflineReservationSource(source), Valid: true}
		}
		arg.Source = source
		arg.ContactName = strings.TrimSpace(arg.ContactName)
		arg.ContactPhone = strings.TrimSpace(arg.ContactPhone)
		offlineCustomer, err := q.UpsertMerchantOfflineCustomer(ctx, UpsertMerchantOfflineCustomerParams{
			MerchantID:      arg.MerchantID,
			ContactName:     arg.ContactName,
			ContactPhone:    arg.ContactPhone,
			Source:          source.String,
			CreatedByUserID: pgtype.Int8{Int64: arg.UserID, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("upsert merchant offline customer: %w", err)
		}
		arg.OfflineCustomerID = pgtype.Int8{Int64: offlineCustomer.ID, Valid: true}
		arg.CreatedByUserID = pgtype.Int8{Int64: arg.UserID, Valid: true}

		result, err = q.CreateTableReservationByMerchant(ctx, arg.CreateTableReservationByMerchantParams)
		if err != nil {
			return fmt.Errorf("create merchant reservation: %w", err)
		}

		return nil
	})

	return result, err
}
