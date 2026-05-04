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
)

func (svc *PaymentOrderService) createReservationBaofuPayment(
	ctx context.Context,
	input CreatePaymentOrderInput,
	merchantID int64,
	merchantName string,
	paymentMode string,
	amount int64,
	attach string,
	expiresAt time.Time,
) (CreatePaymentOrderResult, error) {
	if merchantID > 0 {
		merchant, err := svc.store.GetMerchant(ctx, merchantID)
		if err == nil {
			if paymentMode == paymentModeFull {
				merchantName = merchant.Name + " - Reservation Prepaid"
			} else {
				merchantName = merchant.Name + " - Reservation Deposit"
			}
		}
	}
	return svc.createBaofuPayment(ctx, ordinaryPaymentCreateInput{
		CreatePaymentOrderInput: input,
		MerchantID:              merchantID,
		MerchantName:            merchantName,
		PaymentMode:             paymentMode,
		Amount:                  amount,
		Attach:                  attach,
		ExpiresAt:               expiresAt,
		BusinessOwner:           db.ExternalPaymentBusinessOwnerReservation,
		ProfitSharing:           true,
	})
}

func (svc *PaymentOrderService) createOrderBaofuPayment(
	ctx context.Context,
	input CreatePaymentOrderInput,
	merchantID int64,
	merchantName string,
	amount int64,
	attach string,
	expiresAt time.Time,
) (CreatePaymentOrderResult, error) {
	if merchantID > 0 {
		merchant, err := svc.store.GetMerchant(ctx, merchantID)
		if err == nil {
			merchantName = merchant.Name + " - Order Payment"
		}
	}
	return svc.createBaofuPayment(ctx, ordinaryPaymentCreateInput{
		CreatePaymentOrderInput: input,
		MerchantID:              merchantID,
		MerchantName:            merchantName,
		Amount:                  amount,
		Attach:                  attach,
		ExpiresAt:               expiresAt,
		BusinessOwner:           db.ExternalPaymentBusinessOwnerOrder,
		ProfitSharing:           true,
	})
}

func (svc *PaymentOrderService) createBaofuPayment(ctx context.Context, createInput ordinaryPaymentCreateInput) (CreatePaymentOrderResult, error) {
	var result CreatePaymentOrderResult
	if svc.baofuPaymentService == nil {
		return result, fmt.Errorf("baofu payment service: not configured")
	}
	readiness, err := merchantBaofuReadinessForPayment(ctx, svc.store, createInput.MerchantID)
	if err != nil {
		return result, err
	}
	user, err := svc.store.GetUser(ctx, createInput.UserID)
	if err != nil {
		return result, fmt.Errorf("get user: %w", err)
	}
	if strings.TrimSpace(user.WechatOpenid) == "" {
		return result, NewRequestError(http.StatusBadRequest, errors.New("wechat openid not found"))
	}

	txResult, err := svc.createLocalBaofuPaymentOrder(ctx, createInput)
	if err != nil {
		return result, err
	}
	result.PaymentOrder = txResult.PaymentOrder

	baofuResult, err := svc.baofuPaymentService.CreateWechatJSAPIOrder(ctx, CreateBaofuWechatJSAPIOrderInput{
		PaymentOrder:     txResult.PaymentOrder,
		MerchantSubMchID: readiness.SubMchID,
		PayerOpenID:      user.WechatOpenid,
		Body:             createInput.MerchantName,
		ClientIP:         createInput.ClientIP,
		BusinessOwner:    createInput.BusinessOwner,
	})
	if err != nil {
		if _, closeErr := svc.store.UpdatePaymentOrderToClosed(ctx, txResult.PaymentOrder.ID); closeErr != nil {
			return result, fmt.Errorf("close baofu payment order after create failure: %w", closeErr)
		}
		return result, mapBaofuPaymentCreateError(err)
	}
	payParams, err := baofuWechatPayDataToPayParams(baofuResult.WechatPayData)
	if err != nil {
		if _, closeErr := svc.baofuPaymentService.CloseOrder(ctx, CloseBaofuOrderInput{
			PaymentOrder:  txResult.PaymentOrder,
			BusinessOwner: createInput.BusinessOwner,
		}); closeErr != nil {
			return result, fmt.Errorf("close baofu upstream order after local parse failure: %w", closeErr)
		}
		if _, closeErr := svc.store.UpdatePaymentOrderToClosed(ctx, txResult.PaymentOrder.ID); closeErr != nil {
			return result, fmt.Errorf("close baofu payment order after local parse failure: %w", closeErr)
		}
		return result, err
	}
	result.PaymentOrder = txResult.PaymentOrder
	result.PayParams = payParams
	return result, nil
}

func (svc *PaymentOrderService) createLocalBaofuPaymentOrder(ctx context.Context, createInput ordinaryPaymentCreateInput) (db.CreatePartnerPaymentTxResult, error) {
	prefix := "BF"
	orderID := createInput.OrderID
	reservationID := int64(0)
	if createInput.BusinessType == businessTypeReservation {
		prefix = "BFR"
		orderID = 0
		reservationID = createInput.OrderID
	}
	outTradeNo, err := generateOutTradeNoWithPrefix(prefix)
	if err != nil {
		return db.CreatePartnerPaymentTxResult{}, fmt.Errorf("generate baofu out trade no: %w", err)
	}
	txResult, err := svc.store.CreatePartnerPaymentTx(ctx, db.CreatePartnerPaymentTxParams{
		UserID:                createInput.UserID,
		MerchantID:            createInput.MerchantID,
		OrderID:               orderID,
		ReservationID:         reservationID,
		PaymentMode:           createInput.PaymentMode,
		BusinessType:          createInput.BusinessType,
		Amount:                createInput.Amount,
		OutTradeNo:            outTradeNo,
		ExpiresAt:             createInput.ExpiresAt,
		Attach:                createInput.Attach,
		PaymentChannel:        db.PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
	})
	if err != nil {
		return db.CreatePartnerPaymentTxResult{}, mapReservationEcommerceError(err)
	}
	return txResult, nil
}

func baofuWechatPayDataToPayParams(raw json.RawMessage) (*wechat.JSAPIPayParams, error) {
	var payParams wechat.JSAPIPayParams
	if err := json.Unmarshal(raw, &payParams); err != nil {
		return nil, fmt.Errorf("parse baofu wechat pay data: %w", err)
	}
	if strings.TrimSpace(payParams.TimeStamp) == "" ||
		strings.TrimSpace(payParams.NonceStr) == "" ||
		strings.TrimSpace(payParams.Package) == "" ||
		strings.TrimSpace(payParams.SignType) == "" ||
		strings.TrimSpace(payParams.PaySign) == "" {
		return nil, ErrBaofuPaymentWechatPayDataRequired
	}
	return &payParams, nil
}
