package wechat

import (
	"context"
	"encoding/json"
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

func TestUploadShippingInfoUsesTransactionIDOrderKey(t *testing.T) {
	body := captureUploadShippingInfoBody(t, func(client *Client) error {
		return client.UploadShippingInfo(context.Background(), &UploadShippingInfoRequest{
			TransactionID: "420000000020260523000001",
			OutTradeNo:    "SHOULD_NOT_BE_USED",
			MchID:         "1900000109",
			PayerOpenID:   "openid-user",
			ItemDesc:      "招牌牛肉饭",
			NotifyURL:     "https://api.example.com/v1/webhooks/wechat-miniprogram/settlement-notify",
			UploadTime:    time.Date(2026, 5, 23, 12, 30, 0, 0, time.UTC),
		})
	})

	orderKey := body["order_key"].(map[string]interface{})
	require.Equal(t, float64(2), orderKey["order_number_type"])
	require.Equal(t, "420000000020260523000001", orderKey["transaction_id"])
	require.NotContains(t, orderKey, "out_trade_no")
	require.NotContains(t, body, "notify_url")
	require.Equal(t, "招牌牛肉饭", bodyShippingItem(t, body)["item_desc"])
}

func TestUploadShippingInfoUsesMerchantOrderKey(t *testing.T) {
	body := captureUploadShippingInfoBody(t, func(client *Client) error {
		return client.UploadShippingInfo(context.Background(), &UploadShippingInfoRequest{
			OutTradeNo:  "BF202605230001",
			MchID:       "1900000109",
			PayerOpenID: "openid-user",
			ItemDesc:    "同城外卖订单",
			UploadTime:  time.Date(2026, 5, 23, 12, 30, 0, 0, time.UTC),
		})
	})

	orderKey := body["order_key"].(map[string]interface{})
	require.Equal(t, float64(1), orderKey["order_number_type"])
	require.Equal(t, "1900000109", orderKey["mchid"])
	require.Equal(t, "BF202605230001", orderKey["out_trade_no"])
	require.NotContains(t, orderKey, "transaction_id")
	require.Equal(t, "同城外卖订单", bodyShippingItem(t, body)["item_desc"])
}

func TestUploadCombinedShippingInfoIncludesItemDescriptionAndOmitsNotifyURL(t *testing.T) {
	body := captureUploadShippingInfoBody(t, func(client *Client) error {
		return client.UploadCombinedShippingInfo(context.Background(), &UploadCombinedShippingInfoRequest{
			CombineOutTradeNo: "COMBINE202605230001",
			PayerOpenID:       "openid-user",
			ItemDesc:          "合单外卖订单",
			NotifyURL:         "https://api.example.com/v1/webhooks/wechat-miniprogram/settlement-notify",
			UploadTime:        time.Date(2026, 5, 23, 12, 30, 0, 0, time.UTC),
			SubOrders: []ShippingSubOrder{
				{MchID: "1900000109", OutTradeNo: "BF202605230001", ItemDesc: "商户子单菜品"},
			},
		})
	})

	require.NotContains(t, body, "notify_url")
	subOrders := body["sub_orders"].([]interface{})
	require.Len(t, subOrders, 1)
	subOrder := subOrders[0].(map[string]interface{})
	shippingList := subOrder["shipping_list"].([]interface{})
	shippingItem := shippingList[0].(map[string]interface{})
	require.Equal(t, "商户子单菜品", shippingItem["item_desc"])
}

func captureUploadShippingInfoBody(t *testing.T, call func(*Client) error) map[string]interface{} {
	t.Helper()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetWechatAccessToken(gomock.Any(), gomock.Eq("mp")).
		Times(1).
		Return(db.WechatAccessToken{
			AppType:     "mp",
			AccessToken: "cached-token",
			ExpiresAt:   time.Now().Add(30 * time.Minute),
		}, nil)

	var captured map[string]interface{}
	client := NewClient("test_app_id", "test_app_secret", store)
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodPost, req.Method)
			require.Contains(t, req.URL.String(), "access_token=cached-token")
			requestBody, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(requestBody, &captured))
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"errcode":0,"errmsg":"ok"}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	err := call(client)
	require.NoError(t, err)
	require.NotNil(t, captured)
	return captured
}

func bodyShippingItem(t *testing.T, body map[string]interface{}) map[string]interface{} {
	t.Helper()

	shippingList := body["shipping_list"].([]interface{})
	require.Len(t, shippingList, 1)
	return shippingList[0].(map[string]interface{})
}
