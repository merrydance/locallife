package wechatruntime

import (
	"context"
	"testing"

	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCreateEcommerceRefundContractMapsAmountFrom(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mockwechat.NewMockEcommerceClientInterface(ctrl)
	req := &wechatcontracts.EcommerceRefundRequest{
		SubMchID:    "sub_mch_001",
		OutTradeNo:  "trade_001",
		OutRefundNo: "refund_001",
		Reason:      "test",
		Amount: &wechatcontracts.EcommerceRefundRequestAmount{
			Refund:   100,
			Total:    200,
			Currency: wechatcontracts.EcommerceRefundCurrencyCNY,
			From: []wechatcontracts.EcommerceRefundAmountFrom{{
				Account: "AVAILABLE",
				Amount:  100,
			}},
		},
	}

	client.EXPECT().CreateEcommerceRefund(gomock.Any(), gomock.AssignableToTypeOf(&wechat.EcommerceRefundRequest{})).DoAndReturn(
		func(_ context.Context, runtimeReq *wechat.EcommerceRefundRequest) (*wechat.EcommerceRefundResponse, error) {
			require.Len(t, runtimeReq.AmountFrom, 1)
			require.Equal(t, "AVAILABLE", runtimeReq.AmountFrom[0].Account)
			require.Equal(t, int64(100), runtimeReq.AmountFrom[0].Amount)
			return &wechat.EcommerceRefundResponse{
				RefundID:    "wx_refund_001",
				OutRefundNo: req.OutRefundNo,
				CreateTime:  "2026-04-17T12:00:00+08:00",
				Amount: wechat.EcommerceRefundAmount{
					Refund:   100,
					Currency: wechatcontracts.EcommerceRefundCurrencyCNY,
					From: []wechat.EcommerceRefundAmountFrom{{
						Account: "AVAILABLE",
						Amount:  100,
					}},
				},
			}, nil
		},
	)

	resp, err := CreateEcommerceRefundContract(context.Background(), client, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Amount.From, 1)
	require.Equal(t, "AVAILABLE", resp.Amount.From[0].Account)
	require.Equal(t, int64(100), resp.Amount.From[0].Amount)
}
