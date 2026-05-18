package api

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
)

var (
	errMerchantBaofuAccountMissing       = errors.New("商户结算账户未开通，暂不能开业接收分账订单")
	errMerchantBaofuWechatChannelPending = errors.New("商户微信支付通道待开通，暂不能开业接收微信生态支付订单")
)

func (server *Server) ensureMerchantBaofuPaymentReady(ctx context.Context, merchant db.Merchant) error {
	readiness, err := server.getMerchantBaofuSettlementReadiness(ctx, merchant)
	if err != nil {
		return err
	}
	if readiness.PaymentReady {
		return nil
	}
	if readiness.State == logic.BaofuOnboardingStateWechatChannelPending {
		return errMerchantBaofuWechatChannelPending
	}
	return errMerchantBaofuAccountMissing
}

func (server *Server) getMerchantBaofuSettlementReadiness(ctx context.Context, merchant db.Merchant) (logic.BaofuAccountReadiness, error) {
	binding, err := server.store.GetBaofuAccountBindingByOwner(ctx, db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: db.BaofuAccountOwnerTypeMerchant,
		OwnerID:   merchant.ID,
	})
	service := logic.NewBaofuAccountService(nil, nil)
	if err != nil {
		if isNotFoundError(err) {
			return service.ReadinessFromBinding(db.BaofuAccountBinding{}, false), nil
		}
		return logic.BaofuAccountReadiness{}, err
	}
	report, err := server.store.GetBaofuMerchantReportByOwner(ctx, db.GetBaofuMerchantReportByOwnerParams{
		OwnerType:  db.BaofuAccountOwnerTypeMerchant,
		OwnerID:    merchant.ID,
		ReportType: db.BaofuMerchantReportTypeWechat,
	})
	if err != nil {
		if isNotFoundError(err) {
			report = db.BaofuMerchantReport{
				OwnerType:       db.BaofuAccountOwnerTypeMerchant,
				OwnerID:         merchant.ID,
				ReportType:      db.BaofuMerchantReportTypeWechat,
				ReportState:     db.BaofuMerchantReportStateProcessing,
				AppletAuthState: db.BaofuMerchantReportAppletAuthStatePending,
				SubMchID:        pgtype.Text{},
			}
		} else {
			return logic.BaofuAccountReadiness{}, err
		}
	}
	return logic.ReadinessFromBaofuBindingAndMerchantReport(binding, report), nil
}
