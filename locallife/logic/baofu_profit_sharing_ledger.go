package logic

import (
	"encoding/json"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

func buildBaofuSharingDetailSnapshot(amounts BaofuSettlementCalculationResult, receivers BaofuProfitSharingReceiverResult) ([]byte, error) {
	snapshot := baofuSharingDetailSnapshot{
		Provider:               db.ExternalPaymentProviderBaofu,
		Channel:                db.PaymentChannelBaofuAggregate,
		CalculationVersion:     amounts.CalculationVersion,
		SettlementMode:         amounts.SettlementMode,
		ShareableAmount:        amounts.ShareableAmountFen,
		PlatformReceiverAmount: amounts.PlatformReceiverAmountFen,
		Fees: baofuSharingDetailSnapshotFees{
			ProviderPaymentFee:       amounts.ProviderPaymentFeeFen,
			MerchantPaymentFee:       amounts.MerchantPaymentFeeFen,
			RiderPaymentFee:          amounts.RiderPaymentFeeFen,
			ProviderPaymentFeeSource: amounts.ProviderPaymentFeeSource,
			ProviderPaymentFeeTiming: BaofuProviderPaymentFeeTimingRealtimeDeductedBeforeReserve,
		},
		Bases: baofuSharingDetailSnapshotBases{
			TotalAmount:            amounts.TotalAmountFen,
			MerchantPaymentFeeBase: amounts.MerchantPaymentFeeBaseFen,
			RiderPaymentFeeBase:    amounts.RiderPaymentFeeBaseFen,
			CommissionBase:         amounts.CommissionBaseFen,
		},
		Receivers: []baofuSharingDetailSnapshotRow{
			{Role: "merchant", SharingMerID: receivers.MerchantSharingMerID, Amount: amounts.MerchantAmountFen},
		},
	}
	if amounts.RiderAmountFen > 0 {
		snapshot.Receivers = append(snapshot.Receivers, baofuSharingDetailSnapshotRow{Role: "rider", SharingMerID: receivers.RiderSharingMerID, Amount: amounts.RiderAmountFen})
	}
	if amounts.OperatorCommissionFen > 0 {
		snapshot.Receivers = append(snapshot.Receivers, baofuSharingDetailSnapshotRow{Role: "operator", SharingMerID: receivers.OperatorSharingMerID, Amount: amounts.OperatorCommissionFen})
	}
	if amounts.PlatformReceiverAmountFen > 0 {
		snapshot.Receivers = append(snapshot.Receivers, baofuSharingDetailSnapshotRow{Role: "platform", SharingMerID: receivers.PlatformSharingMerID, Amount: amounts.PlatformReceiverAmountFen})
	}
	return json.Marshal(snapshot)
}

func buildBaofuOrderPaymentFeeLedgers(input BaofuProfitSharingOrderInput, amounts BaofuSettlementCalculationResult) []db.CreateOrderPaymentFeeLedgerParams {
	ledgers := []db.CreateOrderPaymentFeeLedgerParams{
		{
			Provider:           db.ExternalPaymentProviderBaofu,
			Channel:            db.PaymentChannelBaofuAggregate,
			PaymentOrderID:     input.PaymentOrderID,
			FeeType:            db.OrderPaymentFeeTypeProviderPaymentFee,
			PayerType:          db.OrderPaymentFeePayerTypePlatform,
			PayeeType:          db.OrderPaymentFeePayeeTypeBaofu,
			BaseAmount:         amounts.ProviderPaymentFeeBaseFen,
			RateBps:            amounts.ProviderPaymentFeeRateBps,
			Amount:             amounts.ProviderPaymentFeeFen,
			AmountSource:       db.OrderPaymentFeeAmountSourceCalculated,
			Status:             db.OrderPaymentFeeStatusRecorded,
			CalculationVersion: amounts.CalculationVersion,
		},
	}
	if amounts.MerchantPaymentFeeFen > 0 {
		ledgers = append(ledgers, db.CreateOrderPaymentFeeLedgerParams{
			Provider:           db.ExternalPaymentProviderBaofu,
			Channel:            db.PaymentChannelBaofuAggregate,
			PaymentOrderID:     input.PaymentOrderID,
			FeeType:            db.OrderPaymentFeeTypeMerchantPaymentServiceFee,
			PayerType:          db.OrderPaymentFeePayerTypeMerchant,
			PayerID:            pgtype.Int8{Int64: input.MerchantID, Valid: true},
			PayeeType:          db.OrderPaymentFeePayeeTypePlatform,
			BaseAmount:         amounts.MerchantPaymentFeeBaseFen,
			RateBps:            amounts.MerchantPaymentFeeRateBps,
			Amount:             amounts.MerchantPaymentFeeFen,
			AmountSource:       db.OrderPaymentFeeAmountSourceCalculated,
			Status:             db.OrderPaymentFeeStatusRecorded,
			CalculationVersion: amounts.CalculationVersion,
		})
	}
	if amounts.RiderPaymentFeeFen > 0 && input.RiderID > 0 {
		ledgers = append(ledgers, db.CreateOrderPaymentFeeLedgerParams{
			Provider:           db.ExternalPaymentProviderBaofu,
			Channel:            db.PaymentChannelBaofuAggregate,
			PaymentOrderID:     input.PaymentOrderID,
			FeeType:            db.OrderPaymentFeeTypeRiderPaymentServiceFee,
			PayerType:          db.OrderPaymentFeePayerTypeRider,
			PayerID:            pgtype.Int8{Int64: input.RiderID, Valid: true},
			PayeeType:          db.OrderPaymentFeePayeeTypePlatform,
			BaseAmount:         amounts.RiderPaymentFeeBaseFen,
			RateBps:            amounts.RiderPaymentFeeRateBps,
			Amount:             amounts.RiderPaymentFeeFen,
			AmountSource:       db.OrderPaymentFeeAmountSourceCalculated,
			Status:             db.OrderPaymentFeeStatusRecorded,
			CalculationVersion: amounts.CalculationVersion,
		})
	}
	return ledgers
}
