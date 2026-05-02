package ordinaryserviceprovider

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api.mch.weixin.qq.com"

type Config struct {
	ServiceProviderAppID       string
	ServiceProviderMchID       string
	ServiceProviderMchName     string
	CertificateSerialNumber    string
	PrivateKeyPath             string
	APIV3Key                   string
	PlatformPublicKeyPath      string
	PlatformPublicKeyID        string
	BaseURL                    string
	PaymentNotifyURL           string
	CombineNotifyURL           string
	RefundNotifyURL            string
	ProfitSharingNotifyURL     string
	MerchantViolationNotifyURL string
	HTTPTimeout                time.Duration
}

func (cfg Config) Validate() error {
	cfg = cfg.Normalized()
	requiredFields := map[string]string{
		"service_provider_appid":        cfg.ServiceProviderAppID,
		"service_provider_mchid":        cfg.ServiceProviderMchID,
		"certificate_serial_number":     cfg.CertificateSerialNumber,
		"private_key_path":              cfg.PrivateKeyPath,
		"api_v3_key":                    cfg.APIV3Key,
		"platform_public_key_path":      cfg.PlatformPublicKeyPath,
		"platform_public_key_id":        cfg.PlatformPublicKeyID,
		"payment_notify_url":            cfg.PaymentNotifyURL,
		"combine_notify_url":            cfg.CombineNotifyURL,
		"refund_notify_url":             cfg.RefundNotifyURL,
		"profit_sharing_notify_url":     cfg.ProfitSharingNotifyURL,
		"merchant_violation_notify_url": cfg.MerchantViolationNotifyURL,
	}
	for field, value := range requiredFields {
		if value == "" {
			return fmt.Errorf("%s is required", field)
		}
	}
	if len(cfg.APIV3Key) != 32 {
		return fmt.Errorf("api_v3_key must be 32 bytes")
	}
	if err := validateHTTPSURL("base_url", cfg.BaseURL); err != nil {
		return err
	}
	for field, value := range map[string]string{
		"payment_notify_url":            cfg.PaymentNotifyURL,
		"combine_notify_url":            cfg.CombineNotifyURL,
		"refund_notify_url":             cfg.RefundNotifyURL,
		"profit_sharing_notify_url":     cfg.ProfitSharingNotifyURL,
		"merchant_violation_notify_url": cfg.MerchantViolationNotifyURL,
	} {
		if err := validateHTTPSURL(field, value); err != nil {
			return err
		}
	}
	return nil
}

func (cfg Config) Normalized() Config {
	cfg.ServiceProviderAppID = strings.TrimSpace(cfg.ServiceProviderAppID)
	cfg.ServiceProviderMchID = strings.TrimSpace(cfg.ServiceProviderMchID)
	cfg.ServiceProviderMchName = strings.TrimSpace(cfg.ServiceProviderMchName)
	cfg.CertificateSerialNumber = strings.TrimSpace(cfg.CertificateSerialNumber)
	cfg.PrivateKeyPath = strings.TrimSpace(cfg.PrivateKeyPath)
	cfg.APIV3Key = strings.TrimSpace(cfg.APIV3Key)
	cfg.PlatformPublicKeyPath = strings.TrimSpace(cfg.PlatformPublicKeyPath)
	cfg.PlatformPublicKeyID = strings.TrimSpace(cfg.PlatformPublicKeyID)
	cfg.BaseURL = strings.TrimSpace(cfg.BaseURL)
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	cfg.PaymentNotifyURL = strings.TrimSpace(cfg.PaymentNotifyURL)
	cfg.CombineNotifyURL = strings.TrimSpace(cfg.CombineNotifyURL)
	cfg.RefundNotifyURL = strings.TrimSpace(cfg.RefundNotifyURL)
	cfg.ProfitSharingNotifyURL = strings.TrimSpace(cfg.ProfitSharingNotifyURL)
	cfg.MerchantViolationNotifyURL = strings.TrimSpace(cfg.MerchantViolationNotifyURL)
	if cfg.HTTPTimeout == 0 {
		cfg.HTTPTimeout = 30 * time.Second
	}
	return cfg
}

func validateHTTPSURL(field, value string) error {
	parsedURL, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return fmt.Errorf("%s is invalid: %w", field, err)
	}
	if parsedURL.Scheme != "https" {
		return fmt.Errorf("%s must use https", field)
	}
	if strings.TrimSpace(parsedURL.Host) == "" {
		return fmt.Errorf("%s host is required", field)
	}
	return nil
}
