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
	accountclient "github.com/merrydance/locallife/baofu/account"
	accountcontracts "github.com/merrydance/locallife/baofu/account/contracts"
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
	fmt.Printf("baofu env=%s collect=%s/%s payout=%s/%s\n", baofuCfg.Environment, baofuCfg.CollectMerchantID, baofuCfg.CollectTerminalID, baofuCfg.PayoutMerchantID, baofuCfg.PayoutTerminalID)
	fmt.Printf("endpoints account=%s aggregate=%s merchant_report=%s\n", baofuCfg.AccountGatewayBaseURL, baofuCfg.AggregatePayBaseURL, baofuCfg.MerchantReportBaseURL)
	fmt.Println("sandbox_note: Baofoo confirmed sandbox does not support real payment; fake probes prove request shape/parsing/provider classification, not real callbacks or settlement.")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	aggregateClient := payclient.NewClient(root)
	accountClient := accountclient.NewClient(root)
	now := baofuProbeNow()
	fakePaymentOutTradeNo := "BAOFU_PROBE_PAY_" + now
	fakeShareOutTradeNo := "BAOFU_PROBE_SH_" + now
	fakeRefundOutTradeNo := "BAOFU_PROBE_RF_" + now
	fakeCloseOutTradeNo := "BAOFU_PROBE_OC_" + now
	fakeWithdrawSerialNo := "BAOFU_PROBE_WD_" + now
	accountType := envDefault("BAOFU_TEST_ACCOUNT_TYPE", "personal")

	if contractNo := strings.TrimSpace(os.Getenv("BAOFU_TEST_CONTRACT_NO")); contractNo != "" {
		result, err := accountClient.QueryBalance(ctx, accountcontracts.BalanceQueryRequest{
			MerchantID:  baofuCfg.CollectMerchantID,
			TerminalID:  baofuCfg.CollectTerminalID,
			ContractNo:  contractNo,
			AccountType: accountType,
		})
		printResult("account_balance", result, err)
	} else {
		fmt.Println("account_balance: skipped; set BAOFU_TEST_CONTRACT_NO to enable")
	}

	shareQueryResult, err := aggregateClient.QueryProfitSharing(ctx, paycontracts.ShareQueryRequest{
		MerchantID: baofuCfg.CollectMerchantID,
		TerminalID: baofuCfg.CollectTerminalID,
		OutTradeNo: fakeShareOutTradeNo,
	})
	printResult("share_query_fake", shareQueryResult, err)

	refundQueryResult, err := aggregateClient.QueryRefund(ctx, paycontracts.RefundQueryRequest{
		MerchantID: baofuCfg.CollectMerchantID,
		TerminalID: baofuCfg.CollectTerminalID,
		OutTradeNo: fakeRefundOutTradeNo,
	})
	printResult("refund_query_fake", refundQueryResult, err)

	closeResult, err := aggregateClient.CloseOrder(ctx, paycontracts.OrderCloseRequest{
		MerchantID: baofuCfg.CollectMerchantID,
		TerminalID: baofuCfg.CollectTerminalID,
		OutTradeNo: fakeCloseOutTradeNo,
	})
	printResult("order_close_fake", closeResult, err)

	refundResult, err := aggregateClient.CreateRefund(ctx, paycontracts.RefundBeforeShareRequest{
		MerchantID:       baofuCfg.CollectMerchantID,
		TerminalID:       baofuCfg.CollectTerminalID,
		OriginOutTradeNo: fakePaymentOutTradeNo,
		OutTradeNo:       fakeRefundOutTradeNo,
		NotifyURL:        cfg.EffectiveBaofuRefundNotifyURL(),
		RefundAmountFen:  1,
		TotalAmountFen:   1,
		TransactionTime:  now,
		RefundReason:     "sandbox fake payment probe",
	})
	printResult("order_refund_fake", refundResult, err)

	if sharingMerID := strings.TrimSpace(os.Getenv("BAOFU_TEST_CONTRACT_NO")); sharingMerID != "" {
		shareResult, err := aggregateClient.CreateProfitSharing(ctx, paycontracts.ShareAfterPayRequest{
			MerchantID:       baofuCfg.CollectMerchantID,
			TerminalID:       baofuCfg.CollectTerminalID,
			OriginOutTradeNo: fakePaymentOutTradeNo,
			OutTradeNo:       fakeShareOutTradeNo,
			TxnTime:          now,
			NotifyURL:        cfg.EffectiveBaofuProfitSharingNotifyURL(),
			SharingDetails: []paycontracts.SharingDetail{{
				SharingMerID:     sharingMerID,
				SharingAmountFen: 1,
			}},
		})
		printResult("share_after_pay_fake", shareResult, err)
	} else {
		fmt.Println("share_after_pay_fake: skipped; set BAOFU_TEST_CONTRACT_NO to use as fake receiver sharingMerId")
	}

	withdrawQueryResult, err := accountClient.QueryWithdraw(ctx, accountcontracts.WithdrawQueryRequest{
		MerchantID:    baofuCfg.PayoutMerchantID,
		TerminalID:    baofuCfg.PayoutTerminalID,
		TransSerialNo: envDefault("BAOFU_TEST_WITHDRAW_TRANS_SERIAL_NO", fakeWithdrawSerialNo),
	})
	printResult("withdraw_query_fake", withdrawQueryResult, err)

	if strings.EqualFold(strings.TrimSpace(os.Getenv("BAOFU_RUN_WITHDRAW")), "true") {
		amountFen, parseErr := strconv.ParseInt(strings.TrimSpace(os.Getenv("BAOFU_TEST_WITHDRAW_AMOUNT_FEN")), 10, 64)
		if parseErr != nil || amountFen <= 0 {
			fmt.Println("withdraw_real: skipped; BAOFU_TEST_WITHDRAW_AMOUNT_FEN must be a positive integer when BAOFU_RUN_WITHDRAW=true")
		} else if contractNo := strings.TrimSpace(os.Getenv("BAOFU_TEST_CONTRACT_NO")); contractNo == "" {
			fmt.Println("withdraw_real: skipped; BAOFU_TEST_CONTRACT_NO is required when BAOFU_RUN_WITHDRAW=true")
		} else {
			withdrawResult, err := accountClient.CreateWithdraw(ctx, accountcontracts.WithdrawRequest{
				MerchantID:    baofuCfg.PayoutMerchantID,
				TerminalID:    baofuCfg.PayoutTerminalID,
				ContractNo:    strings.TrimSpace(os.Getenv("BAOFU_TEST_CONTRACT_NO")),
				TransSerialNo: envDefault("BAOFU_TEST_WITHDRAW_TRANS_SERIAL_NO", fakeWithdrawSerialNo),
				AmountFen:     amountFen,
				NotifyURL:     strings.TrimRight(baofuCfg.NotifyBaseURL, "/") + "/account/withdraw",
			})
			printResult("withdraw_real", withdrawResult, err)
		}
	} else {
		fmt.Println("withdraw_real: skipped; set BAOFU_RUN_WITHDRAW=true plus BAOFU_TEST_WITHDRAW_AMOUNT_FEN to perform a real funds action")
	}

	return nil
}

func printResult(name string, result any, err error) {
	if err == nil {
		fmt.Printf("%s: success %s\n", name, summarizeResult(result))
		return
	}
	var providerErr *baofu.ProviderError
	if errors.As(err, &providerErr) {
		fmt.Printf("%s: provider_error operation=%s status=%d upstream_code=%s frontend_code=%s retryable=%v\n", name, providerErr.Operation, providerErr.StatusCode, providerErr.UpstreamCode, providerErr.Frontend.Code, providerErr.Frontend.Retryable)
		return
	}
	fmt.Printf("%s: local_error=%v\n", name, err)
}

func summarizeResult(result any) string {
	switch v := result.(type) {
	case *accountcontracts.BalanceResult:
		if v == nil {
			return "result=nil"
		}
		return fmt.Sprintf("contract=%s available_fen=%d pending_fen=%d ledger_fen=%d frozen_fen=%d", mask(v.ContractNo), v.AvailableAmountFen, v.PendingAmountFen, v.LedgerAmountFen, v.FrozenAmountFen)
	case *accountcontracts.WithdrawResult:
		if v == nil {
			return "result=nil"
		}
		return fmt.Sprintf("trans_serial_no=%s withdraw_no=%s state=%s status=%s", mask(v.TransSerialNo), mask(v.BaofuWithdrawNo), v.UpstreamState, v.Status)
	case *paycontracts.ShareResult:
		if v == nil {
			return "result=nil"
		}
		return fmt.Sprintf("out_trade_no=%s trade_no=%s txn_state=%s result_code=%s err_code=%s", mask(v.OutTradeNo), mask(v.TradeNo), v.TxnState, v.ResultCode, v.ErrorCode)
	case *paycontracts.RefundResult:
		if v == nil {
			return "result=nil"
		}
		return fmt.Sprintf("out_trade_no=%s trade_no=%s refund_state=%s result_code=%s err_code=%s", mask(v.OutTradeNo), mask(v.TradeNo), v.RefundState, v.ResultCode, v.ErrorCode)
	case *paycontracts.OrderCloseResult:
		if v == nil {
			return "result=nil"
		}
		return fmt.Sprintf("out_trade_no=%s trade_no=%s result_code=%s err_code=%s", mask(v.OutTradeNo), mask(v.TradeNo), v.ResultCode, v.ErrorCode)
	default:
		return fmt.Sprintf("result_type=%T", result)
	}
}

func baofuProbeNow() string {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.Local
	}
	return time.Now().In(loc).Format("20060102150405")
}

func envDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
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
