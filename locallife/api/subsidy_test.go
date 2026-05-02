package api

import (
	"context"
	"encoding/json"
	"errors"
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

func expectSubsidyCommandAccepted(
	t *testing.T,
	store *mockdb.MockStore,
	commandType string,
	externalObjectType string,
	externalObjectKey string,
	externalSecondaryKey string,
	businessObjectID int64,
	snapshotContains map[string]string,
	snapshotNotContains ...string,
) {
	t.Helper()
	store.EXPECT().
		CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
			require.Equal(t, db.ExternalPaymentProviderWechat, arg.Provider)
			require.Equal(t, db.PaymentChannelEcommerce, arg.Channel)
			require.Equal(t, db.ExternalPaymentCapabilitySubsidy, arg.Capability)
			require.Equal(t, commandType, arg.CommandType)
			require.Equal(t, db.ExternalPaymentBusinessOwnerSubsidy, arg.BusinessOwner)
			require.Equal(t, pgtype.Text{String: "subsidy_order", Valid: true}, arg.BusinessObjectType)
			require.Equal(t, pgtype.Int8{Int64: businessObjectID, Valid: true}, arg.BusinessObjectID)
			require.Equal(t, externalObjectType, arg.ExternalObjectType)
			require.Equal(t, externalObjectKey, arg.ExternalObjectKey)
			if externalSecondaryKey == "" {
				require.False(t, arg.ExternalSecondaryKey.Valid)
			} else {
				require.Equal(t, pgtype.Text{String: externalSecondaryKey, Valid: true}, arg.ExternalSecondaryKey)
			}
			require.Equal(t, db.ExternalPaymentCommandStatusAccepted, arg.CommandStatus)
			require.False(t, arg.SubmittedAt.IsZero())
			require.True(t, arg.AcceptedAt.Valid)
			require.False(t, arg.RejectedAt.Valid)

			var snapshot map[string]string
			require.NoError(t, json.Unmarshal(arg.ResponseSnapshot, &snapshot))
			for key, expected := range snapshotContains {
				require.Equal(t, expected, snapshot[key])
			}
			rawSnapshot := string(arg.ResponseSnapshot)
			for _, forbidden := range snapshotNotContains {
				require.NotContains(t, rawSnapshot, forbidden)
			}

			return db.ExternalPaymentCommand{ID: 9001, ExternalObjectKey: externalObjectKey, CommandStatus: arg.CommandStatus}, nil
		})
}

func TestCreateSubsidy_RetryFailedOrderReusesExistingRecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{
		ID:            101,
		Status:        PaymentStatusPaid,
		TransactionID: pgtype.Text{String: "wx_tx_101", Valid: true},
	}
	existing := db.SubsidyOrder{
		ID:             201,
		PaymentOrderID: paymentOrder.ID,
		SubMchID:       "sub_mch_101",
		TransactionID:  paymentOrder.TransactionID,
		OutSubsidyNo:   "S-101-301",
		PayerAmount:    1200,
		Amount:         200,
		Description:    "平台补差",
		Status:         "failed",
	}
	updated := existing
	updated.Status = "success"
	updated.WxpaySubsidyID = pgtype.Text{String: "wx_subsidy_201", Valid: true}

	store.EXPECT().
		GetPaymentOrder(gomock.Any(), paymentOrder.ID).
		Return(paymentOrder, nil)
	store.EXPECT().
		GetMerchantPaymentConfig(gomock.Any(), int64(301)).
		Return(db.MerchantPaymentConfig{MerchantID: 301, SubMchID: "sub_mch_101"}, nil)
	store.EXPECT().
		GetSubsidyOrderByOutSubsidyNo(gomock.Any(), "S-101-301").
		Return(existing, nil)
	ecommerce.EXPECT().
		CreateSubsidy(gomock.Any(), wechatcontracts.SubsidyRequest{
			SubMchID:      "sub_mch_101",
			TransactionID: "wx_tx_101",
			Amount:        200,
			Description:   "平台补差",
			OutSubsidyNo:  "S-101-301",
		}).
		Return(&wechatcontracts.SubsidyResponse{
			SubsidyID: "wx_subsidy_201",
			Result:    wechatcontracts.SubsidyResultSuccess,
		}, nil)
	store.EXPECT().
		UpdateSubsidyOrderToSuccess(gomock.Any(), db.UpdateSubsidyOrderToSuccessParams{
			ID:             existing.ID,
			WxpaySubsidyID: pgtype.Text{String: "wx_subsidy_201", Valid: true},
			TransactionID:  paymentOrder.TransactionID,
		}).
		Return(updated, nil)
	expectSubsidyCommandAccepted(t, store, db.ExternalPaymentCommandTypeCreateSubsidy, db.ExternalPaymentObjectSubsidy, "S-101-301", "wx_subsidy_201", updated.ID, map[string]string{
		"out_subsidy_no":   "S-101-301",
		"sub_mchid":        "sub_mch_101",
		"transaction_id":   "wx_tx_101",
		"wxpay_subsidy_id": "wx_subsidy_201",
		"result":           wechatcontracts.SubsidyResultSuccess,
		"amount":           "200",
	}, "平台补差")

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	request, err := http.NewRequest(http.MethodPost, "/v1/operators/me/payment-orders/101/subsidies", strings.NewReader(`{"merchant_id":301,"payer_amount":1200,"amount":200,"description":"平台补差"}`))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	ctx.Request = request
	ctx.Params = gin.Params{{Key: "id", Value: "101"}}

	server.createSubsidy(ctx)

	require.Equal(t, http.StatusCreated, recorder.Code)
	var response subsidyOrderResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, "success", response.Status)
	require.NotNil(t, response.WxpaySubsidyID)
	require.Equal(t, "wx_subsidy_201", *response.WxpaySubsidyID)
}

func TestCreateSubsidy_AcceptsEmptyWechatBody(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{
		ID:            107,
		Status:        PaymentStatusPaid,
		TransactionID: pgtype.Text{String: "wx_tx_107", Valid: true},
	}
	created := db.SubsidyOrder{
		ID:             207,
		PaymentOrderID: paymentOrder.ID,
		SubMchID:       "sub_mch_107",
		TransactionID:  paymentOrder.TransactionID,
		OutSubsidyNo:   "S-107-307",
		PayerAmount:    1200,
		Amount:         200,
		Description:    "平台补差",
		Status:         "pending",
	}
	updated := created
	updated.Status = "success"

	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), int64(307)).Return(db.MerchantPaymentConfig{MerchantID: 307, SubMchID: "sub_mch_107"}, nil)
	store.EXPECT().GetSubsidyOrderByOutSubsidyNo(gomock.Any(), "S-107-307").Return(db.SubsidyOrder{}, db.ErrRecordNotFound)
	store.EXPECT().CreateSubsidyOrder(gomock.Any(), db.CreateSubsidyOrderParams{
		PaymentOrderID: paymentOrder.ID,
		SubMchID:       "sub_mch_107",
		TransactionID:  paymentOrder.TransactionID,
		OutSubsidyNo:   "S-107-307",
		PayerAmount:    1200,
		Amount:         200,
		Description:    "平台补差",
	}).Return(created, nil)
	ecommerce.EXPECT().CreateSubsidy(gomock.Any(), wechatcontracts.SubsidyRequest{
		SubMchID:      "sub_mch_107",
		TransactionID: "wx_tx_107",
		Amount:        200,
		Description:   "平台补差",
		OutSubsidyNo:  "S-107-307",
	}).Return(&wechatcontracts.SubsidyResponse{}, nil)
	store.EXPECT().UpdateSubsidyOrderToSuccess(gomock.Any(), db.UpdateSubsidyOrderToSuccessParams{
		ID:             created.ID,
		WxpaySubsidyID: pgtype.Text{},
		TransactionID:  paymentOrder.TransactionID,
	}).Return(updated, nil)
	expectSubsidyCommandAccepted(t, store, db.ExternalPaymentCommandTypeCreateSubsidy, db.ExternalPaymentObjectSubsidy, "S-107-307", "", updated.ID, map[string]string{
		"out_subsidy_no": "S-107-307",
		"sub_mchid":      "sub_mch_107",
		"transaction_id": "wx_tx_107",
		"amount":         "200",
	}, "wxpay_subsidy_id", "result", "平台补差")

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	request, err := http.NewRequest(http.MethodPost, "/v1/operators/me/payment-orders/107/subsidies", strings.NewReader(`{"merchant_id":307,"payer_amount":1200,"amount":200,"description":"平台补差"}`))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	ctx.Request = request
	ctx.Params = gin.Params{{Key: "id", Value: "107"}}

	server.createSubsidy(ctx)

	require.Equal(t, http.StatusCreated, recorder.Code)
	var response subsidyOrderResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, "success", response.Status)
	require.Nil(t, response.WxpaySubsidyID)
}

func TestReturnSubsidy_RetryFailedReturnReusesOriginalOutOrderNo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	subsidyOrder := db.SubsidyOrder{
		ID:             202,
		PaymentOrderID: 102,
		SubMchID:       "sub_mch_102",
		TransactionID:  pgtype.Text{String: "wx_tx_102", Valid: true},
		OutSubsidyNo:   "S-102-302",
		Amount:         300,
		Status:         "success",
		WxpaySubsidyID: pgtype.Text{String: "wx_subsidy_202", Valid: true},
		OutReturnNo:    pgtype.Text{String: "SR-102", Valid: true},
		ReturnAmount:   pgtype.Int8{Int64: 100, Valid: true},
		ReturnStatus:   pgtype.Text{String: "return_failed", Valid: true},
	}
	updated := subsidyOrder
	updated.ReturnStatus = pgtype.Text{String: "return_success", Valid: true}
	updated.ReturnWxpayID = pgtype.Text{String: "wx_return_202", Valid: true}

	store.EXPECT().
		GetSubsidyOrderByPaymentOrderID(gomock.Any(), int64(102)).
		Return(subsidyOrder, nil)
	ecommerce.EXPECT().
		ReturnSubsidy(gomock.Any(), wechatcontracts.SubsidyReturnRequest{
			SubMchID:      "sub_mch_102",
			OutOrderNo:    "SR-102",
			TransactionID: "wx_tx_102",
			RefundID:      "wx_refund_102",
			Amount:        100,
			Description:   "退款退回补差",
			SubsidyID:     "wx_subsidy_202",
		}).
		Return(&wechatcontracts.SubsidyReturnResponse{
			SubsidyRefundID: "wx_return_202",
			Result:          wechatcontracts.SubsidyResultSuccess,
		}, nil)
	store.EXPECT().
		UpdateSubsidyReturnToSuccess(gomock.Any(), db.UpdateSubsidyReturnToSuccessParams{
			OutReturnNo:   pgtype.Text{String: "SR-102", Valid: true},
			ReturnWxpayID: pgtype.Text{String: "wx_return_202", Valid: true},
		}).
		Return(updated, nil)
	expectSubsidyCommandAccepted(t, store, db.ExternalPaymentCommandTypeReturnSubsidy, db.ExternalPaymentObjectSubsidyReturn, "SR-102", "wx_return_202", updated.ID, map[string]string{
		"out_return_no":     "SR-102",
		"out_subsidy_no":    "S-102-302",
		"sub_mchid":         "sub_mch_102",
		"transaction_id":    "wx_tx_102",
		"subsidy_refund_id": "wx_return_202",
		"result":            wechatcontracts.SubsidyResultSuccess,
		"amount":            "100",
	}, "退款退回补差")

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	request, err := http.NewRequest(http.MethodPost, "/v1/operators/me/payment-orders/102/subsidies/return", strings.NewReader(`{"refund_id":"wx_refund_102","amount":100,"description":"退款退回补差"}`))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	ctx.Request = request
	ctx.Params = gin.Params{{Key: "id", Value: "102"}}

	server.returnSubsidy(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response subsidyOrderResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.NotNil(t, response.ReturnStatus)
	require.Equal(t, "return_success", *response.ReturnStatus)
	require.NotNil(t, response.ReturnWxpayID)
	require.Equal(t, "wx_return_202", *response.ReturnWxpayID)
}

func TestCancelSubsidy_NilResponseDoesNotPanic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	subsidyOrder := db.SubsidyOrder{
		ID:             203,
		PaymentOrderID: 103,
		SubMchID:       "sub_mch_103",
		TransactionID:  pgtype.Text{String: "wx_tx_103", Valid: true},
		OutSubsidyNo:   "S-103-303",
		Status:         "pending",
	}

	store.EXPECT().
		GetSubsidyOrderByPaymentOrderID(gomock.Any(), int64(103)).
		Return(subsidyOrder, nil)
	ecommerce.EXPECT().
		CancelSubsidy(gomock.Any(), wechatcontracts.SubsidyCancelRequest{
			SubMchID:      "sub_mch_103",
			TransactionID: "wx_tx_103",
			Description:   "operator cancel",
		}).
		Return(nil, nil)

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	request, err := http.NewRequest(http.MethodPost, "/v1/operators/me/payment-orders/103/subsidies/cancel", nil)
	require.NoError(t, err)
	ctx.Request = request
	ctx.Params = gin.Params{{Key: "id", Value: "103"}}

	server.cancelSubsidy(ctx)

	require.Equal(t, http.StatusConflict, recorder.Code)
}

func TestCancelSubsidy_RecordsAcceptedCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	subsidyOrder := db.SubsidyOrder{
		ID:             204,
		PaymentOrderID: 104,
		SubMchID:       "sub_mch_104",
		TransactionID:  pgtype.Text{String: "wx_tx_104", Valid: true},
		OutSubsidyNo:   "S-104-304",
		Amount:         200,
		Status:         "pending",
	}
	updated := subsidyOrder
	updated.Status = "canceled"

	store.EXPECT().
		GetSubsidyOrderByPaymentOrderID(gomock.Any(), int64(104)).
		Return(subsidyOrder, nil)
	ecommerce.EXPECT().
		CancelSubsidy(gomock.Any(), wechatcontracts.SubsidyCancelRequest{
			SubMchID:      "sub_mch_104",
			TransactionID: "wx_tx_104",
			Description:   "operator cancel",
		}).
		Return(&wechatcontracts.SubsidyCancelResponse{Result: wechatcontracts.SubsidyResultSuccess}, nil)
	store.EXPECT().
		UpdateSubsidyOrderToCanceled(gomock.Any(), subsidyOrder.ID).
		Return(updated, nil)
	expectSubsidyCommandAccepted(t, store, db.ExternalPaymentCommandTypeCancelSubsidy, db.ExternalPaymentObjectSubsidy, "S-104-304", "", updated.ID, map[string]string{
		"out_subsidy_no": "S-104-304",
		"sub_mchid":      "sub_mch_104",
		"transaction_id": "wx_tx_104",
		"result":         wechatcontracts.SubsidyResultSuccess,
	}, "operator cancel")

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	request, err := http.NewRequest(http.MethodPost, "/v1/operators/me/payment-orders/104/subsidies/cancel", nil)
	require.NoError(t, err)
	ctx.Request = request
	ctx.Params = gin.Params{{Key: "id", Value: "104"}}

	server.cancelSubsidy(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response subsidyOrderResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, "canceled", response.Status)
}

func TestCreateSubsidy_WechatFailureReturnsBadGatewayMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{
		ID:            104,
		Status:        PaymentStatusPaid,
		TransactionID: pgtype.Text{String: "wx_tx_104", Valid: true},
	}
	created := db.SubsidyOrder{
		ID:             204,
		PaymentOrderID: paymentOrder.ID,
		SubMchID:       "sub_mch_104",
		TransactionID:  paymentOrder.TransactionID,
		OutSubsidyNo:   "S-104-304",
		PayerAmount:    1200,
		Amount:         200,
		Description:    "平台补差",
		Status:         "pending",
	}

	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), int64(304)).Return(db.MerchantPaymentConfig{MerchantID: 304, SubMchID: "sub_mch_104"}, nil)
	store.EXPECT().GetSubsidyOrderByOutSubsidyNo(gomock.Any(), "S-104-304").Return(db.SubsidyOrder{}, db.ErrRecordNotFound)
	store.EXPECT().CreateSubsidyOrder(gomock.Any(), db.CreateSubsidyOrderParams{
		PaymentOrderID: paymentOrder.ID,
		SubMchID:       "sub_mch_104",
		TransactionID:  paymentOrder.TransactionID,
		OutSubsidyNo:   "S-104-304",
		PayerAmount:    1200,
		Amount:         200,
		Description:    "平台补差",
	}).Return(created, nil)
	ecommerce.EXPECT().CreateSubsidy(gomock.Any(), wechatcontracts.SubsidyRequest{
		SubMchID:      "sub_mch_104",
		TransactionID: "wx_tx_104",
		Amount:        200,
		Description:   "平台补差",
		OutSubsidyNo:  "S-104-304",
	}).Return(nil, errors.New("wechat unavailable"))
	store.EXPECT().UpdateSubsidyOrderToFailed(gomock.Any(), db.UpdateSubsidyOrderToFailedParams{
		ID:         created.ID,
		FailReason: pgtype.Text{String: "wechat unavailable", Valid: true},
	}).Return(created, nil)

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	request, err := http.NewRequest(http.MethodPost, "/v1/operators/me/payment-orders/104/subsidies", strings.NewReader(`{"merchant_id":304,"payer_amount":1200,"amount":200,"description":"平台补差"}`))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	ctx.Request = request
	ctx.Params = gin.Params{{Key: "id", Value: "104"}}

	server.createSubsidy(ctx)

	require.Equal(t, http.StatusBadGateway, recorder.Code)
	var response ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "微信补差接口暂不可用；普通服务商模式不支持补差，请联系平台管理员确认该入口是否应下线", response.Error)
}

func TestCreateSubsidy_MarkFailedErrorReturnsInternalServerError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{
		ID:            105,
		Status:        PaymentStatusPaid,
		TransactionID: pgtype.Text{String: "wx_tx_105", Valid: true},
	}
	created := db.SubsidyOrder{
		ID:             205,
		PaymentOrderID: paymentOrder.ID,
		SubMchID:       "sub_mch_105",
		TransactionID:  paymentOrder.TransactionID,
		OutSubsidyNo:   "S-105-305",
		PayerAmount:    1200,
		Amount:         200,
		Description:    "平台补差",
		Status:         "pending",
	}

	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), int64(305)).Return(db.MerchantPaymentConfig{MerchantID: 305, SubMchID: "sub_mch_105"}, nil)
	store.EXPECT().GetSubsidyOrderByOutSubsidyNo(gomock.Any(), "S-105-305").Return(db.SubsidyOrder{}, db.ErrRecordNotFound)
	store.EXPECT().CreateSubsidyOrder(gomock.Any(), db.CreateSubsidyOrderParams{
		PaymentOrderID: paymentOrder.ID,
		SubMchID:       "sub_mch_105",
		TransactionID:  paymentOrder.TransactionID,
		OutSubsidyNo:   "S-105-305",
		PayerAmount:    1200,
		Amount:         200,
		Description:    "平台补差",
	}).Return(created, nil)
	ecommerce.EXPECT().CreateSubsidy(gomock.Any(), wechatcontracts.SubsidyRequest{
		SubMchID:      "sub_mch_105",
		TransactionID: "wx_tx_105",
		Amount:        200,
		Description:   "平台补差",
		OutSubsidyNo:  "S-105-305",
	}).Return(nil, errors.New("wechat unavailable"))
	store.EXPECT().UpdateSubsidyOrderToFailed(gomock.Any(), db.UpdateSubsidyOrderToFailedParams{
		ID:         created.ID,
		FailReason: pgtype.Text{String: "wechat unavailable", Valid: true},
	}).Return(db.SubsidyOrder{}, errors.New("write failed"))

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	request, err := http.NewRequest(http.MethodPost, "/v1/operators/me/payment-orders/105/subsidies", strings.NewReader(`{"merchant_id":305,"payer_amount":1200,"amount":200,"description":"平台补差"}`))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	ctx.Request = request
	ctx.Params = gin.Params{{Key: "id", Value: "105"}}

	server.createSubsidy(ctx)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	var response ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "internal server error", response.Error)
}

func TestReturnSubsidy_MarkFailedErrorReturnsInternalServerError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerce := mockwechat.NewMockEcommerceClientInterface(ctrl)

	subsidyOrder := db.SubsidyOrder{
		ID:             206,
		PaymentOrderID: 106,
		SubMchID:       "sub_mch_106",
		TransactionID:  pgtype.Text{String: "wx_tx_106", Valid: true},
		OutSubsidyNo:   "S-106-306",
		Amount:         300,
		Status:         "success",
		WxpaySubsidyID: pgtype.Text{String: "wx_subsidy_206", Valid: true},
		OutReturnNo:    pgtype.Text{String: "SR-106", Valid: true},
		ReturnAmount:   pgtype.Int8{Int64: 100, Valid: true},
		ReturnStatus:   pgtype.Text{String: "return_failed", Valid: true},
	}

	store.EXPECT().GetSubsidyOrderByPaymentOrderID(gomock.Any(), int64(106)).Return(subsidyOrder, nil)
	ecommerce.EXPECT().ReturnSubsidy(gomock.Any(), wechatcontracts.SubsidyReturnRequest{
		SubMchID:      "sub_mch_106",
		OutOrderNo:    "SR-106",
		TransactionID: "wx_tx_106",
		RefundID:      "wx_refund_106",
		Amount:        100,
		Description:   "退款退回补差",
		SubsidyID:     "wx_subsidy_206",
	}).Return(nil, errors.New("wechat unavailable"))
	store.EXPECT().UpdateSubsidyReturnToFailed(gomock.Any(), db.UpdateSubsidyReturnToFailedParams{
		OutReturnNo:      pgtype.Text{String: "SR-106", Valid: true},
		ReturnFailReason: pgtype.Text{String: "wechat unavailable", Valid: true},
	}).Return(db.SubsidyOrder{}, errors.New("write failed"))

	server := newTestServer(t, store)
	server.SetEcommerceClientForTest(ecommerce)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	request, err := http.NewRequest(http.MethodPost, "/v1/operators/me/payment-orders/106/subsidies/return", strings.NewReader(`{"refund_id":"wx_refund_106","amount":100,"description":"退款退回补差"}`))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	ctx.Request = request
	ctx.Params = gin.Params{{Key: "id", Value: "106"}}

	server.returnSubsidy(ctx)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	var response ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "internal server error", response.Error)
}
