package logic

import (
	"context"

	"github.com/merrydance/locallife/internal/wechatruntime"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
)

func generateOutRefundNo() (string, error) {
	return util.GenerateOutRefundNo()
}

func createDirectRefundContract(ctx context.Context, paymentClient wechat.DirectPaymentClientInterface, req *wechatcontracts.DirectRefundRequest) (*wechatcontracts.DirectRefundResponse, error) {
	return wechatruntime.CreateDirectRefundContract(ctx, paymentClient, req)
}

func createEcommerceRefundContract(ctx context.Context, ecommerceClient wechat.EcommerceClientInterface, req *wechatcontracts.EcommerceRefundRequest) (*wechatcontracts.EcommerceRefundCreateResponse, error) {
	return wechatruntime.CreateEcommerceRefundContract(ctx, ecommerceClient, req)
}

func applyEcommerceAbnormalRefundContract(ctx context.Context, ecommerceClient wechat.EcommerceClientInterface, req *wechatcontracts.EcommerceAbnormalRefundRequest) (*wechatcontracts.EcommerceRefundQueryResponse, error) {
	return wechatruntime.ApplyEcommerceAbnormalRefundContract(ctx, ecommerceClient, req)
}
