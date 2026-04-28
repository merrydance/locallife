package wechat

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
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

func TestGetWXACodeUnlimitedNormalizesJPEGResponseToPNG(t *testing.T) {
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

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 0x12, G: 0x34, B: 0x56, A: 0xff})
	var jpegBody bytes.Buffer
	require.NoError(t, jpeg.Encode(&jpegBody, img, nil))

	client := NewClient("app_id", "app_secret", store)
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodPost, req.Method)
			require.Contains(t, req.URL.String(), "access_token=cached_token")

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(jpegBody.Bytes())),
				Header: http.Header{
					"Content-Type": []string{"image/jpeg"},
				},
			}, nil
		}),
	}

	resp, err := client.GetWXACodeUnlimited(context.Background(), &WXACodeRequest{
		Scene:      "m_2-t_A01",
		Page:       "pages/dine-in/menu/menu",
		EnvVersion: "develop",
		Width:      430,
	})

	require.NoError(t, err)
	require.True(t, bytes.HasPrefix(resp, pngSignature))
}
