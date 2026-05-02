package ordinaryserviceprovider

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
)

const operationParseNotification = "parse ordinary service provider notification"

type NotificationHeaders struct {
	Serial        string
	Signature     string
	Timestamp     string
	Nonce         string
	SignatureType string
}

type NotificationEnvelope struct {
	ID           string
	CreateTime   *time.Time
	EventType    string
	ResourceType string
	OriginalType string
	Summary      string
	Plaintext    string
	Decoded      map[string]any
	Headers      NotificationHeaders
}

func (c *Client) ParseNotification(ctx context.Context, request *http.Request, target NotificationTarget) (*NotificationEnvelope, error) {
	operation := operationParseNotification
	if strings.TrimSpace(string(target)) != "" {
		operation += " " + strings.TrimSpace(string(target))
	}
	if request == nil {
		return nil, notificationProviderError(operation, ErrorCategoryValidation, "LOCAL_NOTIFICATION_REQUEST_REQUIRED", errors.New("http request is required"))
	}
	if c == nil {
		return nil, notificationProviderError(operation, ErrorCategoryAuthConfig, "LOCAL_NOTIFICATION_CONFIG_ERROR", errors.New("ordinary service provider client is not configured"))
	}
	platformPublicKey, err := utils.LoadPublicKeyWithPath(c.config.PlatformPublicKeyPath)
	if err != nil {
		return nil, notificationProviderError(operation, ErrorCategoryAuthConfig, "LOCAL_NOTIFICATION_PUBLIC_KEY_ERROR", err)
	}
	verifier := verifiers.NewSHA256WithRSAPubkeyVerifier(c.config.PlatformPublicKeyID, *platformPublicKey)
	handler, err := notify.NewRSANotifyHandler(c.config.APIV3Key, verifier)
	if err != nil {
		return nil, notificationProviderError(operation, ErrorCategoryAuthConfig, "LOCAL_NOTIFICATION_HANDLER_ERROR", err)
	}

	decoded := map[string]any{}
	notifyRequest, err := handler.ParseNotifyRequest(ctx, request, decoded)
	if err != nil {
		return nil, notificationProviderError(operation, ErrorCategoryAuthConfig, "LOCAL_NOTIFICATION_PARSE_ERROR", err)
	}
	envelope := buildNotificationEnvelope(notifyRequest, decoded, request.Header)
	return &envelope, nil
}

func (e NotificationEnvelope) LogFields() map[string]any {
	fields := map[string]any{
		"notification_id": e.ID,
		"event_type":      e.EventType,
		"resource_type":   e.ResourceType,
		"original_type":   e.OriginalType,
		"summary":         e.Summary,
	}
	if e.CreateTime != nil {
		fields["create_time"] = e.CreateTime.Format(time.RFC3339)
	}
	if strings.TrimSpace(e.Headers.Serial) != "" {
		fields["wechatpay_serial"] = e.Headers.Serial
	}
	return fields
}

func buildNotificationEnvelope(request *notify.Request, decoded map[string]any, headers http.Header) NotificationEnvelope {
	if request == nil {
		return NotificationEnvelope{Decoded: decoded, Headers: notificationHeadersFromHTTP(headers)}
	}
	envelope := NotificationEnvelope{
		ID:           request.ID,
		CreateTime:   request.CreateTime,
		EventType:    request.EventType,
		ResourceType: request.ResourceType,
		Summary:      request.Summary,
		Decoded:      decoded,
		Headers:      notificationHeadersFromHTTP(headers),
	}
	if request.Resource != nil {
		envelope.OriginalType = request.Resource.OriginalType
		envelope.Plaintext = request.Resource.Plaintext
	}
	return envelope
}

func notificationHeadersFromHTTP(headers http.Header) NotificationHeaders {
	return NotificationHeaders{
		Serial:        strings.TrimSpace(headers.Get("Wechatpay-Serial")),
		Signature:     strings.TrimSpace(headers.Get("Wechatpay-Signature")),
		Timestamp:     strings.TrimSpace(headers.Get("Wechatpay-Timestamp")),
		Nonce:         strings.TrimSpace(headers.Get("Wechatpay-Nonce")),
		SignatureType: strings.TrimSpace(headers.Get("Wechatpay-Signature-Type")),
	}
}

func notificationProviderError(operation string, category ErrorCategory, providerCode string, cause error) error {
	if cause == nil {
		cause = errors.New("ordinary service provider notification error")
	}
	return &ProviderError{
		Operation:       strings.TrimSpace(operation),
		ProviderCode:    providerCode,
		ProviderMessage: cause.Error(),
		Category:        category,
		Frontend:        frontendGuidanceForCategory(category),
		cause:           cause,
	}
}
