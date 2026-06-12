package db

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type CreateBaofuWithdrawalOrderWithSubmittedCommandTxParams struct {
	WithdrawalOrder CreateBaofuWithdrawalOrderParams
	BusinessOwner   string
	SubmittedAt     time.Time
}

type CreateBaofuWithdrawalOrderWithSubmittedCommandTxResult struct {
	WithdrawalOrder  BaofuWithdrawalOrder
	SubmittedCommand ExternalPaymentCommand
}

func (store *SQLStore) CreateBaofuWithdrawalOrderWithSubmittedCommandTx(ctx context.Context, arg CreateBaofuWithdrawalOrderWithSubmittedCommandTxParams) (CreateBaofuWithdrawalOrderWithSubmittedCommandTxResult, error) {
	var result CreateBaofuWithdrawalOrderWithSubmittedCommandTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		withdrawalOrder, err := q.CreateBaofuWithdrawalOrder(ctx, arg.WithdrawalOrder)
		if err != nil {
			return err
		}
		result.WithdrawalOrder = withdrawalOrder

		submittedAt := arg.SubmittedAt.UTC()
		if submittedAt.IsZero() {
			submittedAt = time.Now().UTC()
		}
		command, err := q.CreateExternalPaymentCommand(ctx, CreateExternalPaymentCommandParams{
			Provider:           ExternalPaymentProviderBaofu,
			Channel:            PaymentChannelBaofuAggregate,
			Capability:         ExternalPaymentCapabilityBaofuWithdraw,
			CommandType:        ExternalPaymentCommandTypeCreateBaofuWithdraw,
			BusinessOwner:      strings.TrimSpace(arg.BusinessOwner),
			BusinessObjectType: pgtype.Text{String: "baofu_withdrawal_order", Valid: true},
			BusinessObjectID:   pgtype.Int8{Int64: withdrawalOrder.ID, Valid: true},
			ExternalObjectType: ExternalPaymentObjectWithdraw,
			ExternalObjectKey:  strings.TrimSpace(withdrawalOrder.OutRequestNo),
			CommandStatus:      ExternalPaymentCommandStatusSubmitted,
			SubmittedAt:        submittedAt,
			ResponseSnapshot:   baofuWithdrawSubmittedCommandSnapshot(withdrawalOrder),
		})
		if err != nil {
			return err
		}
		result.SubmittedCommand = command
		return nil
	})

	return result, err
}

func baofuWithdrawSubmittedCommandSnapshot(order BaofuWithdrawalOrder) []byte {
	payload := map[string]any{
		"provider":                  ExternalPaymentProviderBaofu,
		"operation":                 "create_baofu_withdraw",
		"state":                     "submitted",
		"dispatch_mode":             "async_worker",
		"baofu_withdrawal_order_id": order.ID,
		"owner_type":                order.OwnerType,
		"owner_id":                  order.OwnerID,
		"out_request_no":            strings.TrimSpace(order.OutRequestNo),
		"amount":                    order.Amount,
		"status":                    order.Status,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return []byte(`{}`)
	}
	return data
}
