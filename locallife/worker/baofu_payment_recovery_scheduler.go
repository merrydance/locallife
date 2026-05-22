package worker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/baofu/aggregatepay"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

const (
	baofuPaymentRecoveryCron       = "*/5 * * * *"
	baofuPaymentRecoveryBatchLimit = int32(200)
	baofuShareTaskUniqueWindow     = 30 * time.Second
	baofuShareRecoveryMinAge       = 2 * time.Minute
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
	client      aggregatepay.Client
	shareConfig BaofuProfitSharingWorkerConfig
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

func (s *BaofuPaymentRecoveryScheduler) SetBaofuAggregateClient(client aggregatepay.Client, config BaofuProfitSharingWorkerConfig) {
	s.client = client
	s.shareConfig = config.normalized()
}

func (s *BaofuPaymentRecoveryScheduler) SetBaofuAggregateClientForTest(client aggregatepay.Client, config BaofuProfitSharingWorkerConfig) {
	s.SetBaofuAggregateClient(client, config)
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
	if err := s.queryProcessingProfitSharingOrders(ctx); err != nil {
		log.Error().Err(err).Msg("baofu processing profit sharing recovery scan failed")
	}
	if err := s.queryPendingPaymentOrders(ctx); err != nil {
		log.Error().Err(err).Msg("baofu pending payment recovery scan failed")
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
		var created db.CreateBaofuProfitSharingOrderTxResult
		switch row.BusinessType {
		case db.ExternalPaymentBusinessOwnerOrder:
			if !row.OrderID.Valid {
				log.Warn().
					Int64("payment_order_id", row.PaymentOrderID).
					Msg("skip baofu profit sharing creation because order id is missing")
				continue
			}
			created, err = s.createBaofuProfitSharingOrder(ctx, service, row.PaymentOrderID, row.OrderID.Int64)
		case db.ExternalPaymentBusinessOwnerReservation, reservationPaymentAddonBusinessType:
			if !row.ReservationID.Valid {
				log.Warn().
					Int64("payment_order_id", row.PaymentOrderID).
					Str("business_type", row.BusinessType).
					Msg("skip baofu profit sharing creation because reservation id is missing")
				continue
			}
			created, err = s.createBaofuReservationProfitSharingOrder(ctx, service, row.PaymentOrderID, row.ReservationID.Int64)
		default:
			log.Warn().
				Int64("payment_order_id", row.PaymentOrderID).
				Str("business_type", row.BusinessType).
				Msg("skip baofu profit sharing creation because business type is unsupported")
			continue
		}
		if err != nil {
			log.Error().Err(err).
				Int64("payment_order_id", row.PaymentOrderID).
				Int64("order_id", row.OrderID.Int64).
				Int64("reservation_id", row.ReservationID.Int64).
				Str("business_type", row.BusinessType).
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
	operatorID, err := s.resolveBaofuProfitSharingOperator(ctx, merchant)
	if err != nil {
		return db.CreateBaofuProfitSharingOrderTxResult{}, err
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

func (s *BaofuPaymentRecoveryScheduler) createBaofuReservationProfitSharingOrder(ctx context.Context, service *logic.BaofuProfitSharingService, paymentOrderID int64, reservationID int64) (db.CreateBaofuProfitSharingOrderTxResult, error) {
	paymentOrder, err := s.store.GetPaymentOrder(ctx, paymentOrderID)
	if err != nil {
		return db.CreateBaofuProfitSharingOrderTxResult{}, fmt.Errorf("get payment order: %w", err)
	}
	if paymentOrder.PaymentChannel != db.PaymentChannelBaofuAggregate || !paymentOrder.RequiresProfitSharing || paymentOrder.Status != "paid" {
		return db.CreateBaofuProfitSharingOrderTxResult{}, fmt.Errorf("payment order %d is not ready for baofu reservation profit sharing", paymentOrder.ID)
	}
	if paymentOrder.BusinessType != db.ExternalPaymentBusinessOwnerReservation && paymentOrder.BusinessType != reservationPaymentAddonBusinessType {
		return db.CreateBaofuProfitSharingOrderTxResult{}, fmt.Errorf("payment order %d business type %q is not reservation profit sharing", paymentOrder.ID, paymentOrder.BusinessType)
	}
	if !paymentOrder.ReservationID.Valid || paymentOrder.ReservationID.Int64 != reservationID {
		return db.CreateBaofuProfitSharingOrderTxResult{}, fmt.Errorf("payment order %d reservation id does not match %d", paymentOrder.ID, reservationID)
	}

	reservation, err := s.store.GetTableReservation(ctx, reservationID)
	if err != nil {
		return db.CreateBaofuProfitSharingOrderTxResult{}, fmt.Errorf("get reservation: %w", err)
	}
	if !baofuReservationReadyForProfitSharing(reservation.Status) {
		return db.CreateBaofuProfitSharingOrderTxResult{}, fmt.Errorf("reservation %d status %q is not ready for baofu profit sharing", reservation.ID, reservation.Status)
	}
	merchant, err := s.store.GetMerchant(ctx, reservation.MerchantID)
	if err != nil {
		return db.CreateBaofuProfitSharingOrderTxResult{}, fmt.Errorf("get merchant: %w", err)
	}
	operatorID, err := s.resolveBaofuProfitSharingOperator(ctx, merchant)
	if err != nil {
		return db.CreateBaofuProfitSharingOrderTxResult{}, err
	}

	return service.CreatePendingOrder(ctx, logic.BaofuProfitSharingOrderInput{
		PaymentOrderID:  paymentOrder.ID,
		MerchantID:      reservation.MerchantID,
		OperatorID:      operatorID,
		PlatformOwnerID: 0,
		OrderSource:     db.OrderTypeReservation,
		TotalAmountFen:  paymentOrder.Amount,
		DeliveryFeeFen:  0,
		PlatformRateBps: baofuPlatformRateBps,
		OperatorRateBps: baofuOperatorRateBps,
		OutOrderNo:      fmt.Sprintf("BFPS%dR%d", paymentOrder.ID, reservation.ID),
	})
}

func (s *BaofuPaymentRecoveryScheduler) resolveBaofuProfitSharingOperator(ctx context.Context, merchant db.Merchant) (int64, error) {
	if merchant.RegionID <= 0 {
		return 0, nil
	}
	operator, err := s.store.GetActiveOperatorByRegion(ctx, merchant.RegionID)
	if err != nil && !errors.Is(err, db.ErrRecordNotFound) {
		return 0, fmt.Errorf("get active operator by region: %w", err)
	}
	if err != nil {
		return 0, nil
	}
	return operator.ID, nil
}

func baofuReservationReadyForProfitSharing(status string) bool {
	switch status {
	case "paid", "confirmed", "checked_in", "completed":
		return true
	default:
		return false
	}
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

func (s *BaofuPaymentRecoveryScheduler) queryProcessingProfitSharingOrders(ctx context.Context) error {
	if s.client == nil {
		log.Warn().Msg("baofu aggregate client not configured, skip baofu profit sharing query recovery")
		return nil
	}
	cfg := s.shareConfig.normalized()
	if cfg.CollectMerchantID == "" || cfg.CollectTerminalID == "" {
		log.Warn().Msg("baofu collect merchant config not configured, skip baofu profit sharing query recovery")
		return nil
	}
	factDistributor, ok := s.distributor.(PaymentFactApplicationTaskDistributor)
	if !ok {
		log.Warn().Msg("payment fact application distributor not configured, skip baofu profit sharing fact application enqueue")
		return nil
	}

	orders, err := s.store.ListBaofuProcessingProfitSharingOrdersForRecovery(ctx, db.ListBaofuProcessingProfitSharingOrdersForRecoveryParams{
		CreatedBefore: time.Now().Add(-baofuShareRecoveryMinAge),
		Limit:         baofuPaymentRecoveryBatchLimit,
	})
	if err != nil {
		return fmt.Errorf("list baofu processing profit sharing orders for recovery: %w", err)
	}
	service := logic.NewBaofuProfitSharingService(s.store)
	for _, order := range orders {
		result, err := s.queryBaofuProfitSharing(ctx, cfg, order)
		if err != nil {
			log.Error().Err(err).
				Int64("profit_sharing_order_id", order.ID).
				Str("out_order_no", order.OutOrderNo).
				Msg("query baofu profit sharing failed")
			continue
		}
		recorded, err := service.RecordShareFact(ctx, logic.RecordBaofuShareFactInput{
			ProfitSharingOrder: order,
			Fact:               baofuShareFactFromQueryResult(result, order),
			FactSource:         db.ExternalPaymentFactSourceManualReconciliation,
			ObservedAt:         time.Now().UTC(),
		})
		if err != nil {
			log.Error().Err(err).
				Int64("profit_sharing_order_id", order.ID).
				Str("out_order_no", order.OutOrderNo).
				Msg("record baofu profit sharing query fact failed")
			continue
		}
		if recorded.Application == nil {
			continue
		}
		if err := factDistributor.DistributeTaskProcessPaymentFactApplication(ctx, &PaymentFactApplicationPayload{
			ApplicationID: recorded.Application.ID,
		}, asynq.Queue(QueueCritical), asynq.Unique(paymentFactApplicationTaskUnique)); err != nil {
			log.Error().Err(err).
				Int64("payment_fact_application_id", recorded.Application.ID).
				Int64("profit_sharing_order_id", order.ID).
				Msg("enqueue baofu profit sharing fact application failed")
		}
	}
	return nil
}

func (s *BaofuPaymentRecoveryScheduler) queryBaofuProfitSharing(ctx context.Context, cfg BaofuProfitSharingWorkerConfig, order db.ProfitSharingOrder) (*aggregatecontracts.ShareResult, error) {
	req := aggregatecontracts.ShareQueryRequest{
		MerchantID: cfg.CollectMerchantID,
		TerminalID: cfg.CollectTerminalID,
	}
	if order.SharingOrderID.Valid && strings.TrimSpace(order.SharingOrderID.String) != "" {
		req.TradeNo = strings.TrimSpace(order.SharingOrderID.String)
	} else {
		req.OutTradeNo = strings.TrimSpace(order.OutOrderNo)
	}
	return s.client.QueryProfitSharing(ctx, req)
}

func baofuShareFactFromQueryResult(result *aggregatecontracts.ShareResult, order db.ProfitSharingOrder) aggregatecontracts.ShareFact {
	if result == nil {
		return aggregatecontracts.ShareFact{OutTradeNo: order.OutOrderNo, TransactionState: aggregatecontracts.ShareStateAbnormal}
	}
	outTradeNo := strings.TrimSpace(result.OutTradeNo)
	if outTradeNo == "" {
		outTradeNo = strings.TrimSpace(order.OutOrderNo)
	}
	return aggregatecontracts.ShareFact{
		OutTradeNo:       outTradeNo,
		TradeNo:          strings.TrimSpace(result.TradeNo),
		TransactionState: strings.TrimSpace(result.TxnState),
		SuccessAmountFen: result.SuccessAmountFen,
		ResultCode:       strings.TrimSpace(result.ResultCode),
		Raw:              result.Raw,
	}
}

func (s *BaofuPaymentRecoveryScheduler) queryPendingPaymentOrders(ctx context.Context) error {
	if s.client == nil {
		log.Warn().Msg("baofu aggregate client not configured, skip baofu payment query recovery")
		return nil
	}
	cfg := s.shareConfig.normalized()
	if cfg.CollectMerchantID == "" || cfg.CollectTerminalID == "" {
		log.Warn().Msg("baofu collect merchant config not configured, skip baofu payment query recovery")
		return nil
	}
	factDistributor, ok := s.distributor.(PaymentFactApplicationTaskDistributor)
	if !ok {
		log.Warn().Msg("payment fact application distributor not configured, skip baofu payment fact application enqueue")
		return nil
	}

	orders, err := s.store.ListBaofuPendingPaymentOrdersForRecovery(ctx, db.ListBaofuPendingPaymentOrdersForRecoveryParams{
		CreatedBefore: time.Now().Add(-paymentRecoveryMinAge),
		Limit:         baofuPaymentRecoveryBatchLimit,
	})
	if err != nil {
		return fmt.Errorf("list baofu pending payment orders for recovery: %w", err)
	}
	service := logic.NewBaofuPaymentService(s.store, s.client, logic.BaofuPaymentServiceConfig{
		CollectMerchantID: cfg.CollectMerchantID,
		CollectTerminalID: cfg.CollectTerminalID,
	})
	for _, order := range orders {
		result, err := s.queryBaofuPayment(ctx, cfg, order)
		if err != nil {
			log.Error().Err(err).
				Int64("payment_order_id", order.ID).
				Str("out_trade_no", order.OutTradeNo).
				Msg("query baofu payment failed")
			continue
		}
		recorded, err := service.RecordPaymentFact(ctx, logic.RecordBaofuPaymentFactInput{
			PaymentOrder: order,
			Fact:         baofuPaymentFactFromQueryResult(result, order),
			FactSource:   db.ExternalPaymentFactSourceManualReconciliation,
			ObservedAt:   time.Now().UTC(),
		})
		if err != nil {
			log.Error().Err(err).
				Int64("payment_order_id", order.ID).
				Str("out_trade_no", order.OutTradeNo).
				Msg("record baofu payment query fact failed")
			continue
		}
		if recorded.Application == nil {
			continue
		}
		if err := factDistributor.DistributeTaskProcessPaymentFactApplication(ctx, &PaymentFactApplicationPayload{
			ApplicationID: recorded.Application.ID,
		}, asynq.Queue(QueueCritical), asynq.Unique(paymentFactApplicationTaskUnique)); err != nil {
			log.Error().Err(err).
				Int64("payment_fact_application_id", recorded.Application.ID).
				Int64("payment_order_id", order.ID).
				Msg("enqueue baofu payment fact application failed")
		}
	}
	return nil
}

func (s *BaofuPaymentRecoveryScheduler) queryBaofuPayment(ctx context.Context, cfg BaofuProfitSharingWorkerConfig, order db.PaymentOrder) (*aggregatecontracts.UnifiedOrderResult, error) {
	req := aggregatecontracts.PaymentQueryRequest{
		MerchantID: cfg.CollectMerchantID,
		TerminalID: cfg.CollectTerminalID,
	}
	if order.TransactionID.Valid && strings.TrimSpace(order.TransactionID.String) != "" {
		req.TradeNo = strings.TrimSpace(order.TransactionID.String)
	} else {
		req.OutTradeNo = strings.TrimSpace(order.OutTradeNo)
	}
	return s.client.QueryPayment(ctx, req)
}

func baofuPaymentFactFromQueryResult(result *aggregatecontracts.UnifiedOrderResult, order db.PaymentOrder) aggregatecontracts.PaymentFact {
	if result == nil {
		return aggregatecontracts.PaymentFact{OutTradeNo: order.OutTradeNo, TransactionState: aggregatecontracts.PaymentStateAbnormal}
	}
	outTradeNo := strings.TrimSpace(result.OutTradeNo)
	if outTradeNo == "" {
		outTradeNo = strings.TrimSpace(order.OutTradeNo)
	}
	return aggregatecontracts.PaymentFact{
		OutTradeNo:       outTradeNo,
		TradeNo:          strings.TrimSpace(result.TradeNo),
		TransactionState: strings.TrimSpace(result.TxnState),
		SuccessAmountFen: result.SuccessAmountFen,
		ResultCode:       strings.TrimSpace(result.ResultCode),
		Raw:              result.Raw,
	}
}
