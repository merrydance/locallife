package db

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// CreateOrderTxParams contains the input parameters for creating an order with items
type CreateOrderTxParams struct {
	CreateOrderParams CreateOrderParams
	Items             []CreateOrderItemParams
	BillingGroupID    *int64

	// 优惠券相关（可选）
	UserVoucherID *int64 // 用户优惠券ID
	VoucherAmount int64  // 优惠券抵扣金额

	// 余额支付相关（可选）
	MembershipID *int64 // 会员卡ID
	BalancePaid  int64  // 余额支付金额

	// 配送精度相关（可选）
	DeliveryDuration int32 // 配送预计在途时间（秒），由 LBS 真实路径计算得出
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
			amount := result.Order.TotalAmount
			if result.Order.FinalAmount.Valid {
				amount = result.Order.FinalAmount.Int64
			}
			if _, err := q.CreateBillingGroupOrder(ctx, CreateBillingGroupOrderParams{
				BillingGroupID: *arg.BillingGroupID,
				OrderID:        result.Order.ID,
				Amount:         amount,
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
			newBalance := result.Membership.Balance - arg.BalancePaid

			updatedMembership, err := q.UpdateMembershipBalance(ctx, UpdateMembershipBalanceParams{
				ID:             *arg.MembershipID,
				Balance:        newBalance,
				TotalRecharged: result.Membership.TotalRecharged,
				TotalConsumed:  result.Membership.TotalConsumed + arg.BalancePaid,
			})
			if err != nil {
				return fmt.Errorf("update membership balance: %w", err)
			}
			result.Membership = &updatedMembership

			// 创建消费交易记录
			transaction, err := q.CreateMembershipTransaction(ctx, CreateMembershipTransactionParams{
				MembershipID:   *arg.MembershipID,
				Type:           "consume",
				Amount:         -arg.BalancePaid,
				BalanceAfter:   newBalance,
				RelatedOrderID: pgtype.Int8{Int64: result.Order.ID, Valid: true},
				RechargeRuleID: pgtype.Int8{},
				Notes:          pgtype.Text{String: fmt.Sprintf("订单支付: %s", result.Order.OrderNo), Valid: true},
			})
			if err != nil {
				return fmt.Errorf("create membership transaction: %w", err)
			}
			result.Transaction = &transaction
		}

		return nil
	})

	return result, err
}

// ProcessOrderPaymentTxParams contains the input parameters for processing order payment
type ProcessOrderPaymentTxParams struct {
	OrderID            int64
	RiderAverageSpeed  int
	DefaultPrepareTime int
	DeliveryDuration   int32 // 配送预计在途时间（秒），由 LBS 提供
}

// ProcessOrderPaymentTxResult contains the result of order payment processing
type ProcessOrderPaymentTxResult struct {
	Order    Order
	Delivery *Delivery     // 只有外卖订单才有配送单
	PoolItem *DeliveryPool // 只有外卖订单才会进入配送池
}

// ProcessOrderPaymentTx processes order payment and decrements inventory in a single transaction
// This ensures inventory is only deducted when payment succeeds, preventing overselling
// For takeout orders, it also creates delivery record and adds to delivery pool
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

	result.Order, err = q.UpdateOrderStatus(ctx, UpdateOrderStatusParams{
		ID:                order.ID,
		Status:            OrderStatusPaid,
		FulfillmentStatus: pgtype.Text{String: newFulfillment, Valid: true},
	})
	if err != nil {
		return result, fmt.Errorf("update order status: %w", err)
	}

	// 5. 🚀 如果是外卖订单(takeout)，创建配送单并推入配送池
	if result.Order.OrderType == "takeout" {
		// 获取商户信息（取餐地址）
		merchant, err := q.GetMerchant(ctx, result.Order.MerchantID)
		if err != nil {
			return result, fmt.Errorf("get merchant: %w", err)
		}

		// 获取收货地址
		if !result.Order.AddressID.Valid {
			return result, fmt.Errorf("takeout order missing delivery address")
		}
		userAddress, err := q.GetUserAddress(ctx, result.Order.AddressID.Int64)
		if err != nil {
			return result, fmt.Errorf("get user address: %w", err)
		}

		// ========== 计算预估出餐时间 ==========
		// 策略：取订单中各菜品制作时间的最大值
		// 如果没有菜品制作时间数据，则使用商户平均出餐时间
		// 如果商户也没有历史数据，使用入参中的默认值
		const avgPrepareTimeCalcDays = 7 // 计算平均出餐时间的天数范围

		riderSpeedMetersPerHour := arg.RiderAverageSpeed
		if riderSpeedMetersPerHour <= 0 {
			riderSpeedMetersPerHour = 15000 // Fallback
		}

		defaultPrepareTimeMinutes := arg.DefaultPrepareTime
		if defaultPrepareTimeMinutes <= 0 {
			defaultPrepareTimeMinutes = 20 // Fallback
		}

		now := time.Now()
		var maxPrepareTime int16 = 0

		// 遍历订单菜品，获取最长制作时间
		for _, item := range orderItems {
			if item.DishID.Valid {
				dish, err := q.GetDish(ctx, item.DishID.Int64)
				if err == nil && dish.PrepareTime > maxPrepareTime {
					maxPrepareTime = dish.PrepareTime
				}
			}
		}

		// 如果没有找到菜品制作时间，尝试获取商户平均出餐时间
		if maxPrepareTime == 0 {
			calcStartTime := now.AddDate(0, 0, -avgPrepareTimeCalcDays)
			avgTime, err := q.GetMerchantAvgPrepareTime(ctx, GetMerchantAvgPrepareTimeParams{
				MerchantID: merchant.ID,
				StartAt:    calcStartTime,
			})
			if err == nil && avgTime > 0 {
				maxPrepareTime = int16(avgTime)
			}
		}

		// 如果仍然没有数据，使用默认值
		if maxPrepareTime == 0 {
			maxPrepareTime = int16(defaultPrepareTimeMinutes)
		}

		// 预计出餐时间 = 当前时间 + 最大菜品制作时间
		estimatedPickupAt := now.Add(time.Duration(maxPrepareTime) * time.Minute)

		// ========== 计算预估送达时间 ==========
		// 配送时间计算策略（SSOT原则）：
		// 1. 优先使用订单落库时持久化的精准时间 (Consistent Content)
		// 2. 其次使用当前事务传入的实时 LBS 时间
		// 3. 最后回退到基于平均速度的物理估算
		var deliveryDistance int32
		if result.Order.DeliveryDistance.Valid {
			deliveryDistance = result.Order.DeliveryDistance.Int32
		}

		var deliveryTimeMinutes float64
		if result.Order.DeliveryDuration.Valid && result.Order.DeliveryDuration.Int32 > 0 {
			deliveryTimeMinutes = float64(result.Order.DeliveryDuration.Int32) / 60.0
		} else if arg.DeliveryDuration > 0 {
			deliveryTimeMinutes = float64(arg.DeliveryDuration) / 60.0
		} else {
			// 配送时间（分钟）= 距离(米) / 速度(米/小时) * 60
			deliveryTimeMinutes = float64(deliveryDistance) / float64(riderSpeedMetersPerHour) * 60
		}

		// 最少5分钟配送时间
		if deliveryTimeMinutes < 5 {
			deliveryTimeMinutes = 5
		}

		// 预计送达时间 = 预计出餐时间 + 配送时间
		estimatedDeliveryAt := estimatedPickupAt.Add(time.Duration(deliveryTimeMinutes) * time.Minute)

		// 创建配送单
		delivery, err := q.CreateDelivery(ctx, CreateDeliveryParams{
			OrderID:             result.Order.ID,
			PickupAddress:       merchant.Address,
			PickupLongitude:     merchant.Longitude,
			PickupLatitude:      merchant.Latitude,
			PickupContact:       pgtype.Text{String: merchant.Name, Valid: true},
			PickupPhone:         pgtype.Text{String: merchant.Phone, Valid: true},
			DeliveryAddress:     userAddress.DetailAddress,
			DeliveryLongitude:   userAddress.Longitude,
			DeliveryLatitude:    userAddress.Latitude,
			DeliveryContact:     pgtype.Text{String: userAddress.ContactName, Valid: true},
			DeliveryPhone:       pgtype.Text{String: userAddress.ContactPhone, Valid: true},
			Distance:            deliveryDistance,
			DeliveryFee:         result.Order.DeliveryFee,
			EstimatedPickupAt:   pgtype.Timestamptz{Time: estimatedPickupAt, Valid: true},
			EstimatedDeliveryAt: pgtype.Timestamptz{Time: estimatedDeliveryAt, Valid: true},
		})
		if err != nil {
			return result, fmt.Errorf("create delivery: %w", err)
		}
		result.Delivery = &delivery

		// 推入配送池
		// 优先级根据运费金额设置：运费越高优先级越高，骑手更愿意接
		priority := int32(1)
		if result.Order.DeliveryFee >= 1000 { // 运费>=10元，提高优先级
			priority = 2
		}
		if result.Order.DeliveryFee >= 2000 { // 运费>=20元，高优先级
			priority = 3
		}

		// expires_at 字段不再用于过滤，设置一个很远的未来时间
		// 外卖订单会一直在配送池中可见，直到被骑手接单或订单取消
		poolItem, err := q.AddToDeliveryPool(ctx, AddToDeliveryPoolParams{
			OrderID:            result.Order.ID,
			MerchantID:         merchant.ID,
			PickupLongitude:    merchant.Longitude,
			PickupLatitude:     merchant.Latitude,
			DeliveryLongitude:  userAddress.Longitude,
			DeliveryLatitude:   userAddress.Latitude,
			Distance:           deliveryDistance,
			DeliveryFee:        result.Order.DeliveryFee,
			ExpectedPickupAt:   estimatedPickupAt,
			ExpectedDeliveryAt: pgtype.Timestamptz{Time: estimatedDeliveryAt, Valid: true},
			ExpiresAt:          now.Add(365 * 24 * time.Hour), // 设置一年后，实际不再用于过滤
			Priority:           priority,
		})
		if err != nil {
			return result, fmt.Errorf("add to delivery pool: %w", err)
		}
		result.PoolItem = &poolItem
	}

	return result, nil
}
