package scheduler

import (
	"context"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"

	db "github.com/merrydance/locallife/db/sqlc"
)

const (
	// OrderPaymentTimeoutMinutes 订单支付超时时间（分钟）
	OrderPaymentTimeoutMinutes = 30
)

// OrderTimeoutScheduler 订单超时清理调度器
type OrderTimeoutScheduler struct {
	cron  *cron.Cron
	store db.Store
}

// NewOrderTimeoutScheduler 创建订单超时清理调度器
func NewOrderTimeoutScheduler(store db.Store) *OrderTimeoutScheduler {
	return &OrderTimeoutScheduler{
		cron:  cron.New(cron.WithSeconds()),
		store: store,
	}
}

// Start 启动调度器
func (s *OrderTimeoutScheduler) Start() error {
	// 每5分钟执行一次订单超时清理
	_, err := s.cron.AddFunc("0 */5 * * * *", s.cleanupTimeoutOrders)
	if err != nil {
		return err
	}

	s.cron.Start()
	log.Info().Msg("order timeout scheduler started")
	return nil
}

// Stop 停止调度器
func (s *OrderTimeoutScheduler) Stop() {
	s.cron.Stop()
	log.Info().Msg("order timeout scheduler stopped")
}

// cleanupTimeoutOrders 清理超时未支付的订单
func (s *OrderTimeoutScheduler) cleanupTimeoutOrders() {
	ctx := context.Background()

	// 计算超时时间点：当前时间 - 超时时间
	timeoutBefore := time.Now().Add(-OrderPaymentTimeoutMinutes * time.Minute)

	// 批量获取超时的 pending 订单
	orders, err := s.store.ListPendingOrdersBefore(ctx, db.ListPendingOrdersBeforeParams{
		Status:    db.OrderStatusPending,
		CreatedAt: timeoutBefore,
		Limit:     100, // 每次最多处理100条
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to list timeout pending orders")
		return
	}

	if len(orders) == 0 {
		return
	}

	log.Info().Int("count", len(orders)).Msg("found timeout pending orders to cancel")

	// 逐个取消订单
	cancelledCount := 0
	for _, order := range orders {
		_, err := s.store.CancelOrderTx(ctx, db.CancelOrderTxParams{
			OrderID:      order.ID,
			OldStatus:    order.Status,
			CancelReason: "支付超时自动取消",
			OperatorID:   order.UserID,
			OperatorType: "system",
		})
		if err != nil {
			log.Warn().Err(err).
				Int64("order_id", order.ID).
				Str("order_no", order.OrderNo).
				Msg("failed to cancel timeout order")
			continue
		}
		cancelledCount++
	}

	log.Info().
		Int("cancelled", cancelledCount).
		Int("total", len(orders)).
		Msg("completed timeout order cleanup")
}
