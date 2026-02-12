package logic

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComputeOrderTotals(t *testing.T) {
	testCases := []struct {
		name  string
		input OrderTotalsInput
		check func(t *testing.T, result OrderTotalsResult, err error)
	}{
		{
			name: "BasicTotal",
			input: OrderTotalsInput{
				Subtotal:            1000,
				DiscountAmount:      100,
				VoucherAmount:       200,
				DeliveryFee:         50,
				DeliveryFeeDiscount: 10,
			},
			check: func(t *testing.T, result OrderTotalsResult, err error) {
				require.NoError(t, err)
				require.Equal(t, int64(740), result.TotalAmount)
				require.Equal(t, int64(0), result.BalancePaid)
			},
		},
		{
			name: "TotalNegative",
			input: OrderTotalsInput{
				Subtotal:       100,
				DiscountAmount: 200,
			},
			check: func(t *testing.T, result OrderTotalsResult, err error) {
				require.NoError(t, err)
				require.Equal(t, int64(0), result.TotalAmount)
			},
		},
		{
			name: "DepositDeductionCap",
			input: OrderTotalsInput{
				Subtotal:         500,
				DepositDeduction: 800,
			},
			check: func(t *testing.T, result OrderTotalsResult, err error) {
				require.NoError(t, err)
				require.Equal(t, int64(0), result.TotalAmount)
			},
		},
		{
			name: "BalanceRequired",
			input: OrderTotalsInput{
				Subtotal:          300,
				UseBalance:        true,
				MembershipBalance: 0,
			},
			check: func(t *testing.T, _ OrderTotalsResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "insufficient membership balance", reqErr.Err.Error())
			},
		},
		{
			name: "BalancePartial",
			input: OrderTotalsInput{
				Subtotal:          1000,
				UseBalance:        true,
				MembershipBalance: 300,
			},
			check: func(t *testing.T, result OrderTotalsResult, err error) {
				require.NoError(t, err)
				require.Equal(t, int64(1000), result.TotalAmount)
				require.Equal(t, int64(300), result.BalancePaid)
			},
		},
		{
			name: "BalanceFull",
			input: OrderTotalsInput{
				Subtotal:          500,
				UseBalance:        true,
				MembershipBalance: 700,
			},
			check: func(t *testing.T, result OrderTotalsResult, err error) {
				require.NoError(t, err)
				require.Equal(t, int64(500), result.TotalAmount)
				require.Equal(t, int64(500), result.BalancePaid)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			result, err := ComputeOrderTotals(tc.input)
			tc.check(t, result, err)
		})
	}
}
