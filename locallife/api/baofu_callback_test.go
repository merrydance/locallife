package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	baofunotification "github.com/merrydance/locallife/baofu/account/notification"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	baofuaggregatenotification "github.com/merrydance/locallife/baofu/aggregatepay/notification"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestBaofuAccountOpenCallbackPersistsFactBeforeAck(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAccountNotificationParserForTest(fakeBaofuOpenAccountParser{})

	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, db.ExternalPaymentProviderBaofu, arg.Provider)
			require.Equal(t, db.PaymentChannelBaofuAggregate, arg.Channel)
			require.Equal(t, db.ExternalPaymentCapabilityBaofuAccount, arg.Capability)
			require.Equal(t, db.ExternalPaymentFactSourceCallback, arg.FactSource)
			require.Equal(t, "OPEN123", arg.ExternalObjectKey)
			require.Equal(t, "baofu:callback:account:OPEN123:1", arg.DedupeKey)
			require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
			require.True(t, arg.IsTerminal)
			require.Equal(t, pgtype.Text{String: db.ExternalPaymentBusinessOwnerApplyment, Valid: true}, arg.BusinessOwner)
			require.False(t, arg.BusinessObjectType.Valid)
			require.False(t, arg.BusinessObjectID.Valid)
			return db.ExternalPaymentFact{ID: 88, DedupeKey: arg.DedupeKey}, nil
		})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/account/open", bytes.NewBufferString(`{"encrypted":true}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "OK", recorder.Body.String())
	require.Equal(t, "text/plain; charset=utf-8", recorder.Header().Get("Content-Type"))
}

func TestBaofuPaymentCallbackPersistsFactAndEnqueuesApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAggregatePaymentNotificationParserForTest(fakeBaofuPaymentParser{})
	taskRecorder := &refundFactApplicationEnqueueRecorder{}
	server.taskDistributor = taskRecorder

	paymentOrder := db.PaymentOrder{
		ID:             4001,
		OutTradeNo:     "PO_BAOFU_4001",
		Amount:         1200,
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		BusinessType:   "order",
	}
	store.EXPECT().GetPaymentOrderByOutTradeNo(gomock.Any(), "PO_BAOFU_4001").Return(paymentOrder, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, db.ExternalPaymentProviderBaofu, arg.Provider)
			require.Equal(t, db.PaymentChannelBaofuAggregate, arg.Channel)
			require.Equal(t, db.ExternalPaymentCapabilityBaofuPayment, arg.Capability)
			require.Equal(t, db.ExternalPaymentFactSourceCallback, arg.FactSource)
			require.Equal(t, db.ExternalPaymentObjectBaofuPaymentOrder, arg.ExternalObjectType)
			require.Equal(t, "PO_BAOFU_4001", arg.ExternalObjectKey)
			require.Equal(t, "BFPAY_4001", arg.ExternalSecondaryKey.String)
			require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
			require.True(t, arg.IsTerminal)
			require.Equal(t, int64(1200), arg.Amount.Int64)
			require.Equal(t, "baofu:callback:payment:PO_BAOFU_4001:BFN_4001", arg.DedupeKey)
			return db.ExternalPaymentFact{ID: 501, IsTerminal: true, DedupeKey: arg.DedupeKey}, nil
		})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             501,
		Consumer:           paymentFactConsumerOrderDomain,
		BusinessObjectType: paymentFactBusinessObjectPaymentOrder,
		BusinessObjectID:   paymentOrder.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{ID: 601, FactID: 501, Consumer: paymentFactConsumerOrderDomain, BusinessObjectType: paymentFactBusinessObjectPaymentOrder, BusinessObjectID: paymentOrder.ID}, nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/payment", bytes.NewBufferString(`{"notifyId":"BFN_4001"}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "SUCCESS")
	require.Equal(t, []int64{601}, taskRecorder.applicationIDs)
}

func TestBaofuShareCallbackPersistsFactAndEnqueuesApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAggregatePaymentNotificationParserForTest(fakeBaofuPaymentParser{})
	taskRecorder := &refundFactApplicationEnqueueRecorder{}
	server.taskDistributor = taskRecorder

	profitSharingOrder := db.ProfitSharingOrder{
		ID:             3001,
		PaymentOrderID: 4001,
		OutOrderNo:     "BFSHARE_3001",
		Status:         db.ProfitSharingOrderStatusProcessing,
		MerchantAmount: 8970,
		RiderAmount:    500,
	}
	store.EXPECT().GetProfitSharingOrderByOutOrderNo(gomock.Any(), "BFSHARE_3001").Return(profitSharingOrder, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, db.ExternalPaymentProviderBaofu, arg.Provider)
			require.Equal(t, db.PaymentChannelBaofuAggregate, arg.Channel)
			require.Equal(t, db.ExternalPaymentCapabilityBaofuProfitSharing, arg.Capability)
			require.Equal(t, db.ExternalPaymentFactSourceCallback, arg.FactSource)
			require.Equal(t, db.ExternalPaymentObjectProfitSharing, arg.ExternalObjectType)
			require.Equal(t, "BFSHARE_3001", arg.ExternalObjectKey)
			require.Equal(t, "BFSHARE_UP_3001", arg.ExternalSecondaryKey.String)
			require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
			require.True(t, arg.IsTerminal)
			require.Equal(t, int64(9470), arg.Amount.Int64)
			require.Equal(t, "baofu:callback:profit_sharing:BFSHARE_3001:BFSN_3001", arg.DedupeKey)
			return db.ExternalPaymentFact{ID: 701, IsTerminal: true, DedupeKey: arg.DedupeKey}, nil
		})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             701,
		Consumer:           paymentFactConsumerProfitSharingDomain,
		BusinessObjectType: paymentFactBusinessObjectProfitSharingOrder,
		BusinessObjectID:   profitSharingOrder.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{ID: 801, FactID: 701, Consumer: paymentFactConsumerProfitSharingDomain, BusinessObjectType: paymentFactBusinessObjectProfitSharingOrder, BusinessObjectID: profitSharingOrder.ID}, nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/share", bytes.NewBufferString(`{"notifyId":"BFSN_3001"}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "SUCCESS")
	require.Equal(t, []int64{801}, taskRecorder.applicationIDs)
}

func TestBaofuShareCallbackUsesDefaultParser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	taskRecorder := &refundFactApplicationEnqueueRecorder{}
	server.taskDistributor = taskRecorder

	profitSharingOrder := db.ProfitSharingOrder{ID: 3002, OutOrderNo: "BFSHARE_3002", Status: db.ProfitSharingOrderStatusProcessing}
	store.EXPECT().GetProfitSharingOrderByOutOrderNo(gomock.Any(), "BFSHARE_3002").Return(profitSharingOrder, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).DoAndReturn(func(_ any, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, "BFSHARE_3002", arg.ExternalObjectKey)
		require.Equal(t, "BFSHARE_UP_3002", arg.ExternalSecondaryKey.String)
		require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
		return db.ExternalPaymentFact{ID: 702, IsTerminal: true}, nil
	})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), gomock.Any()).Return(db.ExternalPaymentFactApplication{
		ID:                 802,
		FactID:             702,
		Consumer:           paymentFactConsumerProfitSharingDomain,
		BusinessObjectType: paymentFactBusinessObjectProfitSharingOrder,
		BusinessObjectID:   profitSharingOrder.ID,
	}, nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/share", bytes.NewBufferString(`{"notifyId":"BFSN_3002","notifyType":"SHARE.SUCCESS","outTradeNo":"BFSHARE_3002","tradeNo":"BFSHARE_UP_3002","txnState":"SUCCESS","succAmt":9470}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, []int64{802}, taskRecorder.applicationIDs)
}

type fakeBaofuOpenAccountParser struct{}

func (fakeBaofuOpenAccountParser) ParseOpenAccountNotification(body []byte) (*baofunotification.AccountNotification, error) {
	return &baofunotification.AccountNotification{
		OutRequestNo:  "OPEN123",
		ContractNo:    "CM_BCT_123",
		UpstreamState: "1",
		OpenState:     db.BaofuAccountOpenStateActive,
		OccurredAt:    time.Now().UTC(),
		Raw:           []byte(`{"transSerialNo":"OPEN123","state":"1","contractNo":"CM_BCT_123"}`),
	}, nil
}

type fakeBaofuPaymentParser struct{}

func (fakeBaofuPaymentParser) ParsePaymentNotification(body []byte) (*baofuaggregatenotification.PaymentNotification, error) {
	return &baofuaggregatenotification.PaymentNotification{
		NotifyID:       "BFN_4001",
		NotifyType:     "PAYMENT.SUCCESS",
		TerminalStatus: db.ExternalPaymentTerminalStatusSuccess,
		IsTerminal:     true,
		OccurredAt:     time.Now().UTC(),
		Raw:            []byte(`{"notifyId":"BFN_4001","outTradeNo":"PO_BAOFU_4001","tradeNo":"BFPAY_4001","txnState":"SUCCESS"}`),
		Fact: aggregatecontracts.PaymentFact{
			OutTradeNo:       "PO_BAOFU_4001",
			TradeNo:          "BFPAY_4001",
			TransactionState: aggregatecontracts.PaymentStateSuccess,
			SuccessAmountFen: 1200,
			FeeAmountFen:     4,
			Raw:              []byte(`{"notifyId":"BFN_4001","outTradeNo":"PO_BAOFU_4001","tradeNo":"BFPAY_4001","txnState":"SUCCESS"}`),
		},
	}, nil
}

func (fakeBaofuPaymentParser) ParseShareNotification(body []byte) (*baofuaggregatenotification.ShareNotification, error) {
	return &baofuaggregatenotification.ShareNotification{
		NotifyID:       "BFSN_3001",
		NotifyType:     "SHARE.SUCCESS",
		TerminalStatus: db.ExternalPaymentTerminalStatusSuccess,
		IsTerminal:     true,
		OccurredAt:     time.Now().UTC(),
		Raw:            []byte(`{"notifyId":"BFSN_3001","outTradeNo":"BFSHARE_3001","tradeNo":"BFSHARE_UP_3001","txnState":"SUCCESS"}`),
		Fact: aggregatecontracts.ShareFact{
			OutTradeNo:       "BFSHARE_3001",
			TradeNo:          "BFSHARE_UP_3001",
			TransactionState: aggregatecontracts.ShareStateSuccess,
			SuccessAmountFen: 9470,
			Raw:              []byte(`{"notifyId":"BFSN_3001","outTradeNo":"BFSHARE_3001","tradeNo":"BFSHARE_UP_3001","txnState":"SUCCESS"}`),
		},
	}, nil
}

var _ = gin.TestMode
