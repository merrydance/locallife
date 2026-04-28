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

func TestGetWXACodeUnlimitedReturnsAPIErrorForJSONResponseWithCharset(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetWechatAccessToken(gomock.Any(), "mp").
		Return(db.WechatAccessToken{
			AppType:     "mp",
			AccessToken: "cached_token",
			ExpiresAt:   time.Now().Add(30 * time.Minute),
		}, nil)

	client := NewClient("app_id", "app_secret", store)
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodPost, req.Method)
			require.Contains(t, req.URL.String(), "access_token=cached_token")

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"errcode":41030,"errmsg":"invalid page path for merchant storefront qrcode generation, please check page routing configuration"}`)),
				Header: http.Header{
					"Content-Type": []string{"application/json; charset=utf-8"},
				},
			}, nil
		}),
	}

	resp, err := client.GetWXACodeUnlimited(context.Background(), &WXACodeRequest{
		Scene:      "m_2",
		Page:       "pages/takeout/restaurant-detail/index",
		EnvVersion: "release",
		Width:      430,
	})

	require.Nil(t, resp)
	require.Error(t, err)

	apiErr, ok := err.(*APIError)
	require.True(t, ok)
	require.Equal(t, 41030, apiErr.Code)
	require.Contains(t, apiErr.Msg, "invalid page path")
}
