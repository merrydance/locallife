package logic

import (
	"errors"
	"testing"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
)

func TestBuildMerchantOrderFeeBreakdown_BaofuV2UsesMerchantVisibleAmounts(t *testing.T) {
	order := db.Order{
		ID:                  101,
		Subtotal:            10000,
		DiscountAmount:      300,
		VoucherAmount:       200,
		DeliveryFee:         800,
		DeliveryFeeDiscount: 0,
		TotalAmount:         10300,
		Status:              db.OrderStatusPaid,
	}
	profitSharingOrder := db.ProfitSharingOrder{
		ID:                           201,
		PaymentOrderID:               301,
		TotalAmount:                  10300,
		PlatformCommission:           190,
		OperatorCommission:           285,
		MerchantAmount:               8968,
		CalculationVersion:           BaofuSettlementCalculationVersionV2,
		ProviderPaymentFee:           31,
		MerchantPaymentFee:           57,
		PaymentFee:                   31,
		CommissionBaseAmount:         9500,
		DistributableAmount:          9500,
		MerchantPaymentFeeBaseAmount: 9500,
		RiderGrossAmount:             800,
		RiderPaymentFee:              5,
		RiderAmount:                  795,
	}

	breakdown, err := BuildMerchantOrderFeeBreakdown(BuildMerchantOrderFeeBreakdownInput{
		Order:              order,
		ProfitSharingOrder: &profitSharingOrder,
	})
	require.NoError(t, err)

	require.Equal(t, int64(10000), breakdown.FoodAmount)
	require.Equal(t, int64(300), breakdown.MerchantDiscountAmount)
	require.Equal(t, int64(200), breakdown.VoucherDiscountAmount)
	require.Equal(t, int64(9500), breakdown.FoodPayableAmount)
	require.Equal(t, int64(800), breakdown.DeliveryFeeAmount)
	require.Equal(t, int64(0), breakdown.DeliveryFeeDiscountAmount)
	require.Equal(t, int64(800), breakdown.DeliveryPayableAmount)
	require.Equal(t, int64(10300), breakdown.CustomerPayableAmount)
	require.Equal(t, int64(475), breakdown.PlatformServiceFeeAmount)
	require.Equal(t, int64(57), breakdown.PaymentChannelFeeAmount)
	require.Equal(t, int64(8968), breakdown.MerchantReceivableAmount)
	require.Equal(t, int64(800), breakdown.RiderGrossAmount)
	require.Equal(t, int64(5), breakdown.RiderPaymentFeeAmount)
	require.Equal(t, int64(795), breakdown.RiderNetEarningsAmount)
}

func TestBuildMerchantOrderFeeBreakdown_ReturnsUnavailableWhenProfitSharingMissing(t *testing.T) {
	_, err := BuildMerchantOrderFeeBreakdown(BuildMerchantOrderFeeBreakdownInput{
		Order: db.Order{
			ID:          101,
			Status:      db.OrderStatusPaid,
			Subtotal:    1000,
			TotalAmount: 1000,
		},
	})

	require.ErrorIs(t, err, ErrMerchantFeeBreakdownUnavailable)
}

func TestBuildMerchantOrderFeeBreakdown_ReturnsInconsistentWhenCustomerPayableMismatches(t *testing.T) {
	profitSharingOrder := db.ProfitSharingOrder{ID: 201, TotalAmount: 1200, MerchantAmount: 1000}

	_, err := BuildMerchantOrderFeeBreakdown(BuildMerchantOrderFeeBreakdownInput{
		Order: db.Order{
			ID:                  101,
			Status:              db.OrderStatusPaid,
			Subtotal:            1000,
			DiscountAmount:      0,
			VoucherAmount:       0,
			DeliveryFee:         200,
			DeliveryFeeDiscount: 0,
			TotalAmount:         1199,
		},
		ProfitSharingOrder: &profitSharingOrder,
	})

	require.ErrorIs(t, err, ErrMerchantFeeBreakdownInconsistent)
	require.True(t, errors.Is(err, ErrMerchantFeeBreakdownInconsistent))
}

func TestBuildMerchantOrderFeeBreakdown_ReturnsInconsistentWhenBillTotalMismatchesOrder(t *testing.T) {
	profitSharingOrder := db.ProfitSharingOrder{ID: 201, TotalAmount: 1201, MerchantAmount: 1000}

	_, err := BuildMerchantOrderFeeBreakdown(BuildMerchantOrderFeeBreakdownInput{
		Order: db.Order{
			ID:                  101,
			Status:              db.OrderStatusCancelled,
			Subtotal:            1000,
			DiscountAmount:      0,
			VoucherAmount:       0,
			DeliveryFee:         200,
			DeliveryFeeDiscount: 0,
			TotalAmount:         1200,
		},
		ProfitSharingOrder: &profitSharingOrder,
	})

	require.ErrorIs(t, err, ErrMerchantFeeBreakdownInconsistent)
}
