package logic

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	db "github.com/merrydance/locallife/db/sqlc"
)

var (
	errMerchantBaofuPaymentAccountMissing       = errors.New("商户结算账户未开通，暂不能创建支付订单")
	errMerchantBaofuPaymentWechatChannelPending = errors.New("商户微信支付通道待开通，暂不能创建微信生态支付订单")
)

func merchantBaofuReadinessForPayment(ctx context.Context, store db.Store, merchantID int64) (BaofuAccountReadiness, error) {
	binding, err := store.GetBaofuAccountBindingByOwner(ctx, db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: db.BaofuAccountOwnerTypeMerchant,
		OwnerID:   merchantID,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return BaofuAccountReadiness{}, NewRequestError(http.StatusBadRequest, errMerchantBaofuPaymentAccountMissing)
		}
		return BaofuAccountReadiness{}, err
	}
	report, err := store.GetBaofuMerchantReportByOwner(ctx, db.GetBaofuMerchantReportByOwnerParams{
		OwnerType:  db.BaofuAccountOwnerTypeMerchant,
		OwnerID:    merchantID,
		ReportType: db.BaofuMerchantReportTypeWechat,
	})
	if err != nil && !errors.Is(err, db.ErrRecordNotFound) {
		return BaofuAccountReadiness{}, err
	}
	readiness := ReadinessFromBaofuBindingAndMerchantReport(binding, report)
	if readiness.PaymentReady {
		return readiness, nil
	}
	if readiness.State == BaofuOnboardingStateWechatChannelPending {
		return readiness, NewRequestError(http.StatusBadRequest, errMerchantBaofuPaymentWechatChannelPending)
	}
	return readiness, NewRequestError(http.StatusBadRequest, errMerchantBaofuPaymentAccountMissing)
}

func ensureMerchantBaofuReadyForPayment(ctx context.Context, store db.Store, merchantID int64) error {
	_, err := merchantBaofuReadinessForPayment(ctx, store, merchantID)
	return err
}

func ReadinessFromBaofuBindingAndMerchantReport(binding db.BaofuAccountBinding, report db.BaofuMerchantReport) BaofuAccountReadiness {
	service := NewBaofuAccountService(nil, nil)
	accountReadiness := service.ReadinessFromBinding(binding, true)
	if !accountReadiness.PaymentReady {
		return accountReadiness
	}
	subMchID := strings.TrimSpace(report.SubMchID.String)
	if strings.TrimSpace(report.ReportState) != db.BaofuMerchantReportStateSucceeded || subMchID == "" || strings.TrimSpace(report.AppletAuthState) != db.BaofuMerchantReportAppletAuthStateSucceeded {
		return BaofuAccountReadiness{State: BaofuOnboardingStateWechatChannelPending, Label: baofuOnboardingStateLabel(BaofuOnboardingStateWechatChannelPending), PaymentReady: false, SubMchID: subMchID}
	}
	return BaofuAccountReadiness{State: BaofuOnboardingStateReady, Label: baofuOnboardingStateLabel(BaofuOnboardingStateReady), PaymentReady: true, SubMchID: subMchID}
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
