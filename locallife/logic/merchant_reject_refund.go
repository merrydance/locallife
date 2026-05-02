package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
	"github.com/rs/zerolog/log"
)

// MerchantRejectRefundInput defines the input for refunding a rejected order.
type MerchantRejectRefundInput struct {
	MerchantID int64 // 商户ID，收付通退款路径需要用于获取 SubMchID
	OrderID    int64
	Reason     string
}

// MerchantRejectRefundResult captures refund processing details.
type MerchantRejectRefundResult struct {
	PaymentOrder *db.PaymentOrder
	RefundOrder  *db.RefundOrder
}

type merchantRejectOrdinaryRefundClient interface {
	RefundNotifyURL() string
	CreateRefund(ctx context.Context, req ospcontracts.RefundCreateRequest) (*ospcontracts.RefundResponse, error)
}

// ProcessMerchantRejectRefund handles full refund for a merchant-rejected order.
// Deprecated active callers should use ProcessMerchantRejectRefundWithOrdinaryServiceProvider.
func ProcessMerchantRejectRefund(
	ctx context.Context,
	store db.Store,
	ecommerceClient wechat.EcommerceClientInterface,
	input MerchantRejectRefundInput,
) (MerchantRejectRefundResult, error) {
	return ProcessMerchantRejectRefundWithOrdinaryServiceProvider(ctx, store, ecommerceClient, nil, input)
}

// ProcessMerchantRejectRefundWithOrdinaryServiceProvider handles full refund for a merchant-rejected order.
// Main-business refunds route by the persisted payment channel; ecommerce remains a cold-reserve branch only.
func ProcessMerchantRejectRefundWithOrdinaryServiceProvider(
	ctx context.Context,
	store db.Store,
	ecommerceClient wechat.EcommerceClientInterface,
	ordinaryClient merchantRejectOrdinaryRefundClient,
	input MerchantRejectRefundInput,
) (MerchantRejectRefundResult, error) {
	var result MerchantRejectRefundResult

	paymentOrder, err := store.GetLatestPaymentOrderByOrder(ctx, db.GetLatestPaymentOrderByOrderParams{
		OrderID:      pgtype.Int8{Int64: input.OrderID, Valid: true},
		BusinessType: "order",
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, nil
		}
		return result, err
	}
	if paymentOrder.Status != "paid" {
		paymentOrders, listErr := store.GetPaymentOrdersByOrder(ctx, pgtype.Int8{Int64: input.OrderID, Valid: true})
		if listErr != nil {
			return result, listErr
		}

		foundPaid := false
		for _, candidate := range paymentOrders {
			if candidate.BusinessType == "order" && candidate.Status == "paid" {
				paymentOrder = candidate
				foundPaid = true
				break
			}
		}

		if !foundPaid {
			return result, nil
		}
	}
	result.PaymentOrder = &paymentOrder
	if !paymentOrderUsesEcommerceChannel(paymentOrder) && !db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder) {
		return result, mainBusinessEcommerceOnlyError("处理商户拒单退款")
	}

	reason := fmt.Sprintf("商户拒单：%s", input.Reason)
	outRefundNo, err := generateOutRefundNo()
	if err != nil {
		return result, fmt.Errorf("generate out refund no: %w", err)
	}

	txResult, err := store.CreateRefundOrderTx(ctx, db.CreateRefundOrderTxParams{
		PaymentOrderID: paymentOrder.ID,
		RefundType:     paymentTypeProfitSharing,
		RefundAmount:   paymentOrder.Amount,
		RefundReason:   reason,
		OutRefundNo:    outRefundNo,
	})
	if err != nil {
		if _, ok := db.IsRefundRequestError(err); ok {
			return result, fmt.Errorf("refund validation: %w", err)
		}
		return result, err
	}
	refundOrder := txResult.RefundOrder
	result.RefundOrder = &refundOrder
	if db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder) {
		return result, processMerchantRejectOrdinaryServiceProviderRefund(ctx, store, ordinaryClient, paymentOrder, refundOrder, outRefundNo, reason, input.MerchantID)
	}
	return result, processMerchantRejectEcommerceRefund(ctx, store, ecommerceClient, paymentOrder, refundOrder, outRefundNo, reason, input.MerchantID)
}

// processMerchantRejectEcommerceRefund 收付通合单支付订单的商户拒单退款。
// 商户拒单时分账尚未执行，可直接调用电商退款接口，无需分账回退。
func processMerchantRejectEcommerceRefund(
	ctx context.Context,
	store db.Store,
	ecommerceClient wechat.EcommerceClientInterface,
	paymentOrder db.PaymentOrder,
	refundOrder db.RefundOrder,
	outRefundNo, reason string,
	merchantID int64,
) error {
	if ecommerceClient == nil {
		return nil
	}

	paymentConfig, err := store.GetMerchantPaymentConfig(ctx, merchantID)
	if err != nil {
		return fmt.Errorf("get merchant payment config: %w", err)
	}

	wxRefund, err := createEcommerceRefundContract(ctx, ecommerceClient, &wechatcontracts.EcommerceRefundRequest{
		SubMchID:    paymentConfig.SubMchID,
		OutTradeNo:  paymentOrder.OutTradeNo,
		OutRefundNo: outRefundNo,
		Reason:      reason,
		Amount: &wechatcontracts.EcommerceRefundRequestAmount{
			Refund:   paymentOrder.Amount,
			Total:    paymentOrder.Amount,
			Currency: wechatcontracts.EcommerceRefundCurrencyCNY,
		},
	})
	if err != nil {
		// 微信API失败时保持pending状态，由 RefundRecoveryScheduler 每5分钟自动补偿重试
		return fmt.Errorf("wechat ecommerce refund api: %w", err)
	}

	if _, dbErr := store.UpdateRefundOrderToProcessing(ctx, db.UpdateRefundOrderToProcessingParams{
		ID:       refundOrder.ID,
		RefundID: pgtype.Text{String: wxRefund.RefundID, Valid: wxRefund.RefundID != ""},
	}); dbErr != nil {
		log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as processing")
	}
	recordMerchantRejectRefundCommandAccepted(ctx, store, paymentOrder, refundOrder, outRefundNo, wxRefund.RefundID)
	return nil
}

// processMerchantRejectOrdinaryServiceProviderRefund handles merchant-reject refund for ordinary service-provider orders.
func processMerchantRejectOrdinaryServiceProviderRefund(
	ctx context.Context,
	store db.Store,
	ordinaryClient merchantRejectOrdinaryRefundClient,
	paymentOrder db.PaymentOrder,
	refundOrder db.RefundOrder,
	outRefundNo, reason string,
	merchantID int64,
) error {
	if ordinaryClient == nil {
		configErr := errors.New("ordinary service provider client not configured")
		log.Error().
			Err(configErr).
			Int64("payment_order_id", paymentOrder.ID).
			Int64("refund_order_id", refundOrder.ID).
			Msg("ordinary service provider refund client missing for merchant reject refund")
		return NewRequestErrorWithCause(http.StatusServiceUnavailable, errors.New("微信服务商退款配置未完成，当前无法发起退款，请联系平台处理"), configErr)
	}

	paymentConfig, err := store.GetMerchantPaymentConfig(ctx, merchantID)
	if err != nil {
		return fmt.Errorf("get merchant payment config: %w", err)
	}

	wxRefund, err := ordinaryClient.CreateRefund(ctx, ospcontracts.RefundCreateRequest{
		SubMchID:    paymentConfig.SubMchID,
		OutTradeNo:  paymentOrder.OutTradeNo,
		OutRefundNo: outRefundNo,
		Reason:      reason,
		NotifyURL:   ordinaryClient.RefundNotifyURL(),
		Amount: ospcontracts.RefundAmountRequest{
			Refund:   paymentOrder.Amount,
			Total:    paymentOrder.Amount,
			Currency: ospcontracts.CurrencyCNY,
		},
	})
	if err != nil {
		return fmt.Errorf("wechat ordinary service provider refund api: %w", mapOrdinaryServiceProviderRefundCreateError(err))
	}

	refundID := ""
	if wxRefund != nil {
		refundID = wxRefund.RefundID
	}
	if _, dbErr := store.UpdateRefundOrderToProcessing(ctx, db.UpdateRefundOrderToProcessingParams{
		ID:       refundOrder.ID,
		RefundID: pgtype.Text{String: refundID, Valid: refundID != ""},
	}); dbErr != nil {
		log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as processing")
	}
	recordMerchantRejectRefundCommandAccepted(ctx, store, paymentOrder, refundOrder, outRefundNo, refundID)
	return nil
}

func recordMerchantRejectRefundCommandAccepted(
	ctx context.Context,
	store db.Store,
	paymentOrder db.PaymentOrder,
	refundOrder db.RefundOrder,
	outRefundNo string,
	refundID string,
) {
	paymentCommandSvc := NewPaymentCommandService(store)
	_, err := paymentCommandSvc.RecordExternalPaymentCommand(ctx, dbMerchantRejectRefundCommandInput(
		paymentOrder,
		refundOrder,
		outRefundNo,
		db.ExternalPaymentCommandStatusAccepted,
		stringPtrIfNotEmpty(refundID),
		merchantRejectRefundCommandSnapshot(map[string]string{
			"out_refund_no": outRefundNo,
			"refund_id":     refundID,
		}),
	))
	if err != nil {
		log.Error().Err(err).
			Int64("refund_order_id", refundOrder.ID).
			Str("out_refund_no", outRefundNo).
			Msg("record merchant reject ecommerce refund command accepted failed")
	}
}

func dbMerchantRejectRefundCommandInput(
	paymentOrder db.PaymentOrder,
	refundOrder db.RefundOrder,
	outRefundNo string,
	commandStatus string,
	externalSecondaryKey *string,
	responseSnapshot []byte,
) RecordExternalPaymentCommandInput {
	businessObjectType := "refund_order"
	businessObjectID := refundOrder.ID
	return RecordExternalPaymentCommandInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              paymentOrder.PaymentChannel,
		Capability:           refundServiceCreateRefundCapability(paymentOrder.PaymentChannel),
		CommandType:          db.ExternalPaymentCommandTypeCreateRefund,
		BusinessOwner:        db.ExternalPaymentBusinessOwnerOrder,
		BusinessObjectType:   &businessObjectType,
		BusinessObjectID:     &businessObjectID,
		ExternalObjectType:   db.ExternalPaymentObjectRefund,
		ExternalObjectKey:    outRefundNo,
		ExternalSecondaryKey: externalSecondaryKey,
		CommandStatus:        commandStatus,
		ResponseSnapshot:     responseSnapshot,
	}
}

func merchantRejectRefundCommandSnapshot(values map[string]string) []byte {
	filtered := make(map[string]string, len(values))
	for key, value := range values {
		if value != "" {
			filtered[key] = value
		}
	}
	if len(filtered) == 0 {
		return []byte(`{}`)
	}
	data, err := json.Marshal(filtered)
	if err != nil {
		return []byte(`{}`)
	}
	return data
}
