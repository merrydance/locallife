package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type EcommerceApplymentPendingFollowUp struct {
	ID                       int64              `json:"id"`
	SubjectType              string             `json:"subject_type"`
	SubjectID                int64              `json:"subject_id"`
	OutRequestNo             string             `json:"out_request_no"`
	ApplymentID              pgtype.Int8        `json:"applyment_id"`
	Status                   string             `json:"status"`
	SignUrl                  pgtype.Text        `json:"sign_url"`
	SignState                pgtype.Text        `json:"sign_state"`
	RejectReason             pgtype.Text        `json:"reject_reason"`
	SubMchID                 pgtype.Text        `json:"sub_mch_id"`
	UpdatedAt                time.Time          `json:"updated_at"`
	ResultTaskProcessedState pgtype.Text        `json:"result_task_processed_state"`
	ResultTaskProcessedAt    pgtype.Timestamptz `json:"result_task_processed_at"`
}

type ListEcommerceApplymentsPendingFollowUpParams struct {
	UpdatedBefore time.Time `json:"updated_before"`
	Limit         int32     `json:"limit"`
}

func (store *SQLStore) ListEcommerceApplymentsPendingFollowUp(ctx context.Context, arg ListEcommerceApplymentsPendingFollowUpParams) ([]EcommerceApplymentPendingFollowUp, error) {
	rows, err := store.connPool.Query(ctx, `
		SELECT id, subject_type, subject_id, out_request_no, applyment_id, status, sign_url, sign_state,
		       reject_reason, sub_mch_id, updated_at, result_task_processed_state, result_task_processed_at
		FROM ecommerce_applyments
		WHERE updated_at <= $1
		  AND subject_type = 'merchant'
		  AND status IN ('submitted', 'checking', 'auditing', 'account_need_verify', 'to_be_signed', 'to_be_confirmed', 'signing', 'finish', 'rejected')
		ORDER BY updated_at ASC, id ASC
		LIMIT $2`,
		arg.UpdatedBefore,
		arg.Limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]EcommerceApplymentPendingFollowUp, 0)
	for rows.Next() {
		var item EcommerceApplymentPendingFollowUp
		if err := rows.Scan(
			&item.ID,
			&item.SubjectType,
			&item.SubjectID,
			&item.OutRequestNo,
			&item.ApplymentID,
			&item.Status,
			&item.SignUrl,
			&item.SignState,
			&item.RejectReason,
			&item.SubMchID,
			&item.UpdatedAt,
			&item.ResultTaskProcessedState,
			&item.ResultTaskProcessedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

type MarkEcommerceApplymentResultProcessedParams struct {
	ID                       int64       `json:"id"`
	ResultTaskProcessedState pgtype.Text `json:"result_task_processed_state"`
}

func (store *SQLStore) MarkEcommerceApplymentResultProcessed(ctx context.Context, arg MarkEcommerceApplymentResultProcessedParams) error {
	_, err := store.connPool.Exec(ctx, `
		UPDATE ecommerce_applyments
		SET result_task_processed_state = $2,
		    result_task_processed_at = NOW()
		WHERE id = $1`,
		arg.ID,
		arg.ResultTaskProcessedState,
	)
	return err
}
