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
	payclient "github.com/merrydance/locallife/baofu/aggregatepay"
	paycontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
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

	outTradeNo := strings.TrimSpace(os.Getenv("BAOFU_TEST_OUT_TRADE_NO"))
	tradeNo := strings.TrimSpace(os.Getenv("BAOFU_TEST_TRADE_NO"))
	if outTradeNo == "" && tradeNo == "" {
		return fmt.Errorf("BAOFU_TEST_OUT_TRADE_NO or BAOFU_TEST_TRADE_NO is required")
	}

	fmt.Printf("baofu env=%s aggregate=%s collect=%s/%s\n", baofuCfg.Environment, baofuCfg.AggregatePayBaseURL, baofuCfg.CollectMerchantID, baofuCfg.CollectTerminalID)
	fmt.Printf("order_query_request out_trade_no=%s trade_no=%s\n", mask(outTradeNo), mask(tradeNo))
	if baofuCfg.Environment == baofu.BaofuEnvironmentSandbox {
		fmt.Println("sandbox_note: Baofoo confirmed sandbox does not support real payment; order_query can prove query parsing/status mapping, not callback or downstream settlement.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := payclient.NewClient(root)
	result, err := client.QueryPayment(ctx, paycontracts.PaymentQueryRequest{
		MerchantID: baofuCfg.CollectMerchantID,
		TerminalID: baofuCfg.CollectTerminalID,
		OutTradeNo: outTradeNo,
		TradeNo:    tradeNo,
	})
	if err != nil {
		printProviderError("order_query", err)
		return err
	}
	fmt.Printf("order_query: success out_trade_no=%s trade_no=%s txn_state=%s normalized_state=%s result_code=%s err_code=%s wc_pay_data=%v\n",
		mask(result.OutTradeNo),
		mask(result.TradeNo),
		result.TxnState,
		paycontracts.NormalizePaymentTerminalStatus(result.TxnState),
		result.ResultCode,
		result.ErrorCode,
		len(result.ChannelReturn.WechatPayData) > 0,
	)
	return nil
}

func printProviderError(name string, err error) {
	var providerErr *baofu.ProviderError
	if errors.As(err, &providerErr) {
		cause := errors.Unwrap(providerErr)
		causeText := "-"
		if cause != nil {
			causeText = cause.Error()
		}
		fmt.Printf("%s: provider_error operation=%s status=%d upstream_code=%s frontend_code=%s retryable=%v cause=%q\n", name, providerErr.Operation, providerErr.StatusCode, providerErr.UpstreamCode, providerErr.Frontend.Code, providerErr.Frontend.Retryable, causeText)
		return
	}
	fmt.Printf("%s: local_error=%v\n", name, err)
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
