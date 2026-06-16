package worker

import (
	"testing"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
)

func TestRefundTypeForPaymentOrder(t *testing.T) {
	t.Parallel()

	require.Equal(t, paymentTypeProfitSharing, refundTypeForPaymentOrder(db.PaymentOrder{PaymentChannel: db.PaymentChannelBaofuAggregate, RequiresProfitSharing: true}))
	require.Equal(t, "miniprogram", refundTypeForPaymentOrder(db.PaymentOrder{PaymentType: "native"}))
	require.Equal(t, "miniprogram", refundTypeForPaymentOrder(db.PaymentOrder{PaymentType: "miniprogram"}))
}
