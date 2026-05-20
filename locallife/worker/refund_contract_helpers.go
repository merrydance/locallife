package worker

import (
	"context"

	"github.com/merrydance/locallife/internal/wechatruntime"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	"github.com/rs/zerolog/log"
)

func createDirectRefundContract(ctx context.Context, paymentClient wechat.DirectPaymentClientInterface, req *wechatcontracts.DirectRefundRequest) (*wechatcontracts.DirectRefundResponse, error) {
	return wechatruntime.CreateDirectRefundContract(ctx, paymentClient, req)
}

func createEcommerceRefundContract(ctx context.Context, ecommerceClient wechat.EcommerceClientInterface, req *wechatcontracts.EcommerceRefundRequest) (*wechatcontracts.EcommerceRefundCreateResponse, error) {
	return wechatruntime.CreateEcommerceRefundContract(ctx, ecommerceClient, req)
}

func logRefundRequestFailure(refundOrderID, paymentOrderID int64, outRefundNo, refundChannel string, err error) {
	log.Error().
		Err(err).
		Int64("refund_order_id", refundOrderID).
		Int64("payment_order_id", paymentOrderID).
		Str("out_refund_no", outRefundNo).
		Str("refund_channel", refundChannel).
		Msg("refund request failed")
}
