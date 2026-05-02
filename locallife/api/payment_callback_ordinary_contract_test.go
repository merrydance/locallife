package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	ordinaryserviceprovider "github.com/merrydance/locallife/wechat/ordinaryserviceprovider"
	mockordinaryserviceprovider "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestOrdinaryRefundResourceFromEnvelopeRequiresOwnershipAndPaymentFields(t *testing.T) {
	base := map[string]any{
		"sp_mchid":       "sp-mch-001",
		"sub_mchid":      "sub-mch-001",
		"mchid":          "sub-mch-001",
		"out_trade_no":   "trade-001",
		"transaction_id": "wx-txn-001",
		"out_refund_no":  "refund-001",
		"refund_id":      "wx-refund-001",
		"refund_status":  "SUCCESS",
		"amount": map[string]any{
			"refund": float64(100),
		},
	}
	tests := []struct {
		name      string
		removeKey string
		want      string
	}{
		{name: "missing provider mchid", removeKey: "sp_mchid", want: "sp_mchid"},
		{name: "missing sub mchid", removeKey: "sub_mchid", want: "sub_mchid_or_mchid"},
		{name: "missing out trade no", removeKey: "out_trade_no", want: "out_trade_no"},
		{name: "missing transaction id", removeKey: "transaction_id", want: "transaction_id"},
		{name: "missing refund id", removeKey: "refund_id", want: "refund_id"},
		{name: "missing refund amount", removeKey: "amount", want: "amount.refund"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := cloneStringAnyMap(base)
			delete(payload, tt.removeKey)
			envelope := ordinaryNotificationEnvelopeForTest(t, payload)

			resource, err := ordinaryRefundResourceFromEnvelope(envelope)

			require.Nil(t, resource)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestOrdinaryRefundResourceFromEnvelopeRejectsConflictingMerchantFields(t *testing.T) {
	envelope := ordinaryNotificationEnvelopeForTest(t, map[string]any{
		"sp_mchid":       "sp-mch-001",
		"sub_mchid":      "sub-mch-001",
		"mchid":          "sub-mch-002",
		"out_trade_no":   "trade-001",
		"transaction_id": "wx-txn-001",
		"out_refund_no":  "refund-001",
		"refund_id":      "wx-refund-001",
		"refund_status":  "SUCCESS",
		"amount": map[string]any{
			"refund": float64(100),
		},
	})

	resource, err := ordinaryRefundResourceFromEnvelope(envelope)

	require.Nil(t, resource)
	require.Error(t, err)
	require.Contains(t, err.Error(), "sub_mchid mismatch mchid")
}

func TestValidateOrdinaryRefundLocalOwnershipChecksSubMerchantAndAmount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := &Server{store: store}
	resource := &ordinaryRefundNotificationResource{
		SpMchID:       "sp-mch-001",
		SubMchID:      "sub-mch-001",
		MchID:         "sub-mch-001",
		OutTradeNo:    "trade-001",
		TransactionID: "wx-txn-001",
		OutRefundNo:   "refund-001",
		RefundID:      "wx-refund-001",
		RefundStatus:  "SUCCESS",
		Amount:        ordinaryRefundNotificationAmount{Refund: 100},
	}
	refundOrder := db.RefundOrder{ID: 61, PaymentOrderID: 71, OutRefundNo: "refund-001", RefundAmount: 100}
	paymentOrder := db.PaymentOrder{
		ID:            71,
		OrderID:       pgtype.Int8{Int64: 81, Valid: true},
		OutTradeNo:    "trade-001",
		TransactionID: pgtype.Text{String: "wx-txn-001", Valid: true},
	}

	store.EXPECT().GetOrder(gomock.Any(), int64(81)).Times(2).Return(db.Order{ID: 81, MerchantID: 91}, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), int64(91)).Times(2).Return(db.MerchantPaymentConfig{MerchantID: 91, SubMchID: "sub-mch-001"}, nil)

	require.NoError(t, server.validateOrdinaryRefundLocalOwnership(context.Background(), resource, refundOrder, paymentOrder))

	resource.SubMchID = "sub-mch-other"
	resource.MchID = "sub-mch-other"
	err := server.validateOrdinaryRefundLocalOwnership(context.Background(), resource, refundOrder, paymentOrder)
	require.Error(t, err)
	require.Contains(t, err.Error(), "sub_mchid mismatch")
}

func TestOrdinaryProfitSharingResourceFromEnvelopeRequiresContractFields(t *testing.T) {
	base := map[string]any{
		"sp_mchid":       "sp-mch-001",
		"sub_mchid":      "sub-mch-001",
		"transaction_id": "wx-txn-001",
		"order_id":       "wx-profit-sharing-001",
		"out_order_no":   "profit-sharing-001",
		"receiver": map[string]any{
			"type":        "MERCHANT_ID",
			"account":     "sub-mch-001",
			"amount":      float64(100),
			"description": "订单分账",
		},
	}
	tests := []struct {
		name      string
		removeKey string
		want      string
	}{
		{name: "missing provider mchid", removeKey: "sp_mchid", want: "sp_mchid"},
		{name: "missing sub mchid", removeKey: "sub_mchid", want: "sub_mchid"},
		{name: "missing out order no", removeKey: "out_order_no", want: "out_order_no"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := cloneStringAnyMap(base)
			delete(payload, tt.removeKey)
			envelope := ordinaryNotificationEnvelopeForTest(t, payload)

			resource, err := ordinaryProfitSharingResourceFromEnvelope(envelope)

			require.Nil(t, resource)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestValidateOrdinaryProfitSharingOwnershipRejectsMissingProviderIdentity(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ordinaryClient := mockordinaryserviceprovider.NewMockOrdinaryServiceProviderClientInterface(ctrl)
	server := &Server{ordinarySPClient: ordinaryClient}
	resource := &wechatcontracts.ProfitSharingNotification{SubMchID: "sub-mch-001", OutOrderNo: "profit-sharing-001"}

	err := server.validateOrdinaryProfitSharingOwnership(resource)

	require.Error(t, err)
	require.Contains(t, err.Error(), "sp_mchid missing")
}

func ordinaryNotificationEnvelopeForTest(t *testing.T, payload map[string]any) *ordinaryserviceprovider.NotificationEnvelope {
	t.Helper()
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	return &ordinaryserviceprovider.NotificationEnvelope{Plaintext: string(raw)}
}

func cloneStringAnyMap(input map[string]any) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		if nested, ok := value.(map[string]any); ok {
			output[key] = cloneStringAnyMap(nested)
			continue
		}
		output[key] = value
	}
	return output
}
