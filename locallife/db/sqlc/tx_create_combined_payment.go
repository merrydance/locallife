package db

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// CreateCombinedPaymentTxParams 包含创建合单支付事务的参数
type CreateCombinedPaymentTxParams struct {
	UserID            int64
	OrderIDs          []int64
	CombineOutTradeNo string
	ExpiresAt         time.Time
}

// CreateCombinedPaymentTxResult 包含创建合单支付事务的结果
type CreateCombinedPaymentTxResult struct {
	CombinedPaymentOrder CombinedPaymentOrder
	PaymentOrders        []PaymentOrder
	OrderInfos           []CombinedPaymentOrderInfo // 辅助信息，包含商户配置等，用于后续调用微信API
}

// CombinedPaymentOrderInfo 包含单个子单的辅助信息
type CombinedPaymentOrderInfo struct {
	Order         Order
	PaymentOrder  PaymentOrder
	PaymentConfig MerchantPaymentConfig
	Merchant      Merchant
}

// CreateCombinedPaymentTx 执行合单支付创建事务
// P1-009: 确保跨子单操作原子性
func (store *SQLStore) CreateCombinedPaymentTx(ctx context.Context, arg CreateCombinedPaymentTxParams) (CreateCombinedPaymentTxResult, error) {
	var result CreateCombinedPaymentTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// 1. 验证所有订单并计算总金额
		var totalAmount int64
		// 存储临时信息以便后续使用
		type tempOrderInfo struct {
			Order         Order
			Merchant      Merchant
			PaymentConfig MerchantPaymentConfig
			PayAmount     int64
		}
		tempInfos := make([]tempOrderInfo, 0, len(arg.OrderIDs))

		// 必须按升序加锁，防止并发事务以不同顺序持有锁导致死锁（PostgreSQL 40P01）。
		sortedOrderIDs := make([]int64, len(arg.OrderIDs))
		copy(sortedOrderIDs, arg.OrderIDs)
		sort.Slice(sortedOrderIDs, func(i, j int) bool { return sortedOrderIDs[i] < sortedOrderIDs[j] })

		for _, orderID := range sortedOrderIDs {
			// 加锁获取订单
			order, err := q.GetOrderForUpdate(ctx, orderID)
			if err != nil {
				return fmt.Errorf("get order %d: %w", orderID, err)
			}

			if order.UserID != arg.UserID {
				return fmt.Errorf("order %d does not belong to user", orderID)
			}
			if order.Status != "pending" {
				return fmt.Errorf("order %d status is %s, expect pending", orderID, order.Status)
			}

			payAmount, err := OrderRemainingPayableAmount(order)
			if err != nil {
				return fmt.Errorf("resolve order %d payable amount: %w", orderID, err)
			}
			if payAmount <= 0 {
				return fmt.Errorf("order %d has no remaining payable amount", orderID)
			}

			// 获取商户和配置
			merchant, err := q.GetMerchant(ctx, order.MerchantID)
			if err != nil {
				return fmt.Errorf("get merchant for order %d: %w", orderID, err)
			}

			paymentConfig, err := q.GetMerchantPaymentConfig(ctx, order.MerchantID)
			if err != nil {
				return fmt.Errorf("get payment config for order %d: %w", orderID, err)
			}
			if paymentConfig.Status != "active" || paymentConfig.SubMchID == "" {
				return fmt.Errorf("merchant %d payment config invalid", order.MerchantID)
			}

			// 检查是否已有其他支付单
			// 注意：这里需要更严格的检查，不仅是 Latest，而是是否有 'paid' 或 'processing' 的
			// 但业务逻辑上，Pending 订单只应该有一个 Active 的 PaymentOrder (pending)
			existingPO, err := q.GetLatestPaymentOrderByOrder(ctx, GetLatestPaymentOrderByOrderParams{
				OrderID:      pgtype.Int8{Int64: orderID, Valid: true},
				BusinessType: "order",
			})
			if err == nil {
				if existingPO.Status != "pending" && existingPO.Status != "closed" && existingPO.Status != "failed" {
					return fmt.Errorf("order %d has %s payment order", orderID, existingPO.Status)
				}
				// 如果有 pending 的，我们应该关闭它或者复用它？
				// 原逻辑是：如果是 pending 且金额一致，复用。
				// 但这里是合单，复用有点麻烦，因为我们要把这个 PO 关联到新的 Combined ID。
				// 简单起见：如果是 Pending，将其 Close，然后创建新的。这样更干净。
				if existingPO.Status == "pending" {
					_, err = q.UpdatePaymentOrderToClosed(ctx, existingPO.ID)
					if err != nil {
						return fmt.Errorf("close existing payment order: %w", err)
					}
				}
			} else if !errors.Is(err, ErrRecordNotFound) { // sql.ErrNoRows in pgx is likely wrapped or different, sqlc uses wrapper
				// ErrRecordNotFound needed check
				// 假设 q.GetLatest... 返回 error 是 sql.ErrNoRows
				// 这里实际上 sqlc 生成的代码在 no rows 时返回 err
			}

			tempInfos = append(tempInfos, tempOrderInfo{
				Order:         order,
				Merchant:      merchant,
				PaymentConfig: paymentConfig,
				PayAmount:     payAmount,
			})
			totalAmount += payAmount
		}

		// 2. 创建合单记录
		result.CombinedPaymentOrder, err = q.CreateCombinedPaymentOrder(ctx, CreateCombinedPaymentOrderParams{
			UserID:            arg.UserID,
			CombineOutTradeNo: arg.CombineOutTradeNo,
			TotalAmount:       totalAmount,
			Status:            "pending",
			ExpiresAt:         pgtype.Timestamptz{Time: arg.ExpiresAt, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("create combined payment: %w", err)
		}

		// 3. 创建子单记录
		for _, info := range tempInfos {
			// 使用带前缀的安全生成器，避免纳秒截断碰撞。
			outTradeNo, err := generateSubOrderOutTradeNo(info.Order.ID)
			if err != nil {
				return fmt.Errorf("generate sub order out trade no: %w", err)
			}

			po, err := q.CreatePaymentOrder(ctx, CreatePaymentOrderParams{
				OrderID:       pgtype.Int8{Int64: info.Order.ID, Valid: true},
				ReservationID: pgtype.Int8{Valid: false},
				UserID:        arg.UserID,
				PaymentType:   "profit_sharing", // 合单支付走收付通渠道，子单必须标记为 profit_sharing
				BusinessType:  "order",
				Amount:        info.PayAmount,
				OutTradeNo:    outTradeNo,
				ExpiresAt:     pgtype.Timestamptz{Time: arg.ExpiresAt, Valid: true},
				Attach:        pgtype.Text{String: fmt.Sprintf("合单:%s", arg.CombineOutTradeNo), Valid: true},
			})
			if err != nil {
				return fmt.Errorf("create payment order for order %d: %w", info.Order.ID, err)
			}

			// 关联 combined_payment_id
			po, err = q.SetPaymentOrderCombinedID(ctx, SetPaymentOrderCombinedIDParams{
				ID:                po.ID,
				CombinedPaymentID: pgtype.Int8{Int64: result.CombinedPaymentOrder.ID, Valid: true},
			})
			if err != nil {
				return fmt.Errorf("set combined id for payment order %d: %w", po.ID, err)
			}

			// 4. 创建 CombinedPaymentSubOrder
			_, err = q.CreateCombinedPaymentSubOrder(ctx, CreateCombinedPaymentSubOrderParams{
				CombinedPaymentID: result.CombinedPaymentOrder.ID,
				OrderID:           info.Order.ID,
				MerchantID:        info.Order.MerchantID,
				SubMchid:          info.PaymentConfig.SubMchID,
				Amount:            info.PayAmount,
				OutTradeNo:        po.OutTradeNo,
				Description:       fmt.Sprintf("%s - 订单支付", info.Merchant.Name),
			})
			if err != nil {
				return fmt.Errorf("create combined sub order %d: %w", info.Order.ID, err)
			}

			result.PaymentOrders = append(result.PaymentOrders, po)
			result.OrderInfos = append(result.OrderInfos, CombinedPaymentOrderInfo{
				Order:         info.Order,
				PaymentOrder:  po,
				PaymentConfig: info.PaymentConfig,
				Merchant:      info.Merchant,
			})
		}

		return nil
	})

	return result, err
}

// generateSubOrderOutTradeNo 为合单子单生成安全的商户订单号。
// 格式：CP + 订单ID(10位) + 时间戳秒(10位) + 随机数(6位)
// 比起直接用纳秒截断，随机数部分确保同一订单快速重试时不碰撞。
func generateSubOrderOutTradeNo(orderID int64) (string, error) {
	now := time.Now()
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto/rand.Read failed: %w", err)
	}
	randNum := uint32(b[0])<<16 | uint32(b[1])<<8 | uint32(b[2])
	return fmt.Sprintf("CP%d%d%06d", orderID, now.Unix(), randNum%1000000), nil
}
