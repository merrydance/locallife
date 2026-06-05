package cloudprint

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/merrydance/locallife/util"
)

type YilianyunOAuthClient struct {
	baseURL      string
	clientID     string
	clientSecret string
	redirectURI  string
	httpClient   *http.Client
	now          func() string
	requestID    func() string
}

type YilianyunAuthorizationToken struct {
	AccessToken      string
	RefreshToken     string
	MachineCode      string
	ExpiresInSeconds int64
}

type YilianyunScannedPrinterAuthorizationInput struct {
	MachineCode string
	QRKey       string
	MSign       string
}

type yilianyunOAuthResponse struct {
	Error            *string         `json:"error"`
	ErrorDescription string          `json:"error_description"`
	Body             json.RawMessage `json:"body"`
}

type yilianyunTokenBody struct {
	AccessToken  string          `json:"access_token"`
	RefreshToken string          `json:"refresh_token"`
	MachineCode  string          `json:"machine_code"`
	ExpiresIn    json.RawMessage `json:"expires_in"`
}

func NewYilianyunOAuthClientFromConfig(config util.Config) *YilianyunOAuthClient {
	return newYilianyunOAuthClientFromConfig(config, nil, nil)
}

func newYilianyunOAuthClientFromConfig(config util.Config, now func() string, requestID func() string) *YilianyunOAuthClient {
	if !config.YilianyunEnabled ||
		strings.TrimSpace(config.YilianyunAPIBaseURL) == "" ||
		strings.TrimSpace(config.YilianyunAppID) == "" ||
		strings.TrimSpace(config.YilianyunAppSecret) == "" {
		return nil
	}

	timeout := config.YilianyunHTTPTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	baseURL := strings.TrimRight(strings.TrimSpace(config.YilianyunAPIBaseURL), "/")
	if now == nil {
		now = func() string { return strconv.FormatInt(time.Now().Unix(), 10) }
	}
	if requestID == nil {
		requestID = func() string { return uuid.NewString() }
	}

	return &YilianyunOAuthClient{
		baseURL:      baseURL,
		clientID:     strings.TrimSpace(config.YilianyunAppID),
		clientSecret: strings.TrimSpace(config.YilianyunAppSecret),
		redirectURI:  strings.TrimSpace(config.YilianyunAuthCallbackURL),
		httpClient: &http.Client{
			Timeout: timeout,
		},
		now:       now,
		requestID: requestID,
	}
}

func (c *YilianyunOAuthClient) BuildAuthorizeURL(state string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("yilianyun oauth client is not configured")
	}
	if strings.TrimSpace(c.redirectURI) == "" {
		return "", fmt.Errorf("yilianyun auth callback url is required")
	}
	if strings.TrimSpace(state) == "" {
		return "", fmt.Errorf("state is required")
	}

	parsed, err := url.Parse(c.baseURL + "/oauth/authorize")
	if err != nil {
		return "", fmt.Errorf("build yilianyun authorize url: %w", err)
	}
	query := parsed.Query()
	query.Set("response_type", "code")
	query.Set("client_id", c.clientID)
	query.Set("redirect_uri", c.redirectURI)
	query.Set("state", strings.TrimSpace(state))
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (c *YilianyunOAuthClient) ExchangeAuthorizationCode(ctx context.Context, code string) (YilianyunAuthorizationToken, error) {
	if strings.TrimSpace(code) == "" {
		return YilianyunAuthorizationToken{}, fmt.Errorf("authorization code is required")
	}

	return c.callTokenEndpoint(ctx, "/oauth/oauth", url.Values{
		"grant_type": []string{"authorization_code"},
		"code":       []string{strings.TrimSpace(code)},
		"scope":      []string{"all"},
	}, true)
}

func (c *YilianyunOAuthClient) AuthorizeScannedPrinter(ctx context.Context, input YilianyunScannedPrinterAuthorizationInput) (YilianyunAuthorizationToken, error) {
	machineCode := strings.TrimSpace(input.MachineCode)
	qrKey := strings.TrimSpace(input.QRKey)
	msign := strings.TrimSpace(input.MSign)

	if machineCode == "" {
		return YilianyunAuthorizationToken{}, fmt.Errorf("machine_code is required")
	}
	if (qrKey == "" && msign == "") || (qrKey != "" && msign != "") {
		return YilianyunAuthorizationToken{}, fmt.Errorf("exactly one of qr_key or msign is required")
	}

	params := url.Values{
		"machine_code": []string{machineCode},
		"scope":        []string{"all"},
	}
	if qrKey != "" {
		params.Set("qr_key", qrKey)
	}
	if msign != "" {
		params.Set("msign", msign)
	}

	token, err := c.callTokenEndpoint(ctx, "/oauth/scancodemodel", params, false)
	if err != nil {
		return YilianyunAuthorizationToken{}, err
	}
	token.MachineCode = machineCode
	return token, nil
}

func (c *YilianyunOAuthClient) callTokenEndpoint(ctx context.Context, path string, params url.Values, requireMachineCode bool) (YilianyunAuthorizationToken, error) {
	if c == nil {
		return YilianyunAuthorizationToken{}, fmt.Errorf("yilianyun oauth client is not configured")
	}

	timestamp := c.now()
	params = cloneValues(params)
	params.Set("client_id", c.clientID)
	params.Set("timestamp", timestamp)
	params.Set("id", c.requestID())
	params.Set("sign", BuildYilianyunSign(c.clientID, timestamp, c.clientSecret))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, strings.NewReader(params.Encode()))
	if err != nil {
		return YilianyunAuthorizationToken{}, fmt.Errorf("create yilianyun oauth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return YilianyunAuthorizationToken{}, fmt.Errorf("call yilianyun oauth: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return YilianyunAuthorizationToken{}, fmt.Errorf("read yilianyun oauth response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return YilianyunAuthorizationToken{}, fmt.Errorf("yilianyun oauth http status %d", resp.StatusCode)
	}

	return parseYilianyunTokenResponse(rawBody, requireMachineCode)
}

func parseYilianyunTokenResponse(rawBody []byte, requireMachineCode bool) (YilianyunAuthorizationToken, error) {
	decoder := json.NewDecoder(bytes.NewReader(rawBody))
	decoder.UseNumber()

	var response yilianyunOAuthResponse
	if err := decoder.Decode(&response); err != nil {
		return YilianyunAuthorizationToken{}, fmt.Errorf("decode yilianyun oauth response: %w", err)
	}
	if response.Error == nil {
		return YilianyunAuthorizationToken{}, fmt.Errorf("yilianyun oauth response missing error")
	}
	errorCode := strings.TrimSpace(*response.Error)
	if errorCode != "0" {
		return YilianyunAuthorizationToken{}, fmt.Errorf("yilianyun api error %s", errorCode)
	}
	if len(response.Body) == 0 || string(response.Body) == "null" {
		return YilianyunAuthorizationToken{}, fmt.Errorf("yilianyun oauth response missing body")
	}

	var body yilianyunTokenBody
	bodyDecoder := json.NewDecoder(bytes.NewReader(response.Body))
	bodyDecoder.UseNumber()
	if err := bodyDecoder.Decode(&body); err != nil {
		return YilianyunAuthorizationToken{}, fmt.Errorf("decode yilianyun oauth body: %w", err)
	}

	token := YilianyunAuthorizationToken{
		AccessToken:  strings.TrimSpace(body.AccessToken),
		RefreshToken: strings.TrimSpace(body.RefreshToken),
		MachineCode:  strings.TrimSpace(body.MachineCode),
	}
	if token.AccessToken == "" {
		return YilianyunAuthorizationToken{}, fmt.Errorf("yilianyun oauth response missing access_token")
	}
	if token.RefreshToken == "" {
		return YilianyunAuthorizationToken{}, fmt.Errorf("yilianyun oauth response missing refresh_token")
	}
	if requireMachineCode && token.MachineCode == "" {
		return YilianyunAuthorizationToken{}, fmt.Errorf("yilianyun oauth response missing machine_code")
	}
	expiresIn, err := parseYilianyunExpiresIn(body.ExpiresIn)
	if err != nil {
		return YilianyunAuthorizationToken{}, err
	}
	token.ExpiresInSeconds = expiresIn
	return token, nil
}

func parseYilianyunExpiresIn(raw json.RawMessage) (int64, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, fmt.Errorf("yilianyun oauth response missing expires_in")
	}

	var number json.Number
	if err := json.Unmarshal(raw, &number); err == nil {
		parsed, parseErr := number.Int64()
		if parseErr != nil || parsed <= 0 {
			return 0, fmt.Errorf("yilianyun oauth response invalid expires_in")
		}
		return parsed, nil
	}

	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return 0, fmt.Errorf("yilianyun oauth response invalid expires_in")
	}
	parsed, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("yilianyun oauth response invalid expires_in")
	}
	return parsed, nil
}

func BuildYilianyunSign(clientID, timestamp, clientSecret string) string {
	digest := md5.Sum([]byte(clientID + timestamp + clientSecret))
	return strings.ToLower(hex.EncodeToString(digest[:]))
}
