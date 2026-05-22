package worker

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
)

func TestRefundOrderAlertExtra_IncludesCommonIdentifiers(t *testing.T) {
	paymentOrder := db.PaymentOrder{
		ID:             11,
		OrderID:        pgtype.Int8{Int64: 22, Valid: true},
		UserID:         33,
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		BusinessType:   "takeout_order",
		Amount:         4567,
		OutTradeNo:     "OT123",
		TransactionID:  pgtype.Text{String: "WX123", Valid: true},
	}
	refundOrder := db.RefundOrder{
		ID:             44,
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "user_cancel",
		RefundAmount:   1200,
		OutRefundNo:    "RF123",
		RefundID:       pgtype.Text{String: "WR123", Valid: true},
	}

	extra := refundOrderAlertExtra(paymentOrder, refundOrder, 55, map[string]interface{}{"wechat_status": "ABNORMAL"})

	require.EqualValues(t, paymentOrder.ID, extra["payment_order_id"])
	require.EqualValues(t, int64(22), extra["order_id"])
	require.EqualValues(t, int64(55), extra["merchant_id"])
	require.Equal(t, "OT123", extra["out_trade_no"])
	require.Equal(t, "WX123", extra["transaction_id"])
	require.EqualValues(t, refundOrder.ID, extra["refund_order_id"])
	require.Equal(t, "RF123", extra["out_refund_no"])
	require.Equal(t, "WR123", extra["refund_id"])
	require.Equal(t, "ABNORMAL", extra["wechat_status"])
}

func TestBaofuAlertExtraIncludesProviderChannelAndSanitizesSensitiveFields(t *testing.T) {
	extra := baofuReconciliationAlertExtra(map[string]interface{}{
		"payment_order_id": 11,
		"provider":         "bad-provider",
		"channel":          "bad-channel",
		"contract_no":      "CONTRACT-SECRET",
		"sharing_mer_id":   "SHARE-SECRET",
		"sharingMerId":     "SHARE-SECRET",
		"raw_resource":     map[string]any{"id_card": "secret"},
		"signature":        "SIGN",
	})

	require.Equal(t, db.ExternalPaymentProviderBaofu, extra["provider"])
	require.Equal(t, db.PaymentChannelBaofuAggregate, extra["channel"])
	require.EqualValues(t, int64(11), extra["payment_order_id"])
	require.NotContains(t, extra, "contract_no")
	require.NotContains(t, extra, "sharing_mer_id")
	require.NotContains(t, extra, "sharingMerId")
	require.NotContains(t, extra, "raw_resource")
	require.NotContains(t, extra, "signature")
}

func TestBaofuReconciliationAlertsUseSanitizedProviderPayloads(t *testing.T) {
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)

	alerts := []AlertData{
		newBaofuPaymentCallbackMissingAlert(db.PaymentOrder{ID: 11, Status: "pending", CreatedAt: now.Add(-2 * time.Hour)}, time.Hour),
		newBaofuProfitSharingProcessingSLAAlert(db.ProfitSharingOrder{ID: 12, PaymentOrderID: 11, Status: db.ProfitSharingOrderStatusProcessing, CreatedAt: now.Add(-2 * time.Hour)}, time.Hour),
		newBaofuWithdrawalProcessingSLAAlert(db.BaofuWithdrawalOrder{ID: 13, Status: db.BaofuWithdrawalStatusProcessing, CreatedAt: now.Add(-2 * time.Hour), Amount: 5000}, time.Hour),
		newBaofuFailedFactAlert(db.ExternalPaymentFact{ID: 14, Provider: db.ExternalPaymentProviderBaofu, Channel: db.PaymentChannelBaofuAggregate, ProcessingStatus: db.ExternalPaymentFactProcessingStatusFailed}),
		newBaofuFeeLedgerMismatchAlert(15, 11, 30, 29),
	}

	require.Equal(t, AlertTypeBaofuPaymentCallbackMissing, alerts[0].AlertType)
	require.Equal(t, AlertTypeBaofuShareProcessingSLA, alerts[1].AlertType)
	require.Equal(t, AlertTypeBaofuWithdrawalProcessingSLA, alerts[2].AlertType)
	require.Equal(t, AlertTypeBaofuFactApplicationFailed, alerts[3].AlertType)
	require.Equal(t, AlertTypeBaofuFeeLedgerMismatch, alerts[4].AlertType)
	for _, alert := range alerts {
		require.Equal(t, AlertLevelWarning, alert.Level)
		require.Equal(t, db.ExternalPaymentProviderBaofu, alert.Extra["provider"])
		require.Equal(t, db.PaymentChannelBaofuAggregate, alert.Extra["channel"])
		require.NotContains(t, alert.Extra, "contract_no")
		require.NotContains(t, alert.Extra, "sharing_mer_id")
		require.NotContains(t, alert.Extra, "raw_resource")
	}
}
