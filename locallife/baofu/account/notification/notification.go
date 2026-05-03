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
	var payload struct {
		OutRequestNo  string `json:"outRequestNo"`
		TransSerialNo string `json:"transSerialNo"`
		ContractNo    string `json:"contractNo"`
		SharingMerID  string `json:"sharingMerId"`
		Status        string `json:"status"`
		OpenState     string `json:"openState"`
		FailCode      string `json:"failCode"`
		FailMessage   string `json:"failMessage"`
		OccurredAt    string `json:"occurredAt"`
	}
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		return nil, err
	}
	outRequestNo := strings.TrimSpace(payload.OutRequestNo)
	if outRequestNo == "" {
		outRequestNo = strings.TrimSpace(payload.TransSerialNo)
	}
	openState := strings.TrimSpace(payload.OpenState)
	if openState == "" {
		openState = contracts.OpenStateFromUpstream(payload.Status)
	}
	occurredAt := time.Now().UTC()
	if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(payload.OccurredAt)); err == nil {
		occurredAt = parsed.UTC()
	}
	sharingMerID := strings.TrimSpace(payload.SharingMerID)
	contractNo := strings.TrimSpace(payload.ContractNo)
	return &AccountNotification{
		OutRequestNo:  outRequestNo,
		ContractNo:    contractNo,
		SharingMerID:  sharingMerID,
		UpstreamState: strings.TrimSpace(payload.Status),
		OpenState:     openState,
		FailCode:      strings.TrimSpace(payload.FailCode),
		FailMessage:   strings.TrimSpace(payload.FailMessage),
		OccurredAt:    occurredAt,
		Raw:           plaintext,
	}, nil
}
