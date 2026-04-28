package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetLatestAppVersionAPI(t *testing.T) {
	publishedAt := time.Date(2026, 4, 12, 12, 0, 0, 0, time.UTC)

	testCases := []struct {
		name          string
		query         string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "HasUpdate",
			query: "platform=android&channel=merchant_app&package_name=com.merrydance.locallife.merchant&version_code=1&version_name=1.0.0",
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
						VersionCode:    2,
						VersionName:    "1.0.1",
						DownloadUrl:    "https://example.com/merchant-app-1.0.1.apk",
						Changelog:      "修复稳定性问题",
						FileSizeBytes:  pgtype.Int8{Int64: 48392120, Valid: true},
						ChecksumSha256: pgtype.Text{String: "3a6eb0790f39ac87c94f3856b2dd2c5d110e6811602261a9a923d3bb23adc8b7", Valid: true},
						Status:         db.AppVersionStatusActive,
						PublishedAt:    publishedAt,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response getLatestAppVersionResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.True(t, response.HasUpdate)
				require.Equal(t, int32(2), response.VersionCode)
				require.Equal(t, "1.0.1", response.VersionName)
				require.Equal(t, "https://example.com/merchant-app-1.0.1.apk", response.DownloadURL)
				require.NotNil(t, response.FileSizeBytes)
				require.Equal(t, int64(48392120), *response.FileSizeBytes)
			},
		},
		{
			name:  "NoUpdate",
			query: "platform=android&channel=merchant_app&package_name=com.merrydance.locallife.merchant&version_code=2&version_name=1.0.1",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetLatestActiveAppVersion(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.AppVersion{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response getLatestAppVersionResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.False(t, response.HasUpdate)
				require.Equal(t, int32(2), response.VersionCode)
				require.Equal(t, "1.0.1", response.VersionName)
				require.Empty(t, response.DownloadURL)
			},
		},
		{
			name:       "MissingVersionCode",
			query:      "platform=android&channel=merchant_app&package_name=com.merrydance.locallife.merchant",
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				var response APIResponse
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
				require.NotEmpty(t, response.Message)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			server := newTestServer(t, store)
			tc.buildStubs(store)

			request, err := http.NewRequest(http.MethodGet, "/v1/app/version/latest?"+tc.query, nil)
			require.NoError(t, err)

			recorder := httptest.NewRecorder()
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
