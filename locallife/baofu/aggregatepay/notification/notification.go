package notification

import (
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
)

var ErrPaymentNotificationOutTradeNoRequired = errors.New("baofu payment notification out trade no is required")
var ErrShareNotificationOutTradeNoRequired = errors.New("baofu share notification out trade no is required")
var ErrRefundNotificationOutTradeNoRequired = errors.New("baofu refund notification out trade no is required")

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

type Parser struct{}

func NewParser() *Parser {
	return &Parser{}
}

func (p *Parser) ParsePaymentNotification(body []byte) (*PaymentNotification, error) {
	var payload struct {
		NotifyID         string `json:"notifyId"`
		NotifyType       string `json:"notifyType"`
		OutTradeNo       string `json:"outTradeNo"`
		TradeNo          string `json:"tradeNo"`
		TransactionState string `json:"txnState"`
		State            string `json:"state"`
		SuccessAmount    int64  `json:"succAmt"`
		TransactionAmt   int64  `json:"txnAmt"`
		FeeAmount        int64  `json:"feeAmt"`
		ResultCode       string `json:"resultCode"`
		OccurredAt       string `json:"occurredAt"`
		NotifyTime       string `json:"notifyTime"`
	}
	normalizedBody, err := normalizeAggregateNotificationBody(body)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(normalizedBody, &payload); err != nil {
		return nil, err
	}
	outTradeNo := strings.TrimSpace(payload.OutTradeNo)
	if outTradeNo == "" {
		return nil, ErrPaymentNotificationOutTradeNoRequired
	}
	upstreamState := strings.TrimSpace(payload.TransactionState)
	if upstreamState == "" {
		upstreamState = strings.TrimSpace(payload.State)
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
			OutTradeNo:       outTradeNo,
			TradeNo:          strings.TrimSpace(payload.TradeNo),
			TransactionState: upstreamState,
			SuccessAmountFen: amount,
			FeeAmountFen:     payload.FeeAmount,
			ResultCode:       strings.TrimSpace(payload.ResultCode),
			Raw:              normalizedBody,
		},
	}, nil
}

func (p *Parser) ParseShareNotification(body []byte) (*ShareNotification, error) {
	var payload struct {
		NotifyID       string `json:"notifyId"`
		NotifyType     string `json:"notifyType"`
		OutTradeNo     string `json:"outTradeNo"`
		TradeNo        string `json:"tradeNo"`
		TxnState       string `json:"txnState"`
		State          string `json:"state"`
		SuccessAmount  int64  `json:"succAmt"`
		SharingAmount  int64  `json:"sharingAmt"`
		ResultCode     string `json:"resultCode"`
		OccurredAt     string `json:"occurredAt"`
		NotifyTime     string `json:"notifyTime"`
		FinishTime     string `json:"finishTime"`
		ClearingDate   string `json:"clearingDate"`
		RequestReserve string `json:"reqReserved"`
	}
	normalizedBody, err := normalizeAggregateNotificationBody(body)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(normalizedBody, &payload); err != nil {
		return nil, err
	}
	outTradeNo := strings.TrimSpace(payload.OutTradeNo)
	if outTradeNo == "" {
		return nil, ErrShareNotificationOutTradeNoRequired
	}
	upstreamState := strings.TrimSpace(payload.TxnState)
	if upstreamState == "" {
		upstreamState = strings.TrimSpace(payload.State)
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
			OutTradeNo:       outTradeNo,
			TradeNo:          strings.TrimSpace(payload.TradeNo),
			TransactionState: upstreamState,
			SuccessAmountFen: amount,
			ResultCode:       strings.TrimSpace(payload.ResultCode),
			Raw:              normalizedBody,
		},
	}, nil
}

func (p *Parser) ParseRefundNotification(body []byte) (*RefundNotification, error) {
	var payload struct {
		NotifyID      string `json:"notifyId"`
		NotifyType    string `json:"notifyType"`
		OutTradeNo    string `json:"outTradeNo"`
		TradeNo       string `json:"tradeNo"`
		RefundState   string `json:"refundState"`
		State         string `json:"state"`
		SuccessAmount int64  `json:"succAmt"`
		RefundAmount  int64  `json:"refundAmt"`
		ResultCode    string `json:"resultCode"`
		OccurredAt    string `json:"occurredAt"`
		NotifyTime    string `json:"notifyTime"`
		FinishTime    string `json:"finishTime"`
	}
	normalizedBody, err := normalizeAggregateNotificationBody(body)
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
	upstreamState := strings.TrimSpace(payload.RefundState)
	if upstreamState == "" {
		upstreamState = strings.TrimSpace(payload.State)
	}
	amount := payload.SuccessAmount
	if amount == 0 {
		amount = payload.RefundAmount
	}
	occurredAt := parseBaofuPaymentNotifyTime(payload.OccurredAt, payload.NotifyTime, payload.FinishTime)
	terminalStatus := aggregatecontracts.NormalizeRefundTerminalStatus(upstreamState)
	return &RefundNotification{
		NotifyID:       strings.TrimSpace(payload.NotifyID),
		NotifyType:     strings.TrimSpace(payload.NotifyType),
		TerminalStatus: terminalStatus,
		IsTerminal:     isTerminalRefundStatus(terminalStatus),
		OccurredAt:     occurredAt,
		Raw:            normalizedBody,
		Fact: aggregatecontracts.RefundFact{
			OutTradeNo:       outTradeNo,
			TradeNo:          strings.TrimSpace(payload.TradeNo),
			TransactionState: upstreamState,
			SuccessAmountFen: amount,
			ResultCode:       strings.TrimSpace(payload.ResultCode),
			Raw:              normalizedBody,
		},
	}, nil
}

func normalizeAggregateNotificationBody(body []byte) ([]byte, error) {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return nil, errors.New("baofu aggregate notification payload is required")
	}
	if json.Valid([]byte(trimmed)) {
		return []byte(trimmed), nil
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

func isAggregateNotificationIntegerField(key string) bool {
	switch strings.TrimSpace(key) {
	case "succAmt", "txnAmt", "feeAmt", "instFeeAmt", "sharingAmt", "refundAmt":
		return true
	default:
		return false
	}
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
