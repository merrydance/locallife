package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	api "github.com/merrydance/locallife/api"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/scheduler"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type takeoutOrderResponse struct {
	ID        int64  `json:"id"`
	OrderType string `json:"order_type"`
	Status    string `json:"status"`
}

type takeoutDeliveryResponse struct {
	ID     int64  `json:"id"`
	Status string `json:"status"`
}

type takeoutPaymentOrderResponse struct {
	ID           int64  `json:"id"`
	Status       string `json:"status"`
	BusinessType string `json:"business_type"`
}

type dineInSessionResponse struct {
	Session struct {
		ID int64 `json:"id"`
	} `json:"session"`
}

type kitchenOrderStatusResponse struct {
	ID     int64  `json:"id"`
	Status string `json:"status"`
}

type reservationStatusResponse struct {
	ID     int64  `json:"id"`
	Status string `json:"status"`
}

type claimSubmitResponse struct {
	ClaimID        int64  `json:"claim_id"`
	Status         string `json:"status"`
	ApprovedAmount *int64 `json:"approved_amount"`
}

type appealSubmitResponse struct {
	ID            int64  `json:"id"`
	ClaimID       int64  `json:"claim_id"`
	AppellantType string `json:"appellant_type"`
	AppellantID   int64  `json:"appellant_id"`
	Status        string `json:"status"`
}

type claimRecoveryStatusResponse struct {
	ID             int64   `json:"id"`
	Status         string  `json:"status"`
	RecoveryTarget *string `json:"recovery_target"`
}

type roomAvailabilitySlot struct {
	Time      string `json:"time"`
	Available bool   `json:"available"`
}

type roomAvailabilityResponse struct {
	RoomID    int64                  `json:"room_id"`
	Date      string                 `json:"date"`
	TimeSlots []roomAvailabilitySlot `json:"time_slots"`
}

type scanTableResponse struct {
	Merchant struct {
		ID     int64  `json:"id"`
		Status string `json:"status"`
	} `json:"merchant"`
	Table struct {
		ID      int64  `json:"id"`
		TableNo string `json:"table_no"`
	} `json:"table"`
}

type deliveryRecommendResponse struct {
	OrderID    int64 `json:"order_id"`
	MerchantID int64 `json:"merchant_id"`
}

type combinedPaymentOrderResponse struct {
	ID                int64   `json:"id"`
	Status            string  `json:"status"`
	CombineOutTradeNo string  `json:"combine_out_trade_no"`
	TotalAmount       int64   `json:"total_amount"`
	PrepayID          *string `json:"prepay_id"`
	SubOrders         []struct {
		OrderID    int64  `json:"order_id"`
		SubMchID   string `json:"sub_mch_id"`
		OutTradeNo string `json:"out_trade_no"`
	} `json:"sub_orders"`
}

type listTodayReservationsResponse struct {
	Reservations []reservationStatusResponse `json:"reservations"`
}

type cartResponse struct {
	ID         int64 `json:"id"`
	MerchantID int64 `json:"merchant_id"`
	TotalCount int   `json:"total_count"`
	Subtotal   int64 `json:"subtotal"`
}

type calculateCartResponse struct {
	Subtotal       int64 `json:"subtotal"`
	DeliveryFee    int64 `json:"delivery_fee"`
	TotalAmount    int64 `json:"total_amount"`
	MinOrderAmount int64 `json:"min_order_amount"`
}

// TestTakeoutJourneyB1Integration
// 外卖旅程（B1）端到端验收：下单 -> 支付成功推进 -> 商户接单/出餐 -> 骑手抢单/取餐/配送/送达 -> 用户确认完成。
//
// 说明：支付回调与异步 worker 在 integration harness 中未配置（taskDistributor=nil），
// 这里用 store 事务直接模拟“支付成功后置处理”：
// - UpdatePaymentOrderToPaid
// - ProcessPaymentSuccessTx
func TestTakeoutJourneyB1Integration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)

	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err = store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	// 1) C端下单：/v1/orders
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
		require.Equal(t, "takeout", created.OrderType)
	}
	orderID := created.ID

	// 2) 创建支付单：/v1/payments（native 不依赖外部微信客户端）
	var payment takeoutPaymentOrderResponse
	{
		payBody := map[string]any{
			"order_id":      orderID,
			"payment_type":  "native",
			"business_type": "order",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/payments", payBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payment)
		require.Equal(t, "pending", payment.Status)
		require.Equal(t, "order", payment.BusinessType)
	}

	// 3) 模拟支付成功后置处理（创建 delivery / pool 并置 paid）
	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: "integration_tx_001", Valid: true},
	})
	require.NoError(t, err)

	payRes, err := store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     payment.ID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	})
	require.NoError(t, err)
	require.True(t, payRes.Processed)

	paidOrder, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "paid", paidOrder.Status)

	delivery, err := store.GetDeliveryByOrderID(ctx, orderID)
	require.NoError(t, err)
	poolItem, err := store.GetDeliveryPoolByOrderID(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, orderID, delivery.OrderID)
	require.Equal(t, orderID, poolItem.OrderID)

	// 3) 商户接单：/v1/merchant/orders/:id/accept
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/accept", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)

		var resp takeoutOrderResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, "preparing", resp.Status)
	}

	// 4) 商户出餐完成：/v1/merchant/orders/:id/ready
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/ready", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)

		var resp takeoutOrderResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, "ready", resp.Status)
	}

	// 5) 骑手抢单：/v1/delivery/grab/:order_id
	{
		url := fmt.Sprintf("/v1/delivery/grab/%d", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)

		var resp takeoutDeliveryResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, delivery.ID, resp.ID)
		require.Equal(t, "assigned", resp.Status)
	}

	delivery, err = store.GetDeliveryByOrderID(ctx, orderID)
	require.NoError(t, err)
	require.True(t, delivery.RiderID.Valid)
	require.Equal(t, rider.ID, delivery.RiderID.Int64)

	// 6) 骑手开始取餐：/v1/delivery/:delivery_id/start-pickup
	{
		url := fmt.Sprintf("/v1/delivery/%d/start-pickup", delivery.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)

		var resp takeoutDeliveryResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, "picking", resp.Status)
	}

	// 7) 骑手确认取餐：/v1/delivery/:delivery_id/confirm-pickup
	{
		url := fmt.Sprintf("/v1/delivery/%d/confirm-pickup", delivery.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)

		var resp takeoutDeliveryResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, "picked", resp.Status)
	}

	// 8) 骑手开始配送：/v1/delivery/:delivery_id/start-delivery
	{
		url := fmt.Sprintf("/v1/delivery/%d/start-delivery", delivery.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)

		var resp takeoutDeliveryResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, "delivering", resp.Status)
	}

	// 9) 骑手确认送达：/v1/delivery/:delivery_id/confirm-delivery
	{
		url := fmt.Sprintf("/v1/delivery/%d/confirm-delivery", delivery.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)

		var resp takeoutDeliveryResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, "delivered", resp.Status)
	}

	o, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "rider_delivered", o.Status)

	// 10) 用户确认收货：/v1/orders/:id/confirm
	{
		url := fmt.Sprintf("/v1/orders/%d/confirm", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)

		var resp takeoutOrderResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, "completed", resp.Status)
	}

	// 关键落库断言：订单完成、押金解冻
	o, err = store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", o.Status)

	updatedRider, err := store.GetRider(ctx, rider.ID)
	require.NoError(t, err)
	require.Equal(t, int64(0), updatedRider.FrozenDeposit)
}

// TestTakeoutJourneyB1WebhookIntegration
// 外卖旅程（B1）端到端验收：走微信支付回调路径 + 任务入队，再由 worker 处理支付成功推进。
func TestTakeoutJourneyB1WebhookIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)

	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	// 1) C端下单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}
	orderID := created.ID

	// 2) 创建支付单
	var payment takeoutPaymentOrderResponse
	{
		payBody := map[string]any{
			"order_id":      orderID,
			"payment_type":  "native",
			"business_type": "order",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/payments", payBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payment)
		require.Equal(t, "pending", payment.Status)
	}

	paymentOrder, err := store.GetPaymentOrder(ctx, payment.ID)
	require.NoError(t, err)

	// 3) 注入 mock payment client 与任务分发器
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentClient := mockwechat.NewMockPaymentClientInterface(ctrl)
	mockPaymentClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1).
		Return(nil)

	mockPaymentClient.EXPECT().
		DecryptPaymentNotification(gomock.Any()).
		Times(1).
		Return(&wechat.PaymentNotificationResource{
			TransactionID: "integration_tx_webhook_001",
			OutTradeNo:    paymentOrder.OutTradeNo,
			TradeState:    "SUCCESS",
			Amount: struct {
				Total         int64  `json:"total"`
				PayerTotal    int64  `json:"payer_total"`
				Currency      string `json:"currency"`
				PayerCurrency string `json:"payer_currency"`
			}{
				Total:         paymentOrder.Amount,
				PayerTotal:    paymentOrder.Amount,
				Currency:      "CNY",
				PayerCurrency: "CNY",
			},
		}, nil)

	distributor := &capturePaymentSuccessDistributor{}
	server.SetPaymentClientForTest(mockPaymentClient)
	server.SetTaskDistributorForTest(distributor)
	defer server.SetPaymentClientForTest(nil)
	defer server.SetTaskDistributorForTest(nil)

	// 4) 触发支付回调
	notificationID := "notify_" + util.RandomString(8)
	body := map[string]any{
		"id":            notificationID,
		"event_type":    "TRANSACTION.SUCCESS",
		"resource_type": "encrypt-resource",
		"resource": map[string]any{
			"algorithm":       "AEAD_AES_256_GCM",
			"ciphertext":      "mock_ciphertext",
			"nonce":           "mock_nonce",
			"associated_data": "transaction",
			"original_type":   "transaction",
		},
		"summary": "success",
	}

	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-pay/notify", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Wechatpay-Timestamp", "1234567890")
	req.Header.Set("Wechatpay-Nonce", "test_nonce")
	req.Header.Set("Wechatpay-Signature", "test_signature")
	req.Header.Set("Wechatpay-Serial", "test_serial")

	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	updatedPayment, err := store.GetPaymentOrder(ctx, payment.ID)
	require.NoError(t, err)
	require.Equal(t, "paid", updatedPayment.Status)

	// 5) 运行支付成功任务，推进订单与配送单
	payloads := distributor.Payloads()
	require.Len(t, payloads, 1)

	payloadBytes, err := json.Marshal(payloads[0])
	require.NoError(t, err)

	processor := worker.NewTestTaskProcessor(store, worker.NewNoopTaskDistributor(), nil, nil)
	task := asynq.NewTask(worker.TaskProcessPaymentSuccess, payloadBytes)
	require.NoError(t, processor.ProcessTaskPaymentSuccess(ctx, task))

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "paid", order.Status)

	_, err = store.GetDeliveryByOrderID(ctx, orderID)
	require.NoError(t, err)
	_, err = store.GetDeliveryPoolByOrderID(ctx, orderID)
	require.NoError(t, err)
}

// TestTakeoutJourneyB0CombinedPaymentIntegration
// 外卖旅程（B0）端到端验收：合单支付创建与关闭。
func TestTakeoutJourneyB0CombinedPaymentIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	_, err = store.CreateMerchantPaymentConfig(ctx, db.CreateMerchantPaymentConfigParams{
		MerchantID: merchant.ID,
		SubMchID:   "sub_mch_001",
		Status:     "active",
	})
	require.NoError(t, err)

	dish := createIntegrationDish(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}
	orderID := created.ID

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)

	customerRow, err := store.GetUser(ctx, customer.ID)
	require.NoError(t, err)
	if customerRow.WechatOpenid == "" {
		_, err = integrationPool.Exec(ctx, `UPDATE users SET wechat_openid = $2 WHERE id = $1`, customer.ID, "wx_openid_"+util.RandomString(8))
		require.NoError(t, err)
	}

	if existingPayment, err := store.GetLatestPaymentOrderByOrder(ctx, db.GetLatestPaymentOrderByOrderParams{
		OrderID:      pgtype.Int8{Int64: orderID, Valid: true},
		BusinessType: "order",
	}); err == nil {
		if existingPayment.Status == "pending" && existingPayment.PaymentType != "profit_sharing" {
			_, err = integrationPool.Exec(ctx, `UPDATE payment_orders SET status = 'closed' WHERE id = $1`, existingPayment.ID)
			require.NoError(t, err)
		}
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEcommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	mockEcommerceClient.EXPECT().
		CreateCombineOrder(gomock.Any(), gomock.Any()).
		Times(1).
		Return(&wechat.CombineOrderResponse{PrepayID: "prepay_combined_001"}, &wechat.JSAPIPayParams{
			TimeStamp: "1",
			NonceStr:  "nonce",
			Package:   "prepay_id=prepay_combined_001",
			SignType:  "RSA",
			PaySign:   "sign",
		}, nil)
	mockEcommerceClient.EXPECT().
		CloseCombineOrder(gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1).
		Return(nil)

	server.SetEcommerceClientForTest(mockEcommerceClient)
	defer server.SetEcommerceClientForTest(nil)

	// 2) 创建合单支付
	var combined combinedPaymentOrderResponse
	{
		body := map[string]any{
			"order_ids": []int64{orderID},
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/payments/combined", body, customer.ID)
		if rec.Code != http.StatusOK {
			t.Fatalf("create combined payment failed: %s", rec.Body.String())
		}
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &combined)
		require.Equal(t, "pending", combined.Status)
		require.Equal(t, order.TotalAmount, combined.TotalAmount)
		require.Len(t, combined.SubOrders, 1)
		require.Equal(t, orderID, combined.SubOrders[0].OrderID)
	}

	// 3) 查询合单支付详情
	{
		url := fmt.Sprintf("/v1/payments/combined/%d", combined.ID)
		rec := doGET(t, server, url, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		var detail combinedPaymentOrderResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &detail)
		require.Equal(t, combined.ID, detail.ID)
		require.Equal(t, combined.Status, detail.Status)
		require.Len(t, detail.SubOrders, 1)
		require.Equal(t, orderID, detail.SubOrders[0].OrderID)
	}

	// 4) 关闭合单支付
	{
		url := fmt.Sprintf("/v1/payments/combined/%d/close", combined.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		var closed combinedPaymentOrderResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &closed)
		require.Equal(t, "closed", closed.Status)
	}
}

// TestTakeoutJourneyB0DeliveryRecommendIntegration
// 外卖旅程（B0）端到端验收：骑手推荐订单列表包含待配送订单。
func TestTakeoutJourneyB0DeliveryRecommendIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err = store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}
	orderID := created.ID

	// 2) 创建支付单并模拟支付成功
	var payment takeoutPaymentOrderResponse
	{
		payBody := map[string]any{
			"order_id":      orderID,
			"payment_type":  "native",
			"business_type": "order",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/payments", payBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payment)
	}

	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: "integration_tx_recommend_001", Valid: true},
	})
	require.NoError(t, err)

	payRes, err := store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     payment.ID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	})
	require.NoError(t, err)
	require.True(t, payRes.Processed)

	// 3) 推荐订单列表
	url := "/v1/delivery/recommend?longitude=116.397&latitude=39.908"
	rec := doGET(t, server, url, riderUser.ID)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp []deliveryRecommendResponse
	requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
	require.NotEmpty(t, resp)

	found := false
	for _, item := range resp {
		if item.OrderID == orderID {
			found = true
			break
		}
	}
	require.True(t, found)
}

// TestTakeoutJourneyB0CartCalculateIntegration
// 外卖旅程（B0）端到端验收：购物车加购与试算。
func TestTakeoutJourneyB0CartCalculateIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)

	dish := createIntegrationDish(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)

	// 1) 加购
	addBody := map[string]any{
		"merchant_id": merchant.ID,
		"order_type":  "takeout",
		"dish_id":     dish.ID,
		"quantity":    2,
	}
	var cart cartResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/cart/items", addBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &cart)
		require.Equal(t, merchant.ID, cart.MerchantID)
		require.Equal(t, 2, cart.TotalCount)
		require.Greater(t, cart.Subtotal, int64(0))
	}

	// 2) 查询购物车
	{
		url := fmt.Sprintf("/v1/cart?merchant_id=%d&order_type=takeout", merchant.ID)
		rec := doGET(t, server, url, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		var got cartResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &got)
		require.Equal(t, cart.ID, got.ID)
		require.Equal(t, 2, got.TotalCount)
	}

	// 3) 试算
	calcBody := map[string]any{
		"merchant_id": merchant.ID,
		"order_type":  "takeout",
	}
	var calc calculateCartResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/cart/calculate", calcBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &calc)
		require.Greater(t, calc.Subtotal, int64(0))
		require.GreaterOrEqual(t, calc.TotalAmount, calc.Subtotal)
	}
}

// TestTakeoutJourneyB4PaymentOrderTimeoutIntegration
// 外卖旅程（B4）支付单超时兜底：payment_order:timeout 关闭支付单并取消待支付订单。
func TestTakeoutJourneyB4PaymentOrderTimeoutIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	// 2) 创建支付单
	var payment takeoutPaymentOrderResponse
	{
		payBody := map[string]any{
			"order_id":      created.ID,
			"payment_type":  "native",
			"business_type": "order",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/payments", payBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payment)
		require.Equal(t, "pending", payment.Status)
	}

	po, err := store.GetPaymentOrder(ctx, payment.ID)
	require.NoError(t, err)

	// 3) 回拨支付单过期时间，触发 payment_order:timeout
	_, err = integrationPool.Exec(ctx, `UPDATE payment_orders SET expires_at = $2 WHERE id = $1`, po.ID, time.Now().Add(-10*time.Minute))
	require.NoError(t, err)

	processor := worker.NewTestTaskProcessor(store, worker.NewNoopTaskDistributor(), nil, nil)
	payloadBytes, err := json.Marshal(worker.PayloadPaymentOrderTimeout{PaymentOrderNo: po.OutTradeNo})
	require.NoError(t, err)
	if err := processor.ProcessTaskPaymentOrderTimeout(ctx, asynq.NewTask(worker.TaskPaymentOrderTimeout, payloadBytes)); err != nil {
		require.NoError(t, err)
	}

	updatedPO, err := store.GetPaymentOrder(ctx, po.ID)
	require.NoError(t, err)
	require.Equal(t, "closed", updatedPO.Status)

	updatedOrder, err := store.GetOrder(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "cancelled", updatedOrder.Status)
}

// TestTakeoutJourneyB7MerchantRejectRefundIntegration
// 外卖旅程（B7）异常链路：商户拒单触发退款处理。
func TestTakeoutJourneyB7MerchantRejectRefundIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}
	orderID := created.ID

	// 2) 创建支付单并模拟支付成功
	var payment takeoutPaymentOrderResponse
	{
		payBody := map[string]any{
			"order_id":      orderID,
			"payment_type":  "native",
			"business_type": "order",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/payments", payBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payment)
	}

	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: "integration_tx_reject_001", Valid: true},
	})
	require.NoError(t, err)

	payRes, err := store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     payment.ID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	})
	require.NoError(t, err)
	require.True(t, payRes.Processed)

	latestPayment, err := store.GetLatestPaymentOrderByOrder(ctx, db.GetLatestPaymentOrderByOrderParams{
		OrderID:      pgtype.Int8{Int64: orderID, Valid: true},
		BusinessType: "order",
	})
	require.NoError(t, err)
	if latestPayment.Status != "paid" {
		_, err := integrationPool.Exec(ctx, `UPDATE payment_orders SET status = 'paid' WHERE id = $1`, latestPayment.ID)
		require.NoError(t, err)
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentClient := mockwechat.NewMockPaymentClientInterface(ctrl)
	mockPaymentClient.EXPECT().
		CreateRefund(gomock.Any(), gomock.Any()).
		Times(1).
		Return(&wechat.RefundResponse{Status: wechat.RefundStatusProcessing, RefundID: "refund_reject_001"}, nil)

	server.SetPaymentClientForTest(mockPaymentClient)
	defer server.SetPaymentClientForTest(nil)

	// 3) 商户拒单
	{
		body := map[string]any{"reason": "out_of_stock"}
		url := fmt.Sprintf("/v1/merchant/orders/%d/reject", orderID)
		rec := doJSON(t, server, http.MethodPost, url, body, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		var resp takeoutOrderResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, "cancelled", resp.Status)
	}

	updatedOrder, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "cancelled", updatedOrder.Status)

	updatedPayment, err := store.GetPaymentOrder(ctx, latestPayment.ID)
	require.NoError(t, err)
	require.Equal(t, "paid", updatedPayment.Status)

	refunds, err := store.ListRefundOrdersByPaymentOrder(ctx, latestPayment.ID)
	require.NoError(t, err)
	require.NotEmpty(t, refunds)
	require.Equal(t, "processing", refunds[0].Status)
}

// TestTakeoutJourneyB5PaymentRecoveryIntegration
// 外卖旅程（B5）回调丢失补偿：payment recovery scheduler 扫描 paid 未处理支付单并入队。
func TestTakeoutJourneyB5PaymentRecoveryIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	// 2) 创建支付单并标记为 paid，但不执行 ProcessPaymentSuccessTx
	var payment takeoutPaymentOrderResponse
	{
		payBody := map[string]any{
			"order_id":      created.ID,
			"payment_type":  "native",
			"business_type": "order",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/payments", payBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payment)
	}

	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: "integration_tx_recovery_001", Valid: true},
	})
	require.NoError(t, err)

	// backdate paid_at to pass recovery min age
	_, err = integrationPool.Exec(ctx, `UPDATE payment_orders SET paid_at = $2 WHERE id = $1`, payment.ID, time.Now().Add(-5*time.Minute))
	require.NoError(t, err)

	// 3) 触发 recovery scheduler 并断言入队
	d := &capturePaymentSuccessDistributor{}
	recovery := worker.NewPaymentRecoveryScheduler(store, d)
	recovery.RunOnce()

	payloads := d.Payloads()
	require.Len(t, payloads, 1)
	require.Equal(t, payment.ID, payloads[0].PaymentOrderID)
}

// TestTakeoutJourneyB6OrderPaymentTimeoutIntegration
// 外卖旅程（B6）订单支付超时兜底：order:payment_timeout 取消 pending 订单。
func TestTakeoutJourneyB6OrderPaymentTimeoutIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	// 1) 创建外卖订单（pending）
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	// 2) 回拨创建时间，触发订单支付超时
	backTo := time.Now().Add(-time.Duration(worker.OrderPaymentTimeoutMinutes) * time.Minute).Add(-2 * time.Minute)
	_, err = integrationPool.Exec(ctx, `UPDATE orders SET created_at = $2 WHERE id = $1`, created.ID, backTo)
	require.NoError(t, err)

	processor := worker.NewTestTaskProcessor(store, worker.NewNoopTaskDistributor(), nil, nil)
	payloadBytes, err := json.Marshal(worker.PayloadOrderPaymentTimeout{OrderID: created.ID})
	require.NoError(t, err)
	if err := processor.ProcessTaskOrderPaymentTimeout(ctx, asynq.NewTask(worker.TaskOrderPaymentTimeout, payloadBytes)); err != nil {
		require.NoError(t, err)
	}

	updatedOrder, err := store.GetOrder(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "cancelled", updatedOrder.Status)
}

// TestDineInJourneyA0ScanTableIntegration
// 堂食旅程（A0）端到端验收：扫码入口返回商户与桌台信息。
func TestDineInJourneyA0ScanTableIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)

	createIntegrationDish(t, store, merchant.ID)

	table := createIntegrationTable(t, store, merchant.ID, pgtype.Text{Valid: false})
	customer := createIntegrationUser(t, store)

	url := fmt.Sprintf("/v1/scan/table?merchant_id=%d&table_no=%s", merchant.ID, table.TableNo)
	rec := doGET(t, server, url, customer.ID)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp scanTableResponse
	requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
	require.Equal(t, merchant.ID, resp.Merchant.ID)
	require.Equal(t, table.ID, resp.Table.ID)
	require.Equal(t, table.TableNo, resp.Table.TableNo)
}

// TestDineInJourneyA1Integration
// 堂食旅程（A1）端到端验收：开台 -> 下单 -> 支付回调 -> 厨房制作/出餐 -> 结账离店。
func TestDineInJourneyA1Integration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)

	accessCode := "1234"
	accessHash, err := util.HashPassword(accessCode)
	require.NoError(t, err)

	table := createIntegrationTable(t, store, merchant.ID, pgtype.Text{String: accessHash, Valid: true})

	// 1) 开台
	var openResp dineInSessionResponse
	{
		body := map[string]any{
			"table_id":   table.ID,
			"table_code": accessCode,
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/dining-sessions/open", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &openResp)
		require.NotZero(t, openResp.Session.ID)
	}

	// 2) 下单（堂食）
	createBody := map[string]any{
		"merchant_id": merchant.ID,
		"order_type":  "dine_in",
		"table_id":    table.ID,
		"items":       []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
		require.Equal(t, "dine_in", created.OrderType)
	}
	orderID := created.ID

	// 3) 创建支付单
	var payment takeoutPaymentOrderResponse
	{
		payBody := map[string]any{
			"order_id":      orderID,
			"payment_type":  "native",
			"business_type": "order",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/payments", payBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payment)
		require.Equal(t, "pending", payment.Status)
	}

	po, err := store.GetPaymentOrder(ctx, payment.ID)
	require.NoError(t, err)

	// 4) 走支付回调 + 任务入队
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentClient := mockwechat.NewMockPaymentClientInterface(ctrl)
	mockPaymentClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1).
		Return(nil)

	mockPaymentClient.EXPECT().
		DecryptPaymentNotification(gomock.Any()).
		Times(1).
		Return(&wechat.PaymentNotificationResource{
			TransactionID: "integration_tx_dinein_001",
			OutTradeNo:    po.OutTradeNo,
			TradeState:    "SUCCESS",
			Amount: struct {
				Total         int64  `json:"total"`
				PayerTotal    int64  `json:"payer_total"`
				Currency      string `json:"currency"`
				PayerCurrency string `json:"payer_currency"`
			}{
				Total:         po.Amount,
				PayerTotal:    po.Amount,
				Currency:      "CNY",
				PayerCurrency: "CNY",
			},
		}, nil)

	distributor := &capturePaymentSuccessDistributor{}
	server.SetPaymentClientForTest(mockPaymentClient)
	server.SetTaskDistributorForTest(distributor)
	defer server.SetPaymentClientForTest(nil)
	defer server.SetTaskDistributorForTest(nil)

	notificationID := "notify_dinein_" + util.RandomString(8)
	body := map[string]any{
		"id":            notificationID,
		"event_type":    "TRANSACTION.SUCCESS",
		"resource_type": "encrypt-resource",
		"resource": map[string]any{
			"algorithm":       "AEAD_AES_256_GCM",
			"ciphertext":      "mock_ciphertext",
			"nonce":           "mock_nonce",
			"associated_data": "transaction",
			"original_type":   "transaction",
		},
		"summary": "success",
	}

	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-pay/notify", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Wechatpay-Timestamp", "1234567890")
	req.Header.Set("Wechatpay-Nonce", "test_nonce")
	req.Header.Set("Wechatpay-Signature", "test_signature")
	req.Header.Set("Wechatpay-Serial", "test_serial")

	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	// 5) 处理支付成功任务
	payloads := distributor.Payloads()
	require.Len(t, payloads, 1)

	payloadBytes, err := json.Marshal(payloads[0])
	require.NoError(t, err)

	processor := worker.NewTestTaskProcessor(store, worker.NewNoopTaskDistributor(), nil, nil)
	task := asynq.NewTask(worker.TaskProcessPaymentSuccess, payloadBytes)
	require.NoError(t, processor.ProcessTaskPaymentSuccess(ctx, task))

	paidOrder, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "paid", paidOrder.Status)

	// 6) 厨房开始制作/出餐
	{
		url := fmt.Sprintf("/v1/kitchen/orders/%d/preparing", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		var resp kitchenOrderStatusResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, "preparing", resp.Status)
	}
	{
		url := fmt.Sprintf("/v1/kitchen/orders/%d/ready", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		var resp kitchenOrderStatusResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, "ready", resp.Status)
	}

	// 7) 结账离店
	{
		url := fmt.Sprintf("/v1/dining-sessions/%d/checkout", openResp.Session.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	updatedSession, err := store.GetDiningSession(ctx, openResp.Session.ID)
	require.NoError(t, err)
	require.Equal(t, "closed", updatedSession.Status)

	updatedTable, err := store.GetTable(ctx, table.ID)
	require.NoError(t, err)
	require.Equal(t, "available", updatedTable.Status)
}

// TestDineInJourneyA2InvalidTableCodeIntegration
// 堂食旅程（A2）端到端验收：桌台码错误拒绝开台。
func TestDineInJourneyA2InvalidTableCodeIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)

	accessCode := "1234"
	accessHash, err := util.HashPassword(accessCode)
	require.NoError(t, err)

	table := createIntegrationTable(t, store, merchant.ID, pgtype.Text{String: accessHash, Valid: true})
	customer := createIntegrationUser(t, store)

	body := map[string]any{
		"table_id":   table.ID,
		"table_code": "9999",
	}
	rec := doJSON(t, server, http.MethodPost, "/v1/dining-sessions/open", body, customer.ID)
	require.Equal(t, http.StatusForbidden, rec.Code)
}

// TestDineInJourneyA3MerchantOpenWithoutReservationIntegration
// 堂食旅程（A3）端到端验收：商户无预订不能代客开台。
func TestDineInJourneyA3MerchantOpenWithoutReservationIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)

	accessCode := "1234"
	accessHash, err := util.HashPassword(accessCode)
	require.NoError(t, err)

	table := createIntegrationTable(t, store, merchant.ID, pgtype.Text{String: accessHash, Valid: true})

	body := map[string]any{
		"table_id": table.ID,
	}
	rec := doJSON(t, server, http.MethodPost, "/v1/dining-sessions/open", body, merchantOwner.ID)
	require.Equal(t, http.StatusForbidden, rec.Code)
}

// TestDineInJourneyA4CheckoutForbiddenIntegration
// 堂食旅程（A4）端到端验收：非商户用户结账被拒绝。
func TestDineInJourneyA4CheckoutForbiddenIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)

	accessCode := "1234"
	accessHash, err := util.HashPassword(accessCode)
	require.NoError(t, err)

	table := createIntegrationTable(t, store, merchant.ID, pgtype.Text{String: accessHash, Valid: true})
	customer := createIntegrationUser(t, store)

	// 1) 客户开台
	var openResp dineInSessionResponse
	{
		body := map[string]any{
			"table_id":   table.ID,
			"table_code": accessCode,
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/dining-sessions/open", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &openResp)
		require.NotZero(t, openResp.Session.ID)
	}

	// 2) 客户尝试结账
	{
		url := fmt.Sprintf("/v1/dining-sessions/%d/checkout", openResp.Session.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, customer.ID)
		require.Equal(t, http.StatusForbidden, rec.Code)
	}
}

// TestDineInJourneyA5OrderPaymentTimeoutIntegration
// 堂食旅程（A5）端到端验收：堂食订单支付超时自动取消。
func TestDineInJourneyA5OrderPaymentTimeoutIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)

	dish := createIntegrationDish(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)

	accessCode := "1234"
	accessHash, err := util.HashPassword(accessCode)
	require.NoError(t, err)

	table := createIntegrationTable(t, store, merchant.ID, pgtype.Text{String: accessHash, Valid: true})

	// 1) 开台
	{
		body := map[string]any{
			"table_id":   table.ID,
			"table_code": accessCode,
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/dining-sessions/open", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 2) 创建堂食订单（pending）
	createBody := map[string]any{
		"merchant_id": merchant.ID,
		"order_type":  "dine_in",
		"table_id":    table.ID,
		"items":       []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
		require.Equal(t, "dine_in", created.OrderType)
	}

	// 3) 回拨创建时间，触发订单支付超时
	backTo := time.Now().Add(-time.Duration(worker.OrderPaymentTimeoutMinutes) * time.Minute).Add(-2 * time.Minute)
	_, err = integrationPool.Exec(ctx, `UPDATE orders SET created_at = $2 WHERE id = $1`, created.ID, backTo)
	require.NoError(t, err)

	processor := worker.NewTestTaskProcessor(store, worker.NewNoopTaskDistributor(), nil, nil)
	payloadBytes, err := json.Marshal(worker.PayloadOrderPaymentTimeout{OrderID: created.ID})
	require.NoError(t, err)
	if err := processor.ProcessTaskOrderPaymentTimeout(ctx, asynq.NewTask(worker.TaskOrderPaymentTimeout, payloadBytes)); err != nil {
		require.NoError(t, err)
	}

	updatedOrder, err := store.GetOrder(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "cancelled", updatedOrder.Status)
}

// TestReservationJourneyCAvailabilityIntegration
// 包间预订可用性（C1）端到端验收：营业时段内有可用时段，已有预订时段不可用。
func TestReservationJourneyCAvailabilityIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)

	room := createIntegrationRoomTable(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)

	reservationDate := time.Now().Add(24 * time.Hour)
	dateStr := reservationDate.Format("2006-01-02")
	dayOfWeek := int32(reservationDate.Weekday())

	openTime := pgtype.Time{Microseconds: int64(18*3600) * 1000000, Valid: true}
	closeTime := pgtype.Time{Microseconds: int64(21*3600) * 1000000, Valid: true}
	_, err = store.CreateBusinessHour(ctx, db.CreateBusinessHourParams{
		MerchantID:  merchant.ID,
		DayOfWeek:   dayOfWeek,
		OpenTime:    openTime,
		CloseTime:   closeTime,
		IsClosed:    false,
		SpecialDate: pgtype.Date{Valid: false},
	})
	require.NoError(t, err)

	availabilityURL := fmt.Sprintf("/v1/rooms/%d/availability?date=%s", room.ID, dateStr)
	{
		rec := doGET(t, server, availabilityURL, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		var resp roomAvailabilityResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, room.ID, resp.RoomID)
		require.Equal(t, dateStr, resp.Date)
		require.NotEmpty(t, resp.TimeSlots)

		found := false
		for _, slot := range resp.TimeSlots {
			if slot.Time == "19:00" {
				require.True(t, slot.Available)
				found = true
				break
			}
		}
		require.True(t, found)
	}

	reservationDateTime := time.Date(reservationDate.Year(), reservationDate.Month(), reservationDate.Day(), 19, 0, 0, 0, time.Local)
	_, err = store.CreateReservationTx(ctx, db.CreateReservationTxParams{
		CreateTableReservationParams: db.CreateTableReservationParams{
			TableID:         room.ID,
			UserID:          customer.ID,
			MerchantID:      merchant.ID,
			ReservationDate: pgtype.Date{Time: reservationDate, Valid: true},
			ReservationTime: pgtype.Time{Microseconds: int64(19*3600) * 1000000, Valid: true},
			GuestCount:      2,
			ContactName:     "张三",
			ContactPhone:    "13800138001",
			PaymentMode:     "deposit",
			DepositAmount:   10000,
			PrepaidAmount:   0,
			RefundDeadline:  reservationDateTime.Add(-2 * time.Hour),
			PaymentDeadline: time.Now().Add(30 * time.Minute),
			Notes:           pgtype.Text{Valid: false},
			Status:          "pending",
		},
	})
	require.NoError(t, err)

	{
		rec := doGET(t, server, availabilityURL, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		var resp roomAvailabilityResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		blocked := false
		for _, slot := range resp.TimeSlots {
			if slot.Time == "19:00" {
				require.False(t, slot.Available)
				blocked = true
				break
			}
		}
		require.True(t, blocked)
	}
}

// TestReservationJourneyCCheckInPendingIntegration
// 包间预订异常链路（C-CheckIn-Pending）：未支付预订签到被拒。
func TestReservationJourneyCCheckInPendingIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)

	room := createIntegrationRoomTable(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)

	reservationDate := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	reservationTime := "19:00"

	// 1) 创建预订（未支付）
	var created reservationStatusResponse
	{
		body := map[string]any{
			"table_id":      room.ID,
			"date":          reservationDate,
			"time":          reservationTime,
			"guest_count":   2,
			"contact_name":  "张三",
			"contact_phone": "13800138001",
			"payment_mode":  "deposit",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/reservations", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	// 2) 未支付直接签到
	{
		url := fmt.Sprintf("/v1/reservations/%d/checkin", created.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, customer.ID)
		require.Equal(t, http.StatusConflict, rec.Code)
	}
}

// TestReservationJourneyC1Integration
// 包间预订旅程（C1）端到端验收：创建预订 -> 支付回调 -> 商户确认 -> 顾客签到 -> 完结。
func TestReservationJourneyC1Integration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)

	room := createIntegrationRoomTable(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)

	reservationDate := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	reservationTime := "19:00"

	// 1) 创建预订（定金模式）
	var created reservationStatusResponse
	{
		body := map[string]any{
			"table_id":      room.ID,
			"date":          reservationDate,
			"time":          reservationTime,
			"guest_count":   2,
			"contact_name":  "张三",
			"contact_phone": "13800138001",
			"payment_mode":  "deposit",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/reservations", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	// 2) 创建预订支付单
	var payment takeoutPaymentOrderResponse
	{
		payBody := map[string]any{
			"order_id":      created.ID,
			"payment_type":  "native",
			"business_type": "reservation",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/payments", payBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payment)
		require.Equal(t, "pending", payment.Status)
	}

	po, err := store.GetPaymentOrder(ctx, payment.ID)
	require.NoError(t, err)

	// 3) 走支付回调 + 任务入队
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentClient := mockwechat.NewMockPaymentClientInterface(ctrl)
	mockPaymentClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1).
		Return(nil)

	mockPaymentClient.EXPECT().
		DecryptPaymentNotification(gomock.Any()).
		Times(1).
		Return(&wechat.PaymentNotificationResource{
			TransactionID: "integration_tx_reservation_001",
			OutTradeNo:    po.OutTradeNo,
			TradeState:    "SUCCESS",
			Amount: struct {
				Total         int64  `json:"total"`
				PayerTotal    int64  `json:"payer_total"`
				Currency      string `json:"currency"`
				PayerCurrency string `json:"payer_currency"`
			}{
				Total:         po.Amount,
				PayerTotal:    po.Amount,
				Currency:      "CNY",
				PayerCurrency: "CNY",
			},
		}, nil)

	distributor := &capturePaymentSuccessDistributor{}
	server.SetPaymentClientForTest(mockPaymentClient)
	server.SetTaskDistributorForTest(distributor)
	defer server.SetPaymentClientForTest(nil)
	defer server.SetTaskDistributorForTest(nil)

	notificationID := "notify_reservation_" + util.RandomString(8)
	body := map[string]any{
		"id":            notificationID,
		"event_type":    "TRANSACTION.SUCCESS",
		"resource_type": "encrypt-resource",
		"resource": map[string]any{
			"algorithm":       "AEAD_AES_256_GCM",
			"ciphertext":      "mock_ciphertext",
			"nonce":           "mock_nonce",
			"associated_data": "transaction",
			"original_type":   "transaction",
		},
		"summary": "success",
	}

	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-pay/notify", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Wechatpay-Timestamp", "1234567890")
	req.Header.Set("Wechatpay-Nonce", "test_nonce")
	req.Header.Set("Wechatpay-Signature", "test_signature")
	req.Header.Set("Wechatpay-Serial", "test_serial")

	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	// 4) 处理支付成功任务
	payloads := distributor.Payloads()
	require.Len(t, payloads, 1)

	payloadBytes, err := json.Marshal(payloads[0])
	require.NoError(t, err)

	processor := worker.NewTestTaskProcessor(store, worker.NewNoopTaskDistributor(), nil, nil)
	task := asynq.NewTask(worker.TaskProcessPaymentSuccess, payloadBytes)
	require.NoError(t, processor.ProcessTaskPaymentSuccess(ctx, task))

	paidReservation, err := store.GetTableReservation(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "paid", paidReservation.Status)

	// 5) 商户确认预订
	{
		url := fmt.Sprintf("/v1/reservations/%d/confirm", created.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		var resp reservationStatusResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, "confirmed", resp.Status)
	}

	// 6) 顾客签到
	{
		url := fmt.Sprintf("/v1/reservations/%d/checkin", created.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		var resp reservationStatusResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, "checked_in", resp.Status)
	}

	// 7) 商户完结
	{
		url := fmt.Sprintf("/v1/reservations/%d/complete", created.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		var resp reservationStatusResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, "completed", resp.Status)
	}

	updatedTable, err := store.GetTable(ctx, room.ID)
	require.NoError(t, err)
	require.Equal(t, "available", updatedTable.Status)
}

// TestReservationJourneyC2StartCookingIntegration
// 包间预订旅程（C2）端到端验收：商户对已确认预订起菜通知。
func TestReservationJourneyC2StartCookingIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)

	room := createIntegrationRoomTable(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)

	reservationDate := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	reservationTime := "19:00"

	// 1) 创建预订
	var created reservationStatusResponse
	{
		body := map[string]any{
			"table_id":      room.ID,
			"date":          reservationDate,
			"time":          reservationTime,
			"guest_count":   2,
			"contact_name":  "张三",
			"contact_phone": "13800138001",
			"payment_mode":  "deposit",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/reservations", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	_, err = store.UpdateReservationStatus(ctx, db.UpdateReservationStatusParams{
		ID:     created.ID,
		Status: "confirmed",
	})
	require.NoError(t, err)

	// 2) 商户起菜通知
	{
		url := fmt.Sprintf("/v1/reservations/%d/start-cooking", created.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		var resp reservationStatusResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, "confirmed", resp.Status)
	}
}

// TestReservationJourneyCTodayListIntegration
// 包间预订旅程（C3）端到端验收：商户获取今日预订列表。
func TestReservationJourneyCTodayListIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)

	room := createIntegrationRoomTable(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)

	reservationDate := time.Now()
	reservationTime := time.Date(reservationDate.Year(), reservationDate.Month(), reservationDate.Day(), 19, 0, 0, 0, time.Local)

	created, err := store.CreateReservationTx(ctx, db.CreateReservationTxParams{
		CreateTableReservationParams: db.CreateTableReservationParams{
			TableID:         room.ID,
			UserID:          customer.ID,
			MerchantID:      merchant.ID,
			ReservationDate: pgtype.Date{Time: reservationDate, Valid: true},
			ReservationTime: pgtype.Time{Microseconds: int64(19*3600) * 1000000, Valid: true},
			GuestCount:      2,
			ContactName:     "张三",
			ContactPhone:    "13800138001",
			PaymentMode:     "deposit",
			DepositAmount:   10000,
			PrepaidAmount:   0,
			RefundDeadline:  reservationTime.Add(-2 * time.Hour),
			PaymentDeadline: time.Now().Add(30 * time.Minute),
			Notes:           pgtype.Text{Valid: false},
			Status:          "confirmed",
		},
	})
	require.NoError(t, err)

	url := "/v1/reservations/merchant/today"
	rec := doGET(t, server, url, merchantOwner.ID)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp listTodayReservationsResponse
	requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
	require.NotEmpty(t, resp.Reservations)

	found := false
	for _, r := range resp.Reservations {
		if r.ID == created.Reservation.ID {
			found = true
			break
		}
	}
	require.True(t, found)
}

// TestReservationJourneyCPaymentTimeoutIntegration
// 包间预订异常链路（C-Timeout）：支付超时后自动取消预订。
func TestReservationJourneyCPaymentTimeoutIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)

	room := createIntegrationRoomTable(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)

	reservationDate := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	reservationTime := "19:00"

	// 1) 创建预订
	var created reservationStatusResponse
	{
		body := map[string]any{
			"table_id":      room.ID,
			"date":          reservationDate,
			"time":          reservationTime,
			"guest_count":   2,
			"contact_name":  "张三",
			"contact_phone": "13800138001",
			"payment_mode":  "deposit",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/reservations", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	_, err = integrationPool.Exec(ctx,
		`UPDATE table_reservations SET refund_deadline = $2 WHERE id = $1`,
		created.ID,
		time.Now().Add(2*time.Hour),
	)
	require.NoError(t, err)

	// 2) 回写支付截止时间为过去，触发超时处理
	_, err = integrationPool.Exec(ctx,
		`UPDATE table_reservations SET payment_deadline = $2 WHERE id = $1`,
		created.ID,
		time.Now().Add(-5*time.Minute),
	)
	require.NoError(t, err)

	payloadBytes, err := json.Marshal(worker.PayloadReservationPaymentTimeout{ReservationID: created.ID})
	require.NoError(t, err)

	processor := worker.NewTestTaskProcessor(store, worker.NewNoopTaskDistributor(), nil, nil)
	task := asynq.NewTask(worker.TaskReservationPaymentTimeout, payloadBytes)
	require.NoError(t, processor.ProcessTaskReservationPaymentTimeout(ctx, task))

	updatedReservation, err := store.GetTableReservation(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "cancelled", updatedReservation.Status)
}

// TestReservationJourneyCNoShowIntegration
// 包间预订异常链路（C-NoShow）：商户标记爽约后释放桌台。
func TestReservationJourneyCNoShowIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)

	room := createIntegrationRoomTable(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)

	reservationDate := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	reservationTime := "19:00"

	// 1) 创建预订
	var created reservationStatusResponse
	{
		body := map[string]any{
			"table_id":      room.ID,
			"date":          reservationDate,
			"time":          reservationTime,
			"guest_count":   2,
			"contact_name":  "张三",
			"contact_phone": "13800138001",
			"payment_mode":  "deposit",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/reservations", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	// 2) 创建预订支付单
	var payment takeoutPaymentOrderResponse
	{
		payBody := map[string]any{
			"order_id":      created.ID,
			"payment_type":  "native",
			"business_type": "reservation",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/payments", payBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payment)
		require.Equal(t, "pending", payment.Status)
	}

	po, err := store.GetPaymentOrder(ctx, payment.ID)
	require.NoError(t, err)

	// 3) 走支付回调 + 任务入队
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentClient := mockwechat.NewMockPaymentClientInterface(ctrl)
	mockPaymentClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1).
		Return(nil)

	mockPaymentClient.EXPECT().
		DecryptPaymentNotification(gomock.Any()).
		Times(1).
		Return(&wechat.PaymentNotificationResource{
			TransactionID: "integration_tx_reservation_noshow_001",
			OutTradeNo:    po.OutTradeNo,
			TradeState:    "SUCCESS",
			Amount: struct {
				Total         int64  `json:"total"`
				PayerTotal    int64  `json:"payer_total"`
				Currency      string `json:"currency"`
				PayerCurrency string `json:"payer_currency"`
			}{
				Total:         po.Amount,
				PayerTotal:    po.Amount,
				Currency:      "CNY",
				PayerCurrency: "CNY",
			},
		}, nil)

	distributor := &capturePaymentSuccessDistributor{}
	server.SetPaymentClientForTest(mockPaymentClient)
	server.SetTaskDistributorForTest(distributor)
	defer server.SetPaymentClientForTest(nil)
	defer server.SetTaskDistributorForTest(nil)

	notificationID := "notify_reservation_noshow_" + util.RandomString(8)
	body := map[string]any{
		"id":            notificationID,
		"event_type":    "TRANSACTION.SUCCESS",
		"resource_type": "encrypt-resource",
		"resource": map[string]any{
			"algorithm":       "AEAD_AES_256_GCM",
			"ciphertext":      "mock_ciphertext",
			"nonce":           "mock_nonce",
			"associated_data": "transaction",
			"original_type":   "transaction",
		},
		"summary": "success",
	}

	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-pay/notify", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Wechatpay-Timestamp", "1234567890")
	req.Header.Set("Wechatpay-Nonce", "test_nonce")
	req.Header.Set("Wechatpay-Signature", "test_signature")
	req.Header.Set("Wechatpay-Serial", "test_serial")

	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	// 4) 处理支付成功任务
	payloads := distributor.Payloads()
	require.Len(t, payloads, 1)

	payloadBytes, err := json.Marshal(payloads[0])
	require.NoError(t, err)

	processor := worker.NewTestTaskProcessor(store, worker.NewNoopTaskDistributor(), nil, nil)
	task := asynq.NewTask(worker.TaskProcessPaymentSuccess, payloadBytes)
	require.NoError(t, processor.ProcessTaskPaymentSuccess(ctx, task))

	// 5) 商户确认预订
	{
		url := fmt.Sprintf("/v1/reservations/%d/confirm", created.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 6) 商户标记未到店
	{
		url := fmt.Sprintf("/v1/reservations/%d/no-show", created.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		var resp reservationStatusResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, "no_show", resp.Status)
	}

	updatedTable, err := store.GetTable(ctx, room.ID)
	require.NoError(t, err)
	require.Equal(t, "available", updatedTable.Status)
}

// TestReservationJourneyCCancelRefundIntegration
// 包间预订异常链路（C-Cancel）：退款截止前取消预订触发退款。
func TestReservationJourneyCCancelRefundIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)

	room := createIntegrationRoomTable(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)

	reservationDate := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	reservationTime := "19:00"

	// 1) 创建预订
	var created reservationStatusResponse
	{
		body := map[string]any{
			"table_id":      room.ID,
			"date":          reservationDate,
			"time":          reservationTime,
			"guest_count":   2,
			"contact_name":  "张三",
			"contact_phone": "13800138001",
			"payment_mode":  "deposit",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/reservations", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	// 2) 创建预订支付单
	var payment takeoutPaymentOrderResponse
	{
		payBody := map[string]any{
			"order_id":      created.ID,
			"payment_type":  "native",
			"business_type": "reservation",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/payments", payBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payment)
		require.Equal(t, "pending", payment.Status)
	}

	po, err := store.GetPaymentOrder(ctx, payment.ID)
	require.NoError(t, err)

	// 3) 直接标记支付成功（该路径在 C2-C5 已覆盖）
	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            po.ID,
		TransactionID: pgtype.Text{String: "tx_cancel_001", Valid: true},
	})
	require.NoError(t, err)
	_, err = store.UpdateReservationStatus(ctx, db.UpdateReservationStatusParams{
		ID:     created.ID,
		Status: "paid",
	})
	require.NoError(t, err)

	currentReservation, err := store.GetTableReservation(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "paid", currentReservation.Status)
	require.True(t, time.Now().Before(currentReservation.RefundDeadline))

	currentPayment, err := store.GetPaymentOrder(ctx, payment.ID)
	require.NoError(t, err)
	require.Equal(t, "paid", currentPayment.Status)
	require.Greater(t, currentPayment.Amount, int64(0))

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentClient := mockwechat.NewMockPaymentClientInterface(ctrl)
	mockPaymentClient.EXPECT().
		CreateRefund(gomock.Any(), gomock.Any()).
		Times(1).
		Return(&wechat.RefundResponse{Status: wechat.RefundStatusSuccess, RefundID: "refund_001"}, nil)

	server.SetPaymentClientForTest(mockPaymentClient)
	defer server.SetPaymentClientForTest(nil)

	// 5) 取消预订（退款截止前）
	{
		body := map[string]any{
			"reason": "changed_plan",
		}
		url := fmt.Sprintf("/v1/reservations/%d/cancel", created.ID)
		rec := doJSON(t, server, http.MethodPost, url, body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		var resp reservationStatusResponse
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.Equal(t, "cancelled", resp.Status)
	}

	updatedPayment, err := store.GetPaymentOrder(ctx, payment.ID)
	require.NoError(t, err)
	require.Equal(t, "refunded", updatedPayment.Status)

	refunds, err := store.ListRefundOrdersByPaymentOrder(ctx, payment.ID)
	require.NoError(t, err)
	require.NotEmpty(t, refunds)
}

// TestReservationJourneyCCancelAfterDeadlineIntegration
// 包间预订异常链路（C-Cancel-Deadline）：退款截止后用户取消会被拒绝。
func TestReservationJourneyCCancelAfterDeadlineIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)

	room := createIntegrationRoomTable(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)

	reservationDate := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	reservationTime := "19:00"

	// 1) 创建预订
	var created reservationStatusResponse
	{
		body := map[string]any{
			"table_id":      room.ID,
			"date":          reservationDate,
			"time":          reservationTime,
			"guest_count":   2,
			"contact_name":  "张三",
			"contact_phone": "13800138001",
			"payment_mode":  "deposit",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/reservations", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	// 2) 创建预订支付单
	var payment takeoutPaymentOrderResponse
	{
		payBody := map[string]any{
			"order_id":      created.ID,
			"payment_type":  "native",
			"business_type": "reservation",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/payments", payBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payment)
		require.Equal(t, "pending", payment.Status)
	}

	po, err := store.GetPaymentOrder(ctx, payment.ID)
	require.NoError(t, err)

	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            po.ID,
		TransactionID: pgtype.Text{String: "tx_cancel_deadline_001", Valid: true},
	})
	require.NoError(t, err)
	_, err = store.UpdateReservationStatus(ctx, db.UpdateReservationStatusParams{
		ID:     created.ID,
		Status: "paid",
	})
	require.NoError(t, err)

	_, err = integrationPool.Exec(ctx,
		`UPDATE table_reservations SET refund_deadline = $2 WHERE id = $1`,
		created.ID,
		time.Now().Add(-10*time.Minute),
	)
	require.NoError(t, err)

	// 3) 退款截止后取消
	{
		body := map[string]any{
			"reason": "too_late",
		}
		url := fmt.Sprintf("/v1/reservations/%d/cancel", created.ID)
		rec := doJSON(t, server, http.MethodPost, url, body, customer.ID)
		require.Equal(t, http.StatusConflict, rec.Code)
	}

	updatedReservation, err := store.GetTableReservation(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "paid", updatedReservation.Status)
}

// TestReservationJourneyCRefundNotifyIntegration
// 包间预订异常链路（C-Refund-Notify）：退款回调通知入队退款结果处理。
func TestReservationJourneyCRefundNotifyIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)

	room := createIntegrationRoomTable(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)

	reservationDate := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	reservationTime := "19:00"

	// 1) 创建预订
	var created reservationStatusResponse
	{
		body := map[string]any{
			"table_id":      room.ID,
			"date":          reservationDate,
			"time":          reservationTime,
			"guest_count":   2,
			"contact_name":  "张三",
			"contact_phone": "13800138001",
			"payment_mode":  "deposit",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/reservations", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	// 2) 创建预订支付单并标记已支付
	var payment takeoutPaymentOrderResponse
	{
		payBody := map[string]any{
			"order_id":      created.ID,
			"payment_type":  "native",
			"business_type": "reservation",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/payments", payBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payment)
		require.Equal(t, "pending", payment.Status)
	}

	po, err := store.GetPaymentOrder(ctx, payment.ID)
	require.NoError(t, err)

	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            po.ID,
		TransactionID: pgtype.Text{String: "tx_refund_notify_001", Valid: true},
	})
	require.NoError(t, err)
	_, err = store.UpdateReservationStatus(ctx, db.UpdateReservationStatusParams{
		ID:     created.ID,
		Status: "paid",
	})
	require.NoError(t, err)

	outRefundNo := "refund_notify_" + util.RandomString(8)
	_, err = store.CreateRefundOrder(ctx, db.CreateRefundOrderParams{
		PaymentOrderID: po.ID,
		RefundType:     "miniprogram",
		RefundAmount:   po.Amount,
		RefundReason:   pgtype.Text{String: "预定取消退款", Valid: true},
		OutRefundNo:    outRefundNo,
		Status:         "processing",
	})
	require.NoError(t, err)

	// 3) 退款回调通知
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPaymentClient := mockwechat.NewMockPaymentClientInterface(ctrl)
	mockPaymentClient.EXPECT().
		VerifyNotificationSignature(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1).
		Return(nil)

	mockPaymentClient.EXPECT().
		DecryptRefundNotification(gomock.Any()).
		Times(1).
		Return(&wechat.RefundNotificationResource{
			OutTradeNo:   po.OutTradeNo,
			OutRefundNo:  outRefundNo,
			RefundID:     "refund_notify_id_001",
			RefundStatus: "SUCCESS",
			Amount: struct {
				Total       int64 `json:"total"`
				Refund      int64 `json:"refund"`
				PayerTotal  int64 `json:"payer_total"`
				PayerRefund int64 `json:"payer_refund"`
			}{
				Total:       po.Amount,
				Refund:      po.Amount,
				PayerTotal:  po.Amount,
				PayerRefund: po.Amount,
			},
		}, nil)

	distributor := &captureRefundResultDistributor{}
	server.SetPaymentClientForTest(mockPaymentClient)
	server.SetTaskDistributorForTest(distributor)
	defer server.SetPaymentClientForTest(nil)
	defer server.SetTaskDistributorForTest(nil)

	notificationID := "refund_notify_" + util.RandomString(8)
	body := map[string]any{
		"id":            notificationID,
		"event_type":    "REFUND.SUCCESS",
		"resource_type": "encrypt-resource",
		"resource": map[string]any{
			"algorithm":       "AEAD_AES_256_GCM",
			"ciphertext":      "mock_ciphertext",
			"nonce":           "mock_nonce",
			"associated_data": "refund",
		},
		"summary": "refund success",
	}

	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/v1/webhooks/wechat-pay/refund-notify", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Wechatpay-Timestamp", "1234567890")
	req.Header.Set("Wechatpay-Nonce", "test_nonce")
	req.Header.Set("Wechatpay-Signature", "test_signature")
	req.Header.Set("Wechatpay-Serial", "test_serial")

	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	payloads := distributor.Payloads()
	require.Len(t, payloads, 1)
	require.Equal(t, outRefundNo, payloads[0].OutRefundNo)
	require.Equal(t, "SUCCESS", payloads[0].RefundStatus)
	require.Equal(t, "refund_notify_id_001", payloads[0].RefundID)
}

// TestClaimJourneyD1Integration
// 索赔旅程（D1）端到端验收：完成订单后提交索赔并落库。
func TestClaimJourneyD1Integration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 3) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
		require.Equal(t, "instant", claimResp.Status)
		require.NotNil(t, claimResp.ApprovedAmount)
		require.Equal(t, order.TotalAmount, *claimResp.ApprovedAmount)
	}

	claim, err := store.GetClaim(ctx, claimResp.ClaimID)
	require.NoError(t, err)
	require.Equal(t, orderID, claim.OrderID)

	recovery, err := store.GetClaimRecoveryByClaimID(ctx, claim.ID)
	require.NoError(t, err)
	require.Equal(t, "pending", recovery.Status)
}

// TestClaimJourneyD2MerchantAppealIntegration
// 索赔旅程（D2）端到端验收：商户对已批准索赔提交申诉。
func TestClaimJourneyD2MerchantAppealIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 3) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
		require.Equal(t, "instant", claimResp.Status)
	}

	// 4) 商户申诉
	var appealResp appealSubmitResponse
	{
		appealBody := map[string]any{
			"claim_id": claimResp.ClaimID,
			"reason":   "evidence shows no foreign object in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/merchant/appeals", appealBody, merchantOwner.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &appealResp)
		require.NotZero(t, appealResp.ID)
		require.Equal(t, claimResp.ClaimID, appealResp.ClaimID)
		require.Equal(t, "merchant", appealResp.AppellantType)
		require.Equal(t, merchant.ID, appealResp.AppellantID)
		require.Equal(t, "pending", appealResp.Status)
	}

	appeal, err := store.GetAppealByClaim(ctx, db.GetAppealByClaimParams{
		ClaimID:       claimResp.ClaimID,
		AppellantType: "merchant",
	})
	require.NoError(t, err)
	require.Equal(t, appealResp.ID, appeal.ID)
	require.Equal(t, "pending", appeal.Status)
}

// TestClaimJourneyD3RiderAppealIntegration
// 索赔旅程（D3）端到端验收：骑手对关联索赔提交申诉。
func TestClaimJourneyD3RiderAppealIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err = store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 创建支付单并模拟支付成功
	var payment takeoutPaymentOrderResponse
	{
		payBody := map[string]any{
			"order_id":      orderID,
			"payment_type":  "native",
			"business_type": "order",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/payments", payBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payment)
	}

	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: "integration_tx_d3_001", Valid: true},
	})
	require.NoError(t, err)

	payRes, err := store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     payment.ID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	})
	require.NoError(t, err)
	require.True(t, payRes.Processed)

	// 3) 商户接单/出餐
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/accept", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/ready", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 4) 骑手抢单，建立配送关联
	{
		url := fmt.Sprintf("/v1/delivery/grab/%d", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 5) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "ready",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 6) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
		require.Equal(t, "instant", claimResp.Status)
	}

	// 7) 骑手申诉
	var appealResp appealSubmitResponse
	{
		appealBody := map[string]any{
			"claim_id": claimResp.ClaimID,
			"reason":   "delivery handled properly with no issues",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/rider/appeals", appealBody, riderUser.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &appealResp)
		require.NotZero(t, appealResp.ID)
		require.Equal(t, claimResp.ClaimID, appealResp.ClaimID)
		require.Equal(t, "rider", appealResp.AppellantType)
		require.Equal(t, rider.ID, appealResp.AppellantID)
		require.Equal(t, "pending", appealResp.Status)
	}

	appeal, err := store.GetAppealByClaim(ctx, db.GetAppealByClaimParams{
		ClaimID:       claimResp.ClaimID,
		AppellantType: "rider",
	})
	require.NoError(t, err)
	require.Equal(t, appealResp.ID, appeal.ID)
	require.Equal(t, "pending", appeal.Status)
}

// TestClaimJourneyD4OperatorReviewIntegration
// 索赔旅程（D4）端到端验收：运营商审核申诉并写入结果。
func TestClaimJourneyD4OperatorReviewIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	operatorUser := createIntegrationUser(t, store)
	commissionRate := pgtype.Numeric{}
	_ = commissionRate.Scan("0.03")
	operator, err := store.CreateOperator(ctx, db.CreateOperatorParams{
		UserID:            operatorUser.ID,
		RegionID:          region.ID,
		Name:              "op_" + util.RandomString(6),
		ContactName:       "contact_" + util.RandomString(6),
		ContactPhone:      "138" + util.RandomString(8),
		WechatMchID:       pgtype.Text{Valid: false},
		CommissionRate:    commissionRate,
		Status:            "active",
		ContractStartDate: pgtype.Date{Valid: false},
		ContractEndDate:   pgtype.Date{Valid: false},
		ContractYears:     1,
	})
	require.NoError(t, err)
	_, err = store.CreateUserRole(ctx, db.CreateUserRoleParams{
		UserID:          operatorUser.ID,
		Role:            api.RoleOperator,
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.NoError(t, err)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 3) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
		require.Equal(t, "instant", claimResp.Status)
	}

	// 4) 商户申诉
	var appealResp appealSubmitResponse
	{
		appealBody := map[string]any{
			"claim_id": claimResp.ClaimID,
			"reason":   "evidence shows no foreign object in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/merchant/appeals", appealBody, merchantOwner.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &appealResp)
		require.NotZero(t, appealResp.ID)
		require.Equal(t, claimResp.ClaimID, appealResp.ClaimID)
		require.Equal(t, "merchant", appealResp.AppellantType)
		require.Equal(t, merchant.ID, appealResp.AppellantID)
		require.Equal(t, "pending", appealResp.Status)
	}

	// 5) 运营商审核申诉
	var reviewed appealSubmitResponse
	{
		reviewBody := map[string]any{
			"status":              "approved",
			"review_notes":        "appeal verified by operator",
			"compensation_amount": int64(500),
		}
		url := fmt.Sprintf("/v1/operator/appeals/%d/review", appealResp.ID)
		rec := doJSON(t, server, http.MethodPost, url, reviewBody, operatorUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &reviewed)
		require.Equal(t, appealResp.ID, reviewed.ID)
		require.Equal(t, "approved", reviewed.Status)
	}

	appeal, err := store.GetAppeal(ctx, appealResp.ID)
	require.NoError(t, err)
	require.Equal(t, "approved", appeal.Status)
	require.True(t, appeal.ReviewerID.Valid)
	require.Equal(t, operator.ID, appeal.ReviewerID.Int64)
	require.True(t, appeal.CompensationAmount.Valid)
	require.Equal(t, int64(500), appeal.CompensationAmount.Int64)
}

// TestClaimJourneyD5AppealReviewNotificationsIntegration
// 索赔旅程（D5）端到端验收：申诉审核后的追偿回滚与通知落库。
func TestClaimJourneyD5AppealReviewNotificationsIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	operatorUser := createIntegrationUser(t, store)
	commissionRate := pgtype.Numeric{}
	_ = commissionRate.Scan("0.03")
	operator, err := store.CreateOperator(ctx, db.CreateOperatorParams{
		UserID:            operatorUser.ID,
		RegionID:          region.ID,
		Name:              "op_" + util.RandomString(6),
		ContactName:       "contact_" + util.RandomString(6),
		ContactPhone:      "138" + util.RandomString(8),
		WechatMchID:       pgtype.Text{Valid: false},
		CommissionRate:    commissionRate,
		Status:            "active",
		ContractStartDate: pgtype.Date{Valid: false},
		ContractEndDate:   pgtype.Date{Valid: false},
		ContractYears:     1,
	})
	require.NoError(t, err)
	_, err = store.CreateUserRole(ctx, db.CreateUserRoleParams{
		UserID:          operatorUser.ID,
		Role:            api.RoleOperator,
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.NoError(t, err)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 3) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
		require.Equal(t, "instant", claimResp.Status)
	}

	// 4) 商户申诉
	var appealResp appealSubmitResponse
	{
		appealBody := map[string]any{
			"claim_id": claimResp.ClaimID,
			"reason":   "evidence shows no foreign object in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/merchant/appeals", appealBody, merchantOwner.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &appealResp)
		require.NotZero(t, appealResp.ID)
		require.Equal(t, claimResp.ClaimID, appealResp.ClaimID)
		require.Equal(t, "merchant", appealResp.AppellantType)
		require.Equal(t, merchant.ID, appealResp.AppellantID)
		require.Equal(t, "pending", appealResp.Status)
	}

	// 5) 运营商审核申诉
	{
		reviewBody := map[string]any{
			"status":              "approved",
			"review_notes":        "appeal verified by operator",
			"compensation_amount": int64(500),
		}
		url := fmt.Sprintf("/v1/operator/appeals/%d/review", appealResp.ID)
		rec := doJSON(t, server, http.MethodPost, url, reviewBody, operatorUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	recovery, err := store.GetClaimRecoveryByClaimID(ctx, claimResp.ClaimID)
	require.NoError(t, err)
	require.Equal(t, "waived", recovery.Status)

	notifications, err := store.GetNotificationsByRelated(ctx, db.GetNotificationsByRelatedParams{
		RelatedType: pgtype.Text{String: "appeal", Valid: true},
		RelatedID:   pgtype.Int8{Int64: appealResp.ID, Valid: true},
	})
	require.NoError(t, err)
	require.Len(t, notifications, 2)

	userIDs := map[int64]bool{}
	for _, n := range notifications {
		userIDs[n.UserID] = true
		require.Equal(t, "appeal", n.Type)
	}
	require.True(t, userIDs[merchantOwner.ID])
	require.True(t, userIDs[customer.ID])
}

// TestClaimJourneyD6AppealRejectRecoveryResumeIntegration
// 索赔旅程（D6）端到端验收：申诉驳回后追偿恢复与通知落库。
func TestClaimJourneyD6AppealRejectRecoveryResumeIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	operatorUser := createIntegrationUser(t, store)
	commissionRate := pgtype.Numeric{}
	_ = commissionRate.Scan("0.03")
	operator, err := store.CreateOperator(ctx, db.CreateOperatorParams{
		UserID:            operatorUser.ID,
		RegionID:          region.ID,
		Name:              "op_" + util.RandomString(6),
		ContactName:       "contact_" + util.RandomString(6),
		ContactPhone:      "138" + util.RandomString(8),
		WechatMchID:       pgtype.Text{Valid: false},
		CommissionRate:    commissionRate,
		Status:            "active",
		ContractStartDate: pgtype.Date{Valid: false},
		ContractEndDate:   pgtype.Date{Valid: false},
		ContractYears:     1,
	})
	require.NoError(t, err)
	_, err = store.CreateUserRole(ctx, db.CreateUserRoleParams{
		UserID:          operatorUser.ID,
		Role:            api.RoleOperator,
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.NoError(t, err)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 3) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
		require.Equal(t, "instant", claimResp.Status)
	}

	// 4) 商户申诉
	var appealResp appealSubmitResponse
	{
		appealBody := map[string]any{
			"claim_id": claimResp.ClaimID,
			"reason":   "evidence shows no foreign object in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/merchant/appeals", appealBody, merchantOwner.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &appealResp)
		require.NotZero(t, appealResp.ID)
		require.Equal(t, claimResp.ClaimID, appealResp.ClaimID)
		require.Equal(t, "merchant", appealResp.AppellantType)
		require.Equal(t, merchant.ID, appealResp.AppellantID)
		require.Equal(t, "pending", appealResp.Status)
	}

	// 5) 运营商驳回申诉
	{
		reviewBody := map[string]any{
			"status":       "rejected",
			"review_notes": "insufficient evidence for appeal",
		}
		url := fmt.Sprintf("/v1/operator/appeals/%d/review", appealResp.ID)
		rec := doJSON(t, server, http.MethodPost, url, reviewBody, operatorUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	recovery, err := store.GetClaimRecoveryByClaimID(ctx, claimResp.ClaimID)
	require.NoError(t, err)
	require.Equal(t, "pending", recovery.Status)

	notifications, err := store.GetNotificationsByRelated(ctx, db.GetNotificationsByRelatedParams{
		RelatedType: pgtype.Text{String: "appeal", Valid: true},
		RelatedID:   pgtype.Int8{Int64: appealResp.ID, Valid: true},
	})
	require.NoError(t, err)
	require.Len(t, notifications, 2)

	userIDs := map[int64]bool{}
	for _, n := range notifications {
		userIDs[n.UserID] = true
		require.Equal(t, "appeal", n.Type)
	}
	require.True(t, userIDs[merchantOwner.ID])
	require.True(t, userIDs[customer.ID])
}

// TestClaimJourneyD7AppealReviewEnqueueIntegration
// 索赔旅程（D7）端到端验收：审核时入队申诉后处理任务。
func TestClaimJourneyD7AppealReviewEnqueueIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	operatorUser := createIntegrationUser(t, store)
	commissionRate := pgtype.Numeric{}
	_ = commissionRate.Scan("0.03")
	operator, err := store.CreateOperator(ctx, db.CreateOperatorParams{
		UserID:            operatorUser.ID,
		RegionID:          region.ID,
		Name:              "op_" + util.RandomString(6),
		ContactName:       "contact_" + util.RandomString(6),
		ContactPhone:      "138" + util.RandomString(8),
		WechatMchID:       pgtype.Text{Valid: false},
		CommissionRate:    commissionRate,
		Status:            "active",
		ContractStartDate: pgtype.Date{Valid: false},
		ContractEndDate:   pgtype.Date{Valid: false},
		ContractYears:     1,
	})
	require.NoError(t, err)
	_, err = store.CreateUserRole(ctx, db.CreateUserRoleParams{
		UserID:          operatorUser.ID,
		Role:            api.RoleOperator,
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.NoError(t, err)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 3) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
		require.Equal(t, "instant", claimResp.Status)
	}

	// 4) 商户申诉
	var appealResp appealSubmitResponse
	{
		appealBody := map[string]any{
			"claim_id": claimResp.ClaimID,
			"reason":   "evidence shows no foreign object in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/merchant/appeals", appealBody, merchantOwner.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &appealResp)
		require.NotZero(t, appealResp.ID)
		require.Equal(t, claimResp.ClaimID, appealResp.ClaimID)
		require.Equal(t, "merchant", appealResp.AppellantType)
		require.Equal(t, merchant.ID, appealResp.AppellantID)
		require.Equal(t, "pending", appealResp.Status)
	}

	distributor := &captureAppealResultDistributor{}
	server.SetTaskDistributorForTest(distributor)
	defer server.SetTaskDistributorForTest(nil)

	// 5) 运营商审核申诉（入队）
	{
		reviewBody := map[string]any{
			"status":              "approved",
			"review_notes":        "appeal verified by operator",
			"compensation_amount": int64(500),
		}
		url := fmt.Sprintf("/v1/operator/appeals/%d/review", appealResp.ID)
		rec := doJSON(t, server, http.MethodPost, url, reviewBody, operatorUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	payloads := distributor.Payloads()
	require.Len(t, payloads, 1)
	require.Equal(t, appealResp.ID, payloads[0].AppealID)
	require.Equal(t, claimResp.ClaimID, payloads[0].ClaimID)
	require.Equal(t, "approved", payloads[0].Status)
	require.Equal(t, "merchant", payloads[0].AppellantType)
	require.Equal(t, merchant.ID, payloads[0].AppellantID)
	require.Equal(t, customer.ID, payloads[0].ClaimantUserID)

	recovery, err := store.GetClaimRecoveryByClaimID(ctx, claimResp.ClaimID)
	require.NoError(t, err)
	require.Equal(t, "appealed", recovery.Status)
}

// TestClaimJourneyD8AppealResultWorkerIntegration
// 索赔旅程（D8）端到端验收：申诉处理任务执行后回滚追偿并发送通知。
func TestClaimJourneyD8AppealResultWorkerIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 3) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
		require.Equal(t, "instant", claimResp.Status)
	}

	// 4) 商户申诉
	var appealResp appealSubmitResponse
	{
		appealBody := map[string]any{
			"claim_id": claimResp.ClaimID,
			"reason":   "evidence shows no foreign object in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/merchant/appeals", appealBody, merchantOwner.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &appealResp)
		require.NotZero(t, appealResp.ID)
		require.Equal(t, claimResp.ClaimID, appealResp.ClaimID)
		require.Equal(t, "merchant", appealResp.AppellantType)
		require.Equal(t, merchant.ID, appealResp.AppellantID)
		require.Equal(t, "pending", appealResp.Status)
	}

	recovery, err := store.GetClaimRecoveryByClaimID(ctx, claimResp.ClaimID)
	require.NoError(t, err)
	require.Equal(t, "appealed", recovery.Status)

	claim, err := store.GetClaim(ctx, claimResp.ClaimID)
	require.NoError(t, err)

	distributor := &captureSendNotificationDistributor{}
	processor := worker.NewTestTaskProcessor(store, distributor, nil, nil)

	payload := worker.ProcessAppealResultPayload{
		AppealID:           appealResp.ID,
		ClaimID:            claimResp.ClaimID,
		Status:             "approved",
		AppellantType:      "merchant",
		AppellantID:        merchant.ID,
		ClaimantUserID:     customer.ID,
		ClaimType:          claim.ClaimType,
		ClaimAmount:        claim.ClaimAmount,
		CompensationAmount: 500,
		OrderNo:            order.OrderNo,
	}

	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)
	if err := processor.ProcessTaskProcessAppealResult(ctx, asynq.NewTask(worker.TaskProcessAppealResult, payloadBytes)); err != nil {
		require.NoError(t, err)
	}

	recovery, err = store.GetClaimRecoveryByClaimID(ctx, claimResp.ClaimID)
	require.NoError(t, err)
	require.Equal(t, "waived", recovery.Status)

	notifications := distributor.Payloads()
	require.Len(t, notifications, 2)

	userIDs := map[int64]bool{}
	for _, n := range notifications {
		userIDs[n.UserID] = true
		require.Equal(t, "appeal", n.Type)
		require.Equal(t, appealResp.ID, n.RelatedID)
	}
	require.True(t, userIDs[merchantOwner.ID])
	require.True(t, userIDs[customer.ID])
}

// TestClaimJourneyD9AppealResultWorkerRejectIntegration
// 索赔旅程（D9）端到端验收：申诉处理任务驳回时恢复追偿并发送通知。
func TestClaimJourneyD9AppealResultWorkerRejectIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 3) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
		require.Equal(t, "instant", claimResp.Status)
	}

	// 4) 商户申诉
	var appealResp appealSubmitResponse
	{
		appealBody := map[string]any{
			"claim_id": claimResp.ClaimID,
			"reason":   "evidence shows no foreign object in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/merchant/appeals", appealBody, merchantOwner.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &appealResp)
		require.NotZero(t, appealResp.ID)
		require.Equal(t, claimResp.ClaimID, appealResp.ClaimID)
		require.Equal(t, "merchant", appealResp.AppellantType)
		require.Equal(t, merchant.ID, appealResp.AppellantID)
		require.Equal(t, "pending", appealResp.Status)
	}

	claim, err := store.GetClaim(ctx, claimResp.ClaimID)
	require.NoError(t, err)

	distributor := &captureSendNotificationDistributor{}
	processor := worker.NewTestTaskProcessor(store, distributor, nil, nil)

	payload := worker.ProcessAppealResultPayload{
		AppealID:           appealResp.ID,
		ClaimID:            claimResp.ClaimID,
		Status:             "rejected",
		AppellantType:      "merchant",
		AppellantID:        merchant.ID,
		ClaimantUserID:     customer.ID,
		ClaimType:          claim.ClaimType,
		ClaimAmount:        claim.ClaimAmount,
		CompensationAmount: 0,
		OrderNo:            order.OrderNo,
	}

	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)
	if err := processor.ProcessTaskProcessAppealResult(ctx, asynq.NewTask(worker.TaskProcessAppealResult, payloadBytes)); err != nil {
		require.NoError(t, err)
	}

	recovery, err := store.GetClaimRecoveryByClaimID(ctx, claimResp.ClaimID)
	require.NoError(t, err)
	require.Equal(t, "pending", recovery.Status)

	notifications := distributor.Payloads()
	require.Len(t, notifications, 2)

	userIDs := map[int64]bool{}
	for _, n := range notifications {
		userIDs[n.UserID] = true
		require.Equal(t, "appeal", n.Type)
		require.Equal(t, appealResp.ID, n.RelatedID)
	}
	require.True(t, userIDs[merchantOwner.ID])
	require.True(t, userIDs[customer.ID])
}

// TestClaimJourneyD10MerchantRecoveryPayIntegration
// 索赔旅程（D10）端到端验收：商户支付追偿单并写入结算调整。
func TestClaimJourneyD10MerchantRecoveryPayIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 3) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
		require.Equal(t, "instant", claimResp.Status)
	}

	recovery, err := store.GetClaimRecoveryByClaimID(ctx, claimResp.ClaimID)
	require.NoError(t, err)
	require.Equal(t, "pending", recovery.Status)
	require.True(t, recovery.RecoveryTarget.Valid)
	require.Equal(t, "merchant", recovery.RecoveryTarget.String)

	// 4) 商户支付追偿单
	var payResp claimRecoveryStatusResponse
	{
		url := fmt.Sprintf("/v1/merchant/claims/%d/recovery/pay", claimResp.ClaimID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payResp)
		require.Equal(t, "paid", payResp.Status)
	}

	recovery, err = store.GetClaimRecoveryByClaimID(ctx, claimResp.ClaimID)
	require.NoError(t, err)
	require.Equal(t, "paid", recovery.Status)

	_, err = store.GetMerchantSettlementAdjustmentByRelatedAndType(ctx, db.GetMerchantSettlementAdjustmentByRelatedAndTypeParams{
		RelatedType:    pgtype.Text{String: "claim_recovery", Valid: true},
		RelatedID:      pgtype.Int8{Int64: recovery.ID, Valid: true},
		AdjustmentType: "claim_recovery_charge",
	})
	require.NoError(t, err)
}

// TestClaimJourneyD11OperatorRecoveryWaiveIntegration
// 索赔旅程（D11）端到端验收：运营商核销追偿单。
func TestClaimJourneyD11OperatorRecoveryWaiveIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	operatorUser := createIntegrationUser(t, store)
	commissionRate := pgtype.Numeric{}
	_ = commissionRate.Scan("0.03")
	operator, err := store.CreateOperator(ctx, db.CreateOperatorParams{
		UserID:            operatorUser.ID,
		RegionID:          region.ID,
		Name:              "op_" + util.RandomString(6),
		ContactName:       "contact_" + util.RandomString(6),
		ContactPhone:      "138" + util.RandomString(8),
		WechatMchID:       pgtype.Text{Valid: false},
		CommissionRate:    commissionRate,
		Status:            "active",
		ContractStartDate: pgtype.Date{Valid: false},
		ContractEndDate:   pgtype.Date{Valid: false},
		ContractYears:     1,
	})
	require.NoError(t, err)
	_, err = store.CreateUserRole(ctx, db.CreateUserRoleParams{
		UserID:          operatorUser.ID,
		Role:            api.RoleOperator,
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.NoError(t, err)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 3) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
		require.Equal(t, "instant", claimResp.Status)
	}

	recovery, err := store.GetClaimRecoveryByClaimID(ctx, claimResp.ClaimID)
	require.NoError(t, err)
	require.Equal(t, "pending", recovery.Status)

	// 4) 运营商核销追偿单
	var waiveResp claimRecoveryStatusResponse
	{
		url := fmt.Sprintf("/v1/operator/claims/%d/recovery/waive", claimResp.ClaimID)
		rec := doJSON(t, server, http.MethodPost, url, nil, operatorUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &waiveResp)
		require.Equal(t, "waived", waiveResp.Status)
	}

	recovery, err = store.GetClaimRecoveryByClaimID(ctx, claimResp.ClaimID)
	require.NoError(t, err)
	require.Equal(t, "waived", recovery.Status)
}

// TestClaimJourneyD12RiderRecoveryPayIntegration
// 索赔旅程（D12）端到端验收：骑手支付追偿单并恢复接单。
func TestClaimJourneyD12RiderRecoveryPayIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err = store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 创建支付单并模拟支付成功
	var payment takeoutPaymentOrderResponse
	{
		payBody := map[string]any{
			"order_id":      orderID,
			"payment_type":  "native",
			"business_type": "order",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/payments", payBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payment)
	}

	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: "integration_tx_d12_001", Valid: true},
	})
	require.NoError(t, err)

	payRes, err := store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     payment.ID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	})
	require.NoError(t, err)
	require.True(t, payRes.Processed)

	// 3) 商户接单/出餐
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/accept", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/ready", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 4) 骑手抢单，建立配送关联
	{
		url := fmt.Sprintf("/v1/delivery/grab/%d", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 5) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "ready",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 6) 提交索赔（餐损类使 rider 为追偿对象）
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "damage",
			"claim_amount": order.TotalAmount,
			"claim_reason": "food was damaged during delivery",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
	}

	recovery, err := store.GetClaimRecoveryByClaimID(ctx, claimResp.ClaimID)
	require.NoError(t, err)
	require.Equal(t, "pending", recovery.Status)
	require.True(t, recovery.RecoveryTarget.Valid)
	require.Equal(t, "rider", recovery.RecoveryTarget.String)

	// 7) 骑手支付追偿单
	var payResp claimRecoveryStatusResponse
	{
		url := fmt.Sprintf("/v1/rider/claims/%d/recovery/pay", claimResp.ClaimID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payResp)
		require.Equal(t, "paid", payResp.Status)
	}

	recovery, err = store.GetClaimRecoveryByClaimID(ctx, claimResp.ClaimID)
	require.NoError(t, err)
	require.Equal(t, "paid", recovery.Status)
}

// TestClaimJourneyD13OperatorRecoveryViewIntegration
// 索赔旅程（D13）端到端验收：运营商查看追偿单详情。
func TestClaimJourneyD13OperatorRecoveryViewIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	operatorUser := createIntegrationUser(t, store)
	commissionRate := pgtype.Numeric{}
	_ = commissionRate.Scan("0.03")
	operator, err := store.CreateOperator(ctx, db.CreateOperatorParams{
		UserID:            operatorUser.ID,
		RegionID:          region.ID,
		Name:              "op_" + util.RandomString(6),
		ContactName:       "contact_" + util.RandomString(6),
		ContactPhone:      "138" + util.RandomString(8),
		WechatMchID:       pgtype.Text{Valid: false},
		CommissionRate:    commissionRate,
		Status:            "active",
		ContractStartDate: pgtype.Date{Valid: false},
		ContractEndDate:   pgtype.Date{Valid: false},
		ContractYears:     1,
	})
	require.NoError(t, err)
	_, err = store.CreateUserRole(ctx, db.CreateUserRoleParams{
		UserID:          operatorUser.ID,
		Role:            api.RoleOperator,
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.NoError(t, err)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 3) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
		require.Equal(t, "instant", claimResp.Status)
	}

	// 4) 运营商查看追偿单
	var recoveryResp claimRecoveryStatusResponse
	{
		url := fmt.Sprintf("/v1/operator/claims/%d/recovery", claimResp.ClaimID)
		rec := doGET(t, server, url, operatorUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &recoveryResp)
		require.Equal(t, "pending", recoveryResp.Status)
		require.NotNil(t, recoveryResp.RecoveryTarget)
		require.Equal(t, "merchant", *recoveryResp.RecoveryTarget)
	}
}

// TestClaimJourneyD14MerchantRecoveryViewIntegration
// 索赔旅程（D14）端到端验收：商户查看追偿单详情。
func TestClaimJourneyD14MerchantRecoveryViewIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 3) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
		require.Equal(t, "instant", claimResp.Status)
	}

	// 4) 商户查看追偿单
	var recoveryResp claimRecoveryStatusResponse
	{
		url := fmt.Sprintf("/v1/merchant/claims/%d/recovery", claimResp.ClaimID)
		rec := doGET(t, server, url, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &recoveryResp)
		require.Equal(t, "pending", recoveryResp.Status)
		require.NotNil(t, recoveryResp.RecoveryTarget)
		require.Equal(t, "merchant", *recoveryResp.RecoveryTarget)
	}
}

// TestClaimJourneyD15RiderRecoveryViewIntegration
// 索赔旅程（D15）端到端验收：骑手查看追偿单详情。
func TestClaimJourneyD15RiderRecoveryViewIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err = store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 创建支付单并模拟支付成功
	var payment takeoutPaymentOrderResponse
	{
		payBody := map[string]any{
			"order_id":      orderID,
			"payment_type":  "native",
			"business_type": "order",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/payments", payBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payment)
	}

	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: "integration_tx_d15_001", Valid: true},
	})
	require.NoError(t, err)

	payRes, err := store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     payment.ID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	})
	require.NoError(t, err)
	require.True(t, payRes.Processed)

	// 3) 商户接单/出餐
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/accept", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/ready", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 4) 骑手抢单，建立配送关联
	{
		url := fmt.Sprintf("/v1/delivery/grab/%d", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 5) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "ready",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 6) 提交索赔（餐损类使 rider 为追偿对象）
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "damage",
			"claim_amount": order.TotalAmount,
			"claim_reason": "food was damaged during delivery",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
	}

	// 7) 骑手查看追偿单
	var recoveryResp claimRecoveryStatusResponse
	{
		url := fmt.Sprintf("/v1/rider/claims/%d/recovery", claimResp.ClaimID)
		rec := doGET(t, server, url, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &recoveryResp)
		require.Equal(t, "pending", recoveryResp.Status)
		require.NotNil(t, recoveryResp.RecoveryTarget)
		require.Equal(t, "rider", *recoveryResp.RecoveryTarget)
	}
}

// TestClaimJourneyD16MerchantRecoveryForbiddenIntegration
// 索赔旅程（D16）端到端验收：商户查看他人追偿单应被拒绝。
func TestClaimJourneyD16MerchantRecoveryForbiddenIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	otherOwner := createIntegrationUser(t, store)
	otherMerchant, err := store.CreateMerchant(ctx, db.CreateMerchantParams{
		OwnerUserID:     otherOwner.ID,
		Name:            "集成测试餐厅-其他",
		Description:     pgtype.Text{String: "integration", Valid: true},
		LogoUrl:         pgtype.Text{String: "https://example.com/logo.png", Valid: true},
		Phone:           fmt.Sprintf("139%08d", util.RandomInt(0, 99999999)),
		Address:         "测试地址-" + util.RandomString(6),
		Latitude:        pgtype.Numeric{Valid: false},
		Longitude:       pgtype.Numeric{Valid: false},
		Status:          "approved",
		ApplicationData: []byte("{}"),
		RegionID:        region.ID,
	})
	require.NoError(t, err)
	_, err = store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: otherMerchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: otherMerchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	otherMerchant = ensureIntegrationMerchantCoords(t, store, otherMerchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 3) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
		require.Equal(t, "instant", claimResp.Status)
	}

	// 4) 非所属商户查看追偿单应被拒绝
	url := fmt.Sprintf("/v1/merchant/claims/%d/recovery", claimResp.ClaimID)
	rec := doGET(t, server, url, otherOwner.ID)
	require.Equal(t, http.StatusForbidden, rec.Code)
}

// TestClaimJourneyD17RiderRecoveryForbiddenIntegration
// 索赔旅程（D17）端到端验收：骑手查看他人追偿单应被拒绝。
func TestClaimJourneyD17RiderRecoveryForbiddenIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err = store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	otherRiderUser := createIntegrationUser(t, store)
	otherRider, err := store.CreateRider(ctx, db.CreateRiderParams{
		UserID:   otherRiderUser.ID,
		RealName: "测试骑手-其他",
		IDCardNo: fmt.Sprintf("11010119900101%04d", util.RandomInt(0, 9999)),
		Phone:    fmt.Sprintf("139%08d", util.RandomInt(0, 99999999)),
		RegionID: pgtype.Int8{Int64: region.ID, Valid: true},
	})
	require.NoError(t, err)
	_, err = store.CreateRiderProfile(ctx, otherRider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: otherRider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: otherRider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: otherRider.ID, IsOnline: true})
	require.NoError(t, err)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 创建支付单并模拟支付成功
	var payment takeoutPaymentOrderResponse
	{
		payBody := map[string]any{
			"order_id":      orderID,
			"payment_type":  "native",
			"business_type": "order",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/payments", payBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payment)
	}

	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: "integration_tx_d17_001", Valid: true},
	})
	require.NoError(t, err)

	payRes, err := store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     payment.ID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	})
	require.NoError(t, err)
	require.True(t, payRes.Processed)

	// 3) 商户接单/出餐
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/accept", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/ready", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 4) 骑手抢单，建立配送关联
	{
		url := fmt.Sprintf("/v1/delivery/grab/%d", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 5) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "ready",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 6) 提交索赔（餐损类使 rider 为追偿对象）
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "damage",
			"claim_amount": order.TotalAmount,
			"claim_reason": "food was damaged during delivery",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
	}

	// 7) 非所属骑手查看追偿单应被拒绝
	url := fmt.Sprintf("/v1/rider/claims/%d/recovery", claimResp.ClaimID)
	rec := doGET(t, server, url, otherRiderUser.ID)
	require.Equal(t, http.StatusForbidden, rec.Code)
}

// TestClaimJourneyD18OperatorRecoveryCrossRegionForbiddenIntegration
// 索赔旅程（D18）端到端验收：跨区域运营商查看追偿单应被拒绝。
func TestClaimJourneyD18OperatorRecoveryCrossRegionForbiddenIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	otherRegion := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	operatorUser := createIntegrationUser(t, store)
	commissionRate := pgtype.Numeric{}
	_ = commissionRate.Scan("0.03")
	operator, err := store.CreateOperator(ctx, db.CreateOperatorParams{
		UserID:            operatorUser.ID,
		RegionID:          otherRegion.ID,
		Name:              "op_" + util.RandomString(6),
		ContactName:       "contact_" + util.RandomString(6),
		ContactPhone:      "138" + util.RandomString(8),
		WechatMchID:       pgtype.Text{Valid: false},
		CommissionRate:    commissionRate,
		Status:            "active",
		ContractStartDate: pgtype.Date{Valid: false},
		ContractEndDate:   pgtype.Date{Valid: false},
		ContractYears:     1,
	})
	require.NoError(t, err)
	_, err = store.CreateUserRole(ctx, db.CreateUserRoleParams{
		UserID:          operatorUser.ID,
		Role:            api.RoleOperator,
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.NoError(t, err)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 3) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
		require.Equal(t, "instant", claimResp.Status)
	}

	// 4) 跨区域运营商查看追偿单应被拒绝
	url := fmt.Sprintf("/v1/operator/claims/%d/recovery", claimResp.ClaimID)
	rec := doGET(t, server, url, operatorUser.ID)
	require.Equal(t, http.StatusForbidden, rec.Code)
}

// TestClaimJourneyD19MerchantRecoveryNotFoundIntegration
// 索赔旅程（D19）端到端验收：商户查看不存在的追偿单返回 404。
func TestClaimJourneyD19MerchantRecoveryNotFoundIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	url := "/v1/merchant/claims/999999/recovery"
	rec := doGET(t, server, url, merchantOwner.ID)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

// TestClaimJourneyD20RiderRecoveryNotFoundIntegration
// 索赔旅程（D20）端到端验收：骑手查看不存在的追偿单返回 404。
func TestClaimJourneyD20RiderRecoveryNotFoundIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err := store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	url := "/v1/rider/claims/999999/recovery"
	rec := doGET(t, server, url, riderUser.ID)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

// TestClaimJourneyD21OperatorRecoveryNotFoundIntegration
// 索赔旅程（D21）端到端验收：运营商查看不存在的追偿单返回 404。
func TestClaimJourneyD21OperatorRecoveryNotFoundIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	operatorUser := createIntegrationUser(t, store)
	commissionRate := pgtype.Numeric{}
	_ = commissionRate.Scan("0.03")
	operator, err := store.CreateOperator(ctx, db.CreateOperatorParams{
		UserID:            operatorUser.ID,
		RegionID:          region.ID,
		Name:              "op_" + util.RandomString(6),
		ContactName:       "contact_" + util.RandomString(6),
		ContactPhone:      "138" + util.RandomString(8),
		WechatMchID:       pgtype.Text{Valid: false},
		CommissionRate:    commissionRate,
		Status:            "active",
		ContractStartDate: pgtype.Date{Valid: false},
		ContractEndDate:   pgtype.Date{Valid: false},
		ContractYears:     1,
	})
	require.NoError(t, err)
	_, err = store.CreateUserRole(ctx, db.CreateUserRoleParams{
		UserID:          operatorUser.ID,
		Role:            api.RoleOperator,
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.NoError(t, err)

	url := "/v1/operator/claims/999999/recovery"
	rec := doGET(t, server, url, operatorUser.ID)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

// TestClaimJourneyD22MerchantRecoveryViewAfterPayIntegration
// 索赔旅程（D22）端到端验收：商户支付后查看追偿单状态。
func TestClaimJourneyD22MerchantRecoveryViewAfterPayIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 3) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
		require.Equal(t, "instant", claimResp.Status)
	}

	// 4) 商户支付追偿单
	{
		url := fmt.Sprintf("/v1/merchant/claims/%d/recovery/pay", claimResp.ClaimID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 5) 商户查看追偿单状态
	var recoveryResp claimRecoveryStatusResponse
	{
		url := fmt.Sprintf("/v1/merchant/claims/%d/recovery", claimResp.ClaimID)
		rec := doGET(t, server, url, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &recoveryResp)
		require.Equal(t, "paid", recoveryResp.Status)
	}
}

// TestClaimJourneyD23RiderRecoveryViewAfterPayIntegration
// 索赔旅程（D23）端到端验收：骑手支付后查看追偿单状态。
func TestClaimJourneyD23RiderRecoveryViewAfterPayIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err = store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 创建支付单并模拟支付成功
	var payment takeoutPaymentOrderResponse
	{
		payBody := map[string]any{
			"order_id":      orderID,
			"payment_type":  "native",
			"business_type": "order",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/payments", payBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payment)
	}

	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: "integration_tx_d23_001", Valid: true},
	})
	require.NoError(t, err)

	payRes, err := store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     payment.ID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	})
	require.NoError(t, err)
	require.True(t, payRes.Processed)

	// 3) 商户接单/出餐
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/accept", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/ready", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 4) 骑手抢单，建立配送关联
	{
		url := fmt.Sprintf("/v1/delivery/grab/%d", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 5) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "ready",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 6) 提交索赔（餐损类使 rider 为追偿对象）
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "damage",
			"claim_amount": order.TotalAmount,
			"claim_reason": "food was damaged during delivery",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
	}

	// 7) 骑手支付追偿单
	{
		url := fmt.Sprintf("/v1/rider/claims/%d/recovery/pay", claimResp.ClaimID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 8) 骑手查看追偿单状态
	var recoveryResp claimRecoveryStatusResponse
	{
		url := fmt.Sprintf("/v1/rider/claims/%d/recovery", claimResp.ClaimID)
		rec := doGET(t, server, url, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &recoveryResp)
		require.Equal(t, "paid", recoveryResp.Status)
	}
}

// TestClaimJourneyD24OperatorRecoveryViewAfterWaiveIntegration
// 索赔旅程（D24）端到端验收：运营商核销后查看追偿单状态。
func TestClaimJourneyD24OperatorRecoveryViewAfterWaiveIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	operatorUser := createIntegrationUser(t, store)
	commissionRate := pgtype.Numeric{}
	_ = commissionRate.Scan("0.03")
	operator, err := store.CreateOperator(ctx, db.CreateOperatorParams{
		UserID:            operatorUser.ID,
		RegionID:          region.ID,
		Name:              "op_" + util.RandomString(6),
		ContactName:       "contact_" + util.RandomString(6),
		ContactPhone:      "138" + util.RandomString(8),
		WechatMchID:       pgtype.Text{Valid: false},
		CommissionRate:    commissionRate,
		Status:            "active",
		ContractStartDate: pgtype.Date{Valid: false},
		ContractEndDate:   pgtype.Date{Valid: false},
		ContractYears:     1,
	})
	require.NoError(t, err)
	_, err = store.CreateUserRole(ctx, db.CreateUserRoleParams{
		UserID:          operatorUser.ID,
		Role:            api.RoleOperator,
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.NoError(t, err)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 3) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
		require.Equal(t, "instant", claimResp.Status)
	}

	// 4) 运营商核销追偿单
	{
		url := fmt.Sprintf("/v1/operator/claims/%d/recovery/waive", claimResp.ClaimID)
		rec := doJSON(t, server, http.MethodPost, url, nil, operatorUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 5) 运营商查看追偿单状态
	var recoveryResp claimRecoveryStatusResponse
	{
		url := fmt.Sprintf("/v1/operator/claims/%d/recovery", claimResp.ClaimID)
		rec := doGET(t, server, url, operatorUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &recoveryResp)
		require.Equal(t, "waived", recoveryResp.Status)
	}
}

// TestClaimJourneyD25MerchantAppealDetailNotFoundIntegration
// 索赔旅程（D25）端到端验收：非所属商户查看申诉详情返回 404。
func TestClaimJourneyD25MerchantAppealDetailNotFoundIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	otherOwner := createIntegrationUser(t, store)
	otherMerchant, err := store.CreateMerchant(ctx, db.CreateMerchantParams{
		OwnerUserID:     otherOwner.ID,
		Name:            "集成测试餐厅-其他",
		Description:     pgtype.Text{String: "integration", Valid: true},
		LogoUrl:         pgtype.Text{String: "https://example.com/logo.png", Valid: true},
		Phone:           fmt.Sprintf("139%08d", util.RandomInt(0, 99999999)),
		Address:         "测试地址-" + util.RandomString(6),
		Latitude:        pgtype.Numeric{Valid: false},
		Longitude:       pgtype.Numeric{Valid: false},
		Status:          "approved",
		ApplicationData: []byte("{}"),
		RegionID:        region.ID,
	})
	require.NoError(t, err)
	_, err = store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: otherMerchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: otherMerchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	otherMerchant = ensureIntegrationMerchantCoords(t, store, otherMerchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 3) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
		require.Equal(t, "instant", claimResp.Status)
	}

	// 4) 商户申诉
	var appealResp appealSubmitResponse
	{
		appealBody := map[string]any{
			"claim_id": claimResp.ClaimID,
			"reason":   "evidence shows no foreign object in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/merchant/appeals", appealBody, merchantOwner.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &appealResp)
		require.NotZero(t, appealResp.ID)
	}

	// 5) 非所属商户查看申诉详情
	url := fmt.Sprintf("/v1/merchant/appeals/%d", appealResp.ID)
	rec := doGET(t, server, url, otherOwner.ID)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

// TestClaimJourneyD26RiderAppealDetailNotFoundIntegration
// 索赔旅程（D26）端到端验收：非所属骑手查看申诉详情返回 404。
func TestClaimJourneyD26RiderAppealDetailNotFoundIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err = store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	otherRiderUser := createIntegrationUser(t, store)
	otherRider, err := store.CreateRider(ctx, db.CreateRiderParams{
		UserID:   otherRiderUser.ID,
		RealName: "测试骑手-其他",
		IDCardNo: fmt.Sprintf("11010119900101%04d", util.RandomInt(0, 9999)),
		Phone:    fmt.Sprintf("139%08d", util.RandomInt(0, 99999999)),
		RegionID: pgtype.Int8{Int64: region.ID, Valid: true},
	})
	require.NoError(t, err)
	_, err = store.CreateRiderProfile(ctx, otherRider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: otherRider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: otherRider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: otherRider.ID, IsOnline: true})
	require.NoError(t, err)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 创建支付单并模拟支付成功
	var payment takeoutPaymentOrderResponse
	{
		payBody := map[string]any{
			"order_id":      orderID,
			"payment_type":  "native",
			"business_type": "order",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/payments", payBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payment)
	}

	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: "integration_tx_d26_001", Valid: true},
	})
	require.NoError(t, err)

	payRes, err := store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     payment.ID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	})
	require.NoError(t, err)
	require.True(t, payRes.Processed)

	// 3) 商户接单/出餐
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/accept", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/ready", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 4) 骑手抢单，建立配送关联
	{
		url := fmt.Sprintf("/v1/delivery/grab/%d", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 5) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "ready",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 6) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
		require.Equal(t, "instant", claimResp.Status)
	}

	// 7) 骑手申诉
	var appealResp appealSubmitResponse
	{
		appealBody := map[string]any{
			"claim_id": claimResp.ClaimID,
			"reason":   "delivery handled properly with no issues",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/rider/appeals", appealBody, riderUser.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &appealResp)
		require.NotZero(t, appealResp.ID)
	}

	// 8) 非所属骑手查看申诉详情
	url := fmt.Sprintf("/v1/rider/appeals/%d", appealResp.ID)
	rec := doGET(t, server, url, otherRiderUser.ID)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

// TestClaimJourneyD27OperatorAppealDetailNotFoundIntegration
// 索赔旅程（D27）端到端验收：跨区域运营商查看申诉详情返回 404。
func TestClaimJourneyD27OperatorAppealDetailNotFoundIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	otherRegion := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	operatorUser := createIntegrationUser(t, store)
	commissionRate := pgtype.Numeric{}
	_ = commissionRate.Scan("0.03")
	operator, err := store.CreateOperator(ctx, db.CreateOperatorParams{
		UserID:            operatorUser.ID,
		RegionID:          otherRegion.ID,
		Name:              "op_" + util.RandomString(6),
		ContactName:       "contact_" + util.RandomString(6),
		ContactPhone:      "138" + util.RandomString(8),
		WechatMchID:       pgtype.Text{Valid: false},
		CommissionRate:    commissionRate,
		Status:            "active",
		ContractStartDate: pgtype.Date{Valid: false},
		ContractEndDate:   pgtype.Date{Valid: false},
		ContractYears:     1,
	})
	require.NoError(t, err)
	_, err = store.CreateUserRole(ctx, db.CreateUserRoleParams{
		UserID:          operatorUser.ID,
		Role:            api.RoleOperator,
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.NoError(t, err)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 3) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
		require.Equal(t, "instant", claimResp.Status)
	}

	// 4) 商户申诉
	var appealResp appealSubmitResponse
	{
		appealBody := map[string]any{
			"claim_id": claimResp.ClaimID,
			"reason":   "evidence shows no foreign object in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/merchant/appeals", appealBody, merchantOwner.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &appealResp)
		require.NotZero(t, appealResp.ID)
	}

	// 5) 跨区域运营商查看申诉详情
	url := fmt.Sprintf("/v1/operator/appeals/%d", appealResp.ID)
	rec := doGET(t, server, url, operatorUser.ID)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

// TestClaimJourneyD28OperatorReviewCrossRegionForbiddenIntegration
// 索赔旅程（D28）端到端验收：跨区域运营商审核申诉返回 403。
func TestClaimJourneyD28OperatorReviewCrossRegionForbiddenIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	otherRegion := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	operatorUser := createIntegrationUser(t, store)
	commissionRate := pgtype.Numeric{}
	_ = commissionRate.Scan("0.03")
	operator, err := store.CreateOperator(ctx, db.CreateOperatorParams{
		UserID:            operatorUser.ID,
		RegionID:          otherRegion.ID,
		Name:              "op_" + util.RandomString(6),
		ContactName:       "contact_" + util.RandomString(6),
		ContactPhone:      "138" + util.RandomString(8),
		WechatMchID:       pgtype.Text{Valid: false},
		CommissionRate:    commissionRate,
		Status:            "active",
		ContractStartDate: pgtype.Date{Valid: false},
		ContractEndDate:   pgtype.Date{Valid: false},
		ContractYears:     1,
	})
	require.NoError(t, err)
	_, err = store.CreateUserRole(ctx, db.CreateUserRoleParams{
		UserID:          operatorUser.ID,
		Role:            api.RoleOperator,
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.NoError(t, err)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 3) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
		require.Equal(t, "instant", claimResp.Status)
	}

	// 4) 商户申诉
	var appealResp appealSubmitResponse
	{
		appealBody := map[string]any{
			"claim_id": claimResp.ClaimID,
			"reason":   "evidence shows no foreign object in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/merchant/appeals", appealBody, merchantOwner.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &appealResp)
		require.NotZero(t, appealResp.ID)
	}

	// 5) 跨区域运营商审核申诉
	reviewBody := map[string]any{
		"status":              "approved",
		"review_notes":        "appeal verified by operator",
		"compensation_amount": int64(500),
	}
	url := fmt.Sprintf("/v1/operator/appeals/%d/review", appealResp.ID)
	rec := doJSON(t, server, http.MethodPost, url, reviewBody, operatorUser.ID)
	require.Equal(t, http.StatusForbidden, rec.Code)
}

// TestClaimJourneyD29OperatorReviewAlreadyReviewedIntegration
// 索赔旅程（D29）端到端验收：申诉已审核再次提交返回 400。
func TestClaimJourneyD29OperatorReviewAlreadyReviewedIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	operatorUser := createIntegrationUser(t, store)
	commissionRate := pgtype.Numeric{}
	_ = commissionRate.Scan("0.03")
	operator, err := store.CreateOperator(ctx, db.CreateOperatorParams{
		UserID:            operatorUser.ID,
		RegionID:          region.ID,
		Name:              "op_" + util.RandomString(6),
		ContactName:       "contact_" + util.RandomString(6),
		ContactPhone:      "138" + util.RandomString(8),
		WechatMchID:       pgtype.Text{Valid: false},
		CommissionRate:    commissionRate,
		Status:            "active",
		ContractStartDate: pgtype.Date{Valid: false},
		ContractEndDate:   pgtype.Date{Valid: false},
		ContractYears:     1,
	})
	require.NoError(t, err)
	_, err = store.CreateUserRole(ctx, db.CreateUserRoleParams{
		UserID:          operatorUser.ID,
		Role:            api.RoleOperator,
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.NoError(t, err)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 3) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
		require.Equal(t, "instant", claimResp.Status)
	}

	// 4) 商户申诉
	var appealResp appealSubmitResponse
	{
		appealBody := map[string]any{
			"claim_id": claimResp.ClaimID,
			"reason":   "evidence shows no foreign object in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/merchant/appeals", appealBody, merchantOwner.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &appealResp)
		require.NotZero(t, appealResp.ID)
	}

	// 5) 运营商首次审核通过
	reviewBody := map[string]any{
		"status":              "approved",
		"review_notes":        "appeal verified by operator",
		"compensation_amount": int64(500),
	}
	url := fmt.Sprintf("/v1/operator/appeals/%d/review", appealResp.ID)
	{
		rec := doJSON(t, server, http.MethodPost, url, reviewBody, operatorUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 6) 再次审核应返回 400
	{
		rec := doJSON(t, server, http.MethodPost, url, reviewBody, operatorUser.ID)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	}
}

// TestClaimJourneyD30MerchantAppealDuplicateIntegration
// 索赔旅程（D30）端到端验收：商户对同一索赔重复申诉返回 409。
func TestClaimJourneyD30MerchantAppealDuplicateIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 3) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
		require.Equal(t, "instant", claimResp.Status)
	}

	// 4) 商户首次申诉
	appealBody := map[string]any{
		"claim_id": claimResp.ClaimID,
		"reason":   "evidence shows no foreign object in dish",
	}
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/merchant/appeals", appealBody, merchantOwner.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
	}

	// 5) 商户重复申诉
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/merchant/appeals", appealBody, merchantOwner.ID)
		require.Equal(t, http.StatusConflict, rec.Code)
	}
}

// TestReservationJourneyCConfirmBeforePaidIntegration
// 包间预订异常链路（C-Confirm-Pending）：未支付预订确认被拒。
func TestReservationJourneyCConfirmBeforePaidIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)

	room := createIntegrationRoomTable(t, store, merchant.ID)
	customer := createIntegrationUser(t, store)

	reservationDate := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	reservationTime := "19:00"

	// 1) 创建预订（未支付）
	var created reservationStatusResponse
	{
		body := map[string]any{
			"table_id":      room.ID,
			"date":          reservationDate,
			"time":          reservationTime,
			"guest_count":   2,
			"contact_name":  "张三",
			"contact_phone": "13800138001",
			"payment_mode":  "deposit",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/reservations", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	// 2) 商户确认预订
	{
		url := fmt.Sprintf("/v1/reservations/%d/confirm", created.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusConflict, rec.Code)
	}
}

// TestClaimJourneyD31RiderAppealDuplicateIntegration
// 索赔旅程（D31）端到端验收：骑手重复申诉返回已存在申诉。
func TestClaimJourneyD31RiderAppealDuplicateIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err = store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 创建支付单并模拟支付成功
	var payment takeoutPaymentOrderResponse
	{
		payBody := map[string]any{
			"order_id":      orderID,
			"payment_type":  "native",
			"business_type": "order",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/payments", payBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payment)
	}

	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: "integration_tx_d31_001", Valid: true},
	})
	require.NoError(t, err)

	payRes, err := store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     payment.ID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	})
	require.NoError(t, err)
	require.True(t, payRes.Processed)

	// 3) 商户接单/出餐
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/accept", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/ready", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 4) 骑手抢单，建立配送关联
	{
		url := fmt.Sprintf("/v1/delivery/grab/%d", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 5) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "ready",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 6) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
		require.Equal(t, "instant", claimResp.Status)
	}

	// 7) 骑手首次申诉
	appealBody := map[string]any{
		"claim_id": claimResp.ClaimID,
		"reason":   "delivery handled properly with no issues",
	}
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/rider/appeals", appealBody, riderUser.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
	}

	// 8) 骑手重复申诉
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/rider/appeals", appealBody, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
}

// TestClaimJourneyD32RiderAppealDuplicateConflictIntegration
// 索赔旅程（D32）端到端验收：不同骑手对同一索赔申诉返回 403。
func TestClaimJourneyD32RiderAppealDuplicateConflictIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err = store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	otherRiderUser := createIntegrationUser(t, store)
	otherRider, err := store.CreateRider(ctx, db.CreateRiderParams{
		UserID:   otherRiderUser.ID,
		RealName: "测试骑手-其他",
		IDCardNo: fmt.Sprintf("11010119900101%04d", util.RandomInt(0, 9999)),
		Phone:    fmt.Sprintf("139%08d", util.RandomInt(0, 99999999)),
		RegionID: pgtype.Int8{Int64: region.ID, Valid: true},
	})
	require.NoError(t, err)
	_, err = store.CreateRiderProfile(ctx, otherRider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: otherRider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: otherRider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: otherRider.ID, IsOnline: true})
	require.NoError(t, err)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 创建支付单并模拟支付成功
	var payment takeoutPaymentOrderResponse
	{
		payBody := map[string]any{
			"order_id":      orderID,
			"payment_type":  "native",
			"business_type": "order",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/payments", payBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payment)
	}

	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: "integration_tx_d32_001", Valid: true},
	})
	require.NoError(t, err)

	payRes, err := store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     payment.ID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	})
	require.NoError(t, err)
	require.True(t, payRes.Processed)

	// 3) 商户接单/出餐
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/accept", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/ready", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 4) 骑手抢单，建立配送关联
	{
		url := fmt.Sprintf("/v1/delivery/grab/%d", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 5) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "ready",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 6) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
		require.Equal(t, "instant", claimResp.Status)
	}

	// 7) 骑手首次申诉
	appealBody := map[string]any{
		"claim_id": claimResp.ClaimID,
		"reason":   "delivery handled properly with no issues",
	}
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/rider/appeals", appealBody, riderUser.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
	}

	// 8) 其他骑手重复申诉
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/rider/appeals", appealBody, otherRiderUser.ID)
		require.Equal(t, http.StatusForbidden, rec.Code)
	}
}

// TestClaimJourneyD33OperatorAppealListByStatusIntegration
// 索赔旅程（D33）端到端验收：运营商按状态筛选申诉列表。
func TestClaimJourneyD33OperatorAppealListByStatusIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	operatorUser := createIntegrationUser(t, store)
	commissionRate := pgtype.Numeric{}
	_ = commissionRate.Scan("0.03")
	operator, err := store.CreateOperator(ctx, db.CreateOperatorParams{
		UserID:            operatorUser.ID,
		RegionID:          region.ID,
		Name:              "op_" + util.RandomString(6),
		ContactName:       "contact_" + util.RandomString(6),
		ContactPhone:      "138" + util.RandomString(8),
		WechatMchID:       pgtype.Text{Valid: false},
		CommissionRate:    commissionRate,
		Status:            "active",
		ContractStartDate: pgtype.Date{Valid: false},
		ContractEndDate:   pgtype.Date{Valid: false},
		ContractYears:     1,
	})
	require.NoError(t, err)
	_, err = store.CreateUserRole(ctx, db.CreateUserRoleParams{
		UserID:          operatorUser.ID,
		Role:            api.RoleOperator,
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.NoError(t, err)

	// 1) 创建外卖订单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	orderID := created.ID

	// 2) 标记订单完成
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      orderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", order.Status)

	// 3) 提交索赔
	var claimResp claimSubmitResponse
	{
		body := map[string]any{
			"order_id":     orderID,
			"claim_type":   "foreign-object",
			"claim_amount": order.TotalAmount,
			"claim_reason": "foreign object found in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
		require.Equal(t, "instant", claimResp.Status)
	}

	// 4) 商户申诉
	var appealResp appealSubmitResponse
	{
		appealBody := map[string]any{
			"claim_id": claimResp.ClaimID,
			"reason":   "evidence shows no foreign object in dish",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/merchant/appeals", appealBody, merchantOwner.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &appealResp)
		require.NotZero(t, appealResp.ID)
	}

	// 5) 运营商审核为 approved
	{
		reviewBody := map[string]any{
			"status":              "approved",
			"review_notes":        "appeal verified by operator",
			"compensation_amount": int64(500),
		}
		url := fmt.Sprintf("/v1/operator/appeals/%d/review", appealResp.ID)
		rec := doJSON(t, server, http.MethodPost, url, reviewBody, operatorUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 6) 再提交一笔待审核申诉
	var pendingAppeal appealSubmitResponse
	{
		// 新建订单用于 pending 申诉
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}

	secondOrderID := created.ID
	_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
		OrderID:      secondOrderID,
		OldStatus:    "pending",
		OperatorID:   customer.ID,
		OperatorType: "user",
	})
	require.NoError(t, err)

	secondOrder, err := store.GetOrder(ctx, secondOrderID)
	require.NoError(t, err)
	require.Equal(t, "completed", secondOrder.Status)

	{
		claimBody := map[string]any{
			"order_id":     secondOrderID,
			"claim_type":   "foreign-object",
			"claim_amount": secondOrder.TotalAmount,
			"claim_reason": "another foreign object report",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/claims", claimBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
		require.NotZero(t, claimResp.ClaimID)
	}
	{
		appealBody := map[string]any{
			"claim_id": claimResp.ClaimID,
			"reason":   "pending appeal for filter",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/merchant/appeals", appealBody, merchantOwner.ID)
		require.Equal(t, http.StatusCreated, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &pendingAppeal)
	}

	// 7) 按 status=approved 查询
	{
		url := "/v1/operator/appeals?status=approved&page=1&limit=10"
		rec := doGET(t, server, url, operatorUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		var resp struct {
			Appeals []struct {
				ID     int64  `json:"id"`
				Status string `json:"status"`
			} `json:"appeals"`
			Total int64 `json:"total"`
		}
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.GreaterOrEqual(t, resp.Total, int64(1))
		found := false
		for _, a := range resp.Appeals {
			if a.ID == appealResp.ID {
				found = true
				require.Equal(t, "approved", a.Status)
			}
		}
		require.True(t, found)
	}

	// 8) 按 status=pending 查询
	{
		url := "/v1/operator/appeals?status=pending&page=1&limit=10"
		rec := doGET(t, server, url, operatorUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		var resp struct {
			Appeals []struct {
				ID     int64  `json:"id"`
				Status string `json:"status"`
			} `json:"appeals"`
			Total int64 `json:"total"`
		}
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
		require.GreaterOrEqual(t, resp.Total, int64(1))
		found := false
		for _, a := range resp.Appeals {
			if a.ID == pendingAppeal.ID {
				found = true
				require.Equal(t, "pending", a.Status)
			}
		}
		require.True(t, found)
	}
}

// TestClaimJourneyD34OperatorAppealListEmptyIntegration
// 索赔旅程（D34）端到端验收：运营商按状态筛选返回空列表。
func TestClaimJourneyD34OperatorAppealListEmptyIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	operatorUser := createIntegrationUser(t, store)
	commissionRate := pgtype.Numeric{}
	_ = commissionRate.Scan("0.03")
	operator, err := store.CreateOperator(ctx, db.CreateOperatorParams{
		UserID:            operatorUser.ID,
		RegionID:          region.ID,
		Name:              "op_" + util.RandomString(6),
		ContactName:       "contact_" + util.RandomString(6),
		ContactPhone:      "138" + util.RandomString(8),
		WechatMchID:       pgtype.Text{Valid: false},
		CommissionRate:    commissionRate,
		Status:            "active",
		ContractStartDate: pgtype.Date{Valid: false},
		ContractEndDate:   pgtype.Date{Valid: false},
		ContractYears:     1,
	})
	require.NoError(t, err)
	_, err = store.CreateUserRole(ctx, db.CreateUserRoleParams{
		UserID:          operatorUser.ID,
		Role:            api.RoleOperator,
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.NoError(t, err)

	url := "/v1/operator/appeals?status=approved&page=1&limit=10"
	rec := doGET(t, server, url, operatorUser.ID)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Appeals []struct {
			ID int64 `json:"id"`
		} `json:"appeals"`
		Total int64 `json:"total"`
	}
	requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
	require.Equal(t, int64(0), resp.Total)
	require.Len(t, resp.Appeals, 0)
}

// TestClaimJourneyD35MerchantAppealListEmptyIntegration
// 索赔旅程（D35）端到端验收：商户申诉列表空结果。
func TestClaimJourneyD35MerchantAppealListEmptyIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	url := "/v1/merchant/appeals?page_id=1&page_size=10"
	rec := doGET(t, server, url, merchantOwner.ID)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Appeals []struct {
			ID int64 `json:"id"`
		} `json:"appeals"`
		Total int64 `json:"total"`
	}
	requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
	require.Equal(t, int64(0), resp.Total)
	require.Len(t, resp.Appeals, 0)
}

// TestClaimJourneyD36RiderAppealListEmptyIntegration
// 索赔旅程（D36）端到端验收：骑手申诉列表空结果。
func TestClaimJourneyD36RiderAppealListEmptyIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err := store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	url := "/v1/rider/appeals?page_id=1&page_size=10"
	rec := doGET(t, server, url, riderUser.ID)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Appeals []struct {
			ID int64 `json:"id"`
		} `json:"appeals"`
		Total int64 `json:"total"`
	}
	requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
	require.Equal(t, int64(0), resp.Total)
	require.Len(t, resp.Appeals, 0)
}

// TestClaimJourneyD37OperatorAppealListPaginationIntegration
// 索赔旅程（D37）端到端验收：运营商申诉列表分页返回稳定。
func TestClaimJourneyD37OperatorAppealListPaginationIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	operatorUser := createIntegrationUser(t, store)
	commissionRate := pgtype.Numeric{}
	_ = commissionRate.Scan("0.03")
	operator, err := store.CreateOperator(ctx, db.CreateOperatorParams{
		UserID:            operatorUser.ID,
		RegionID:          region.ID,
		Name:              "op_" + util.RandomString(6),
		ContactName:       "contact_" + util.RandomString(6),
		ContactPhone:      "138" + util.RandomString(8),
		WechatMchID:       pgtype.Text{Valid: false},
		CommissionRate:    commissionRate,
		Status:            "active",
		ContractStartDate: pgtype.Date{Valid: false},
		ContractEndDate:   pgtype.Date{Valid: false},
		ContractYears:     1,
	})
	require.NoError(t, err)
	_, err = store.CreateUserRole(ctx, db.CreateUserRoleParams{
		UserID:          operatorUser.ID,
		Role:            api.RoleOperator,
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.NoError(t, err)

	createAppeal := func(orderID int64, reason string) appealSubmitResponse {
		var claimResp claimSubmitResponse
		{
			body := map[string]any{
				"order_id":     orderID,
				"claim_type":   "foreign-object",
				"claim_amount": int64(1000),
				"claim_reason": "foreign object found in dish",
			}
			rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
			require.Equal(t, http.StatusOK, rec.Code)
			requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
			require.NotZero(t, claimResp.ClaimID)
		}

		var appealResp appealSubmitResponse
		{
			appealBody := map[string]any{
				"claim_id": claimResp.ClaimID,
				"reason":   reason,
			}
			rec := doJSON(t, server, http.MethodPost, "/v1/merchant/appeals", appealBody, merchantOwner.ID)
			require.Equal(t, http.StatusCreated, rec.Code)
			requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &appealResp)
			require.NotZero(t, appealResp.ID)
		}
		return appealResp
	}

	createOrder := func() int64 {
		createBody := map[string]any{
			"merchant_id":           merchant.ID,
			"order_type":            "takeout",
			"address_id":            addr.ID,
			"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
			"use_balance":           false,
			"delivery_fee":          int64(500),
			"delivery_distance":     int32(1000),
			"delivery_fee_discount": int64(0),
		}

		var created takeoutOrderResponse
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)

		_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
			OrderID:      created.ID,
			OldStatus:    "pending",
			OperatorID:   customer.ID,
			OperatorType: "user",
		})
		require.NoError(t, err)
		return created.ID
	}

	orderID1 := createOrder()
	appeal1 := createAppeal(orderID1, "appeal one")

	orderID2 := createOrder()
	appeal2 := createAppeal(orderID2, "appeal two")

	page1URL := "/v1/operator/appeals?page=1&limit=1"
	page2URL := "/v1/operator/appeals?page=2&limit=1"

	var page1 struct {
		Appeals []struct {
			ID int64 `json:"id"`
		} `json:"appeals"`
		Total int64 `json:"total"`
	}
	{
		rec := doGET(t, server, page1URL, operatorUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &page1)
		require.Equal(t, int64(2), page1.Total)
		require.Len(t, page1.Appeals, 1)
	}

	var page2 struct {
		Appeals []struct {
			ID int64 `json:"id"`
		} `json:"appeals"`
		Total int64 `json:"total"`
	}
	{
		rec := doGET(t, server, page2URL, operatorUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &page2)
		require.Equal(t, int64(2), page2.Total)
		require.Len(t, page2.Appeals, 1)
	}

	ids := map[int64]bool{
		page1.Appeals[0].ID: true,
		page2.Appeals[0].ID: true,
	}
	require.True(t, ids[appeal1.ID])
	require.True(t, ids[appeal2.ID])
}

// TestClaimJourneyD38MerchantAppealListPaginationIntegration
// 索赔旅程（D38）端到端验收：商户申诉列表分页返回稳定。
func TestClaimJourneyD38MerchantAppealListPaginationIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	createOrder := func() int64 {
		createBody := map[string]any{
			"merchant_id":           merchant.ID,
			"order_type":            "takeout",
			"address_id":            addr.ID,
			"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
			"use_balance":           false,
			"delivery_fee":          int64(500),
			"delivery_distance":     int32(1000),
			"delivery_fee_discount": int64(0),
		}

		var created takeoutOrderResponse
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)

		_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
			OrderID:      created.ID,
			OldStatus:    "pending",
			OperatorID:   customer.ID,
			OperatorType: "user",
		})
		require.NoError(t, err)
		return created.ID
	}

	createAppeal := func(orderID int64, reason string) appealSubmitResponse {
		var claimResp claimSubmitResponse
		{
			body := map[string]any{
				"order_id":     orderID,
				"claim_type":   "foreign-object",
				"claim_amount": int64(1000),
				"claim_reason": "foreign object found in dish",
			}
			rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
			require.Equal(t, http.StatusOK, rec.Code)
			requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
			require.NotZero(t, claimResp.ClaimID)
		}

		var appealResp appealSubmitResponse
		{
			appealBody := map[string]any{
				"claim_id": claimResp.ClaimID,
				"reason":   reason,
			}
			rec := doJSON(t, server, http.MethodPost, "/v1/merchant/appeals", appealBody, merchantOwner.ID)
			require.Equal(t, http.StatusCreated, rec.Code)
			requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &appealResp)
			require.NotZero(t, appealResp.ID)
		}
		return appealResp
	}

	orderID1 := createOrder()
	appeal1 := createAppeal(orderID1, "appeal one")

	orderID2 := createOrder()
	appeal2 := createAppeal(orderID2, "appeal two")

	page1URL := "/v1/merchant/appeals?page_id=1&page_size=1"
	page2URL := "/v1/merchant/appeals?page_id=2&page_size=1"

	var page1 struct {
		Appeals []struct {
			ID int64 `json:"id"`
		} `json:"appeals"`
		Total int64 `json:"total"`
	}
	{
		rec := doGET(t, server, page1URL, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &page1)
		require.Equal(t, int64(2), page1.Total)
		require.Len(t, page1.Appeals, 1)
	}

	var page2 struct {
		Appeals []struct {
			ID int64 `json:"id"`
		} `json:"appeals"`
		Total int64 `json:"total"`
	}
	{
		rec := doGET(t, server, page2URL, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &page2)
		require.Equal(t, int64(2), page2.Total)
		require.Len(t, page2.Appeals, 1)
	}

	ids := map[int64]bool{
		page1.Appeals[0].ID: true,
		page2.Appeals[0].ID: true,
	}
	require.True(t, ids[appeal1.ID])
	require.True(t, ids[appeal2.ID])
}

// TestClaimJourneyD39RiderAppealListPaginationIntegration
// 索赔旅程（D39）端到端验收：骑手申诉列表分页返回稳定。
func TestClaimJourneyD39RiderAppealListPaginationIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err = store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	createOrder := func() int64 {
		createBody := map[string]any{
			"merchant_id":           merchant.ID,
			"order_type":            "takeout",
			"address_id":            addr.ID,
			"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
			"use_balance":           false,
			"delivery_fee":          int64(500),
			"delivery_distance":     int32(1000),
			"delivery_fee_discount": int64(0),
		}

		var created takeoutOrderResponse
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)

		var payment takeoutPaymentOrderResponse
		{
			payBody := map[string]any{
				"order_id":      created.ID,
				"payment_type":  "native",
				"business_type": "order",
			}
			rec := doJSON(t, server, http.MethodPost, "/v1/payments", payBody, customer.ID)
			require.Equal(t, http.StatusOK, rec.Code)
			requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payment)
		}

		_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
			ID:            payment.ID,
			TransactionID: pgtype.Text{String: util.RandomString(12), Valid: true},
		})
		require.NoError(t, err)

		payRes, err := store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
			PaymentOrderID:     payment.ID,
			RiderAverageSpeed:  15000,
			DefaultPrepareTime: 20,
		})
		require.NoError(t, err)
		require.True(t, payRes.Processed)

		{
			url := fmt.Sprintf("/v1/merchant/orders/%d/accept", created.ID)
			rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
			require.Equal(t, http.StatusOK, rec.Code)
		}
		{
			url := fmt.Sprintf("/v1/merchant/orders/%d/ready", created.ID)
			rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
			require.Equal(t, http.StatusOK, rec.Code)
		}
		{
			url := fmt.Sprintf("/v1/delivery/grab/%d", created.ID)
			rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
			require.Equal(t, http.StatusOK, rec.Code)
		}

		_, err = store.CompleteOrderTx(ctx, db.CompleteOrderTxParams{
			OrderID:      created.ID,
			OldStatus:    "ready",
			OperatorID:   customer.ID,
			OperatorType: "user",
		})
		require.NoError(t, err)
		return created.ID
	}

	createAppeal := func(orderID int64, reason string) appealSubmitResponse {
		var claimResp claimSubmitResponse
		{
			body := map[string]any{
				"order_id":     orderID,
				"claim_type":   "foreign-object",
				"claim_amount": int64(1000),
				"claim_reason": "foreign object found in dish",
			}
			rec := doJSON(t, server, http.MethodPost, "/v1/claims", body, customer.ID)
			require.Equal(t, http.StatusOK, rec.Code)
			requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &claimResp)
			require.NotZero(t, claimResp.ClaimID)
		}

		var appealResp appealSubmitResponse
		{
			appealBody := map[string]any{
				"claim_id": claimResp.ClaimID,
				"reason":   reason,
			}
			rec := doJSON(t, server, http.MethodPost, "/v1/rider/appeals", appealBody, riderUser.ID)
			require.Equal(t, http.StatusCreated, rec.Code)
			requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &appealResp)
			require.NotZero(t, appealResp.ID)
		}
		return appealResp
	}

	orderID1 := createOrder()
	appeal1 := createAppeal(orderID1, "appeal one")

	orderID2 := createOrder()
	appeal2 := createAppeal(orderID2, "appeal two")

	page1URL := "/v1/rider/appeals?page_id=1&page_size=1"
	page2URL := "/v1/rider/appeals?page_id=2&page_size=1"

	var page1 struct {
		Appeals []struct {
			ID int64 `json:"id"`
		} `json:"appeals"`
		Total int64 `json:"total"`
	}
	{
		rec := doGET(t, server, page1URL, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &page1)
		require.Equal(t, int64(2), page1.Total)
		require.Len(t, page1.Appeals, 1)
	}

	var page2 struct {
		Appeals []struct {
			ID int64 `json:"id"`
		} `json:"appeals"`
		Total int64 `json:"total"`
	}
	{
		rec := doGET(t, server, page2URL, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &page2)
		require.Equal(t, int64(2), page2.Total)
		require.Len(t, page2.Appeals, 1)
	}

	ids := map[int64]bool{
		page1.Appeals[0].ID: true,
		page2.Appeals[0].ID: true,
	}
	require.True(t, ids[appeal1.ID])
	require.True(t, ids[appeal2.ID])
}

// TestClaimJourneyD40OperatorAppealListInvalidStatusIntegration
// 索赔旅程（D40）端到端验收：运营商申诉列表非法状态返回 400。
func TestClaimJourneyD40OperatorAppealListInvalidStatusIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	operatorUser := createIntegrationUser(t, store)
	commissionRate := pgtype.Numeric{}
	_ = commissionRate.Scan("0.03")
	operator, err := store.CreateOperator(ctx, db.CreateOperatorParams{
		UserID:            operatorUser.ID,
		RegionID:          region.ID,
		Name:              "op_" + util.RandomString(6),
		ContactName:       "contact_" + util.RandomString(6),
		ContactPhone:      "138" + util.RandomString(8),
		WechatMchID:       pgtype.Text{Valid: false},
		CommissionRate:    commissionRate,
		Status:            "active",
		ContractStartDate: pgtype.Date{Valid: false},
		ContractEndDate:   pgtype.Date{Valid: false},
		ContractYears:     1,
	})
	require.NoError(t, err)
	_, err = store.CreateUserRole(ctx, db.CreateUserRoleParams{
		UserID:          operatorUser.ID,
		Role:            api.RoleOperator,
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.NoError(t, err)

	url := "/v1/operator/appeals?status=invalid&page=1&limit=10"
	rec := doGET(t, server, url, operatorUser.ID)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestClaimJourneyD41MerchantAppealListInvalidPaginationIntegration
// 索赔旅程（D41）端到端验收：商户申诉列表非法分页返回 400。
func TestClaimJourneyD41MerchantAppealListInvalidPaginationIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	url := "/v1/merchant/appeals?page_id=0&page_size=10"
	rec := doGET(t, server, url, merchantOwner.ID)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestClaimJourneyD42RiderAppealListInvalidPaginationIntegration
// 索赔旅程（D42）端到端验收：骑手申诉列表非法分页返回 400。
func TestClaimJourneyD42RiderAppealListInvalidPaginationIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err := store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	url := "/v1/rider/appeals?page_id=0&page_size=10"
	rec := doGET(t, server, url, riderUser.ID)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestClaimJourneyD43OperatorAppealListDefaultPaginationIntegration
// 索赔旅程（D43）端到端验收：运营商申诉列表默认分页参数。
func TestClaimJourneyD43OperatorAppealListDefaultPaginationIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	operatorUser := createIntegrationUser(t, store)
	commissionRate := pgtype.Numeric{}
	_ = commissionRate.Scan("0.03")
	operator, err := store.CreateOperator(ctx, db.CreateOperatorParams{
		UserID:            operatorUser.ID,
		RegionID:          region.ID,
		Name:              "op_" + util.RandomString(6),
		ContactName:       "contact_" + util.RandomString(6),
		ContactPhone:      "138" + util.RandomString(8),
		WechatMchID:       pgtype.Text{Valid: false},
		CommissionRate:    commissionRate,
		Status:            "active",
		ContractStartDate: pgtype.Date{Valid: false},
		ContractEndDate:   pgtype.Date{Valid: false},
		ContractYears:     1,
	})
	require.NoError(t, err)
	_, err = store.CreateUserRole(ctx, db.CreateUserRoleParams{
		UserID:          operatorUser.ID,
		Role:            api.RoleOperator,
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.NoError(t, err)

	url := "/v1/operator/appeals"
	rec := doGET(t, server, url, operatorUser.ID)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Page  int32 `json:"page"`
		Limit int32 `json:"limit"`
	}
	requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &resp)
	require.Equal(t, int32(1), resp.Page)
	require.Equal(t, int32(10), resp.Limit)
}

// TestClaimJourneyD44MerchantAppealListInvalidPageSizeIntegration
// 索赔旅程（D44）端到端验收：商户申诉列表超限 page_size 返回 400。
func TestClaimJourneyD44MerchantAppealListInvalidPageSizeIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	url := "/v1/merchant/appeals?page_id=1&page_size=51"
	rec := doGET(t, server, url, merchantOwner.ID)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestClaimJourneyD45RiderAppealListInvalidPageSizeIntegration
// 索赔旅程（D45）端到端验收：骑手申诉列表超限 page_size 返回 400。
func TestClaimJourneyD45RiderAppealListInvalidPageSizeIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err := store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	url := "/v1/rider/appeals?page_id=1&page_size=51"
	rec := doGET(t, server, url, riderUser.ID)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestClaimJourneyD46MerchantAppealDetailInvalidIDIntegration
// 索赔旅程（D46）端到端验收：商户申诉详情非法 ID 返回 400。
func TestClaimJourneyD46MerchantAppealDetailInvalidIDIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	_ = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	url := "/v1/merchant/appeals/invalid"
	rec := doGET(t, server, url, merchantOwner.ID)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestClaimJourneyD47RiderAppealDetailInvalidIDIntegration
// 索赔旅程（D47）端到端验收：骑手申诉详情非法 ID 返回 400。
func TestClaimJourneyD47RiderAppealDetailInvalidIDIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err := store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	url := "/v1/rider/appeals/invalid"
	rec := doGET(t, server, url, riderUser.ID)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestClaimJourneyD48OperatorAppealDetailInvalidIDIntegration
// 索赔旅程（D48）端到端验收：运营商申诉详情非法 ID 返回 400。
func TestClaimJourneyD48OperatorAppealDetailInvalidIDIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	operatorUser := createIntegrationUser(t, store)
	commissionRate := pgtype.Numeric{}
	_ = commissionRate.Scan("0.03")
	operator, err := store.CreateOperator(ctx, db.CreateOperatorParams{
		UserID:            operatorUser.ID,
		RegionID:          region.ID,
		Name:              "op_" + util.RandomString(6),
		ContactName:       "contact_" + util.RandomString(6),
		ContactPhone:      "138" + util.RandomString(8),
		WechatMchID:       pgtype.Text{Valid: false},
		CommissionRate:    commissionRate,
		Status:            "active",
		ContractStartDate: pgtype.Date{Valid: false},
		ContractEndDate:   pgtype.Date{Valid: false},
		ContractYears:     1,
	})
	require.NoError(t, err)
	_, err = store.CreateUserRole(ctx, db.CreateUserRoleParams{
		UserID:          operatorUser.ID,
		Role:            api.RoleOperator,
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: operator.ID, Valid: true},
	})
	require.NoError(t, err)

	url := "/v1/operator/appeals/invalid"
	rec := doGET(t, server, url, operatorUser.ID)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestTakeoutJourneyB2Integration
// 外卖旅程（B2）端到端验收：骑手送达后 1 小时无索赔自动完成（兜底终点）。
//
// 说明：integration 环境不启动 cron，这里直接调用 scheduler.RunOnce() 触发一次扫描。
func TestTakeoutJourneyB2Integration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)

	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err = store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	// 1) C端下单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}
	orderID := created.ID

	// 2) 创建支付单并同步模拟支付成功后置
	var payment takeoutPaymentOrderResponse
	{
		payBody := map[string]any{
			"order_id":      orderID,
			"payment_type":  "native",
			"business_type": "order",
		}
		rec := doJSON(t, server, http.MethodPost, "/v1/payments", payBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &payment)
	}

	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            payment.ID,
		TransactionID: pgtype.Text{String: "integration_tx_b2_001", Valid: true},
	})
	require.NoError(t, err)

	payRes, err := store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     payment.ID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	})
	require.NoError(t, err)
	require.True(t, payRes.Processed)

	// 3) 商户接单/出餐
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/accept", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/ready", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 4) 骑手抢单并推进到送达（rider_delivered）
	{
		url := fmt.Sprintf("/v1/delivery/grab/%d", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	delivery, err := store.GetDeliveryByOrderID(ctx, orderID)
	require.NoError(t, err)

	{
		url := fmt.Sprintf("/v1/delivery/%d/start-pickup", delivery.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/delivery/%d/confirm-pickup", delivery.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/delivery/%d/start-delivery", delivery.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/delivery/%d/confirm-delivery", delivery.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	o, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "rider_delivered", o.Status)

	// 5) 回拨 rider_delivered_at 使其满足“送达超过 1 小时”
	backTo := time.Now().Add(-scheduler.TakeoutAutoCompleteAfter).Add(-2 * time.Minute)
	_, err = integrationPool.Exec(ctx, `UPDATE orders SET rider_delivered_at = $2 WHERE id = $1`, orderID, backTo)
	require.NoError(t, err)

	// 6) 触发 scheduler 扫描并断言自动完成
	s := scheduler.NewTakeoutAutoCompleteScheduler(store, nil)
	s.RunOnce()

	updated, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", updated.Status)
	require.True(t, updated.CompletedAt.Valid)

	// 幂等：重复扫描不应改变结果/报错
	s.RunOnce()
	updated2, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", updated2.Status)
}

type captureTaskDistributor struct {
	worker.NoopTaskDistributor

	mu    sync.Mutex
	calls []worker.ProfitSharingPayload
}

func (d *captureTaskDistributor) DistributeTaskProcessProfitSharing(ctx context.Context, payload *worker.ProfitSharingPayload, opts ...asynq.Option) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if payload != nil {
		d.calls = append(d.calls, *payload)
	}
	return nil
}

func (d *captureTaskDistributor) Calls() []worker.ProfitSharingPayload {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]worker.ProfitSharingPayload, len(d.calls))
	copy(out, d.calls)
	return out
}

type capturePaymentSuccessDistributor struct {
	worker.NoopTaskDistributor

	mu       sync.Mutex
	payloads []worker.PaymentSuccessPayload
}

func (d *capturePaymentSuccessDistributor) DistributeTaskProcessPaymentSuccess(ctx context.Context, payload *worker.PaymentSuccessPayload, opts ...asynq.Option) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if payload != nil {
		d.payloads = append(d.payloads, *payload)
	}
	return nil
}

func (d *capturePaymentSuccessDistributor) Payloads() []worker.PaymentSuccessPayload {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]worker.PaymentSuccessPayload, len(d.payloads))
	copy(out, d.payloads)
	return out
}

type captureRefundResultDistributor struct {
	worker.NoopTaskDistributor

	mu       sync.Mutex
	payloads []worker.RefundResultPayload
}

type captureAppealResultDistributor struct {
	worker.NoopTaskDistributor

	mu       sync.Mutex
	payloads []worker.ProcessAppealResultPayload
}

func (d *captureAppealResultDistributor) DistributeTaskProcessAppealResult(ctx context.Context, payload *worker.ProcessAppealResultPayload, opts ...asynq.Option) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if payload != nil {
		d.payloads = append(d.payloads, *payload)
	}
	return nil
}

func (d *captureAppealResultDistributor) Payloads() []worker.ProcessAppealResultPayload {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]worker.ProcessAppealResultPayload, len(d.payloads))
	copy(out, d.payloads)
	return out
}

type captureSendNotificationDistributor struct {
	worker.NoopTaskDistributor

	mu       sync.Mutex
	payloads []worker.SendNotificationPayload
}

func (d *captureSendNotificationDistributor) DistributeTaskSendNotification(ctx context.Context, payload *worker.SendNotificationPayload, opts ...asynq.Option) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if payload != nil {
		d.payloads = append(d.payloads, *payload)
	}
	return nil
}

func (d *captureSendNotificationDistributor) Payloads() []worker.SendNotificationPayload {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]worker.SendNotificationPayload, len(d.payloads))
	copy(out, d.payloads)
	return out
}

func (d *captureRefundResultDistributor) DistributeTaskProcessRefundResult(ctx context.Context, payload *worker.RefundResultPayload, opts ...asynq.Option) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if payload != nil {
		d.payloads = append(d.payloads, *payload)
	}
	return nil
}

func (d *captureRefundResultDistributor) Payloads() []worker.RefundResultPayload {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]worker.RefundResultPayload, len(d.payloads))
	copy(out, d.payloads)
	return out
}

// TestTakeoutJourneyB3Integration
// 外卖旅程（B3）端到端验收：完成后触发 profit_sharing 分账 + 恢复补偿兜底。
//
// integration harness 不配置队列/worker（taskDistributor=nil），这里按文档“兜底证据”口径验收：
// - 构造一条超龄 pending 的 profit_sharing_orders 记录
// - 触发 profit sharing recovery scheduler 扫描
// - 断言它会尝试入队 payment:process_profit_sharing（通过捕获 distributor 调用）
func TestTakeoutJourneyB3Integration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)

	merchantOwner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, merchantOwner.ID, region.ID)

	_, err := store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{ID: merchant.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateMerchantIsOpen(ctx, db.UpdateMerchantIsOpenParams{ID: merchant.ID, IsOpen: true, AutoCloseAt: pgtype.Timestamptz{Valid: false}})
	require.NoError(t, err)
	merchant = ensureIntegrationMerchantCoords(t, store, merchant.ID)

	dish := createIntegrationDish(t, store, merchant.ID)

	customer := createIntegrationUser(t, store)
	addr := createIntegrationUserAddress(t, store, customer.ID, region.ID)

	riderUser := createIntegrationUser(t, store)
	rider := createIntegrationRider(t, store, riderUser.ID, region.ID)
	_, err = store.CreateRiderProfile(ctx, rider.ID)
	require.NoError(t, err)
	_, err = store.UpdateRiderStatus(ctx, db.UpdateRiderStatusParams{ID: rider.ID, Status: "active"})
	require.NoError(t, err)
	_, err = store.UpdateRiderDeposit(ctx, db.UpdateRiderDepositParams{ID: rider.ID, DepositAmount: 100_000, FrozenDeposit: 0})
	require.NoError(t, err)
	_, err = store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: true})
	require.NoError(t, err)

	// 1) 下单
	createBody := map[string]any{
		"merchant_id":           merchant.ID,
		"order_type":            "takeout",
		"address_id":            addr.ID,
		"items":                 []map[string]any{{"dish_id": dish.ID, "quantity": 1}},
		"use_balance":           false,
		"delivery_fee":          int64(500),
		"delivery_distance":     int32(1000),
		"delivery_fee_discount": int64(0),
	}

	var created takeoutOrderResponse
	{
		rec := doJSON(t, server, http.MethodPost, "/v1/orders", createBody, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
		requireUnmarshalAPIResponseData(t, rec.Body.Bytes(), &created)
		require.Equal(t, "pending", created.Status)
	}
	orderID := created.ID

	order, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)

	// 2) 创建 profit_sharing 支付单（API 不支持该类型；这里直接落库以验收分账/恢复逻辑）
	po, err := store.CreatePaymentOrder(ctx, db.CreatePaymentOrderParams{
		OrderID:      pgtype.Int8{Int64: orderID, Valid: true},
		UserID:       customer.ID,
		PaymentType:  "profit_sharing",
		BusinessType: "order",
		Amount:       order.TotalAmount,
		OutTradeNo:   fmt.Sprintf("PS_%d_%d", orderID, util.RandomInt(1000, 9999)),
		ExpiresAt:    pgtype.Timestamptz{Time: time.Now().Add(30 * time.Minute), Valid: true},
	})
	require.NoError(t, err)

	_, err = store.UpdatePaymentOrderToPaid(ctx, db.UpdatePaymentOrderToPaidParams{
		ID:            po.ID,
		TransactionID: pgtype.Text{String: "integration_profit_sharing_tx_001", Valid: true},
	})
	require.NoError(t, err)

	payRes, err := store.ProcessPaymentSuccessTx(ctx, db.ProcessPaymentSuccessTxParams{
		PaymentOrderID:     po.ID,
		RiderAverageSpeed:  15000,
		DefaultPrepareTime: 20,
	})
	require.NoError(t, err)
	require.True(t, payRes.Processed)

	// 3) 商户接单/出餐
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/accept", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/merchant/orders/%d/ready", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, merchantOwner.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 4) 骑手配送到送达
	{
		url := fmt.Sprintf("/v1/delivery/grab/%d", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	delivery, err := store.GetDeliveryByOrderID(ctx, orderID)
	require.NoError(t, err)
	{
		url := fmt.Sprintf("/v1/delivery/%d/start-pickup", delivery.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/delivery/%d/confirm-pickup", delivery.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/delivery/%d/start-delivery", delivery.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	{
		url := fmt.Sprintf("/v1/delivery/%d/confirm-delivery", delivery.ID)
		rec := doJSON(t, server, http.MethodPost, url, nil, riderUser.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	// 5) 用户确认完成（此处在真实环境会尝试入队分账任务；integration 里 taskDistributor=nil）
	{
		url := fmt.Sprintf("/v1/orders/%d/confirm", orderID)
		rec := doJSON(t, server, http.MethodPost, url, nil, customer.ID)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	updatedOrder, err := store.GetOrder(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, "completed", updatedOrder.Status)

	// 6) 构造一条“超龄 pending”的分账记录，验证 recovery 会尝试入队
	ps, err := store.CreateProfitSharingOrder(ctx, db.CreateProfitSharingOrderParams{
		PaymentOrderID:      po.ID,
		MerchantID:          merchant.ID,
		OperatorID:          pgtype.Int8{Valid: false},
		OrderSource:         "takeout",
		TotalAmount:         order.TotalAmount,
		DeliveryFee:         order.DeliveryFee,
		RiderID:             pgtype.Int8{Int64: rider.ID, Valid: true},
		RiderAmount:         order.DeliveryFee,
		DistributableAmount: order.TotalAmount - order.DeliveryFee,
		PlatformRate:        0,
		OperatorRate:        0,
		PlatformCommission:  0,
		OperatorCommission:  0,
		MerchantAmount:      order.TotalAmount,
		OutOrderNo:          fmt.Sprintf("PS_OUT_%d", orderID),
		Status:              "pending",
	})
	require.NoError(t, err)

	// backdate created_at so ListProfitSharingOrdersForRetry can pick it up
	_, err = integrationPool.Exec(ctx, `UPDATE profit_sharing_orders SET created_at = $2 WHERE id = $1`, ps.ID, time.Now().Add(-20*time.Minute))
	require.NoError(t, err)

	d := &captureTaskDistributor{}
	recovery := worker.NewProfitSharingRecoveryScheduler(store, d)
	recovery.RunOnce()

	calls := d.Calls()
	require.Len(t, calls, 1)
	require.Equal(t, po.ID, calls[0].PaymentOrderID)
	require.Equal(t, orderID, calls[0].OrderID)
}

func doJSON(t *testing.T, server *api.Server, method, url string, body any, userID int64) *httptest.ResponseRecorder {
	t.Helper()

	var payload []byte
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		payload = b
	} else {
		payload = []byte("{}")
	}

	req, err := http.NewRequest(method, url, bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthorization(t, req, integrationTokenMaker, userID, time.Minute)

	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	return rec
}

func doGET(t *testing.T, server *api.Server, url string, userID int64) *httptest.ResponseRecorder {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	addAuthorization(t, req, integrationTokenMaker, userID, time.Minute)

	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	return rec
}

func numericE7(v int64) pgtype.Numeric {
	return pgtype.Numeric{Int: big.NewInt(v), Exp: -7, Valid: true}
}

func ensureIntegrationMerchantCoords(t *testing.T, store *db.SQLStore, merchantID int64) db.Merchant {
	t.Helper()

	_, err := integrationPool.Exec(context.Background(), `UPDATE merchants SET longitude = $2, latitude = $3 WHERE id = $1`,
		merchantID,
		numericE7(1163970000),
		numericE7(399085000),
	)
	require.NoError(t, err)

	m, err := store.GetMerchant(context.Background(), merchantID)
	require.NoError(t, err)
	require.True(t, m.Longitude.Valid)
	require.True(t, m.Latitude.Valid)
	return m
}

func createIntegrationDish(t *testing.T, store *db.SQLStore, merchantID int64) db.Dish {
	t.Helper()

	res, err := store.CreateDishTx(context.Background(), db.CreateDishTxParams{
		MerchantID:  merchantID,
		CategoryID:  pgtype.Int8{Valid: false},
		Name:        "集成测试菜品",
		Description: pgtype.Text{String: "integration", Valid: true},
		ImageUrl:    pgtype.Text{String: "https://example.com/dish.png", Valid: true},
		Price:       1999,
		MemberPrice: pgtype.Int8{Valid: false},
		IsAvailable: true,
		IsOnline:    true,
		SortOrder:   0,
		PrepareTime: 10,
	})
	require.NoError(t, err)
	return res.Dish
}

func createIntegrationUserAddress(t *testing.T, store *db.SQLStore, userID int64, regionID int64) db.UserAddress {
	t.Helper()

	addr, err := store.CreateUserAddress(context.Background(), db.CreateUserAddressParams{
		UserID:        userID,
		RegionID:      regionID,
		DetailAddress: "东华门街道 1 号",
		ContactName:   "张三",
		ContactPhone:  "13800138001",
		Longitude:     numericE7(1163975000),
		Latitude:      numericE7(399084000),
		IsDefault:     true,
	})
	require.NoError(t, err)
	return addr
}

func createIntegrationRider(t *testing.T, store *db.SQLStore, userID, regionID int64) db.Rider {
	t.Helper()

	phone := fmt.Sprintf("139%08d", util.RandomInt(0, 99999999))
	rider, err := store.CreateRider(context.Background(), db.CreateRiderParams{
		UserID:   userID,
		RealName: "测试骑手",
		IDCardNo: "110101199001011234",
		Phone:    phone,
		RegionID: pgtype.Int8{Int64: regionID, Valid: true},
	})
	require.NoError(t, err)
	return rider
}
