package wechat

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	wechaterrorcodes "github.com/merrydance/locallife/wechat/errorcodes"
)

const (
	tradeBillAPIPath           = "/v3/bill/tradebill"
	fundFlowBillAPIPath        = "/v3/bill/fundflowbill"
	refundBillAPIPath          = "/v3/bill/refundbill"
	profitSharingBillAPIPath   = "/v3/profitsharing/bills"
	ecommerceTradeBillAPIPath  = "/v3/ecommerce/bill/tradebill"
	ecommerceRefundBillAPIPath = "/v3/ecommerce/bill/refundbill"
)

var (
	ErrBillNotReady = errors.New("wechat bill not ready")
	ErrBillNotFound = errors.New("wechat bill not found")
)

type billDownloadStateError struct {
	Kind  error
	Cause error
}

func (e *billDownloadStateError) Error() string {
	if e.Cause != nil {
		return e.Cause.Error()
	}
	if e.Kind != nil {
		return e.Kind.Error()
	}
	return "wechat bill unavailable"
}

func (e *billDownloadStateError) Unwrap() []error {
	if e.Cause == nil {
		return []error{e.Kind}
	}

	return []error{e.Kind, e.Cause}
}

// BillDownloadURLResponse 微信支付账单下载地址响应
type BillDownloadURLResponse struct {
	HashType    string `json:"hash_type"`
	HashValue   string `json:"hash_value"`
	DownloadURL string `json:"download_url"`
}

// BillRecord 解析后的单条账单记录
type BillRecord struct {
	OutTradeNo    string // 商户订单号 / 商户退款单号
	TransactionID string // 微信订单号 / 微信退款单号
	Amount        int64  // 金额（分）
}

// BillClientInterface 账单下载接口（用于每日对账调度器）
// *EcommerceClient 通过内嵌 *PaymentClient 同时实现所有方法
type BillClientInterface interface {
	// DownloadTradeBill 下载小程序直连支付交易账单（对账用）
	// 返回 map[out_trade_no]BillRecord
	DownloadTradeBill(ctx context.Context, billDate time.Time) (map[string]BillRecord, error)
	// DownloadRefundBill 下载退款账单（对账用）
	// 返回 map[out_refund_no]BillRecord
	DownloadRefundBill(ctx context.Context, billDate time.Time) (map[string]BillRecord, error)
	// DownloadEcommerceTradeBill 下载电商收付通合单交易账单（对账用）
	// 返回 map[合单商户单号]BillRecord
	DownloadEcommerceTradeBill(ctx context.Context, billDate time.Time) (map[string]BillRecord, error)
	// DownloadEcommerceRefundBill 下载电商收付通退款账单（对账用）
	// 对应通过 /v3/ecommerce/refunds/apply 产生的退款，区别于直连退款账单
	// 返回 map[out_refund_no]BillRecord
	DownloadEcommerceRefundBill(ctx context.Context, billDate time.Time) (map[string]BillRecord, error)
}

// DownloadTradeBill 下载指定日期的小程序直连支付交易账单
// 对应 payment_orders WHERE payment_type = 'miniprogram'
func (c *PaymentClient) DownloadTradeBill(ctx context.Context, billDate time.Time) (map[string]BillRecord, error) {
	apiPath := fmt.Sprintf("%s?bill_date=%s&bill_type=ALL&tar_type=GZIP",
		tradeBillAPIPath, billDate.Format("2006-01-02"))
	return c.fetchAndParseBill(ctx, apiPath, "商户订单号", "微信订单号", "订单金额")
}

// GetTradeBillDownloadURL 获取交易账单下载地址。
func (c *PaymentClient) GetTradeBillDownloadURL(ctx context.Context, billDate time.Time, subMchID, billType, tarType string) (*BillDownloadURLResponse, error) {
	params := url.Values{}
	params.Set("bill_date", billDate.Format("2006-01-02"))
	if subMchID != "" {
		params.Set("sub_mchid", subMchID)
	}
	if billType != "" {
		params.Set("bill_type", billType)
	}
	if tarType != "" {
		params.Set("tar_type", tarType)
	}

	return c.getBillDownloadURL(ctx, buildBillRequestPath(tradeBillAPIPath, params))
}

// GetFundFlowBillDownloadURL 获取资金账单下载地址。
func (c *PaymentClient) GetFundFlowBillDownloadURL(ctx context.Context, billDate time.Time, accountType, tarType string) (*BillDownloadURLResponse, error) {
	params := url.Values{}
	params.Set("bill_date", billDate.Format("2006-01-02"))
	if accountType != "" {
		params.Set("account_type", accountType)
	}
	if tarType != "" {
		params.Set("tar_type", tarType)
	}

	return c.getBillDownloadURL(ctx, buildBillRequestPath(fundFlowBillAPIPath, params))
}

// DownloadRefundBill 下载指定日期的退款账单
// 对应 refund_orders WHERE refund_type = 'miniprogram' AND status = 'success'
func (c *PaymentClient) DownloadRefundBill(ctx context.Context, billDate time.Time) (map[string]BillRecord, error) {
	apiPath := fmt.Sprintf("%s?bill_date=%s&tar_type=GZIP",
		refundBillAPIPath, billDate.Format("2006-01-02"))
	return c.fetchAndParseBill(ctx, apiPath, "商户退款单号", "微信退款单号", "退款金额")
}

// DownloadEcommerceTradeBill 下载指定日期的电商收付通合单交易账单
// 对应 combined_payment_orders WHERE status IN ('paid', 'refunded')
// 注：列名基于微信支付 v3 电商账单格式，如与实际不符请参照真实账单调整
func (c *EcommerceClient) DownloadEcommerceTradeBill(ctx context.Context, billDate time.Time) (map[string]BillRecord, error) {
	apiPath := fmt.Sprintf("%s?bill_date=%s&account_type=ALL&tar_type=GZIP",
		ecommerceTradeBillAPIPath, billDate.Format("2006-01-02"))
	return c.fetchAndParseBill(ctx, apiPath, "合单商户单号", "合单微信单号", "合单应结订单金额")
}

// DownloadEcommerceRefundBill 下载指定日期的电商收付通退款账单
// 对应通过 /v3/ecommerce/refunds/apply 产生的退款记录（区别于直连退款账单）
// 注：列名基于微信支付 v3 电商退款账单格式，如与实际不符请参照真实账单调整
func (c *EcommerceClient) DownloadEcommerceRefundBill(ctx context.Context, billDate time.Time) (map[string]BillRecord, error) {
	apiPath := fmt.Sprintf("%s?bill_date=%s&account_type=ALL&tar_type=GZIP",
		ecommerceRefundBillAPIPath, billDate.Format("2006-01-02"))
	return c.fetchAndParseBill(ctx, apiPath, "商户退款单号", "微信退款单号", "退款金额")
}

// GetProfitSharingBillDownloadURL 获取分账账单下载地址。
func (c *EcommerceClient) GetProfitSharingBillDownloadURL(ctx context.Context, billDate time.Time, subMchID, tarType string) (*BillDownloadURLResponse, error) {
	params := url.Values{}
	params.Set("bill_date", billDate.Format("2006-01-02"))
	if subMchID != "" {
		params.Set("sub_mchid", subMchID)
	}
	if tarType != "" {
		params.Set("tar_type", tarType)
	}

	return c.getBillDownloadURL(ctx, buildBillRequestPath(profitSharingBillAPIPath, params))
}

// fetchAndParseBill 通用账单获取与解析流程：
// 1. 调 API 获取下载 URL
// 2. 下载 gzip 账单文件（带签名认证）
// 3. 解压并解析 CSV，按 outTradeNoCol 列构建返回 map
func (c *PaymentClient) fetchAndParseBill(
	ctx context.Context,
	apiPath string,
	outTradeNoCol, transactionIDCol, amountCol string,
) (map[string]BillRecord, error) {
	// 第一步：获取账单下载 URL
	urlResp, err := c.getBillDownloadURL(ctx, apiPath)
	if err != nil {
		return nil, err
	}

	// 第三步：下载账单文件。文件下载接口不返回响应签名，需跳过响应验签。
	fileBytes, err := c.DownloadBillFile(ctx, urlResp.DownloadURL)
	if err != nil {
		return nil, fmt.Errorf("download bill file: %w", err)
	}
	if err := verifyBillHash(fileBytes, urlResp.HashType, urlResp.HashValue); err != nil {
		return nil, fmt.Errorf("verify bill hash: %w", err)
	}

	// 第四步：解压并解析 CSV
	return parseBillGzip(fileBytes, outTradeNoCol, transactionIDCol, amountCol)
}

func (c *PaymentClient) getBillDownloadURL(ctx context.Context, apiPath string) (*BillDownloadURLResponse, error) {
	respBytes, err := c.doRequest(ctx, http.MethodGet, apiPath, nil)
	if err != nil {
		return nil, fmt.Errorf("get bill download url: %w", normalizeBillDownloadURLError(err))
	}

	var urlResp BillDownloadURLResponse
	if err := json.Unmarshal(respBytes, &urlResp); err != nil {
		return nil, fmt.Errorf("parse bill download url response: %w", err)
	}
	if strings.TrimSpace(urlResp.DownloadURL) == "" {
		return nil, fmt.Errorf("empty download_url in response")
	}

	return &urlResp, nil
}

// DownloadBillFile 下载账单文件原始字节。
func (c *PaymentClient) DownloadBillFile(ctx context.Context, downloadURL string) ([]byte, error) {
	if strings.TrimSpace(downloadURL) == "" {
		return nil, fmt.Errorf("download url is required")
	}

	return c.doRequestWithoutResponseVerification(ctx, http.MethodGet, downloadURL, nil)
}

func buildBillRequestPath(basePath string, params url.Values) string {
	if len(params) == 0 {
		return basePath
	}

	return basePath + "?" + params.Encode()
}

func verifyBillHash(fileBytes []byte, hashType, hashValue string) error {
	hashType = strings.ToUpper(strings.TrimSpace(hashType))
	hashValue = strings.TrimSpace(hashValue)
	if hashType == "" {
		return fmt.Errorf("missing hash_type in bill download response")
	}
	if hashValue == "" {
		return fmt.Errorf("missing hash_value in bill download response")
	}

	var actual string
	switch hashType {
	case "SHA1":
		sum := sha1.Sum(fileBytes)
		actual = fmt.Sprintf("%x", sum)
	case "SHA256":
		sum := sha256.Sum256(fileBytes)
		actual = fmt.Sprintf("%x", sum)
	default:
		return fmt.Errorf("unsupported bill hash type: %s", hashType)
	}

	if !strings.EqualFold(actual, hashValue) {
		return fmt.Errorf("bill hash mismatch: got %s want %s", actual, hashValue)
	}

	return nil
}

func normalizeBillDownloadURLError(err error) error {
	if err == nil {
		return nil
	}

	var wxErr *WechatPayError
	if errors.As(err, &wxErr) {
		switch {
		case wechaterrorcodes.FundManagementCodeEquals(wxErr.Code, wechaterrorcodes.FundManagementCodeStatementCreating):
			return &billDownloadStateError{Kind: ErrBillNotReady, Cause: err}
		case wxErr.StatusCode == http.StatusNotFound || wechaterrorcodes.FundManagementCodeEquals(wxErr.Code, wechaterrorcodes.FundManagementCodeNoStatementExist):
			return &billDownloadStateError{Kind: ErrBillNotFound, Cause: err}
		}
	}

	if strings.Contains(err.Error(), "wechat pay api error: status=404") {
		return &billDownloadStateError{Kind: ErrBillNotFound, Cause: err}
	}

	return err
}

// parseBillGzip 解压 gzip 后调用 CSV 解析器
func parseBillGzip(gzipData []byte, outTradeNoCol, transactionIDCol, amountCol string) (map[string]BillRecord, error) {
	gr, err := gzip.NewReader(bytes.NewReader(gzipData))
	if err != nil {
		return nil, fmt.Errorf("create gzip reader: %w", err)
	}
	defer gr.Close()

	raw, err := io.ReadAll(gr)
	if err != nil {
		return nil, fmt.Errorf("decompress bill: %w", err)
	}
	return parseBillCSV(raw, outTradeNoCol, transactionIDCol, amountCol)
}

// parseBillCSV 解析微信账单 CSV
// 微信账单格式：字段值用反引号（`）包裹，逗号分隔；尾部汇总行不含反引号，自动跳过
func parseBillCSV(data []byte, outTradeNoCol, transactionIDCol, amountCol string) (map[string]BillRecord, error) {
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")

	// 找到第一行含反引号的行作为表头
	headerIdx := -1
	for i, line := range lines {
		if strings.Contains(line, "`") {
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 {
		return nil, fmt.Errorf("no header row found in bill CSV (no backtick-wrapped fields)")
	}

	headers := parseBillRow(lines[headerIdx])
	colIndex := make(map[string]int, len(headers))
	for i, h := range headers {
		colIndex[h] = i
	}

	outIdx, ok1 := colIndex[outTradeNoCol]
	txIdx, ok2 := colIndex[transactionIDCol]
	amtIdx, ok3 := colIndex[amountCol]
	if !ok1 || !ok2 || !ok3 {
		return nil, fmt.Errorf(
			"required columns not found in bill: %s(found=%v) %s(found=%v) %s(found=%v); available columns: %v",
			outTradeNoCol, ok1, transactionIDCol, ok2, amountCol, ok3, headers,
		)
	}

	result := make(map[string]BillRecord)
	for _, line := range lines[headerIdx+1:] {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "`") {
			// 空行或汇总行（汇总行不含反引号，如"总金额,总笔数,..."）
			continue
		}
		fields := parseBillRow(line)
		if len(fields) <= outIdx || len(fields) <= txIdx || len(fields) <= amtIdx {
			continue
		}
		outTradeNo := fields[outIdx]
		txID := fields[txIdx]
		amtStr := fields[amtIdx]
		if outTradeNo == "" {
			continue
		}
		// 微信账单金额单位为元（如 "100.00"），转换为分
		amtYuan, err := strconv.ParseFloat(amtStr, 64)
		if err != nil {
			continue
		}
		result[outTradeNo] = BillRecord{
			OutTradeNo:    outTradeNo,
			TransactionID: txID,
			Amount:        int64(math.Round(amtYuan * 100)),
		}
	}
	return result, nil
}

// parseBillRow 将账单行拆分为字段列表，去掉每个字段的反引号包裹
// 例："`value1`,`value2`" → ["value1", "value2"]
func parseBillRow(line string) []string {
	parts := strings.Split(line, ",")
	fields := make([]string, 0, len(parts))
	for _, p := range parts {
		fields = append(fields, strings.Trim(strings.TrimSpace(p), "`"))
	}
	return fields
}
