package ordinaryserviceprovider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/consts"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
)

type sdkClient interface {
	Request(ctx context.Context, method, requestPath string, headerParams http.Header, queryParams url.Values, postBody interface{}, contentType string) (*core.APIResult, error)
	Upload(ctx context.Context, requestURL, meta, reqBody, formContentType string) (*core.APIResult, error)
}

type sdkClientFactory func(Config) (sdkClient, error)

func newSDKClient(cfg Config) (sdkClient, error) {
	privateKey, err := utils.LoadPrivateKeyWithPath(cfg.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("load private key: %w", err)
	}
	platformPublicKey, err := utils.LoadPublicKeyWithPath(cfg.PlatformPublicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("load platform public key: %w", err)
	}
	client, err := core.NewClient(context.Background(), option.WithWechatPayPublicKeyAuthCipher(
		cfg.ServiceProviderMchID,
		cfg.CertificateSerialNumber,
		privateKey,
		cfg.PlatformPublicKeyID,
		platformPublicKey,
	))
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (c *Client) requestJSON(ctx context.Context, operation, method, path string, query url.Values, requestBody interface{}, responseBody interface{}) error {
	if c == nil || c.sdk == nil {
		return &ProviderError{
			Operation: strings.TrimSpace(operation),
			Category:  ErrorCategoryAuthConfig,
			Frontend:  frontendGuidanceForCategory(ErrorCategoryAuthConfig),
			cause:     fmt.Errorf("ordinary service provider sdk client is not configured"),
		}
	}
	requestURL := strings.TrimRight(c.config.BaseURL, "/") + path
	contentType := ""
	if requestBody != nil {
		contentType = consts.ApplicationJSON
	}
	result, err := c.sdk.Request(ctx, method, requestURL, nil, query, requestBody, contentType)
	if err != nil {
		return mapSDKAPIError(operation, err)
	}
	if responseBody == nil || result == nil || result.Response == nil || result.Response.Body == nil {
		return nil
	}
	defer result.Response.Body.Close()
	data, err := io.ReadAll(result.Response.Body)
	if err != nil {
		return localProviderError(operation, "LOCAL_RESPONSE_READ_ERROR", err)
	}
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, responseBody); err != nil {
		return localProviderError(operation, "LOCAL_RESPONSE_DECODE_ERROR", err)
	}
	return nil
}

func localProviderError(operation, providerCode string, cause error) error {
	if cause == nil {
		cause = errors.New("ordinary service provider local error")
	}
	return &ProviderError{
		Operation:       strings.TrimSpace(operation),
		ProviderCode:    providerCode,
		ProviderMessage: cause.Error(),
		Category:        ErrorCategoryProvider,
		Frontend:        frontendGuidanceForCategory(ErrorCategoryProvider),
		cause:           cause,
	}
}

func (c *Client) requestNoBody(ctx context.Context, operation, method, path string, query url.Values) error {
	return c.requestJSON(ctx, operation, method, path, query, nil, nil)
}
