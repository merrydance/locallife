package worker

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type testPublisher struct {
	channel string
	payload []byte
}

func (p *testPublisher) Publish(ctx context.Context, channel string, payload []byte) error {
	p.channel = channel
	p.payload = append([]byte(nil), payload...)
	return nil
}

func TestRefundOrderAlertExtra_IncludesCommonIdentifiers(t *testing.T) {
	paymentOrder := db.PaymentOrder{
		ID:            11,
		OrderID:       pgtype.Int8{Int64: 22, Valid: true},
		UserID:        33,
		PaymentType:   "profit_sharing",
		BusinessType:  "takeout_order",
		Amount:        4567,
		OutTradeNo:    "OT123",
		TransactionID: pgtype.Text{String: "WX123", Valid: true},
	}
	refundOrder := db.RefundOrder{
		ID:             44,
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "user_cancel",
		RefundAmount:   1200,
		OutRefundNo:    "RF123",
		RefundID:       pgtype.Text{String: "WR123", Valid: true},
	}

	extra := refundOrderAlertExtra(paymentOrder, refundOrder, 55, map[string]interface{}{"wechat_status": "ABNORMAL"})

	require.EqualValues(t, paymentOrder.ID, extra["payment_order_id"])
	require.EqualValues(t, int64(22), extra["order_id"])
	require.EqualValues(t, int64(55), extra["merchant_id"])
	require.Equal(t, "OT123", extra["out_trade_no"])
	require.Equal(t, "WX123", extra["transaction_id"])
	require.EqualValues(t, refundOrder.ID, extra["refund_order_id"])
	require.Equal(t, "RF123", extra["out_refund_no"])
	require.Equal(t, "WR123", extra["refund_id"])
	require.Equal(t, "ABNORMAL", extra["wechat_status"])
}

func TestProcessTaskMerchantWithdrawResult_FailedPublishesAlert(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, ecommerceClient)
	publisher := &testPublisher{}
	processor.pubSubPublisher = publisher

	accountInfoBytes, err := json.Marshal(merchantWithdrawAccountInfo{
		MerchantID:   88,
		SubMchID:     "sub-mch-1",
		OutRequestNo: "req-1",
		Remark:       "merchant withdraw",
	})
	require.NoError(t, err)

	record := db.WithdrawalRecord{
		ID:          66,
		UserID:      99,
		Amount:      12345,
		Status:      "pending",
		Channel:     merchantWithdrawChannel,
		AccountInfo: accountInfoBytes,
	}

	store.EXPECT().GetWithdrawalRecord(gomock.Any(), record.ID).Return(record, nil)
	ecommerceClient.EXPECT().QueryEcommerceWithdrawByOutRequestNo(gomock.Any(), "sub-mch-1", "req-1").Return(&wechat.EcommerceWithdrawResponse{
		SubMchID:     "sub-mch-1",
		WithdrawID:   "wd-1",
		OutRequestNo: "req-1",
		Status:       "FAILED",
		FailReason:   "balance not enough",
	}, nil)
	store.EXPECT().UpdateWithdrawalStatus(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.UpdateWithdrawalStatusParams) (db.WithdrawalRecord, error) {
			require.Equal(t, record.ID, arg.ID)
			require.Equal(t, "failed", arg.Status)
			require.True(t, arg.Reason.Valid)
			return record, nil
		},
	)

	payloadBytes, err := json.Marshal(MerchantWithdrawResultPayload{WithdrawalRecordID: record.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskMerchantWithdrawResult(context.Background(), asynq.NewTask(TaskProcessMerchantWithdrawResult, payloadBytes))
	require.NoError(t, err)
	require.Equal(t, AlertChannel, publisher.channel)

	var published map[string]interface{}
	require.NoError(t, json.Unmarshal(publisher.payload, &published))
	data := published["data"].(map[string]interface{})
	require.Equal(t, string(AlertTypeRefundFailed), data["alert_type"])
	require.Equal(t, "商户提现失败", data["title"])
	extra := data["extra"].(map[string]interface{})
	require.EqualValues(t, float64(record.ID), extra["withdrawal_record_id"])
	require.EqualValues(t, float64(88), extra["merchant_id"])
	require.Equal(t, "req-1", extra["out_request_no"])
	require.Equal(t, "sub-mch-1", extra["sub_mch_id"])
	require.Equal(t, "FAILED", extra["wechat_status"])
	require.Equal(t, "balance not enough", extra["fail_reason"])
}
