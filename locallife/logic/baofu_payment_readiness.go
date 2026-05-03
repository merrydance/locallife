package logic

import (
	"context"
	"errors"
	"net/http"

	db "github.com/merrydance/locallife/db/sqlc"
)

var (
	errMerchantBaofuPaymentAccountMissing       = errors.New("商户结算账户未开通，暂不能创建支付订单")
	errMerchantBaofuPaymentWechatChannelPending = errors.New("商户微信渠道待报备，暂不能创建微信生态支付订单")
)

func ensureMerchantBaofuReadyForPayment(ctx context.Context, store db.Store, merchantID int64) error {
	binding, err := store.GetBaofuAccountBindingByOwner(ctx, db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: db.BaofuAccountOwnerTypeMerchant,
		OwnerID:   merchantID,
	})
	service := NewBaofuAccountService(nil, nil)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return NewRequestError(http.StatusBadRequest, errMerchantBaofuPaymentAccountMissing)
		}
		return err
	}

	readiness := service.ReadinessFromBinding(binding, true, true)
	if readiness.PaymentReady {
		return nil
	}
	if readiness.State == BaofuOnboardingStateWechatChannelPending {
		return NewRequestError(http.StatusBadRequest, errMerchantBaofuPaymentWechatChannelPending)
	}
	return NewRequestError(http.StatusBadRequest, errMerchantBaofuPaymentAccountMissing)
}
