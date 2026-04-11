package worker

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

const (
	refundRecoveryCron                  = "*/5 * * * *" // 每5分钟运行一次
	refundRecoveryBatchLimit            = int32(50)
	refundRecoveryStuckProcessingMinAge = 15 * time.Minute
)

var (
	errRefundRecoveryPaymentClientMissing   = errors.New("refund recovery payment client not configured")
	errRefundRecoveryEcommerceClientMissing = errors.New("refund recovery ecommerce client not configured")
	errRefundRecoverySubMchIDMissing        = errors.New("refund recovery sub mchid missing")
	errRefundRecoveryMerchantUnresolved     = errors.New("refund recovery merchant could not be resolved")
)

// RefundRecoveryScheduler 扫描已取消但未退款的订单并触发退款任务
type RefundRecoveryScheduler struct {
	cron            *cron.Cron
	wg              sync.WaitGroup
	stopCtx         context.Context
	stopCancel      context.CancelFunc
	runMu           sync.Mutex
	store           db.Store
	distributor     TaskDistributor
	paymentClient   wechat.PaymentClientInterface
	ecommerceClient wechat.EcommerceClientInterface
}

// NewRefundRecoveryScheduler 创建新的退款恢复调度器
func NewRefundRecoveryScheduler(
	store db.Store,
	distributor TaskDistributor,
	paymentClient wechat.PaymentClientInterface,
	ecommerceClient wechat.EcommerceClientInterface,
) *RefundRecoveryScheduler {
	stopCtx, stopCancel := context.WithCancel(context.Background())
	return &RefundRecoveryScheduler{
		cron: cron.New(cron.WithChain(
			cron.SkipIfStillRunning(cron.DefaultLogger),
			cron.Recover(cron.DefaultLogger),
		)),
		stopCtx:         stopCtx,
		stopCancel:      stopCancel,
		store:           store,
		distributor:     distributor,
		paymentClient:   paymentClient,
		ecommerceClient: ecommerceClient,
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

// RunOnce 触发一次扫描，便于测试和人工补偿。
func (s *RefundRecoveryScheduler) RunOnce() {
	s.runOnce(context.Background())
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

	s.recoverCancelledOrderRefunds(ctx)
	s.recoverCancelledReservationRefunds(ctx)
	s.recoverPendingReservationRefunds(ctx)
	s.recoverStuckProcessingRefunds(ctx)
}

func (s *RefundRecoveryScheduler) recoverCancelledOrderRefunds(ctx context.Context) {

	// 查找最近7天内，订单已取消但支付状态仍为paid的记录
	paymentOrders, err := s.store.ListPaidUnrefundedPaymentOrders(ctx, refundRecoveryBatchLimit)
	if err != nil {
		log.Error().Err(err).Msg("list paid unrefunded payment orders failed")
		return
	}

	if len(paymentOrders) > 0 {
		log.Info().Int("count", len(paymentOrders)).Msg("found unrefunded payment orders, triggering recovery")
	}

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

func (s *RefundRecoveryScheduler) recoverCancelledReservationRefunds(ctx context.Context) {

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

func (s *RefundRecoveryScheduler) recoverPendingReservationRefunds(ctx context.Context) {

	// ==================== 预定退款单 pending 恢复 ====================
	pendingReservationRefundOrders, err := s.store.ListPendingReservationRefundOrdersForRecovery(ctx, db.ListPendingReservationRefundOrdersForRecoveryParams{
		CreatedBefore: time.Now().Add(-1 * time.Minute),
		Limit:         refundRecoveryBatchLimit,
	})
	if err != nil {
		log.Error().Err(err).Msg("list pending reservation refund orders for recovery failed")
		return
	}

	for _, refundOrder := range pendingReservationRefundOrders {
		if !refundOrder.ReservationID.Valid {
			continue
		}

		reason := "系统自动退款补偿（预订退款）"
		if refundOrder.RefundReason.Valid && refundOrder.RefundReason.String != "" {
			reason = refundOrder.RefundReason.String
		}

		err = s.distributor.DistributeTaskProcessRefund(ctx, &PayloadProcessRefund{
			PaymentOrderID: refundOrder.PaymentOrderID,
			ReservationID:  refundOrder.ReservationID.Int64,
			RefundAmount:   refundOrder.RefundAmount,
			Reason:         reason,
			OutRefundNo:    refundOrder.OutRefundNo,
		})
		if err != nil {
			log.Error().Err(err).
				Int64("refund_order_id", refundOrder.ID).
				Int64("payment_order_id", refundOrder.PaymentOrderID).
				Str("business_type", refundOrder.BusinessType).
				Msg("enqueue pending reservation refund recovery task failed")
			continue
		}

		log.Info().
			Int64("refund_order_id", refundOrder.ID).
			Int64("payment_order_id", refundOrder.PaymentOrderID).
			Str("business_type", refundOrder.BusinessType).
			Msg("pending reservation refund recovery task enqueued")
	}
}

func (s *RefundRecoveryScheduler) recoverStuckProcessingRefunds(ctx context.Context) {
	stuckOrders, err := s.store.ListStuckProcessingRefundOrders(ctx, db.ListStuckProcessingRefundOrdersParams{
		CreatedBefore: time.Now().Add(-refundRecoveryStuckProcessingMinAge),
		Limit:         refundRecoveryBatchLimit,
	})
	if err != nil {
		log.Error().Err(err).Msg("list stuck processing refund orders failed")
		return
	}

	for _, row := range stuckOrders {
		refundOrder, err := s.store.GetRefundOrder(ctx, row.ID)
		if err != nil {
			log.Error().Err(err).Int64("refund_order_id", row.ID).Msg("get refund order for recovery failed")
			continue
		}
		if refundOrder.Status != "processing" {
			continue
		}

		paymentOrder, err := s.store.GetPaymentOrder(ctx, refundOrder.PaymentOrderID)
		if err != nil {
			log.Error().Err(err).
				Int64("refund_order_id", refundOrder.ID).
				Int64("payment_order_id", refundOrder.PaymentOrderID).
				Msg("get payment order for stuck refund recovery failed")
			continue
		}

		refundStatus, refundID, err := s.queryRefundStatus(ctx, paymentOrder, refundOrder)
		if err != nil {
			log.Error().Err(err).
				Int64("refund_order_id", refundOrder.ID).
				Str("out_refund_no", refundOrder.OutRefundNo).
				Str("payment_type", paymentOrder.PaymentType).
				Msg("query stuck processing refund status failed")
			continue
		}

		if refundStatus == wechat.RefundStatusProcessing {
			log.Info().
				Int64("refund_order_id", refundOrder.ID).
				Str("out_refund_no", refundOrder.OutRefundNo).
				Msg("stuck refund query still reports processing; keep waiting")
			continue
		}

		err = s.distributor.DistributeTaskProcessRefundResult(
			ctx,
			&RefundResultPayload{
				OutRefundNo:  refundOrder.OutRefundNo,
				RefundStatus: refundStatus,
				RefundID:     refundID,
			},
			asynq.MaxRetry(5),
			asynq.Queue(QueueCritical),
		)
		if err != nil {
			log.Error().Err(err).
				Int64("refund_order_id", refundOrder.ID).
				Str("out_refund_no", refundOrder.OutRefundNo).
				Str("refund_status", refundStatus).
				Msg("enqueue stuck refund result recovery task failed")
			continue
		}

		log.Info().
			Int64("refund_order_id", refundOrder.ID).
			Str("out_refund_no", refundOrder.OutRefundNo).
			Str("refund_status", refundStatus).
			Msg("stuck refund result recovery task enqueued")
	}
}

func (s *RefundRecoveryScheduler) queryRefundStatus(ctx context.Context, paymentOrder db.PaymentOrder, refundOrder db.RefundOrder) (string, string, error) {
	if paymentOrder.PaymentType == "profit_sharing" {
		if s.ecommerceClient == nil {
			return "", "", errRefundRecoveryEcommerceClientMissing
		}

		subMchID, err := s.resolveSubMchID(ctx, paymentOrder)
		if err != nil {
			return "", "", err
		}

		resp, err := s.ecommerceClient.QueryEcommerceRefund(ctx, subMchID, refundOrder.OutRefundNo)
		if err != nil {
			return "", "", err
		}
		return resp.Status, resp.RefundID, nil
	}

	if s.paymentClient == nil {
		return "", "", errRefundRecoveryPaymentClientMissing
	}

	resp, err := s.paymentClient.QueryRefund(ctx, refundOrder.OutRefundNo)
	if err != nil {
		return "", "", err
	}
	return resp.Status, resp.RefundID, nil
}

func (s *RefundRecoveryScheduler) resolveSubMchID(ctx context.Context, paymentOrder db.PaymentOrder) (string, error) {
	if paymentOrder.OrderID.Valid {
		order, err := s.store.GetOrder(ctx, paymentOrder.OrderID.Int64)
		if err != nil {
			return "", err
		}
		cfg, err := s.store.GetMerchantPaymentConfig(ctx, order.MerchantID)
		if err != nil {
			return "", err
		}
		if cfg.SubMchID == "" {
			return "", errRefundRecoverySubMchIDMissing
		}
		return cfg.SubMchID, nil
	}

	if paymentOrder.ReservationID.Valid {
		reservation, err := s.store.GetTableReservation(ctx, paymentOrder.ReservationID.Int64)
		if err != nil {
			return "", err
		}
		cfg, err := s.store.GetMerchantPaymentConfig(ctx, reservation.MerchantID)
		if err != nil {
			return "", err
		}
		if cfg.SubMchID == "" {
			return "", errRefundRecoverySubMchIDMissing
		}
		return cfg.SubMchID, nil
	}

	return "", errRefundRecoveryMerchantUnresolved
}
