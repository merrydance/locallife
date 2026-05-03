package logic

import "errors"

const (
	BaofuPaymentFeeRateBps = 30
)

var (
	ErrBaofuProfitSharingInvalidAmount          = errors.New("baofu profit sharing amount input is invalid")
	ErrBaofuProfitSharingMerchantAmountNegative = errors.New("baofu profit sharing merchant amount is negative")
)

type BaofuProfitSharingAmountInput struct {
	TotalAmountFen                              int64
	DeliveryFeeFen                              int64
	PlatformRateBps                             int32
	OperatorRateBps                             int32
	HasRiderReceiver                            bool
	HasOperatorReceiver                         bool
	RedirectMissingOperatorCommissionToPlatform bool
}

type BaofuProfitSharingAmountResult struct {
	TotalAmountFen                         int64
	DeliveryFeeFen                         int64
	RiderAmountFen                         int64
	DistributableAmountFen                 int64
	PlatformRateBps                        int32
	OperatorRateBps                        int32
	PlatformCommissionFen                  int64
	OperatorCommissionFen                  int64
	PaymentFeeFen                          int64
	PaymentFeeRateBps                      int32
	MerchantAmountFen                      int64
	OperatorCommissionRedirectedToPlatform bool
}

func CalculateBaofuPaymentFeeFen(totalAmountFen int64) int64 {
	if totalAmountFen <= 0 {
		return 0
	}
	return (totalAmountFen*BaofuPaymentFeeRateBps + 9999) / 10000
}

func CalculateBaofuProfitSharingAmounts(input BaofuProfitSharingAmountInput) (BaofuProfitSharingAmountResult, error) {
	if input.TotalAmountFen < 0 || input.DeliveryFeeFen < 0 || input.PlatformRateBps < 0 || input.OperatorRateBps < 0 {
		return BaofuProfitSharingAmountResult{}, ErrBaofuProfitSharingInvalidAmount
	}

	result := BaofuProfitSharingAmountResult{
		TotalAmountFen:    input.TotalAmountFen,
		DeliveryFeeFen:    input.DeliveryFeeFen,
		PlatformRateBps:   input.PlatformRateBps,
		OperatorRateBps:   input.OperatorRateBps,
		PaymentFeeFen:     CalculateBaofuPaymentFeeFen(input.TotalAmountFen),
		PaymentFeeRateBps: BaofuPaymentFeeRateBps,
	}
	if input.HasRiderReceiver {
		result.RiderAmountFen = minInt64(input.DeliveryFeeFen, input.TotalAmountFen)
	}
	result.DistributableAmountFen = input.TotalAmountFen - result.RiderAmountFen
	if result.DistributableAmountFen < 0 {
		result.DistributableAmountFen = 0
	}

	result.PlatformCommissionFen = input.TotalAmountFen * int64(input.PlatformRateBps) / 10000
	result.OperatorCommissionFen = input.TotalAmountFen * int64(input.OperatorRateBps) / 10000
	if !input.HasOperatorReceiver && result.OperatorCommissionFen > 0 && input.RedirectMissingOperatorCommissionToPlatform {
		result.PlatformCommissionFen += result.OperatorCommissionFen
		result.OperatorCommissionFen = 0
		result.OperatorCommissionRedirectedToPlatform = true
	}

	result.MerchantAmountFen = result.DistributableAmountFen - result.PlatformCommissionFen - result.OperatorCommissionFen - result.PaymentFeeFen
	if result.MerchantAmountFen < 0 {
		return BaofuProfitSharingAmountResult{}, ErrBaofuProfitSharingMerchantAmountNegative
	}
	return result, nil
}
