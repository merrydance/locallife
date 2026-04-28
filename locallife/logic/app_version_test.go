package logic

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetLatestAppVersion(t *testing.T) {
	publishedAt := time.Date(2026, 4, 12, 12, 0, 0, 0, time.UTC)

	testCases := []struct {
		name        string
		input       AppVersionLatestInput
		buildStubs  func(store *mockdb.MockStore)
		checkResult func(t *testing.T, result AppVersionLatestResult, err error)
	}{
		{
			name: "HasUpdate",
			input: AppVersionLatestInput{
				Platform:    "android",
				Channel:     "merchant_app",
				PackageName: "com.merrydance.locallife.merchant",
				VersionCode: 1,
				VersionName: "1.0.0",
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetLatestActiveAppVersion(gomock.Any(), db.GetLatestActiveAppVersionParams{
						Platform:           db.AppVersionPlatformAndroid,
						Channel:            db.AppVersionChannelMerchant,
						PackageName:        "com.merrydance.locallife.merchant",
						CurrentVersionCode: 1,
					}).
					Times(1).
					Return(db.AppVersion{
						Platform:       db.AppVersionPlatformAndroid,
						Channel:        db.AppVersionChannelMerchant,
						PackageName:    "com.merrydance.locallife.merchant",
						VersionCode:    2,
						VersionName:    "1.0.1",
						DownloadUrl:    "https://example.com/merchant-app-1.0.1.apk",
						Changelog:      "修复稳定性问题",
						IsForce:        true,
						FileSizeBytes:  pgtype.Int8{Int64: 48392120, Valid: true},
						ChecksumSha256: pgtype.Text{String: "3a6eb0790f39ac87c94f3856b2dd2c5d110e6811602261a9a923d3bb23adc8b7", Valid: true},
						Status:         db.AppVersionStatusActive,
						PublishedAt:    publishedAt,
					}, nil)
			},
			checkResult: func(t *testing.T, result AppVersionLatestResult, err error) {
				require.NoError(t, err)
				require.True(t, result.HasUpdate)
				require.Equal(t, int32(2), result.VersionCode)
				require.Equal(t, "1.0.1", result.VersionName)
				require.Equal(t, "https://example.com/merchant-app-1.0.1.apk", result.DownloadURL)
				require.True(t, result.IsForce)
				require.NotNil(t, result.PublishedAt)
				require.NotNil(t, result.FileSizeBytes)
				require.Equal(t, int64(48392120), *result.FileSizeBytes)
				require.Equal(t, "3a6eb0790f39ac87c94f3856b2dd2c5d110e6811602261a9a923d3bb23adc8b7", result.Sha256)
			},
		},
		{
			name: "NoUpdate",
			input: AppVersionLatestInput{
				Platform:    "android",
				Channel:     "merchant_app",
				PackageName: "com.merrydance.locallife.merchant",
				VersionCode: 2,
				VersionName: "1.0.1",
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetLatestActiveAppVersion(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.AppVersion{}, db.ErrRecordNotFound)
			},
			checkResult: func(t *testing.T, result AppVersionLatestResult, err error) {
				require.NoError(t, err)
				require.False(t, result.HasUpdate)
				require.Equal(t, int32(2), result.VersionCode)
				require.Equal(t, "1.0.1", result.VersionName)
				require.Empty(t, result.DownloadURL)
			},
		},
		{
			name: "UnsupportedChannel",
			input: AppVersionLatestInput{
				Platform:    "android",
				Channel:     "customer_app",
				PackageName: "com.merrydance.locallife.merchant",
				VersionCode: 1,
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResult: func(t *testing.T, result AppVersionLatestResult, err error) {
				require.Error(t, err)
				require.Empty(t, result)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			result, err := GetLatestAppVersion(context.Background(), store, tc.input)
			tc.checkResult(t, result, err)
		})
	}
}
