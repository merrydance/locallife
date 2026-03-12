package wechat

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	tradeBillAPIPath          = "/v3/bill/tradebill"
	refundBillAPIPath         = "/v3/bill/refundbill"
	ecommerceTradeBillAPIPath = "/v3/ecommerce/bill/tradebill"
	wxPayBillAPIBase          = "https://api.mch.weixin.qq.com"
)

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
}

// DownloadTradeBill 下载指定日期的小程序直连支付交易账单
// 对应 payment_orders WHERE payment_type = 'miniprogram'
func (c *PaymentClient) DownloadTradeBill(ctx context.Context, billDate time.Time) (map[string]BillRecord, error) {
	apiPath := fmt.Sprintf("%s?bill_date=%s&bill_type=ALL&tar_type=GZIP",
		tradeBillAPIPath, billDate.Format("2006-01-02"))
	return c.fetchAndParseBill(ctx, apiPath, "商户订单号", "微信订单号", "订单金额")
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
	respBytes, err := c.doRequest(ctx, http.MethodGet, apiPath, nil)
	if err != nil {
		return nil, fmt.Errorf("get bill download url: %w", err)
	}
	var urlResp BillDownloadURLResponse
	if err := json.Unmarshal(respBytes, &urlResp); err != nil {
		return nil, fmt.Errorf("parse bill download url response: %w", err)
	}
	if urlResp.DownloadURL == "" {
		return nil, fmt.Errorf("empty download_url in response")
	}

	// 第二步：从完整 URL 中提取路径（去掉 base），用于生成请求签名
	billPath := strings.TrimPrefix(urlResp.DownloadURL, wxPayBillAPIBase)
	if billPath == urlResp.DownloadURL {
		return nil, fmt.Errorf("unexpected download url format (expected %s prefix): %s",
			wxPayBillAPIBase, urlResp.DownloadURL)
	}

	// 第三步：下载账单文件（带微信支付签名认证，返回 gzip 原始字节）
	fileBytes, err := c.doRequest(ctx, http.MethodGet, billPath, nil)
	if err != nil {
		return nil, fmt.Errorf("download bill file: %w", err)
	}

	// 第四步：解压并解析 CSV
	return parseBillGzip(fileBytes, outTradeNoCol, transactionIDCol, amountCol)
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
