package logic

import (
	"errors"
	"fmt"

	db "github.com/merrydance/locallife/db/sqlc"
)

var (
	ErrMerchantFeeBreakdownUnavailable  = errors.New("merchant fee breakdown unavailable")
	ErrMerchantFeeBreakdownInconsistent = errors.New("merchant fee breakdown inconsistent")
)

type MerchantOrderFeeBreakdown struct {
	FoodAmount                int64 `json:"food_amount"`
	MerchantDiscountAmount    int64 `json:"merchant_discount_amount"`
	VoucherDiscountAmount     int64 `json:"voucher_discount_amount"`
	FoodPayableAmount         int64 `json:"food_payable_amount"`
	PackagingFeeAmount        int64 `json:"packaging_fee_amount"`
	DeliveryFeeAmount         int64 `json:"delivery_fee_amount"`
	DeliveryFeeDiscountAmount int64 `json:"delivery_fee_discount_amount"`
	DeliveryPayableAmount     int64 `json:"delivery_payable_amount"`
	CustomerPayableAmount     int64 `json:"customer_payable_amount"`
	PlatformServiceFeeAmount  int64 `json:"platform_service_fee_amount"`
	PaymentChannelFeeAmount   int64 `json:"payment_channel_fee_amount"`
	MerchantReceivableAmount  int64 `json:"merchant_receivable_amount"`
	RiderGrossAmount          int64 `json:"rider_gross_amount"`
	RiderPaymentFeeAmount     int64 `json:"rider_payment_fee_amount"`
	RiderNetEarningsAmount    int64 `json:"rider_net_earnings_amount"`
}

type BuildMerchantOrderFeeBreakdownInput struct {
	Order              db.Order
	ProfitSharingOrder *db.ProfitSharingOrder
}

func BuildMerchantOrderFeeBreakdown(input BuildMerchantOrderFeeBreakdownInput) (MerchantOrderFeeBreakdown, error) {
	order := input.Order
	if input.ProfitSharingOrder == nil {
		return MerchantOrderFeeBreakdown{}, fmt.Errorf("%w: order_id=%d", ErrMerchantFeeBreakdownUnavailable, order.ID)
	}
	profitSharingOrder := *input.ProfitSharingOrder

	if order.Subtotal < 0 ||
		order.DiscountAmount < 0 ||
		order.VoucherAmount < 0 ||
		order.PackagingFee < 0 ||
		order.DeliveryFee < 0 ||
		order.DeliveryFeeDiscount < 0 ||
		order.TotalAmount < 0 ||
		profitSharingOrder.PlatformCommission < 0 ||
		profitSharingOrder.OperatorCommission < 0 ||
		profitSharingOrder.MerchantAmount < 0 ||
		profitSharingOrder.RiderGrossAmount < 0 ||
		profitSharingOrder.RiderPaymentFee < 0 ||
		profitSharingOrder.RiderAmount < 0 {
		return MerchantOrderFeeBreakdown{}, fmt.Errorf("%w: negative amount for order_id=%d profit_sharing_order_id=%d", ErrMerchantFeeBreakdownInconsistent, order.ID, profitSharingOrder.ID)
	}

	foodPayable := order.Subtotal - order.DiscountAmount - order.VoucherAmount
	deliveryPayable := order.DeliveryFee - order.DeliveryFeeDiscount
	if foodPayable < 0 || deliveryPayable < 0 {
		return MerchantOrderFeeBreakdown{}, fmt.Errorf("%w: payable amount negative for order_id=%d profit_sharing_order_id=%d", ErrMerchantFeeBreakdownInconsistent, order.ID, profitSharingOrder.ID)
	}
	if foodPayable+order.PackagingFee+deliveryPayable != order.TotalAmount {
		return MerchantOrderFeeBreakdown{}, fmt.Errorf("%w: customer payable mismatch for order_id=%d profit_sharing_order_id=%d", ErrMerchantFeeBreakdownInconsistent, order.ID, profitSharingOrder.ID)
	}
	if profitSharingOrder.TotalAmount != order.TotalAmount {
		return MerchantOrderFeeBreakdown{}, fmt.Errorf("%w: bill total mismatch for order_id=%d profit_sharing_order_id=%d", ErrMerchantFeeBreakdownInconsistent, order.ID, profitSharingOrder.ID)
	}

	paymentChannelFee := profitSharingOrder.PaymentFee
	if profitSharingOrder.CalculationVersion == BaofuSettlementCalculationVersionV2 {
		paymentChannelFee = profitSharingOrder.MerchantPaymentFee
	}
	if paymentChannelFee < 0 {
		return MerchantOrderFeeBreakdown{}, fmt.Errorf("%w: payment channel fee negative for order_id=%d profit_sharing_order_id=%d", ErrMerchantFeeBreakdownInconsistent, order.ID, profitSharingOrder.ID)
	}

	return MerchantOrderFeeBreakdown{
		FoodAmount:                order.Subtotal,
		MerchantDiscountAmount:    order.DiscountAmount,
		VoucherDiscountAmount:     order.VoucherAmount,
		FoodPayableAmount:         foodPayable,
		PackagingFeeAmount:        order.PackagingFee,
		DeliveryFeeAmount:         order.DeliveryFee,
		DeliveryFeeDiscountAmount: order.DeliveryFeeDiscount,
		DeliveryPayableAmount:     deliveryPayable,
		CustomerPayableAmount:     order.TotalAmount,
		PlatformServiceFeeAmount:  profitSharingOrder.PlatformCommission + profitSharingOrder.OperatorCommission,
		PaymentChannelFeeAmount:   paymentChannelFee,
		MerchantReceivableAmount:  profitSharingOrder.MerchantAmount,
		RiderGrossAmount:          profitSharingOrder.RiderGrossAmount,
		RiderPaymentFeeAmount:     profitSharingOrder.RiderPaymentFee,
		RiderNetEarningsAmount:    profitSharingOrder.RiderAmount,
	}, nil
}
