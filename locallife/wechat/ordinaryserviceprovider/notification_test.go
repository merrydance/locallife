package ordinaryserviceprovider

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
)

func TestBuildNotificationEnvelopePreservesMetadataAndRedactsPlaintextFromLogFields(t *testing.T) {
	createTime := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	notifyRequest := &notify.Request{
		ID:           "notify-001",
		CreateTime:   &createTime,
		EventType:    "TRANSACTION.SUCCESS",
		ResourceType: "encrypt-resource",
		Summary:      "支付成功",
		Resource: &notify.EncryptedResource{
			OriginalType: "transaction",
			Plaintext:    `{"out_trade_no":"order-001","openid":"sensitive"}`,
		},
	}
	headers := http.Header{
		"Wechatpay-Serial":         []string{"serial-001"},
		"Wechatpay-Signature":      []string{"signature-001"},
		"Wechatpay-Timestamp":      []string{"1777603200"},
		"Wechatpay-Nonce":          []string{"nonce-001"},
		"Wechatpay-Signature-Type": []string{"WECHATPAY2-SHA256-RSA2048"},
	}

	envelope := buildNotificationEnvelope(notifyRequest, map[string]any{"out_trade_no": "order-001"}, headers)

	require.Equal(t, "notify-001", envelope.ID)
	require.Equal(t, "TRANSACTION.SUCCESS", envelope.EventType)
	require.Equal(t, "transaction", envelope.OriginalType)
	require.Equal(t, `{"out_trade_no":"order-001","openid":"sensitive"}`, envelope.Plaintext)
	require.Equal(t, "serial-001", envelope.Headers.Serial)
	require.Equal(t, "signature-001", envelope.Headers.Signature)
	require.Equal(t, "order-001", envelope.Decoded["out_trade_no"])

	logFields := envelope.LogFields()
	require.Equal(t, "notify-001", logFields["notification_id"])
	require.Equal(t, "TRANSACTION.SUCCESS", logFields["event_type"])
	require.NotContains(t, logFields, "plaintext")
	require.NotContains(t, logFields, "decoded")
}
