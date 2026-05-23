package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

const (
	orderPaymentFactBusinessObjectOrder = "payment_order"
	orderPaymentFactConsumerDomain      = "order_domain"
)

func enqueueOrderPaymentFactApplication(ctx context.Context, distributor any, application *db.ExternalPaymentFactApplication) error {
	if application == nil {
		return nil
	}
	applicationDistributor, ok := distributor.(PaymentFactApplicationTaskDistributor)
	if !ok {
		return fmt.Errorf("payment fact application distributor not configured")
	}
	return applicationDistributor.DistributeTaskProcessPaymentFactApplication(
		ctx,
		&PaymentFactApplicationPayload{ApplicationID: application.ID},
		asynq.MaxRetry(5),
		asynq.Queue(QueueCritical),
		asynq.Unique(paymentFactApplicationTaskUnique),
	)
}

func recoveredOrderPaymentFactResource(order db.PaymentOrder) []byte {
	raw, err := json.Marshal(map[string]any{
		"payment_order_id":    order.ID,
		"out_trade_no":        order.OutTradeNo,
		"transaction_id":      orderPaymentTextValue(order.TransactionID),
		"business_type":       order.BusinessType,
		"payment_channel":     order.PaymentChannel,
		"combined_payment_id": orderPaymentInt8Value(order.CombinedPaymentID),
		"recovery_reason":     "paid_unprocessed_scan",
	})
	if err != nil {
		return nil
	}
	return raw
}

func orderPaymentStringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func orderPaymentOptionalStringPtr(value pgtype.Text) *string {
	if !value.Valid || value.String == "" {
		return nil
	}
	return &value.String
}

func orderPaymentInt64Ptr(value int64) *int64 {
	return &value
}

func orderPaymentParseFactTime(value string) *time.Time {
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil
	}
	parsed = parsed.UTC()
	return &parsed
}

func orderPaymentTextValue(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func orderPaymentInt8Value(value pgtype.Int8) any {
	if !value.Valid {
		return nil
	}
	return value.Int64
}
