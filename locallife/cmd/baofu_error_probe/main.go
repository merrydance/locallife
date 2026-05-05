package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/merrydance/locallife/baofu"
	accountclient "github.com/merrydance/locallife/baofu/account"
	accountcontracts "github.com/merrydance/locallife/baofu/account/contracts"
	payclient "github.com/merrydance/locallife/baofu/aggregatepay"
	paycontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	reportclient "github.com/merrydance/locallife/baofu/merchantreport"
	reportcontracts "github.com/merrydance/locallife/baofu/merchantreport/contracts"
	"github.com/merrydance/locallife/util"
)

func main() {
	if err := run(); err != nil {
		fmt.Printf("fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := util.LoadConfig(".")
	if err != nil {
		return fmt.Errorf("load app.env: %w", err)
	}
	if err := cfg.ValidateBaofuConfig(); err != nil {
		return fmt.Errorf("validate baofu config: %w", err)
	}
	root, err := baofu.NewClient(cfg.ToBaofuConfig(), http.DefaultClient)
	if err != nil {
		return fmt.Errorf("create baofu client: %w", err)
	}
	baofuCfg := root.Config()
	fmt.Printf("baofu env=%s collect=%s/%s payout=%s/%s\n", baofuCfg.Environment, baofuCfg.CollectMerchantID, baofuCfg.CollectTerminalID, baofuCfg.PayoutMerchantID, baofuCfg.PayoutTerminalID)
	fmt.Printf("endpoints account=%s aggregate=%s merchant_report=%s\n", baofuCfg.AccountGatewayBaseURL, baofuCfg.AggregatePayBaseURL, baofuCfg.MerchantReportBaseURL)
	fmt.Println("probe_note: safe fake identifiers only; this command proves provider classification/frontend guidance and must not perform real funds actions.")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	now := baofuErrorProbeNow()
	accountClient := accountclient.NewClient(root)
	aggregateClient := payclient.NewClient(root)
	merchantReportClient := reportclient.NewClient(root)

	fakeLoginNo := firstNonEmpty(os.Getenv("BAOFU_TEST_FAKE_LOGIN_NO"), "BAOFU_ERR_ACC_"+now)
	accountResult, err := accountClient.QueryAccount(ctx, accountcontracts.QueryAccountRequest{
		OutRequestNo: fakeLoginNo,
		AccountType:  firstNonEmpty(os.Getenv("BAOFU_TEST_ACCOUNT_TYPE"), "personal"),
	})
	printResult("account_query_fake", summarizeAccountResult(accountResult), err)

	fakeReportNo := firstNonEmpty(os.Getenv("BAOFU_TEST_FAKE_REPORT_NO"), "MR_ERR_"+now)
	reportResult, err := merchantReportClient.QueryReport(ctx, reportcontracts.MerchantReportQueryRequest{
		MerchantID: baofuCfg.CollectMerchantID,
		TerminalID: baofuCfg.CollectTerminalID,
		ReportType: reportcontracts.ReportTypeWechat,
		ReportNo:   fakeReportNo,
	})
	printResult("merchant_report_query_fake", summarizeReportResult(reportResult), err)

	fakeOrderOutTradeNo := firstNonEmpty(os.Getenv("BAOFU_TEST_FAKE_OUT_TRADE_NO"), "BAOFU_ERR_ORDER_"+now)
	orderResult, err := aggregateClient.QueryPayment(ctx, paycontracts.PaymentQueryRequest{
		MerchantID: baofuCfg.CollectMerchantID,
		TerminalID: baofuCfg.CollectTerminalID,
		OutTradeNo: fakeOrderOutTradeNo,
	})
	printResult("order_query_fake", summarizeOrderResult(orderResult), err)

	return nil
}

func printResult(name string, summary string, err error) {
	if err == nil {
		fmt.Printf("%s: success %s\n", name, summary)
		return
	}
	var providerErr *baofu.ProviderError
	if errors.As(err, &providerErr) {
		fmt.Printf("%s: provider_error operation=%s status=%d upstream_code=%s frontend_code=%s frontend_message=%q frontend_action=%s retryable=%v\n",
			name,
			providerErr.Operation,
			providerErr.StatusCode,
			providerErr.UpstreamCode,
			providerErr.Frontend.Code,
			providerErr.Frontend.Message,
			providerErr.Frontend.Action,
			providerErr.Frontend.Retryable,
		)
		return
	}
	fmt.Printf("%s: local_error=%v\n", name, err)
}

func summarizeAccountResult(result *accountcontracts.AccountResult) string {
	if result == nil {
		return "result=nil"
	}
	return fmt.Sprintf("login_no=%s state=%s open_state=%s contract=%s sharing_mer_id=%s fail_code=%s",
		mask(result.OutRequestNo),
		result.UpstreamState,
		result.OpenState,
		mask(result.ContractNo),
		mask(result.SharingMerID),
		result.FailCode,
	)
}

func summarizeReportResult(result *reportcontracts.MerchantReportResult) string {
	if result == nil {
		return "result=nil"
	}
	return fmt.Sprintf("report_no=%s state=%s normalized_state=%s sub_mch_id=%s result_code=%s err_code=%s",
		mask(result.ReportNo),
		result.ReportState,
		result.NormalizedReportState(),
		mask(result.SubMchID),
		result.ResultCode,
		result.ErrorCode,
	)
}

func summarizeOrderResult(result *paycontracts.UnifiedOrderResult) string {
	if result == nil {
		return "result=nil"
	}
	return fmt.Sprintf("out_trade_no=%s trade_no=%s txn_state=%s normalized_state=%s result_code=%s err_code=%s",
		mask(result.OutTradeNo),
		mask(result.TradeNo),
		result.TxnState,
		paycontracts.NormalizePaymentTerminalStatus(result.TxnState),
		result.ResultCode,
		result.ErrorCode,
	)
}

func baofuErrorProbeNow() string {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.Local
	}
	return time.Now().In(loc).Format("20060102150405")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func mask(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	if len(value) <= 8 {
		return value
	}
	return value[:4] + "***" + value[len(value)-4:]
}
