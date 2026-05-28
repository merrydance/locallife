package account

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

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
	if err := c.root.PostAccount(ctx, "T-1001-013-01", c.collectMerchantID(), c.collectTerminalID(), officialReq, &result); err != nil {
		return nil, err
	}
	if err := result.validateOpenAccountResponse(); err != nil {
		return nil, baofu.NewProviderContractError("T-1001-013-01", err)
	}
	return result.toAccountResult(), nil
}

func (c *Client) QueryAccount(ctx context.Context, req contracts.QueryAccountRequest) (*contracts.AccountResult, error) {
	if c == nil || c.root == nil {
		return nil, errors.New("baofu account client is not configured")
	}
	officialReq := contracts.OfficialQueryAccountRequest{
		Version:         contracts.OfficialQueryAccountVersion,
		AccountType:     officialAccountType(req.AccountType),
		ContractNo:      strings.TrimSpace(req.ContractNo),
		LoginNo:         strings.TrimSpace(req.LoginNo),
		CertificateNo:   strings.TrimSpace(req.CertificateNo),
		CertificateType: strings.TrimSpace(req.CertificateType),
		PlatformNo:      strings.TrimSpace(req.PlatformNo),
	}
	if err := officialReq.Validate(); err != nil {
		return nil, err
	}
	var result officialAccountResult
	if err := c.root.PostAccount(ctx, "T-1001-013-03", c.collectMerchantID(), c.collectTerminalID(), officialReq, &result); err != nil {
		return nil, err
	}
	if err := result.validateQueryAccountResponse(); err != nil {
		return nil, baofu.NewProviderContractError("T-1001-013-03", err)
	}
	return result.toAccountResult(), nil
}

func (c *Client) QueryBalance(ctx context.Context, req contracts.BalanceQueryRequest) (*contracts.BalanceResult, error) {
	if c == nil || c.root == nil {
		return nil, errors.New("baofu account client is not configured")
	}
	officialReq := contracts.OfficialBalanceQueryRequest{
		Version:     contracts.OfficialBalanceVersion,
		ContractNo:  strings.TrimSpace(req.ContractNo),
		AccountType: officialAccountType(req.AccountType),
	}
	if err := officialReq.Validate(); err != nil {
		return nil, err
	}
	var result officialBalanceResult
	if err := c.root.PostAccount(ctx, "T-1001-013-06", c.merchantID(req.MerchantID), c.terminalID(req.TerminalID), officialReq, &result); err != nil {
		return nil, err
	}
	balance, err := result.toBalanceResult()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(balance.ContractNo) == "" {
		balance.ContractNo = strings.TrimSpace(req.ContractNo)
	}
	return balance, nil
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
		FeeMemberID:   strings.TrimSpace(req.FeeMemberID),
		ReturnURL:     strings.TrimSpace(req.NotifyURL),
	}
	if err := officialReq.Validate(); err != nil {
		return nil, err
	}
	var result officialWithdrawResult
	if err := c.root.PostAccount(ctx, "T-1001-013-14", c.merchantID(req.MerchantID), c.terminalID(req.TerminalID), officialReq, &result); err != nil {
		return nil, err
	}
	if err := result.validateWithdrawAcceptanceResponse(); err != nil {
		return nil, baofu.NewProviderContractError("T-1001-013-14", err)
	}
	withdrawResult, err := result.toWithdrawAcceptanceResult()
	if err != nil {
		return nil, baofu.NewProviderContractError("T-1001-013-14", err)
	}
	return withdrawResult, nil
}

func (c *Client) QueryWithdraw(ctx context.Context, req contracts.WithdrawQueryRequest) (*contracts.WithdrawResult, error) {
	if c == nil || c.root == nil {
		return nil, errors.New("baofu account client is not configured")
	}
	officialReq := contracts.OfficialWithdrawQueryRequest{
		Version:       contracts.OfficialWithdrawVersion,
		TransSerialNo: strings.TrimSpace(req.TransSerialNo),
		TradeTime:     strings.TrimSpace(req.TradeTime),
	}
	if err := officialReq.Validate(); err != nil {
		return nil, err
	}
	var result officialWithdrawResult
	if err := c.root.PostAccount(ctx, "T-1001-013-15", c.merchantID(req.MerchantID), c.terminalID(req.TerminalID), officialReq, &result); err != nil {
		return nil, err
	}
	if err := result.validateWithdrawQueryResponse(); err != nil {
		return nil, baofu.NewProviderContractError("T-1001-013-15", err)
	}
	withdrawResult, err := result.toWithdrawResult()
	if err != nil {
		return nil, baofu.NewProviderContractError("T-1001-013-15", err)
	}
	return withdrawResult, nil
}

func (c *Client) merchantID(value string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return c.root.Config().PayoutMerchantID
}

func (c *Client) terminalID(value string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return c.root.Config().PayoutTerminalID
}

func (c *Client) collectMerchantID() string {
	return c.root.Config().CollectMerchantID
}

func (c *Client) collectTerminalID() string {
	return c.root.Config().CollectTerminalID
}

func (c *Client) accountOpenNotifyURL() string {
	return strings.TrimRight(strings.TrimSpace(c.root.Config().NotifyBaseURL), "/") + "/account/open"
}

func officialOpenAccountRequest(req contracts.OpenAccountRequest, noticeURL string) (contracts.OfficialOpenAccountRequest, error) {
	if err := req.Validate(); err != nil {
		return contracts.OfficialOpenAccountRequest{}, err
	}
	accountType := officialAccountType(req.AccountType)
	loginNo := firstNonEmpty(req.LoginNo, req.OutRequestNo)
	var accountInfo any
	switch strings.ToLower(strings.TrimSpace(req.AccountType)) {
	case "personal":
		cardUserName := firstNonEmpty(req.CardUserName, req.LegalName)
		accountInfo = contracts.OfficialPersonalAccountInfo{
			TransSerialNo:   strings.TrimSpace(req.OutRequestNo),
			LoginNo:         strings.TrimSpace(loginNo),
			CustomerName:    strings.TrimSpace(req.LegalName),
			CertificateType: contracts.OfficialCertificateTypeID,
			CertificateNo:   strings.TrimSpace(req.CertificateNo),
			CardNo:          strings.TrimSpace(req.BankAccountNo),
			MobileNo:        strings.TrimSpace(req.BankMobile),
			CardUserName:    strings.TrimSpace(cardUserName),
		}
	default:
		certificateType := firstNonEmpty(req.CertificateType, contracts.OfficialBusinessCertificateTypeLicense)
		accountInfo = contracts.OfficialBusinessAccountInfo{
			TransSerialNo:       strings.TrimSpace(req.OutRequestNo),
			LoginNo:             strings.TrimSpace(loginNo),
			Email:               strings.TrimSpace(req.Email),
			SelfEmployed:        req.SelfEmployed,
			CustomerName:        strings.TrimSpace(firstNonEmpty(req.CustomerName, req.LegalName)),
			AliasName:           strings.TrimSpace(req.AliasName),
			CertificateType:     strings.TrimSpace(certificateType),
			CertificateNo:       strings.TrimSpace(req.CertificateNo),
			CorporateName:       strings.TrimSpace(req.CorporateName),
			CorporateCertType:   strings.TrimSpace(req.CorporateCertType),
			CorporateCertID:     strings.TrimSpace(req.CorporateCertID),
			CorporateMobile:     strings.TrimSpace(req.CorporateMobile),
			IndustryID:          strings.TrimSpace(req.IndustryID),
			ContactName:         strings.TrimSpace(req.ContactName),
			ContactMobile:       strings.TrimSpace(req.ContactMobile),
			CardNo:              strings.TrimSpace(req.BankAccountNo),
			BankName:            strings.TrimSpace(req.BankName),
			DepositBankProvince: strings.TrimSpace(req.DepositBankProvince),
			DepositBankCity:     strings.TrimSpace(req.DepositBankCity),
			DepositBankName:     strings.TrimSpace(req.DepositBankName),
			RegisterCapital:     strings.TrimSpace(req.RegisterCapital),
			CardUserName:        strings.TrimSpace(req.CardUserName),
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

func officialAccountType(accountType string) int {
	switch strings.ToLower(strings.TrimSpace(accountType)) {
	case "personal":
		return contracts.OfficialAccountTypePersonal
	default:
		return contracts.OfficialAccountTypeBusiness
	}
}

type officialAccountResult struct {
	TransSerialNo officialScalarString `json:"transSerialNo"`
	ContractNo    officialScalarString `json:"contractNo"`
	State         officialScalarString `json:"state"`
	ErrorCode     officialScalarString `json:"errorCode"`
	ErrorMessage  officialScalarString `json:"errorMsg"`
	Result        json.RawMessage      `json:"result"`
	Operation     string               `json:"-"`
	RetCode       string               `json:"-"`
}

func (r *officialAccountResult) SetRaw(raw json.RawMessage) {
	if r == nil {
		return
	}
	var payload map[string]json.RawMessage
	if len(raw) == 0 || json.Unmarshal(raw, &payload) != nil {
		return
	}
	r.RetCode = accountScalarString(payload["retCode"])
	if len(r.Result) == 0 {
		r.Result = append(r.Result[:0], payload["result"]...)
	}
}

func accountScalarString(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return ""
	}
	var decoded string
	if err := json.Unmarshal(raw, &decoded); err == nil {
		return strings.TrimSpace(decoded)
	}
	return strings.Trim(trimmed, `"`)
}

func (r *officialAccountResult) SetOperation(operation string) {
	if r != nil {
		r.Operation = strings.TrimSpace(operation)
	}
}

func (r officialAccountResult) toAccountResult() *contracts.AccountResult {
	item := r.firstResultItem()
	contractNo := firstNonEmpty(item.ContractNo.String(), r.ContractNo.String())
	state := firstNonEmpty(item.State.String(), r.State.String())
	if state == "" && contractNo != "" {
		state = "1"
	}
	failCode := firstNonEmpty(item.ErrorCode.String(), r.ErrorCode.String())
	if isOfficialSuccessCode(failCode) {
		failCode = ""
	}
	return &contracts.AccountResult{
		OutRequestNo:  firstNonEmpty(item.TransSerialNo.String(), r.TransSerialNo.String()),
		ContractNo:    contractNo,
		SharingMerID:  contractNo,
		UpstreamState: state,
		OpenState:     contracts.OpenStateFromUpstream(state),
		FailCode:      failCode,
		FailMessage:   firstNonEmpty(item.ErrorMessage.String(), r.ErrorMessage.String()),
		Raw:           r.safeDiagnosticSnapshot(item),
	}
}

func (r officialAccountResult) safeDiagnosticSnapshot(item officialAccountResultItem) []byte {
	snapshot := map[string]any{
		"provider":   "baofu",
		"capability": "account",
	}
	if v := strings.TrimSpace(r.Operation); v != "" {
		snapshot["operation"] = v
	}
	if v := strings.TrimSpace(r.RetCode); v != "" {
		snapshot["ret_code"] = v
	}
	if v := strings.TrimSpace(item.State.String()); v != "" {
		snapshot["result_state"] = v
	}
	resultCode := strings.TrimSpace(item.ErrorCode.String())
	if resultCode != "" && !isOfficialSuccessCode(resultCode) {
		snapshot["source_path"] = "body.result[0].errorCode"
		snapshot["result_error_code"] = resultCode
		if strings.TrimSpace(item.ErrorMessage.String()) != "" {
			snapshot["result_error_message_present"] = true
		}
	} else if _, ok := accountFirstResultItemForRaw(r.Result); ok {
		snapshot["source_path"] = "body.result[0]"
	} else if strings.TrimSpace(r.State.String()) != "" || strings.TrimSpace(r.ErrorCode.String()) != "" {
		snapshot["source_path"] = "body"
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return []byte(`{"provider":"baofu","capability":"account"}`)
	}
	return raw
}

func (r officialAccountResult) firstResultItem() officialAccountResultItem {
	if item, ok := accountFirstResultItemForRaw(r.Result); ok {
		return item
	}
	return officialAccountResultItem{}
}

func accountFirstResultItemForRaw(raw json.RawMessage) (officialAccountResultItem, bool) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return officialAccountResultItem{}, false
	}
	if strings.HasPrefix(trimmed, "[") {
		var items []officialAccountResultItem
		if err := json.Unmarshal(raw, &items); err != nil || len(items) == 0 {
			return officialAccountResultItem{}, false
		}
		return items[0], true
	}
	var item officialAccountResultItem
	if err := json.Unmarshal(raw, &item); err != nil {
		return officialAccountResultItem{}, false
	}
	return item, true
}

type officialAccountResultItem struct {
	TransSerialNo officialScalarString `json:"transSerialNo"`
	LoginNo       officialScalarString `json:"loginNo"`
	CustomerName  officialScalarString `json:"customerName"`
	ContractNo    officialScalarString `json:"contractNo"`
	State         officialScalarString `json:"state"`
	ErrorCode     officialScalarString `json:"errorCode"`
	ErrorMessage  officialScalarString `json:"errorMsg"`
}

func (r officialAccountResult) validateOpenAccountResponse() error {
	item := r.firstResultItem()
	for _, field := range []struct{ name, value string }{
		{"state", item.State.String()},
		{"transSerialNo", item.TransSerialNo.String()},
		{"loginNo", item.LoginNo.String()},
		{"customerName", item.CustomerName.String()},
	} {
		if strings.TrimSpace(field.value) == "" {
			return errors.New("baofu open account response " + field.name + " is required")
		}
	}
	if !isOfficialOpenAccountState(item.State.String()) {
		return errors.New("baofu open account response state is unsupported")
	}
	if item.State.String() == "1" && strings.TrimSpace(item.ContractNo.String()) == "" {
		return errors.New("baofu open account response contractNo is required for success")
	}
	return nil
}

func (r officialAccountResult) validateQueryAccountResponse() error {
	if strings.TrimSpace(r.firstResultItem().ContractNo.String()) == "" {
		return errors.New("baofu query account response contractNo is required")
	}
	return nil
}

func isOfficialOpenAccountState(value string) bool {
	switch strings.TrimSpace(value) {
	case "1", "0", "-1", "2":
		return true
	default:
		return false
	}
}

type officialScalarString string

func (s *officialScalarString) UnmarshalJSON(raw []byte) error {
	value := strings.TrimSpace(string(raw))
	if value == "" || value == "null" {
		*s = ""
		return nil
	}
	var decoded string
	if err := json.Unmarshal(raw, &decoded); err == nil {
		*s = officialScalarString(strings.TrimSpace(decoded))
		return nil
	}
	*s = officialScalarString(strings.Trim(value, `"`))
	return nil
}

func (s officialScalarString) String() string {
	return strings.TrimSpace(string(s))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func isOfficialSuccessCode(value string) bool {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "", "1", "SUCCESS":
		return true
	default:
		return false
	}
}

type officialBalanceResult struct {
	ContractNo   officialScalarString `json:"contractNo"`
	AvailableBal officialScalarString `json:"availableBal"`
	PendingBal   officialScalarString `json:"pendingBal"`
	CurrBal      officialScalarString `json:"currBal"`
	FreezeBal    officialScalarString `json:"freezeBal"`
}

func (r officialBalanceResult) toBalanceResult() (*contracts.BalanceResult, error) {
	if firstNonEmpty(r.AvailableBal.String(), r.PendingBal.String(), r.CurrBal.String(), r.FreezeBal.String()) == "" {
		return nil, errors.New("baofu balance response amount fields are required")
	}
	available, err := optionalOfficialBalanceAmountFen(r.AvailableBal)
	if err != nil {
		return nil, err
	}
	pending, err := optionalOfficialBalanceAmountFen(r.PendingBal)
	if err != nil {
		return nil, err
	}
	ledger, err := optionalOfficialBalanceAmountFen(r.CurrBal)
	if err != nil {
		return nil, err
	}
	frozen, err := optionalOfficialBalanceAmountFen(r.FreezeBal)
	if err != nil {
		return nil, err
	}
	return &contracts.BalanceResult{
		ContractNo:         r.ContractNo.String(),
		AvailableAmountFen: available,
		PendingAmountFen:   pending,
		LedgerAmountFen:    ledger,
		FrozenAmountFen:    frozen,
		UpstreamAvailable:  r.AvailableBal.String(),
		UpstreamPending:    r.PendingBal.String(),
		UpstreamLedger:     r.CurrBal.String(),
		UpstreamFrozen:     r.FreezeBal.String(),
	}, nil
}

func optionalOfficialBalanceAmountFen(value officialScalarString) (int64, error) {
	amount := value.String()
	if amount == "" {
		return 0, nil
	}
	return contracts.YuanStringToFen(amount)
}

type officialWithdrawResult struct {
	MemberID            officialScalarString `json:"memberId"`
	TransSerialNo       officialScalarString `json:"transSerialNo"`
	OrderID             officialScalarString `json:"orderId"`
	ContractNo          officialScalarString `json:"contractNo"`
	State               officialScalarString `json:"state"`
	TransMoney          officialScalarString `json:"transMoney"`
	TransFee            officialScalarString `json:"transFee"`
	TransferTotalAmount officialScalarString `json:"transferTotalAmount"`
	TransRemark         officialScalarString `json:"transRemark"`
	Raw                 json.RawMessage      `json:"-"`
}

func (r officialWithdrawResult) validateWithdrawAcceptanceResponse() error {
	if strings.TrimSpace(r.TransSerialNo.String()) == "" {
		return errors.New("baofu withdraw response transSerialNo is required")
	}
	if strings.TrimSpace(r.ContractNo.String()) == "" {
		return errors.New("baofu withdraw response contractNo is required")
	}
	switch strings.TrimSpace(r.State.String()) {
	case "1", "2":
		return nil
	case "":
		return errors.New("baofu withdraw response state is required")
	default:
		return errors.New("baofu withdraw response state is unsupported")
	}
}

func (r officialWithdrawResult) validateWithdrawQueryResponse() error {
	for _, field := range []struct{ name, value string }{
		{"memberId", r.MemberID.String()},
		{"transSerialNo", r.TransSerialNo.String()},
		{"state", r.State.String()},
		{"contractNo", r.ContractNo.String()},
		{"transMoney", r.TransMoney.String()},
		{"transFee", r.TransFee.String()},
		{"transferTotalAmount", r.TransferTotalAmount.String()},
	} {
		if strings.TrimSpace(field.value) == "" {
			return errors.New("baofu withdraw query response " + field.name + " is required")
		}
	}
	switch strings.TrimSpace(r.State.String()) {
	case "0", "1", "2", "3":
	default:
		return errors.New("baofu withdraw query response state is unsupported")
	}
	if _, err := contracts.YuanStringToFen(r.TransMoney.String()); err != nil {
		return err
	}
	if _, err := contracts.YuanStringToFen(r.TransFee.String()); err != nil {
		return err
	}
	if _, err := contracts.YuanStringToFen(r.TransferTotalAmount.String()); err != nil {
		return err
	}
	return nil
}

func (r officialWithdrawResult) toWithdrawResult() (*contracts.WithdrawResult, error) {
	return r.toWithdrawResultWithStatus(contracts.WithdrawStatusFromUpstream(r.State.String()))
}

func (r officialWithdrawResult) toWithdrawAcceptanceResult() (*contracts.WithdrawResult, error) {
	return r.toWithdrawResultWithStatus(contracts.WithdrawAcceptanceStatusFromUpstream(r.State.String()))
}

func (r officialWithdrawResult) toWithdrawResultWithStatus(status string) (*contracts.WithdrawResult, error) {
	amount, err := optionalOfficialBalanceAmountFen(r.TransMoney)
	if err != nil {
		return nil, err
	}
	fee, err := optionalOfficialBalanceAmountFen(r.TransFee)
	if err != nil {
		return nil, err
	}
	total, err := optionalOfficialBalanceAmountFen(r.TransferTotalAmount)
	if err != nil {
		return nil, err
	}
	return &contracts.WithdrawResult{
		TransSerialNo:   r.TransSerialNo.String(),
		BaofuWithdrawNo: r.OrderID.String(),
		ContractNo:      r.ContractNo.String(),
		UpstreamState:   r.State.String(),
		Status:          status,
		AmountFen:       amount,
		FeeFen:          fee,
		TotalAmountFen:  total,
		Remark:          r.TransRemark.String(),
	}, nil
}
