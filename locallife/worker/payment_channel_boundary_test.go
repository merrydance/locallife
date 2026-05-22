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

func TestPaymentOrderRequiresProfitSharing(t *testing.T) {
	t.Parallel()

	require.True(t, paymentOrderRequiresProfitSharing(db.PaymentOrder{PaymentChannel: db.PaymentChannelBaofuAggregate, RequiresProfitSharing: true}))
	require.False(t, paymentOrderRequiresProfitSharing(db.PaymentOrder{PaymentChannel: db.PaymentChannelBaofuAggregate}))
	require.False(t, paymentOrderRequiresProfitSharing(db.PaymentOrder{PaymentChannel: db.PaymentChannelDirect, RequiresProfitSharing: true}))
	require.False(t, paymentOrderRequiresProfitSharing(db.PaymentOrder{RequiresProfitSharing: true}))
}

func TestPaymentOrderUsesMainBusinessRefundChannel(t *testing.T) {
	t.Parallel()

	require.True(t, paymentOrderUsesMainBusinessRefundChannel(db.PaymentOrder{PaymentChannel: db.PaymentChannelBaofuAggregate}))
	require.False(t, paymentOrderUsesMainBusinessRefundChannel(db.PaymentOrder{PaymentChannel: db.PaymentChannelDirect}))
	require.False(t, paymentOrderUsesMainBusinessRefundChannel(db.PaymentOrder{}))
}
