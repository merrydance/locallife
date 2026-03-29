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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestMediaCheckAsyncSuccess(t *testing.T) {
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

			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			require.Contains(t, string(body), `"media_url":"https://example.com/image.jpg"`)
			require.Contains(t, string(body), `"media_type":2`)

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"errcode":0,"errmsg":"ok","trace_id":"trace-123"}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	resp, err := client.MediaCheckAsync(context.Background(), MediaCheckAsyncRequest{
		MediaURL:  "https://example.com/image.jpg",
		MediaType: SecCheckMediaTypeImage,
		Version:   2,
		OpenID:    "openid-123",
		Scene:     2,
	})

	require.NoError(t, err)
	require.Equal(t, "trace-123", resp.TraceID)
}

func TestMediaCheckAsyncReturnsAPIError(t *testing.T) {
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
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"errcode":87014,"errmsg":"risky content"}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	resp, err := client.MediaCheckAsync(context.Background(), MediaCheckAsyncRequest{
		MediaURL:  "https://example.com/image.jpg",
		MediaType: SecCheckMediaTypeImage,
	})

	require.Nil(t, resp)
	require.Error(t, err)

	apiErr, ok := err.(*APIError)
	require.True(t, ok)
	require.Equal(t, 87014, apiErr.Code)
	require.Equal(t, "risky content", apiErr.Msg)
}
