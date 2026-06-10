package db

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

var ErrReservationActiveOrderConflict = errors.New("reservation already has an active order")

// CreateOrderTxParams contains the input parameters for creating an order with items
type CreateOrderTxParams struct {
	CreateOrderParams                   CreateOrderParams
	Items                               []CreateOrderItemParams
	BillingGroupID                      *int64
	EnforceSingleActiveReservationOrder bool

	// 优惠券相关（可选）
	UserVoucherID *int64 // 用户优惠券ID
	VoucherAmount int64  // 优惠券抵扣金额

	// 余额支付相关（可选）
	MembershipID         *int64 // 会员卡ID
	BalancePaid          int64  // 余额支付金额
	BalancePaidPrincipal int64  // 余额支付本金
	BalancePaidBonus     int64  // 余额支付赠额

	// 代取精度相关（可选）
	// 代取精度相关（可选）
	DeliveryDuration   int32 // 代取预计在途时间（秒），由 LBS 真实路径计算得出
	RiderAverageSpeed  int   // 骑手平均速度（km/h），用于兜底估算
	DefaultPrepareTime int   // 默认出餐时间（分钟），用于兜底估算
	PickupTime         time.Time
}

// CreateOrderTxResult contains the result of the create order transaction
type CreateOrderTxResult struct {
	Order       Order
	Items       []OrderItem
	UserVoucher *UserVoucher           // 如果使用了优惠券
	Membership  *MerchantMembership    // 如果使用了余额
	Transaction *MembershipTransaction // 余额消费记录
}

// CreateOrderTx creates an order with all its items in a single transaction
// Also handles voucher usage and balance payment atomically
func (store *SQLStore) CreateOrderTx(ctx context.Context, arg CreateOrderTxParams) (CreateOrderTxResult, error) {
	var result CreateOrderTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		if orderTypeUsesDailyPickupCode(arg.CreateOrderParams.OrderType) && !arg.CreateOrderParams.PickupCode.Valid {
			pickupTime := arg.PickupTime
			if pickupTime.IsZero() {
				pickupTime = time.Now()
			}

			sequence, err := q.AllocateDailyPickupSequence(ctx, AllocateDailyPickupSequenceParams{
				MerchantID: arg.CreateOrderParams.MerchantID,
				PickupDate: pgtype.Date{Time: pickupTime, Valid: true},
			})
			if err != nil {
				return fmt.Errorf("allocate pickup sequence: %w", err)
			}

			if sequence > 9999 {
				return fmt.Errorf("pickup sequence exhausted for merchant %d on %s", arg.CreateOrderParams.MerchantID, pickupTime.Format("2006-01-02"))
			}

			arg.CreateOrderParams.PickupCode = pgtype.Text{String: fmt.Sprintf("%04d", sequence), Valid: true}
		}

		if arg.EnforceSingleActiveReservationOrder && arg.CreateOrderParams.ReservationID.Valid {
			if _, err := q.GetTableReservationForUpdate(ctx, arg.CreateOrderParams.ReservationID.Int64); err != nil {
				return fmt.Errorf("lock table reservation: %w", err)
			}

			existingOrder, err := q.GetLatestOrderByReservation(ctx, arg.CreateOrderParams.ReservationID)
			if err != nil {
				if !errors.Is(err, ErrRecordNotFound) {
					return fmt.Errorf("get latest reservation order: %w", err)
				}
			} else if existingOrder.Status != OrderStatusCancelled && !existingOrder.ReplacedByOrderID.Valid {
				return ErrReservationActiveOrderConflict
			}
		}

		// 1. 如果使用优惠券，先验证并锁定
		if arg.UserVoucherID != nil {
			userVoucher, err := q.GetUserVoucherForUpdate(ctx, *arg.UserVoucherID)
			if err != nil {
				return fmt.Errorf("get user voucher: %w", err)
			}

			// 检查优惠券状态
			if userVoucher.Status != "unused" {
				return fmt.Errorf("voucher already used: status=%s", userVoucher.Status)
			}

			// 检查优惠券是否过期
			if time.Now().After(userVoucher.ExpiresAt) {
				return fmt.Errorf("voucher has expired")
			}

			if _, err := lockUsableVoucherTemplate(ctx, q, userVoucher.VoucherID, time.Now()); err != nil {
				return err
			}

			result.UserVoucher = &userVoucher
		}

		// 2. 如果使用余额，先验证并锁定会员卡
		if arg.MembershipID != nil && arg.BalancePaid > 0 {
			membership, err := q.GetMembershipForUpdate(ctx, *arg.MembershipID)
			if err != nil {
				return fmt.Errorf("get membership: %w", err)
			}

			// 检查余额是否足够
			if membership.Balance < arg.BalancePaid {
				return fmt.Errorf("insufficient balance: have %d, need %d", membership.Balance, arg.BalancePaid)
			}

			result.Membership = &membership
		}

		// 3. 创建订单
		result.Order, err = q.CreateOrder(ctx, arg.CreateOrderParams)
		if err != nil {
			return fmt.Errorf("create order: %w", err)
		}

		// 4. 创建订单明细
		result.Items = make([]OrderItem, 0, len(arg.Items))
		for _, item := range arg.Items {
			item.OrderID = result.Order.ID
			orderItem, err := q.CreateOrderItem(ctx, item)
			if err != nil {
				return fmt.Errorf("create order item: %w", err)
			}
			result.Items = append(result.Items, orderItem)
		}

		// 4.1 账单组订单关联（可选）
		if arg.BillingGroupID != nil {
			if _, err := q.CreateBillingGroupOrder(ctx, CreateBillingGroupOrderParams{
				BillingGroupID: *arg.BillingGroupID,
				OrderID:        result.Order.ID,
				Amount:         result.Order.TotalAmount,
				Status:         "linked",
			}); err != nil {
				return fmt.Errorf("create billing group order: %w", err)
			}
		}

		// 5. 创建初始状态日志
		_, err = q.CreateOrderStatusLog(ctx, CreateOrderStatusLogParams{
			OrderID:    result.Order.ID,
			FromStatus: pgtype.Text{String: "", Valid: false},
			ToStatus:   result.Order.Status,
		})
		if err != nil {
			return fmt.Errorf("create status log: %w", err)
		}

		// 6. 如果使用优惠券，标记为已使用
		if arg.UserVoucherID != nil && result.UserVoucher != nil {
			updatedVoucher, err := q.MarkUserVoucherAsUsed(ctx, MarkUserVoucherAsUsedParams{
				ID:      *arg.UserVoucherID,
				OrderID: pgtype.Int8{Int64: result.Order.ID, Valid: true},
			})
			if err != nil {
				return fmt.Errorf("mark voucher as used: %w", err)
			}
			result.UserVoucher = &updatedVoucher

			// 增加优惠券使用计数
			_, err = q.IncrementVoucherUsedQuantity(ctx, result.UserVoucher.VoucherID)
			if err != nil {
				return fmt.Errorf("increment voucher used quantity: %w", err)
			}
		}

		// 7. 如果使用余额，扣减会员余额
		if arg.MembershipID != nil && arg.BalancePaid > 0 && result.Membership != nil {
			principalPaid := arg.BalancePaidPrincipal
			bonusPaid := arg.BalancePaidBonus
			if principalPaid+bonusPaid != arg.BalancePaid {
				bonusPaid = result.Membership.BonusBalance
				if bonusPaid > arg.BalancePaid {
					bonusPaid = arg.BalancePaid
				}
				principalPaid = arg.BalancePaid - bonusPaid
			}

			newPrincipal := result.Membership.PrincipalBalance - principalPaid
			newBonus := result.Membership.BonusBalance - bonusPaid
			newBalance := newPrincipal + newBonus

			updatedMembership, err := q.UpdateMembershipBalance(ctx, UpdateMembershipBalanceParams{
				ID:               *arg.MembershipID,
				Balance:          newBalance,
				PrincipalBalance: newPrincipal,
				BonusBalance:     newBonus,
				TotalRecharged:   result.Membership.TotalRecharged,
				TotalConsumed:    result.Membership.TotalConsumed + arg.BalancePaid,
			})
			if err != nil {
				return fmt.Errorf("update membership balance: %w", err)
			}
			result.Membership = &updatedMembership

			// 创建消费交易记录
			transaction, err := q.CreateMembershipTransaction(ctx, CreateMembershipTransactionParams{
				MembershipID:    *arg.MembershipID,
				Type:            "consume",
				Amount:          -arg.BalancePaid,
				PrincipalAmount: -principalPaid,
				BonusAmount:     -bonusPaid,
				BalanceAfter:    newBalance,
				RelatedOrderID:  pgtype.Int8{Int64: result.Order.ID, Valid: true},
				RechargeRuleID:  pgtype.Int8{},
				Notes:           pgtype.Text{String: fmt.Sprintf("订单支付: %s", result.Order.OrderNo), Valid: true},
			})
			if err != nil {
				return fmt.Errorf("create membership transaction: %w", err)
			}
			result.Transaction = &transaction
		}

		// 8. 🚀 如果是全额余额支付，在同一事务中直接完成支付处理（扣库存、推履约）
		// P1-031 / P1-047 修复：原子化余额支付流程，避免“扣了余额但订单仍为pending”的情况
		if arg.MembershipID != nil && arg.BalancePaid > 0 && arg.BalancePaid >= result.Order.TotalAmount {
			paymentResult, err := processOrderPaymentWithQueries(ctx, q, ProcessOrderPaymentTxParams{
				OrderID:            result.Order.ID,
				PaymentMethod:      orderPaymentMethodBalance,
				RiderAverageSpeed:  arg.RiderAverageSpeed,
				DefaultPrepareTime: arg.DefaultPrepareTime,
				DeliveryDuration:   arg.DeliveryDuration,
			})
			if err != nil {
				// 关键：如果扣库存或后续步骤失败，整个事务（包括余额扣除）回滚！
				return fmt.Errorf("process atomic balance payment: %w", err)
			}
			// 更新返回结果中的订单状态
			result.Order = paymentResult.Order
		}

		return nil
	})

	return result, err
}

func orderTypeUsesDailyPickupCode(orderType string) bool {
	switch orderType {
	case OrderTypeTakeout, OrderTypeTakeaway, OrderTypeDineIn:
		return true
	default:
		return false
	}
}

// ProcessOrderPaymentTxParams contains the input parameters for processing order payment
type ProcessOrderPaymentTxParams struct {
	OrderID            int64
	PaymentMethod      string
	RiderAverageSpeed  int
	DefaultPrepareTime int
	DeliveryDuration   int32 // 代取预计在途时间（秒），由 LBS 提供
}

// ProcessOrderPaymentTxResult contains the result of order payment processing
type ProcessOrderPaymentTxResult struct {
	Order    Order
	Delivery *Delivery     // 外卖订单仅在后续显式投池动作后才会有代取单
	PoolItem *DeliveryPool // 仅在订单已被投放到代取池时才有值
}

// ProcessOrderPaymentTx processes order payment and decrements inventory in a single transaction
// This ensures inventory is only deducted when payment succeeds, preventing overselling
// For takeout orders, delivery creation and entering delivery pool both happen later
// when the merchant explicitly advances the fulfillment flow.
func (store *SQLStore) ProcessOrderPaymentTx(ctx context.Context, arg ProcessOrderPaymentTxParams) (ProcessOrderPaymentTxResult, error) {
	var result ProcessOrderPaymentTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error
		result, err = processOrderPaymentWithQueries(ctx, q, arg)
		if err != nil {
			return err
		}
		return nil
	})

	return result, err
}

func processOrderPaymentWithQueries(ctx context.Context, q *Queries, arg ProcessOrderPaymentTxParams) (ProcessOrderPaymentTxResult, error) {
	var result ProcessOrderPaymentTxResult
	orderID := arg.OrderID

	// 1. Get order with lock for idempotency
	order, err := q.GetOrderForUpdate(ctx, orderID)
	if err != nil {
		return result, fmt.Errorf("get order: %w", err)
	}
	result.Order = order

	if order.Status == OrderStatusPaid {
		if order.OrderType == "takeout" {
			if delivery, err := q.GetDeliveryByOrderID(ctx, order.ID); err == nil {
				result.Delivery = &delivery
			}
			if poolItem, err := q.GetDeliveryPoolByOrderID(ctx, order.ID); err == nil {
				result.PoolItem = &poolItem
			}
		}
		return result, nil
	}

	// P1-060 Fix: Ensure we only process Pending orders.
	// If order is Cancelled/Refunded/Completed, we must not proceed with payment processing.
	if order.Status != OrderStatusPending {
		return result, fmt.Errorf("order status is %s, expected pending", order.Status)
	}

	// 2. Get order items
	orderItems, err := q.ListOrderItemsByOrder(ctx, order.ID)
	if err != nil {
		return result, fmt.Errorf("list order items: %w", err)
	}

	// ✅ P2-4: 按dish_id排序，确保所有事务按相同顺序加锁，避免死锁
	sort.Slice(orderItems, func(i, j int) bool {
		if !orderItems[i].DishID.Valid {
			return false
		}
		if !orderItems[j].DishID.Valid {
			return true
		}
		return orderItems[i].DishID.Int64 < orderItems[j].DishID.Int64
	})

	// 3. Decrement inventory for each dish (with FOR UPDATE lock)
	inventoryDate := pgtype.Date{Time: time.Now(), Valid: true}
	if order.OrderType == OrderTypeReservation && order.ReservationID.Valid {
		reservation, err := q.GetTableReservation(ctx, order.ReservationID.Int64)
		if err != nil && !errors.Is(err, ErrRecordNotFound) {
			return result, fmt.Errorf("get reservation for inventory date: %w", err)
		}
		if reservation.ReservationDate.Valid {
			inventoryDate = reservation.ReservationDate
		}
	}

	for _, item := range orderItems {
		// Skip if it's a combo (combos don't have direct inventory)
		if !item.DishID.Valid {
			continue
		}

		// 🔒 Lock the inventory row (FOR UPDATE)
		inventory, err := q.GetDailyInventoryForUpdate(ctx, GetDailyInventoryForUpdateParams{
			MerchantID: order.MerchantID,
			DishID:     item.DishID.Int64,
			Date:       inventoryDate,
		})
		if err != nil {
			if errors.Is(err, ErrRecordNotFound) {
				// No inventory configured means unlimited stock
				continue
			}
			return result, fmt.Errorf("get inventory for dish %d: %w", item.DishID.Int64, err)
		}

		// ✅ Check if there's enough stock
		if inventory.TotalQuantity != -1 { // -1 means unlimited
			available := inventory.TotalQuantity - inventory.SoldQuantity - inventory.ReservedQuantity
			if available < int32(item.Quantity) {
				return result, fmt.Errorf("insufficient inventory for dish %d: need %d, have %d",
					item.DishID.Int64, item.Quantity, available)
			}
		}

		// 预订订单：先释放预留库存
		if order.OrderType == OrderTypeReservation {
			if _, err := q.ReleaseReservedInventory(ctx, ReleaseReservedInventoryParams{
				MerchantID:       order.MerchantID,
				DishID:           item.DishID.Int64,
				Date:             inventoryDate,
				ReservedQuantity: int32(item.Quantity),
			}); err != nil {
				return result, fmt.Errorf("release reserved inventory for dish %d: %w", item.DishID.Int64, err)
			}
		}

		// ✅ Decrement inventory (already holding FOR UPDATE lock)
		_, err = q.CheckAndDecrementInventory(ctx, CheckAndDecrementInventoryParams{
			MerchantID:   order.MerchantID,
			DishID:       item.DishID.Int64,
			Date:         inventoryDate,
			SoldQuantity: int32(item.Quantity),
		})
		if err != nil {
			return result, fmt.Errorf("decrement inventory for dish %d: %w", item.DishID.Int64, err)
		}
	}

	// 4. Update order status to paid并推进履约状态
	newFulfillment := order.FulfillmentStatus
	if order.OrderType != OrderTypeReservation {
		newFulfillment = FulfillmentStatusPendingKitchen
	}
	paymentMethod := normalizeOrderPaymentMethod(arg.PaymentMethod)

	result.Order, err = q.UpdateOrderToPaid(ctx, UpdateOrderToPaidParams{
		ID:                order.ID,
		PaymentMethod:     pgtype.Text{String: paymentMethod, Valid: true},
		FulfillmentStatus: pgtype.Text{String: newFulfillment, Valid: true},
	})
	if err != nil {
		return result, fmt.Errorf("update order to paid: %w", err)
	}

	return result, nil
}
