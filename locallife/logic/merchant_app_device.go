package logic

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

const (
	merchantAppDeviceIDMaxLength      = 255
	merchantAppPushTokenMaxLength     = 512
	merchantAppDeviceModelMaxLength   = 100
	merchantAppDeviceOSVersionMaxLen  = 50
	merchantAppDeviceAppVersionMaxLen = 32
)

// MerchantAppDeviceRegisterInput carries authenticated merchant app push registration data.
type MerchantAppDeviceRegisterInput struct {
	MerchantID  int64
	UserID      int64
	DeviceID    string
	PushToken   string
	Platform    string
	Provider    string
	DeviceModel string
	OSVersion   string
	AppVersion  string
}

// MerchantAppDeviceHeartbeatInput carries authenticated merchant app heartbeat data.
type MerchantAppDeviceHeartbeatInput struct {
	MerchantID  int64
	UserID      int64
	DeviceID    string
	Provider    string
	PushToken   string
	DeviceModel string
	OSVersion   string
	AppVersion  string
}

// MerchantAppDeviceUnregisterInput carries authenticated merchant app unregister data.
type MerchantAppDeviceUnregisterInput struct {
	MerchantID int64
	UserID     int64
	DeviceID   string
}

// MerchantAppDeviceResult is a stable logic result for API response mapping.
type MerchantAppDeviceResult struct {
	Device       db.MerchantAppDevice
	DeviceID     string
	Registered   bool
	Heartbeat    bool
	Unregistered bool
}

// RegisterMerchantAppDevice registers or updates the authenticated merchant app push device.
func RegisterMerchantAppDevice(ctx context.Context, store db.Store, input MerchantAppDeviceRegisterInput) (MerchantAppDeviceResult, error) {
	if err := validateMerchantAppDevicePrincipal(input.MerchantID, input.UserID); err != nil {
		return MerchantAppDeviceResult{}, err
	}

	deviceID, err := requiredMerchantAppDeviceText(input.DeviceID, "device_id", merchantAppDeviceIDMaxLength)
	if err != nil {
		return MerchantAppDeviceResult{}, err
	}
	pushToken, err := requiredMerchantAppDeviceText(input.PushToken, "push_token", merchantAppPushTokenMaxLength)
	if err != nil {
		return MerchantAppDeviceResult{}, err
	}
	platform, err := normalizeMerchantAppDevicePlatform(input.Platform)
	if err != nil {
		return MerchantAppDeviceResult{}, err
	}
	provider, err := normalizeMerchantAppDeviceProvider(input.Provider, true)
	if err != nil {
		return MerchantAppDeviceResult{}, err
	}
	deviceModel, err := optionalMerchantAppDeviceText(input.DeviceModel, "device_model", merchantAppDeviceModelMaxLength)
	if err != nil {
		return MerchantAppDeviceResult{}, err
	}
	osVersion, err := optionalMerchantAppDeviceText(input.OSVersion, "os_version", merchantAppDeviceOSVersionMaxLen)
	if err != nil {
		return MerchantAppDeviceResult{}, err
	}
	appVersion, err := optionalMerchantAppDeviceText(input.AppVersion, "app_version", merchantAppDeviceAppVersionMaxLen)
	if err != nil {
		return MerchantAppDeviceResult{}, err
	}

	device, err := store.RegisterMerchantAppDeviceTx(ctx, db.RegisterMerchantAppDeviceParams{
		MerchantID:  input.MerchantID,
		UserID:      input.UserID,
		DeviceID:    deviceID,
		Platform:    platform,
		Provider:    provider,
		PushToken:   pushToken,
		DeviceModel: deviceModel,
		OsVersion:   osVersion,
		AppVersion:  appVersion,
	})
	if err != nil {
		return MerchantAppDeviceResult{}, err
	}

	return MerchantAppDeviceResult{Device: device, DeviceID: device.DeviceID, Registered: true}, nil
}

// HeartbeatMerchantAppDevice updates the authenticated merchant app device heartbeat.
func HeartbeatMerchantAppDevice(ctx context.Context, store db.Store, input MerchantAppDeviceHeartbeatInput) (MerchantAppDeviceResult, error) {
	if err := validateMerchantAppDevicePrincipal(input.MerchantID, input.UserID); err != nil {
		return MerchantAppDeviceResult{}, err
	}

	deviceID, err := requiredMerchantAppDeviceText(input.DeviceID, "device_id", merchantAppDeviceIDMaxLength)
	if err != nil {
		return MerchantAppDeviceResult{}, err
	}
	provider, err := optionalMerchantAppDeviceProvider(input.Provider)
	if err != nil {
		return MerchantAppDeviceResult{}, err
	}
	pushToken, err := optionalMerchantAppDeviceText(input.PushToken, "push_token", merchantAppPushTokenMaxLength)
	if err != nil {
		return MerchantAppDeviceResult{}, err
	}
	deviceModel, err := optionalMerchantAppDeviceText(input.DeviceModel, "device_model", merchantAppDeviceModelMaxLength)
	if err != nil {
		return MerchantAppDeviceResult{}, err
	}
	osVersion, err := optionalMerchantAppDeviceText(input.OSVersion, "os_version", merchantAppDeviceOSVersionMaxLen)
	if err != nil {
		return MerchantAppDeviceResult{}, err
	}
	appVersion, err := optionalMerchantAppDeviceText(input.AppVersion, "app_version", merchantAppDeviceAppVersionMaxLen)
	if err != nil {
		return MerchantAppDeviceResult{}, err
	}

	device, err := store.UpdateMerchantAppDeviceHeartbeatTx(ctx, db.UpdateMerchantAppDeviceHeartbeatParams{
		Provider:    provider,
		PushToken:   pushToken,
		DeviceModel: deviceModel,
		OsVersion:   osVersion,
		AppVersion:  appVersion,
		MerchantID:  input.MerchantID,
		DeviceID:    deviceID,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return MerchantAppDeviceResult{}, NewRequestError(http.StatusNotFound, errors.New("merchant app device not found"))
		}
		return MerchantAppDeviceResult{}, err
	}

	return MerchantAppDeviceResult{Device: device, DeviceID: device.DeviceID, Heartbeat: true}, nil
}

// UnregisterMerchantAppDevice deactivates the authenticated merchant app device binding.
func UnregisterMerchantAppDevice(ctx context.Context, store db.Store, input MerchantAppDeviceUnregisterInput) (MerchantAppDeviceResult, error) {
	if err := validateMerchantAppDevicePrincipal(input.MerchantID, input.UserID); err != nil {
		return MerchantAppDeviceResult{}, err
	}

	deviceID, err := requiredMerchantAppDeviceText(input.DeviceID, "device_id", merchantAppDeviceIDMaxLength)
	if err != nil {
		return MerchantAppDeviceResult{}, err
	}

	rows, err := store.UnregisterMerchantAppDevice(ctx, db.UnregisterMerchantAppDeviceParams{
		MerchantID: input.MerchantID,
		DeviceID:   deviceID,
	})
	if err != nil {
		return MerchantAppDeviceResult{}, err
	}

	return MerchantAppDeviceResult{DeviceID: deviceID, Unregistered: rows > 0}, nil
}

func validateMerchantAppDevicePrincipal(merchantID, userID int64) error {
	if merchantID <= 0 {
		return NewRequestError(http.StatusBadRequest, errors.New("merchant_id is required"))
	}
	if userID <= 0 {
		return NewRequestError(http.StatusBadRequest, errors.New("user_id is required"))
	}
	return nil
}

func requiredMerchantAppDeviceText(value, field string, maxLen int) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", NewRequestError(http.StatusBadRequest, errors.New(field+" is required"))
	}
	if len(trimmed) > maxLen {
		return "", NewRequestError(http.StatusBadRequest, errors.New(field+" is too long"))
	}
	return trimmed, nil
}

func optionalMerchantAppDeviceText(value, field string, maxLen int) (pgtype.Text, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return pgtype.Text{}, nil
	}
	if len(trimmed) > maxLen {
		return pgtype.Text{}, NewRequestError(http.StatusBadRequest, errors.New(field+" is too long"))
	}
	return pgtype.Text{String: trimmed, Valid: true}, nil
}

func normalizeMerchantAppDevicePlatform(value string) (string, error) {
	platform := strings.ToLower(strings.TrimSpace(value))
	if platform == "" {
		return "", NewRequestError(http.StatusBadRequest, errors.New("platform is required"))
	}
	if platform != db.MerchantAppDevicePlatformAndroid {
		return "", NewRequestError(http.StatusBadRequest, errors.New("unsupported platform"))
	}
	return platform, nil
}

func optionalMerchantAppDeviceProvider(value string) (pgtype.Text, error) {
	provider, err := normalizeMerchantAppDeviceProvider(value, false)
	if err != nil || provider == "" {
		return pgtype.Text{}, err
	}
	return pgtype.Text{String: provider, Valid: true}, nil
}

func normalizeMerchantAppDeviceProvider(value string, defaultUnknown bool) (string, error) {
	provider := strings.ToLower(strings.TrimSpace(value))
	if provider == "" {
		if defaultUnknown {
			return db.MerchantAppDeviceProviderUnknown, nil
		}
		return "", nil
	}

	switch provider {
	case db.MerchantAppDeviceProviderHuawei,
		db.MerchantAppDeviceProviderHonor,
		db.MerchantAppDeviceProviderXiaomi,
		db.MerchantAppDeviceProviderOppo,
		db.MerchantAppDeviceProviderVivo,
		db.MerchantAppDeviceProviderUnknown:
		return provider, nil
	default:
		return "", NewRequestError(http.StatusBadRequest, errors.New("unsupported provider"))
	}
}
