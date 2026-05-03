package logic

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCalculateBaofuPaymentFeeFenRoundsUpMerchantBorneFee(t *testing.T) {
	require.Equal(t, int64(30), CalculateBaofuPaymentFeeFen(10000))
	require.Equal(t, int64(1), CalculateBaofuPaymentFeeFen(1))
	require.Equal(t, int64(1), CalculateBaofuPaymentFeeFen(333))
	require.Equal(t, int64(0), CalculateBaofuPaymentFeeFen(0))
}

func TestCalculateBaofuProfitSharingAmountsMerchantBearsPaymentFee(t *testing.T) {
	result, err := CalculateBaofuProfitSharingAmounts(BaofuProfitSharingAmountInput{
		TotalAmountFen:      10000,
		DeliveryFeeFen:      500,
		PlatformRateBps:     200,
		OperatorRateBps:     300,
		HasRiderReceiver:    true,
		HasOperatorReceiver: true,
		RedirectMissingOperatorCommissionToPlatform: true,
	})

	require.NoError(t, err)
	require.Equal(t, int64(500), result.RiderAmountFen)
	require.Equal(t, int64(9500), result.DistributableAmountFen)
	require.Equal(t, int64(200), result.PlatformCommissionFen)
	require.Equal(t, int64(300), result.OperatorCommissionFen)
	require.Equal(t, int64(30), result.PaymentFeeFen)
	require.Equal(t, int64(8970), result.MerchantAmountFen)
	require.False(t, result.OperatorCommissionRedirectedToPlatform)
}

func TestCalculateBaofuProfitSharingAmountsRedirectsMissingOperatorToPlatform(t *testing.T) {
	result, err := CalculateBaofuProfitSharingAmounts(BaofuProfitSharingAmountInput{
		TotalAmountFen:      10000,
		DeliveryFeeFen:      500,
		PlatformRateBps:     200,
		OperatorRateBps:     300,
		HasRiderReceiver:    true,
		HasOperatorReceiver: false,
		RedirectMissingOperatorCommissionToPlatform: true,
	})

	require.NoError(t, err)
	require.Equal(t, int64(500), result.RiderAmountFen)
	require.Equal(t, int64(500), result.PlatformCommissionFen)
	require.Equal(t, int64(0), result.OperatorCommissionFen)
	require.Equal(t, int64(30), result.PaymentFeeFen)
	require.Equal(t, int64(8970), result.MerchantAmountFen)
	require.True(t, result.OperatorCommissionRedirectedToPlatform)
}

func TestCalculateBaofuProfitSharingAmountsRejectsNegativeMerchantAmount(t *testing.T) {
	_, err := CalculateBaofuProfitSharingAmounts(BaofuProfitSharingAmountInput{
		TotalAmountFen:      100,
		DeliveryFeeFen:      0,
		PlatformRateBps:     9900,
		OperatorRateBps:     100,
		HasOperatorReceiver: true,
		RedirectMissingOperatorCommissionToPlatform: true,
	})

	require.ErrorIs(t, err, ErrBaofuProfitSharingMerchantAmountNegative)
}
