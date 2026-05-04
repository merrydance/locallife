package notification

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/merrydance/locallife/baofu/account/contracts"
	baofucrypto "github.com/merrydance/locallife/baofu/crypto"
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
	codec *baofucrypto.UnionGWCodec
}

func NewParser(codec *baofucrypto.UnionGWCodec) *Parser {
	return &Parser{codec: codec}
}

func (p *Parser) ParseOpenAccountNotification(body []byte) (*AccountNotification, error) {
	if p == nil || p.codec == nil {
		return nil, errors.New("baofu account notification parser is not configured")
	}
	var envelope baofucrypto.UnionGWEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	plaintext, err := p.codec.OpenEnvelope(envelope)
	if err != nil {
		return nil, err
	}
	return ParseOpenAccountPlaintext(plaintext)
}

func (p *Parser) ParseWithdrawNotification(body []byte) (*WithdrawNotification, error) {
	if p == nil || p.codec == nil {
		return nil, errors.New("baofu account notification parser is not configured")
	}
	var envelope baofucrypto.UnionGWEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	plaintext, err := p.codec.OpenEnvelope(envelope)
	if err != nil {
		return nil, err
	}
	return ParseWithdrawPlaintext(plaintext)
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
