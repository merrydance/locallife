package worker

import (
	"testing"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
)

func TestPaymentOrderUsesEcommerceChannel(t *testing.T) {
	t.Parallel()

	require.True(t, paymentOrderUsesEcommerceChannel(db.PaymentOrder{PaymentChannel: db.PaymentChannelEcommerce, PaymentType: "miniprogram"}))
	require.False(t, paymentOrderUsesEcommerceChannel(db.PaymentOrder{PaymentChannel: db.PaymentChannelDirect, PaymentType: "miniprogram"}))
}

func TestRefundTypeForPaymentOrder(t *testing.T) {
	t.Parallel()

	require.Equal(t, paymentTypeEcommerce, refundTypeForPaymentOrder(db.PaymentOrder{PaymentChannel: db.PaymentChannelEcommerce, PaymentType: "miniprogram"}))
	require.Equal(t, "miniprogram", refundTypeForPaymentOrder(db.PaymentOrder{PaymentType: "native"}))
	require.Equal(t, "miniprogram", refundTypeForPaymentOrder(db.PaymentOrder{PaymentType: "miniprogram"}))
}

func TestPaymentOrderRequiresProfitSharing(t *testing.T) {
	t.Parallel()

	require.True(t, paymentOrderRequiresProfitSharing(db.PaymentOrder{PaymentChannel: db.PaymentChannelEcommerce, RequiresProfitSharing: true}))
	require.False(t, paymentOrderRequiresProfitSharing(db.PaymentOrder{PaymentChannel: db.PaymentChannelEcommerce, RequiresProfitSharing: false}))
}
