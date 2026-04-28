package logic

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	db "github.com/merrydance/locallife/db/sqlc"
)

type MerchantAppPushTarget struct {
	Provider  string
	DeviceID  string
	PushToken string
}

type MerchantAppPushMessage struct {
	MessageID string
	Title     string
	Content   string
	Data      MerchantAppNotificationPayload
}

type MerchantAppPushProvider interface {
	Send(ctx context.Context, target MerchantAppPushTarget, message MerchantAppPushMessage) error
}

type MerchantAppPushProviderRegistry interface {
	Provider(name string) (MerchantAppPushProvider, bool)
}

type StaticMerchantAppPushProviderRegistry map[string]MerchantAppPushProvider

func (r StaticMerchantAppPushProviderRegistry) Provider(name string) (MerchantAppPushProvider, bool) {
	provider, ok := r[name]
	return provider, ok && provider != nil
}

type NoopMerchantAppPushProvider struct{}

func (NoopMerchantAppPushProvider) Send(context.Context, MerchantAppPushTarget, MerchantAppPushMessage) error {
	return nil
}

type MerchantAppPushSendError struct {
	Err       error
	Retryable bool
}

func (e MerchantAppPushSendError) Error() string {
	if e.Err == nil {
		return "merchant app push send failed"
	}
	return e.Err.Error()
}

func (e MerchantAppPushSendError) Unwrap() error {
	return e.Err
}

func NewRetryableMerchantAppPushError(err error) error {
	return MerchantAppPushSendError{Err: err, Retryable: true}
}

func NewPermanentMerchantAppPushError(err error) error {
	return MerchantAppPushSendError{Err: err, Retryable: false}
}

type MerchantAppPushDispatcher struct {
	store    db.Store
	registry MerchantAppPushProviderRegistry
}

func NewMerchantAppPushDispatcher(store db.Store, registry MerchantAppPushProviderRegistry) *MerchantAppPushDispatcher {
	return &MerchantAppPushDispatcher{store: store, registry: registry}
}

type MerchantAppPushDispatchInput struct {
	MerchantID int64
	Payload    MerchantAppNotificationPayload
}

type MerchantAppPushDispatchResult struct {
	Attempted             int
	Sent                  int
	Skipped               int
	RetryableFailures     int
	PermanentFailures     int
	DeviceResultSummaries []MerchantAppPushDeviceResult
}

type MerchantAppPushDeviceResult struct {
	DeviceID  string
	Provider  string
	Sent      bool
	Skipped   bool
	Retryable bool
	Error     string
}

func (d *MerchantAppPushDispatcher) Dispatch(ctx context.Context, input MerchantAppPushDispatchInput) (MerchantAppPushDispatchResult, error) {
	if d == nil || d.store == nil {
		return MerchantAppPushDispatchResult{}, errors.New("merchant app push dispatcher store is required")
	}
	if input.MerchantID <= 0 {
		return MerchantAppPushDispatchResult{}, NewRequestError(http.StatusBadRequest, errors.New("merchant_id is required"))
	}
	if input.Payload.MessageID == "" {
		return MerchantAppPushDispatchResult{}, NewRequestError(http.StatusBadRequest, errors.New("message_id is required"))
	}

	devices, err := d.store.ListActiveMerchantAppDevicesByMerchant(ctx, input.MerchantID)
	if err != nil {
		return MerchantAppPushDispatchResult{}, fmt.Errorf("list merchant app devices: %w", err)
	}

	message := MerchantAppPushMessage{
		MessageID: input.Payload.MessageID,
		Title:     input.Payload.Title,
		Content:   input.Payload.Content,
		Data:      input.Payload,
	}

	result := MerchantAppPushDispatchResult{DeviceResultSummaries: make([]MerchantAppPushDeviceResult, 0, len(devices))}
	for _, device := range devices {
		deviceResult := MerchantAppPushDeviceResult{DeviceID: device.DeviceID, Provider: device.Provider}
		provider, ok := d.providerFor(device.Provider)
		if !ok {
			deviceResult.Skipped = true
			deviceResult.Error = "push provider not configured"
			result.Skipped++
			result.DeviceResultSummaries = append(result.DeviceResultSummaries, deviceResult)
			continue
		}

		result.Attempted++
		err := provider.Send(ctx, MerchantAppPushTarget{Provider: device.Provider, DeviceID: device.DeviceID, PushToken: device.PushToken}, message)
		if err == nil {
			deviceResult.Sent = true
			result.Sent++
			result.DeviceResultSummaries = append(result.DeviceResultSummaries, deviceResult)
			continue
		}

		if merchantAppPushErrorIsRetryable(err) {
			deviceResult.Retryable = true
			deviceResult.Error = "push provider retryable failure"
			result.RetryableFailures++
		} else {
			deviceResult.Error = "push provider permanent failure"
			result.PermanentFailures++
		}
		result.DeviceResultSummaries = append(result.DeviceResultSummaries, deviceResult)
	}

	return result, nil
}

func (d *MerchantAppPushDispatcher) providerFor(provider string) (MerchantAppPushProvider, bool) {
	if d.registry == nil {
		return nil, false
	}
	return d.registry.Provider(provider)
}

func merchantAppPushErrorIsRetryable(err error) bool {
	var sendErr MerchantAppPushSendError
	if errors.As(err, &sendErr) {
		return sendErr.Retryable
	}
	return true
}
