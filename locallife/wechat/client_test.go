package wechat

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetAccessToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := NewClient("test_app_id", "test_app_secret", store)

	testCases := []struct {
		name        string
		appType     string
		buildStub   func(store *mockdb.MockStore)
		checkResult func(t *testing.T, token string, err error)
	}{
		{
			name:    "ValidCachedToken",
			appType: "miniprogram",
			buildStub: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetWechatAccessToken(gomock.Any(), gomock.Eq("miniprogram")).
					Times(1).
					Return(db.WechatAccessToken{
						AppType:     "miniprogram",
						AccessToken: "cached_token",
						ExpiresAt:   time.Now().Add(30 * time.Minute),
					}, nil)
			},
			checkResult: func(t *testing.T, token string, err error) {
				require.NoError(t, err)
				require.Equal(t, "cached_token", token)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.buildStub(store)

			token, err := client.GetAccessToken(context.Background(), tc.appType)
			tc.checkResult(t, token, err)
		})
	}
}

func TestCode2SessionRejectsMissingOpenID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := NewClient("test_app_id", "test_app_secret", store)
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodGet, req.Method)
			require.Contains(t, req.URL.String(), "jscode2session")
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"errcode":0,"session_key":"session-key-without-openid"}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	resp, err := client.Code2Session(context.Background(), "code-with-empty-openid")

	require.Nil(t, resp)
	require.Error(t, err)
	require.ErrorContains(t, err, "missing openid")
}

func TestCode2SessionTransportErrorDoesNotLeakSecret(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const appSecret = "super_secret_value"
	store := mockdb.NewMockStore(ctrl)
	client := NewClient("test_app_id", appSecret, store)
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, context.Canceled
		}),
	}

	resp, err := client.Code2Session(context.Background(), "bad code?secret=probe")

	require.Nil(t, resp)
	require.Error(t, err)
	require.NotContains(t, err.Error(), appSecret)
	require.NotContains(t, err.Error(), "secret=")
	require.NotContains(t, err.Error(), "js_code=")
}
