package logic

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCalculateBaofuSettlementTakeoutChargesMerchantAndRiderFees(t *testing.T) {
	result, err := CalculateBaofuSettlementAmounts(BaofuSettlementCalculationInput{
		OrderScene:                BaofuSettlementSceneTakeout,
		TotalAmountFen:            10000,
		DeliveryFeeFen:            500,
		PlatformCommissionRateBps: 200,
		OperatorCommissionRateBps: 300,
		MerchantPaymentFeeRateBps: 60,
		RiderPaymentFeeRateBps:    60,
		HasRiderReceiver:          true,
		HasOperatorReceiver:       true,
	})

	require.NoError(t, err)
	require.Equal(t, BaofuSettlementCalculationVersionV2, result.CalculationVersion)
	require.Equal(t, BaofuSettlementModeCommissionShare, result.SettlementMode)
	require.Equal(t, int64(30), result.ProviderPaymentFeeFen)
	require.Equal(t, BaofuProviderPaymentFeeSourceEstimated, result.ProviderPaymentFeeSource)
	require.Equal(t, int64(9500), result.MerchantPaymentFeeBaseFen)
	require.Equal(t, int64(57), result.MerchantPaymentFeeFen)
	require.Equal(t, int64(500), result.RiderGrossAmountFen)
	require.Equal(t, int64(500), result.RiderPaymentFeeBaseFen)
	require.Equal(t, int64(3), result.RiderPaymentFeeFen)
	require.Equal(t, int64(9500), result.CommissionBaseFen)
	require.Equal(t, int64(190), result.PlatformCommissionFen)
	require.Equal(t, int64(285), result.OperatorCommissionFen)
	require.Equal(t, int64(8968), result.MerchantAmountFen)
	require.Equal(t, int64(497), result.RiderAmountFen)
	require.Equal(t, int64(220), result.PlatformReceiverAmountFen)
	require.Equal(t, int64(9970), result.ShareableAmountFen)
	require.Equal(t,
		result.ShareableAmountFen,
		result.MerchantAmountFen+result.RiderAmountFen+result.OperatorCommissionFen+result.PlatformReceiverAmountFen,
	)
}

func TestCalculateBaofuSettlementTakeoutUsesItemBaseBeforeRiderAccepts(t *testing.T) {
	result, err := CalculateBaofuSettlementAmounts(BaofuSettlementCalculationInput{
		OrderScene:                BaofuSettlementSceneTakeout,
		TotalAmountFen:            1459,
		DeliveryFeeFen:            459,
		PlatformCommissionRateBps: 200,
		OperatorCommissionRateBps: 300,
		MerchantPaymentFeeRateBps: 60,
		RiderPaymentFeeRateBps:    60,
		HasOperatorReceiver:       true,
	})

	require.NoError(t, err)
	require.Equal(t, int64(4), result.ProviderPaymentFeeFen)
	require.Equal(t, int64(1000), result.MerchantPaymentFeeBaseFen)
	require.Equal(t, int64(6), result.MerchantPaymentFeeFen)
	require.Equal(t, int64(459), result.RiderGrossAmountFen)
	require.Equal(t, int64(0), result.RiderPaymentFeeBaseFen)
	require.Equal(t, int64(0), result.RiderPaymentFeeFen)
	require.Equal(t, int64(1000), result.CommissionBaseFen)
	require.Equal(t, int64(20), result.PlatformCommissionFen)
	require.Equal(t, int64(30), result.OperatorCommissionFen)
	require.Equal(t, int64(944), result.MerchantAmountFen)
	require.Equal(t, int64(0), result.RiderAmountFen)
	require.Equal(t, int64(22), result.PlatformReceiverAmountFen)
}

func TestCalculateBaofuSettlementTakeoutCompletesRiderSplitAfterRiderAccepts(t *testing.T) {
	result, err := CalculateBaofuSettlementAmounts(BaofuSettlementCalculationInput{
		OrderScene:                BaofuSettlementSceneTakeout,
		TotalAmountFen:            1459,
		DeliveryFeeFen:            459,
		PlatformCommissionRateBps: 200,
		OperatorCommissionRateBps: 300,
		MerchantPaymentFeeRateBps: 60,
		RiderPaymentFeeRateBps:    60,
		HasRiderReceiver:          true,
		HasOperatorReceiver:       true,
	})

	require.NoError(t, err)
	require.Equal(t, int64(4), result.ProviderPaymentFeeFen)
	require.Equal(t, int64(1000), result.MerchantPaymentFeeBaseFen)
	require.Equal(t, int64(6), result.MerchantPaymentFeeFen)
	require.Equal(t, int64(459), result.RiderGrossAmountFen)
	require.Equal(t, int64(459), result.RiderPaymentFeeBaseFen)
	require.Equal(t, int64(3), result.RiderPaymentFeeFen)
	require.Equal(t, int64(1000), result.CommissionBaseFen)
	require.Equal(t, int64(20), result.PlatformCommissionFen)
	require.Equal(t, int64(30), result.OperatorCommissionFen)
	require.Equal(t, int64(944), result.MerchantAmountFen)
	require.Equal(t, int64(456), result.RiderAmountFen)
	require.Equal(t, int64(25), result.PlatformReceiverAmountFen)
	require.Equal(t,
		result.ShareableAmountFen,
		result.MerchantAmountFen+result.RiderAmountFen+result.OperatorCommissionFen+result.PlatformReceiverAmountFen,
	)
}

func TestCalculateBaofuSettlementReservationChargesMerchantFeeAndCommission(t *testing.T) {
	result, err := CalculateBaofuSettlementAmounts(BaofuSettlementCalculationInput{
		OrderScene:                BaofuSettlementSceneReservation,
		TotalAmountFen:            10000,
		PlatformCommissionRateBps: 200,
		OperatorCommissionRateBps: 300,
		MerchantPaymentFeeRateBps: 60,
		RiderPaymentFeeRateBps:    60,
		HasOperatorReceiver:       true,
	})

	require.NoError(t, err)
	require.Equal(t, BaofuSettlementModeCommissionShare, result.SettlementMode)
	require.Equal(t, int64(30), result.ProviderPaymentFeeFen)
	require.Equal(t, int64(10000), result.MerchantPaymentFeeBaseFen)
	require.Equal(t, int64(60), result.MerchantPaymentFeeFen)
	require.Equal(t, int64(0), result.RiderPaymentFeeFen)
	require.Equal(t, int64(10000), result.CommissionBaseFen)
	require.Equal(t, int64(200), result.PlatformCommissionFen)
	require.Equal(t, int64(300), result.OperatorCommissionFen)
	require.Equal(t, int64(9440), result.MerchantAmountFen)
	require.Equal(t, int64(0), result.RiderAmountFen)
	require.Equal(t, int64(230), result.PlatformReceiverAmountFen)
	require.Equal(t, int64(9970), result.ShareableAmountFen)
	require.Equal(t,
		result.ShareableAmountFen,
		result.MerchantAmountFen+result.OperatorCommissionFen+result.PlatformReceiverAmountFen,
	)
}

func TestCalculateBaofuSettlementDineInUsesFeeOnlyShare(t *testing.T) {
	result, err := CalculateBaofuSettlementAmounts(BaofuSettlementCalculationInput{
		OrderScene:                BaofuSettlementSceneDineIn,
		TotalAmountFen:            10000,
		PlatformCommissionRateBps: 200,
		OperatorCommissionRateBps: 300,
		MerchantPaymentFeeRateBps: 60,
		RiderPaymentFeeRateBps:    60,
		HasOperatorReceiver:       true,
	})

	require.NoError(t, err)
	require.Equal(t, BaofuSettlementModeFeeOnlyShare, result.SettlementMode)
	require.Equal(t, int64(30), result.ProviderPaymentFeeFen)
	require.Equal(t, int64(10000), result.MerchantPaymentFeeBaseFen)
	require.Equal(t, int64(60), result.MerchantPaymentFeeFen)
	require.Equal(t, int64(0), result.RiderPaymentFeeFen)
	require.Equal(t, int64(0), result.CommissionBaseFen)
	require.Equal(t, int64(0), result.PlatformCommissionFen)
	require.Equal(t, int64(0), result.OperatorCommissionFen)
	require.Equal(t, int64(9940), result.MerchantAmountFen)
	require.Equal(t, int64(30), result.PlatformReceiverAmountFen)
	require.Equal(t, int64(9970), result.ShareableAmountFen)
	require.Equal(t, result.ShareableAmountFen, result.MerchantAmountFen+result.PlatformReceiverAmountFen)
}

func TestBuildBaofuSharingDetailSnapshotRecordsRealtimeProviderFeeDeduction(t *testing.T) {
	amounts, err := CalculateBaofuSettlementAmounts(BaofuSettlementCalculationInput{
		OrderScene:                BaofuSettlementSceneReservation,
		TotalAmountFen:            10000,
		PlatformCommissionRateBps: 200,
		OperatorCommissionRateBps: 300,
		MerchantPaymentFeeRateBps: 60,
		RiderPaymentFeeRateBps:    60,
		HasOperatorReceiver:       true,
	})
	require.NoError(t, err)

	raw, err := buildBaofuSharingDetailSnapshot(amounts, BaofuProfitSharingReceiverResult{
		MerchantSharingMerID: "MER_SHARE",
		OperatorSharingMerID: "OP_SHARE",
		PlatformSharingMerID: "PLATFORM_SHARE",
	})
	require.NoError(t, err)

	var snapshot map[string]any
	require.NoError(t, json.Unmarshal(raw, &snapshot))
	fees := snapshot["fees"].(map[string]any)
	require.Equal(t, "realtime_deducted_before_reserve", fees["provider_payment_fee_timing"])
	require.Equal(t, float64(9970), snapshot["shareable_amount"])
	require.Equal(t, float64(230), snapshot["platform_receiver_amount"])
}

func TestCalculateBaofuSettlementUsesActualProviderFeeWhenAvailable(t *testing.T) {
	result, err := CalculateBaofuSettlementAmounts(BaofuSettlementCalculationInput{
		OrderScene:                 BaofuSettlementSceneTakeout,
		TotalAmountFen:             10000,
		DeliveryFeeFen:             500,
		ProviderPaymentFeeFen:      31,
		PlatformCommissionRateBps:  200,
		OperatorCommissionRateBps:  300,
		MerchantPaymentFeeRateBps:  60,
		RiderPaymentFeeRateBps:     60,
		HasRiderReceiver:           true,
		HasOperatorReceiver:        false,
		RedirectMissingOperatorFee: true,
	})

	require.NoError(t, err)
	require.Equal(t, int64(31), result.ProviderPaymentFeeFen)
	require.Equal(t, BaofuProviderPaymentFeeSourceActual, result.ProviderPaymentFeeSource)
	require.Equal(t, int64(475), result.PlatformCommissionFen)
	require.Equal(t, int64(0), result.OperatorCommissionFen)
	require.True(t, result.OperatorCommissionRedirectedToPlatform)
	require.Equal(t, int64(504), result.PlatformReceiverAmountFen)
	require.Equal(t, int64(9969), result.ShareableAmountFen)
	require.Equal(t,
		result.ShareableAmountFen,
		result.MerchantAmountFen+result.RiderAmountFen+result.PlatformReceiverAmountFen,
	)
}

func TestCalculateBaofuSettlementRejectsNegativePlatformReceiver(t *testing.T) {
	_, err := CalculateBaofuSettlementAmounts(BaofuSettlementCalculationInput{
		OrderScene:                BaofuSettlementSceneDineIn,
		TotalAmountFen:            100,
		ProviderPaymentFeeFen:     99,
		MerchantPaymentFeeRateBps: 60,
	})

	require.ErrorIs(t, err, ErrBaofuSettlementPlatformReceiverAmountNegative)
}
