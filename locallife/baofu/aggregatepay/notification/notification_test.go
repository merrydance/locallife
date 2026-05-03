package notification

import (
	"testing"

	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	"github.com/stretchr/testify/require"
)

func TestParserParsePaymentNotificationNormalizesPaymentFact(t *testing.T) {
	body := []byte(`{
		"notifyId":"BFN202605030001",
		"notifyType":"PAYMENT.SUCCESS",
		"outTradeNo":"PO202605030001",
		"tradeNo":"BFPAY202605030001",
		"txnState":"SUCCESS",
		"succAmt":12345,
		"feeAmt":37,
		"sub_openid":"payer-openid-must-not-be-receiver",
		"occurredAt":"2026-05-03T10:15:00Z"
	}`)
	parser := NewParser()

	notification, err := parser.ParsePaymentNotification(body)

	require.NoError(t, err)
	require.Equal(t, "BFN202605030001", notification.NotifyID)
	require.Equal(t, "PAYMENT.SUCCESS", notification.NotifyType)
	require.Equal(t, "PO202605030001", notification.Fact.OutTradeNo)
	require.Equal(t, "BFPAY202605030001", notification.Fact.TradeNo)
	require.Equal(t, aggregatecontracts.PaymentStateSuccess, notification.Fact.TransactionState)
	require.Equal(t, int64(12345), notification.Fact.SuccessAmountFen)
	require.Equal(t, int64(37), notification.Fact.FeeAmountFen)
	require.Equal(t, "success", notification.TerminalStatus)
	require.True(t, notification.IsTerminal)
	require.Equal(t, "2026-05-03T10:15:00Z", notification.OccurredAt.UTC().Format("2006-01-02T15:04:05Z"))
	require.JSONEq(t, string(body), string(notification.Raw))
	require.NotContains(t, string(notification.Raw), "sharingMerId")
}

func TestParserParsePaymentNotificationRequiresOutTradeNo(t *testing.T) {
	parser := NewParser()

	_, err := parser.ParsePaymentNotification([]byte(`{"notifyId":"BFN1","txnState":"SUCCESS"}`))

	require.ErrorIs(t, err, ErrPaymentNotificationOutTradeNoRequired)
}
