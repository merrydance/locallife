package logic

import (
	"testing"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
)

func TestPaymentOrderUsesEcommerceChannel(t *testing.T) {
	t.Parallel()

	require.True(t, paymentOrderUsesEcommerceChannel(db.PaymentOrder{PaymentChannel: db.PaymentChannelEcommerce, PaymentType: paymentTypeMiniProgram}))
	require.False(t, paymentOrderUsesEcommerceChannel(db.PaymentOrder{PaymentChannel: db.PaymentChannelDirect, PaymentType: paymentTypeMiniProgram}))
}

func TestRefundTypeForPaymentOrder(t *testing.T) {
	t.Parallel()

	require.Equal(t, paymentTypeProfitSharing, refundTypeForPaymentOrder(db.PaymentOrder{PaymentChannel: db.PaymentChannelEcommerce, PaymentType: paymentTypeMiniProgram}))
	require.Equal(t, paymentTypeMiniProgram, refundTypeForPaymentOrder(db.PaymentOrder{PaymentType: paymentTypeNative}))
	require.Equal(t, paymentTypeMiniProgram, refundTypeForPaymentOrder(db.PaymentOrder{PaymentType: paymentTypeMiniProgram}))
}

func TestPaymentOrderRequiresProfitSharing(t *testing.T) {
	t.Parallel()

	require.True(t, paymentOrderRequiresProfitSharing(db.PaymentOrder{PaymentChannel: db.PaymentChannelEcommerce, RequiresProfitSharing: true}))
	require.False(t, paymentOrderRequiresProfitSharing(db.PaymentOrder{PaymentChannel: db.PaymentChannelEcommerce, RequiresProfitSharing: false}))
}
