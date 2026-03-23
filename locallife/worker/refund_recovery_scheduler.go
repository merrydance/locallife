package worker

import (
	"context"
	"sync"
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
	wg          sync.WaitGroup
	stopCtx     context.Context
	stopCancel  context.CancelFunc
	runMu       sync.Mutex
	store       db.Store
	distributor TaskDistributor
}

// NewRefundRecoveryScheduler 创建新的退款恢复调度器
func NewRefundRecoveryScheduler(store db.Store, distributor TaskDistributor) *RefundRecoveryScheduler {
	stopCtx, stopCancel := context.WithCancel(context.Background())
	return &RefundRecoveryScheduler{
		cron: cron.New(cron.WithChain(
			cron.SkipIfStillRunning(cron.DefaultLogger),
			cron.Recover(cron.DefaultLogger),
		)),
		stopCtx:     stopCtx,
		stopCancel:  stopCancel,
		store:       store,
		distributor: distributor,
	}
}

// Start 启动调度器
func (s *RefundRecoveryScheduler) Start() error {
	_, err := s.cron.AddFunc(refundRecoveryCron, func() {
		s.runOnce(s.stopCtx)
	})
	if err != nil {
		return err
	}

	s.cron.Start()
	log.Info().Msg("refund recovery scheduler started (every 5 minutes)")

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runOnce(s.stopCtx)
	}()
	return nil
}

// Stop 停止调度器
func (s *RefundRecoveryScheduler) Stop() {
	s.stopCancel()
	s.cron.Stop()
	s.wg.Wait()
	log.Info().Msg("refund recovery scheduler stopped")
}

func (s *RefundRecoveryScheduler) runOnce(ctx context.Context) {
	if !s.runMu.TryLock() {
		log.Warn().Msg("refund recovery already running, skipping concurrent execution")
		return
	}
	defer s.runMu.Unlock()

	if s.distributor == nil {
		log.Warn().Msg("task distributor not configured, skip refund recovery")
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
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

	// ==================== 预定退款恢复 ====================
	reservationPaymentOrders, err := s.store.ListPaidUnrefundedReservationPaymentOrders(ctx, refundRecoveryBatchLimit)
	if err != nil {
		log.Error().Err(err).Msg("list paid unrefunded reservation payment orders failed")
		return
	}

	if len(reservationPaymentOrders) > 0 {
		log.Info().Int("count", len(reservationPaymentOrders)).Msg("found unrefunded reservation payment orders, triggering recovery")
	}

	for _, po := range reservationPaymentOrders {
		if !po.ReservationID.Valid {
			continue
		}

		reservation, err := s.store.GetTableReservation(ctx, po.ReservationID.Int64)
		if err != nil {
			log.Error().Err(err).Int64("reservation_id", po.ReservationID.Int64).Msg("get reservation failed during recovery")
			continue
		}

		if reservation.Status != "cancelled" {
			continue
		}

		err = s.distributor.DistributeTaskProcessRefund(ctx, &PayloadProcessRefund{
			PaymentOrderID: po.ID,
			ReservationID:  reservation.ID,
			RefundAmount:   po.Amount,
			Reason:         "系统自动退款补偿（预定取消）",
		})

		if err != nil {
			log.Error().Err(err).
				Int64("payment_order_id", po.ID).
				Int64("reservation_id", reservation.ID).
				Msg("enqueue reservation refund recovery task failed")
		} else {
			log.Info().
				Int64("payment_order_id", po.ID).
				Int64("reservation_id", reservation.ID).
				Msg("reservation refund recovery task enqueued")
		}
	}
}
