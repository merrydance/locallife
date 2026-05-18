package notification

import (
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/merrydance/locallife/baofu"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
)

var ErrPaymentNotificationMerchantIDRequired = errors.New("baofu payment notification merId is required")
var ErrPaymentNotificationTerminalIDRequired = errors.New("baofu payment notification terId is required")
var ErrPaymentNotificationPayCodeRequired = errors.New("baofu payment notification payCode is required")
var ErrShareNotificationTransactionStateRequired = errors.New("baofu share notification txnState is required")
var ErrShareNotificationResultCodeRequired = errors.New("baofu share notification resultCode is required")
var ErrRefundNotificationMerchantIDRequired = errors.New("baofu refund notification merId is required")
var ErrRefundNotificationTerminalIDRequired = errors.New("baofu refund notification terId is required")
var ErrRefundNotificationTradeNoRequired = errors.New("baofu refund notification tradeNo is required")
var ErrRefundNotificationOutTradeNoRequired = errors.New("baofu refund notification outTradeNo is required")
var ErrRefundNotificationResultCodeRequired = errors.New("baofu refund notification resultCode is required")
var ErrRefundNotificationTransactionTimeRequired = errors.New("baofu refund notification txnTime is required")

type PaymentNotification struct {
	NotifyID       string
	NotifyType     string
	TerminalStatus string
	IsTerminal     bool
	OccurredAt     time.Time
	Fact           aggregatecontracts.PaymentFact
	Raw            []byte
}

type ShareNotification struct {
	NotifyID       string
	NotifyType     string
	TerminalStatus string
	IsTerminal     bool
	OccurredAt     time.Time
	Fact           aggregatecontracts.ShareFact
	Raw            []byte
}

type RefundNotification struct {
	NotifyID       string
	NotifyType     string
	TerminalStatus string
	IsTerminal     bool
	OccurredAt     time.Time
	Fact           aggregatecontracts.RefundFact
	Raw            []byte
}

type Parser struct {
	publicKeyPEM          string
	requireSignedEnvelope bool
}

func NewParser() *Parser {
	return &Parser{}
}

func NewParserWithPublicKey(publicKeyPEM string) *Parser {
	return &Parser{
		publicKeyPEM:          strings.TrimSpace(publicKeyPEM),
		requireSignedEnvelope: true,
	}
}

func (p *Parser) ParsePaymentNotification(body []byte) (*PaymentNotification, error) {
	var payload struct {
		NotifyID             string          `json:"notifyId"`
		NotifyType           string          `json:"notifyType"`
		AgentMerchantID      string          `json:"agentMerId"`
		AgentTerminalID      string          `json:"agentTerId"`
		MerchantID           string          `json:"merId"`
		TerminalID           string          `json:"terId"`
		OutTradeNo           string          `json:"outTradeNo"`
		TradeNo              string          `json:"tradeNo"`
		TransactionState     string          `json:"txnState"`
		State                string          `json:"state"`
		FinishTime           string          `json:"finishTime"`
		SuccessAmount        int64           `json:"succAmt"`
		TransactionAmt       int64           `json:"txnAmt"`
		FeeAmount            int64           `json:"feeAmt"`
		InstallmentFeeAmount int64           `json:"instFeeAmt"`
		ResultCode           string          `json:"resultCode"`
		ErrorCode            string          `json:"errCode"`
		ErrorMessage         string          `json:"errMsg"`
		RequestChannelNo     string          `json:"reqChlNo"`
		PayCode              string          `json:"payCode"`
		ChannelReturnParam   json.RawMessage `json:"chlRetParam"`
		ClearingDate         string          `json:"clearingDate"`
		OccurredAt           string          `json:"occurredAt"`
		NotifyTime           string          `json:"notifyTime"`
	}
	normalizedBody, err := p.normalizeAggregateNotificationBody(body)
	if err != nil {
		return nil, err
	}
	normalizedBody, err = normalizeAggregateNotificationStringScalars(normalizedBody)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(normalizedBody, &payload); err != nil {
		return nil, err
	}
	outTradeNo := strings.TrimSpace(payload.OutTradeNo)
	if strings.TrimSpace(payload.MerchantID) == "" {
		return nil, ErrPaymentNotificationMerchantIDRequired
	}
	if strings.TrimSpace(payload.TerminalID) == "" {
		return nil, ErrPaymentNotificationTerminalIDRequired
	}
	if strings.TrimSpace(payload.PayCode) == "" {
		return nil, ErrPaymentNotificationPayCodeRequired
	}
	if err := validateOfficialNotifyType("baofu payment notification", payload.NotifyType, baofu.PublicNotificationTypePayment); err != nil {
		return nil, err
	}
	upstreamState := strings.TrimSpace(payload.TransactionState)
	if upstreamState == "" {
		upstreamState = strings.TrimSpace(payload.State)
	}
	if upstreamState != "" && !aggregatecontracts.IsSupportedPaymentState(upstreamState) {
		return nil, errors.New("baofu payment notification txnState is unsupported")
	}
	if err := validateOfficialBusinessResultCode("baofu payment notification", payload.ResultCode); err != nil {
		return nil, err
	}
	if err := validateOptionalOfficialDateTime("baofu payment notification", "finishTime", payload.FinishTime); err != nil {
		return nil, err
	}
	if err := validateOptionalOfficialDate("baofu payment notification", "clearingDate", payload.ClearingDate); err != nil {
		return nil, err
	}
	amount := payload.SuccessAmount
	if amount == 0 {
		amount = payload.TransactionAmt
	}
	occurredAt := parseBaofuPaymentNotifyTime(payload.OccurredAt, payload.NotifyTime)
	terminalStatus := aggregatecontracts.NormalizePaymentTerminalStatus(upstreamState)
	return &PaymentNotification{
		NotifyID:       strings.TrimSpace(payload.NotifyID),
		NotifyType:     strings.TrimSpace(payload.NotifyType),
		TerminalStatus: terminalStatus,
		IsTerminal:     isTerminalPaymentStatus(terminalStatus),
		OccurredAt:     occurredAt,
		Raw:            normalizedBody,
		Fact: aggregatecontracts.PaymentFact{
			AgentMerchantID:         strings.TrimSpace(payload.AgentMerchantID),
			AgentTerminalID:         strings.TrimSpace(payload.AgentTerminalID),
			MerchantID:              strings.TrimSpace(payload.MerchantID),
			TerminalID:              strings.TrimSpace(payload.TerminalID),
			OutTradeNo:              outTradeNo,
			TradeNo:                 strings.TrimSpace(payload.TradeNo),
			TransactionState:        upstreamState,
			FinishTime:              strings.TrimSpace(payload.FinishTime),
			SuccessAmountFen:        amount,
			FeeAmountFen:            payload.FeeAmount,
			InstallmentFeeAmountFen: payload.InstallmentFeeAmount,
			ResultCode:              strings.TrimSpace(payload.ResultCode),
			ErrorCode:               strings.TrimSpace(payload.ErrorCode),
			ErrorMessage:            strings.TrimSpace(payload.ErrorMessage),
			RequestChannelNo:        strings.TrimSpace(payload.RequestChannelNo),
			PayCode:                 strings.TrimSpace(payload.PayCode),
			ChannelReturnParam:      cloneRawMessage(payload.ChannelReturnParam),
			ClearingDate:            strings.TrimSpace(payload.ClearingDate),
			Raw:                     normalizedBody,
		},
	}, nil
}

func (p *Parser) ParseShareNotification(body []byte) (*ShareNotification, error) {
	var payload struct {
		NotifyID        string `json:"notifyId"`
		NotifyType      string `json:"notifyType"`
		AgentMerchantID string `json:"agentMerId"`
		AgentTerminalID string `json:"agentTerId"`
		MerchantID      string `json:"merId"`
		TerminalID      string `json:"terId"`
		OutTradeNo      string `json:"outTradeNo"`
		TradeNo         string `json:"tradeNo"`
		TxnState        string `json:"txnState"`
		State           string `json:"state"`
		SuccessAmount   int64  `json:"succAmt"`
		SharingAmount   int64  `json:"sharingAmt"`
		ResultCode      string `json:"resultCode"`
		OccurredAt      string `json:"occurredAt"`
		NotifyTime      string `json:"notifyTime"`
		FinishTime      string `json:"finishTime"`
		ClearingDate    string `json:"clearingDate"`
		RequestReserve  string `json:"reqReserved"`
	}
	normalizedBody, err := p.normalizeAggregateNotificationBody(body)
	if err != nil {
		return nil, err
	}
	normalizedBody, err = normalizeAggregateNotificationStringScalars(normalizedBody)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(normalizedBody, &payload); err != nil {
		return nil, err
	}
	outTradeNo := strings.TrimSpace(payload.OutTradeNo)
	if strings.TrimSpace(payload.TxnState) == "" && strings.TrimSpace(payload.State) == "" {
		return nil, ErrShareNotificationTransactionStateRequired
	}
	if strings.TrimSpace(payload.ResultCode) == "" {
		return nil, ErrShareNotificationResultCodeRequired
	}
	if err := validateOfficialNotifyType("baofu share notification", payload.NotifyType, baofu.PublicNotificationTypeSharing); err != nil {
		return nil, err
	}
	upstreamState := strings.TrimSpace(payload.TxnState)
	if upstreamState == "" {
		upstreamState = strings.TrimSpace(payload.State)
	}
	if !aggregatecontracts.IsSupportedShareState(upstreamState) {
		return nil, errors.New("baofu share notification txnState is unsupported")
	}
	if err := validateOfficialBusinessResultCode("baofu share notification", payload.ResultCode); err != nil {
		return nil, err
	}
	if err := validateOptionalOfficialDateTime("baofu share notification", "finishTime", payload.FinishTime); err != nil {
		return nil, err
	}
	if err := validateOptionalOfficialDate("baofu share notification", "clearingDate", payload.ClearingDate); err != nil {
		return nil, err
	}
	amount := payload.SuccessAmount
	if amount == 0 {
		amount = payload.SharingAmount
	}
	occurredAt := parseBaofuPaymentNotifyTime(payload.OccurredAt, payload.NotifyTime, payload.FinishTime)
	terminalStatus := aggregatecontracts.NormalizeShareTerminalStatus(upstreamState)
	return &ShareNotification{
		NotifyID:       strings.TrimSpace(payload.NotifyID),
		NotifyType:     strings.TrimSpace(payload.NotifyType),
		TerminalStatus: terminalStatus,
		IsTerminal:     isTerminalShareStatus(terminalStatus),
		OccurredAt:     occurredAt,
		Raw:            normalizedBody,
		Fact: aggregatecontracts.ShareFact{
			AgentMerchantID:  strings.TrimSpace(payload.AgentMerchantID),
			AgentTerminalID:  strings.TrimSpace(payload.AgentTerminalID),
			MerchantID:       strings.TrimSpace(payload.MerchantID),
			TerminalID:       strings.TrimSpace(payload.TerminalID),
			OutTradeNo:       outTradeNo,
			TradeNo:          strings.TrimSpace(payload.TradeNo),
			TransactionState: upstreamState,
			FinishTime:       strings.TrimSpace(payload.FinishTime),
			SuccessAmountFen: amount,
			ClearingDate:     strings.TrimSpace(payload.ClearingDate),
			ResultCode:       strings.TrimSpace(payload.ResultCode),
			Raw:              normalizedBody,
		},
	}, nil
}

func (p *Parser) ParseRefundNotification(body []byte) (*RefundNotification, error) {
	var payload struct {
		NotifyID        string `json:"notifyId"`
		NotifyType      string `json:"notifyType"`
		AgentMerchantID string `json:"agentMerId"`
		AgentTerminalID string `json:"agentTerId"`
		MerchantID      string `json:"merId"`
		TerminalID      string `json:"terId"`
		OutTradeNo      string `json:"outTradeNo"`
		TradeNo         string `json:"tradeNo"`
		RefundState     string `json:"refundState"`
		State           string `json:"state"`
		SuccessAmount   int64  `json:"succAmt"`
		RefundAmount    int64  `json:"refundAmt"`
		ResultCode      string `json:"resultCode"`
		TransactionTime string `json:"txnTime"`
		ErrorCode       string `json:"errCode"`
		ErrorMessage    string `json:"errMsg"`
		OccurredAt      string `json:"occurredAt"`
		NotifyTime      string `json:"notifyTime"`
		FinishTime      string `json:"finishTime"`
	}
	normalizedBody, err := p.normalizeAggregateNotificationBody(body)
	if err != nil {
		return nil, err
	}
	normalizedBody, err = normalizeAggregateNotificationStringScalars(normalizedBody)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(normalizedBody, &payload); err != nil {
		return nil, err
	}
	outTradeNo := strings.TrimSpace(payload.OutTradeNo)
	if outTradeNo == "" {
		return nil, ErrRefundNotificationOutTradeNoRequired
	}
	if strings.TrimSpace(payload.MerchantID) == "" {
		return nil, ErrRefundNotificationMerchantIDRequired
	}
	if strings.TrimSpace(payload.TerminalID) == "" {
		return nil, ErrRefundNotificationTerminalIDRequired
	}
	if strings.TrimSpace(payload.TradeNo) == "" {
		return nil, ErrRefundNotificationTradeNoRequired
	}
	if strings.TrimSpace(payload.ResultCode) == "" {
		return nil, ErrRefundNotificationResultCodeRequired
	}
	if strings.TrimSpace(payload.TransactionTime) == "" {
		return nil, ErrRefundNotificationTransactionTimeRequired
	}
	if err := validateOptionalOfficialDateTime("baofu refund notification", "txnTime", payload.TransactionTime); err != nil {
		return nil, err
	}
	if err := validateOfficialNotifyType("baofu refund notification", payload.NotifyType, baofu.PublicNotificationTypeRefund); err != nil {
		return nil, err
	}
	if err := validateOfficialBusinessResultCode("baofu refund notification", payload.ResultCode); err != nil {
		return nil, err
	}
	if err := validateOptionalOfficialDateTime("baofu refund notification", "finishTime", payload.FinishTime); err != nil {
		return nil, err
	}
	upstreamState := strings.TrimSpace(payload.RefundState)
	if upstreamState == "" {
		upstreamState = strings.TrimSpace(payload.State)
	}
	if upstreamState != "" && !aggregatecontracts.IsSupportedRefundState(upstreamState) {
		return nil, errors.New("baofu refund notification refundState is unsupported")
	}
	amount := payload.SuccessAmount
	if amount == 0 {
		amount = payload.RefundAmount
	}
	occurredAt := parseBaofuPaymentNotifyTime(payload.OccurredAt, payload.NotifyTime, payload.FinishTime)
	terminalStatus := normalizeRefundTerminalStatus(upstreamState, payload.ResultCode)
	return &RefundNotification{
		NotifyID:       strings.TrimSpace(payload.NotifyID),
		NotifyType:     strings.TrimSpace(payload.NotifyType),
		TerminalStatus: terminalStatus,
		IsTerminal:     isTerminalRefundStatus(terminalStatus),
		OccurredAt:     occurredAt,
		Raw:            normalizedBody,
		Fact: aggregatecontracts.RefundFact{
			AgentMerchantID:  strings.TrimSpace(payload.AgentMerchantID),
			AgentTerminalID:  strings.TrimSpace(payload.AgentTerminalID),
			MerchantID:       strings.TrimSpace(payload.MerchantID),
			TerminalID:       strings.TrimSpace(payload.TerminalID),
			OutTradeNo:       outTradeNo,
			TradeNo:          strings.TrimSpace(payload.TradeNo),
			TransactionState: upstreamState,
			FinishTime:       strings.TrimSpace(payload.FinishTime),
			SuccessAmountFen: amount,
			ResultCode:       strings.TrimSpace(payload.ResultCode),
			TransactionTime:  strings.TrimSpace(payload.TransactionTime),
			ErrorCode:        strings.TrimSpace(payload.ErrorCode),
			ErrorMessage:     strings.TrimSpace(payload.ErrorMessage),
			Raw:              normalizedBody,
		},
	}, nil
}

func validateOfficialNotifyType(prefix, actual, expected string) error {
	actual = strings.TrimSpace(actual)
	if actual == "" {
		return nil
	}
	if actual != expected {
		return errors.New(prefix + " notifyType must be " + expected)
	}
	return nil
}

func validateOfficialBusinessResultCode(prefix, resultCode string) error {
	switch strings.TrimSpace(resultCode) {
	case "", aggregatecontracts.BusinessResultCodeSuccess, aggregatecontracts.BusinessResultCodeFail:
		return nil
	default:
		return errors.New(prefix + " resultCode is unsupported")
	}
}

func validateOptionalOfficialDateTime(prefix, field, value string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	if len(strings.TrimSpace(value)) != len("20060102150405") {
		return errors.New(prefix + " " + field + " must use yyyyMMddHHmmss")
	}
	if _, err := time.Parse("20060102150405", strings.TrimSpace(value)); err != nil {
		return errors.New(prefix + " " + field + " must use yyyyMMddHHmmss")
	}
	return nil
}

func validateOptionalOfficialDate(prefix, field, value string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	if len(strings.TrimSpace(value)) != len("20060102") {
		return errors.New(prefix + " " + field + " must use yyyyMMdd")
	}
	if _, err := time.Parse("20060102", strings.TrimSpace(value)); err != nil {
		return errors.New(prefix + " " + field + " must use yyyyMMdd")
	}
	return nil
}

func normalizeRefundTerminalStatus(upstreamState, resultCode string) string {
	if strings.TrimSpace(upstreamState) != "" {
		return aggregatecontracts.NormalizeRefundTerminalStatus(upstreamState)
	}
	switch strings.TrimSpace(resultCode) {
	case aggregatecontracts.BusinessResultCodeSuccess:
		return db.ExternalPaymentTerminalStatusSuccess
	case aggregatecontracts.BusinessResultCodeFail:
		return db.ExternalPaymentTerminalStatusFailed
	default:
		return db.ExternalPaymentTerminalStatusUnknown
	}
}

func (p *Parser) normalizeAggregateNotificationBody(body []byte) ([]byte, error) {
	if p != nil && p.requireSignedEnvelope {
		return normalizeSignedAggregateNotificationBody(body, p.publicKeyPEM)
	}
	return normalizeAggregateNotificationBody(body)
}

func normalizeSignedAggregateNotificationBody(body []byte, publicKeyPEM string) ([]byte, error) {
	envelope, err := parsePublicNotificationEnvelope(body)
	if err != nil {
		return nil, err
	}
	if err := envelope.Validate(); err != nil {
		return nil, err
	}
	if err := envelope.VerifySignature(publicKeyPEM); err != nil {
		return nil, err
	}
	return normalizeAggregateNotificationDataContent(envelope.DataContent, map[string]string{
		"notifyType": envelope.NotifyType,
		"merId":      envelope.MerchantID,
		"terId":      envelope.TerminalID,
	})
}

func parsePublicNotificationEnvelope(body []byte) (baofu.PublicNotificationEnvelope, error) {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return baofu.PublicNotificationEnvelope{}, errors.New("baofu aggregate notification payload is required")
	}
	if json.Valid([]byte(trimmed)) {
		var envelope baofu.PublicNotificationEnvelope
		if err := json.Unmarshal([]byte(trimmed), &envelope); err != nil {
			return baofu.PublicNotificationEnvelope{}, err
		}
		if len(envelope.DataContent) == 0 && strings.TrimSpace(envelope.SignString) == "" {
			return baofu.PublicNotificationEnvelope{}, errors.New("baofu aggregate notification signed public envelope is required")
		}
		return envelope, nil
	}
	values, err := url.ParseQuery(trimmed)
	if err != nil {
		return baofu.PublicNotificationEnvelope{}, err
	}
	if strings.TrimSpace(values.Get("dataContent")) == "" && strings.TrimSpace(values.Get("signStr")) == "" {
		return baofu.PublicNotificationEnvelope{}, errors.New("baofu aggregate notification signed public envelope is required")
	}
	return baofu.PublicNotificationEnvelope{
		MerchantID:         values.Get("merId"),
		TerminalID:         values.Get("terId"),
		Charset:            values.Get("charset"),
		Version:            values.Get("version"),
		Format:             values.Get("format"),
		NotifyType:         values.Get("notifyType"),
		SignType:           values.Get("signType"),
		SignSerialNo:       values.Get("signSn"),
		EncryptionSerialNo: values.Get("ncrptnSn"),
		DigitalEnvelope:    values.Get("dgtlEnvlp"),
		SignString:         values.Get("signStr"),
		DataContent:        baofu.JSONString(values.Get("dataContent")),
	}, nil
}

func normalizeAggregateNotificationBody(body []byte) ([]byte, error) {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return nil, errors.New("baofu aggregate notification payload is required")
	}
	if json.Valid([]byte(trimmed)) {
		return normalizeAggregateNotificationJSONObject([]byte(trimmed))
	}
	if !strings.Contains(trimmed, "=") {
		var payload any
		if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
			return nil, err
		}
	}
	values, err := url.ParseQuery(trimmed)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, errors.New("baofu aggregate notification form payload is empty")
	}
	if dataContent := strings.TrimSpace(values.Get("dataContent")); dataContent != "" {
		return normalizeAggregateNotificationDataContent([]byte(dataContent), map[string]string{
			"notifyType": values.Get("notifyType"),
		})
	}
	payload := make(map[string]any, len(values))
	for key, vals := range values {
		key = strings.TrimSpace(key)
		if key == "" || len(vals) == 0 {
			continue
		}
		value := strings.TrimSpace(vals[0])
		if isAggregateNotificationIntegerField(key) {
			if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
				payload[key] = parsed
				continue
			}
		}
		payload[key] = value
	}
	if len(payload) == 0 {
		return nil, errors.New("baofu aggregate notification form payload is empty")
	}
	return json.Marshal(payload)
}

func normalizeAggregateNotificationJSONObject(raw []byte) ([]byte, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	dataContent, ok := payload["dataContent"]
	if !ok {
		return raw, nil
	}
	metadata := map[string]string{}
	if notifyTypeRaw, ok := payload["notifyType"]; ok {
		metadata["notifyType"] = jsonRawString(notifyTypeRaw)
	}
	return normalizeAggregateNotificationDataContent(dataContent, metadata)
}

func normalizeAggregateNotificationDataContent(raw []byte, metadata map[string]string) ([]byte, error) {
	content := json.RawMessage(strings.TrimSpace(string(raw)))
	var text string
	if err := json.Unmarshal(content, &text); err == nil {
		content = json.RawMessage(strings.TrimSpace(text))
	}
	if !json.Valid(content) {
		return nil, errors.New("baofu aggregate notification dataContent must be valid JSON")
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(content, &payload); err != nil {
		return nil, err
	}
	for key, value := range metadata {
		if strings.TrimSpace(value) == "" {
			continue
		}
		if existing, exists := payload[key]; exists {
			if isAggregateNotificationEnvelopeIdentityField(key) && jsonRawString(existing) != strings.TrimSpace(value) {
				return nil, errors.New("baofu aggregate notification dataContent " + key + " does not match envelope")
			}
			continue
		}
		encoded, err := json.Marshal(strings.TrimSpace(value))
		if err != nil {
			return nil, err
		}
		payload[key] = encoded
	}
	return json.Marshal(payload)
}

func isAggregateNotificationEnvelopeIdentityField(key string) bool {
	switch strings.TrimSpace(key) {
	case "merId", "terId":
		return true
	default:
		return false
	}
}

func normalizeAggregateNotificationStringScalars(raw []byte) ([]byte, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	changed := false
	for key, value := range payload {
		if !isAggregateNotificationStringField(key) {
			continue
		}
		text, ok := jsonStringOrNumber(value)
		if !ok {
			continue
		}
		encoded, err := json.Marshal(text)
		if err != nil {
			return nil, err
		}
		payload[key] = encoded
		changed = true
	}
	if !changed {
		return raw, nil
	}
	return json.Marshal(payload)
}

func jsonStringOrNumber(raw json.RawMessage) (string, bool) {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text), true
	}
	value := strings.TrimSpace(string(raw))
	if value == "" || value == "null" {
		return "", false
	}
	var number json.Number
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.UseNumber()
	if err := decoder.Decode(&number); err == nil {
		return strings.TrimSpace(number.String()), true
	}
	return "", false
}

func jsonRawString(raw json.RawMessage) string {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}
	return strings.Trim(strings.TrimSpace(string(raw)), `"`)
}

func isAggregateNotificationStringField(key string) bool {
	switch strings.TrimSpace(key) {
	case "notifyId",
		"notifyType",
		"agentMerId",
		"agentTerId",
		"merId",
		"terId",
		"outTradeNo",
		"tradeNo",
		"txnState",
		"state",
		"finishTime",
		"resultCode",
		"errCode",
		"errMsg",
		"reqChlNo",
		"payCode",
		"clearingDate",
		"occurredAt",
		"notifyTime",
		"reqReserved",
		"refundState",
		"txnTime":
		return true
	default:
		return false
	}
}

func isAggregateNotificationIntegerField(key string) bool {
	switch strings.TrimSpace(key) {
	case "succAmt", "txnAmt", "feeAmt", "instFeeAmt", "sharingAmt", "refundAmt":
		return true
	default:
		return false
	}
}

func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	clone := make([]byte, len(raw))
	copy(clone, raw)
	return clone
}

func parseBaofuPaymentNotifyTime(candidates ...string) time.Time {
	for _, candidate := range candidates {
		value := strings.TrimSpace(candidate)
		if value == "" {
			continue
		}
		for _, layout := range []string{time.RFC3339, "20060102150405", "2006-01-02 15:04:05"} {
			if parsed, err := time.Parse(layout, value); err == nil {
				return parsed.UTC()
			}
		}
	}
	return time.Now().UTC()
}

func isTerminalPaymentStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case db.ExternalPaymentTerminalStatusSuccess,
		db.ExternalPaymentTerminalStatusFailed,
		db.ExternalPaymentTerminalStatusClosed,
		db.ExternalPaymentTerminalStatusExpired:
		return true
	default:
		return false
	}
}

func isTerminalShareStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case db.ExternalPaymentTerminalStatusSuccess,
		db.ExternalPaymentTerminalStatusFailed,
		db.ExternalPaymentTerminalStatusClosed:
		return true
	default:
		return false
	}
}

func isTerminalRefundStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case db.ExternalPaymentTerminalStatusSuccess,
		db.ExternalPaymentTerminalStatusFailed,
		db.ExternalPaymentTerminalStatusClosed:
		return true
	default:
		return false
	}
}
