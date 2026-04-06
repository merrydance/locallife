package worker

import (
	"context"
	"encoding/json"
	"errors"
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

func TestAbnormalRefundActionExtra_ForEcommerceRefundIncludesAdminAction(t *testing.T) {
	paymentOrder := db.PaymentOrder{
		ID:          11,
		PaymentType: "profit_sharing",
	}
	refundOrder := db.RefundOrder{
		ID:       44,
		RefundID: pgtype.Text{String: "WR123", Valid: true},
	}

	extra := abnormalRefundActionExtra(paymentOrder, refundOrder)

	require.Equal(t, true, extra["abnormal_refund_api_available"])
	require.Equal(t, "POST", extra["abnormal_refund_api_method"])
	require.Equal(t, "/v1/platform/refunds/44/apply-abnormal-refund", extra["abnormal_refund_api_path"])
	require.Equal(t, wechat.EcommerceAbnormalRefundTypeMerchantBankCard, extra["abnormal_refund_default_type"])
	require.Equal(t, []string{wechat.EcommerceAbnormalRefundTypeMerchantBankCard, wechat.EcommerceAbnormalRefundTypeUserBankCard}, extra["abnormal_refund_supported_types"])
	require.Equal(t, []string{"bank_type", "bank_account", "real_name"}, extra["abnormal_refund_user_bank_card_required_fields"])
}

func TestAbnormalRefundActionExtra_SkipsNonEcommerceRefund(t *testing.T) {
	paymentOrder := db.PaymentOrder{
		ID:          11,
		PaymentType: "miniprogram",
	}
	refundOrder := db.RefundOrder{
		ID:       44,
		RefundID: pgtype.Text{String: "WR123", Valid: true},
	}

	require.Nil(t, abnormalRefundActionExtra(paymentOrder, refundOrder))
}

func TestProcessTaskRefundResult_AbnormalPublishesActionableAlertForEcommerceRefund(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, nil)
	publisher := &testPublisher{}
	processor.pubSubPublisher = publisher

	paymentOrder := db.PaymentOrder{
		ID:          11,
		OrderID:     pgtype.Int8{Int64: 22, Valid: true},
		UserID:      33,
		PaymentType: "profit_sharing",
		Amount:      4567,
		OutTradeNo:  "OT123",
	}
	refundOrder := db.RefundOrder{
		ID:             44,
		PaymentOrderID: paymentOrder.ID,
		RefundAmount:   1200,
		RefundType:     "user_cancel",
		OutRefundNo:    "RF123",
		RefundID:       pgtype.Text{String: "WR123", Valid: true},
		Status:         "processing",
	}

	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), refundOrder.OutRefundNo).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), refundOrder.PaymentOrderID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), int64(22)).Return(db.Order{ID: 22, MerchantID: 55}, nil)
	store.EXPECT().UpdateRefundOrderToFailed(gomock.Any(), refundOrder.ID).Return(db.RefundOrder{ID: refundOrder.ID, Status: "failed"}, nil)

	payloadBytes, err := json.Marshal(RefundResultPayload{
		OutRefundNo:  refundOrder.OutRefundNo,
		RefundStatus: "ABNORMAL",
		RefundID:     refundOrder.RefundID.String,
	})
	require.NoError(t, err)

	err = processor.ProcessTaskRefundResult(context.Background(), asynq.NewTask(TaskProcessRefundResult, payloadBytes))
	require.NoError(t, err)
	require.Equal(t, AlertChannel, publisher.channel)

	var published map[string]interface{}
	require.NoError(t, json.Unmarshal(publisher.payload, &published))
	data := published["data"].(map[string]interface{})
	extra := data["extra"].(map[string]interface{})
	require.Equal(t, true, extra["abnormal_refund_api_available"])
	require.Equal(t, "POST", extra["abnormal_refund_api_method"])
	require.Equal(t, "/v1/platform/refunds/44/apply-abnormal-refund", extra["abnormal_refund_api_path"])
	require.Equal(t, wechat.EcommerceAbnormalRefundTypeMerchantBankCard, extra["abnormal_refund_default_type"])
	require.Equal(t, "WR123", extra["refund_id"])

	fields := extra["abnormal_refund_user_bank_card_required_fields"].([]interface{})
	require.Equal(t, []interface{}{"bank_type", "bank_account", "real_name"}, fields)
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
		Status:       "FAIL",
		Reason:       "balance not enough",
	}, nil)
	store.EXPECT().UpdateWithdrawalStatus(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.UpdateWithdrawalStatusParams) (db.WithdrawalRecord, error) {
			require.Equal(t, record.ID, arg.ID)
			require.Equal(t, "failed", arg.Status)
			require.True(t, arg.Reason.Valid)
			require.False(t, arg.ClearReason)
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
	require.Equal(t, string(AlertTypeWithdrawFailed), data["alert_type"])
	require.Equal(t, "商户提现失败", data["title"])
	extra := data["extra"].(map[string]interface{})
	require.EqualValues(t, float64(record.ID), extra["withdrawal_record_id"])
	require.EqualValues(t, float64(88), extra["merchant_id"])
	require.Equal(t, "req-1", extra["out_request_no"])
	require.Equal(t, "sub-mch-1", extra["sub_mch_id"])
	require.Equal(t, "FAIL", extra["wechat_status"])
	require.Equal(t, "balance not enough", extra["fail_reason"])
}

func TestProcessTaskMerchantWithdrawResult_QueryFailureKeepsPendingForRecovery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, ecommerceClient)
	publisher := &testPublisher{}
	processor.pubSubPublisher = publisher
	queryErr := errors.New("wechat unavailable")

	accountInfoBytes, err := json.Marshal(merchantWithdrawAccountInfo{
		MerchantID:   88,
		SubMchID:     "sub-mch-1",
		OutRequestNo: "req-1",
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
	ecommerceClient.EXPECT().QueryEcommerceWithdrawByOutRequestNo(gomock.Any(), "sub-mch-1", "req-1").Return(nil, queryErr)
	store.EXPECT().UpdateWithdrawalStatus(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.UpdateWithdrawalStatusParams) (db.WithdrawalRecord, error) {
			require.Equal(t, record.ID, arg.ID)
			require.Equal(t, "pending", arg.Status)
			require.True(t, arg.Reason.Valid)
			require.False(t, arg.ClearReason)
			return record, nil
		},
	)

	payloadBytes, err := json.Marshal(MerchantWithdrawResultPayload{WithdrawalRecordID: record.ID, RetryCount: merchantWithdrawMaxRetry})
	require.NoError(t, err)

	err = processor.ProcessTaskMerchantWithdrawResult(context.Background(), asynq.NewTask(TaskProcessMerchantWithdrawResult, payloadBytes))
	require.Error(t, err)
	require.Equal(t, AlertChannel, publisher.channel)

	var published map[string]interface{}
	require.NoError(t, json.Unmarshal(publisher.payload, &published))
	data := published["data"].(map[string]interface{})
	require.Equal(t, "商户提现结果查询失败", data["title"])
	extra := data["extra"].(map[string]interface{})
	require.EqualValues(t, float64(record.ID), extra["withdrawal_record_id"])
	require.Equal(t, "req-1", extra["out_request_no"])
	require.Equal(t, queryErr.Error(), extra["fail_reason"])
}

func TestProcessTaskMerchantWithdrawResult_RequestNotFoundMarksFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, ecommerceClient)
	publisher := &testPublisher{}
	processor.pubSubPublisher = publisher
	queryErr := &wechat.WechatPayError{StatusCode: 404, Code: "RESOURCE_NOT_EXISTS", Message: "withdraw not found"}

	accountInfoBytes, err := json.Marshal(merchantWithdrawAccountInfo{
		MerchantID:   88,
		SubMchID:     "sub-mch-1",
		OutRequestNo: "req-404",
	})
	require.NoError(t, err)

	record := db.WithdrawalRecord{
		ID:          67,
		UserID:      99,
		Amount:      12345,
		Status:      "pending",
		Channel:     merchantWithdrawChannel,
		AccountInfo: accountInfoBytes,
	}

	store.EXPECT().GetWithdrawalRecord(gomock.Any(), record.ID).Return(record, nil)
	ecommerceClient.EXPECT().QueryEcommerceWithdrawByOutRequestNo(gomock.Any(), "sub-mch-1", "req-404").Return(nil, queryErr)
	store.EXPECT().UpdateWithdrawalStatus(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.UpdateWithdrawalStatusParams) (db.WithdrawalRecord, error) {
			require.Equal(t, record.ID, arg.ID)
			require.Equal(t, "failed", arg.Status)
			require.True(t, arg.Reason.Valid)
			require.Contains(t, arg.Reason.String, "withdraw request not found in wechat")
			require.False(t, arg.ClearReason)
			return record, nil
		},
	)

	payloadBytes, err := json.Marshal(MerchantWithdrawResultPayload{WithdrawalRecordID: record.ID, RetryCount: merchantWithdrawMaxRetry})
	require.NoError(t, err)

	err = processor.ProcessTaskMerchantWithdrawResult(context.Background(), asynq.NewTask(TaskProcessMerchantWithdrawResult, payloadBytes))
	require.Error(t, err)
	require.Equal(t, AlertChannel, publisher.channel)

	var published map[string]interface{}
	require.NoError(t, json.Unmarshal(publisher.payload, &published))
	data := published["data"].(map[string]interface{})
	require.Equal(t, "商户提现提交状态不明", data["title"])
	extra := data["extra"].(map[string]interface{})
	require.EqualValues(t, float64(record.ID), extra["withdrawal_record_id"])
	require.Equal(t, "req-404", extra["out_request_no"])
	require.Equal(t, "withdraw_request_not_found", extra["result"])
	require.Contains(t, extra["fail_reason"].(string), "RESOURCE_NOT_EXISTS")
}

func TestProcessTaskMerchantWithdrawResult_TerminalStatusSkipsWechatQuery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, ecommerceClient)

	accountInfoBytes, err := json.Marshal(merchantWithdrawAccountInfo{
		MerchantID:   88,
		SubMchID:     "sub-mch-1",
		OutRequestNo: "req-success",
	})
	require.NoError(t, err)

	record := db.WithdrawalRecord{
		ID:          68,
		UserID:      99,
		Amount:      12345,
		Status:      "success",
		Channel:     merchantWithdrawChannel,
		AccountInfo: accountInfoBytes,
	}

	store.EXPECT().GetWithdrawalRecord(gomock.Any(), record.ID).Return(record, nil)
	ecommerceClient.EXPECT().QueryEcommerceWithdrawByOutRequestNo(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	payloadBytes, err := json.Marshal(MerchantWithdrawResultPayload{WithdrawalRecordID: record.ID, RetryCount: 1})
	require.NoError(t, err)

	err = processor.ProcessTaskMerchantWithdrawResult(context.Background(), asynq.NewTask(TaskProcessMerchantWithdrawResult, payloadBytes))
	require.NoError(t, err)
}

func TestProcessTaskMerchantWithdrawResult_ClearsStaleReasonOnSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, ecommerceClient)

	accountInfoBytes, err := json.Marshal(merchantWithdrawAccountInfo{
		MerchantID:   88,
		SubMchID:     "sub-mch-1",
		OutRequestNo: "req-success",
	})
	require.NoError(t, err)

	record := db.WithdrawalRecord{
		ID:          69,
		UserID:      99,
		Amount:      12345,
		Status:      "pending",
		Channel:     merchantWithdrawChannel,
		Reason:      pgtype.Text{String: "query withdraw result failed: timeout", Valid: true},
		AccountInfo: accountInfoBytes,
	}

	store.EXPECT().GetWithdrawalRecord(gomock.Any(), record.ID).Return(record, nil)
	ecommerceClient.EXPECT().QueryEcommerceWithdrawByOutRequestNo(gomock.Any(), "sub-mch-1", "req-success").Return(&wechat.EcommerceWithdrawResponse{
		SubMchID:     "sub-mch-1",
		WithdrawID:   "wd-1",
		OutRequestNo: "req-success",
		Status:       "SUCCESS",
	}, nil)
	store.EXPECT().UpdateWithdrawalStatus(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.UpdateWithdrawalStatusParams) (db.WithdrawalRecord, error) {
			require.Equal(t, record.ID, arg.ID)
			require.Equal(t, "success", arg.Status)
			require.False(t, arg.Reason.Valid)
			require.True(t, arg.ClearReason)
			return record, nil
		},
	)

	payloadBytes, err := json.Marshal(MerchantWithdrawResultPayload{WithdrawalRecordID: record.ID, RetryCount: 1})
	require.NoError(t, err)

	err = processor.ProcessTaskMerchantWithdrawResult(context.Background(), asynq.NewTask(TaskProcessMerchantWithdrawResult, payloadBytes))
	require.NoError(t, err)
}
