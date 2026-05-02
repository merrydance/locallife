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
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	"github.com/rs/zerolog/log"
)

const (
	businessTypeClaimRecovery           = "claim_recovery"
	claimRecoveryPaymentOrderObjectType = "payment_order"
)

type ClaimRecoveryPaymentResult struct {
	Recovery     db.ClaimRecovery
	PaymentOrder db.PaymentOrder
	PayParams    *wechat.JSAPIPayParams
}

type CreateMerchantClaimRecoveryPaymentInput struct {
	RecoveryID  int64
	MerchantID  int64
	PayerUserID int64
	ClientIP    string
}

type CreateRiderClaimRecoveryPaymentInput struct {
	RecoveryID  int64
	RiderID     int64
	PayerUserID int64
	ClientIP    string
}

type claimRecoveryPaymentAttach struct {
	ClaimID        int64  `json:"claim_id"`
	RecoveryID     int64  `json:"recovery_id"`
	RecoveryTarget string `json:"recovery_target"`
}

func CreateMerchantClaimRecoveryPayment(ctx context.Context, store db.Store, paymentClient wechat.DirectPaymentClientInterface, input CreateMerchantClaimRecoveryPaymentInput) (ClaimRecoveryPaymentResult, error) {
	recoveryCtx, err := getClaimRecoveryContextByID(ctx, store, input.RecoveryID)
	if err != nil {
		return ClaimRecoveryPaymentResult{}, err
	}
	if recoveryCtx.MerchantID != input.MerchantID {
		return ClaimRecoveryPaymentResult{}, NewRequestError(http.StatusForbidden, errors.New("this claim does not belong to your merchant"))
	}
	recovery := claimRecoveryFromContextByID(recoveryCtx)
	if !recovery.RecoveryTarget.Valid || recovery.RecoveryTarget.String != "merchant" {
		return ClaimRecoveryPaymentResult{}, NewRequestError(http.StatusBadRequest, errors.New("recovery target mismatch"))
	}

	return createClaimRecoveryPayment(ctx, store, paymentClient, recoveryCtx, recovery, input.PayerUserID, input.ClientIP, "merchant")
}

func CreateRiderClaimRecoveryPayment(ctx context.Context, store db.Store, paymentClient wechat.DirectPaymentClientInterface, input CreateRiderClaimRecoveryPaymentInput) (ClaimRecoveryPaymentResult, error) {
	recoveryCtx, err := getClaimRecoveryContextByID(ctx, store, input.RecoveryID)
	if err != nil {
		return ClaimRecoveryPaymentResult{}, err
	}
	if !recoveryCtx.RiderID.Valid || recoveryCtx.RiderID.Int64 != input.RiderID {
		return ClaimRecoveryPaymentResult{}, NewRequestError(http.StatusForbidden, errors.New("this claim does not belong to your rider"))
	}
	recovery := claimRecoveryFromContextByID(recoveryCtx)
	if !recovery.RecoveryTarget.Valid || recovery.RecoveryTarget.String != "rider" {
		return ClaimRecoveryPaymentResult{}, NewRequestError(http.StatusBadRequest, errors.New("recovery target mismatch"))
	}

	return createClaimRecoveryPayment(ctx, store, paymentClient, recoveryCtx, recovery, input.PayerUserID, input.ClientIP, "rider")
}

func createClaimRecoveryPayment(ctx context.Context, store db.Store, paymentClient wechat.DirectPaymentClientInterface, recoveryCtx db.GetClaimRecoveryContextByIDRow, recovery db.ClaimRecovery, payerUserID int64, clientIP string, recoveryTarget string) (ClaimRecoveryPaymentResult, error) {
	if paymentClient == nil {
		return ClaimRecoveryPaymentResult{}, fmt.Errorf("payment client: not configured")
	}
	if !recoveryCtx.PaidAt.Valid {
		return ClaimRecoveryPaymentResult{}, NewRequestError(http.StatusBadRequest, errors.New("claim recovery cannot be paid before platform payout completes"))
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
		OrderID:               pgtype.Int8{Int64: recovery.OrderID, Valid: true},
		UserID:                payerUserID,
		PaymentType:           "miniprogram",
		PaymentChannel:        db.PaymentChannelDirect,
		RequiresProfitSharing: false,
		BusinessType:          businessTypeClaimRecovery,
		Amount:                recovery.RecoveryAmount,
		OutTradeNo:            outTradeNo,
		ExpiresAt:             pgtype.Timestamptz{Time: expiresAt, Valid: true},
		Attach:                pgtype.Text{String: attachText, Valid: true},
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

	wxResp, payParams, err := paymentClient.CreateJSAPIOrder(ctx, &wechatcontracts.DirectJSAPIOrderRequest{
		OutTradeNo:    outTradeNo,
		Description:   description,
		TotalAmount:   recovery.RecoveryAmount,
		PayerOpenID:   user.WechatOpenid,
		ExpireTime:    expiresAt,
		Attach:        attachText,
		PayerClientIP: clientIP,
	})
	if err != nil {
		if _, closeErr := store.UpdatePaymentOrderToClosed(ctx, paymentOrder.ID); closeErr != nil {
			log.Error().Err(closeErr).Int64("payment_order_id", paymentOrder.ID).Msg("failed to close claim recovery payment order after create rejection")
		} else {
			recordClaimRecoveryPaymentCommandRejected(ctx, store, paymentOrder, err)
		}
		if mapped := mapDirectJSAPIOrderCreateError(err); mapped != nil {
			return ClaimRecoveryPaymentResult{}, mapped
		}
		return ClaimRecoveryPaymentResult{}, fmt.Errorf("wechat pay: %w", err)
	}

	updatedPaymentOrder, err := store.UpdatePaymentOrderPrepayId(ctx, db.UpdatePaymentOrderPrepayIdParams{
		ID:       paymentOrder.ID,
		PrepayID: pgtype.Text{String: wxResp.PrepayID, Valid: true},
	})
	if err != nil {
		markClaimRecoveryPaymentOrderFailedForCleanup(ctx, store, paymentOrder.ID)
		if closeErr := paymentClient.CloseOrder(ctx, outTradeNo); closeErr != nil {
			log.Warn().Err(closeErr).Str("out_trade_no", outTradeNo).Msg("close claim recovery wechat order after prepay update failure")
		}
		return ClaimRecoveryPaymentResult{}, fmt.Errorf("update claim recovery prepay id: %w", err)
	}
	recordClaimRecoveryPaymentCommandAccepted(ctx, store, paymentOrder, wxResp.PrepayID)
	if err := db.WriteClaimRecoveryEvent(ctx, store, recovery, db.ClaimRecoveryEventTypePaymentStarted, map[string]any{
		"claim_id":         recovery.ClaimID,
		"payment_order_id": updatedPaymentOrder.ID,
		"recovery_target":  recovery.RecoveryTarget.String,
		"recovery_amount":  recovery.RecoveryAmount,
		"status":           recovery.Status,
	}); err != nil {
		return ClaimRecoveryPaymentResult{}, fmt.Errorf("write claim recovery payment_started event: %w", err)
	}

	return ClaimRecoveryPaymentResult{Recovery: recovery, PaymentOrder: updatedPaymentOrder, PayParams: payParams}, nil
}

func markClaimRecoveryPaymentOrderFailedForCleanup(ctx context.Context, store db.Store, paymentOrderID int64) {
	if _, err := store.UpdatePaymentOrderToFailed(ctx, paymentOrderID); err != nil {
		log.Error().Err(err).
			Int64("payment_order_id", paymentOrderID).
			Msg("failed to mark claim recovery payment order failed after prepay update failure")
	}
}

func recordClaimRecoveryPaymentCommandAccepted(ctx context.Context, store db.Store, paymentOrder db.PaymentOrder, prepayID string) {
	paymentCommandSvc := NewPaymentCommandService(store)
	_, err := paymentCommandSvc.RecordExternalPaymentCommand(ctx, dbClaimRecoveryPaymentCommandInput(
		paymentOrder,
		db.ExternalPaymentCommandStatusAccepted,
		stringPtrIfNotEmpty(prepayID),
		nil,
		nil,
		claimRecoveryPaymentCommandSnapshot(map[string]string{
			"out_trade_no": paymentOrder.OutTradeNo,
			"prepay_id":    prepayID,
		}),
	))
	if err != nil {
		log.Error().Err(err).
			Int64("payment_order_id", paymentOrder.ID).
			Str("out_trade_no", paymentOrder.OutTradeNo).
			Msg("record claim recovery payment command accepted failed")
	}
}

func recordClaimRecoveryPaymentCommandRejected(ctx context.Context, store db.Store, paymentOrder db.PaymentOrder, paymentErr error) {
	paymentCommandSvc := NewPaymentCommandService(store)
	errorCode, errorMessage := directPaymentCommandErrorFields(paymentErr)
	_, err := paymentCommandSvc.RecordExternalPaymentCommand(ctx, dbClaimRecoveryPaymentCommandInput(
		paymentOrder,
		db.ExternalPaymentCommandStatusRejected,
		nil,
		errorCode,
		errorMessage,
		claimRecoveryPaymentCommandSnapshot(map[string]string{
			"out_trade_no":  paymentOrder.OutTradeNo,
			"error_code":    stringValue(errorCode),
			"error_message": stringValue(errorMessage),
		}),
	))
	if err != nil {
		log.Error().Err(err).
			Int64("payment_order_id", paymentOrder.ID).
			Str("out_trade_no", paymentOrder.OutTradeNo).
			Msg("record claim recovery payment command rejected failed")
	}
}

func dbClaimRecoveryPaymentCommandInput(
	paymentOrder db.PaymentOrder,
	commandStatus string,
	externalSecondaryKey *string,
	lastErrorCode *string,
	lastErrorMessage *string,
	responseSnapshot []byte,
) RecordExternalPaymentCommandInput {
	businessObjectType := claimRecoveryPaymentOrderObjectType
	businessObjectID := paymentOrder.ID
	return RecordExternalPaymentCommandInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelDirect,
		Capability:           db.ExternalPaymentCapabilityDirectJSAPIPayment,
		CommandType:          db.ExternalPaymentCommandTypeCreatePayment,
		BusinessOwner:        db.ExternalPaymentBusinessOwnerClaimRecovery,
		BusinessObjectType:   &businessObjectType,
		BusinessObjectID:     &businessObjectID,
		ExternalObjectType:   db.ExternalPaymentObjectPayment,
		ExternalObjectKey:    paymentOrder.OutTradeNo,
		ExternalSecondaryKey: externalSecondaryKey,
		CommandStatus:        commandStatus,
		LastErrorCode:        lastErrorCode,
		LastErrorMessage:     lastErrorMessage,
		ResponseSnapshot:     responseSnapshot,
	}
}

func directPaymentCommandErrorFields(err error) (*string, *string) {
	loggableErr := LoggableError(err)
	var wxErr *wechat.WechatPayError
	if errors.As(loggableErr, &wxErr) {
		return stringPtrIfNotEmpty(wxErr.Code), stringPtrIfNotEmpty(wxErr.Message)
	}
	if loggableErr == nil {
		return nil, nil
	}
	return nil, stringPtrIfNotEmpty(loggableErr.Error())
}

func claimRecoveryPaymentCommandSnapshot(values map[string]string) []byte {
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

func reuseClaimRecoveryPayment(paymentClient wechat.DirectPaymentClientInterface, recovery db.ClaimRecovery, paymentOrder db.PaymentOrder) (ClaimRecoveryPaymentResult, error) {
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

func closeExpiredClaimRecoveryPayment(ctx context.Context, store db.Store, paymentClient wechat.DirectPaymentClientInterface, paymentOrder db.PaymentOrder) error {
	if paymentOrder.OutTradeNo != "" && paymentClient != nil {
		if err := paymentClient.CloseOrder(ctx, paymentOrder.OutTradeNo); err != nil {
			return fmt.Errorf("close expired claim recovery wechat order: %w", err)
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

func regenerateClaimRecoveryPayParams(paymentClient wechat.DirectPaymentClientInterface, paymentOrder db.PaymentOrder) (*wechat.JSAPIPayParams, error) {
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
