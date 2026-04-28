package wechat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"strings"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
)

const (
	// 小程序码API端点
	getWXACodeUnlimitedURL = "https://api.weixin.qq.com/wxa/getwxacodeunlimit"
)

var pngSignature = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
var jpegSignature = []byte{0xff, 0xd8, 0xff}

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
	accessToken, err := c.GetAccessToken(ctx, "mp")
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	body, err := c.doWXACodeRequest(ctx, accessToken, req)
	if err != nil {
		// 检查是否是 40001/40014 token 错误
		if apiErr, ok := err.(*APIError); ok && (apiErr.Code == 40001 || apiErr.Code == 40014) {
			// 强制刷新 token 并重试
			newTokenResp, fetchErr := c.fetchAccessToken(ctx)
			if fetchErr != nil {
				return nil, fmt.Errorf("failed to refresh access token: %w", fetchErr)
			}

			// 更新缓存
			expiresAt := time.Now().Add(time.Duration(newTokenResp.ExpiresIn) * time.Second)
			_, _ = c.store.UpsertWechatAccessToken(ctx, db.UpsertWechatAccessTokenParams{
				AppType:     "mp",
				AccessToken: newTokenResp.AccessToken,
				ExpiresAt:   expiresAt,
			})

			// 重试
			body, err = c.doWXACodeRequest(ctx, newTokenResp.AccessToken, req)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	return body, nil
}

// doWXACodeRequest 执行二维码生成请求
func (c *Client) doWXACodeRequest(ctx context.Context, accessToken string, req *WXACodeRequest) ([]byte, error) {
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

	if bytes.HasPrefix(body, pngSignature) {
		return body, nil
	}

	var errResp WXACodeResponse
	if err := json.Unmarshal(body, &errResp); err == nil {
		if errResp.ErrCode != 0 {
			return nil, &APIError{Code: errResp.ErrCode, Msg: errResp.ErrMsg}
		}
		return nil, fmt.Errorf("unexpected wxa code json response: %s", strings.TrimSpace(errResp.ErrMsg))
	}

	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if bytes.HasPrefix(body, jpegSignature) || strings.HasPrefix(strings.ToLower(contentType), "image/jpeg") {
		pngData, err := normalizeWXACodeJPEGToPNG(body)
		if err != nil {
			return nil, err
		}
		return pngData, nil
	}

	if contentType != "" {
		return nil, fmt.Errorf("unexpected wxa code response content type %q", contentType)
	}

	return nil, errors.New("unexpected non-png wxa code response")
}

func normalizeWXACodeJPEGToPNG(body []byte) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to decode wxa code jpeg response: %w", err)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("failed to encode wxa code jpeg response as png: %w", err)
	}

	return buf.Bytes(), nil
}
