package worker

import (
	"context"
	"errors"
	"strings"

	"github.com/merrydance/locallife/baofu"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func baofuProcessingShareCanReturnToFailed(order db.ProfitSharingOrder, err error) bool {
	if order.Status != db.ProfitSharingOrderStatusProcessing {
		return false
	}
	if order.SharingOrderID.Valid && strings.TrimSpace(order.SharingOrderID.String) != "" && strings.TrimSpace(order.SharingOrderID.String) != strings.TrimSpace(order.OutOrderNo) {
		return false
	}
	var providerErr *baofu.ProviderError
	if !errors.As(err, &providerErr) || providerErr == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(providerErr.UpstreamCode), "ORDER_NOT_EXIST")
}

func (s *BaofuPaymentRecoveryScheduler) queryBaofuProfitSharing(ctx context.Context, cfg BaofuProfitSharingWorkerConfig, order db.ProfitSharingOrder) (*aggregatecontracts.ShareResult, error) {
	req := aggregatecontracts.ShareQueryRequest{
		MerchantID: cfg.CollectMerchantID,
		TerminalID: cfg.CollectTerminalID,
	}
	sharingOrderID := strings.TrimSpace(order.SharingOrderID.String)
	outOrderNo := strings.TrimSpace(order.OutOrderNo)
	if order.SharingOrderID.Valid && sharingOrderID != "" && sharingOrderID != outOrderNo {
		req.TradeNo = strings.TrimSpace(order.SharingOrderID.String)
	} else {
		req.OutTradeNo = outOrderNo
	}
	result, err := s.client.QueryProfitSharing(ctx, req)
	if err == nil || req.TradeNo == "" || outOrderNo == "" || !baofuShareQueryShouldRetryByOutTradeNo(err) {
		return result, err
	}

	logBaofuProfitSharingQueryError(log.Warn().Err(err), order, "tradeNo", err).
		Msg("baofu share query by tradeNo failed; retrying by outTradeNo")
	req.TradeNo = ""
	req.OutTradeNo = outOrderNo
	result, err = s.client.QueryProfitSharing(ctx, req)
	if err != nil {
		logBaofuProfitSharingQueryError(log.Warn().Err(err), order, "outTradeNo", err).
			Msg("baofu share query by outTradeNo failed after tradeNo retry")
	}
	return result, err
}

func baofuShareQueryShouldRetryByOutTradeNo(err error) bool {
	var providerErr *baofu.ProviderError
	if !errors.As(err, &providerErr) || providerErr == nil {
		return false
	}
	if baofu.IsProviderBusinessResponseError(err) {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(providerErr.UpstreamCode), baofu.PublicEnvelopeUpstreamCodeInvalidDataContent)
}

func logBaofuProfitSharingQueryError(event *zerolog.Event, order db.ProfitSharingOrder, queryKeyMode string, err error) *zerolog.Event {
	event = event.
		Int64("profit_sharing_order_id", order.ID).
		Str("out_order_no", strings.TrimSpace(order.OutOrderNo)).
		Str("provider_operation", "share_query").
		Str("query_key_mode", strings.TrimSpace(queryKeyMode))
	var providerErr *baofu.ProviderError
	if errors.As(err, &providerErr) && providerErr != nil {
		event = event.
			Str("provider_method", strings.TrimSpace(providerErr.Operation)).
			Str("provider_capability", strings.TrimSpace(providerErr.Capability)).
			Str("upstream_code", strings.TrimSpace(providerErr.UpstreamCode)).
			Str("frontend_code", strings.TrimSpace(providerErr.Frontend.Code)).
			Bool("retryable", providerErr.Frontend.Retryable)
		if providerErr.StatusCode != 0 {
			event = event.Int("http_status", providerErr.StatusCode)
		}
		if cause := errors.Unwrap(providerErr); cause != nil {
			event = event.Str("provider_error_cause", strings.TrimSpace(cause.Error()))
		}
	}
	return event
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
