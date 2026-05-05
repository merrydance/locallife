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
		return providerResponseError(method, resp.StatusCode, responseEnvelope.Header.SystemRespCode, responseEnvelope.Header.SystemRespDesc, errors.New("baofu union-gw system response failed"))
	}
	if code, message, failed := accountBusinessFailure(responseEnvelope.Body); failed {
		return providerResponseError(method, resp.StatusCode, code, message, errors.New("baofu account business response failed"))
	}
	if out != nil {
		if err := json.Unmarshal(responseEnvelope.Body, out); err != nil {
			return providerRequestError(method, resp.StatusCode, responseEnvelope.Header.SystemRespCode, err)
		}
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
	if strings.TrimSpace(responseEnvelope.ReturnCode) == PublicEnvelopeReturnCodeFail {
		return providerResponseError(method, resp.StatusCode, responseEnvelope.ReturnCode, responseEnvelope.ReturnMessage, errors.New("baofu upstream returned failure"))
	}
	if err := responseEnvelope.Validate(); err != nil {
		return providerRequestError(method, resp.StatusCode, responseEnvelope.ValidationUpstreamCode(err), err)
	}
	if err := responseEnvelope.VerifySignature(c.config.BaofuPublicKeyPEM); err != nil {
		return providerRequestError(method, resp.StatusCode, responseEnvelope.ValidationUpstreamCode(err), err)
	}
	responseBusinessContent := responseEnvelope.BusinessContent()
	if code, message, failed := publicBusinessFailure(json.RawMessage(responseBusinessContent)); failed {
		return providerResponseError(method, resp.StatusCode, code, message, errors.New("baofu public business response failed"))
	}
	if out != nil {
		if err := json.Unmarshal(responseBusinessContent, out); err != nil {
			return providerRequestError(method, resp.StatusCode, PublicEnvelopeUpstreamCodeInvalidDataContent, err)
		}
	}
	return nil
}

func providerRequestError(operation string, statusCode int, upstreamCode string, cause error) error {
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

func providerResponseError(operation string, statusCode int, upstreamCode string, upstreamMessage string, cause error) error {
	classified := ClassifyBaofuError(upstreamCode, upstreamMessage)
	return &ProviderError{
		Operation:       strings.TrimSpace(operation),
		Capability:      "baofu",
		StatusCode:      statusCode,
		UpstreamCode:    strings.TrimSpace(upstreamCode),
		UpstreamMessage: strings.TrimSpace(upstreamMessage),
		Frontend:        classified.FrontendGuidance(),
		cause:           cause,
	}
}

func accountBusinessFailure(raw json.RawMessage) (string, string, bool) {
	var payload map[string]json.RawMessage
	if len(raw) == 0 || json.Unmarshal(raw, &payload) != nil {
		return "", "", false
	}
	retCode := strings.ToUpper(jsonScalarString(payload["retCode"]))
	errorCode := jsonScalarString(payload["errorCode"])
	errorMessage := jsonScalarString(payload["errorMsg"])
	if retCode == "" && (errorCode != "" || errorMessage != "") {
		return errorCode, errorMessage, true
	}
	if retCode == "" {
		return "MISSING_RET_CODE", errorMessage, true
	}
	if retCode == "1" || retCode == "SUCCESS" {
		return "", "", false
	}
	code := errorCode
	if code == "" {
		code = retCode
	}
	return code, errorMessage, true
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
