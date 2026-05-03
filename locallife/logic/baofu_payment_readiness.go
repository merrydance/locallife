package logic

import (
	"context"
	"errors"
	"fmt"
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

func ensureCombinedPaymentMerchantsBaofuReady(ctx context.Context, store db.Store, userID int64, orderIDs []int64) error {
	merchantIDs := make([]int64, 0, len(orderIDs))
	seenMerchantIDs := make(map[int64]struct{}, len(orderIDs))

	for _, orderID := range orderIDs {
		order, err := store.GetOrder(ctx, orderID)
		if err != nil {
			if errors.Is(err, db.ErrRecordNotFound) {
				return NewRequestError(http.StatusBadRequest, errors.New("订单已不在待支付状态，请刷新页面确认"))
			}
			return fmt.Errorf("get order %d for baofu readiness: %w", orderID, err)
		}
		if order.UserID != userID {
			return NewRequestError(http.StatusForbidden, errors.New("订单不属于当前用户"))
		}
		if _, exists := seenMerchantIDs[order.MerchantID]; exists {
			continue
		}
		seenMerchantIDs[order.MerchantID] = struct{}{}
		merchantIDs = append(merchantIDs, order.MerchantID)
	}

	for _, merchantID := range merchantIDs {
		if err := ensureMerchantBaofuReadyForPayment(ctx, store, merchantID); err != nil {
			return err
		}
	}
	return nil
}
