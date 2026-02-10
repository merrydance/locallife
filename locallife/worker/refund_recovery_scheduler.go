package worker

import (
	"context"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

const (
	refundRecoveryCron       = "*/5 * * * *" // 每5分钟运行一次
	refundRecoveryBatchLimit = int32(50)
)

// RefundRecoveryScheduler 扫描已取消但未退款的订单并触发退款任务
type RefundRecoveryScheduler struct {
	cron        *cron.Cron
	store       db.Store
	distributor TaskDistributor
}

// NewRefundRecoveryScheduler 创建新的退款恢复调度器
func NewRefundRecoveryScheduler(store db.Store, distributor TaskDistributor) *RefundRecoveryScheduler {
	return &RefundRecoveryScheduler{
		cron:        cron.New(),
		store:       store,
		distributor: distributor,
	}
}

// Start 启动调度器
func (s *RefundRecoveryScheduler) Start() error {
	_, err := s.cron.AddFunc(refundRecoveryCron, func() {
		s.runOnce()
	})
	if err != nil {
		return err
	}

	s.cron.Start()
	log.Info().Msg("refund recovery scheduler started (every 5 minutes)")

	// 启动时立即运行一次
	go s.runOnce()
	return nil
}

// Stop 停止调度器
func (s *RefundRecoveryScheduler) Stop() {
	s.cron.Stop()
	log.Info().Msg("refund recovery scheduler stopped")
}

func (s *RefundRecoveryScheduler) runOnce() {
	if s.distributor == nil {
		log.Warn().Msg("task distributor not configured, skip refund recovery")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// 查找最近7天内，订单已取消但支付状态仍为paid的记录
	paymentOrders, err := s.store.ListPaidUnrefundedPaymentOrders(ctx, refundRecoveryBatchLimit)
	if err != nil {
		log.Error().Err(err).Msg("list paid unrefunded payment orders failed")
		return
	}

	if len(paymentOrders) == 0 {
		return
	}

	log.Info().Int("count", len(paymentOrders)).Msg("found unrefunded payment orders, triggering recovery")

	for _, po := range paymentOrders {
		// 再次确认订单状态（防止查询延时导致的脏数据）
		// ListPaidUnrefundedPaymentOrders 已经包含了 JOIN 检查
		// 但为了保险起见，获取 Order 再检查一次
		if !po.OrderID.Valid {
			continue
		}

		order, err := s.store.GetOrder(ctx, po.OrderID.Int64)
		if err != nil {
			log.Error().Err(err).Int64("order_id", po.OrderID.Int64).Msg("get order failed during recovery")
			continue
		}

		// 只有订单确实取消了，才退款
		if order.Status != "cancelled" {
			continue
		}

		// 触发退款任务
		// 注意：CancelReason 可能在 Order 中，也可能在 StatusLog 中。
		// 这里简单使用 "系统自动退款补偿" 或 Order.CancelReason
		reason := "系统自动退款补偿"
		if order.CancelReason.Valid && order.CancelReason.String != "" {
			reason = order.CancelReason.String
		}

		err = s.distributor.DistributeTaskProcessRefund(ctx, &PayloadProcessRefund{
			PaymentOrderID: po.ID,
			OrderID:        order.ID,
			RefundAmount:   po.Amount,
			Reason:         reason,
		})

		if err != nil {
			log.Error().Err(err).
				Int64("payment_order_id", po.ID).
				Int64("order_id", order.ID).
				Msg("enqueue refund recovery task failed")
		} else {
			log.Info().
				Int64("payment_order_id", po.ID).
				Int64("order_id", order.ID).
				Msg("refund recovery task enqueued")
		}
	}
}
