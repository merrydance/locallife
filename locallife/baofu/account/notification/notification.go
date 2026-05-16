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
	MemberID      string
	TerminalID    string
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
	MemberID        string
	TerminalID      string
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
	decoded, err := p.decodeOfficialDataContent(body)
	if err != nil {
		return nil, err
	}
	notification, err := ParseOpenAccountPlaintext(decoded.plaintext)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(notification.MemberID) != decoded.memberID {
		return nil, errors.New("baofu open account notification member_id does not match transport")
	}
	if strings.TrimSpace(notification.TerminalID) != decoded.terminalID {
		return nil, errors.New("baofu open account notification terminal_id does not match transport")
	}
	return notification, nil
}

func (p *Parser) ParseWithdrawNotification(body []byte) (*WithdrawNotification, error) {
	if p == nil || p.baofuPublicKeyPEM == "" {
		return nil, errors.New("baofu account notification parser is not configured")
	}
	decoded, err := p.decodeOfficialDataContent(body)
	if err != nil {
		return nil, err
	}
	notification, err := ParseWithdrawPlaintext(decoded.plaintext)
	if err != nil {
		return nil, err
	}
	notification.MemberID = decoded.memberID
	notification.TerminalID = decoded.terminalID
	return notification, nil
}

type officialDecodedNotification struct {
	memberID   string
	terminalID string
	plaintext  []byte
}

func (p *Parser) decodeOfficialDataContent(body []byte) (officialDecodedNotification, error) {
	values, err := parseOfficialNotificationValues(body)
	if err != nil {
		return officialDecodedNotification{}, err
	}
	memberID := strings.TrimSpace(values.Get("member_id"))
	if memberID == "" {
		return officialDecodedNotification{}, errors.New("baofu account notification member_id is required")
	}
	terminalID := strings.TrimSpace(values.Get("terminal_id"))
	if terminalID == "" {
		return officialDecodedNotification{}, errors.New("baofu account notification terminal_id is required")
	}
	dataType := strings.TrimSpace(values.Get("data_type"))
	if dataType == "" {
		return officialDecodedNotification{}, errors.New("baofu account notification data_type is required")
	}
	if !strings.EqualFold(dataType, "JSON") {
		return officialDecodedNotification{}, errors.New("baofu account notification data_type must be JSON")
	}
	dataContent := strings.TrimSpace(values.Get("data_content"))
	if dataContent == "" {
		return officialDecodedNotification{}, errors.New("baofu account notification data_content is required")
	}
	plaintext, err := baofu.DecodeUnionGWVerifyType1Content(p.baofuPublicKeyPEM, dataContent)
	if err != nil {
		return officialDecodedNotification{}, err
	}
	return officialDecodedNotification{memberID: memberID, terminalID: terminalID, plaintext: plaintext}, nil
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
		MemberID        string `json:"member_id"`
		MemberIDCamel   string `json:"memberId"`
		TerminalID      string `json:"terminal_id"`
		TerminalIDCamel string `json:"terminalId"`
		MemberType      string `json:"memberType"`
		State           string `json:"state"`
		ErrorCode       string `json:"errorCode"`
		ErrorMessage    string `json:"errorMsg"`
		TransSerialNo   string `json:"transSerialNo"`
		LoginNo         string `json:"loginNo"`
		CustomerName    string `json:"customerName"`
		ContractNo      string `json:"contractNo"`
		NoticeType      string `json:"noticeType"`
		OccurredAt      string `json:"occurredAt"`
	}
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		return nil, err
	}
	memberID, err := openAccountNotificationIdentityValue("member_id", payload.MemberID, "memberId", payload.MemberIDCamel)
	if err != nil {
		return nil, err
	}
	terminalID, err := openAccountNotificationIdentityValue("terminal_id", payload.TerminalID, "terminalId", payload.TerminalIDCamel)
	if err != nil {
		return nil, err
	}
	for _, field := range []struct{ name, value string }{
		{"member_id", memberID},
		{"terminal_id", terminalID},
		{"memberType", payload.MemberType},
		{"state", payload.State},
		{"transSerialNo", payload.TransSerialNo},
		{"loginNo", payload.LoginNo},
		{"customerName", payload.CustomerName},
		{"contractNo", payload.ContractNo},
		{"noticeType", payload.NoticeType},
	} {
		if strings.TrimSpace(field.value) == "" {
			return nil, errors.New("baofu open account notification " + field.name + " is required")
		}
	}
	if !isSupportedOpenAccountNotifyState(payload.State) {
		return nil, errors.New("baofu open account notification state is unsupported")
	}
	outRequestNo := strings.TrimSpace(payload.TransSerialNo)
	openState := contracts.OpenStateFromUpstream(payload.State)
	occurredAt := time.Now().UTC()
	if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(payload.OccurredAt)); err == nil {
		occurredAt = parsed.UTC()
	}
	contractNo := strings.TrimSpace(payload.ContractNo)
	return &AccountNotification{
		MemberID:      memberID,
		TerminalID:    terminalID,
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

func openAccountNotificationIdentityValue(snakeName string, snakeValue string, camelName string, camelValue string) (string, error) {
	snakeValue = strings.TrimSpace(snakeValue)
	camelValue = strings.TrimSpace(camelValue)
	if snakeValue != "" && camelValue != "" && snakeValue != camelValue {
		return "", errors.New("baofu open account notification " + snakeName + " does not match " + camelName)
	}
	if snakeValue != "" {
		return snakeValue, nil
	}
	return camelValue, nil
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
	for _, field := range []struct{ name, value string }{
		{"contractNo", payload.ContractNo},
		{"orderId", payload.OrderID},
		{"transSerialNo", payload.TransSerialNo},
		{"transMoney", payload.TransMoney},
		{"transFee", payload.TransFee},
		{"transferTotalAmount", payload.TransferTotalAmount},
		{"state", payload.State},
		{"transRemark", payload.TransRemark},
		{"reqReserved", payload.RequestReserved},
	} {
		if strings.TrimSpace(field.value) == "" {
			return nil, errors.New("baofu withdraw notification " + field.name + " is required")
		}
	}
	if !isSupportedWithdrawNotifyState(payload.State) {
		return nil, errors.New("baofu withdraw notification state is unsupported")
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

func isSupportedOpenAccountNotifyState(state string) bool {
	switch strings.TrimSpace(state) {
	case "1", "0", "-1", "2":
		return true
	default:
		return false
	}
}

func isSupportedWithdrawNotifyState(state string) bool {
	switch strings.TrimSpace(state) {
	case "0", "1", "2", "3":
		return true
	default:
		return false
	}
}
