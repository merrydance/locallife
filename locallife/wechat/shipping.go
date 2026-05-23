package wechat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	uploadShippingInfoURL         = "https://api.weixin.qq.com/wxa/sec/order/upload_shipping_info?access_token=%s"
	uploadCombinedShippingInfoURL = "https://api.weixin.qq.com/wxa/sec/order/upload_combined_shipping_info?access_token=%s"
)

// UploadShippingInfoRequest 单商户支付发货信息上传请求（用于收付通子单）
type UploadShippingInfoRequest struct {
	// TransactionID 微信支付订单号（与 OutTradeNo 二选一，优先使用）
	TransactionID string
	// OutTradeNo 商户子单号（当 TransactionID 为空时使用）
	OutTradeNo string
	// MchID 商户号。使用 out_trade_no 作为订单标识时必填。
	MchID string
	// PayerOpenID 支付者 openid
	PayerOpenID string
	// ItemDesc 商品信息，例如商品名称摘要。
	ItemDesc string
	// NotifyURL 保留兼容旧调用；微信发货结算事件通过小程序消息推送 URL 配置接收。
	NotifyURL string
	// UploadTime 发货时间
	UploadTime time.Time
}

// UploadCombinedShippingInfoRequest 合单支付发货信息上传请求
type UploadCombinedShippingInfoRequest struct {
	// CombineOutTradeNo 合单商户订单号
	CombineOutTradeNo string
	// PayerOpenID 支付者 openid
	PayerOpenID string
	// ItemDesc 商品信息默认值，子单未设置时使用。
	ItemDesc string
	// NotifyURL 保留兼容旧调用；微信发货结算事件通过小程序消息推送 URL 配置接收。
	NotifyURL string
	// UploadTime 发货时间
	UploadTime time.Time
	// SubOrders 需要上报的子订单列表（通常只含当前订单的子单）
	SubOrders []ShippingSubOrder
}

// ShippingSubOrder 合单发货子订单信息
type ShippingSubOrder struct {
	// MchID 子商户号
	MchID string
	// OutTradeNo 子单商户订单号
	OutTradeNo string
	// ItemDesc 商品信息，例如商品名称摘要。
	ItemDesc string
}

type shippingAPIResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

// UploadShippingInfo 上传单笔订单发货信息（同城配送，logistics_type=2）
// 在骑手取货后调用，满足微信平台「发货信息管理」合规要求。
func (c *Client) UploadShippingInfo(ctx context.Context, req *UploadShippingInfoRequest) error {
	token, err := c.GetAccessToken(ctx, "mp")
	if err != nil {
		return fmt.Errorf("get access token: %w", err)
	}

	// 构建 order_key：优先使用 transaction_id
	var orderKey map[string]interface{}
	if req.TransactionID != "" {
		orderKey = map[string]interface{}{
			"order_number_type": 2,
			"transaction_id":    req.TransactionID,
		}
	} else {
		orderKey = map[string]interface{}{
			"order_number_type": 1,
			"mchid":             req.MchID,
			"out_trade_no":      req.OutTradeNo,
		}
	}

	body := map[string]interface{}{
		"order_key":      orderKey,
		"logistics_type": 2, // 同城配送
		"delivery_mode":  1, // 统一配送
		"shipping_list": []map[string]string{
			{"tracking_no": "", "express_company": "", "item_desc": req.ItemDesc},
		},
		"upload_time": req.UploadTime.Format(time.RFC3339),
		"payer": map[string]string{
			"openid": req.PayerOpenID,
		},
	}

	return c.doShippingAPICall(ctx, fmt.Sprintf(uploadShippingInfoURL, token), body)
}

// UploadCombinedShippingInfo 上传合单支付发货信息（同城配送，logistics_type=2）
func (c *Client) UploadCombinedShippingInfo(ctx context.Context, req *UploadCombinedShippingInfoRequest) error {
	token, err := c.GetAccessToken(ctx, "mp")
	if err != nil {
		return fmt.Errorf("get access token: %w", err)
	}

	subOrders := make([]map[string]interface{}, len(req.SubOrders))
	for i, sub := range req.SubOrders {
		itemDesc := sub.ItemDesc
		if itemDesc == "" {
			itemDesc = req.ItemDesc
		}
		subOrders[i] = map[string]interface{}{
			"mchid":          sub.MchID,
			"out_trade_no":   sub.OutTradeNo,
			"logistics_type": 2, // 同城配送
			"delivery_mode":  1, // 统一配送
			"deadline_type":  1, // 不承诺时效
			"shipping_list": []map[string]string{
				{"tracking_no": "", "express_company": "", "item_desc": itemDesc},
			},
		}
	}

	body := map[string]interface{}{
		"combine_out_trade_no": req.CombineOutTradeNo,
		"payer": map[string]string{
			"openid": req.PayerOpenID,
		},
		"upload_time": req.UploadTime.Format(time.RFC3339),
		"sub_orders":  subOrders,
	}

	return c.doShippingAPICall(ctx, fmt.Sprintf(uploadCombinedShippingInfoURL, token), body)
}

func (c *Client) doShippingAPICall(ctx context.Context, url string, body map[string]interface{}) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	var result shippingAPIResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}

	if result.ErrCode != 0 {
		return &APIError{Code: result.ErrCode, Msg: result.ErrMsg}
	}

	return nil
}
