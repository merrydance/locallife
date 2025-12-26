package wechat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const (
	msgSecCheckURL = "https://api.weixin.qq.com/wxa/msg_sec_check?access_token=%s"
)

// ErrRiskyTextContent 表示文本内容未通过微信内容安全检测。
var ErrRiskyTextContent = errors.New("risky text content")

type msgSecCheckRequest struct {
	OpenID  string `json:"openid"`
	Scene   int    `json:"scene"`
	Version int    `json:"version"`
	Content string `json:"content"`
}

type msgSecCheckResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
	Result  *struct {
		Suggest string `json:"suggest"` // pass / review / risky
		Label   int    `json:"label"`
		Scene   int    `json:"scene"`
	} `json:"result"`
	TraceID string `json:"trace_id"`
}

// MsgSecCheck 文本内容安全检测（msg_sec_check v2）。
// - openid: 发表内容的用户 openid（官方要求用户近 2 小时内访问过小程序）
// - scene: 场景值（如 2=评论）
// - content: 待检测文本
func (c *Client) MsgSecCheck(ctx context.Context, openid string, scene int, content string) error {
	if openid == "" {
		return fmt.Errorf("missing openid")
	}
	if content == "" {
		return nil
	}

	token, err := c.GetAccessToken(ctx, "mp")
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}
	url := fmt.Sprintf(msgSecCheckURL, token)

	payload, err := json.Marshal(msgSecCheckRequest{
		OpenID:  openid,
		Scene:   scene,
		Version: 2,
		Content: content,
	})
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
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	var result msgSecCheckResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}

	if result.ErrCode != 0 {
		// 87014 常见表示内容违规（不同接口/版本返回可能不同，这里兜底处理）
		if result.ErrCode == 87014 {
			return fmt.Errorf("%w: %s (code: %d)", ErrRiskyTextContent, result.ErrMsg, result.ErrCode)
		}
		return &APIError{Code: result.ErrCode, Msg: result.ErrMsg}
	}

	if result.Result != nil && result.Result.Suggest != "" && result.Result.Suggest != "pass" {
		return fmt.Errorf("%w: suggest=%s label=%d scene=%d", ErrRiskyTextContent, result.Result.Suggest, result.Result.Label, result.Result.Scene)
	}

	return nil
}
