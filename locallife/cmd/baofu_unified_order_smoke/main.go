package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
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

	amountFen := int64(100)
	if raw := strings.TrimSpace(os.Getenv("BAOFU_TEST_AMOUNT_FEN")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			return fmt.Errorf("BAOFU_TEST_AMOUNT_FEN must be a positive integer")
		}
		amountFen = parsed
	}
	subOpenID := strings.TrimSpace(os.Getenv("BAOFU_TEST_SUB_OPENID"))
	if subOpenID == "" {
		return fmt.Errorf("BAOFU_TEST_SUB_OPENID is required")
	}
	subAppID := firstNonEmpty(cfg.WechatMiniAppID, baofuCfg.AppID)
	if subAppID == "" {
		return fmt.Errorf("WECHAT_MINI_APP_ID is required")
	}
	clientIP := firstNonEmpty(os.Getenv("BAOFU_TEST_CLIENT_IP"), "127.0.0.1")
	subMchID := strings.TrimSpace(os.Getenv("BAOFU_TEST_SUB_MCH_ID"))
	if baofuCfg.Environment == baofu.BaofuEnvironmentProduction && subMchID == "" {
		return fmt.Errorf("BAOFU_TEST_SUB_MCH_ID is required in production")
	}
	now := baofuSmokeNow()
	outTradeNo := firstNonEmpty(os.Getenv("BAOFU_TEST_OUT_TRADE_NO"), "BAOFU_UO_"+now)

	fmt.Printf("baofu env=%s aggregate=%s collect=%s/%s\n", baofuCfg.Environment, baofuCfg.AggregatePayBaseURL, baofuCfg.CollectMerchantID, baofuCfg.CollectTerminalID)
	fmt.Printf("unified_order_request out_trade_no=%s requested_sub_mch_id=%s effective_wire_sub_mch_id=%s sub_appid=%s amount_fen=%d notify=%s client_ip=%s\n",
		outTradeNo,
		mask(subMchID),
		effectiveSubMchForPrint(baofuCfg.Environment, subMchID),
		mask(subAppID),
		amountFen,
		cfg.EffectiveBaofuPaymentNotifyURL(),
		clientIP,
	)
	if baofuCfg.Environment == baofu.BaofuEnvironmentSandbox {
		fmt.Println("sandbox_note: Baofoo confirmed sandbox must omit subMchId and does not support real payment; wc_pay_data only proves payload parsing, not real payment/callback success.")
	}

	client := payclient.NewClient(root)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := client.CreateUnifiedOrder(ctx, paycontracts.NewWechatJSAPISharingUnifiedOrderRequest(paycontracts.UnifiedOrderInput{
		MerchantID: baofuCfg.CollectMerchantID,
		TerminalID: baofuCfg.CollectTerminalID,
		OutTradeNo: outTradeNo,
		AmountFen:  amountFen,
		TxnTime:    now,
		TimeExpire: 30,
		SubMchID:   subMchID,
		SubAppID:   subAppID,
		SubOpenID:  subOpenID,
		Body:       "LocalLife Baofoo sandbox smoke",
		NotifyURL:  cfg.EffectiveBaofuPaymentNotifyURL(),
		ClientIP:   clientIP,
	}))
	if err != nil {
		printProviderError("unified_order", err)
		return err
	}
	fmt.Printf("unified_order: success out_trade_no=%s trade_no=%s txn_state=%s result_code=%s err_code=%s wc_pay_data=%v\n",
		mask(result.OutTradeNo), mask(result.TradeNo), result.TxnState, result.ResultCode, result.ErrorCode, len(result.ChannelReturn.WechatPayData) > 0)
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

func baofuSmokeNow() string {
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

func effectiveSubMchForPrint(environment string, subMchID string) string {
	if environment == baofu.BaofuEnvironmentSandbox {
		return "omitted_by_client"
	}
	return mask(subMchID)
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
