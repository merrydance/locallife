package cloudprint

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

var ErrUnsupportedCapability = errors.New("cloud printer provider capability unsupported")

const (
	PrintStatePending   = "pending"
	PrintStateQueued    = "queued"
	PrintStateSent      = "sent"
	PrintStateAcked     = "acked"
	PrintStateSuccess   = "success"
	PrintStateFailed    = "failed"
	PrintStateTimeout   = "timeout"
	PrintStateCancelled = "cancelled"

	PrinterProviderStatusOffline    = "offline"
	PrinterProviderStatusOnline     = "online"
	PrinterProviderStatusOutOfPaper = "out_of_paper"

	PrinterPaperStatusNormal     = "normal"
	PrinterPaperStatusOutOfPaper = "out_of_paper"
	PrinterPaperStatusUnknown    = "unknown"
)

type PrintResult struct {
	ProviderOrderID  string
	ProviderOriginID string
	AcceptedAt       *time.Time
}

type PrintState struct {
	Status       string
	ErrorCode    string
	ErrorMessage string
}

type PrinterStatus struct {
	ProviderStatus string
	Online         *bool
	Working        *bool
	PaperStatus    string
}

type YilianyunClient struct {
	baseURL      string
	clientID     string
	clientSecret string
	httpClient   *http.Client
	now          func() string
	requestID    func() string
}

type YilianyunPrintInput struct {
	MachineCode      string
	AccessToken      string
	Content          string
	ProviderOriginID string
}

type YilianyunPrintResult = PrintResult

type YilianyunQueryOrderStateInput struct {
	AccessToken string
	MachineCode string
	OrderID     string
}

type YilianyunPrinterStatusInput struct {
	AccessToken string
	MachineCode string
}

type YilianyunSetPrintCallbackURLInput struct {
	AccessToken string
	MachineCode string
	CallbackURL string
	Enabled     bool
}

type yilianyunAuthorizedResponse struct {
	Error            *string         `json:"error"`
	ErrorDescription string          `json:"error_description"`
	Body             json.RawMessage `json:"body"`
}

func NewYilianyunClientFromConfig(config util.Config) *YilianyunClient {
	return newYilianyunClientFromConfig(config, nil, nil)
}

func newYilianyunClientFromConfig(config util.Config, now func() string, requestID func() string) *YilianyunClient {
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

	return &YilianyunClient{
		baseURL:      baseURL,
		clientID:     strings.TrimSpace(config.YilianyunAppID),
		clientSecret: strings.TrimSpace(config.YilianyunAppSecret),
		httpClient: &http.Client{
			Timeout: timeout,
		},
		now:       now,
		requestID: requestID,
	}
}

func (c *YilianyunClient) AddPrinter(ctx context.Context, input AddPrinterInput) error {
	return fmt.Errorf("%w: yilianyun uses open-app authorization for printer binding", ErrUnsupportedCapability)
}

func (c *YilianyunClient) RemovePrinter(ctx context.Context, input RemovePrinterInput) error {
	return fmt.Errorf("%w: yilianyun delete authorization requires stored access token", ErrUnsupportedCapability)
}

func (c *YilianyunClient) PrintResultCallbackEnabled() bool {
	return c != nil
}

func (c *YilianyunClient) Print(ctx context.Context, input YilianyunPrintInput) (PrintResult, error) {
	machineCode := strings.TrimSpace(input.MachineCode)
	accessToken := strings.TrimSpace(input.AccessToken)
	content := input.Content
	originID := strings.TrimSpace(input.ProviderOriginID)
	if machineCode == "" {
		return PrintResult{}, fmt.Errorf("machine_code is required")
	}
	if accessToken == "" {
		return PrintResult{}, fmt.Errorf("access_token is required")
	}
	if strings.TrimSpace(content) == "" {
		return PrintResult{}, fmt.Errorf("print content is required")
	}
	if originID == "" {
		return PrintResult{}, fmt.Errorf("origin_id is required")
	}
	if !isValidYilianyunOriginID(originID) {
		return PrintResult{}, fmt.Errorf("origin_id must be 1-32 alphanumeric characters")
	}

	response, err := c.callAuthorized(ctx, "/print/index", accessToken, url.Values{
		"machine_code": []string{machineCode},
		"content":      []string{content},
		"origin_id":    []string{originID},
		"idempotence":  []string{"1"},
	})
	if err != nil {
		return PrintResult{}, err
	}

	var body struct {
		ID       any `json:"id"`
		OriginID any `json:"origin_id"`
	}
	if err := decodeYilianyunBody(response.Body, &body); err != nil {
		return PrintResult{}, err
	}
	orderID := strings.TrimSpace(anyToString(body.ID))
	if orderID == "" {
		return PrintResult{}, fmt.Errorf("yilianyun response missing print order id")
	}
	return PrintResult{
		ProviderOrderID:  orderID,
		ProviderOriginID: originID,
	}, nil
}

func (c *YilianyunClient) QueryOrderState(ctx context.Context, input YilianyunQueryOrderStateInput) (bool, error) {
	state, err := c.QueryPrintState(ctx, input)
	if err != nil {
		return false, err
	}
	return state.Status == PrintStateSuccess, nil
}

func (c *YilianyunClient) QueryPrintState(ctx context.Context, input YilianyunQueryOrderStateInput) (PrintState, error) {
	machineCode := strings.TrimSpace(input.MachineCode)
	accessToken := strings.TrimSpace(input.AccessToken)
	orderID := strings.TrimSpace(input.OrderID)
	if machineCode == "" {
		return PrintState{}, fmt.Errorf("machine_code is required")
	}
	if accessToken == "" {
		return PrintState{}, fmt.Errorf("access_token is required")
	}
	if orderID == "" {
		return PrintState{}, fmt.Errorf("order id is required")
	}

	response, err := c.callAuthorized(ctx, "/printer/getorderstatus", accessToken, url.Values{
		"machine_code": []string{machineCode},
		"order_id":     []string{orderID},
	})
	if err != nil {
		return PrintState{}, err
	}

	var body struct {
		Status any `json:"status"`
	}
	if err := decodeYilianyunBody(response.Body, &body); err != nil {
		return PrintState{}, err
	}
	status := strings.TrimSpace(anyToString(body.Status))
	switch status {
	case "1":
		return PrintState{Status: PrintStateSuccess}, nil
	case "0":
		return PrintState{Status: PrintStatePending}, nil
	case "2":
		return PrintState{Status: PrintStateCancelled}, nil
	case "":
		return PrintState{}, fmt.Errorf("yilianyun response missing order status")
	default:
		return PrintState{}, fmt.Errorf("yilianyun response unknown order status")
	}
}

func (c *YilianyunClient) QueryPrinterStatus(ctx context.Context, input YilianyunPrinterStatusInput) (PrinterStatus, error) {
	machineCode := strings.TrimSpace(input.MachineCode)
	accessToken := strings.TrimSpace(input.AccessToken)
	if machineCode == "" {
		return PrinterStatus{}, fmt.Errorf("machine_code is required")
	}
	if accessToken == "" {
		return PrinterStatus{}, fmt.Errorf("access_token is required")
	}

	response, err := c.callAuthorized(ctx, "/printer/getprintstatus", accessToken, url.Values{
		"machine_code": []string{machineCode},
	})
	if err != nil {
		return PrinterStatus{}, err
	}

	var body struct {
		State any `json:"state"`
	}
	if err := decodeYilianyunBody(response.Body, &body); err != nil {
		return PrinterStatus{}, err
	}
	status := mapYilianyunPrinterStatus(body.State)
	if status.ProviderStatus == "" {
		return PrinterStatus{}, fmt.Errorf("yilianyun response missing or unknown printer status")
	}
	return status, nil
}

func (c *YilianyunClient) GetPrinterInfo(ctx context.Context, input YilianyunPrinterStatusInput) (PrinterInfo, error) {
	machineCode := strings.TrimSpace(input.MachineCode)
	accessToken := strings.TrimSpace(input.AccessToken)
	if machineCode == "" {
		return PrinterInfo{}, fmt.Errorf("machine_code is required")
	}
	if accessToken == "" {
		return PrinterInfo{}, fmt.Errorf("access_token is required")
	}

	response, err := c.callAuthorized(ctx, "/printer/printinfo", accessToken, url.Values{
		"machine_code": []string{machineCode},
	})
	if err != nil {
		return PrinterInfo{}, err
	}

	var body struct {
		Version    any `json:"version"`
		PrintWidth any `json:"print_width"`
	}
	if err := decodeYilianyunBody(response.Body, &body); err != nil {
		return PrinterInfo{}, err
	}
	info := PrinterInfo{
		Model:      strings.TrimSpace(anyToString(body.Version)),
		PrintWidth: strings.TrimSpace(anyToString(body.PrintWidth)),
	}
	if info.Model == "" && info.PrintWidth == "" {
		return PrinterInfo{}, fmt.Errorf("yilianyun response missing printer info")
	}
	return info, nil
}

func (c *YilianyunClient) SetPrintCallbackURL(ctx context.Context, input YilianyunSetPrintCallbackURLInput) error {
	machineCode := strings.TrimSpace(input.MachineCode)
	accessToken := strings.TrimSpace(input.AccessToken)
	callbackURL := strings.TrimSpace(input.CallbackURL)
	if machineCode == "" {
		return fmt.Errorf("machine_code is required")
	}
	if accessToken == "" {
		return fmt.Errorf("access_token is required")
	}
	if callbackURL == "" {
		return fmt.Errorf("callback url is required")
	}
	parsedCallbackURL, err := url.Parse(callbackURL)
	if err != nil {
		return fmt.Errorf("callback url is invalid: %w", err)
	}
	if parsedCallbackURL.Scheme != "http" && parsedCallbackURL.Scheme != "https" {
		return fmt.Errorf("callback url must start with http:// or https://")
	}
	if parsedCallbackURL.Host == "" {
		return fmt.Errorf("callback url host is required")
	}

	status := "close"
	if input.Enabled {
		status = "open"
	}
	_, callErr := c.callAuthorizedNoBody(ctx, "/oauth/setpushurl", accessToken, url.Values{
		"machine_code": []string{machineCode},
		"cmd":          []string{"oauth_finish"},
		"url":          []string{callbackURL},
		"status":       []string{status},
	})
	return callErr
}

func (c *YilianyunClient) callAuthorized(ctx context.Context, path string, accessToken string, params url.Values) (yilianyunAuthorizedResponse, error) {
	response, err := c.callAuthorizedNoBody(ctx, path, accessToken, params)
	if err != nil {
		return yilianyunAuthorizedResponse{}, err
	}
	if len(response.Body) == 0 || string(response.Body) == "null" {
		return yilianyunAuthorizedResponse{}, fmt.Errorf("yilianyun response missing body")
	}
	return response, nil
}

func (c *YilianyunClient) callAuthorizedNoBody(ctx context.Context, path string, accessToken string, params url.Values) (yilianyunAuthorizedResponse, error) {
	if c == nil {
		return yilianyunAuthorizedResponse{}, fmt.Errorf("yilianyun client is not configured")
	}

	timestamp := c.now()
	params = cloneValues(params)
	params.Set("client_id", c.clientID)
	params.Set("access_token", accessToken)
	params.Set("timestamp", timestamp)
	params.Set("id", c.requestID())
	params.Set("sign", BuildYilianyunSign(c.clientID, timestamp, c.clientSecret))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, strings.NewReader(params.Encode()))
	if err != nil {
		return yilianyunAuthorizedResponse{}, fmt.Errorf("create yilianyun request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return yilianyunAuthorizedResponse{}, fmt.Errorf("call yilianyun: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return yilianyunAuthorizedResponse{}, fmt.Errorf("read yilianyun response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return yilianyunAuthorizedResponse{}, fmt.Errorf("yilianyun http status %d", resp.StatusCode)
	}

	response, err := parseYilianyunAuthorizedResponse(rawBody)
	if err != nil {
		return yilianyunAuthorizedResponse{}, err
	}
	return response, nil
}

func parseYilianyunAuthorizedResponse(rawBody []byte) (yilianyunAuthorizedResponse, error) {
	decoder := json.NewDecoder(bytes.NewReader(rawBody))
	decoder.UseNumber()

	var response yilianyunAuthorizedResponse
	if err := decoder.Decode(&response); err != nil {
		return yilianyunAuthorizedResponse{}, fmt.Errorf("decode yilianyun response: %w", err)
	}
	if response.Error == nil {
		return yilianyunAuthorizedResponse{}, fmt.Errorf("yilianyun response missing error")
	}
	errorCode := strings.TrimSpace(*response.Error)
	if errorCode != "0" {
		return yilianyunAuthorizedResponse{}, fmt.Errorf("yilianyun api error %s", errorCode)
	}
	return response, nil
}

func decodeYilianyunBody(raw json.RawMessage, dst any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(dst); err != nil {
		return fmt.Errorf("decode yilianyun response body: %w", err)
	}
	return nil
}

func mapYilianyunPrinterStatus(value any) PrinterStatus {
	switch strings.TrimSpace(anyToString(value)) {
	case "0":
		return PrinterStatus{
			ProviderStatus: PrinterProviderStatusOffline,
			Online:         boolPtr(false),
			PaperStatus:    PrinterPaperStatusUnknown,
		}
	case "1":
		return PrinterStatus{
			ProviderStatus: PrinterProviderStatusOnline,
			Online:         boolPtr(true),
			PaperStatus:    PrinterPaperStatusNormal,
		}
	case "2":
		return PrinterStatus{
			ProviderStatus: PrinterProviderStatusOutOfPaper,
			Online:         boolPtr(true),
			PaperStatus:    PrinterPaperStatusOutOfPaper,
		}
	default:
		return PrinterStatus{}
	}
}

func boolPtr(value bool) *bool {
	return &value
}

func isValidYilianyunOriginID(value string) bool {
	if len(value) == 0 || len(value) > 32 {
		return false
	}
	for _, ch := range value {
		if (ch < '0' || ch > '9') && (ch < 'A' || ch > 'Z') && (ch < 'a' || ch > 'z') {
			return false
		}
	}
	return true
}
