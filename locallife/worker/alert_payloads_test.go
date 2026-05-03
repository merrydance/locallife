package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	wechaterrorcodes "github.com/merrydance/locallife/wechat/errorcodes"
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
		ID:             11,
		OrderID:        pgtype.Int8{Int64: 22, Valid: true},
		UserID:         33,
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelEcommerce,
		BusinessType:   "takeout_order",
		Amount:         4567,
		OutTradeNo:     "OT123",
		TransactionID:  pgtype.Text{String: "WX123", Valid: true},
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
		ID:             11,
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelEcommerce,
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

func TestBaofuAlertExtraIncludesProviderChannelAndSanitizesSensitiveFields(t *testing.T) {
	extra := baofuReconciliationAlertExtra(map[string]interface{}{
		"payment_order_id": 11,
		"provider":         "bad-provider",
		"channel":          "bad-channel",
		"contract_no":      "CONTRACT-SECRET",
		"sharing_mer_id":   "SHARE-SECRET",
		"sharingMerId":     "SHARE-SECRET",
		"raw_resource":     map[string]any{"id_card": "secret"},
		"signature":        "SIGN",
	})

	require.Equal(t, db.ExternalPaymentProviderBaofu, extra["provider"])
	require.Equal(t, db.PaymentChannelBaofuAggregate, extra["channel"])
	require.EqualValues(t, int64(11), extra["payment_order_id"])
	require.NotContains(t, extra, "contract_no")
	require.NotContains(t, extra, "sharing_mer_id")
	require.NotContains(t, extra, "sharingMerId")
	require.NotContains(t, extra, "raw_resource")
	require.NotContains(t, extra, "signature")
}

func TestBaofuReconciliationAlertsUseSanitizedProviderPayloads(t *testing.T) {
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)

	alerts := []AlertData{
		newBaofuPaymentCallbackMissingAlert(db.PaymentOrder{ID: 11, Status: "pending", CreatedAt: now.Add(-2 * time.Hour)}, time.Hour),
		newBaofuProfitSharingProcessingSLAAlert(db.ProfitSharingOrder{ID: 12, PaymentOrderID: 11, Status: db.ProfitSharingOrderStatusProcessing, CreatedAt: now.Add(-2 * time.Hour)}, time.Hour),
		newBaofuWithdrawalProcessingSLAAlert(db.BaofuWithdrawalOrder{ID: 13, Status: db.BaofuWithdrawalStatusProcessing, CreatedAt: now.Add(-2 * time.Hour), Amount: 5000}, time.Hour),
		newBaofuFailedFactAlert(db.ExternalPaymentFact{ID: 14, Provider: db.ExternalPaymentProviderBaofu, Channel: db.PaymentChannelBaofuAggregate, ProcessingStatus: db.ExternalPaymentFactProcessingStatusFailed}),
		newBaofuFeeLedgerMismatchAlert(15, 11, 30, 29),
	}

	require.Equal(t, AlertTypeBaofuPaymentCallbackMissing, alerts[0].AlertType)
	require.Equal(t, AlertTypeBaofuShareProcessingSLA, alerts[1].AlertType)
	require.Equal(t, AlertTypeBaofuWithdrawalProcessingSLA, alerts[2].AlertType)
	require.Equal(t, AlertTypeBaofuFactApplicationFailed, alerts[3].AlertType)
	require.Equal(t, AlertTypeBaofuFeeLedgerMismatch, alerts[4].AlertType)
	for _, alert := range alerts {
		require.Equal(t, AlertLevelWarning, alert.Level)
		require.Equal(t, db.ExternalPaymentProviderBaofu, alert.Extra["provider"])
		require.Equal(t, db.PaymentChannelBaofuAggregate, alert.Extra["channel"])
		require.NotContains(t, alert.Extra, "contract_no")
		require.NotContains(t, alert.Extra, "sharing_mer_id")
		require.NotContains(t, alert.Extra, "raw_resource")
	}
}

func TestProcessTaskRefundResult_AbnormalPublishesActionableAlertForEcommerceRefund(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := NewTestTaskProcessor(store, nil, nil, nil)
	publisher := &testPublisher{}
	processor.pubSubPublisher = publisher

	paymentOrder := db.PaymentOrder{
		ID:             11,
		OrderID:        pgtype.Int8{Int64: 22, Valid: true},
		UserID:         33,
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelEcommerce,
		Amount:         4567,
		OutTradeNo:     "OT123",
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
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).DoAndReturn(
		func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, db.ExternalPaymentCapabilityWithdraw, arg.Capability)
			require.Equal(t, db.ExternalPaymentFactSourceQuery, arg.FactSource)
			require.Equal(t, db.ExternalPaymentObjectWithdraw, arg.ExternalObjectType)
			require.Equal(t, "req-1", arg.ExternalObjectKey)
			require.Equal(t, "wd-1", arg.ExternalSecondaryKey.String)
			require.True(t, arg.ExternalSecondaryKey.Valid)
			require.Equal(t, db.ExternalPaymentBusinessOwnerMerchantFunds, arg.BusinessOwner.String)
			require.Equal(t, merchantWithdrawFactBusinessType, arg.BusinessObjectType.String)
			require.Equal(t, record.ID, arg.BusinessObjectID.Int64)
			require.Equal(t, "FAIL", arg.UpstreamState)
			require.Equal(t, db.ExternalPaymentTerminalStatusFailed, arg.TerminalStatus)
			require.Equal(t, record.Amount, arg.Amount.Int64)
			require.Equal(t, "wechat:query:ecommerce:withdraw:req-1:FAIL:wd-1", arg.DedupeKey)
			var payload map[string]any
			require.NoError(t, json.Unmarshal(arg.RawResource, &payload))
			require.EqualValues(t, record.ID, payload["withdrawal_record_id"])
			require.Equal(t, "req-1", payload["out_request_no"])
			require.Equal(t, "wd-1", payload["withdraw_id"])
			require.Equal(t, "FAIL", payload["wechat_status"])
			require.Equal(t, "balance not enough", payload["reason"])
			return db.ExternalPaymentFact{ID: 8101, DedupeKey: arg.DedupeKey, IsTerminal: true, TerminalStatus: arg.TerminalStatus}, nil
		},
	)
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             8101,
		Consumer:           merchantWithdrawFactConsumerDomain,
		BusinessObjectType: merchantWithdrawFactBusinessType,
		BusinessObjectID:   record.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{
		ID:                 8102,
		FactID:             8101,
		Consumer:           merchantWithdrawFactConsumerDomain,
		BusinessObjectType: merchantWithdrawFactBusinessType,
		BusinessObjectID:   record.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}, nil)
	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), int64(8102)).Return(db.ExternalPaymentFactApplication{
		ID:                 8102,
		FactID:             8101,
		Consumer:           merchantWithdrawFactConsumerDomain,
		BusinessObjectType: merchantWithdrawFactBusinessType,
		BusinessObjectID:   record.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), int64(8101)).Return(db.ExternalPaymentFact{
		ID:                   8101,
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelEcommerce,
		Capability:           db.ExternalPaymentCapabilityWithdraw,
		ExternalObjectType:   db.ExternalPaymentObjectWithdraw,
		ExternalObjectKey:    "req-1",
		ExternalSecondaryKey: pgtype.Text{String: "wd-1", Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerMerchantFunds, Valid: true},
		BusinessObjectType:   pgtype.Text{String: merchantWithdrawFactBusinessType, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: record.ID, Valid: true},
		UpstreamState:        "FAIL",
		TerminalStatus:       db.ExternalPaymentTerminalStatusFailed,
		IsTerminal:           true,
		RawResource:          []byte(`{"withdrawal_record_id":66,"out_request_no":"req-1","withdraw_id":"wd-1","wechat_status":"FAIL","reason":"balance not enough","amount":12345}`),
	}, nil)
	store.EXPECT().GetWithdrawalRecord(gomock.Any(), record.ID).Return(record, nil)
	store.EXPECT().UpdateWithdrawalStatus(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.UpdateWithdrawalStatusParams) (db.WithdrawalRecord, error) {
			require.Equal(t, record.ID, arg.ID)
			require.Equal(t, "failed", arg.Status)
			require.True(t, arg.Reason.Valid)
			require.False(t, arg.ClearReason)
			updated := record
			updated.Status = "failed"
			updated.Reason = pgtype.Text{String: arg.Reason.String, Valid: true}
			return updated, nil
		},
	)
	store.EXPECT().UpdateExternalPaymentFactProcessingStatus(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateExternalPaymentFactProcessingStatusParams{})).Return(db.ExternalPaymentFact{ID: 8101, ProcessingStatus: db.ExternalPaymentFactProcessingStatusTerminalized}, nil)
	store.EXPECT().MarkExternalPaymentFactApplicationApplied(gomock.Any(), gomock.AssignableToTypeOf(db.MarkExternalPaymentFactApplicationAppliedParams{})).Return(db.ExternalPaymentFactApplication{ID: 8102, FactID: 8101, Status: db.ExternalPaymentFactApplicationStatusApplied}, nil)

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
	queryErr := &wechat.WechatPayError{StatusCode: 404, Code: wechaterrorcodes.FundManagementCodeOrderNotExist, Message: "withdraw not found"}

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
	require.Contains(t, extra["fail_reason"].(string), wechaterrorcodes.FundManagementCodeOrderNotExist)
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
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.AssignableToTypeOf(db.CreateExternalPaymentFactParams{})).DoAndReturn(
		func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
			require.Equal(t, db.ExternalPaymentCapabilityWithdraw, arg.Capability)
			require.Equal(t, db.ExternalPaymentFactSourceQuery, arg.FactSource)
			require.Equal(t, db.ExternalPaymentObjectWithdraw, arg.ExternalObjectType)
			require.Equal(t, "req-success", arg.ExternalObjectKey)
			require.Equal(t, "wd-1", arg.ExternalSecondaryKey.String)
			require.True(t, arg.ExternalSecondaryKey.Valid)
			require.Equal(t, db.ExternalPaymentBusinessOwnerMerchantFunds, arg.BusinessOwner.String)
			require.Equal(t, merchantWithdrawFactBusinessType, arg.BusinessObjectType.String)
			require.Equal(t, record.ID, arg.BusinessObjectID.Int64)
			require.Equal(t, "SUCCESS", arg.UpstreamState)
			require.Equal(t, db.ExternalPaymentTerminalStatusSuccess, arg.TerminalStatus)
			require.Equal(t, record.Amount, arg.Amount.Int64)
			require.Equal(t, "wechat:query:ecommerce:withdraw:req-success:SUCCESS:wd-1", arg.DedupeKey)
			var payload map[string]any
			require.NoError(t, json.Unmarshal(arg.RawResource, &payload))
			require.EqualValues(t, record.ID, payload["withdrawal_record_id"])
			require.Equal(t, "req-success", payload["out_request_no"])
			require.Equal(t, "wd-1", payload["withdraw_id"])
			require.Equal(t, "SUCCESS", payload["wechat_status"])
			return db.ExternalPaymentFact{ID: 8102, DedupeKey: arg.DedupeKey, IsTerminal: true, TerminalStatus: arg.TerminalStatus}, nil
		},
	)
	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             8102,
		Consumer:           merchantWithdrawFactConsumerDomain,
		BusinessObjectType: merchantWithdrawFactBusinessType,
		BusinessObjectID:   record.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{
		ID:                 8103,
		FactID:             8102,
		Consumer:           merchantWithdrawFactConsumerDomain,
		BusinessObjectType: merchantWithdrawFactBusinessType,
		BusinessObjectID:   record.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}, nil)
	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), int64(8103)).Return(db.ExternalPaymentFactApplication{
		ID:                 8103,
		FactID:             8102,
		Consumer:           merchantWithdrawFactConsumerDomain,
		BusinessObjectType: merchantWithdrawFactBusinessType,
		BusinessObjectID:   record.ID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}, nil)
	store.EXPECT().GetExternalPaymentFact(gomock.Any(), int64(8102)).Return(db.ExternalPaymentFact{
		ID:                   8102,
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelEcommerce,
		Capability:           db.ExternalPaymentCapabilityWithdraw,
		ExternalObjectType:   db.ExternalPaymentObjectWithdraw,
		ExternalObjectKey:    "req-success",
		ExternalSecondaryKey: pgtype.Text{String: "wd-1", Valid: true},
		BusinessOwner:        pgtype.Text{String: db.ExternalPaymentBusinessOwnerMerchantFunds, Valid: true},
		BusinessObjectType:   pgtype.Text{String: merchantWithdrawFactBusinessType, Valid: true},
		BusinessObjectID:     pgtype.Int8{Int64: record.ID, Valid: true},
		UpstreamState:        "SUCCESS",
		TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
		IsTerminal:           true,
		RawResource:          []byte(`{"withdrawal_record_id":69,"out_request_no":"req-success","withdraw_id":"wd-1","wechat_status":"SUCCESS","reason":"","amount":12345}`),
	}, nil)
	store.EXPECT().GetWithdrawalRecord(gomock.Any(), record.ID).Return(record, nil)
	store.EXPECT().UpdateWithdrawalStatus(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.UpdateWithdrawalStatusParams) (db.WithdrawalRecord, error) {
			require.Equal(t, record.ID, arg.ID)
			require.Equal(t, "success", arg.Status)
			require.False(t, arg.Reason.Valid)
			require.True(t, arg.ClearReason)
			updated := record
			updated.Status = "success"
			updated.Reason = pgtype.Text{}
			return updated, nil
		},
	)
	store.EXPECT().UpdateExternalPaymentFactProcessingStatus(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateExternalPaymentFactProcessingStatusParams{})).Return(db.ExternalPaymentFact{ID: 8102, ProcessingStatus: db.ExternalPaymentFactProcessingStatusTerminalized}, nil)
	store.EXPECT().MarkExternalPaymentFactApplicationApplied(gomock.Any(), gomock.AssignableToTypeOf(db.MarkExternalPaymentFactApplicationAppliedParams{})).Return(db.ExternalPaymentFactApplication{ID: 8103, FactID: 8102, Status: db.ExternalPaymentFactApplicationStatusApplied}, nil)

	payloadBytes, err := json.Marshal(MerchantWithdrawResultPayload{WithdrawalRecordID: record.ID, RetryCount: 1})
	require.NoError(t, err)

	err = processor.ProcessTaskMerchantWithdrawResult(context.Background(), asynq.NewTask(TaskProcessMerchantWithdrawResult, payloadBytes))
	require.NoError(t, err)
}
