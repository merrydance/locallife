package notification

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/merrydance/locallife/baofu"
	"github.com/merrydance/locallife/baofu/account/contracts"
)

type AccountNotification struct {
	OutRequestNo  string
	ContractNo    string
	SharingMerID  string
	UpstreamState string
	OpenState     string
	FailCode      string
	FailMessage   string
	OccurredAt    time.Time
	Raw           []byte
}

type WithdrawNotification struct {
	TransSerialNo   string
	BaofuWithdrawNo string
	ContractNo      string
	UpstreamState   string
	Status          string
	AmountFen       int64
	FeeFen          int64
	TotalAmountFen  int64
	Remark          string
	RequestReserved string
	OccurredAt      time.Time
	Raw             []byte
}

type Parser struct {
	baofuPublicKeyPEM string
}

func NewParser(baofuPublicKeyPEM string) *Parser {
	return &Parser{baofuPublicKeyPEM: strings.TrimSpace(baofuPublicKeyPEM)}
}

func (p *Parser) ParseOpenAccountNotification(body []byte) (*AccountNotification, error) {
	if p == nil || p.baofuPublicKeyPEM == "" {
		return nil, errors.New("baofu account notification parser is not configured")
	}
	plaintext, err := p.decodeOfficialDataContent(body)
	if err != nil {
		return nil, err
	}
	return ParseOpenAccountPlaintext(plaintext)
}

func (p *Parser) ParseWithdrawNotification(body []byte) (*WithdrawNotification, error) {
	if p == nil || p.baofuPublicKeyPEM == "" {
		return nil, errors.New("baofu account notification parser is not configured")
	}
	plaintext, err := p.decodeOfficialDataContent(body)
	if err != nil {
		return nil, err
	}
	return ParseWithdrawPlaintext(plaintext)
}

func (p *Parser) decodeOfficialDataContent(body []byte) ([]byte, error) {
	values, err := parseOfficialNotificationValues(body)
	if err != nil {
		return nil, err
	}
	if dataType := strings.TrimSpace(values.Get("data_type")); dataType != "" && !strings.EqualFold(dataType, "JSON") {
		return nil, errors.New("baofu account notification data_type must be JSON")
	}
	dataContent := strings.TrimSpace(values.Get("data_content"))
	if dataContent == "" {
		return nil, errors.New("baofu account notification data_content is required")
	}
	return baofu.DecodeUnionGWVerifyType1Content(p.baofuPublicKeyPEM, dataContent)
}

func parseOfficialNotificationValues(body []byte) (url.Values, error) {
	raw := strings.TrimSpace(string(body))
	if raw == "" {
		return nil, errors.New("baofu account notification payload is required")
	}
	if strings.HasPrefix(raw, "{") {
		var payload map[string]string
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			return nil, err
		}
		values := url.Values{}
		for k, v := range payload {
			values.Set(k, v)
		}
		return values, nil
	}
	return url.ParseQuery(raw)
}

func ParseOpenAccountPlaintext(plaintext []byte) (*AccountNotification, error) {
	var payload struct {
		MemberID      string `json:"member_id"`
		TerminalID    string `json:"terminal_id"`
		MemberType    string `json:"memberType"`
		State         string `json:"state"`
		ErrorCode     string `json:"errorCode"`
		ErrorMessage  string `json:"errorMsg"`
		TransSerialNo string `json:"transSerialNo"`
		LoginNo       string `json:"loginNo"`
		CustomerName  string `json:"customerName"`
		ContractNo    string `json:"contractNo"`
		NoticeType    string `json:"noticeType"`
		OccurredAt    string `json:"occurredAt"`
	}
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		return nil, err
	}
	outRequestNo := strings.TrimSpace(payload.TransSerialNo)
	openState := contracts.OpenStateFromUpstream(payload.State)
	occurredAt := time.Now().UTC()
	if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(payload.OccurredAt)); err == nil {
		occurredAt = parsed.UTC()
	}
	contractNo := strings.TrimSpace(payload.ContractNo)
	return &AccountNotification{
		OutRequestNo:  outRequestNo,
		ContractNo:    contractNo,
		UpstreamState: strings.TrimSpace(payload.State),
		OpenState:     openState,
		FailCode:      strings.TrimSpace(payload.ErrorCode),
		FailMessage:   strings.TrimSpace(payload.ErrorMessage),
		OccurredAt:    occurredAt,
		Raw:           plaintext,
	}, nil
}

func ParseWithdrawPlaintext(plaintext []byte) (*WithdrawNotification, error) {
	var payload struct {
		ContractNo          string `json:"contractNo"`
		OrderID             string `json:"orderId"`
		TransSerialNo       string `json:"transSerialNo"`
		TransMoney          string `json:"transMoney"`
		TransFee            string `json:"transFee"`
		TransferTotalAmount string `json:"transferTotalAmount"`
		State               string `json:"state"`
		TransRemark         string `json:"transRemark"`
		RequestReserved     string `json:"reqReserved"`
		OccurredAt          string `json:"occurredAt"`
	}
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		return nil, err
	}
	amountFen, err := contracts.YuanStringToFen(payload.TransMoney)
	if err != nil {
		return nil, err
	}
	feeFen, err := contracts.YuanStringToFen(payload.TransFee)
	if err != nil {
		return nil, err
	}
	totalAmountFen, err := contracts.YuanStringToFen(payload.TransferTotalAmount)
	if err != nil {
		return nil, err
	}
	occurredAt := time.Now().UTC()
	if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(payload.OccurredAt)); err == nil {
		occurredAt = parsed.UTC()
	}
	return &WithdrawNotification{
		TransSerialNo:   strings.TrimSpace(payload.TransSerialNo),
		BaofuWithdrawNo: strings.TrimSpace(payload.OrderID),
		ContractNo:      strings.TrimSpace(payload.ContractNo),
		UpstreamState:   strings.TrimSpace(payload.State),
		Status:          contracts.WithdrawStatusFromUpstream(payload.State),
		AmountFen:       amountFen,
		FeeFen:          feeFen,
		TotalAmountFen:  totalAmountFen,
		Remark:          strings.TrimSpace(payload.TransRemark),
		RequestReserved: strings.TrimSpace(payload.RequestReserved),
		OccurredAt:      occurredAt,
		Raw:             plaintext,
	}, nil
}

func AccountNotificationACK() string {
	return "OK"
}
