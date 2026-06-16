package cloudprint

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/merrydance/locallife/util"
)

type Client interface {
	AddPrinter(ctx context.Context, input AddPrinterInput) error
	RemovePrinter(ctx context.Context, input RemovePrinterInput) error
	Print(ctx context.Context, input PrintInput) (string, error)
	PrintResultCallbackEnabled() bool
	QueryOrderState(ctx context.Context, orderID string) (bool, error)
	QueryPrinterStatus(ctx context.Context, sn string) (string, error)
	GetPrinterInfo(ctx context.Context, sn string) (PrinterInfo, error)
}

type AddPrinterInput struct {
	SN       string
	Key      string
	Name     string
	Business string
}

type RemovePrinterInput struct {
	SN       string
	Business string
}

type PrintInput struct {
	OrderID          int64
	PrintLogID       int64
	PrinterID        int64
	MerchantID       int64
	SN               string
	Content          string
	Copies           int
	ExpiredAt        *time.Time
	TaskKey          string
	ProviderOriginID string
}

type PrinterInfo struct {
	Model      string
	Status     string
	PrintWidth string
	PrintLogo  *bool
	ScanSwitch *bool
}

type FeieyunClient struct {
	baseURL          string
	user             string
	ukey             string
	printCallbackURL string
	httpClient       *http.Client
}

type feieyunResponse struct {
	Ret  int             `json:"ret"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

type feieyunDeletePrinterData struct {
	OK []string `json:"ok"`
	No []string `json:"no"`
}

func NewFeieyunClientFromConfig(config util.Config) Client {
	if !config.FeieyunEnabled || config.FeieyunUser == "" || config.FeieyunUkey == "" {
		return nil
	}

	timeout := config.FeieyunHTTPTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	baseURL := strings.TrimRight(config.FeieyunAPIBaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.feieyun.cn"
	}

	return &FeieyunClient{
		baseURL:          baseURL,
		user:             config.FeieyunUser,
		ukey:             config.FeieyunUkey,
		printCallbackURL: resolveFeieyunPrintCallbackURL(config),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *FeieyunClient) PrintResultCallbackEnabled() bool {
	return c != nil && strings.TrimSpace(c.printCallbackURL) != ""
}

func resolveFeieyunPrintCallbackURL(config util.Config) string {
	callbackURL := strings.TrimSpace(config.FeieyunPrintCallbackURL)
	if callbackURL == "" || strings.TrimSpace(config.FeieyunCallbackPublicKeyPEM) == "" {
		return ""
	}
	return callbackURL
}

func (c *FeieyunClient) AddPrinter(ctx context.Context, input AddPrinterInput) error {
	if strings.TrimSpace(input.SN) == "" || strings.TrimSpace(input.Key) == "" {
		return fmt.Errorf("printer sn and key are required")
	}

	printerContent := strings.Join([]string{input.SN, input.Key, input.Name, "", "1"}, "#")
	_, err := c.call(ctx, "/Api/Open/printerAddlist", "Open_printerAddlist", url.Values{
		"printerContent": []string{printerContent},
	})
	return err
}

func (c *FeieyunClient) RemovePrinter(ctx context.Context, input RemovePrinterInput) error {
	if strings.TrimSpace(input.SN) == "" {
		return fmt.Errorf("printer sn is required")
	}

	response, err := c.call(ctx, "/Api/Open/printerDelList", "Open_printerDelList", url.Values{
		"snlist": []string{input.SN},
	})
	if err != nil {
		return err
	}

	if len(response.Data) == 0 || string(response.Data) == "null" {
		return nil
	}

	var result feieyunDeletePrinterData
	if err := json.Unmarshal(response.Data, &result); err != nil {
		return fmt.Errorf("decode delete printer response: %w", err)
	}
	for _, item := range result.No {
		if strings.Contains(item, input.SN) {
			return fmt.Errorf("feieyun delete printer failed: %s", item)
		}
	}

	return nil
}

func (c *FeieyunClient) Print(ctx context.Context, input PrintInput) (string, error) {
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

	params := url.Values{
		"sn":      []string{input.SN},
		"content": []string{input.Content},
		"times":   []string{strconv.Itoa(copies)},
	}
	if input.ExpiredAt != nil {
		params.Set("expired", strconv.FormatInt(input.ExpiredAt.Unix(), 10))
	}
	if c.PrintResultCallbackEnabled() {
		params.Set("backurl", strings.TrimSpace(c.printCallbackURL))
	}

	response, err := c.call(ctx, "/Api/Open/printMsg", "Open_printMsg", params)
	if err != nil {
		return "", err
	}

	var orderID string
	if len(response.Data) > 0 && string(response.Data) != "null" {
		_ = json.Unmarshal(response.Data, &orderID)
	}
	return orderID, nil
}

func (c *FeieyunClient) QueryOrderState(ctx context.Context, orderID string) (bool, error) {
	if strings.TrimSpace(orderID) == "" {
		return false, fmt.Errorf("order id is required")
	}

	response, err := c.call(ctx, "/Api/Open/queryOrderState", "Open_queryOrderState", url.Values{
		"orderid": []string{orderID},
	})
	if err != nil {
		return false, err
	}

	var printed bool
	if len(response.Data) > 0 && string(response.Data) != "null" {
		if err := json.Unmarshal(response.Data, &printed); err != nil {
			return false, fmt.Errorf("decode query order state response: %w", err)
		}
	}
	return printed, nil
}

func (c *FeieyunClient) QueryPrinterStatus(ctx context.Context, sn string) (string, error) {
	if strings.TrimSpace(sn) == "" {
		return "", fmt.Errorf("printer sn is required")
	}

	response, err := c.call(ctx, "/Api/Open/queryPrinterStatus", "Open_queryPrinterStatus", url.Values{
		"sn": []string{sn},
	})
	if err != nil {
		return "", err
	}

	var status string
	if len(response.Data) > 0 && string(response.Data) != "null" {
		if err := json.Unmarshal(response.Data, &status); err != nil {
			return "", fmt.Errorf("decode query printer status response: %w", err)
		}
	}
	return status, nil
}

func (c *FeieyunClient) GetPrinterInfo(ctx context.Context, sn string) (PrinterInfo, error) {
	if strings.TrimSpace(sn) == "" {
		return PrinterInfo{}, fmt.Errorf("printer sn is required")
	}

	response, err := c.call(ctx, "/Api/Open/printerInfo", "Open_printerInfo", url.Values{
		"sn": []string{sn},
	})
	if err != nil {
		return PrinterInfo{}, err
	}

	info := PrinterInfo{}
	if len(response.Data) == 0 || string(response.Data) == "null" {
		return info, nil
	}

	var payload map[string]any
	if err := json.Unmarshal(response.Data, &payload); err != nil {
		return PrinterInfo{}, fmt.Errorf("decode printer info response: %w", err)
	}

	info.Model = anyToString(payload["model"])
	info.Status = anyToString(payload["status"])
	info.PrintLogo = anyToBoolPtr(payload["printlogo"])
	info.ScanSwitch = anyToBoolPtr(payload["scanSwitch"])

	return info, nil
}

func (c *FeieyunClient) call(ctx context.Context, path string, apiName string, params url.Values) (feieyunResponse, error) {
	if c == nil {
		return feieyunResponse{}, fmt.Errorf("feieyun client is not configured")
	}

	stime := strconv.FormatInt(time.Now().Unix(), 10)
	params = cloneValues(params)
	params.Set("user", c.user)
	params.Set("stime", stime)
	params.Set("sig", c.sign(stime))
	params.Set("apiname", apiName)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, strings.NewReader(params.Encode()))
	if err != nil {
		return feieyunResponse{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return feieyunResponse{}, fmt.Errorf("call feieyun: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return feieyunResponse{}, fmt.Errorf("read feieyun response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return feieyunResponse{}, fmt.Errorf("feieyun http status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result feieyunResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return feieyunResponse{}, fmt.Errorf("decode feieyun response: %w", err)
	}
	if result.Ret != 0 {
		return feieyunResponse{}, fmt.Errorf("feieyun api error: %s", result.Msg)
	}

	return result, nil
}

func (c *FeieyunClient) sign(stime string) string {
	hash := sha1.Sum([]byte(c.user + c.ukey + stime))
	return hex.EncodeToString(hash[:])
}

func cloneValues(values url.Values) url.Values {
	cloned := make(url.Values, len(values))
	for key, list := range values {
		copied := make([]string, len(list))
		copy(copied, list)
		cloned[key] = copied
	}
	return cloned
}

func anyToString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case json.Number:
		return typed.String()
	case bool:
		return strconv.FormatBool(typed)
	default:
		return ""
	}
}

func anyToBoolPtr(value any) *bool {
	switch typed := value.(type) {
	case bool:
		result := typed
		return &result
	case float64:
		result := typed != 0
		return &result
	case string:
		normalized := strings.TrimSpace(strings.ToLower(typed))
		switch normalized {
		case "1", "true", "yes", "on":
			result := true
			return &result
		case "0", "false", "no", "off":
			result := false
			return &result
		default:
			return nil
		}
	default:
		return nil
	}
}
