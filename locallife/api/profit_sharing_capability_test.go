package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetProfitSharingAmounts_OK(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{
		ID:                    301,
		PaymentType:           PaymentTypeProfitShare,
		PaymentChannel:        db.PaymentChannelEcommerce,
		RequiresProfitSharing: true,
		TransactionID:         pgtype.Text{String: "wx_tx_301", Valid: true},
	}

	store.EXPECT().
		GetPaymentOrder(gomock.Any(), paymentOrder.ID).
		Return(paymentOrder, nil)
	ecommerce.EXPECT().
		QueryProfitSharingAmounts(gomock.Any(), "wx_tx_301").
		Return(&wechatcontracts.ProfitSharingAmountsResponse{
			TransactionID: "wx_tx_301",
			UnsplitAmount: 180,
		}, nil)

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	request, err := http.NewRequest(http.MethodGet, "/v1/operators/me/payment-orders/301/profit-sharing/amounts", nil)
	require.NoError(t, err)
	ctx.Request = request
	ctx.Params = gin.Params{{Key: "id", Value: "301"}}

	server.getProfitSharingAmounts(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response profitSharingAmountsResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, int64(301), response.PaymentOrderID)
	require.Equal(t, "wx_tx_301", response.TransactionID)
	require.Equal(t, int64(180), response.UnsplitAmount)
}

func TestGetProfitSharingAmounts_RejectsNonProfitSharingOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().
		GetPaymentOrder(gomock.Any(), int64(302)).
		Return(db.PaymentOrder{ID: 302, PaymentType: PaymentTypeMiniProgram}, nil)

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	request, err := http.NewRequest(http.MethodGet, "/v1/operators/me/payment-orders/302/profit-sharing/amounts", nil)
	require.NoError(t, err)
	ctx.Request = request
	ctx.Params = gin.Params{{Key: "id", Value: "302"}}

	server.getProfitSharingAmounts(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestDeleteProfitSharingReceiver_OK(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().
		GetPaymentOrder(gomock.Any(), int64(303)).
		Return(db.PaymentOrder{ID: 303, PaymentType: PaymentTypeProfitShare, PaymentChannel: db.PaymentChannelEcommerce, RequiresProfitSharing: true}, nil)
	ecommerce.EXPECT().
		DeleteProfitSharingReceiver(gomock.Any(), &wechatcontracts.DeleteReceiverRequest{
			AppID:   "wx_app_303",
			Type:    wechatcontracts.ReceiverTypeMerchant,
			Account: "1900000001",
		}).
		Return(&wechatcontracts.DeleteReceiverResponse{
			Type:    wechatcontracts.ReceiverTypeMerchant,
			Account: "1900000001",
		}, nil)

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	request, err := http.NewRequest(http.MethodPost, "/v1/operators/me/payment-orders/303/profit-sharing/receivers/delete", strings.NewReader(`{"appid":"wx_app_303","type":"MERCHANT_ID","account":"1900000001"}`))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	ctx.Request = request
	ctx.Params = gin.Params{{Key: "id", Value: "303"}}

	server.deleteProfitSharingReceiver(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response deleteProfitSharingReceiverResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, int64(303), response.PaymentOrderID)
	require.Equal(t, wechatcontracts.ReceiverTypeMerchant, response.Type)
	require.Equal(t, "1900000001", response.Account)
}

func TestDeleteProfitSharingReceiver_ValidationErrorReturnsBadRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().
		GetPaymentOrder(gomock.Any(), int64(304)).
		Return(db.PaymentOrder{ID: 304, PaymentType: PaymentTypeProfitShare, PaymentChannel: db.PaymentChannelEcommerce, RequiresProfitSharing: true}, nil)
	ecommerce.EXPECT().
		DeleteProfitSharingReceiver(gomock.Any(), &wechatcontracts.DeleteReceiverRequest{
			AppID:   "",
			Type:    wechatcontracts.ReceiverTypePersonal,
			Account: "openid_304",
		}).
		Return(nil, &wechatcontracts.ProfitSharingValidationError{Message: "delete profit sharing receiver: appid is required for personal receivers"})

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	request, err := http.NewRequest(http.MethodPost, "/v1/operators/me/payment-orders/304/profit-sharing/receivers/delete", strings.NewReader(`{"type":"PERSONAL_OPENID","account":"openid_304"}`))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	ctx.Request = request
	ctx.Params = gin.Params{{Key: "id", Value: "304"}}

	server.deleteProfitSharingReceiver(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}
