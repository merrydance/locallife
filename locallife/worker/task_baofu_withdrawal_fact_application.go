package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	baofucontracts "github.com/merrydance/locallife/baofu/account/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
)

const TaskProcessBaofuWithdrawalFactApplication = "baofu:process_withdrawal_fact_application"

type BaofuWithdrawalFactApplicationPayload struct {
	WithdrawalOrderID int64  `json:"withdrawal_order_id"`
	UpstreamState     string `json:"upstream_state"`
	BaofuWithdrawNo   string `json:"baofu_withdraw_no,omitempty"`
	RawSnapshot       []byte `json:"raw_snapshot,omitempty"`
}

func (processor *RedisTaskProcessor) ProcessTaskBaofuWithdrawalFactApplication(ctx context.Context, task *asynq.Task) error {
	var payload BaofuWithdrawalFactApplicationPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal baofu withdrawal fact application payload: %w", err)
	}
	if payload.WithdrawalOrderID <= 0 {
		return fmt.Errorf("baofu withdrawal order id is required")
	}
	withdrawalOrder, err := processor.store.GetBaofuWithdrawalOrder(ctx, payload.WithdrawalOrderID)
	if err != nil {
		return fmt.Errorf("get baofu withdrawal order: %w", err)
	}
	status := baofucontracts.WithdrawStatusFromUpstream(payload.UpstreamState)
	raw := payload.RawSnapshot
	if len(raw) == 0 || !json.Valid(raw) {
		raw = []byte(`{}`)
	}
	baofuWithdrawNo := strings.TrimSpace(payload.BaofuWithdrawNo)
	if baofuWithdrawNo == "" && withdrawalOrder.BaofuWithdrawNo.Valid {
		baofuWithdrawNo = withdrawalOrder.BaofuWithdrawNo.String
	}
	if _, err := processor.store.UpdateBaofuWithdrawalOrderStatus(ctx, db.UpdateBaofuWithdrawalOrderStatusParams{
		ID:     withdrawalOrder.ID,
		Status: status,
		BaofuWithdrawNo: pgtype.Text{
			String: baofuWithdrawNo,
			Valid:  baofuWithdrawNo != "",
		},
		RawSnapshot: raw,
	}); err != nil {
		return fmt.Errorf("update baofu withdrawal order status: %w", err)
	}
	return nil
}
