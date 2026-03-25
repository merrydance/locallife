package wechat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const mediaCheckAsyncURL = "https://api.weixin.qq.com/wxa/media_check_async?access_token=%s"

type mediaCheckAsyncPayload struct {
	MediaURL  string `json:"media_url"`
	MediaType int    `json:"media_type"`
	Version   int    `json:"version,omitempty"`
	OpenID    string `json:"openid,omitempty"`
	Scene     int    `json:"scene,omitempty"`
}

type mediaCheckAsyncAPIResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
	TraceID string `json:"trace_id"`
}

func (c *Client) MediaCheckAsync(ctx context.Context, req MediaCheckAsyncRequest) (*MediaCheckAsyncResponse, error) {
	if req.MediaURL == "" {
		return nil, fmt.Errorf("missing media url")
	}
	if req.MediaType != SecCheckMediaTypeVoice && req.MediaType != SecCheckMediaTypeImage {
		return nil, fmt.Errorf("unsupported media type: %d", req.MediaType)
	}
	if req.Version == 0 {
		req.Version = 2
	}

	token, err := c.GetAccessToken(ctx, "mp")
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	payload, err := json.Marshal(mediaCheckAsyncPayload{
		MediaURL:  req.MediaURL,
		MediaType: req.MediaType,
		Version:   req.Version,
		OpenID:    req.OpenID,
		Scene:     req.Scene,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf(mediaCheckAsyncURL, token), bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result mediaCheckAsyncAPIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if result.ErrCode != 0 {
		return nil, &APIError{Code: result.ErrCode, Msg: result.ErrMsg}
	}
	if result.TraceID == "" {
		return nil, fmt.Errorf("missing trace id in media_check_async response")
	}

	return &MediaCheckAsyncResponse{TraceID: result.TraceID}, nil
}
