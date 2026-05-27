package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/rs/zerolog/log"
)

const TaskProcessBaofuProfitSharing = "baofu:process_profit_sharing"

type BaofuProfitSharingWorkerConfig struct {
	CollectMerchantID string
	CollectTerminalID string
	ShareNotifyURL    string
	RefundNotifyURL   string
}

type BaofuProfitSharingPayload struct {
	ProfitSharingOrderID int64 `json:"profit_sharing_order_id"`
}

func (distributor *RedisTaskDistributor) DistributeTaskProcessBaofuProfitSharing(ctx context.Context, payload *BaofuProfitSharingPayload, opts ...asynq.Option) error {
	if payload == nil || payload.ProfitSharingOrderID <= 0 {
		return fmt.Errorf("baofu profit sharing order id is required")
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal baofu profit sharing payload: %w", err)
	}
	task := asynq.NewTask(TaskProcessBaofuProfitSharing, jsonPayload, opts...)
	info, err := distributor.enqueueTask(ctx, task)
	if err != nil {
		return fmt.Errorf("enqueue baofu profit sharing task: %w", err)
	}
	log.Info().
		Str("type", task.Type()).
		Str("queue", info.Queue).
		Int64("profit_sharing_order_id", payload.ProfitSharingOrderID).
		Msg("enqueued baofu profit sharing task")
	return nil
}

type baofuProfitSharingSnapshot struct {
	ShareableAmount int64                                `json:"shareable_amount"`
	Receivers       []baofuProfitSharingSnapshotReceiver `json:"receivers"`
}

type baofuProfitSharingSnapshotReceiver struct {
	SharingMerID string `json:"sharing_mer_id"`
	Amount       int64  `json:"amount"`
}

func (processor *RedisTaskProcessor) ProcessTaskBaofuProfitSharing(ctx context.Context, task *asynq.Task) error {
	var payload BaofuProfitSharingPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal baofu profit sharing payload: %w", asynq.SkipRetry)
	}
	if payload.ProfitSharingOrderID <= 0 {
		return fmt.Errorf("baofu profit sharing order id is required: %w", asynq.SkipRetry)
	}
	if processor.baofuAggregateClient == nil {
		return fmt.Errorf("baofu aggregate client not configured for profit sharing: %w", asynq.SkipRetry)
	}
	cfg := processor.baofuProfitSharingConfig.normalized()
	if cfg.CollectMerchantID == "" || cfg.CollectTerminalID == "" {
		return fmt.Errorf("baofu collect merchant config not configured for profit sharing: %w", asynq.SkipRetry)
	}

	profitSharingOrder, err := processor.store.GetProfitSharingOrder(ctx, payload.ProfitSharingOrderID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return fmt.Errorf("baofu profit sharing order not found: %w", asynq.SkipRetry)
		}
		return fmt.Errorf("get baofu profit sharing order: %w", err)
	}
	if profitSharingOrder.Provider != db.ExternalPaymentProviderBaofu || profitSharingOrder.Channel != db.PaymentChannelBaofuAggregate {
		return fmt.Errorf("profit sharing order %d is not baofu aggregate: %w", profitSharingOrder.ID, asynq.SkipRetry)
	}
	if profitSharingOrder.Status == db.ProfitSharingOrderStatusFinished {
		return nil
	}
	if profitSharingOrder.Status != db.ProfitSharingOrderStatusPending && profitSharingOrder.Status != db.ProfitSharingOrderStatusFailed {
		return fmt.Errorf("baofu profit sharing order %d status %q cannot create share command: %w", profitSharingOrder.ID, profitSharingOrder.Status, asynq.SkipRetry)
	}

	prepared, err := processor.store.PrepareBaofuProfitSharingCommandTx(ctx, db.PrepareBaofuProfitSharingCommandTxParams{
		ProfitSharingOrderID: profitSharingOrder.ID,
	})
	if err != nil {
		if _, ok := db.IsRefundRequestError(err); ok {
			return fmt.Errorf("prepare baofu profit sharing command: %v: %w", err, asynq.SkipRetry)
		}
		return fmt.Errorf("prepare baofu profit sharing command: %w", err)
	}
	profitSharingOrder = prepared.ProfitSharingOrder
	paymentOrder := prepared.PaymentOrder

	req, err := buildBaofuShareAfterPayRequest(cfg, paymentOrder, profitSharingOrder)
	if err != nil {
		_ = processor.markBaofuProfitSharingCommandFailed(ctx, profitSharingOrder.ID)
		return err
	}
	if err := req.Validate(); err != nil {
		_ = processor.markBaofuProfitSharingCommandFailed(ctx, profitSharingOrder.ID)
		return fmt.Errorf("build baofu share request: %w", err)
	}
	if _, err := processor.store.CreateExternalPaymentCommand(ctx, db.CreateExternalPaymentCommandParams{
		Provider:             db.ExternalPaymentProviderBaofu,
		Channel:              db.PaymentChannelBaofuAggregate,
		Capability:           db.ExternalPaymentCapabilityBaofuProfitSharing,
		CommandType:          db.ExternalPaymentCommandTypeCreateProfitSharing,
		BusinessOwner:        db.ExternalPaymentBusinessOwnerProfitSharing,
		BusinessObjectType:   pgtype.Text{String: profitSharingFactBusinessObjectOrder, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: profitSharingOrder.ID, Valid: true},
		ExternalObjectType:   db.ExternalPaymentObjectProfitSharing,
		ExternalObjectKey:    profitSharingOrder.OutOrderNo,
		ExternalSecondaryKey: pgtype.Text{String: paymentOrder.OutTradeNo, Valid: strings.TrimSpace(paymentOrder.OutTradeNo) != ""},
		CommandStatus:        db.ExternalPaymentCommandStatusSubmitted,
		SubmittedAt:          time.Now().UTC(),
		ResponseSnapshot:     baofuProfitSharingCommandSnapshot(profitSharingOrder),
	}); err != nil {
		_ = processor.markBaofuProfitSharingCommandFailed(ctx, profitSharingOrder.ID)
		return fmt.Errorf("create baofu profit sharing command: %w", err)
	}

	result, err := processor.baofuAggregateClient.CreateProfitSharing(ctx, req)
	if err != nil {
		return fmt.Errorf("create baofu profit sharing: %w", err)
	}
	if result == nil {
		return fmt.Errorf("create baofu profit sharing returned empty result")
	}
	upstreamShareID := strings.TrimSpace(result.TradeNo)
	responseOutTradeNo := strings.TrimSpace(result.OutTradeNo)
	if upstreamShareID == "" && responseOutTradeNo == "" {
		return fmt.Errorf("create baofu profit sharing missing upstream share reference")
	}
	if _, err := processor.store.UpdateProfitSharingOrderSharingID(ctx, db.UpdateProfitSharingOrderSharingIDParams{
		ID:             profitSharingOrder.ID,
		SharingOrderID: pgtype.Text{String: upstreamShareID, Valid: upstreamShareID != ""},
	}); err != nil {
		return fmt.Errorf("record baofu profit sharing upstream reference: %w", err)
	}

	log.Info().
		Int64("profit_sharing_order_id", profitSharingOrder.ID).
		Int64("payment_order_id", paymentOrder.ID).
		Str("out_order_no", profitSharingOrder.OutOrderNo).
		Str("baofu_share_state", strings.TrimSpace(result.TxnState)).
		Msg("baofu profit sharing command accepted")
	return nil
}

func (processor *RedisTaskProcessor) markBaofuProfitSharingCommandFailed(ctx context.Context, profitSharingOrderID int64) error {
	if _, err := processor.store.UpdateProfitSharingOrderToFailed(ctx, profitSharingOrderID); err != nil {
		log.Error().Err(err).
			Int64("profit_sharing_order_id", profitSharingOrderID).
			Msg("mark baofu profit sharing order failed after command preparation failed")
		return err
	}
	return nil
}

func buildBaofuShareAfterPayRequest(cfg BaofuProfitSharingWorkerConfig, paymentOrder db.PaymentOrder, profitSharingOrder db.ProfitSharingOrder) (aggregatecontracts.ShareAfterPayRequest, error) {
	details, err := baofuSharingDetailsFromSnapshot(profitSharingOrder.SharingDetailSnapshot)
	if err != nil {
		return aggregatecontracts.ShareAfterPayRequest{}, err
	}
	req := aggregatecontracts.ShareAfterPayRequest{
		MerchantID:     cfg.CollectMerchantID,
		TerminalID:     cfg.CollectTerminalID,
		OutTradeNo:     strings.TrimSpace(profitSharingOrder.OutOrderNo),
		TxnTime:        time.Now().UTC().Format("20060102150405"),
		NotifyURL:      strings.TrimSpace(cfg.ShareNotifyURL),
		SharingDetails: details,
	}
	if paymentOrder.TransactionID.Valid && strings.TrimSpace(paymentOrder.TransactionID.String) != "" {
		req.OriginTradeNo = strings.TrimSpace(paymentOrder.TransactionID.String)
	} else {
		req.OriginOutTradeNo = strings.TrimSpace(paymentOrder.OutTradeNo)
	}
	return req, nil
}

func baofuSharingDetailsFromSnapshot(raw []byte) ([]aggregatecontracts.SharingDetail, error) {
	var snapshot baofuProfitSharingSnapshot
	if len(raw) == 0 || !json.Valid(raw) {
		return nil, fmt.Errorf("baofu sharing detail snapshot is invalid")
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return nil, fmt.Errorf("decode baofu sharing detail snapshot: %w", err)
	}
	details := make([]aggregatecontracts.SharingDetail, 0, len(snapshot.Receivers))
	var totalAmount int64
	for _, receiver := range snapshot.Receivers {
		sharingMerID := strings.TrimSpace(receiver.SharingMerID)
		if sharingMerID == "" || receiver.Amount <= 0 {
			return nil, fmt.Errorf("baofu sharing detail snapshot receiver is invalid")
		}
		totalAmount += receiver.Amount
		details = append(details, aggregatecontracts.SharingDetail{SharingMerID: sharingMerID, SharingAmountFen: receiver.Amount})
	}
	if len(details) == 0 {
		return nil, fmt.Errorf("baofu sharing detail snapshot receivers are required")
	}
	if snapshot.ShareableAmount > 0 && totalAmount != snapshot.ShareableAmount {
		return nil, fmt.Errorf("baofu sharing detail snapshot receiver amount mismatch")
	}
	return details, nil
}

func baofuProfitSharingCommandSnapshot(order db.ProfitSharingOrder) []byte {
	raw, err := json.Marshal(map[string]any{
		"provider":                db.ExternalPaymentProviderBaofu,
		"operation":               "share_after_pay",
		"profit_sharing_order_id": order.ID,
		"payment_order_id":        order.PaymentOrderID,
		"out_order_no":            order.OutOrderNo,
		"receiver_count":          baofuSharingSnapshotReceiverCount(order.SharingDetailSnapshot),
	})
	if err != nil {
		return []byte(`{"provider":"baofu","operation":"share_after_pay"}`)
	}
	return raw
}

func baofuSharingSnapshotReceiverCount(raw []byte) int {
	var snapshot baofuProfitSharingSnapshot
	if len(raw) == 0 || json.Unmarshal(raw, &snapshot) != nil {
		return 0
	}
	return len(snapshot.Receivers)
}

func (c BaofuProfitSharingWorkerConfig) normalized() BaofuProfitSharingWorkerConfig {
	c.CollectMerchantID = strings.TrimSpace(c.CollectMerchantID)
	c.CollectTerminalID = strings.TrimSpace(c.CollectTerminalID)
	c.ShareNotifyURL = strings.TrimSpace(c.ShareNotifyURL)
	c.RefundNotifyURL = strings.TrimSpace(c.RefundNotifyURL)
	return c
}
