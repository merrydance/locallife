package ordinaryserviceprovider

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"strings"
)

type Client struct {
	config            Config
	sdk               sdkClient
	privateKey        *rsa.PrivateKey
	platformPublicKey *rsa.PublicKey
}

type clientOptions struct {
	sdkFactory sdkClientFactory
}

type Option func(*clientOptions)

func WithSDKClientFactory(factory sdkClientFactory) Option {
	return func(options *clientOptions) {
		options.sdkFactory = factory
	}
}

func NewOrdinaryServiceProviderClient(cfg Config, options ...Option) (*Client, error) {
	normalizedConfig := cfg.Normalized()
	if err := normalizedConfig.Validate(); err != nil {
		return nil, fmt.Errorf("validate ordinary service provider config: %w", err)
	}

	clientOptions := clientOptions{sdkFactory: newSDKClient}
	for _, option := range options {
		option(&clientOptions)
	}

	sdk, err := clientOptions.sdkFactory(normalizedConfig)
	if err != nil {
		return nil, fmt.Errorf("create ordinary service provider sdk client: %w", err)
	}
	privateKey, err := loadPrivateKey(normalizedConfig.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("load ordinary service provider private key: %w", err)
	}
	platformPublicKey, err := loadPublicKey(normalizedConfig.PlatformPublicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("load ordinary service provider platform public key: %w", err)
	}

	return &Client{config: normalizedConfig, sdk: sdk, privateKey: privateKey, platformPublicKey: platformPublicKey}, nil
}

func (c *Client) ServiceProviderMchID() string {
	if c == nil {
		return ""
	}
	return c.config.ServiceProviderMchID
}

func (c *Client) ServiceProviderMchName() string {
	if c == nil {
		return ""
	}
	return c.config.ServiceProviderMchName
}

func (c *Client) ServiceProviderAppID() string {
	if c == nil {
		return ""
	}
	return c.config.ServiceProviderAppID
}

func (c *Client) PaymentNotifyURL() string {
	if c == nil {
		return ""
	}
	return c.config.PaymentNotifyURL
}

func (c *Client) CombineNotifyURL() string {
	if c == nil {
		return ""
	}
	return c.config.CombineNotifyURL
}

func (c *Client) RefundNotifyURL() string {
	if c == nil {
		return ""
	}
	return c.config.RefundNotifyURL
}

func (c *Client) ProfitSharingNotifyURL() string {
	if c == nil {
		return ""
	}
	return c.config.ProfitSharingNotifyURL
}

func (c *Client) EncryptSensitiveData(plaintext string) (string, error) {
	if c == nil || c.platformPublicKey == nil {
		return "", fmt.Errorf("ordinary service provider platform public key not loaded")
	}
	ciphertext, err := rsa.EncryptOAEP(sha1.New(), rand.Reader, c.platformPublicKey, []byte(plaintext), nil)
	if err != nil {
		return "", fmt.Errorf("ordinary service provider encrypt sensitive data: %w", err)
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (c *Client) DecryptSensitiveResponseData(ciphertext string) (string, error) {
	trimmed := strings.TrimSpace(ciphertext)
	if trimmed == "" {
		return "", nil
	}
	if c == nil || c.privateKey == nil {
		return "", fmt.Errorf("ordinary service provider private key not loaded")
	}
	decoded, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		return "", fmt.Errorf("ordinary service provider decode sensitive ciphertext: %w", err)
	}
	plaintext, err := rsa.DecryptOAEP(sha1.New(), rand.Reader, c.privateKey, decoded, nil)
	if err != nil {
		return "", fmt.Errorf("ordinary service provider decrypt sensitive ciphertext: %w", err)
	}
	return string(plaintext), nil
}
