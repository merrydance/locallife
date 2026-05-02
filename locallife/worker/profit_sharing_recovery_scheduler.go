package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

const (
	profitSharingRecoveryCron         = "*/10 * * * *"
	profitSharingRecoveryBatchLimit   = int32(200)
	profitSharingRecoveryMinAge       = 10 * time.Minute
	profitSharingReturnStuckThreshold = 15 * time.Minute // processing 超过此时长视为卡死
)

// ProfitSharingRecoveryScheduler scans failed/stale profit sharing orders and re-enqueues processing.
type ProfitSharingRecoveryScheduler struct {
	cron             *cron.Cron
	wg               sync.WaitGroup
	stopCtx          context.Context
	stopCancel       context.CancelFunc
	runMu            sync.Mutex
	store            db.Store
	distributor      TaskDistributor
	ecommerceClient  wechat.EcommerceClientInterface
	ordinarySPClient OrdinaryServiceProviderWorkerClient
}

// NewProfitSharingRecoveryScheduler creates a new scheduler for profit sharing recovery.
func NewProfitSharingRecoveryScheduler(store db.Store, distributor TaskDistributor, ecommerceClient wechat.EcommerceClientInterface) *ProfitSharingRecoveryScheduler {
	stopCtx, stopCancel := context.WithCancel(context.Background())
	return &ProfitSharingRecoveryScheduler{
		cron: cron.New(cron.WithChain(
			cron.SkipIfStillRunning(cron.DefaultLogger),
			cron.Recover(cron.DefaultLogger),
		)),
		stopCtx:         stopCtx,
		stopCancel:      stopCancel,
		store:           store,
		distributor:     distributor,
		ecommerceClient: ecommerceClient,
	}
}

func (s *ProfitSharingRecoveryScheduler) SetOrdinaryServiceProviderClient(client OrdinaryServiceProviderWorkerClient) {
	s.ordinarySPClient = client
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

	// 补偿扫描：找出卡在 processing 超过阈值的分账回退单，直接查询微信结果并写入 fact application。
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
		paymentOrder, err := s.store.GetPaymentOrder(ctx, r.PaymentOrderID)
		if err != nil {
			log.Error().Err(err).
				Int64("profit_sharing_return_id", r.ID).
				Int64("payment_order_id", r.PaymentOrderID).
				Msg("resolve stuck profit sharing return payment order failed")
			continue
		}

		log.Warn().
			Int64("profit_sharing_return_id", r.ID).
			Int64("refund_order_id", r.RefundOrderID).
			Time("updated_at", r.UpdatedAt).
			Msg("found stuck processing profit sharing return, querying upstream result")

		resp, err := s.queryWechatProfitSharingReturn(ctx, paymentOrder, r)
		if err != nil {
			log.Error().Err(err).
				Int64("profit_sharing_return_id", r.ID).
				Str("out_return_no", r.OutReturnNo).
				Msg("query stuck profit sharing return result failed")
			continue
		}

		s.handleStuckProfitSharingReturnQueryResult(ctx, r, paymentOrder, resp)
	}
}

func (s *ProfitSharingRecoveryScheduler) queryWechatProfitSharingReturn(ctx context.Context, paymentOrder db.PaymentOrder, returnRecord db.ProfitSharingReturn) (*wechatcontracts.ProfitSharingReturnResponse, error) {
	if db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder) {
		if s.ordinarySPClient == nil {
			return nil, fmt.Errorf("ordinary service provider client not configured for profit sharing return recovery query")
		}
		resp, err := s.ordinarySPClient.QueryProfitSharingReturn(ctx, ospcontracts.ProfitSharingReturnQueryRequest{
			SubMchID:    returnRecord.SubMchid,
			OutReturnNo: returnRecord.OutReturnNo,
			OutOrderNo:  returnRecord.OutOrderNo,
		})
		if err != nil {
			return nil, err
		}
		return ordinaryProfitSharingReturnResponse(resp, ""), nil
	}
	if s.ecommerceClient == nil {
		return nil, fmt.Errorf("ecommerce client not configured for profit sharing return recovery query")
	}
	return s.ecommerceClient.QueryProfitSharingReturn(ctx, returnRecord.SubMchid, returnRecord.OutReturnNo, returnRecord.OutOrderNo)
}

func (s *ProfitSharingRecoveryScheduler) handleStuckProfitSharingReturnQueryResult(ctx context.Context, returnRecord db.ProfitSharingReturn, paymentOrder db.PaymentOrder, resp *wechatcontracts.ProfitSharingReturnResponse) {
	if resp == nil {
		log.Warn().
			Int64("profit_sharing_return_id", returnRecord.ID).
			Str("out_return_no", returnRecord.OutReturnNo).
			Msg("profit sharing return query response is empty")
		return
	}

	switch resp.Result {
	case wechatcontracts.ProfitSharingReturnResultProcessing:
		if _, err := s.store.UpdateProfitSharingReturnToProcessing(ctx, db.UpdateProfitSharingReturnToProcessingParams{
			ID:       returnRecord.ID,
			ReturnID: pgtype.Text{String: resp.ReturnID, Valid: resp.ReturnID != ""},
		}); err != nil {
			log.Error().Err(err).
				Int64("profit_sharing_return_id", returnRecord.ID).
				Str("out_return_no", returnRecord.OutReturnNo).
				Str("return_id", resp.ReturnID).
				Msg("refresh processing profit sharing return after recovery query failed")
			return
		}
		log.Info().
			Int64("profit_sharing_return_id", returnRecord.ID).
			Str("out_return_no", returnRecord.OutReturnNo).
			Str("return_id", resp.ReturnID).
			Msg("stuck profit sharing return still processing; keep waiting")
		return
	case wechatcontracts.ProfitSharingReturnResultSuccess, wechatcontracts.ProfitSharingReturnResultFailed:
		application, err := recordProfitSharingReturnQueryFact(ctx, s.store, paymentOrder.PaymentChannel, returnRecord, resp)
		if err != nil {
			log.Error().Err(err).
				Int64("profit_sharing_return_id", returnRecord.ID).
				Str("out_return_no", returnRecord.OutReturnNo).
				Str("result", resp.Result).
				Msg("record stuck profit sharing return query fact failed")
			return
		}
		enqueueProfitSharingReturnPaymentFactApplication(ctx, s.distributor, application)
		log.Info().
			Int64("profit_sharing_return_id", returnRecord.ID).
			Int64("refund_order_id", returnRecord.RefundOrderID).
			Str("out_return_no", returnRecord.OutReturnNo).
			Str("result", resp.Result).
			Int64("payment_fact_application_id", applicationIDForLog(application)).
			Msg("stuck profit sharing return fact application enqueued")
	default:
		log.Error().
			Int64("profit_sharing_return_id", returnRecord.ID).
			Str("out_return_no", returnRecord.OutReturnNo).
			Str("result", resp.Result).
			Msg("profit sharing return query returned unsupported result during recovery")
	}
}

func applicationIDForLog(application *db.ExternalPaymentFactApplication) int64 {
	if application == nil {
		return 0
	}
	return application.ID
}
