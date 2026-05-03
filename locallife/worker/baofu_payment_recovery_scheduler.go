package worker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

const (
	baofuPaymentRecoveryCron       = "*/5 * * * *"
	baofuPaymentRecoveryBatchLimit = int32(200)
	baofuShareTaskUniqueWindow     = 30 * time.Second
	baofuPlatformRateBps           = int32(200)
	baofuOperatorRateBps           = int32(300)
)

type BaofuPaymentRecoveryScheduler struct {
	cron        *cron.Cron
	wg          sync.WaitGroup
	stopCtx     context.Context
	stopCancel  context.CancelFunc
	runMu       sync.Mutex
	store       db.Store
	distributor TaskDistributor
}

func NewBaofuPaymentRecoveryScheduler(store db.Store, distributor TaskDistributor) *BaofuPaymentRecoveryScheduler {
	stopCtx, stopCancel := context.WithCancel(context.Background())
	return &BaofuPaymentRecoveryScheduler{
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

func (s *BaofuPaymentRecoveryScheduler) Start() error {
	_, err := s.cron.AddFunc(baofuPaymentRecoveryCron, func() {
		s.runOnce(s.stopCtx)
	})
	if err != nil {
		return err
	}
	s.cron.Start()
	log.Info().Msg("baofu payment recovery scheduler started (every 5 minutes)")

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runOnce(s.stopCtx)
	}()
	return nil
}

func (s *BaofuPaymentRecoveryScheduler) Stop() {
	s.stopCancel()
	s.cron.Stop()
	s.wg.Wait()
	log.Info().Msg("baofu payment recovery scheduler stopped")
}

func (s *BaofuPaymentRecoveryScheduler) RunOnce() {
	s.runOnce(context.Background())
}

func (s *BaofuPaymentRecoveryScheduler) runOnce(ctx context.Context) {
	if !s.runMu.TryLock() {
		log.Warn().Msg("baofu payment recovery already running, skipping concurrent execution")
		return
	}
	defer s.runMu.Unlock()

	if s.distributor == nil {
		log.Warn().Msg("task distributor not configured, skip baofu payment recovery")
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	if err := s.createReadyProfitSharingOrders(ctx); err != nil {
		log.Error().Err(err).Msg("baofu ready profit sharing scan failed")
	}
}

func (s *BaofuPaymentRecoveryScheduler) createReadyProfitSharingOrders(ctx context.Context) error {
	rows, err := s.store.ListBaofuOrdersReadyForProfitSharing(ctx, db.ListBaofuOrdersReadyForProfitSharingParams{
		RefundClosedBefore: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		Limit:              baofuPaymentRecoveryBatchLimit,
	})
	if err != nil {
		return fmt.Errorf("list baofu orders ready for profit sharing: %w", err)
	}

	service := logic.NewBaofuProfitSharingService(s.store)
	for _, row := range rows {
		if !row.OrderID.Valid {
			log.Warn().
				Int64("payment_order_id", row.PaymentOrderID).
				Msg("skip baofu profit sharing creation because order id is missing")
			continue
		}
		created, err := s.createBaofuProfitSharingOrder(ctx, service, row.PaymentOrderID, row.OrderID.Int64)
		if err != nil {
			log.Error().Err(err).
				Int64("payment_order_id", row.PaymentOrderID).
				Int64("order_id", row.OrderID.Int64).
				Msg("create baofu profit sharing order failed")
			continue
		}
		if err := s.distributor.DistributeTaskProcessBaofuProfitSharing(ctx, &BaofuProfitSharingPayload{
			ProfitSharingOrderID: created.ProfitSharingOrder.ID,
		}, asynq.MaxRetry(5), asynq.Queue(QueueCritical), asynq.Unique(baofuShareTaskUniqueWindow)); err != nil {
			log.Error().Err(err).
				Int64("profit_sharing_order_id", created.ProfitSharingOrder.ID).
				Int64("payment_order_id", row.PaymentOrderID).
				Msg("enqueue baofu profit sharing command failed")
		}
	}
	return nil
}

func (s *BaofuPaymentRecoveryScheduler) createBaofuProfitSharingOrder(ctx context.Context, service *logic.BaofuProfitSharingService, paymentOrderID int64, orderID int64) (db.CreateBaofuProfitSharingOrderTxResult, error) {
	paymentOrder, err := s.store.GetPaymentOrder(ctx, paymentOrderID)
	if err != nil {
		return db.CreateBaofuProfitSharingOrderTxResult{}, fmt.Errorf("get payment order: %w", err)
	}
	if paymentOrder.PaymentChannel != db.PaymentChannelBaofuAggregate || !paymentOrder.RequiresProfitSharing || paymentOrder.Status != "paid" {
		return db.CreateBaofuProfitSharingOrderTxResult{}, fmt.Errorf("payment order %d is not ready for baofu profit sharing", paymentOrder.ID)
	}
	order, err := s.store.GetOrder(ctx, orderID)
	if err != nil {
		return db.CreateBaofuProfitSharingOrderTxResult{}, fmt.Errorf("get order: %w", err)
	}
	if order.Status != db.OrderStatusCompleted {
		return db.CreateBaofuProfitSharingOrderTxResult{}, fmt.Errorf("order %d is not completed for baofu profit sharing", order.ID)
	}

	merchant, err := s.store.GetMerchant(ctx, order.MerchantID)
	if err != nil {
		return db.CreateBaofuProfitSharingOrderTxResult{}, fmt.Errorf("get merchant: %w", err)
	}
	operatorID := int64(0)
	if merchant.RegionID > 0 {
		operator, err := s.store.GetActiveOperatorByRegion(ctx, merchant.RegionID)
		if err != nil && !errors.Is(err, db.ErrRecordNotFound) {
			return db.CreateBaofuProfitSharingOrderTxResult{}, fmt.Errorf("get active operator by region: %w", err)
		}
		if err == nil {
			operatorID = operator.ID
		}
	}

	riderID, err := s.resolveBaofuProfitSharingRider(ctx, order)
	if err != nil {
		return db.CreateBaofuProfitSharingOrderTxResult{}, err
	}

	return service.CreatePendingOrder(ctx, logic.BaofuProfitSharingOrderInput{
		PaymentOrderID:  paymentOrder.ID,
		MerchantID:      order.MerchantID,
		RiderID:         riderID,
		OperatorID:      operatorID,
		PlatformOwnerID: 0,
		OrderSource:     order.OrderType,
		TotalAmountFen:  paymentOrder.Amount,
		DeliveryFeeFen:  order.DeliveryFee,
		PlatformRateBps: baofuPlatformRateBps,
		OperatorRateBps: baofuOperatorRateBps,
		OutOrderNo:      fmt.Sprintf("BFPS%dO%d", paymentOrder.ID, order.ID),
	})
}

func (s *BaofuPaymentRecoveryScheduler) resolveBaofuProfitSharingRider(ctx context.Context, order db.Order) (int64, error) {
	if order.DeliveryFee <= 0 {
		return 0, nil
	}
	delivery, err := s.store.GetDeliveryByOrderID(ctx, order.ID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return 0, fmt.Errorf("baofu profit sharing requires completed delivery rider for order %d", order.ID)
		}
		return 0, fmt.Errorf("get delivery by order id: %w", err)
	}
	if !delivery.RiderID.Valid || delivery.RiderID.Int64 <= 0 {
		return 0, fmt.Errorf("baofu profit sharing requires rider for order %d", order.ID)
	}
	return delivery.RiderID.Int64, nil
}
