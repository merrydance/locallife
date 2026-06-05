package cloudprint

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/merrydance/locallife/util"
)

type ShangpengClient struct {
	baseURL    string
	appID      string
	appSecret  string
	httpClient *http.Client
	now        func() string
}

type shangpengResponse struct {
	ErrorCode *int           `json:"errorcode"`
	ErrorMsg  string         `json:"errormsg"`
	Message   string         `json:"message"`
	Msg       string         `json:"msg"`
	Raw       map[string]any `json:"-"`
}

func NewShangpengClientFromConfig(config util.Config) Client {
	return newShangpengClientFromConfig(config, nil)
}

func newShangpengClientFromConfig(config util.Config, now func() string) Client {
	if !config.ShangpengEnabled ||
		strings.TrimSpace(config.ShangpengAppID) == "" ||
		strings.TrimSpace(config.ShangpengAppSecret) == "" {
		return nil
	}

	timeout := config.ShangpengHTTPTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	baseURL := strings.TrimRight(strings.TrimSpace(config.ShangpengAPIBaseURL), "/")
	if baseURL == "" {
		baseURL = "https://open.spyun.net"
	}
	if now == nil {
		now = func() string { return strconv.FormatInt(time.Now().Unix(), 10) }
	}

	return &ShangpengClient{
		baseURL:   baseURL,
		appID:     strings.TrimSpace(config.ShangpengAppID),
		appSecret: strings.TrimSpace(config.ShangpengAppSecret),
		httpClient: &http.Client{
			Timeout: timeout,
		},
		now: now,
	}
}

func (c *ShangpengClient) PrintResultCallbackEnabled() bool {
	return false
}

func (c *ShangpengClient) AddPrinter(ctx context.Context, input AddPrinterInput) error {
	if strings.TrimSpace(input.SN) == "" || strings.TrimSpace(input.Key) == "" || strings.TrimSpace(input.Business) == "" {
		return fmt.Errorf("printer sn, key and business are required")
	}

	_, err := c.call(ctx, http.MethodPost, "/v1/printer/add", url.Values{
		"business": []string{input.Business},
		"sn":       []string{input.SN},
		"pkey":     []string{input.Key},
		"name":     []string{input.Name},
	})
	return err
}

func (c *ShangpengClient) RemovePrinter(ctx context.Context, input RemovePrinterInput) error {
	if strings.TrimSpace(input.SN) == "" {
		return fmt.Errorf("printer sn is required")
	}

	params := url.Values{"sn": []string{input.SN}}
	if strings.TrimSpace(input.Business) != "" {
		params.Set("business", input.Business)
	}
	_, err := c.call(ctx, http.MethodDelete, "/v1/printer/delete", params)
	return err
}

func (c *ShangpengClient) Print(ctx context.Context, input PrintInput) (string, error) {
	if strings.TrimSpace(input.SN) == "" {
		return "", fmt.Errorf("printer sn is required")
	}
	if strings.TrimSpace(input.Content) == "" {
		return "", fmt.Errorf("print content is required")
	}

	copies := input.Copies
	if copies <= 0 {
		copies = 1
	}

	response, err := c.call(ctx, http.MethodPost, "/v1/printer/print", url.Values{
		"sn":      []string{input.SN},
		"content": []string{input.Content},
		"times":   []string{strconv.Itoa(copies)},
	})
	if err != nil {
		return "", err
	}

	orderID := strings.TrimSpace(anyToString(response.Raw["id"]))
	if orderID == "" {
		return "", fmt.Errorf("shangpeng response missing print order id")
	}
	return orderID, nil
}

func (c *ShangpengClient) QueryOrderState(ctx context.Context, orderID string) (bool, error) {
	if strings.TrimSpace(orderID) == "" {
		return false, fmt.Errorf("order id is required")
	}

	response, err := c.call(ctx, http.MethodGet, "/v1/printer/order/status", url.Values{
		"id": []string{orderID},
	})
	if err != nil {
		return false, err
	}

	status := anyToBoolPtr(response.Raw["status"])
	if status == nil {
		return false, fmt.Errorf("shangpeng response missing order status")
	}
	return *status, nil
}

func (c *ShangpengClient) QueryPrinterStatus(ctx context.Context, sn string) (string, error) {
	info, err := c.GetPrinterInfo(ctx, sn)
	if err != nil {
		return "", err
	}
	return info.Status, nil
}

func (c *ShangpengClient) GetPrinterInfo(ctx context.Context, sn string) (PrinterInfo, error) {
	if strings.TrimSpace(sn) == "" {
		return PrinterInfo{}, fmt.Errorf("printer sn is required")
	}

	response, err := c.call(ctx, http.MethodGet, "/v1/printer/info", url.Values{
		"sn": []string{sn},
	})
	if err != nil {
		return PrinterInfo{}, err
	}

	info := PrinterInfo{
		Model:  anyToString(response.Raw["model"]),
		Status: mapShangpengPrinterStatus(response.Raw["online"], response.Raw["status"]),
	}
	if info.Status == "" {
		return PrinterInfo{}, fmt.Errorf("shangpeng response missing printer status")
	}
	return info, nil
}

func (c *ShangpengClient) call(ctx context.Context, method string, path string, params url.Values) (shangpengResponse, error) {
	if c == nil {
		return shangpengResponse{}, fmt.Errorf("shangpeng client is not configured")
	}

	params = cloneValues(params)
	params.Set("appid", c.appID)
	params.Set("timestamp", c.now())
	params.Set("sign", BuildShangpengSign(params, c.appSecret))

	reqURL := c.baseURL + path
	var body io.Reader
	if method == http.MethodGet || method == http.MethodDelete {
		reqURL += "?" + params.Encode()
	} else {
		body = strings.NewReader(params.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return shangpengResponse{}, fmt.Errorf("create shangpeng request: %w", err)
	}
	if method != http.MethodGet && method != http.MethodDelete {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return shangpengResponse{}, fmt.Errorf("call shangpeng: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return shangpengResponse{}, fmt.Errorf("read shangpeng response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return shangpengResponse{}, fmt.Errorf("shangpeng http status %d: %s", resp.StatusCode, strings.TrimSpace(string(rawBody)))
	}

	var response shangpengResponse
	if err := json.Unmarshal(rawBody, &response); err != nil {
		return shangpengResponse{}, fmt.Errorf("decode shangpeng response: %w", err)
	}
	if err := json.Unmarshal(rawBody, &response.Raw); err != nil {
		return shangpengResponse{}, fmt.Errorf("decode shangpeng response object: %w", err)
	}
	if response.ErrorCode == nil {
		return shangpengResponse{}, fmt.Errorf("shangpeng response missing errorcode")
	}
	if *response.ErrorCode != 0 {
		message := strings.TrimSpace(response.ErrorMsg)
		if message == "" {
			message = strings.TrimSpace(response.Message)
		}
		if message == "" {
			message = strings.TrimSpace(response.Msg)
		}
		return shangpengResponse{}, fmt.Errorf("shangpeng api error %d: %s", *response.ErrorCode, message)
	}
	return response, nil
}

func BuildShangpengSign(values url.Values, appSecret string) string {
	keys := make([]string, 0, len(values))
	for key, list := range values {
		if key == "sign" || len(list) == 0 || strings.TrimSpace(list[0]) == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys)+1)
	for _, key := range keys {
		parts = append(parts, key+"="+values.Get(key))
	}
	parts = append(parts, "appsecret="+appSecret)
	digest := md5.Sum([]byte(strings.Join(parts, "&")))
	return strings.ToUpper(hex.EncodeToString(digest[:]))
}

func mapShangpengPrinterStatus(online any, status any) string {
	onlineText := strings.TrimSpace(strings.ToLower(anyToString(online)))
	statusText := strings.TrimSpace(strings.ToLower(anyToString(status)))
	if onlineText == "" || statusText == "" {
		return ""
	}
	if onlineText == "0" || onlineText == "false" {
		return "offline"
	}
	if statusText != "0" && statusText != "false" {
		return "abnormal"
	}
	return "online"
}
