package baofu

import (
	"errors"
	"net/url"
	"strings"
	"time"
)

const (
	BaofuEnvironmentSandbox    = "sandbox"
	BaofuEnvironmentProduction = "production"

	SandboxAccountGatewayBaseURL    = "https://vgw.baofoo.com/union-gw/api"
	ProductionAccountGatewayBaseURL = "https://public.baofu.com/union-gw/api"

	SandboxAggregatePayBaseURL          = "https://mch-juhe.baofoo.com/api"
	ProductionAggregatePayBaseURL       = "https://juhe.baofoo.com/api"
	ProductionAggregatePayBackupBaseURL = "https://juhe-backup.baofoo.com/api"

	SandboxMerchantReportBaseURL    = "https://mch-juhe.baofoo.com/mch-service/api"
	ProductionMerchantReportBaseURL = "https://juhe.baofoo.com/mch-service/api"
)

type Config struct {
	// BaseURL is kept only for backward compatibility with older local tests.
	// New code must use the endpoint-specific fields below.
	BaseURL                   string
	Environment               string
	AccountGatewayBaseURL     string
	AggregatePayBaseURL       string
	AggregatePayBackupBaseURL string
	MerchantReportBaseURL     string
	CollectMerchantID         string
	CollectTerminalID         string
	PayoutMerchantID          string
	PayoutTerminalID          string
	AppID                     string
	PrivateKeyPEM             string
	BaofuPublicKeyPEM         string
	SignSerialNo              string
	EncryptionSerialNo        string
	NotifyBaseURL             string
	Timeout                   time.Duration
}

func (c Config) Validate() error {
	cfg := c.Normalized()
	if cfg.CollectMerchantID == "" {
		return errors.New("baofu collect merchant id is required")
	}
	if cfg.PayoutMerchantID == "" {
		return errors.New("baofu payout merchant id is required")
	}
	if cfg.CollectMerchantID == cfg.PayoutMerchantID {
		return errors.New("baofu collect merchant id and payout merchant id must be different")
	}
	if cfg.CollectTerminalID == "" {
		return errors.New("baofu collect terminal id is required")
	}
	if cfg.PayoutTerminalID == "" {
		return errors.New("baofu payout terminal id is required")
	}
	if cfg.PrivateKeyPEM == "" {
		return errors.New("baofu private key pem is required")
	}
	if cfg.BaofuPublicKeyPEM == "" {
		return errors.New("baofu public key pem is required")
	}
	if cfg.SignSerialNo == "" {
		return errors.New("baofu sign serial no is required")
	}
	if len(cfg.SignSerialNo) > 10 {
		return errors.New("baofu sign serial no must be at most 10 characters")
	}
	if cfg.EncryptionSerialNo == "" {
		return errors.New("baofu encryption serial no is required")
	}
	if len(cfg.EncryptionSerialNo) > 10 {
		return errors.New("baofu encryption serial no must be at most 10 characters")
	}
	if err := validateOfficialEndpoint("baofu account gateway base url", cfg.AccountGatewayBaseURL, officialAccountGatewayBaseURLs()); err != nil {
		return err
	}
	if err := validateOfficialEndpoint("baofu aggregate pay base url", cfg.AggregatePayBaseURL, officialAggregatePayBaseURLs()); err != nil {
		return err
	}
	if cfg.AggregatePayBackupBaseURL != "" {
		if err := validateOfficialEndpoint("baofu aggregate pay backup base url", cfg.AggregatePayBackupBaseURL, officialAggregatePayBaseURLs()); err != nil {
			return err
		}
	}
	if err := validateOfficialEndpoint("baofu merchant report base url", cfg.MerchantReportBaseURL, officialMerchantReportBaseURLs()); err != nil {
		return err
	}
	return validateHTTPSURL("baofu notify base url", cfg.NotifyBaseURL)
}

func (c Config) Normalized() Config {
	c.Environment = strings.ToLower(strings.TrimSpace(c.Environment))
	if c.Environment == "" {
		c.Environment = BaofuEnvironmentSandbox
	}
	c.BaseURL = strings.TrimSpace(c.BaseURL)
	c.AccountGatewayBaseURL = strings.TrimRight(strings.TrimSpace(c.AccountGatewayBaseURL), "/")
	c.AggregatePayBaseURL = strings.TrimRight(strings.TrimSpace(c.AggregatePayBaseURL), "/")
	c.AggregatePayBackupBaseURL = strings.TrimRight(strings.TrimSpace(c.AggregatePayBackupBaseURL), "/")
	c.MerchantReportBaseURL = strings.TrimRight(strings.TrimSpace(c.MerchantReportBaseURL), "/")
	switch c.Environment {
	case BaofuEnvironmentProduction:
		if c.AccountGatewayBaseURL == "" {
			c.AccountGatewayBaseURL = ProductionAccountGatewayBaseURL
		}
		if c.AggregatePayBaseURL == "" {
			c.AggregatePayBaseURL = ProductionAggregatePayBaseURL
		}
		if c.AggregatePayBackupBaseURL == "" {
			c.AggregatePayBackupBaseURL = ProductionAggregatePayBackupBaseURL
		}
		if c.MerchantReportBaseURL == "" {
			c.MerchantReportBaseURL = ProductionMerchantReportBaseURL
		}
	default:
		if c.AccountGatewayBaseURL == "" {
			c.AccountGatewayBaseURL = SandboxAccountGatewayBaseURL
		}
		if c.AggregatePayBaseURL == "" {
			c.AggregatePayBaseURL = SandboxAggregatePayBaseURL
		}
		if c.MerchantReportBaseURL == "" {
			c.MerchantReportBaseURL = SandboxMerchantReportBaseURL
		}
	}
	c.CollectMerchantID = strings.TrimSpace(c.CollectMerchantID)
	c.CollectTerminalID = strings.TrimSpace(c.CollectTerminalID)
	c.PayoutMerchantID = strings.TrimSpace(c.PayoutMerchantID)
	c.PayoutTerminalID = strings.TrimSpace(c.PayoutTerminalID)
	c.AppID = strings.TrimSpace(c.AppID)
	c.PrivateKeyPEM = strings.TrimSpace(c.PrivateKeyPEM)
	c.BaofuPublicKeyPEM = strings.TrimSpace(c.BaofuPublicKeyPEM)
	c.SignSerialNo = strings.TrimSpace(c.SignSerialNo)
	c.EncryptionSerialNo = strings.TrimSpace(c.EncryptionSerialNo)
	c.NotifyBaseURL = strings.TrimSpace(c.NotifyBaseURL)
	if c.Timeout == 0 {
		c.Timeout = 30 * time.Second
	}
	return c
}

func officialAccountGatewayBaseURLs() map[string]struct{} {
	return map[string]struct{}{
		SandboxAccountGatewayBaseURL:    {},
		ProductionAccountGatewayBaseURL: {},
	}
}

func officialAggregatePayBaseURLs() map[string]struct{} {
	return map[string]struct{}{
		SandboxAggregatePayBaseURL:          {},
		ProductionAggregatePayBaseURL:       {},
		ProductionAggregatePayBackupBaseURL: {},
	}
}

func officialMerchantReportBaseURLs() map[string]struct{} {
	return map[string]struct{}{
		SandboxMerchantReportBaseURL:    {},
		ProductionMerchantReportBaseURL: {},
	}
}

func validateOfficialEndpoint(field, raw string, allowed map[string]struct{}) error {
	if strings.TrimSpace(raw) == "" {
		return errors.New(field + " is required")
	}
	if err := validateHTTPSURL(field, raw); err != nil {
		return err
	}
	normalized := strings.TrimRight(strings.TrimSpace(raw), "/")
	if _, ok := allowed[normalized]; !ok {
		return errors.New(field + " must be an official endpoint")
	}
	return nil
}

func validateHTTPSURL(field, raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return errors.New(field + " is invalid")
	}
	if parsed.Scheme != "https" {
		return errors.New(field + " must use https")
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return errors.New(field + " host is required")
	}
	return nil
}
