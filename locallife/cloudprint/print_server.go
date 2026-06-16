package cloudprint

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
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

type PrintServerClient struct {
	baseURL     string
	appID       string
	secret      string
	callbackURL string
	httpClient  *http.Client
	now         func() time.Time
	nonce       func() string
}

type printServerPrinterRequest struct {
	PrinterSN   string `json:"printer_sn"`
	PrinterKey  string `json:"printer_key"`
	PrinterName string `json:"printer_name"`
	MerchantRef string `json:"merchant_ref"`
}

type printServerPrintJobRequest struct {
	PrinterSN   string            `json:"printer_sn"`
	TaskKey     string            `json:"task_key"`
	Content     string            `json:"content"`
	Copies      int               `json:"copies"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CallbackURL string            `json:"callback_url,omitempty"`
	ExpiresAt   *time.Time        `json:"expires_at,omitempty"`
}

type printServerPrintJobResponse struct {
	PrintJobID      string            `json:"print_job_id"`
	PrinterSN       string            `json:"printer_sn"`
	TaskKey         string            `json:"task_key"`
	Status          string            `json:"status"`
	Copies          int               `json:"copies"`
	Metadata        map[string]string `json:"metadata"`
	ErrorCode       string            `json:"error_code"`
	ErrorMessage    string            `json:"error_message"`
	ContentHash     string            `json:"content_hash"`
	ContentRedacted string            `json:"content_redacted"`
}

type printServerPrinterStatusResponse struct {
	PrinterSN    string `json:"printer_sn"`
	Online       bool   `json:"online"`
	Working      bool   `json:"working"`
	Status       string `json:"status"`
	FaultCode    string `json:"fault_code"`
	FaultMessage string `json:"fault_message"`
}

func NewPrintServerClientFromConfig(config util.Config) Client {
	client := newPrintServerClientFromConfig(config, nil, nil)
	if client == nil {
		return nil
	}
	return client
}

func newPrintServerClientFromConfig(config util.Config, now func() time.Time, nonce func() string) *PrintServerClient {
	if !config.PrintServerEnabled ||
		strings.TrimSpace(config.PrintServerAPIBaseURL) == "" ||
		strings.TrimSpace(config.PrintServerAppID) == "" ||
		strings.TrimSpace(config.PrintServerSecret) == "" {
		return nil
	}

	timeout := config.PrintServerHTTPTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	if nonce == nil {
		nonce = func() string { return uuid.NewString() }
	}

	return &PrintServerClient{
		baseURL:     strings.TrimRight(strings.TrimSpace(config.PrintServerAPIBaseURL), "/"),
		appID:       strings.TrimSpace(config.PrintServerAppID),
		secret:      strings.TrimSpace(config.PrintServerSecret),
		callbackURL: resolvePrintServerCallbackURL(config),
		httpClient: &http.Client{
			Timeout: timeout,
		},
		now:   now,
		nonce: nonce,
	}
}

func (c *PrintServerClient) PrintResultCallbackEnabled() bool {
	return c != nil && strings.TrimSpace(c.callbackURL) != ""
}

func (c *PrintServerClient) callbackURLForPrint(input PrintInput) string {
	if c == nil || input.PrintLogID <= 0 {
		return ""
	}
	return c.callbackURL
}

func resolvePrintServerCallbackURL(config util.Config) string {
	callbackURL := strings.TrimSpace(config.PrintServerCallbackURL)
	if callbackURL == "" || strings.TrimSpace(config.PrintServerCallbackSigningSecret) == "" {
		return ""
	}
	return callbackURL
}

func (c *PrintServerClient) AddPrinter(ctx context.Context, input AddPrinterInput) error {
	if strings.TrimSpace(input.SN) == "" || strings.TrimSpace(input.Key) == "" {
		return fmt.Errorf("printer sn and key are required")
	}
	if strings.TrimSpace(input.Business) == "" {
		return fmt.Errorf("merchant business is required")
	}

	var response struct {
		PrinterSN string `json:"printer_sn"`
		Status    string `json:"status"`
	}
	return c.callJSON(ctx, http.MethodPost, "/v1/printers", printServerPrinterRequest{
		PrinterSN:   strings.TrimSpace(input.SN),
		PrinterKey:  input.Key,
		PrinterName: input.Name,
		MerchantRef: printServerMerchantRef(input.Business),
	}, &response)
}

func (c *PrintServerClient) RemovePrinter(ctx context.Context, input RemovePrinterInput) error {
	sn := strings.TrimSpace(input.SN)
	if sn == "" {
		return fmt.Errorf("printer sn is required")
	}

	var response struct {
		PrinterSN string `json:"printer_sn"`
		Status    string `json:"status"`
	}
	return c.callJSON(ctx, http.MethodDelete, "/v1/printers/"+url.PathEscape(sn), nil, &response)
}

func (c *PrintServerClient) Print(ctx context.Context, input PrintInput) (string, error) {
	if strings.TrimSpace(input.SN) == "" {
		return "", fmt.Errorf("printer sn is required")
	}
	if strings.TrimSpace(input.Content) == "" {
		return "", fmt.Errorf("print content is required")
	}

	taskKey := printServerTaskKey(input)
	if taskKey == "" {
		return "", fmt.Errorf("print server task key is required")
	}
	copies := input.Copies
	if copies <= 0 {
		copies = 1
	}

	var response printServerPrintJobResponse
	err := c.callJSON(ctx, http.MethodPost, "/v1/print-jobs", printServerPrintJobRequest{
		PrinterSN:   strings.TrimSpace(input.SN),
		TaskKey:     taskKey,
		Content:     input.Content,
		Copies:      copies,
		Metadata:    printServerPrintMetadata(input),
		CallbackURL: c.callbackURLForPrint(input),
		ExpiresAt:   input.ExpiredAt,
	}, &response)
	if err != nil {
		return "", err
	}

	printJobID := strings.TrimSpace(response.PrintJobID)
	if printJobID == "" {
		return "", fmt.Errorf("print server response missing print_job_id")
	}
	return printJobID, nil
}

func (c *PrintServerClient) QueryOrderState(ctx context.Context, orderID string) (bool, error) {
	state, err := c.QueryPrintState(ctx, orderID)
	if err != nil {
		return false, err
	}
	switch state.Status {
	case PrintStateSuccess:
		return true, nil
	case PrintStateFailed, PrintStateTimeout, PrintStateCancelled:
		return false, fmt.Errorf("print server terminal failure: %s", printServerFailureMessage(state))
	default:
		return false, nil
	}
}

func (c *PrintServerClient) QueryPrintState(ctx context.Context, orderID string) (PrintState, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return PrintState{}, fmt.Errorf("order id is required")
	}

	var response printServerPrintJobResponse
	if err := c.callJSON(ctx, http.MethodGet, "/v1/print-jobs/"+url.PathEscape(orderID), nil, &response); err != nil {
		return PrintState{}, err
	}

	status := strings.ToLower(strings.TrimSpace(response.Status))
	switch status {
	case PrintStateQueued, PrintStateSent, PrintStateAcked:
		return PrintState{Status: status}, nil
	case PrintStateSuccess, PrintStateFailed, PrintStateTimeout, PrintStateCancelled:
		return PrintState{
			Status:       status,
			ErrorCode:    strings.TrimSpace(response.ErrorCode),
			ErrorMessage: strings.TrimSpace(response.ErrorMessage),
		}, nil
	case "":
		return PrintState{}, fmt.Errorf("print server response missing print job status")
	default:
		return PrintState{}, fmt.Errorf("print server response unknown print job status")
	}
}

func (c *PrintServerClient) QueryPrinterStatus(ctx context.Context, sn string) (string, error) {
	status, err := c.queryPrinterStatus(ctx, sn)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(status.Status) == "" {
		return "", fmt.Errorf("print server response missing printer status")
	}
	return status.Status, nil
}

func (c *PrintServerClient) GetPrinterInfo(ctx context.Context, sn string) (PrinterInfo, error) {
	status, err := c.queryPrinterStatus(ctx, sn)
	if err != nil {
		return PrinterInfo{}, err
	}
	return PrinterInfo{Status: status.Status}, nil
}

func (c *PrintServerClient) queryPrinterStatus(ctx context.Context, sn string) (printServerPrinterStatusResponse, error) {
	sn = strings.TrimSpace(sn)
	if sn == "" {
		return printServerPrinterStatusResponse{}, fmt.Errorf("printer sn is required")
	}

	var response printServerPrinterStatusResponse
	if err := c.callJSON(ctx, http.MethodGet, "/v1/printers/"+url.PathEscape(sn)+"/status", nil, &response); err != nil {
		return printServerPrinterStatusResponse{}, err
	}
	return response, nil
}

func (c *PrintServerClient) callJSON(ctx context.Context, method string, path string, payload any, output any) error {
	if c == nil {
		return fmt.Errorf("print server client is not configured")
	}

	var bodyBytes []byte
	if payload != nil {
		var err error
		bodyBytes, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encode print server request: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create print server request: %w", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	timestamp := c.now().UTC().Format(time.RFC3339)
	nonce := c.nonce()
	req.Header.Set("X-Print-App-Id", c.appID)
	req.Header.Set("X-Print-Timestamp", timestamp)
	req.Header.Set("X-Print-Nonce", nonce)
	req.Header.Set("X-Print-Signature", BuildPrintServerSignature(method, req.URL.EscapedPath(), timestamp, nonce, bodyBytes, c.secret))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call print server: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read print server response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("print server http status %d: %s", resp.StatusCode, strings.TrimSpace(string(rawBody)))
	}
	if output == nil || len(rawBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(rawBody, output); err != nil {
		return fmt.Errorf("decode print server response: %w", err)
	}
	return nil
}

func BuildPrintServerSignature(method string, path string, timestamp string, nonce string, body []byte, secret string) string {
	bodyHash := sha256.Sum256(body)
	signingString := strings.Join([]string{
		strings.ToUpper(strings.TrimSpace(method)),
		path,
		timestamp,
		nonce,
		hex.EncodeToString(bodyHash[:]),
	}, "\n")
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(signingString))
	return hex.EncodeToString(mac.Sum(nil))
}

func BuildPrintServerCallbackSignature(secret, eventID, timestamp string, body []byte) string {
	bodyHash := sha256.Sum256(body)
	signingString := strings.Join([]string{
		eventID,
		timestamp,
		hex.EncodeToString(bodyHash[:]),
	}, "\n")
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(signingString))
	return hex.EncodeToString(mac.Sum(nil))
}

func printServerMerchantRef(business string) string {
	business = strings.TrimSpace(business)
	if strings.HasPrefix(business, "merchant:") {
		return business
	}
	return "merchant:" + business
}

func printServerTaskKey(input PrintInput) string {
	if input.PrintLogID > 0 {
		return "print_log:" + strconv.FormatInt(input.PrintLogID, 10)
	}
	if taskKey := strings.TrimSpace(input.TaskKey); taskKey != "" {
		return taskKey
	}
	if originID := strings.TrimSpace(input.ProviderOriginID); originID != "" {
		return "provider_origin:" + originID
	}
	return ""
}

func printServerPrintMetadata(input PrintInput) map[string]string {
	metadata := map[string]string{
		"content_format": "feie",
	}
	if input.OrderID > 0 {
		metadata["local_life_order_id"] = strconv.FormatInt(input.OrderID, 10)
	}
	if input.PrintLogID > 0 {
		metadata["local_life_print_log_id"] = strconv.FormatInt(input.PrintLogID, 10)
	}
	if originID := strings.TrimSpace(input.ProviderOriginID); originID != "" {
		metadata["local_life_provider_origin_id"] = originID
	}
	if scenario := strings.TrimSpace(input.TaskKey); scenario != "" {
		metadata["scenario"] = scenario
	}
	return metadata
}

func printServerFailureMessage(state PrintState) string {
	message := strings.TrimSpace(state.ErrorMessage)
	code := strings.TrimSpace(state.ErrorCode)
	if code != "" && message != "" {
		return code + ": " + message
	}
	if message != "" {
		return message
	}
	if code != "" {
		return code
	}
	if state.Status != "" {
		return state.Status
	}
	return "print server print failed"
}
