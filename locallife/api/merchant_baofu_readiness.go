package api

import (
	"context"
	"errors"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
)

var (
	errMerchantBaofuAccountMissing       = errors.New("商户结算账户未开通，暂不能开业接收分账订单")
	errMerchantBaofuWechatChannelPending = errors.New("商户微信渠道待报备，暂不能开业接收微信生态支付订单")
)

func (server *Server) ensureMerchantBaofuPaymentReady(ctx context.Context, merchant db.Merchant) error {
	binding, err := server.store.GetBaofuAccountBindingByOwner(ctx, db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: db.BaofuAccountOwnerTypeMerchant,
		OwnerID:   merchant.ID,
	})
	service := logic.NewBaofuAccountService(nil, nil)
	if err != nil {
		if isNotFoundError(err) {
			return errMerchantBaofuAccountMissing
		}
		return err
	}

	readiness := service.ReadinessFromBinding(binding, true, true)
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
			return service.ReadinessFromBinding(db.BaofuAccountBinding{}, false, true), nil
		}
		return logic.BaofuAccountReadiness{}, err
	}
	return service.ReadinessFromBinding(binding, true, true), nil
}
