package logic

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	"github.com/rs/zerolog/log"
)

func (s *BaofuAccountOnboardingService) ensureVerifyFeePayment(ctx context.Context, flow db.BaofuAccountOpeningFlow, userID int64, clientIP string, cfg BaofuAccountOnboardingConfig) (db.PaymentOrder, *wechat.JSAPIPayParams, error) {
	if s.directPaymentClient == nil {
		return db.PaymentOrder{}, nil, ErrBaofuAccountOnboardingNotConfigured
	}
	attach := baofuVerifyFeeAttach(flow.OwnerType, flow.OwnerID)
	if existing, err := s.store.GetReusableBaofuVerifyFeePayment(ctx, db.GetReusableBaofuVerifyFeePaymentParams{Attach: pgtype.Text{String: attach, Valid: true}, UserID: userID, Amount: cfg.VerifyFeeFen}); err == nil {
		if strings.TrimSpace(existing.Status) == "pending" && existing.PrepayID.Valid {
			payParams, signErr := s.directPaymentClient.GenerateJSAPIPayParams(existing.PrepayID.String)
			if signErr != nil {
				return db.PaymentOrder{}, nil, signErr
			}
			return existing, payParams, nil
		}
		return existing, nil, nil
	} else if !errors.Is(err, db.ErrRecordNotFound) {
		return db.PaymentOrder{}, nil, err
	}

	user, err := s.store.GetUser(ctx, userID)
	if err != nil {
		return db.PaymentOrder{}, nil, fmt.Errorf("get verify fee payer: %w", err)
	}
	if strings.TrimSpace(user.WechatOpenid) == "" {
		return db.PaymentOrder{}, nil, NewRequestError(http.StatusBadRequest, errors.New("wechat openid not found"))
	}
	outTradeNo, err := util.GenerateOutTradeNo("BFV")
	if err != nil {
		return db.PaymentOrder{}, nil, err
	}
	expiresAt := s.now().Add(baofuOpeningPaymentTTL)
	payment, err := s.store.CreatePaymentOrder(ctx, db.CreatePaymentOrderParams{
		UserID:                userID,
		PaymentType:           "miniprogram",
		PaymentChannel:        db.PaymentChannelDirect,
		RequiresProfitSharing: false,
		BusinessType:          db.PaymentBusinessTypeBaofuAccountVerifyFee,
		Amount:                cfg.VerifyFeeFen,
		OutTradeNo:            outTradeNo,
		ExpiresAt:             pgtype.Timestamptz{Time: expiresAt, Valid: true},
		Attach:                pgtype.Text{String: attach, Valid: true},
	})
	if err != nil {
		return db.PaymentOrder{}, nil, err
	}
	resp, payParams, err := s.directPaymentClient.CreateJSAPIOrder(ctx, &wechatcontracts.DirectJSAPIOrderRequest{
		Description:   "宝付开户核验费",
		OutTradeNo:    outTradeNo,
		ExpireTime:    expiresAt,
		Attach:        attach,
		TotalAmount:   cfg.VerifyFeeFen,
		PayerOpenID:   user.WechatOpenid,
		PayerClientIP: clientIP,
		ProfitSharing: false,
	})
	if err != nil {
		if _, closeErr := s.store.UpdatePaymentOrderToClosed(ctx, payment.ID); closeErr != nil {
			log.Error().Err(closeErr).Int64("payment_order_id", payment.ID).Msg("failed to close baofu verify fee payment after create rejection")
		}
		if mapped := mapDirectJSAPIOrderCreateError(err); mapped != nil {
			return db.PaymentOrder{}, nil, mapped
		}
		return db.PaymentOrder{}, nil, fmt.Errorf("create baofu verify fee payment: %w", err)
	}
	if resp == nil || strings.TrimSpace(resp.PrepayID) == "" {
		if _, closeErr := s.store.UpdatePaymentOrderToClosed(ctx, payment.ID); closeErr != nil {
			log.Error().Err(closeErr).Int64("payment_order_id", payment.ID).Msg("failed to close baofu verify fee payment after empty prepay id")
		}
		return db.PaymentOrder{}, nil, errors.New("create baofu verify fee payment: empty prepay id")
	}
	updated, err := s.store.UpdatePaymentOrderPrepayId(ctx, db.UpdatePaymentOrderPrepayIdParams{ID: payment.ID, PrepayID: pgtype.Text{String: resp.PrepayID, Valid: true}})
	if err != nil {
		return db.PaymentOrder{}, nil, err
	}
	_, _ = s.store.CreateExternalPaymentCommand(ctx, db.CreateExternalPaymentCommandParams{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelDirect,
		Capability:           db.ExternalPaymentCapabilityDirectJSAPIPayment,
		CommandType:          db.ExternalPaymentCommandTypeCreatePayment,
		BusinessOwner:        db.ExternalPaymentBusinessOwnerBaofuVerifyFee,
		BusinessObjectType:   pgtype.Text{String: "payment_order", Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: updated.ID, Valid: true},
		ExternalObjectType:   db.ExternalPaymentObjectPayment,
		ExternalObjectKey:    updated.OutTradeNo,
		ExternalSecondaryKey: pgtype.Text{String: resp.PrepayID, Valid: true},
		CommandStatus:        db.ExternalPaymentCommandStatusAccepted,
		SubmittedAt:          s.now().UTC(),
		ResponseSnapshot:     baofuOpeningSnapshot(map[string]any{"out_trade_no": updated.OutTradeNo, "prepay_id": resp.PrepayID}),
	})
	return updated, payParams, nil
}
