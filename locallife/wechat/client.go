package wechat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
)

const (
	// 微信API端点
	code2SessionURL   = "https://api.weixin.qq.com/sns/jscode2session"
	getAccessTokenURL = "https://api.weixin.qq.com/cgi-bin/token"

	// Access Token提前刷新时间（5分钟）
	accessTokenRefreshBuffer = 5 * time.Minute
)

// Code2SessionResponse 微信登录code2Session响应
type Code2SessionResponse struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionID    string `json:"unionid"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

// AccessTokenResponse 获取AccessToken响应
type AccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
}

// Client 微信API客户端
type Client struct {
	appID      string
	appSecret  string
	store      db.Store
	httpClient *http.Client
	mutex      sync.Mutex // 用于Access Token刷新的互斥锁
}

// NewClient 创建微信客户端
func NewClient(appID, appSecret string, store db.Store) *Client {
	return &Client{
		appID:     appID,
		appSecret: appSecret,
		store:     store,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Code2Session 使用code换取openid和session_key
func (c *Client) Code2Session(ctx context.Context, code string) (*Code2SessionResponse, error) {
	url := fmt.Sprintf("%s?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		code2SessionURL, c.appID, c.appSecret, code)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result Code2SessionResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if result.ErrCode != 0 {
		return nil, &APIError{Code: result.ErrCode, Msg: result.ErrMsg}
	}

	return &result, nil
}

// GetAccessToken 获取Access Token（带自动刷新）
func (c *Client) GetAccessToken(ctx context.Context, appType string) (string, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// 从数据库获取缓存的token
	token, err := c.store.GetWechatAccessToken(ctx, appType)
	if err == nil {
		// 检查是否需要刷新（提前5分钟）
		if time.Now().Add(accessTokenRefreshBuffer).Before(token.ExpiresAt) {
			return token.AccessToken, nil
		}
	}

	// Token不存在或即将过期，重新获取
	newToken, err := c.fetchAccessToken(ctx)
	if err != nil {
		return "", err
	}

	// 保存到数据库
	expiresAt := time.Now().Add(time.Duration(newToken.ExpiresIn) * time.Second)
	_, err = c.store.UpsertWechatAccessToken(ctx, db.UpsertWechatAccessTokenParams{
		AppType:     appType,
		AccessToken: newToken.AccessToken,
		ExpiresAt:   expiresAt,
	})
	if err != nil {
		return "", fmt.Errorf("failed to save access token: %w", err)
	}

	return newToken.AccessToken, nil
}

// fetchAccessToken 从微信服务器获取Access Token
func (c *Client) fetchAccessToken(ctx context.Context) (*AccessTokenResponse, error) {
	url := fmt.Sprintf("%s?grant_type=client_credential&appid=%s&secret=%s",
		getAccessTokenURL, c.appID, c.appSecret)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result AccessTokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if result.ErrCode != 0 {
		return nil, &APIError{Code: result.ErrCode, Msg: result.ErrMsg}
	}

	return &result, nil
}
