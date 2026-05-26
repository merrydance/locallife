package logic

import "errors"

const (
	BaofuSettlementCalculationVersionV2 = "baofu_fee_v2"

	BaofuSettlementSceneTakeout     = "takeout"
	BaofuSettlementSceneReservation = "reservation"
	BaofuSettlementSceneDineIn      = "dine_in"
	BaofuSettlementSceneTakeaway    = "takeaway"

	BaofuSettlementModeCommissionShare = "commission_share"
	BaofuSettlementModeFeeOnlyShare    = "fee_only_share"

	BaofuProviderPaymentFeeSourceEstimated = "estimated"
	BaofuProviderPaymentFeeSourceActual    = "actual"
	// Baofoo deducts its provider-side payment fee before funds reach the reserve pool.
	BaofuProviderPaymentFeeTimingRealtimeDeductedBeforeReserve = "realtime_deducted_before_reserve"

	DefaultBaofuProviderPaymentFeeRateBps = int32(30)
	DefaultBaofuPaymentServiceFeeRateBps  = int32(60)
)

var (
	ErrBaofuSettlementInvalidAmount                  = errors.New("baofu settlement amount input is invalid")
	ErrBaofuSettlementUnsupportedScene               = errors.New("baofu settlement scene is unsupported")
	ErrBaofuSettlementMerchantAmountNegative         = errors.New("baofu settlement merchant amount is negative")
	ErrBaofuSettlementRiderAmountNegative            = errors.New("baofu settlement rider amount is negative")
	ErrBaofuSettlementPlatformReceiverAmountNegative = errors.New("baofu settlement platform receiver amount is negative")
)

type BaofuSettlementCalculationInput struct {
	OrderScene                 string
	TotalAmountFen             int64
	DeliveryFeeFen             int64
	ProviderPaymentFeeFen      int64
	HasRiderReceiver           bool
	HasOperatorReceiver        bool
	RedirectMissingOperatorFee bool
	PlatformCommissionRateBps  int32
	OperatorCommissionRateBps  int32
	MerchantPaymentFeeRateBps  int32
	RiderPaymentFeeRateBps     int32
}

type BaofuSettlementCalculationResult struct {
	CalculationVersion                     string
	SettlementMode                         string
	TotalAmountFen                         int64
	ShareableAmountFen                     int64
	ProviderPaymentFeeFen                  int64
	ProviderPaymentFeeRateBps              int32
	ProviderPaymentFeeBaseFen              int64
	ProviderPaymentFeeSource               string
	MerchantPaymentFeeBaseFen              int64
	MerchantPaymentFeeFen                  int64
	MerchantPaymentFeeRateBps              int32
	RiderGrossAmountFen                    int64
	RiderPaymentFeeBaseFen                 int64
	RiderPaymentFeeFen                     int64
	RiderPaymentFeeRateBps                 int32
	CommissionBaseFen                      int64
	PlatformCommissionRateBps              int32
	OperatorCommissionRateBps              int32
	PlatformCommissionFen                  int64
	OperatorCommissionFen                  int64
	OperatorCommissionRedirectedToPlatform bool
	MerchantAmountFen                      int64
	RiderAmountFen                         int64
	PlatformReceiverAmountFen              int64
}

func CalculateBaofuSettlementAmounts(input BaofuSettlementCalculationInput) (BaofuSettlementCalculationResult, error) {
	if input.TotalAmountFen < 0 ||
		input.DeliveryFeeFen < 0 ||
		input.ProviderPaymentFeeFen < 0 ||
		input.PlatformCommissionRateBps < 0 ||
		input.OperatorCommissionRateBps < 0 ||
		input.MerchantPaymentFeeRateBps < 0 ||
		input.RiderPaymentFeeRateBps < 0 {
		return BaofuSettlementCalculationResult{}, ErrBaofuSettlementInvalidAmount
	}

	if input.MerchantPaymentFeeRateBps == 0 {
		input.MerchantPaymentFeeRateBps = DefaultBaofuPaymentServiceFeeRateBps
	}
	if input.RiderPaymentFeeRateBps == 0 {
		input.RiderPaymentFeeRateBps = DefaultBaofuPaymentServiceFeeRateBps
	}

	result := BaofuSettlementCalculationResult{
		CalculationVersion:        BaofuSettlementCalculationVersionV2,
		TotalAmountFen:            input.TotalAmountFen,
		ProviderPaymentFeeRateBps: DefaultBaofuProviderPaymentFeeRateBps,
		ProviderPaymentFeeBaseFen: input.TotalAmountFen,
		MerchantPaymentFeeRateBps: input.MerchantPaymentFeeRateBps,
		RiderPaymentFeeRateBps:    input.RiderPaymentFeeRateBps,
		PlatformCommissionRateBps: input.PlatformCommissionRateBps,
		OperatorCommissionRateBps: input.OperatorCommissionRateBps,
	}
	if input.ProviderPaymentFeeFen > 0 {
		result.ProviderPaymentFeeFen = input.ProviderPaymentFeeFen
		result.ProviderPaymentFeeSource = BaofuProviderPaymentFeeSourceActual
	} else {
		result.ProviderPaymentFeeFen = roundBaofuFeeFen(input.TotalAmountFen, DefaultBaofuProviderPaymentFeeRateBps)
		result.ProviderPaymentFeeSource = BaofuProviderPaymentFeeSourceEstimated
	}
	result.ShareableAmountFen = input.TotalAmountFen - result.ProviderPaymentFeeFen
	if result.ShareableAmountFen < 0 {
		result.ShareableAmountFen = 0
	}

	switch input.OrderScene {
	case BaofuSettlementSceneTakeout:
		result.SettlementMode = BaofuSettlementModeCommissionShare
		result.RiderGrossAmountFen = minInt64(input.DeliveryFeeFen, input.TotalAmountFen)
		result.MerchantPaymentFeeBaseFen = input.TotalAmountFen - result.RiderGrossAmountFen
		if input.HasRiderReceiver {
			result.RiderPaymentFeeBaseFen = result.RiderGrossAmountFen
		}
		result.CommissionBaseFen = result.MerchantPaymentFeeBaseFen
	case BaofuSettlementSceneReservation:
		result.SettlementMode = BaofuSettlementModeCommissionShare
		result.MerchantPaymentFeeBaseFen = input.TotalAmountFen
		result.CommissionBaseFen = input.TotalAmountFen
	case BaofuSettlementSceneDineIn, BaofuSettlementSceneTakeaway:
		result.SettlementMode = BaofuSettlementModeFeeOnlyShare
		result.MerchantPaymentFeeBaseFen = input.TotalAmountFen
		result.CommissionBaseFen = 0
	default:
		return BaofuSettlementCalculationResult{}, ErrBaofuSettlementUnsupportedScene
	}
	if result.MerchantPaymentFeeBaseFen < 0 {
		result.MerchantPaymentFeeBaseFen = 0
	}

	result.MerchantPaymentFeeFen = roundBaofuFeeFen(result.MerchantPaymentFeeBaseFen, result.MerchantPaymentFeeRateBps)
	result.RiderPaymentFeeFen = roundBaofuFeeFen(result.RiderPaymentFeeBaseFen, result.RiderPaymentFeeRateBps)
	result.PlatformCommissionFen, result.OperatorCommissionFen = allocateBaofuCommissionFen(
		result.CommissionBaseFen,
		result.PlatformCommissionRateBps,
		result.OperatorCommissionRateBps,
	)
	if !input.HasOperatorReceiver && result.OperatorCommissionFen > 0 && input.RedirectMissingOperatorFee {
		result.PlatformCommissionFen += result.OperatorCommissionFen
		result.OperatorCommissionFen = 0
		result.OperatorCommissionRedirectedToPlatform = true
	}

	if input.HasRiderReceiver {
		result.RiderAmountFen = result.RiderGrossAmountFen - result.RiderPaymentFeeFen
		if result.RiderAmountFen < 0 {
			return BaofuSettlementCalculationResult{}, ErrBaofuSettlementRiderAmountNegative
		}
	}
	result.MerchantAmountFen = result.MerchantPaymentFeeBaseFen -
		result.MerchantPaymentFeeFen -
		result.PlatformCommissionFen -
		result.OperatorCommissionFen
	if result.MerchantAmountFen < 0 {
		return BaofuSettlementCalculationResult{}, ErrBaofuSettlementMerchantAmountNegative
	}
	result.PlatformReceiverAmountFen = result.PlatformCommissionFen +
		result.MerchantPaymentFeeFen +
		result.RiderPaymentFeeFen -
		result.ProviderPaymentFeeFen
	if result.PlatformReceiverAmountFen < 0 {
		return BaofuSettlementCalculationResult{}, ErrBaofuSettlementPlatformReceiverAmountNegative
	}

	return result, nil
}

func roundBaofuFeeFen(baseFen int64, rateBps int32) int64 {
	if baseFen <= 0 || rateBps <= 0 {
		return 0
	}
	return (baseFen*int64(rateBps) + 5000) / 10000
}

func allocateBaofuCommissionFen(baseFen int64, platformRateBps int32, operatorRateBps int32) (int64, int64) {
	if baseFen <= 0 {
		return 0, 0
	}
	totalRateBps := platformRateBps + operatorRateBps
	if totalRateBps <= 0 {
		return 0, 0
	}

	platformRaw := baseFen * int64(platformRateBps)
	operatorRaw := baseFen * int64(operatorRateBps)
	platform := platformRaw / 10000
	operator := operatorRaw / 10000
	remainder := roundBaofuFeeFen(baseFen, totalRateBps) - platform - operator
	if remainder <= 0 {
		return platform, operator
	}

	platformRemainder := platformRaw % 10000
	operatorRemainder := operatorRaw % 10000
	if platformRemainder >= operatorRemainder {
		if remainder > 0 && platformRateBps > 0 {
			platform++
			remainder--
		}
		if remainder > 0 && operatorRateBps > 0 {
			operator++
			remainder--
		}
	} else {
		if remainder > 0 && operatorRateBps > 0 {
			operator++
			remainder--
		}
		if remainder > 0 && platformRateBps > 0 {
			platform++
			remainder--
		}
	}
	if remainder > 0 {
		if platformRateBps > 0 {
			platform += remainder
		} else {
			operator += remainder
		}
	}
	return platform, operator
}
