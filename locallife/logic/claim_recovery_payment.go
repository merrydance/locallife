package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/wechat"
	"github.com/rs/zerolog/log"
)

const businessTypeClaimRecovery = "claim_recovery"

type ClaimRecoveryPaymentResult struct {
	Recovery     db.ClaimRecovery
	PaymentOrder db.PaymentOrder
	PayParams    *wechat.JSAPIPayParams
}

type CreateMerchantClaimRecoveryPaymentInput struct {
	ClaimID     int64
	MerchantID  int64
	PayerUserID int64
	ClientIP    string
}

type CreateRiderClaimRecoveryPaymentInput struct {
	ClaimID     int64
	RiderID     int64
	PayerUserID int64
	ClientIP    string
}

type claimRecoveryPaymentAttach struct {
	ClaimID        int64  `json:"claim_id"`
	RecoveryID     int64  `json:"recovery_id"`
	RecoveryTarget string `json:"recovery_target"`
}

func CreateMerchantClaimRecoveryPayment(ctx context.Context, store db.Store, paymentClient wechat.PaymentClientInterface, input CreateMerchantClaimRecoveryPaymentInput) (ClaimRecoveryPaymentResult, error) {
	claimInfo, err := store.GetClaimForAppeal(ctx, input.ClaimID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return ClaimRecoveryPaymentResult{}, NewRequestError(http.StatusNotFound, errors.New("claim not found or not eligible for recovery"))
		}
		return ClaimRecoveryPaymentResult{}, err
	}
	if claimInfo.MerchantID != input.MerchantID {
		return ClaimRecoveryPaymentResult{}, NewRequestError(http.StatusForbidden, errors.New("this claim does not belong to your merchant"))
	}

	recovery, err := store.GetClaimRecoveryByClaimID(ctx, input.ClaimID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return ClaimRecoveryPaymentResult{}, NewRequestError(http.StatusNotFound, errors.New("claim recovery not found"))
		}
		return ClaimRecoveryPaymentResult{}, err
	}
	if !recovery.RecoveryTarget.Valid || recovery.RecoveryTarget.String != "merchant" {
		return ClaimRecoveryPaymentResult{}, NewRequestError(http.StatusBadRequest, errors.New("recovery target mismatch"))
	}

	return createClaimRecoveryPayment(ctx, store, paymentClient, recovery, input.PayerUserID, input.ClientIP, "merchant")
}

func CreateRiderClaimRecoveryPayment(ctx context.Context, store db.Store, paymentClient wechat.PaymentClientInterface, input CreateRiderClaimRecoveryPaymentInput) (ClaimRecoveryPaymentResult, error) {
	claimInfo, err := store.GetClaimForAppeal(ctx, input.ClaimID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return ClaimRecoveryPaymentResult{}, NewRequestError(http.StatusNotFound, errors.New("claim not found or not eligible for recovery"))
		}
		return ClaimRecoveryPaymentResult{}, err
	}
	if !claimInfo.RiderID.Valid || claimInfo.RiderID.Int64 != input.RiderID {
		return ClaimRecoveryPaymentResult{}, NewRequestError(http.StatusForbidden, errors.New("this claim does not belong to your rider"))
	}

	recovery, err := store.GetClaimRecoveryByClaimID(ctx, input.ClaimID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return ClaimRecoveryPaymentResult{}, NewRequestError(http.StatusNotFound, errors.New("claim recovery not found"))
		}
		return ClaimRecoveryPaymentResult{}, err
	}
	if !recovery.RecoveryTarget.Valid || recovery.RecoveryTarget.String != "rider" {
		return ClaimRecoveryPaymentResult{}, NewRequestError(http.StatusBadRequest, errors.New("recovery target mismatch"))
	}

	return createClaimRecoveryPayment(ctx, store, paymentClient, recovery, input.PayerUserID, input.ClientIP, "rider")
}

func createClaimRecoveryPayment(ctx context.Context, store db.Store, paymentClient wechat.PaymentClientInterface, recovery db.ClaimRecovery, payerUserID int64, clientIP string, recoveryTarget string) (ClaimRecoveryPaymentResult, error) {
	if paymentClient == nil {
		return ClaimRecoveryPaymentResult{}, NewRequestError(http.StatusServiceUnavailable, errors.New("payment client not configured"))
	}
	if recovery.Status == "paid" {
		return ClaimRecoveryPaymentResult{}, NewRequestError(http.StatusBadRequest, errors.New("claim recovery already paid"))
	}
	if recovery.Status != "pending" && recovery.Status != "overdue" {
		return ClaimRecoveryPaymentResult{}, NewRequestError(http.StatusBadRequest, fmt.Errorf("claim recovery cannot be paid in status %s", recovery.Status))
	}

	attachText, err := marshalClaimRecoveryPaymentAttach(recovery)
	if err != nil {
		return ClaimRecoveryPaymentResult{}, err
	}

	existing, err := getExistingClaimRecoveryPayment(ctx, store, attachText, payerUserID)
	if err != nil {
		return ClaimRecoveryPaymentResult{}, err
	}
	if existing != nil {
		if shouldRotateExpiredClaimRecoveryPayment(*existing, time.Now()) {
			if err := closeExpiredClaimRecoveryPayment(ctx, store, paymentClient, *existing); err != nil {
				return ClaimRecoveryPaymentResult{}, err
			}
		} else {
			return reuseClaimRecoveryPayment(paymentClient, recovery, *existing)
		}
	}

	user, err := store.GetUser(ctx, payerUserID)
	if err != nil {
		return ClaimRecoveryPaymentResult{}, fmt.Errorf("get payer user: %w", err)
	}
	if user.WechatOpenid == "" {
		return ClaimRecoveryPaymentResult{}, NewRequestError(http.StatusBadRequest, errors.New("wechat openid not found"))
	}

	outTradeNo, err := util.GenerateOutTradeNo("CR")
	if err != nil {
		return ClaimRecoveryPaymentResult{}, fmt.Errorf("generate out trade no: %w", err)
	}

	expiresAt := time.Now().Add(30 * time.Minute)
	paymentOrder, err := store.CreatePaymentOrder(ctx, db.CreatePaymentOrderParams{
		OrderID:      pgtype.Int8{Int64: recovery.OrderID, Valid: true},
		UserID:       payerUserID,
		PaymentType:  "miniprogram",
		BusinessType: businessTypeClaimRecovery,
		Amount:       recovery.RecoveryAmount,
		OutTradeNo:   outTradeNo,
		ExpiresAt:    pgtype.Timestamptz{Time: expiresAt, Valid: true},
		Attach:       pgtype.Text{String: attachText, Valid: true},
	})
	if err != nil {
		if db.ErrorCode(err) == db.UniqueViolation {
			existing, refetchErr := getExistingClaimRecoveryPayment(ctx, store, attachText, payerUserID)
			if refetchErr != nil {
				return ClaimRecoveryPaymentResult{}, fmt.Errorf("create claim recovery payment order: %w (refetch existing: %v)", err, refetchErr)
			}
			if existing == nil {
				return ClaimRecoveryPaymentResult{}, fmt.Errorf("create claim recovery payment order: %w", err)
			}
			return reuseClaimRecoveryPayment(paymentClient, recovery, *existing)
		}
		return ClaimRecoveryPaymentResult{}, fmt.Errorf("create claim recovery payment order: %w", err)
	}

	description := "索赔追偿支付"
	switch recoveryTarget {
	case "merchant":
		description = "商户索赔追偿支付"
	case "rider":
		description = "骑手索赔追偿支付"
	}

	wxResp, payParams, err := paymentClient.CreateJSAPIOrder(ctx, &wechat.JSAPIOrderRequest{
		OutTradeNo:    outTradeNo,
		Description:   description,
		TotalAmount:   recovery.RecoveryAmount,
		OpenID:        user.WechatOpenid,
		ExpireTime:    expiresAt,
		Attach:        attachText,
		PayerClientIP: clientIP,
	})
	if err != nil {
		_, _ = store.UpdatePaymentOrderToClosed(ctx, paymentOrder.ID)
		return ClaimRecoveryPaymentResult{}, fmt.Errorf("wechat pay: %w", err)
	}

	updatedPaymentOrder, err := store.UpdatePaymentOrderPrepayId(ctx, db.UpdatePaymentOrderPrepayIdParams{
		ID:       paymentOrder.ID,
		PrepayID: pgtype.Text{String: wxResp.PrepayID, Valid: true},
	})
	if err != nil {
		_, _ = store.UpdatePaymentOrderToFailed(ctx, paymentOrder.ID)
		if closeErr := paymentClient.CloseOrder(ctx, outTradeNo); closeErr != nil {
			log.Warn().Err(closeErr).Str("out_trade_no", outTradeNo).Msg("close claim recovery wechat order after prepay update failure")
		}
		return ClaimRecoveryPaymentResult{}, fmt.Errorf("update claim recovery prepay id: %w", err)
	}

	return ClaimRecoveryPaymentResult{Recovery: recovery, PaymentOrder: updatedPaymentOrder, PayParams: payParams}, nil
}

func getExistingClaimRecoveryPayment(ctx context.Context, store db.Store, attachText string, payerUserID int64) (*db.PaymentOrder, error) {
	existing, lookupErr := store.GetLatestPaymentOrderByBusinessTypeAndAttach(ctx, db.GetLatestPaymentOrderByBusinessTypeAndAttachParams{
		BusinessType: businessTypeClaimRecovery,
		Attach:       pgtype.Text{String: attachText, Valid: true},
	})
	if lookupErr != nil {
		if errors.Is(lookupErr, db.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("lookup claim recovery payment order: %w", lookupErr)
	}
	if existing.UserID != payerUserID {
		return nil, NewRequestError(http.StatusBadRequest, errors.New("claim recovery already has an active payer"))
	}
	if existing.Status != "pending" && existing.Status != "paid" {
		return nil, nil
	}
	return &existing, nil
}

func reuseClaimRecoveryPayment(paymentClient wechat.PaymentClientInterface, recovery db.ClaimRecovery, paymentOrder db.PaymentOrder) (ClaimRecoveryPaymentResult, error) {
	payParams, err := regenerateClaimRecoveryPayParams(paymentClient, paymentOrder)
	if err != nil {
		return ClaimRecoveryPaymentResult{}, err
	}
	return ClaimRecoveryPaymentResult{Recovery: recovery, PaymentOrder: paymentOrder, PayParams: payParams}, nil
}

func shouldRotateExpiredClaimRecoveryPayment(paymentOrder db.PaymentOrder, now time.Time) bool {
	return paymentOrder.Status == "pending" &&
		paymentOrder.ExpiresAt.Valid &&
		!paymentOrder.ExpiresAt.Time.After(now)
}

func closeExpiredClaimRecoveryPayment(ctx context.Context, store db.Store, paymentClient wechat.PaymentClientInterface, paymentOrder db.PaymentOrder) error {
	if paymentOrder.OutTradeNo != "" && paymentClient != nil {
		if err := paymentClient.CloseOrder(ctx, paymentOrder.OutTradeNo); err != nil {
			log.Warn().Err(err).
				Str("out_trade_no", paymentOrder.OutTradeNo).
				Int64("payment_order_id", paymentOrder.ID).
				Msg("close expired claim recovery wechat order failed")
		}
	}

	if _, err := store.UpdatePaymentOrderToClosed(ctx, paymentOrder.ID); err != nil && !errors.Is(err, db.ErrRecordNotFound) {
		return fmt.Errorf("close expired claim recovery payment order: %w", err)
	}

	return nil
}

func marshalClaimRecoveryPaymentAttach(recovery db.ClaimRecovery) (string, error) {
	attach, err := json.Marshal(claimRecoveryPaymentAttach{
		ClaimID:        recovery.ClaimID,
		RecoveryID:     recovery.ID,
		RecoveryTarget: recovery.RecoveryTarget.String,
	})
	if err != nil {
		return "", fmt.Errorf("marshal claim recovery payment attach: %w", err)
	}
	return string(attach), nil
}

func regenerateClaimRecoveryPayParams(paymentClient wechat.PaymentClientInterface, paymentOrder db.PaymentOrder) (*wechat.JSAPIPayParams, error) {
	if !paymentOrder.PrepayID.Valid || paymentOrder.PrepayID.String == "" {
		return nil, nil
	}
	if paymentOrder.Status != "pending" {
		return nil, nil
	}
	payParams, err := paymentClient.GenerateJSAPIPayParams(paymentOrder.PrepayID.String)
	if err != nil {
		return nil, fmt.Errorf("generate claim recovery pay params: %w", err)
	}
	return payParams, nil
}
