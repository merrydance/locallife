package api

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestOrderShippingSettlementNotifyVerifiesMiniProgramMessageURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	server := newTestServer(t, mockdb.NewMockStore(ctrl))
	server.config.WechatMiniAppMessageToken = "mini-token"

	timestamp := "1710000000"
	nonce := "nonce-verify"
	signature := signMiniProgramCallback(server.config.WechatMiniAppMessageToken, timestamp, nonce)
	requestURL := fmt.Sprintf("/v1/webhooks/wechat-miniprogram/settlement-notify?signature=%s&timestamp=%s&nonce=%s&echostr=hello-wechat", signature, timestamp, nonce)
	request := httptest.NewRequest(http.MethodGet, requestURL, nil)
	recorder := httptest.NewRecorder()

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "hello-wechat", recorder.Body.String())
}

func TestOrderShippingSettlementNotifyRejectsInvalidMiniProgramSignature(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	server := newTestServer(t, mockdb.NewMockStore(ctrl))
	server.config.WechatMiniAppMessageToken = "mini-token"

	body := miniProgramSettlementXML("WXTXN202605230001", "BF202605230001", 1769169600)
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks/wechat-miniprogram/settlement-notify?signature=bad&timestamp=1710000000&nonce=nonce", bytes.NewBufferString(body))
	recorder := httptest.NewRecorder()

	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Equal(t, "invalid signature", recorder.Body.String())
}

func TestOrderShippingSettlementNotifySkipsShipmentPushWithoutProfitSharing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	server := newTestServer(t, store)
	server.config.WechatMiniAppMessageToken = "mini-token"
	taskDistributor := &settlementProfitSharingTaskRecorder{}
	server.SetTaskDistributorForTest(taskDistributor)

	body := `<xml>
<ToUserName><![CDATA[gh_test]]></ToUserName>
<FromUserName><![CDATA[o_user]]></FromUserName>
<CreateTime>1769169000</CreateTime>
<MsgType><![CDATA[event]]></MsgType>
<Event><![CDATA[trade_manage_order_settlement]]></Event>
<transaction_id><![CDATA[WXTXN202605230001]]></transaction_id>
<merchant_id><![CDATA[1900000109]]></merchant_id>
<sub_merchant_id><![CDATA[1900000118]]></sub_merchant_id>
<merchant_trade_no><![CDATA[BF202605230001]]></merchant_trade_no>
<pay_time>1769165000</pay_time>
<shipped_time>1769169000</shipped_time>
<estimated_settlement_time>1769255400</estimated_settlement_time>
</xml>`
	request := newMiniProgramSettlementRequest(t, server.config.WechatMiniAppMessageToken, body)
	recorder := httptest.NewRecorder()

	server.router.ServeHTTP(recorder, request)

	assertWechatNoContentResponse(t, recorder)
	require.Empty(t, taskDistributor.profitSharingOrderIDs)
}

func TestOrderShippingSettlementNotifyProcessesSettlementMessagePush(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	server := newTestServer(t, store)
	server.config.WechatMiniAppMessageToken = "mini-token"
	taskDistributor := &settlementProfitSharingTaskRecorder{}
	server.SetTaskDistributorForTest(taskDistributor)

	transactionID := "WXTXN202605230002"
	merchantTradeNo := "BF202605230002"
	paymentOrder := db.PaymentOrder{
		ID:                    7102,
		OrderID:               pgtype.Int8{Int64: 8102, Valid: true},
		BusinessType:          db.ExternalPaymentBusinessOwnerOrder,
		Amount:                10300,
		OutTradeNo:            merchantTradeNo,
		TransactionID:         pgtype.Text{String: transactionID, Valid: true},
		Status:                "paid",
		PaymentChannel:        db.PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
	}
	profitSharingOrder := db.ProfitSharingOrder{
		ID:                   9102,
		PaymentOrderID:       paymentOrder.ID,
		MerchantID:           6102,
		OrderSource:          db.OrderTypeTakeout,
		TotalAmount:          paymentOrder.Amount,
		OutOrderNo:           "BFPS7102O8102",
		Status:               db.ProfitSharingOrderStatusPending,
		Provider:             db.ExternalPaymentProviderBaofu,
		Channel:              db.PaymentChannelBaofuAggregate,
		RiderID:              pgtype.Int8{Int64: 5102, Valid: true},
		RiderGrossAmount:     800,
		RiderPaymentFee:      5,
		RiderAmount:          795,
		MerchantSharingMerID: pgtype.Text{String: "merchant-sharing", Valid: true},
		RiderSharingMerID:    pgtype.Text{String: "rider-sharing", Valid: true},
		PlatformSharingMerID: pgtype.Text{String: "platform-sharing", Valid: true},
	}

	expectedEventID := "mpmsg:WXTXN202605230002:1769169600"
	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateWechatNotificationParams) (bool, error) {
			require.Equal(t, expectedEventID, arg.ID)
			require.Equal(t, wechatcontracts.OrderShippingEventType, arg.EventType)
			require.Equal(t, pgtype.Text{String: "mini-program-message", Valid: true}, arg.ResourceType)
			require.Equal(t, pgtype.Text{String: "order shipping settlement message", Valid: true}, arg.Summary)
			return true, nil
		})
	store.EXPECT().
		GetPaymentOrderByTransactionId(gomock.Any(), pgtype.Text{String: transactionID, Valid: true}).
		Return(paymentOrder, nil)
	store.EXPECT().
		GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).
		Return(profitSharingOrder, nil)
	store.EXPECT().
		GetTotalRefundedByPaymentOrder(gomock.Any(), paymentOrder.ID).
		Return(int64(0), nil)
	store.EXPECT().
		CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, db.ExternalPaymentProviderWechat, arg.Provider)
			require.Equal(t, db.PaymentChannelDirect, arg.Channel)
			require.Equal(t, db.ExternalPaymentCapabilityBaofuProfitSharing, arg.Capability)
			require.Equal(t, db.ExternalPaymentFactSourceCallback, arg.FactSource)
			require.Equal(t, pgtype.Text{String: expectedEventID, Valid: true}, arg.SourceEventID)
			require.Equal(t, pgtype.Text{String: wechatcontracts.OrderShippingEventType, Valid: true}, arg.SourceEventType)
			require.Equal(t, transactionID, arg.ExternalObjectKey)
			require.Equal(t, pgtype.Text{String: merchantTradeNo, Valid: true}, arg.ExternalSecondaryKey)
			require.Equal(t, "SETTLED", arg.UpstreamState)
			require.Equal(t, "wechat:settlement:WXTXN202605230002:1769169600", arg.DedupeKey)
			return db.ExternalPaymentFact{ID: 101, DedupeKey: arg.DedupeKey}, nil
		})

	body := miniProgramSettlementXML(transactionID, merchantTradeNo, 1769169600)
	request := newMiniProgramSettlementRequest(t, server.config.WechatMiniAppMessageToken, body)
	recorder := httptest.NewRecorder()

	server.router.ServeHTTP(recorder, request)

	assertWechatNoContentResponse(t, recorder)
	require.Equal(t, []int64{profitSharingOrder.ID}, taskDistributor.profitSharingOrderIDs)
}

func TestOrderShippingSettlementNotifyProcessesJSONSettlementMessagePush(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	server := newTestServer(t, store)
	server.config.WechatMiniAppMessageToken = "mini-token"
	taskDistributor := &settlementProfitSharingTaskRecorder{}
	server.SetTaskDistributorForTest(taskDistributor)

	transactionID := "WXTXN202605230004"
	merchantTradeNo := "BF202605230004"
	settlementTime := int64(1769169600)
	expectedEventID := "mpmsg:WXTXN202605230004:1769169600"
	paymentOrder := db.PaymentOrder{
		ID:                    7104,
		OrderID:               pgtype.Int8{Int64: 8104, Valid: true},
		BusinessType:          db.ExternalPaymentBusinessOwnerOrder,
		Amount:                10300,
		OutTradeNo:            merchantTradeNo,
		TransactionID:         pgtype.Text{String: transactionID, Valid: true},
		Status:                "paid",
		PaymentChannel:        db.PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
	}
	profitSharingOrder := db.ProfitSharingOrder{
		ID:                   9104,
		PaymentOrderID:       paymentOrder.ID,
		MerchantID:           6104,
		OrderSource:          db.OrderTypeTakeout,
		TotalAmount:          paymentOrder.Amount,
		OutOrderNo:           "BFPS7104O8104",
		Status:               db.ProfitSharingOrderStatusPending,
		Provider:             db.ExternalPaymentProviderBaofu,
		Channel:              db.PaymentChannelBaofuAggregate,
		RiderID:              pgtype.Int8{Int64: 5104, Valid: true},
		RiderGrossAmount:     800,
		RiderPaymentFee:      5,
		RiderAmount:          795,
		MerchantSharingMerID: pgtype.Text{String: "merchant-sharing", Valid: true},
		RiderSharingMerID:    pgtype.Text{String: "rider-sharing", Valid: true},
		PlatformSharingMerID: pgtype.Text{String: "platform-sharing", Valid: true},
	}

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateWechatNotificationParams) (bool, error) {
			require.Equal(t, expectedEventID, arg.ID)
			require.Equal(t, wechatcontracts.OrderShippingEventType, arg.EventType)
			return true, nil
		})
	store.EXPECT().
		GetPaymentOrderByTransactionId(gomock.Any(), pgtype.Text{String: transactionID, Valid: true}).
		Return(paymentOrder, nil)
	store.EXPECT().
		GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).
		Return(profitSharingOrder, nil)
	store.EXPECT().
		GetTotalRefundedByPaymentOrder(gomock.Any(), paymentOrder.ID).
		Return(int64(0), nil)
	store.EXPECT().
		CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, expectedEventID, arg.SourceEventID.String)
			require.Equal(t, transactionID, arg.ExternalObjectKey)
			require.Equal(t, pgtype.Text{String: merchantTradeNo, Valid: true}, arg.ExternalSecondaryKey)
			require.Equal(t, "wechat:settlement:WXTXN202605230004:1769169600", arg.DedupeKey)
			return db.ExternalPaymentFact{ID: 104, DedupeKey: arg.DedupeKey}, nil
		})

	body := miniProgramSettlementJSON(transactionID, merchantTradeNo, settlementTime)
	request := newMiniProgramSettlementRequest(t, server.config.WechatMiniAppMessageToken, body)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	server.router.ServeHTTP(recorder, request)

	assertWechatNoContentResponse(t, recorder)
	require.Equal(t, []int64{profitSharingOrder.ID}, taskDistributor.profitSharingOrderIDs)
}

func TestOrderShippingSettlementNotifyDuplicateSettlementMessageIsIdempotent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := newMockStoreWithAlertSink(ctrl)
	server := newTestServer(t, store)
	server.config.WechatMiniAppMessageToken = "mini-token"
	taskDistributor := &settlementProfitSharingTaskRecorder{}
	server.SetTaskDistributorForTest(taskDistributor)

	transactionID := "WXTXN202605230003"
	merchantTradeNo := "BF202605230003"
	expectedEventID := "mpmsg:WXTXN202605230003:1769169600"

	store.EXPECT().
		TryClaimWechatNotification(gomock.Any(), gomock.Any()).
		Return(false, nil)
	store.EXPECT().
		GetWechatNotification(gomock.Any(), expectedEventID).
		Return(db.WechatNotification{
			ID:          expectedEventID,
			ProcessedAt: pgtype.Timestamp{Time: time.Now(), Valid: true},
		}, nil)

	body := miniProgramSettlementXML(transactionID, merchantTradeNo, 1769169600)
	request := newMiniProgramSettlementRequest(t, server.config.WechatMiniAppMessageToken, body)
	recorder := httptest.NewRecorder()

	server.router.ServeHTTP(recorder, request)

	assertWechatNoContentResponse(t, recorder)
	require.Empty(t, taskDistributor.profitSharingOrderIDs)
}

func newMiniProgramSettlementRequest(t *testing.T, token string, body string) *http.Request {
	t.Helper()
	timestamp := "1710000000"
	nonce := "nonce-settle"
	signature := signMiniProgramCallback(token, timestamp, nonce)
	url := fmt.Sprintf("/v1/webhooks/wechat-miniprogram/settlement-notify?signature=%s&timestamp=%s&nonce=%s", signature, timestamp, nonce)
	request := httptest.NewRequest(http.MethodPost, url, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/xml")
	return request
}

func miniProgramSettlementXML(transactionID string, merchantTradeNo string, settlementTime int64) string {
	return fmt.Sprintf(`<xml>
<ToUserName><![CDATA[gh_test]]></ToUserName>
<FromUserName><![CDATA[o_user]]></FromUserName>
<CreateTime>1769169600</CreateTime>
<MsgType><![CDATA[event]]></MsgType>
<Event><![CDATA[trade_manage_order_settlement]]></Event>
<transaction_id><![CDATA[%s]]></transaction_id>
<merchant_id><![CDATA[1900000109]]></merchant_id>
<sub_merchant_id><![CDATA[1900000118]]></sub_merchant_id>
<merchant_trade_no><![CDATA[%s]]></merchant_trade_no>
<pay_time>1769165000</pay_time>
<shipped_time>1769169000</shipped_time>
<confirm_receive_method>1</confirm_receive_method>
<confirm_receive_time>%d</confirm_receive_time>
<settlement_time>%d</settlement_time>
</xml>`, transactionID, merchantTradeNo, settlementTime-60, settlementTime)
}

func miniProgramSettlementJSON(transactionID string, merchantTradeNo string, settlementTime int64) string {
	return fmt.Sprintf(`{
  "ToUserName": "gh_test",
  "FromUserName": "o_user",
  "CreateTime": 1769169600,
  "MsgType": "event",
  "Event": "trade_manage_order_settlement",
  "transaction_id": %q,
  "merchant_id": "1900000109",
  "sub_merchant_id": "1900000118",
  "merchant_trade_no": %q,
  "pay_time": 1769165000,
  "shipped_time": 1769169000,
  "confirm_receive_method": 1,
  "confirm_receive_time": %d,
  "settlement_time": %d
}`, transactionID, merchantTradeNo, settlementTime-60, settlementTime)
}
