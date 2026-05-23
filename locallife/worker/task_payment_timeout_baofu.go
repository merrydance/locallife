package worker

import (
	"context"
	"fmt"
	"strings"
	"time"

	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/rs/zerolog/log"
)

func (p *RedisTaskProcessor) prepareBaofuPaymentOrderTimeoutClose(ctx context.Context, paymentOrder db.PaymentOrder) (paymentOrderTimeoutRemoteClose, bool, error) {
	if p.baofuAggregateClient == nil {
		err := fmt.Errorf("baofu aggregate client not configured for payment timeout query")
		log.Error().Err(err).
			Int64("payment_order_id", paymentOrder.ID).
			Str("payment_order_no", paymentOrder.OutTradeNo).
			Str("out_trade_no", paymentOrder.OutTradeNo).
			Str("operation", "query_baofu_payment_before_timeout_close").
			Msg("baofu payment timeout query cannot start")
		return paymentOrderTimeoutRemoteClose{}, false, err
	}
	cfg := p.baofuProfitSharingConfig.normalized()
	if cfg.CollectMerchantID == "" || cfg.CollectTerminalID == "" {
		err := fmt.Errorf("baofu collect merchant config not configured for payment timeout query")
		log.Error().Err(err).
			Int64("payment_order_id", paymentOrder.ID).
			Str("payment_order_no", paymentOrder.OutTradeNo).
			Str("out_trade_no", paymentOrder.OutTradeNo).
			Str("operation", "query_baofu_payment_before_timeout_close").
			Msg("baofu payment timeout query cannot start")
		return paymentOrderTimeoutRemoteClose{}, false, err
	}

	queryResp, err := p.queryBaofuPaymentOrderForTimeout(ctx, paymentOrder, cfg)
	if err != nil {
		wrapped := fmt.Errorf("query baofu payment order before timeout close: %w", err)
		log.Error().Err(wrapped).
			Int64("payment_order_id", paymentOrder.ID).
			Str("payment_order_no", paymentOrder.OutTradeNo).
			Str("out_trade_no", paymentOrder.OutTradeNo).
			Str("operation", "query_baofu_payment_before_timeout_close").
			Msg("baofu payment timeout query failed")
		return paymentOrderTimeoutRemoteClose{}, false, wrapped
	}
	if queryResp == nil {
		err := fmt.Errorf("query baofu payment order before timeout close returned nil response")
		log.Error().Err(err).
			Int64("payment_order_id", paymentOrder.ID).
			Str("payment_order_no", paymentOrder.OutTradeNo).
			Str("out_trade_no", paymentOrder.OutTradeNo).
			Str("operation", "query_baofu_payment_before_timeout_close").
			Msg("baofu payment timeout query failed")
		return paymentOrderTimeoutRemoteClose{}, false, err
	}

	stop, err := p.handleBaofuPaymentTimeoutQueryResult(ctx, paymentOrder, queryResp)
	if err != nil || stop {
		return paymentOrderTimeoutRemoteClose{}, stop, err
	}
	terminalStatus := aggregatecontracts.NormalizePaymentTerminalStatus(strings.TrimSpace(queryResp.TxnState))
	return paymentOrderTimeoutRemoteClose{required: terminalStatus == db.ExternalPaymentTerminalStatusProcessing, baofu: true}, false, nil
}

func (p *RedisTaskProcessor) queryBaofuPaymentOrderForTimeout(ctx context.Context, paymentOrder db.PaymentOrder, cfg BaofuProfitSharingWorkerConfig) (*aggregatecontracts.UnifiedOrderResult, error) {
	req := aggregatecontracts.PaymentQueryRequest{
		MerchantID: cfg.CollectMerchantID,
		TerminalID: cfg.CollectTerminalID,
	}
	if paymentOrder.TransactionID.Valid && strings.TrimSpace(paymentOrder.TransactionID.String) != "" {
		req.TradeNo = strings.TrimSpace(paymentOrder.TransactionID.String)
	} else {
		req.OutTradeNo = strings.TrimSpace(paymentOrder.OutTradeNo)
	}
	return p.baofuAggregateClient.QueryPayment(ctx, req)
}

func (p *RedisTaskProcessor) handleBaofuPaymentTimeoutQueryResult(ctx context.Context, paymentOrder db.PaymentOrder, queryResp *aggregatecontracts.UnifiedOrderResult) (bool, error) {
	tradeState := strings.TrimSpace(queryResp.TxnState)
	terminalStatus := aggregatecontracts.NormalizePaymentTerminalStatus(tradeState)
	switch terminalStatus {
	case db.ExternalPaymentTerminalStatusSuccess:
		if queryResp.SuccessAmountFen != paymentOrder.Amount {
			log.Error().
				Int64("payment_order_id", paymentOrder.ID).
				Str("payment_order_no", paymentOrder.OutTradeNo).
				Str("out_trade_no", paymentOrder.OutTradeNo).
				Str("remote_state", tradeState).
				Int64("expected_amount", paymentOrder.Amount).
				Int64("remote_amount", queryResp.SuccessAmountFen).
				Str("operation", "query_baofu_payment_before_timeout_close").
				Msg("baofu payment timeout query success amount mismatch; local close skipped")
			p.publishPaymentTimeoutRemoteAmountMismatchAlert(ctx, paymentOrder, queryResp.SuccessAmountFen, tradeState)
			return true, nil
		}
		transactionID := strings.TrimSpace(queryResp.TradeNo)
		service := logic.NewBaofuPaymentService(p.store, p.baofuAggregateClient, logic.BaofuPaymentServiceConfig{
			CollectMerchantID: p.baofuProfitSharingConfig.CollectMerchantID,
			CollectTerminalID: p.baofuProfitSharingConfig.CollectTerminalID,
		})
		recorded, err := service.RecordPaymentFact(ctx, logic.RecordBaofuPaymentFactInput{
			PaymentOrder: paymentOrder,
			Fact:         baofuPaymentFactFromQueryResult(queryResp, paymentOrder),
			FactSource:   db.ExternalPaymentFactSourceQuery,
			ObservedAt:   time.Now().UTC(),
		})
		if err != nil {
			return false, fmt.Errorf("record baofu payment timeout query fact: %w", err)
		}
		if recorded.Application != nil {
			if err := enqueueOrderPaymentFactApplication(ctx, p.distributor, recorded.Application); err != nil {
				return false, fmt.Errorf("enqueue baofu payment timeout query fact application: %w", err)
			}
		}
		log.Info().
			Int64("payment_order_id", paymentOrder.ID).
			Str("out_trade_no", paymentOrder.OutTradeNo).
			Str("transaction_id", transactionID).
			Str("remote_state", tradeState).
			Msg("payment timeout query found remote paid baofu order; local close skipped")
		return true, nil
	case db.ExternalPaymentTerminalStatusProcessing:
		return false, nil
	case db.ExternalPaymentTerminalStatusClosed, db.ExternalPaymentTerminalStatusFailed:
		log.Info().
			Int64("payment_order_id", paymentOrder.ID).
			Str("out_trade_no", paymentOrder.OutTradeNo).
			Str("remote_state", tradeState).
			Str("terminal_status", terminalStatus).
			Msg("baofu payment timeout query found terminal remote state; remote close skipped")
		return false, nil
	default:
		log.Error().
			Int64("payment_order_id", paymentOrder.ID).
			Str("payment_order_no", paymentOrder.OutTradeNo).
			Str("out_trade_no", paymentOrder.OutTradeNo).
			Str("remote_state", tradeState).
			Int64("remote_amount", queryResp.SuccessAmountFen).
			Int64("expected_amount", paymentOrder.Amount).
			Str("operation", "query_baofu_payment_before_timeout_close").
			Msg("baofu payment timeout query returned abnormal or unknown state; local close skipped")
		p.publishPaymentTimeoutUnexpectedRemoteStateAlert(ctx, paymentOrder, tradeState)
		return true, nil
	}
}

func (p *RedisTaskProcessor) closeBaofuPaymentOrderForTimeout(ctx context.Context, paymentOrder db.PaymentOrder) error {
	service := logic.NewBaofuPaymentService(p.store, p.baofuAggregateClient, logic.BaofuPaymentServiceConfig{
		CollectMerchantID: p.baofuProfitSharingConfig.CollectMerchantID,
		CollectTerminalID: p.baofuProfitSharingConfig.CollectTerminalID,
	})
	_, err := service.CloseOrder(ctx, logic.CloseBaofuOrderInput{
		PaymentOrder:  paymentOrder,
		BusinessOwner: paymentOrder.BusinessType,
	})
	if err != nil {
		wrapped := fmt.Errorf("close baofu payment order before local timeout close: %w", err)
		log.Error().Err(wrapped).
			Int64("payment_order_id", paymentOrder.ID).
			Str("payment_order_no", paymentOrder.OutTradeNo).
			Str("out_trade_no", paymentOrder.OutTradeNo).
			Str("operation", "close_baofu_payment_before_timeout_close").
			Msg("baofu payment timeout remote close failed")
		return wrapped
	}
	return nil
}
