package logic

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
)

const (
	appVersionPackageNameMaxLength = 120
	appVersionNameMaxLength        = 50
)

type AppVersionLatestInput struct {
	Platform    string
	Channel     string
	PackageName string
	VersionCode int32
	VersionName string
}

type AppVersionLatestResult struct {
	HasUpdate     bool
	VersionCode   int32
	VersionName   string
	DownloadURL   string
	Changelog     string
	IsForce       bool
	PublishedAt   *time.Time
	FileSizeBytes *int64
	Sha256        string
}

func GetLatestAppVersion(ctx context.Context, store db.Store, input AppVersionLatestInput) (AppVersionLatestResult, error) {
	platform, err := normalizeAppVersionPlatform(input.Platform)
	if err != nil {
		return AppVersionLatestResult{}, err
	}
	channel, err := normalizeAppVersionChannel(input.Channel)
	if err != nil {
		return AppVersionLatestResult{}, err
	}
	packageName, err := requiredAppVersionText(input.PackageName, "package_name", appVersionPackageNameMaxLength)
	if err != nil {
		return AppVersionLatestResult{}, err
	}
	versionName, err := optionalAppVersionText(input.VersionName, "version_name", appVersionNameMaxLength)
	if err != nil {
		return AppVersionLatestResult{}, err
	}
	if input.VersionCode <= 0 {
		return AppVersionLatestResult{}, NewRequestError(http.StatusBadRequest, errors.New("version_code is required"))
	}

	version, err := store.GetLatestActiveAppVersion(ctx, db.GetLatestActiveAppVersionParams{
		Platform:           platform,
		Channel:            channel,
		PackageName:        packageName,
		CurrentVersionCode: input.VersionCode,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return AppVersionLatestResult{HasUpdate: false, VersionCode: input.VersionCode, VersionName: versionName}, nil
		}
		return AppVersionLatestResult{}, err
	}

	result := AppVersionLatestResult{
		HasUpdate:   true,
		VersionCode: version.VersionCode,
		VersionName: version.VersionName,
		DownloadURL: version.DownloadUrl,
		Changelog:   version.Changelog,
		IsForce:     version.IsForce,
		PublishedAt: &version.PublishedAt,
	}
	if version.FileSizeBytes.Valid {
		result.FileSizeBytes = &version.FileSizeBytes.Int64
	}
	if version.ChecksumSha256.Valid {
		result.Sha256 = version.ChecksumSha256.String
	}
	return result, nil
}

func normalizeAppVersionPlatform(value string) (string, error) {
	platform := strings.ToLower(strings.TrimSpace(value))
	if platform == "" {
		return "", NewRequestError(http.StatusBadRequest, errors.New("platform is required"))
	}
	if platform != db.AppVersionPlatformAndroid {
		return "", NewRequestError(http.StatusBadRequest, errors.New("unsupported platform"))
	}
	return platform, nil
}

func normalizeAppVersionChannel(value string) (string, error) {
	channel := strings.ToLower(strings.TrimSpace(value))
	if channel == "" {
		return "", NewRequestError(http.StatusBadRequest, errors.New("channel is required"))
	}
	if channel != db.AppVersionChannelMerchant {
		return "", NewRequestError(http.StatusBadRequest, errors.New("unsupported channel"))
	}
	return channel, nil
}

func requiredAppVersionText(value, field string, maxLen int) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", NewRequestError(http.StatusBadRequest, errors.New(field+" is required"))
	}
	if len(trimmed) > maxLen {
		return "", NewRequestError(http.StatusBadRequest, errors.New(field+" is too long"))
	}
	return trimmed, nil
}

func optionalAppVersionText(value, field string, maxLen int) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	if len(trimmed) > maxLen {
		return "", NewRequestError(http.StatusBadRequest, errors.New(field+" is too long"))
	}
	return trimmed, nil
}
