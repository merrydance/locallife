package wechatruntime

import (
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/wechat"
)

func BuildEcommerceClient(config util.Config) (wechat.EcommerceClientInterface, error) {
	if !config.HasWechatEcommerceRuntimeConfig() {
		return nil, nil
	}

	if err := config.ValidateWechatEcommerceConfig(); err != nil {
		return nil, err
	}

	return wechat.NewEcommerceClient(wechat.EcommerceClientConfig{
		DirectPaymentClientConfig: wechat.DirectPaymentClientConfig{
			MchID:                 config.WechatEcommerceSpMchID,
			AppID:                 config.WechatEcommerceSpAppID,
			SerialNumber:          config.WechatEcommerceSpSerialNumber,
			HTTPTimeout:           config.WechatPayHTTPTimeout,
			PrivateKeyPath:        config.WechatEcommerceSpPrivateKeyPath,
			APIV3Key:              config.WechatEcommerceSpAPIV3Key,
			NotifyURL:             config.EffectiveWechatEcommercePaymentNotifyURL(),
			RefundNotifyURL:       config.EffectiveWechatEcommerceRefundNotifyURL(),
			PlatformPublicKeyPath: config.WechatEcommerceSpPlatformPublicKeyPath,
			PlatformPublicKeyID:   config.WechatEcommerceSpPlatformPublicKeyID,
		},
		SpMchID:            config.WechatEcommerceSpMchID,
		SpAppID:            config.WechatEcommerceSpAppID,
		SpMchName:          config.WechatEcommerceSpName,
		PartnerNotifyURL:   config.EffectiveWechatEcommercePaymentNotifyURL(),
		CombineNotifyURL:   config.EffectiveWechatEcommerceCombineNotifyURL(),
		WithdrawNotifyURL:  config.EffectiveWechatEcommerceWithdrawNotifyURL(),
		ViolationNotifyURL: config.EffectiveWechatEcommerceViolationNotifyURL(),
	})
}
