package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
)

func (svc *PaymentOrderService) QueryPaymentOrder(ctx context.Context, input QueryPaymentOrderInput) (QueryPaymentOrderResult, error) {
	detail, err := svc.GetPaymentOrder(ctx, GetPaymentOrderInput{
		UserID:         input.UserID,
		PaymentOrderID: input.PaymentOrderID,
	})
	if err != nil {
		return QueryPaymentOrderResult{}, err
	}

	paymentOrder := detail.PaymentOrder
	if paymentOrder.CombinedPaymentID.Valid {
		return QueryPaymentOrderResult{}, NewRequestError(http.StatusBadRequest, errors.New("合单支付订单请使用合单查询接口"))
	}
	if paymentOrderUsesEcommerceChannel(paymentOrder) {
		return svc.queryEcommercePaymentOrder(ctx, paymentOrder)
	}
	if paymentOrder.PaymentChannel == db.PaymentChannelDirect {
		return svc.queryDirectPaymentOrder(ctx, paymentOrder)
	}
	return QueryPaymentOrderResult{}, NewRequestError(http.StatusBadRequest, errors.New("当前支付通道不支持微信远端查询"))
}

func (svc *PaymentOrderService) queryEcommercePaymentOrder(ctx context.Context, paymentOrder db.PaymentOrder) (QueryPaymentOrderResult, error) {
	if svc.ecommerceClient == nil {
		return QueryPaymentOrderResult{}, fmt.Errorf("ecommerce client: not configured")
	}

	subMchID, err := svc.resolvePaymentOrderSubMchID(ctx, paymentOrder)
	if err != nil {
		return QueryPaymentOrderResult{}, fmt.Errorf("resolve payment order sub_mchid: %w", err)
	}

	queryResp, err := svc.queryPartnerPaymentOrder(ctx, paymentOrder, subMchID)
	if err != nil {
		return QueryPaymentOrderResult{}, mapPartnerOrderQueryError(err)
	}

	var payParams *wechat.JSAPIPayParams
	if svc.shouldExposePartnerPaymentPayParams(paymentOrder, queryResp) {
		payParams, err = svc.signExistingPartnerPaymentOrder(paymentOrder)
		if err != nil {
			return QueryPaymentOrderResult{}, fmt.Errorf("sign payment order: %w", err)
		}
	}

	return QueryPaymentOrderResult{
		PaymentOrder: paymentOrder,
		PayParams:    payParams,
		WechatOrder:  mapPartnerPaymentWechatOrder(queryResp),
	}, nil
}

func (svc *PaymentOrderService) queryDirectPaymentOrder(ctx context.Context, paymentOrder db.PaymentOrder) (QueryPaymentOrderResult, error) {
	if svc.directPaymentClient == nil {
		return QueryPaymentOrderResult{}, fmt.Errorf("direct payment client: not configured")
	}
	if !isDirectPaymentOrderQueryBusinessType(paymentOrder.BusinessType) {
		return QueryPaymentOrderResult{}, NewRequestError(http.StatusBadRequest, errors.New("当前直连支付业务不支持微信远端查询"))
	}

	queryResp, err := svc.directPaymentClient.QueryOrderByOutTradeNo(ctx, paymentOrder.OutTradeNo)
	if err != nil {
		return QueryPaymentOrderResult{}, mapDirectOrderQueryError(err)
	}

	wechatOrder := mapDirectPaymentWechatOrder(queryResp)
	updatedPaymentOrder, err := svc.recordAndApplyDirectPaymentQueryFact(ctx, paymentOrder, queryResp)
	if err != nil {
		return QueryPaymentOrderResult{}, err
	}
	paymentOrder = updatedPaymentOrder

	var payParams *wechat.JSAPIPayParams
	if svc.shouldExposeDirectPaymentPayParams(paymentOrder, wechatOrder) {
		payParams, err = svc.directPaymentClient.GenerateJSAPIPayParams(paymentOrder.PrepayID.String)
		if err != nil {
			return QueryPaymentOrderResult{}, fmt.Errorf("sign direct payment order: %w", err)
		}
	}

	return QueryPaymentOrderResult{
		PaymentOrder: paymentOrder,
		PayParams:    payParams,
		WechatOrder:  wechatOrder,
	}, nil
}

type QueryPaymentOrderWechatOrder struct {
	AppID           string
	MchID           string
	SpAppID         string
	SpMchID         string
	SubAppID        string
	SubMchID        string
	OutTradeNo      string
	TransactionID   string
	TradeType       string
	TradeState      string
	TradeStateDesc  string
	BankType        string
	Attach          string
	SuccessTime     string
	Payer           QueryPaymentOrderWechatPayer
	Amount          QueryPaymentOrderWechatAmount
	SceneDeviceID   string
	PromotionDetail []wechatcontracts.PartnerPromotionDetail
}

type QueryPaymentOrderWechatPayer struct {
	OpenID    string
	SpOpenID  string
	SubOpenID string
}

type QueryPaymentOrderWechatAmount struct {
	Total         int64
	PayerTotal    int64
	Currency      string
	PayerCurrency string
}

func (svc *PaymentOrderService) shouldExposeDirectPaymentPayParams(paymentOrder db.PaymentOrder, wechatOrder *QueryPaymentOrderWechatOrder) bool {
	if paymentOrder.Status != paymentStatusPending {
		return false
	}
	if !paymentOrder.PrepayID.Valid || strings.TrimSpace(paymentOrder.PrepayID.String) == "" {
		return false
	}
	if paymentOrder.ExpiresAt.Valid && !svc.now().Before(paymentOrder.ExpiresAt.Time) {
		return false
	}
	if wechatOrder == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(wechatOrder.TradeState), "NOTPAY")
}

func mapPartnerPaymentWechatOrder(resp *wechatcontracts.PartnerOrderQueryResponse) *QueryPaymentOrderWechatOrder {
	if resp == nil {
		return nil
	}
	deviceID := ""
	if resp.SceneInfo != nil {
		deviceID = resp.SceneInfo.DeviceID
	}
	return &QueryPaymentOrderWechatOrder{
		SpAppID:         resp.SpAppID,
		SpMchID:         resp.SpMchID,
		SubAppID:        resp.SubAppID,
		SubMchID:        resp.SubMchID,
		OutTradeNo:      resp.OutTradeNo,
		TransactionID:   resp.TransactionID,
		TradeType:       resp.TradeType,
		TradeState:      resp.TradeState,
		TradeStateDesc:  resp.TradeStateDesc,
		BankType:        resp.BankType,
		Attach:          resp.Attach,
		SuccessTime:     resp.SuccessTime,
		Payer:           QueryPaymentOrderWechatPayer{SpOpenID: resp.Payer.SpOpenID, SubOpenID: resp.Payer.SubOpenID},
		Amount:          QueryPaymentOrderWechatAmount{Total: resp.Amount.Total, PayerTotal: resp.Amount.PayerTotal, Currency: resp.Amount.Currency, PayerCurrency: resp.Amount.PayerCurrency},
		SceneDeviceID:   deviceID,
		PromotionDetail: resp.PromotionDetail,
	}
}

func mapDirectPaymentWechatOrder(resp *wechatcontracts.DirectOrderQueryResponse) *QueryPaymentOrderWechatOrder {
	if resp == nil {
		return nil
	}
	deviceID := ""
	if resp.SceneInfo != nil {
		deviceID = resp.SceneInfo.DeviceID
	}
	return &QueryPaymentOrderWechatOrder{
		AppID:           resp.AppID,
		MchID:           resp.MchID,
		OutTradeNo:      resp.OutTradeNo,
		TransactionID:   resp.TransactionID,
		TradeType:       resp.TradeType,
		TradeState:      resp.TradeState,
		TradeStateDesc:  resp.TradeStateDesc,
		BankType:        resp.BankType,
		Attach:          resp.Attach,
		SuccessTime:     resp.SuccessTime,
		Payer:           QueryPaymentOrderWechatPayer{OpenID: resp.Payer.OpenID},
		Amount:          QueryPaymentOrderWechatAmount{Total: resp.Amount.Total, PayerTotal: resp.Amount.PayerTotal, Currency: resp.Amount.Currency, PayerCurrency: resp.Amount.PayerCurrency},
		SceneDeviceID:   deviceID,
		PromotionDetail: mapDirectPromotionDetails(resp.PromotionDetail),
	}
}

func mapDirectPromotionDetails(items []wechatcontracts.DirectPromotionDetail) []wechatcontracts.PartnerPromotionDetail {
	if len(items) == 0 {
		return nil
	}
	mapped := make([]wechatcontracts.PartnerPromotionDetail, 0, len(items))
	for _, item := range items {
		mapped = append(mapped, wechatcontracts.PartnerPromotionDetail{
			CouponID:            item.CouponID,
			Name:                item.Name,
			Scope:               item.Scope,
			Type:                item.Type,
			Amount:              item.Amount,
			StockID:             item.StockID,
			WechatpayContribute: item.WechatpayContribute,
			MerchantContribute:  item.MerchantContribute,
			OtherContribute:     item.OtherContribute,
			Currency:            item.Currency,
			GoodsDetail:         mapDirectPromotionGoodsDetails(item.GoodsDetail),
		})
	}
	return mapped
}

func mapDirectPromotionGoodsDetails(items []wechatcontracts.DirectPromotionGoodsDetail) []wechatcontracts.PartnerPromotionGoodsDetail {
	if len(items) == 0 {
		return nil
	}
	mapped := make([]wechatcontracts.PartnerPromotionGoodsDetail, 0, len(items))
	for _, item := range items {
		mapped = append(mapped, wechatcontracts.PartnerPromotionGoodsDetail{
			GoodsID:        item.GoodsID,
			Quantity:       item.Quantity,
			UnitPrice:      item.UnitPrice,
			DiscountAmount: item.DiscountAmount,
			GoodsRemark:    item.GoodsRemark,
		})
	}
	return mapped
}

func (svc *PaymentOrderService) recordAndApplyDirectPaymentQueryFact(ctx context.Context, paymentOrder db.PaymentOrder, queryResp *wechatcontracts.DirectOrderQueryResponse) (db.PaymentOrder, error) {
	if queryResp == nil {
		return paymentOrder, nil
	}
	consumer, ok := directPaymentFactConsumer(paymentOrder.BusinessType)
	if !ok {
		return paymentOrder, nil
	}
	terminalStatus := NormalizeDirectPaymentTerminalStatus(queryResp.TradeState)
	if !isExternalPaymentTerminalStatus(terminalStatus) {
		return paymentOrder, nil
	}
	rawResource, err := json.Marshal(queryResp)
	if err != nil {
		return paymentOrder, fmt.Errorf("marshal direct payment query response: %w", err)
	}
	occurredAt := parseDirectPaymentQueryFactTime(queryResp.SuccessTime)
	input := RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelDirect,
		Capability:           db.ExternalPaymentCapabilityDirectJSAPIPayment,
		FactSource:           db.ExternalPaymentFactSourceQuery,
		ExternalObjectType:   db.ExternalPaymentObjectPayment,
		ExternalObjectKey:    queryResp.OutTradeNo,
		ExternalSecondaryKey: stringPtrIfNotEmpty(queryResp.TransactionID),
		BusinessOwner:        stringPtrIfNotEmpty(paymentOrder.BusinessType),
		BusinessObjectType:   stringPtrIfNotEmpty(paymentFactBusinessObjectPaymentOrder),
		BusinessObjectID:     &paymentOrder.ID,
		UpstreamState:        queryResp.TradeState,
		TerminalStatus:       terminalStatus,
		Amount:               &queryResp.Amount.Total,
		Currency:             "CNY",
		OccurredAt:           occurredAt,
		UpstreamUpdatedAt:    occurredAt,
		RawResource:          rawResource,
		DedupeKey:            fmt.Sprintf("wechat:query:direct_payment:%s:%s", queryResp.OutTradeNo, queryResp.TradeState),
	}
	if terminalStatus == db.ExternalPaymentTerminalStatusSuccess {
		input.Application = &ExternalPaymentFactApplicationTarget{
			Consumer:           consumer,
			BusinessObjectType: paymentFactBusinessObjectPaymentOrder,
			BusinessObjectID:   paymentOrder.ID,
		}
	}

	factResult, err := NewPaymentFactService(svc.store).RecordExternalPaymentFact(ctx, input)
	if err != nil {
		return paymentOrder, fmt.Errorf("record direct payment query fact: %w", err)
	}
	if factResult.Application == nil {
		return paymentOrder, nil
	}
	if _, err := NewPaymentFactService(svc.store).ApplyExternalPaymentFactApplication(ctx, factResult.Application.ID); err != nil {
		return paymentOrder, fmt.Errorf("apply direct payment query fact: %w", err)
	}
	updatedPaymentOrder, err := svc.store.GetPaymentOrder(ctx, paymentOrder.ID)
	if err != nil {
		return paymentOrder, fmt.Errorf("get direct payment order after query fact application: %w", err)
	}
	return updatedPaymentOrder, nil
}

func directPaymentFactConsumer(businessType string) (string, bool) {
	switch businessType {
	case db.ExternalPaymentBusinessOwnerRiderDeposit:
		return paymentFactConsumerRiderDepositDomain, true
	case db.ExternalPaymentBusinessOwnerClaimRecovery:
		return paymentFactConsumerClaimRecoveryDomain, true
	default:
		return "", false
	}
}

func isDirectPaymentOrderQueryBusinessType(businessType string) bool {
	_, ok := directPaymentFactConsumer(businessType)
	return ok
}

func parseDirectPaymentQueryFactTime(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil
	}
	parsed = parsed.UTC()
	return &parsed
}
