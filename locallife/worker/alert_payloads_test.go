package worker

import (
	"testing"

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
