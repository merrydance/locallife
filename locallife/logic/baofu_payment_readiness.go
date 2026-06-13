package logic

import (
	"context"
	"errors"
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
