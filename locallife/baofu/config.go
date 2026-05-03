package baofu

import (
	"errors"
	"net/url"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api.baofoo.com"

type Config struct {
	BaseURL           string
	CollectMerchantID string
	CollectTerminalID string
	PayoutMerchantID  string
	PayoutTerminalID  string
	AppID             string
	PrivateKeyPEM     string
	BaofuPublicKeyPEM string
	AESKey            string
	NotifyBaseURL     string
	Timeout           time.Duration
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
	if len(cfg.AESKey) != 16 && len(cfg.AESKey) != 24 && len(cfg.AESKey) != 32 {
		return errors.New("baofu aes key must be 16, 24, or 32 bytes")
	}
	if err := validateHTTPSURL("baofu base url", cfg.BaseURL); err != nil {
		return err
	}
	return validateHTTPSURL("baofu notify base url", cfg.NotifyBaseURL)
}

func (c Config) Normalized() Config {
	c.BaseURL = strings.TrimSpace(c.BaseURL)
	if c.BaseURL == "" {
		c.BaseURL = DefaultBaseURL
	}
	c.CollectMerchantID = strings.TrimSpace(c.CollectMerchantID)
	c.CollectTerminalID = strings.TrimSpace(c.CollectTerminalID)
	c.PayoutMerchantID = strings.TrimSpace(c.PayoutMerchantID)
	c.PayoutTerminalID = strings.TrimSpace(c.PayoutTerminalID)
	c.AppID = strings.TrimSpace(c.AppID)
	c.PrivateKeyPEM = strings.TrimSpace(c.PrivateKeyPEM)
	c.BaofuPublicKeyPEM = strings.TrimSpace(c.BaofuPublicKeyPEM)
	c.AESKey = strings.TrimSpace(c.AESKey)
	c.NotifyBaseURL = strings.TrimSpace(c.NotifyBaseURL)
	if c.Timeout == 0 {
		c.Timeout = 30 * time.Second
	}
	return c
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
