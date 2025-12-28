package wechat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	// 小程序码API端点
	getWXACodeUnlimitedURL = "https://api.weixin.qq.com/wxa/getwxacodeunlimit"
)

// WXACodeRequest 小程序码请求参数
type WXACodeRequest struct {
	Scene      string `json:"scene"`                 // 场景参数，最大32字符
	Page       string `json:"page,omitempty"`        // 跳转页面路径，默认主页
	CheckPath  *bool  `json:"check_path,omitempty"`  // 检查page是否存在，设为false跳过验证
	EnvVersion string `json:"env_version,omitempty"` // release/trial/develop
	Width      int    `json:"width,omitempty"`       // 二维码宽度，默认430
	AutoColor  bool   `json:"auto_color,omitempty"`  // 自动配置线条颜色
	LineColor  *RGB   `json:"line_color,omitempty"`  // 自定义线条颜色
	IsHyaline  bool   `json:"is_hyaline,omitempty"`  // 是否透明背景
}

// RGB 颜色
type RGB struct {
	R int `json:"r"`
	G int `json:"g"`
	B int `json:"b"`
}

// WXACodeResponse 小程序码API错误响应
type WXACodeResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

// GetWXACodeUnlimited 获取小程序码（不限量版本）
// 返回PNG图片数据
func (c *Client) GetWXACodeUnlimited(ctx context.Context, req *WXACodeRequest) ([]byte, error) {
	// 获取Access Token
	accessToken, err := c.GetAccessToken(ctx, "miniprogram")
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	// 构建请求
	url := fmt.Sprintf("%s?access_token=%s", getWXACodeUnlimitedURL, accessToken)

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// 检查是不是错误响应（JSON）还是成功响应（PNG图片）
	// 如果是JSON，说明报错了
	contentType := resp.Header.Get("Content-Type")
	if contentType == "application/json" || len(body) < 100 {
		var errResp WXACodeResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.ErrCode != 0 {
			return nil, &APIError{Code: errResp.ErrCode, Msg: errResp.ErrMsg}
		}
	}

	// 返回PNG图片数据
	return body, nil
}
