package wechatruntime

import (
	"github.com/merrydance/locallife/util"
	ordinaryserviceprovider "github.com/merrydance/locallife/wechat/ordinaryserviceprovider"
)

func BuildOrdinaryServiceProviderClient(config util.Config) (*ordinaryserviceprovider.Client, error) {
	if !config.HasWechatOrdinaryServiceProviderRuntimeConfig() {
		return nil, nil
	}
	if err := config.ValidateWechatOrdinaryServiceProviderConfig(); err != nil {
		return nil, err
	}
	return ordinaryserviceprovider.NewOrdinaryServiceProviderClient(ordinaryserviceprovider.Config{
		ServiceProviderAppID:       config.WechatOrdinarySpAppID,
		ServiceProviderMchID:       config.WechatOrdinarySpMchID,
		ServiceProviderMchName:     config.WechatOrdinarySpName,
		CertificateSerialNumber:    config.WechatOrdinarySpSerialNumber,
		PrivateKeyPath:             config.WechatOrdinarySpPrivateKeyPath,
		APIV3Key:                   config.WechatOrdinarySpAPIV3Key,
		PlatformPublicKeyPath:      config.WechatOrdinarySpPlatformPublicKeyPath,
		PlatformPublicKeyID:        config.WechatOrdinarySpPlatformPublicKeyID,
		PaymentNotifyURL:           config.EffectiveWechatOrdinaryPaymentNotifyURL(),
		CombineNotifyURL:           config.EffectiveWechatOrdinaryCombineNotifyURL(),
		RefundNotifyURL:            config.EffectiveWechatOrdinaryRefundNotifyURL(),
		ProfitSharingNotifyURL:     config.EffectiveWechatOrdinaryProfitSharingNotifyURL(),
		MerchantViolationNotifyURL: config.EffectiveWechatOrdinaryViolationNotifyURL(),
		HTTPTimeout:                config.WechatPayHTTPTimeout,
	})
}
