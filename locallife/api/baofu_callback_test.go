package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	baofunotification "github.com/merrydance/locallife/baofu/account/notification"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	baofuaggregatenotification "github.com/merrydance/locallife/baofu/aggregatepay/notification"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
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

func TestBaofuWithdrawCallbackEnqueuesFactApplicationBeforeAck(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAccountNotificationParserForTest(fakeBaofuOpenAccountParser{})
	taskRecorder := &baofuWithdrawalFactApplicationEnqueueRecorder{}
	server.taskDistributor = taskRecorder

	withdrawal := db.BaofuWithdrawalOrder{
		ID:           6101,
		OutRequestNo: "WD202605040001",
		Status:       db.BaofuWithdrawalStatusProcessing,
	}
	store.EXPECT().GetBaofuWithdrawalOrderByOutRequestNo(gomock.Any(), "WD202605040001").Return(withdrawal, nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/withdraw", bytes.NewBufferString(`{"encrypted":true}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "OK", recorder.Body.String())
	require.Equal(t, []int64{withdrawal.ID}, taskRecorder.withdrawalOrderIDs)
	require.Equal(t, []string{"3"}, taskRecorder.upstreamStates)
	require.Equal(t, []string{"BFWD202605040001"}, taskRecorder.baofuWithdrawNos)
}

func TestBaofuAccountCallbackPayloadUsesRawQueryWhenBodyEmpty(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/account/open?member_id=102004465&terminal_id=200005200&data_type=JSON&data_content=ciphertext", http.NoBody)
	ctx := &gin.Context{Request: request}

	payload := baofuAccountCallbackPayload(ctx, nil)

	require.Equal(t, "member_id=102004465&terminal_id=200005200&data_type=JSON&data_content=ciphertext", string(payload))
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
	require.Equal(t, "OK", recorder.Body.String())
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
	require.Equal(t, "OK", recorder.Body.String())
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

func TestBaofuRefundCallbackPersistsFactAndEnqueuesApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.SetBaofuAggregatePaymentNotificationParserForTest(fakeBaofuPaymentParser{})
	taskRecorder := &refundFactApplicationEnqueueRecorder{}
	server.taskDistributor = taskRecorder

	refundOrder := db.RefundOrder{
		ID:             5101,
		PaymentOrderID: 4001,
		OutRefundNo:    "BFRFD_5101",
		RefundAmount:   1200,
		Status:         "processing",
	}
	paymentOrder := db.PaymentOrder{
		ID:             4001,
		OrderID:        pgtype.Int8{Int64: 3101, Valid: true},
		Amount:         1200,
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		BusinessType:   db.ExternalPaymentBusinessOwnerOrder,
	}
	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), "BFRFD_5101").Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), refundOrder.PaymentOrderID).Return(paymentOrder, nil)
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, db.ExternalPaymentProviderBaofu, arg.Provider)
			require.Equal(t, db.PaymentChannelBaofuAggregate, arg.Channel)
			require.Equal(t, db.ExternalPaymentCapabilityBaofuRefund, arg.Capability)
			require.Equal(t, db.ExternalPaymentFactSourceCallback, arg.FactSource)
			require.Equal(t, db.ExternalPaymentObjectRefund, arg.ExternalObjectType)
			require.Equal(t, refundOrder.OutRefundNo, arg.ExternalObjectKey)
			require.Equal(t, "BFREFUND_UP_5101", arg.ExternalSecondaryKey.String)
			require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
			require.True(t, arg.IsTerminal)
			require.Equal(t, refundOrder.RefundAmount, arg.Amount.Int64)
			require.Equal(t, db.ExternalPaymentBusinessOwnerOrder, arg.BusinessOwner.String)
			require.Equal(t, paymentFactBusinessObjectRefundOrder, arg.BusinessObjectType.String)
			require.Equal(t, refundOrder.ID, arg.BusinessObjectID.Int64)
			require.Equal(t, "baofu:callback:refund:BFRFD_5101:BFRN_5101", arg.DedupeKey)
			return db.ExternalPaymentFact{ID: 901, IsTerminal: true, DedupeKey: arg.DedupeKey}, nil
		})
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             901,
		Consumer:           paymentFactConsumerOrderDomain,
		BusinessObjectType: paymentFactBusinessObjectRefundOrder,
		BusinessObjectID:   refundOrder.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{ID: 1001, FactID: 901, Consumer: paymentFactConsumerOrderDomain, BusinessObjectType: paymentFactBusinessObjectRefundOrder, BusinessObjectID: refundOrder.ID}, nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/baofu/refund", bytes.NewBufferString(`{"notifyId":"BFRN_5101"}`))
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "OK", recorder.Body.String())
	require.Equal(t, []int64{1001}, taskRecorder.applicationIDs)
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

func (fakeBaofuOpenAccountParser) ParseWithdrawNotification(body []byte) (*baofunotification.WithdrawNotification, error) {
	return &baofunotification.WithdrawNotification{
		TransSerialNo:   "WD202605040001",
		BaofuWithdrawNo: "BFWD202605040001",
		ContractNo:      "CM_BCT_123",
		UpstreamState:   "3",
		Status:          db.BaofuWithdrawalStatusReturned,
		AmountFen:       12345,
		FeeFen:          100,
		TotalAmountFen:  12445,
		OccurredAt:      time.Now().UTC(),
		Raw:             []byte(`{"transSerialNo":"WD202605040001","orderId":"BFWD202605040001","state":"3"}`),
	}, nil
}

type baofuWithdrawalFactApplicationEnqueueRecorder struct {
	worker.NoopTaskDistributor
	withdrawalOrderIDs []int64
	upstreamStates     []string
	baofuWithdrawNos   []string
}

func (r *baofuWithdrawalFactApplicationEnqueueRecorder) DistributeTaskProcessBaofuWithdrawalFactApplication(_ context.Context, payload *worker.BaofuWithdrawalFactApplicationPayload, _ ...asynq.Option) error {
	r.withdrawalOrderIDs = append(r.withdrawalOrderIDs, payload.WithdrawalOrderID)
	r.upstreamStates = append(r.upstreamStates, payload.UpstreamState)
	r.baofuWithdrawNos = append(r.baofuWithdrawNos, payload.BaofuWithdrawNo)
	return nil
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

func (fakeBaofuPaymentParser) ParseRefundNotification(body []byte) (*baofuaggregatenotification.RefundNotification, error) {
	return &baofuaggregatenotification.RefundNotification{
		NotifyID:       "BFRN_5101",
		NotifyType:     "REFUND.SUCCESS",
		TerminalStatus: db.ExternalPaymentTerminalStatusSuccess,
		IsTerminal:     true,
		OccurredAt:     time.Now().UTC(),
		Raw:            []byte(`{"notifyId":"BFRN_5101","outTradeNo":"BFRFD_5101","tradeNo":"BFREFUND_UP_5101","refundState":"SUCCESS"}`),
		Fact: aggregatecontracts.RefundFact{
			OutTradeNo:       "BFRFD_5101",
			TradeNo:          "BFREFUND_UP_5101",
			TransactionState: aggregatecontracts.RefundStateSuccess,
			SuccessAmountFen: 1200,
			Raw:              []byte(`{"notifyId":"BFRN_5101","outTradeNo":"BFRFD_5101","tradeNo":"BFREFUND_UP_5101","refundState":"SUCCESS"}`),
		},
	}, nil
}

var _ = gin.TestMode
