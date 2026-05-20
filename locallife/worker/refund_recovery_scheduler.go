package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/baofu/aggregatepay"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/wechat"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
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
	errRefundRecoveryOrdinaryClientMissing  = errors.New("refund recovery ordinary service provider client not configured")
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
	paymentClient   wechat.DirectPaymentClientInterface
	ecommerceClient wechat.EcommerceClientInterface
	ordinaryClient  OrdinaryServiceProviderWorkerClient
	baofuClient     aggregatepay.Client
	baofuConfig     BaofuProfitSharingWorkerConfig
	paymentFactSvc  *logic.PaymentFactService
}

// NewRefundRecoveryScheduler 创建新的退款恢复调度器
func NewRefundRecoveryScheduler(
	store db.Store,
	distributor TaskDistributor,
	paymentClient wechat.DirectPaymentClientInterface,
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
		paymentFactSvc:  logic.NewPaymentFactService(store),
	}
}

func (s *RefundRecoveryScheduler) SetOrdinaryServiceProviderClient(client OrdinaryServiceProviderWorkerClient) {
	s.ordinaryClient = client
}

func (s *RefundRecoveryScheduler) SetBaofuAggregateClient(client aggregatepay.Client, config BaofuProfitSharingWorkerConfig) {
	s.baofuClient = client
	s.baofuConfig = config.normalized()
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

		if capabilityErr := s.validateStatusRecoveryCapability(paymentOrder); capabilityErr != nil {
			log.Warn().Err(capabilityErr).
				Int64("refund_order_id", refundOrder.ID).
				Int64("payment_order_id", refundOrder.PaymentOrderID).
				Str("out_refund_no", refundOrder.OutRefundNo).
				Str("payment_type", paymentOrder.PaymentType).
				Str("payment_channel", paymentOrder.PaymentChannel).
				Msg("skip stuck refund status recovery because required client is unavailable")
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

		if normalizeRefundTerminalStatusForPaymentOrder(paymentOrder, refundStatus) == db.ExternalPaymentTerminalStatusProcessing {
			log.Info().
				Int64("refund_order_id", refundOrder.ID).
				Str("out_refund_no", refundOrder.OutRefundNo).
				Msg("stuck refund query still reports processing; keep waiting")
			continue
		}

		application, err := s.recordRiderDepositDirectRefundQueryFact(ctx, paymentOrder, refundOrder, refundStatus, refundID)
		if err != nil {
			log.Error().Err(err).
				Int64("refund_order_id", refundOrder.ID).
				Str("out_refund_no", refundOrder.OutRefundNo).
				Str("refund_status", refundStatus).
				Msg("record rider deposit direct refund query fact failed")
			continue
		}
		if application != nil {
			enqueueRiderDepositRefundPaymentFactApplication(ctx, s.distributor, application)
			log.Info().
				Int64("refund_order_id", refundOrder.ID).
				Str("out_refund_no", refundOrder.OutRefundNo).
				Str("refund_status", refundStatus).
				Int64("payment_fact_application_id", application.ID).
				Msg("stuck rider deposit refund fact application enqueued")
			continue
		}

		application, err = s.recordReservationEcommerceRefundQueryFact(ctx, paymentOrder, refundOrder, refundStatus, refundID)
		if err != nil {
			log.Error().Err(err).
				Int64("refund_order_id", refundOrder.ID).
				Str("out_refund_no", refundOrder.OutRefundNo).
				Str("refund_status", refundStatus).
				Msg("record reservation ecommerce refund query fact failed")
			continue
		}
		if application != nil {
			enqueueReservationRefundPaymentFactApplication(ctx, s.distributor, application)
			log.Info().
				Int64("refund_order_id", refundOrder.ID).
				Str("out_refund_no", refundOrder.OutRefundNo).
				Str("refund_status", refundStatus).
				Int64("payment_fact_application_id", application.ID).
				Msg("stuck reservation refund fact application enqueued")
			continue
		}

		application, err = s.recordOrderEcommerceRefundQueryFact(ctx, paymentOrder, refundOrder, refundStatus, refundID)
		if err != nil {
			log.Error().Err(err).
				Int64("refund_order_id", refundOrder.ID).
				Str("out_refund_no", refundOrder.OutRefundNo).
				Str("refund_status", refundStatus).
				Msg("record order ecommerce refund query fact failed")
			continue
		}
		if application != nil {
			enqueueOrderRefundPaymentFactApplication(ctx, s.distributor, application)
			log.Info().
				Int64("refund_order_id", refundOrder.ID).
				Str("out_refund_no", refundOrder.OutRefundNo).
				Str("refund_status", refundStatus).
				Int64("payment_fact_application_id", application.ID).
				Msg("stuck order refund fact application enqueued")
			continue
		}

		s.persistUnsupportedRefundRecoveryFactAlert(ctx, paymentOrder, refundOrder, refundStatus, refundID)
		log.Info().
			Int64("refund_order_id", refundOrder.ID).
			Int64("payment_order_id", paymentOrder.ID).
			Str("out_refund_no", refundOrder.OutRefundNo).
			Str("refund_status", refundStatus).
			Str("payment_channel", paymentOrder.PaymentChannel).
			Str("business_type", paymentOrder.BusinessType).
			Msg("stuck refund query reached terminal status but no fact application target is modeled")
	}
}

func (s *RefundRecoveryScheduler) persistUnsupportedRefundRecoveryFactAlert(ctx context.Context, paymentOrder db.PaymentOrder, refundOrder db.RefundOrder, refundStatus, refundID string) {
	err := SavePlatformAlertEvent(
		ctx,
		s.store,
		string(AlertTypeRefundFailed),
		string(AlertLevelCritical),
		"退款恢复查询缺少事实应用目标",
		fmt.Sprintf("退款单 %s 查询支付通道已进入 %s，但当前业务类型 %s/%s 没有可用的退款 fact application target，系统已停止 legacy result worker fallback，请人工核对并补建 owner terminalizer。", refundOrder.OutRefundNo, refundStatus, paymentOrder.PaymentChannel, paymentOrder.BusinessType),
		refundOrder.ID,
		"refund_order",
		map[string]any{
			"refund_order_id":   refundOrder.ID,
			"payment_order_id":  paymentOrder.ID,
			"out_refund_no":     refundOrder.OutRefundNo,
			"refund_id":         refundID,
			"refund_status":     refundStatus,
			"payment_channel":   paymentOrder.PaymentChannel,
			"payment_type":      paymentOrder.PaymentType,
			"business_type":     paymentOrder.BusinessType,
			"order_id":          paymentOrderInt8ForAlert(paymentOrder.OrderID),
			"reservation_id":    paymentOrderInt8ForAlert(paymentOrder.ReservationID),
			"requires_modeling": true,
		},
		time.Now(),
	)
	if err != nil {
		log.Error().Err(err).
			Int64("refund_order_id", refundOrder.ID).
			Str("out_refund_no", refundOrder.OutRefundNo).
			Msg("persist unsupported refund recovery fact alert failed")
	}
}

func (s *RefundRecoveryScheduler) validateStatusRecoveryCapability(paymentOrder db.PaymentOrder) error {
	if requiresEcommerceRefund(paymentOrder) && !paymentOrderUsesMainBusinessRefundChannel(paymentOrder) {
		return mainBusinessRefundChannelDriftError(paymentOrder, "query refund status")
	}

	if paymentOrder.PaymentChannel == db.PaymentChannelBaofuAggregate {
		if s.baofuClient == nil {
			return errors.New("refund recovery baofu aggregate client not configured")
		}
		cfg := s.baofuConfig.normalized()
		if cfg.CollectMerchantID == "" || cfg.CollectTerminalID == "" {
			return errors.New("refund recovery baofu collect merchant config not configured")
		}
		return nil
	}
	if paymentOrderUsesEcommerceChannel(paymentOrder) {
		if s.ecommerceClient == nil {
			return errRefundRecoveryEcommerceClientMissing
		}
		return nil
	}
	if db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder) {
		if s.ordinaryClient == nil {
			return errRefundRecoveryOrdinaryClientMissing
		}
		return nil
	}

	if s.paymentClient == nil {
		return errRefundRecoveryPaymentClientMissing
	}

	return nil
}

func (s *RefundRecoveryScheduler) queryRefundStatus(ctx context.Context, paymentOrder db.PaymentOrder, refundOrder db.RefundOrder) (string, string, error) {
	if paymentOrder.PaymentChannel == db.PaymentChannelBaofuAggregate {
		cfg := s.baofuConfig.normalized()
		resp, err := s.baofuClient.QueryRefund(ctx, aggregatecontracts.RefundQueryRequest{
			MerchantID: cfg.CollectMerchantID,
			TerminalID: cfg.CollectTerminalID,
			OutTradeNo: refundOrder.OutRefundNo,
		})
		if err != nil {
			return "", "", err
		}
		if resp == nil {
			return "", "", fmt.Errorf("baofu refund query returned empty result")
		}
		refundID := strings.TrimSpace(resp.TradeNo)
		if refundID == "" {
			refundID = strings.TrimSpace(resp.OutTradeNo)
		}
		return baofuRefundResultUpstreamState(resp), refundID, nil
	}
	if paymentOrderUsesEcommerceChannel(paymentOrder) {

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

	if db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder) {
		subMchID, err := s.resolveSubMchID(ctx, paymentOrder)
		if err != nil {
			return "", "", err
		}
		resp, err := s.ordinaryClient.QueryRefund(ctx, ospcontracts.RefundQueryRequest{
			SubMchID:    subMchID,
			OutRefundNo: refundOrder.OutRefundNo,
		})
		if err != nil {
			return "", "", err
		}
		return string(resp.Status), resp.RefundID, nil
	}

	resp, err := s.paymentClient.QueryRefund(ctx, refundOrder.OutRefundNo)
	if err != nil {
		return "", "", err
	}
	return resp.Status, resp.RefundID, nil
}

func (s *RefundRecoveryScheduler) recordRiderDepositDirectRefundQueryFact(ctx context.Context, paymentOrder db.PaymentOrder, refundOrder db.RefundOrder, refundStatus, refundID string) (*db.ExternalPaymentFactApplication, error) {
	if s.paymentFactSvc == nil || paymentOrder.BusinessType != db.ExternalPaymentBusinessOwnerRiderDeposit || paymentOrderUsesEcommerceChannel(paymentOrder) {
		return nil, nil
	}
	amount := refundOrder.RefundAmount
	result, err := s.paymentFactSvc.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelDirect,
		Capability:           db.ExternalPaymentCapabilityDirectRefund,
		FactSource:           db.ExternalPaymentFactSourceQuery,
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalObjectKey:    refundOrder.OutRefundNo,
		ExternalSecondaryKey: paymentFactStringPtr(refundID),
		BusinessOwner:        paymentFactStringPtr(db.ExternalPaymentBusinessOwnerRiderDeposit),
		BusinessObjectType:   paymentFactStringPtr("refund_order"),
		BusinessObjectID:     paymentFactInt64Ptr(refundOrder.ID),
		UpstreamState:        refundStatus,
		TerminalStatus:       logic.NormalizeDirectRefundTerminalStatus(refundStatus),
		Amount:               &amount,
		Currency:             "CNY",
		RawResource:          directRefundQueryFactResource(refundOrder.OutRefundNo, refundID, refundStatus),
		DedupeKey:            directRefundQueryFactDedupeKey(refundOrder.OutRefundNo, refundStatus),
		Application: &logic.ExternalPaymentFactApplicationTarget{
			Consumer:           riderDepositRefundFactConsumerDomain,
			BusinessObjectType: riderDepositRefundFactBusinessObjectOrder,
			BusinessObjectID:   refundOrder.ID,
		},
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}

func (s *RefundRecoveryScheduler) recordReservationEcommerceRefundQueryFact(ctx context.Context, paymentOrder db.PaymentOrder, refundOrder db.RefundOrder, refundStatus, refundID string) (*db.ExternalPaymentFactApplication, error) {
	if s.paymentFactSvc == nil || !paymentOrderUsesMainBusinessRefundChannel(paymentOrder) || !isReservationRefundPayment(paymentOrder) {
		return nil, nil
	}
	provider, channel, capability := mainBusinessRefundFactProviderChannelCapability(paymentOrder.PaymentChannel)
	amount := refundOrder.RefundAmount
	terminalStatus := normalizeRefundTerminalStatusForPaymentOrder(paymentOrder, refundStatus)
	result, err := s.paymentFactSvc.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:             provider,
		Channel:              channel,
		Capability:           capability,
		FactSource:           db.ExternalPaymentFactSourceQuery,
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalObjectKey:    refundOrder.OutRefundNo,
		ExternalSecondaryKey: paymentFactStringPtr(refundID),
		BusinessOwner:        paymentFactStringPtr(db.ExternalPaymentBusinessOwnerReservation),
		BusinessObjectType:   paymentFactStringPtr(reservationRefundFactBusinessObjectOrder),
		BusinessObjectID:     paymentFactInt64Ptr(refundOrder.ID),
		UpstreamState:        refundStatus,
		TerminalStatus:       terminalStatus,
		Amount:               &amount,
		Currency:             "CNY",
		RawResource:          mainBusinessRefundQueryFactResource(refundOrder.OutRefundNo, refundID, refundStatus),
		DedupeKey:            mainBusinessRefundQueryFactDedupeKey(provider, channel, refundOrder.OutRefundNo, terminalStatus),
		Application: &logic.ExternalPaymentFactApplicationTarget{
			Consumer:           reservationRefundFactConsumerDomain,
			BusinessObjectType: reservationRefundFactBusinessObjectOrder,
			BusinessObjectID:   refundOrder.ID,
		},
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}

func (s *RefundRecoveryScheduler) recordOrderEcommerceRefundQueryFact(ctx context.Context, paymentOrder db.PaymentOrder, refundOrder db.RefundOrder, refundStatus, refundID string) (*db.ExternalPaymentFactApplication, error) {
	if s.paymentFactSvc == nil || !paymentOrderUsesMainBusinessRefundChannel(paymentOrder) || !paymentOrder.OrderID.Valid || paymentOrder.ReservationID.Valid || paymentOrder.BusinessType != db.ExternalPaymentBusinessOwnerOrder {
		return nil, nil
	}
	provider, channel, capability := mainBusinessRefundFactProviderChannelCapability(paymentOrder.PaymentChannel)
	amount := refundOrder.RefundAmount
	terminalStatus := normalizeRefundTerminalStatusForPaymentOrder(paymentOrder, refundStatus)
	result, err := s.paymentFactSvc.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:             provider,
		Channel:              channel,
		Capability:           capability,
		FactSource:           db.ExternalPaymentFactSourceQuery,
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalObjectKey:    refundOrder.OutRefundNo,
		ExternalSecondaryKey: paymentFactStringPtr(refundID),
		BusinessOwner:        paymentFactStringPtr(db.ExternalPaymentBusinessOwnerOrder),
		BusinessObjectType:   paymentFactStringPtr(orderRefundFactBusinessObjectOrder),
		BusinessObjectID:     paymentFactInt64Ptr(refundOrder.ID),
		UpstreamState:        refundStatus,
		TerminalStatus:       terminalStatus,
		Amount:               &amount,
		Currency:             "CNY",
		RawResource:          mainBusinessRefundQueryFactResource(refundOrder.OutRefundNo, refundID, refundStatus),
		DedupeKey:            mainBusinessRefundQueryFactDedupeKey(provider, channel, refundOrder.OutRefundNo, terminalStatus),
		Application: &logic.ExternalPaymentFactApplicationTarget{
			Consumer:           orderRefundFactConsumerDomain,
			BusinessObjectType: orderRefundFactBusinessObjectOrder,
			BusinessObjectID:   refundOrder.ID,
		},
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}

func directRefundQueryFactDedupeKey(outRefundNo, refundStatus string) string {
	return "wechat:query:direct:refund:" + outRefundNo + ":" + logic.NormalizeDirectRefundTerminalStatus(refundStatus)
}

func ecommerceRefundQueryFactDedupeKey(outRefundNo, refundStatus string) string {
	return mainBusinessRefundQueryFactDedupeKey(db.ExternalPaymentProviderWechat, db.PaymentChannelEcommerce, outRefundNo, logic.NormalizeEcommerceRefundTerminalStatus(refundStatus))
}

func mainBusinessRefundQueryFactDedupeKey(provider, channel, outRefundNo, terminalStatus string) string {
	if provider == db.ExternalPaymentProviderBaofu {
		return "baofu:query:refund:" + outRefundNo + ":" + terminalStatus
	}
	return "wechat:query:" + channel + ":refund:" + outRefundNo + ":" + terminalStatus
}

func directRefundQueryFactResource(outRefundNo, refundID, refundStatus string) []byte {
	raw, err := json.Marshal(map[string]any{
		"out_refund_no": outRefundNo,
		"refund_id":     refundID,
		"refund_status": refundStatus,
	})
	if err != nil {
		log.Warn().Err(err).Str("out_refund_no", outRefundNo).Msg("marshal direct refund query fact resource failed")
		return nil
	}
	return raw
}

func ecommerceRefundQueryFactResource(outRefundNo, refundID, refundStatus string) []byte {
	return mainBusinessRefundQueryFactResource(outRefundNo, refundID, refundStatus)
}

func mainBusinessRefundQueryFactResource(outRefundNo, refundID, refundStatus string) []byte {
	raw, err := json.Marshal(map[string]any{
		"out_refund_no": outRefundNo,
		"refund_id":     refundID,
		"refund_status": refundStatus,
	})
	if err != nil {
		log.Warn().Err(err).Str("out_refund_no", outRefundNo).Msg("marshal ecommerce refund query fact resource failed")
		return nil
	}
	return raw
}

func mainBusinessRefundFactProviderChannelCapability(channel string) (string, string, string) {
	if channel == db.PaymentChannelBaofuAggregate {
		return db.ExternalPaymentProviderBaofu, db.PaymentChannelBaofuAggregate, db.ExternalPaymentCapabilityBaofuRefund
	}
	if channel == db.PaymentChannelOrdinaryServiceProvider {
		return db.ExternalPaymentProviderWechat, db.PaymentChannelOrdinaryServiceProvider, db.ExternalPaymentCapabilityPartnerRefund
	}
	return db.ExternalPaymentProviderWechat, db.PaymentChannelEcommerce, db.ExternalPaymentCapabilityEcommerceRefund
}

func normalizeRefundTerminalStatusForPaymentOrder(paymentOrder db.PaymentOrder, refundStatus string) string {
	if paymentOrder.PaymentChannel == db.PaymentChannelBaofuAggregate {
		if terminalStatus := normalizeBaofuRefundResultCodeTerminalStatus(refundStatus); terminalStatus != "" {
			return terminalStatus
		}
		return aggregatecontracts.NormalizeRefundTerminalStatus(refundStatus)
	}
	if paymentOrder.PaymentChannel == db.PaymentChannelDirect {
		return logic.NormalizeDirectRefundTerminalStatus(refundStatus)
	}
	return logic.NormalizeEcommerceRefundTerminalStatus(refundStatus)
}

func baofuRefundResultUpstreamState(result *aggregatecontracts.RefundResult) string {
	if result == nil {
		return ""
	}
	if refundState := strings.TrimSpace(result.RefundState); refundState != "" {
		return refundState
	}
	return strings.TrimSpace(result.ResultCode)
}

func normalizeBaofuRefundResultCodeTerminalStatus(resultCode string) string {
	switch strings.TrimSpace(resultCode) {
	case aggregatecontracts.BusinessResultCodeSuccess:
		return db.ExternalPaymentTerminalStatusSuccess
	case aggregatecontracts.BusinessResultCodeFail:
		return db.ExternalPaymentTerminalStatusFailed
	default:
		return ""
	}
}

func paymentFactStringPtr(value string) *string {
	return &value
}

func paymentFactInt64Ptr(value int64) *int64 {
	return &value
}

func paymentOrderInt8ForAlert(value pgtype.Int8) any {
	if !value.Valid {
		return nil
	}
	return value.Int64
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
