package worker

import (
	"context"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"

	db "github.com/merrydance/locallife/db/sqlc"
)

const (
	profitSharingRecoveryCron         = "*/10 * * * *"
	profitSharingRecoveryBatchLimit   = int32(200)
	profitSharingRecoveryMinAge       = 10 * time.Minute
	profitSharingReturnStuckThreshold = 15 * time.Minute // processing 超过此时长视为卡死
)

// ProfitSharingRecoveryScheduler scans failed/stale profit sharing orders and re-enqueues processing.
type ProfitSharingRecoveryScheduler struct {
	cron        *cron.Cron
	wg          sync.WaitGroup
	stopCtx     context.Context
	stopCancel  context.CancelFunc
	runMu       sync.Mutex
	store       db.Store
	distributor TaskDistributor
}

// NewProfitSharingRecoveryScheduler creates a new scheduler for profit sharing recovery.
func NewProfitSharingRecoveryScheduler(store db.Store, distributor TaskDistributor) *ProfitSharingRecoveryScheduler {
	stopCtx, stopCancel := context.WithCancel(context.Background())
	return &ProfitSharingRecoveryScheduler{
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

// Start starts the recovery scheduler.
func (s *ProfitSharingRecoveryScheduler) Start() error {
	_, err := s.cron.AddFunc(profitSharingRecoveryCron, func() {
		s.runOnce(s.stopCtx)
	})
	if err != nil {
		return err
	}

	s.cron.Start()
	log.Info().Msg("profit sharing recovery scheduler started (every 10 minutes)")

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runOnce(s.stopCtx)
	}()
	return nil
}

// Stop stops the scheduler.
func (s *ProfitSharingRecoveryScheduler) Stop() {
	s.stopCancel()
	s.cron.Stop()
	s.wg.Wait()
	log.Info().Msg("profit sharing recovery scheduler stopped")
}

// RunOnce triggers a single scan cycle.
// Useful for integration tests and manual runs.
func (s *ProfitSharingRecoveryScheduler) RunOnce() {
	s.runOnce(context.Background())
}

func (s *ProfitSharingRecoveryScheduler) runOnce(ctx context.Context) {
	if !s.runMu.TryLock() {
		log.Warn().Msg("profit sharing recovery already running, skipping concurrent execution")
		return
	}
	defer s.runMu.Unlock()

	if s.distributor == nil {
		log.Warn().Msg("task distributor not configured, skip profit sharing recovery")
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	cutoff := time.Now().Add(-profitSharingRecoveryMinAge)
	orders, err := s.store.ListProfitSharingOrdersForRetry(ctx, db.ListProfitSharingOrdersForRetryParams{
		CreatedAt: cutoff,
		Limit:     profitSharingRecoveryBatchLimit,
	})
	if err != nil {
		log.Error().Err(err).Msg("list profit sharing orders for retry failed")
		return
	}

	for _, order := range orders {
		paymentOrder, err := s.store.GetPaymentOrder(ctx, order.PaymentOrderID)
		if err != nil {
			log.Error().Err(err).
				Int64("profit_sharing_order_id", order.ID).
				Int64("payment_order_id", order.PaymentOrderID).
				Msg("get payment order for profit sharing retry failed")
			continue
		}

		retryPayload, ok := buildProfitSharingPayloadFromPaymentOrder(paymentOrder)
		if !ok {
			log.Warn().
				Int64("profit_sharing_order_id", order.ID).
				Int64("payment_order_id", order.PaymentOrderID).
				Msg("payment order missing order_id and reservation_id, skip profit sharing retry")
			continue
		}

		err = s.distributor.DistributeTaskProcessProfitSharing(
			ctx,
			&retryPayload,
			asynq.MaxRetry(5),
			asynq.Queue(QueueCritical),
		)
		if err != nil {
			log.Error().Err(err).
				Int64("profit_sharing_order_id", order.ID).
				Int64("payment_order_id", order.PaymentOrderID).
				Msg("enqueue profit sharing recovery task failed")
		}
	}

	// 新增：扫描已完成但未创建分账单的订单（P1-046 修复）
	missingOrders, err := s.store.ListCompletedOrdersMissingProfitSharing(ctx, profitSharingRecoveryBatchLimit)
	if err != nil {
		log.Error().Err(err).Msg("list completed orders missing profit sharing failed")
		return
	}

	for _, row := range missingOrders {
		if !row.OrderID.Valid {
			continue
		}

		log.Warn().
			Int64("payment_order_id", row.PaymentOrderID).
			Int64("order_id", row.OrderID.Int64).
			Msg("found completed order with missing profit sharing, triggering recovery")

		err := s.distributor.DistributeTaskProcessProfitSharing(
			ctx,
			&ProfitSharingPayload{
				PaymentOrderID: row.PaymentOrderID,
				OrderID:        row.OrderID.Int64,
			},
			asynq.MaxRetry(5),
			asynq.Queue(QueueCritical),
		)
		if err != nil {
			log.Error().Err(err).
				Int64("payment_order_id", row.PaymentOrderID).
				Int64("order_id", row.OrderID.Int64).
				Msg("enqueue missing profit sharing recovery task failed")
		}
	}

	// 补偿扫描：找出卡在 processing 超过阈值的分账回退单，重新入队结果轮询任务。
	// 兜底路径：入队时若 Redis 故障，DB 状态已是 processing 但轮询任务丢失，会导致永久卡死。
	stuckCutoff := time.Now().Add(-profitSharingReturnStuckThreshold)
	stuckReturns, err := s.store.ListStuckProcessingProfitSharingReturns(ctx, db.ListStuckProcessingProfitSharingReturnsParams{
		UpdatedAt: stuckCutoff,
		Limit:     profitSharingRecoveryBatchLimit,
	})
	if err != nil {
		log.Error().Err(err).Msg("list stuck processing profit sharing returns failed")
		return
	}

	for _, r := range stuckReturns {
		log.Warn().
			Int64("profit_sharing_return_id", r.ID).
			Int64("refund_order_id", r.RefundOrderID).
			Time("updated_at", r.UpdatedAt).
			Msg("found stuck processing profit sharing return, re-enqueuing result poll")

		if err := s.distributor.DistributeTaskProcessProfitSharingReturnResult(
			ctx,
			&ProfitSharingReturnResultPayload{
				ProfitSharingReturnID: r.ID,
				OutReturnNo:           r.OutReturnNo,
				OutOrderNo:            r.OutOrderNo,
				SubMchID:              r.SubMchid,
				RefundOrderID:         r.RefundOrderID,
				RetryCount:            0,
			},
			asynq.ProcessIn(0),
			asynq.Queue(QueueCritical),
			asynq.MaxRetry(5),
		); err != nil {
			log.Error().Err(err).
				Int64("profit_sharing_return_id", r.ID).
				Msg("re-enqueue stuck profit sharing return result task failed")
		}
	}
}
