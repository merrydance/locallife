package baofu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	config    Config
	transport *Transport
}

var (
	errProviderUnionGWSystemResponse   = errors.New("baofu union-gw system response failed")
	errProviderAccountBusinessResponse = errors.New("baofu account business response failed")
	errProviderPublicEnvelopeFailure   = errors.New("baofu upstream returned failure")
	errProviderPublicBusinessResponse  = errors.New("baofu public business response failed")
)

func NewClient(cfg Config, httpClient HTTPDoer) (*Client, error) {
	cfg = cfg.Normalized()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Client{config: cfg, transport: NewTransport(httpClient, cfg.Timeout)}, nil
}

func (c *Client) Config() Config {
	if c == nil {
		return Config{}
	}
	return c.config
}

func (c *Client) PostAccount(ctx context.Context, method string, merchantID string, terminalID string, bizRequest any, out any) error {
	if c == nil {
		return errors.New("baofu client is not configured")
	}
	endpoint := strings.TrimRight(c.config.AccountGatewayBaseURL, "/") + "/" + strings.TrimSpace(method) + "/transReq.do"
	return c.postUnionGateway(ctx, endpoint, method, merchantID, terminalID, bizRequest, out)
}

func (c *Client) PostAggregatePay(ctx context.Context, method string, bizRequest any, out any) error {
	if c == nil {
		return errors.New("baofu client is not configured")
	}
	return c.postPublicEnvelope(ctx, c.config.AggregatePayBaseURL, method, c.config.CollectMerchantID, c.config.CollectTerminalID, bizRequest, out)
}

func (c *Client) PostMerchantReport(ctx context.Context, method string, bizRequest any, out any) error {
	if c == nil {
		return errors.New("baofu client is not configured")
	}
	return c.postPublicEnvelope(ctx, c.config.MerchantReportBaseURL, method, c.config.CollectMerchantID, c.config.CollectTerminalID, bizRequest, out)
}

func (c *Client) postUnionGateway(ctx context.Context, endpoint string, method string, merchantID string, terminalID string, bizRequest any, out any) error {
	if c.transport == nil {
		return errors.New("baofu transport is not configured")
	}
	envelope, err := NewUnionGWRequestEnvelope(merchantID, terminalID, method, bizRequest)
	if err != nil {
		return err
	}
	plaintext, err := CanonicalJSON(envelope)
	if err != nil {
		return err
	}
	content, err := EncodeUnionGWVerifyType1Content(c.config.PrivateKeyPEM, plaintext)
	if err != nil {
		return err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, http.NoBody)
	if err != nil {
		return err
	}
	query := httpReq.URL.Query()
	query.Set("memberId", strings.TrimSpace(merchantID))
	query.Set("terminalId", strings.TrimSpace(terminalID))
	query.Set("verifyType", UnionGWVerifyTypeRSA)
	query.Set("content", content)
	httpReq.URL.RawQuery = query.Encode()
	resp, err := c.transport.client.Do(httpReq)
	if err != nil {
		return providerRequestError(method, 0, "", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return providerRequestError(method, resp.StatusCode, "", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return providerRequestError(method, resp.StatusCode, "", fmt.Errorf("baofu upstream http status %d", resp.StatusCode))
	}
	responsePlaintext, err := DecodeUnionGWVerifyType1Content(c.config.BaofuPublicKeyPEM, strings.TrimSpace(string(responseBody)))
	if err != nil {
		return providerRequestError(method, resp.StatusCode, "", err)
	}
	var responseEnvelope UnionGWPlaintextEnvelope
	if err := json.Unmarshal(responsePlaintext, &responseEnvelope); err != nil {
		return providerRequestError(method, resp.StatusCode, "", err)
	}
	if err := responseEnvelope.ValidateResponse(merchantID, terminalID, method); err != nil {
		return providerRequestError(method, resp.StatusCode, responseEnvelope.Header.SystemRespCode, err)
	}
	if strings.TrimSpace(responseEnvelope.Header.SystemRespCode) != UnionGWSystemRespSuccess {
		return providerResponseError(method, resp.StatusCode, responseEnvelope.Header.SystemRespCode, responseEnvelope.Header.SystemRespDesc, errProviderUnionGWSystemResponse)
	}
	if failure := accountBusinessFailure(responseEnvelope.Body); failure.Failed {
		failure.Header = responseEnvelope.Header
		failure.Operation = method
		failure.HTTPStatus = resp.StatusCode
		return providerResponseErrorWithDiagnostic(method, resp.StatusCode, failure.Code, failure.Message, failure.SafeDiagnosticSnapshot(), errProviderAccountBusinessResponse)
	}
	if out != nil {
		if err := json.Unmarshal(responseEnvelope.Body, out); err != nil {
			return providerRequestError(method, resp.StatusCode, responseEnvelope.Header.SystemRespCode, err)
		}
		setAccountRawResponse(out, method, responseEnvelope.Body)
	}
	return nil
}

func (c *Client) postPublicEnvelope(ctx context.Context, endpoint string, method string, merchantID string, terminalID string, bizRequest any, out any) error {
	if c.transport == nil {
		return errors.New("baofu transport is not configured")
	}
	bizContent, err := CanonicalJSON(bizRequest)
	if err != nil {
		return err
	}
	signature, err := SignSHA256WithRSA(c.config.PrivateKeyPEM, bizContent)
	if err != nil {
		return err
	}
	envelope := PublicRequestEnvelope{
		MerchantID:         strings.TrimSpace(merchantID),
		TerminalID:         strings.TrimSpace(terminalID),
		Method:             strings.TrimSpace(method),
		Charset:            PublicEnvelopeCharsetUTF8,
		Version:            PublicEnvelopeVersion10,
		Format:             PublicEnvelopeFormatJSON,
		Timestamp:          PublicEnvelopeTimestamp(time.Now()),
		SignType:           SignTypeRSA,
		SignSerialNo:       c.config.SignSerialNo,
		EncryptionSerialNo: c.config.EncryptionSerialNo,
		SignString:         signature,
		BizContent:         JSONString(bizContent),
	}
	if err := envelope.Validate(); err != nil {
		return err
	}
	requestBody := envelope.FormValues().Encode()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(requestBody))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
	resp, err := c.transport.client.Do(httpReq)
	if err != nil {
		return providerRequestError(method, 0, "", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return providerRequestError(method, resp.StatusCode, "", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return providerRequestError(method, resp.StatusCode, "", fmt.Errorf("baofu upstream http status %d", resp.StatusCode))
	}
	var responseEnvelope PublicResponseEnvelope
	if err := json.Unmarshal(responseBody, &responseEnvelope); err != nil {
		return providerRequestError(method, resp.StatusCode, "", err)
	}
	if err := responseEnvelope.Validate(); err != nil {
		return providerRequestError(method, resp.StatusCode, responseEnvelope.ValidationUpstreamCode(err), err)
	}
	if strings.TrimSpace(responseEnvelope.ReturnCode) == PublicEnvelopeReturnCodeFail {
		return providerResponseError(method, resp.StatusCode, responseEnvelope.ReturnCode, responseEnvelope.ReturnMessage, errProviderPublicEnvelopeFailure)
	}
	if err := validatePublicResponseIdentity(responseEnvelope, merchantID, terminalID); err != nil {
		return providerRequestError(method, resp.StatusCode, responseEnvelope.ValidationUpstreamCode(err), err)
	}
	if err := responseEnvelope.VerifySignature(c.config.BaofuPublicKeyPEM); err != nil {
		return providerRequestError(method, resp.StatusCode, responseEnvelope.ValidationUpstreamCode(err), err)
	}
	responseBusinessContent := responseEnvelope.BusinessContent()
	if out != nil {
		if err := json.Unmarshal(responseBusinessContent, out); err != nil {
			return providerRequestError(method, resp.StatusCode, PublicEnvelopeUpstreamCodeInvalidDataContent, err)
		}
	}
	return nil
}

func validatePublicResponseIdentity(responseEnvelope PublicResponseEnvelope, merchantID string, terminalID string) error {
	if strings.TrimSpace(responseEnvelope.MerchantID) != strings.TrimSpace(merchantID) {
		return errors.New("baofu public response merId does not match request")
	}
	if strings.TrimSpace(responseEnvelope.TerminalID) != strings.TrimSpace(terminalID) {
		return errors.New("baofu public response terId does not match request")
	}
	return nil
}

func setAccountRawResponse(out any, operation string, raw json.RawMessage) {
	if target, ok := out.(interface{ SetOperation(string) }); ok {
		target.SetOperation(operation)
	}
	if target, ok := out.(interface{ SetRaw(json.RawMessage) }); ok {
		target.SetRaw(raw)
	}
}

func NewProviderContractError(operation string, cause error) error {
	if cause == nil {
		return nil
	}
	return providerRequestError(operation, http.StatusOK, PublicEnvelopeUpstreamCodeInvalidDataContent, cause)
}

func NewProviderBusinessError(operation string, upstreamCode string, upstreamMessage string) error {
	return providerResponseError(operation, http.StatusOK, upstreamCode, upstreamMessage, errProviderPublicBusinessResponse)
}

func IsProviderBusinessResponseError(err error) bool {
	return errors.Is(err, errProviderAccountBusinessResponse) || errors.Is(err, errProviderPublicBusinessResponse)
}

func providerRequestError(operation string, statusCode int, upstreamCode string, cause error) error {
	upstreamCode = providerRequestUpstreamCode(statusCode, upstreamCode)
	classified := ClassifyBaofuError(upstreamCode, "")
	return &ProviderError{
		Operation:    strings.TrimSpace(operation),
		Capability:   "baofu",
		StatusCode:   statusCode,
		UpstreamCode: strings.TrimSpace(upstreamCode),
		Frontend:     classified.FrontendGuidance(),
		cause:        cause,
	}
}

func providerRequestUpstreamCode(statusCode int, upstreamCode string) string {
	code := strings.TrimSpace(upstreamCode)
	if code != "" {
		return code
	}
	if statusCode == 0 {
		return "REQUEST_FAILED"
	}
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return "HTTP_STATUS"
	}
	return PublicEnvelopeUpstreamCodeInvalidDataContent
}

func providerResponseError(operation string, statusCode int, upstreamCode string, upstreamMessage string, cause error) error {
	return providerResponseErrorWithDiagnostic(operation, statusCode, upstreamCode, upstreamMessage, nil, cause)
}

func providerResponseErrorWithDiagnostic(operation string, statusCode int, upstreamCode string, upstreamMessage string, diagnostic []byte, cause error) error {
	classified := ClassifyBaofuError(upstreamCode, upstreamMessage)
	return &ProviderError{
		Operation:          strings.TrimSpace(operation),
		Capability:         "baofu",
		StatusCode:         statusCode,
		UpstreamCode:       strings.TrimSpace(upstreamCode),
		UpstreamMessage:    strings.TrimSpace(upstreamMessage),
		DiagnosticSnapshot: diagnostic,
		Frontend:           classified.FrontendGuidance(),
		cause:              cause,
	}
}

type accountBusinessFailureDiagnostic struct {
	Operation                 string        `json:"operation,omitempty"`
	HTTPStatus                int           `json:"http_status,omitempty"`
	Header                    UnionGWHeader `json:"-"`
	Code                      string        `json:"-"`
	Message                   string        `json:"-"`
	Failed                    bool          `json:"-"`
	SourcePath                string        `json:"source_path,omitempty"`
	RetCode                   string        `json:"ret_code,omitempty"`
	TopErrorCode              string        `json:"top_error_code,omitempty"`
	TopErrorMessagePresent    bool          `json:"top_error_message_present,omitempty"`
	ResultState               string        `json:"result_state,omitempty"`
	ResultErrorCode           string        `json:"result_error_code,omitempty"`
	ResultErrorMessagePresent bool          `json:"result_error_message_present,omitempty"`
}

func (d accountBusinessFailureDiagnostic) SafeDiagnosticSnapshot() []byte {
	snapshot := map[string]any{
		"provider":         "baofu",
		"capability":       "account",
		"business_failure": d.Failed,
	}
	if v := strings.TrimSpace(d.Operation); v != "" {
		snapshot["operation"] = v
	}
	if d.HTTPStatus != 0 {
		snapshot["http_status"] = d.HTTPStatus
	}
	if v := strings.TrimSpace(d.Header.SystemRespCode); v != "" {
		snapshot["sys_resp_code"] = v
	}
	if strings.TrimSpace(d.Header.SystemRespDesc) != "" {
		snapshot["sys_resp_desc_present"] = true
	}
	if v := strings.TrimSpace(d.SourcePath); v != "" {
		snapshot["source_path"] = v
	}
	if v := strings.TrimSpace(d.RetCode); v != "" {
		snapshot["ret_code"] = v
	}
	if v := strings.TrimSpace(d.TopErrorCode); v != "" {
		snapshot["top_error_code"] = v
	}
	if d.TopErrorMessagePresent {
		snapshot["top_error_message_present"] = true
	}
	if v := strings.TrimSpace(d.ResultState); v != "" {
		snapshot["result_state"] = v
	}
	if v := strings.TrimSpace(d.ResultErrorCode); v != "" {
		snapshot["result_error_code"] = v
	}
	if d.ResultErrorMessagePresent {
		snapshot["result_error_message_present"] = true
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return []byte(`{"provider":"baofu","capability":"account","business_failure":true}`)
	}
	return raw
}

func accountBusinessFailure(raw json.RawMessage) accountBusinessFailureDiagnostic {
	var payload map[string]json.RawMessage
	if len(raw) == 0 || json.Unmarshal(raw, &payload) != nil {
		return accountBusinessFailureDiagnostic{}
	}
	retCode := strings.ToUpper(jsonScalarString(payload["retCode"]))
	errorCode := jsonScalarString(payload["errorCode"])
	errorMessage := jsonScalarString(payload["errorMsg"])
	item := accountFirstResultItem(payload["result"])
	diagnostic := accountBusinessFailureDiagnostic{
		RetCode:                   retCode,
		TopErrorCode:              errorCode,
		TopErrorMessagePresent:    strings.TrimSpace(errorMessage) != "",
		ResultState:               item.State,
		ResultErrorCode:           item.ErrorCode,
		ResultErrorMessagePresent: strings.TrimSpace(item.ErrorMessage) != "",
	}
	if retCode == "" {
		code := errorCode
		if code == "" {
			code = "MISSING_RET_CODE"
		}
		diagnostic.Code = code
		diagnostic.Message = errorMessage
		diagnostic.Failed = true
		diagnostic.SourcePath = "body.errorCode"
		if errorCode == "" {
			diagnostic.SourcePath = "body.retCode"
		}
		return diagnostic
	}
	if retCode == "1" || retCode == "SUCCESS" {
		return accountBusinessFailureDiagnostic{}
	}
	code := errorCode
	if code == "" {
		code = retCode
		diagnostic.SourcePath = "body.retCode"
	} else {
		diagnostic.SourcePath = "body.errorCode"
	}
	diagnostic.Code = code
	diagnostic.Message = errorMessage
	diagnostic.Failed = true
	return diagnostic
}

type accountBusinessFailureResultItem struct {
	State        string
	ErrorCode    string
	ErrorMessage string
}

func accountFirstResultItem(raw json.RawMessage) accountBusinessFailureResultItem {
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 || string(raw) == "null" {
		return accountBusinessFailureResultItem{}
	}
	if strings.HasPrefix(string(raw), "[") {
		var items []struct {
			State        json.RawMessage `json:"state"`
			ErrorCode    json.RawMessage `json:"errorCode"`
			ErrorMessage json.RawMessage `json:"errorMsg"`
		}
		if err := json.Unmarshal(raw, &items); err != nil || len(items) == 0 {
			return accountBusinessFailureResultItem{}
		}
		return accountBusinessFailureResultItem{
			State:        strings.ToUpper(jsonScalarString(items[0].State)),
			ErrorCode:    jsonScalarString(items[0].ErrorCode),
			ErrorMessage: jsonScalarString(items[0].ErrorMessage),
		}
	}
	var item struct {
		State        json.RawMessage `json:"state"`
		ErrorCode    json.RawMessage `json:"errorCode"`
		ErrorMessage json.RawMessage `json:"errorMsg"`
	}
	if err := json.Unmarshal(raw, &item); err != nil {
		return accountBusinessFailureResultItem{}
	}
	return accountBusinessFailureResultItem{
		State:        strings.ToUpper(jsonScalarString(item.State)),
		ErrorCode:    jsonScalarString(item.ErrorCode),
		ErrorMessage: jsonScalarString(item.ErrorMessage),
	}
}

func publicBusinessFailure(raw json.RawMessage) (string, string, bool) {
	var payload struct {
		ResultCode   string `json:"resultCode"`
		ErrorCode    string `json:"errCode"`
		ErrorMessage string `json:"errMsg"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &payload) != nil {
		return "", "", false
	}
	resultCode := strings.ToUpper(strings.TrimSpace(payload.ResultCode))
	if resultCode == "" && (strings.TrimSpace(payload.ErrorCode) != "" || strings.TrimSpace(payload.ErrorMessage) != "") {
		return strings.TrimSpace(payload.ErrorCode), strings.TrimSpace(payload.ErrorMessage), true
	}
	if resultCode == "" {
		return "MISSING_RESULT_CODE", strings.TrimSpace(payload.ErrorMessage), true
	}
	if resultCode == "SUCCESS" {
		errorCode := strings.ToUpper(strings.TrimSpace(payload.ErrorCode))
		if errorCode != "" && errorCode != "SUCCESS" {
			return strings.TrimSpace(payload.ErrorCode), strings.TrimSpace(payload.ErrorMessage), true
		}
		return "", "", false
	}
	code := strings.TrimSpace(payload.ErrorCode)
	if code == "" {
		code = resultCode
	}
	return code, strings.TrimSpace(payload.ErrorMessage), true
}

func jsonScalarString(raw json.RawMessage) string {
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err == nil {
		return strings.TrimSpace(value)
	}
	return strings.Trim(strings.TrimSpace(string(raw)), `"`)
}

var _ HTTPDoer = (*http.Client)(nil)

func PublicEnvelopeTimestamp(now time.Time) string {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		location = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	return now.In(location).Format("20060102150405")
}
