package db

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// ==================== 订单状态变更事务 ====================

// UpdateOrderStatusTxParams contains the input parameters for updating order status with log
type UpdateOrderStatusTxParams struct {
	OrderID      int64
	NewStatus    string
	OldStatus    string // 用于日志记录
	OperatorID   int64
	OperatorType string // "user", "merchant", "system", "rider"
	Notes        string // 可选备注
	// 可选履约状态变更，nil 表示不变
	NewFulfillmentStatus *string
}

// UpdateOrderStatusTxResult contains the result of the update order status transaction
type UpdateOrderStatusTxResult struct {
	Order     Order
	StatusLog OrderStatusLog
}

// UpdateOrderStatusTx updates order status and creates a status log in a single transaction
func (store *SQLStore) UpdateOrderStatusTx(ctx context.Context, arg UpdateOrderStatusTxParams) (UpdateOrderStatusTxResult, error) {
	var result UpdateOrderStatusTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// 1. 更新订单状态
		var fulfillment pgtype.Text
		if arg.NewFulfillmentStatus != nil {
			fulfillment = pgtype.Text{String: *arg.NewFulfillmentStatus, Valid: true}
		}

		result.Order, err = q.UpdateOrderStatus(ctx, UpdateOrderStatusParams{
			ID:                arg.OrderID,
			Status:            arg.NewStatus,
			FulfillmentStatus: fulfillment,
		})
		if err != nil {
			return fmt.Errorf("update order status: %w", err)
		}

		// 2. 创建状态变更日志
		var notes pgtype.Text
		if arg.Notes != "" {
			notes = pgtype.Text{String: arg.Notes, Valid: true}
		}

		result.StatusLog, err = q.CreateOrderStatusLog(ctx, CreateOrderStatusLogParams{
			OrderID:      arg.OrderID,
			FromStatus:   pgtype.Text{String: arg.OldStatus, Valid: true},
			ToStatus:     arg.NewStatus,
			OperatorID:   pgtype.Int8{Int64: arg.OperatorID, Valid: true},
			OperatorType: pgtype.Text{String: arg.OperatorType, Valid: true},
			Notes:        notes,
		})
		if err != nil {
			return fmt.Errorf("create order status log: %w", err)
		}

		return nil
	})

	return result, err
}

// ==================== 订单完成事务 ====================

// CompleteOrderTxParams contains the input parameters for completing an order
type CompleteOrderTxParams struct {
	OrderID      int64
	OldStatus    string
	OperatorID   int64
	OperatorType string
}

// CompleteOrderTxResult contains the result of the complete order transaction
type CompleteOrderTxResult struct {
	Order     Order
	StatusLog OrderStatusLog
}

// CompleteOrderTx completes an order and creates a status log in a single transaction
func (store *SQLStore) CompleteOrderTx(ctx context.Context, arg CompleteOrderTxParams) (CompleteOrderTxResult, error) {
	var result CompleteOrderTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// 1. 更新订单为已完成
		result.Order, err = q.UpdateOrderToCompleted(ctx, arg.OrderID)
		if err != nil {
			return fmt.Errorf("update order to completed: %w", err)
		}

		// 2. 创建状态变更日志
		result.StatusLog, err = q.CreateOrderStatusLog(ctx, CreateOrderStatusLogParams{
			OrderID:      arg.OrderID,
			FromStatus:   pgtype.Text{String: arg.OldStatus, Valid: true},
			ToStatus:     "completed",
			OperatorID:   pgtype.Int8{Int64: arg.OperatorID, Valid: true},
			OperatorType: pgtype.Text{String: arg.OperatorType, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("create order status log: %w", err)
		}

		return nil
	})

	return result, err
}

// ==================== 订单取消事务 ====================

// CancelOrderTxParams contains the input parameters for cancelling an order
type CancelOrderTxParams struct {
	OrderID      int64
	OldStatus    string
	CancelReason string
	OperatorID   int64
	OperatorType string // "user", "merchant", "system"
}

// CancelOrderTxResult contains the result of the cancel order transaction
type CancelOrderTxResult struct {
	Order     Order
	StatusLog OrderStatusLog
}

// CancelOrderTx cancels an order and creates a status log in a single transaction
func (store *SQLStore) CancelOrderTx(ctx context.Context, arg CancelOrderTxParams) (CancelOrderTxResult, error) {
	var result CancelOrderTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// 0. 如果订单已扣减库存（paid状态），先准备恢复库存
		var orderItems []OrderItem
		if arg.OldStatus == OrderStatusPaid {
			orderItems, err = q.ListOrderItemsByOrder(ctx, arg.OrderID)
			if err != nil {
				return fmt.Errorf("list order items for cancel: %w", err)
			}
		}

		// 1. 更新订单为已取消
		var cancelReason pgtype.Text
		if arg.CancelReason != "" {
			cancelReason = pgtype.Text{String: arg.CancelReason, Valid: true}
		}

		result.Order, err = q.UpdateOrderToCancelled(ctx, UpdateOrderToCancelledParams{
			ID:           arg.OrderID,
			CancelReason: cancelReason,
		})
		if err != nil {
			return fmt.Errorf("update order to cancelled: %w", err)
		}

		// 2. 创建状态变更日志
		result.StatusLog, err = q.CreateOrderStatusLog(ctx, CreateOrderStatusLogParams{
			OrderID:      arg.OrderID,
			FromStatus:   pgtype.Text{String: arg.OldStatus, Valid: true},
			ToStatus:     "cancelled",
			OperatorID:   pgtype.Int8{Int64: arg.OperatorID, Valid: true},
			OperatorType: pgtype.Text{String: arg.OperatorType, Valid: true},
			Notes:        cancelReason,
		})
		if err != nil {
			return fmt.Errorf("create order status log: %w", err)
		}

		// 2.1 取消订单时回滚优惠券使用状态（幂等）
		if result.Order.UserVoucherID.Valid {
			userVoucher, err := q.GetUserVoucherForUpdate(ctx, result.Order.UserVoucherID.Int64)
			if err != nil {
				if !errors.Is(err, ErrRecordNotFound) {
					return fmt.Errorf("get user voucher for rollback: %w", err)
				}
			} else if userVoucher.Status == "used" && userVoucher.OrderID.Valid && userVoucher.OrderID.Int64 == result.Order.ID {
				orderID := pgtype.Int8{Int64: result.Order.ID, Valid: true}
				if time.Now().After(userVoucher.ExpiresAt) {
					if _, err := q.MarkUserVoucherAsExpiredOnRollback(ctx, MarkUserVoucherAsExpiredOnRollbackParams{
						ID:      userVoucher.ID,
						OrderID: orderID,
					}); err != nil && !errors.Is(err, ErrRecordNotFound) {
						return fmt.Errorf("mark voucher expired on rollback: %w", err)
					}
				} else {
					if _, err := q.MarkUserVoucherAsUnused(ctx, MarkUserVoucherAsUnusedParams{
						ID:      userVoucher.ID,
						OrderID: orderID,
					}); err != nil && !errors.Is(err, ErrRecordNotFound) {
						return fmt.Errorf("mark voucher unused on rollback: %w", err)
					}
				}

				if _, err := q.DecrementVoucherUsedQuantity(ctx, userVoucher.VoucherID); err != nil {
					return fmt.Errorf("decrement voucher used quantity: %w", err)
				}
			}
		}

		// 2.2 P1-059 修复：取消订单时回滚会员余额支付（原子性保证）
		if result.Order.BalancePaid > 0 && result.Order.MembershipID.Valid {
			membership, err := q.GetMembershipForUpdate(ctx, result.Order.MembershipID.Int64)
			if err != nil {
				if !errors.Is(err, ErrRecordNotFound) {
					return fmt.Errorf("get membership for balance rollback: %w", err)
				}
				// 会员卡已删除，记录日志但不阻塞取消流程
			} else {
				// 回滚余额：加回已支付金额
				newBalance := membership.Balance + result.Order.BalancePaid
				newTotalConsumed := membership.TotalConsumed - result.Order.BalancePaid
				if newTotalConsumed < 0 {
					newTotalConsumed = 0
				}

				_, err = q.UpdateMembershipBalance(ctx, UpdateMembershipBalanceParams{
					ID:             membership.ID,
					Balance:        newBalance,
					TotalRecharged: membership.TotalRecharged,
					TotalConsumed:  newTotalConsumed,
				})
				if err != nil {
					return fmt.Errorf("rollback membership balance: %w", err)
				}

				// 创建退款流水记录
				_, err = q.CreateMembershipTransaction(ctx, CreateMembershipTransactionParams{
					MembershipID:   membership.ID,
					Type:           "refund",
					Amount:         result.Order.BalancePaid,
					BalanceAfter:   newBalance,
					RelatedOrderID: pgtype.Int8{Int64: result.Order.ID, Valid: true},
					RechargeRuleID: pgtype.Int8{},
					Notes:          pgtype.Text{String: fmt.Sprintf("订单取消退回余额: %s", result.Order.OrderNo), Valid: true},
				})
				if err != nil {
					return fmt.Errorf("create membership refund transaction: %w", err)
				}
			}
		}

		// 3. 若订单之前处于已支付状态，恢复已售库存
		if arg.OldStatus == OrderStatusPaid {
			inventoryDate := pgtype.Date{Time: time.Now(), Valid: true}
			if result.Order.OrderType == OrderTypeReservation && result.Order.ReservationID.Valid {
				reservation, invErr := q.GetTableReservation(ctx, result.Order.ReservationID.Int64)
				if invErr != nil && !errors.Is(invErr, ErrRecordNotFound) {
					return fmt.Errorf("get reservation for inventory restore: %w", invErr)
				}
				if reservation.ReservationDate.Valid {
					inventoryDate = reservation.ReservationDate
				}
			}

			// P1-028 修复：按 DishID 排序防止并发死锁
			sort.Slice(orderItems, func(i, j int) bool {
				return orderItems[i].DishID.Int64 < orderItems[j].DishID.Int64
			})

			for _, item := range orderItems {
				if !item.DishID.Valid {
					continue
				}

				inv, invErr := q.GetDailyInventoryForUpdate(ctx, GetDailyInventoryForUpdateParams{
					MerchantID: result.Order.MerchantID,
					DishID:     item.DishID.Int64,
					Date:       inventoryDate,
				})
				if invErr != nil {
					if errors.Is(invErr, ErrRecordNotFound) {
						continue
					}
					return fmt.Errorf("get inventory for restore: %w", invErr)
				}

				newSold := inv.SoldQuantity - int32(item.Quantity)
				if newSold < 0 {
					newSold = 0
				}

				_, invErr = q.UpdateDailyInventory(ctx, UpdateDailyInventoryParams{
					TotalQuantity: pgtype.Int4{Int32: inv.TotalQuantity, Valid: true},
					SoldQuantity:  pgtype.Int4{Int32: newSold, Valid: true},
					MerchantID:    inv.MerchantID,
					DishID:        inv.DishID,
					Date:          inv.Date,
				})
				if invErr != nil {
					return fmt.Errorf("restore inventory for dish %d: %w", item.DishID.Int64, invErr)
				}
			}
		}

		return nil
	})

	return result, err
}
