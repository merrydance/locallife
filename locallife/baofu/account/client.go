package account

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/merrydance/locallife/baofu"
	"github.com/merrydance/locallife/baofu/account/contracts"
)

type Client struct {
	root *baofu.Client
}

func NewClient(root *baofu.Client) *Client {
	return &Client{root: root}
}

func (c *Client) OpenAccount(ctx context.Context, req contracts.OpenAccountRequest) (*contracts.AccountResult, error) {
	if c == nil || c.root == nil {
		return nil, errors.New("baofu account client is not configured")
	}
	officialReq, err := officialOpenAccountRequest(req, c.accountOpenNotifyURL())
	if err != nil {
		return nil, err
	}
	var result officialAccountResult
	if err := c.root.PostAccount(ctx, "T-1001-013-01", c.merchantID(""), c.terminalID(""), officialReq, &result); err != nil {
		return nil, err
	}
	return result.toAccountResult(), nil
}

func (c *Client) QueryAccount(ctx context.Context, req contracts.QueryAccountRequest) (*contracts.AccountResult, error) {
	if c == nil || c.root == nil {
		return nil, errors.New("baofu account client is not configured")
	}
	officialReq := contracts.OfficialQueryAccountRequest{
		Version:     contracts.OfficialOpenAccountVersion,
		AccountType: contracts.OfficialAccountTypeBusiness,
		ContractNo:  strings.TrimSpace(req.ContractNo),
		LoginNo:     strings.TrimSpace(req.OutRequestNo),
	}
	if err := officialReq.Validate(); err != nil {
		return nil, err
	}
	var result officialAccountResult
	if err := c.root.PostAccount(ctx, "T-1001-013-03", c.merchantID(""), c.terminalID(""), officialReq, &result); err != nil {
		return nil, err
	}
	return result.toAccountResult(), nil
}

func (c *Client) QueryBalance(ctx context.Context, req contracts.BalanceQueryRequest) (*contracts.BalanceResult, error) {
	if c == nil || c.root == nil {
		return nil, errors.New("baofu account client is not configured")
	}
	officialReq := contracts.OfficialBalanceQueryRequest{
		Version:     contracts.OfficialOpenAccountVersion,
		ContractNo:  strings.TrimSpace(req.ContractNo),
		AccountType: contracts.OfficialAccountTypeBusiness,
	}
	if err := officialReq.Validate(); err != nil {
		return nil, err
	}
	var result officialBalanceResult
	if err := c.root.PostAccount(ctx, "T-1001-013-06", c.merchantID(req.MerchantID), c.terminalID(req.TerminalID), officialReq, &result); err != nil {
		return nil, err
	}
	return result.toBalanceResult()
}

func (c *Client) CreateWithdraw(ctx context.Context, req contracts.WithdrawRequest) (*contracts.WithdrawResult, error) {
	if c == nil || c.root == nil {
		return nil, errors.New("baofu account client is not configured")
	}
	amount, err := contracts.FenToYuanString(req.AmountFen)
	if err != nil {
		return nil, err
	}
	officialReq := contracts.OfficialWithdrawRequest{
		Version:       contracts.OfficialWithdrawVersion,
		ContractNo:    strings.TrimSpace(req.ContractNo),
		TransSerialNo: strings.TrimSpace(req.TransSerialNo),
		DealAmount:    amount,
		ReturnURL:     strings.TrimSpace(req.NotifyURL),
	}
	if err := officialReq.Validate(); err != nil {
		return nil, err
	}
	var result officialWithdrawResult
	if err := c.root.PostAccount(ctx, "T-1001-013-14", c.merchantID(req.MerchantID), c.terminalID(req.TerminalID), officialReq, &result); err != nil {
		return nil, err
	}
	return result.toWithdrawResult(), nil
}

func (c *Client) QueryWithdraw(ctx context.Context, req contracts.WithdrawQueryRequest) (*contracts.WithdrawResult, error) {
	if c == nil || c.root == nil {
		return nil, errors.New("baofu account client is not configured")
	}
	officialReq := contracts.OfficialWithdrawQueryRequest{
		Version:       contracts.OfficialWithdrawVersion,
		TransSerialNo: strings.TrimSpace(req.TransSerialNo),
		TradeTime:     time.Now().Format("2006-01-02"),
	}
	if err := officialReq.Validate(); err != nil {
		return nil, err
	}
	var result officialWithdrawResult
	if err := c.root.PostAccount(ctx, "T-1001-013-15", c.merchantID(req.MerchantID), c.terminalID(req.TerminalID), officialReq, &result); err != nil {
		return nil, err
	}
	return result.toWithdrawResult(), nil
}

func (c *Client) merchantID(value string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return c.root.Config().CollectMerchantID
}

func (c *Client) terminalID(value string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return c.root.Config().CollectTerminalID
}

func (c *Client) accountOpenNotifyURL() string {
	return strings.TrimRight(strings.TrimSpace(c.root.Config().NotifyBaseURL), "/") + "/account/open"
}

func officialOpenAccountRequest(req contracts.OpenAccountRequest, noticeURL string) (contracts.OfficialOpenAccountRequest, error) {
	accountType := contracts.OfficialAccountTypeBusiness
	var accountInfo any
	switch strings.TrimSpace(req.AccountType) {
	case "personal":
		accountType = contracts.OfficialAccountTypePersonal
		if strings.TrimSpace(req.BankAccountNo) == "" {
			accountInfo = contracts.OfficialPersonalTwoFactorAccountInfo{
				TransSerialNo:   strings.TrimSpace(req.OutRequestNo),
				LoginNo:         strings.TrimSpace(req.OutRequestNo),
				CustomerName:    strings.TrimSpace(req.LegalName),
				CertificateType: contracts.OfficialCertificateTypeID,
				CertificateNo:   strings.TrimSpace(req.CertificateNo),
			}
		} else {
			accountInfo = contracts.OfficialPersonalAccountInfo{
				TransSerialNo:   strings.TrimSpace(req.OutRequestNo),
				LoginNo:         strings.TrimSpace(req.OutRequestNo),
				CustomerName:    strings.TrimSpace(req.LegalName),
				CertificateType: contracts.OfficialCertificateTypeID,
				CertificateNo:   strings.TrimSpace(req.CertificateNo),
				CardNo:          strings.TrimSpace(req.BankAccountNo),
				MobileNo:        strings.TrimSpace(req.BankMobile),
				CardUserName:    strings.TrimSpace(req.LegalName),
			}
		}
	default:
		accountInfo = contracts.OfficialBusinessAccountInfo{
			TransSerialNo:   strings.TrimSpace(req.OutRequestNo),
			LoginNo:         strings.TrimSpace(req.OutRequestNo),
			CustomerName:    strings.TrimSpace(req.LegalName),
			CertificateType: contracts.OfficialCertificateTypeID,
			CertificateNo:   strings.TrimSpace(req.CertificateNo),
		}
	}
	officialReq := contracts.OfficialOpenAccountRequest{
		Version:      contracts.OfficialOpenAccountVersion,
		AccountType:  accountType,
		AccountInfo:  accountInfo,
		NoticeURL:    strings.TrimSpace(noticeURL),
		BusinessType: contracts.OfficialBusinessTypeBCT20,
	}
	if err := officialReq.Validate(); err != nil {
		return contracts.OfficialOpenAccountRequest{}, err
	}
	return officialReq, nil
}

type officialAccountResult struct {
	TransSerialNo string          `json:"transSerialNo"`
	ContractNo    string          `json:"contractNo"`
	State         string          `json:"state"`
	ErrorCode     string          `json:"errorCode"`
	ErrorMessage  string          `json:"errorMsg"`
	Result        json.RawMessage `json:"result"`
	Raw           json.RawMessage `json:"-"`
}

func (r officialAccountResult) toAccountResult() *contracts.AccountResult {
	item := r.firstResultItem()
	contractNo := firstNonEmpty(item.ContractNo, r.ContractNo)
	state := firstNonEmpty(item.State, r.State)
	return &contracts.AccountResult{
		OutRequestNo:  firstNonEmpty(item.TransSerialNo, r.TransSerialNo),
		ContractNo:    contractNo,
		SharingMerID:  contractNo,
		UpstreamState: state,
		OpenState:     contracts.OpenStateFromUpstream(state),
		FailCode:      firstNonEmpty(item.ErrorCode, r.ErrorCode),
		FailMessage:   firstNonEmpty(item.ErrorMessage, r.ErrorMessage),
	}
}

func (r officialAccountResult) firstResultItem() officialAccountResultItem {
	raw := strings.TrimSpace(string(r.Result))
	if raw == "" || raw == "null" {
		return officialAccountResultItem{}
	}
	if strings.HasPrefix(raw, "[") {
		var items []officialAccountResultItem
		if err := json.Unmarshal(r.Result, &items); err != nil || len(items) == 0 {
			return officialAccountResultItem{}
		}
		return items[0]
	}
	var item officialAccountResultItem
	if err := json.Unmarshal(r.Result, &item); err != nil {
		return officialAccountResultItem{}
	}
	return item
}

type officialAccountResultItem struct {
	TransSerialNo string `json:"transSerialNo"`
	ContractNo    string `json:"contractNo"`
	State         string `json:"state"`
	ErrorCode     string `json:"errorCode"`
	ErrorMessage  string `json:"errorMsg"`
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

type officialBalanceResult struct {
	ContractNo   string `json:"contractNo"`
	AvailableBal string `json:"availableBal"`
	PendingBal   string `json:"pendingBal"`
	CurrBal      string `json:"currBal"`
	FreezeBal    string `json:"freezeBal"`
}

func (r officialBalanceResult) toBalanceResult() (*contracts.BalanceResult, error) {
	available, err := contracts.YuanStringToFen(r.AvailableBal)
	if err != nil {
		return nil, err
	}
	pending, err := contracts.YuanStringToFen(r.PendingBal)
	if err != nil {
		return nil, err
	}
	ledger, err := contracts.YuanStringToFen(r.CurrBal)
	if err != nil {
		return nil, err
	}
	frozen, err := contracts.YuanStringToFen(r.FreezeBal)
	if err != nil {
		return nil, err
	}
	return &contracts.BalanceResult{ContractNo: strings.TrimSpace(r.ContractNo), AvailableAmountFen: available, PendingAmountFen: pending, LedgerAmountFen: ledger, FrozenAmountFen: frozen, UpstreamAvailable: r.AvailableBal, UpstreamPending: r.PendingBal, UpstreamLedger: r.CurrBal, UpstreamFrozen: r.FreezeBal}, nil
}

type officialWithdrawResult struct {
	TransSerialNo string          `json:"transSerialNo"`
	OrderID       string          `json:"orderId"`
	ContractNo    string          `json:"contractNo"`
	State         string          `json:"state"`
	Raw           json.RawMessage `json:"-"`
}

func (r officialWithdrawResult) toWithdrawResult() *contracts.WithdrawResult {
	return &contracts.WithdrawResult{TransSerialNo: strings.TrimSpace(r.TransSerialNo), BaofuWithdrawNo: strings.TrimSpace(r.OrderID), ContractNo: strings.TrimSpace(r.ContractNo), UpstreamState: strings.TrimSpace(r.State), Status: contracts.WithdrawStatusFromUpstream(r.State)}
}
