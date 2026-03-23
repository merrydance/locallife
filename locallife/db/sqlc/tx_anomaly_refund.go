package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

// CreateAnomalyRefundRecordParams 异常退款记录创建参数
type CreateAnomalyRefundRecordParams struct {
	PaymentOrderID int64
	RefundAmount   int64
	// OutRefundNo 由调用方保证幂等性（使用支付单 ID 派生的 "CRF{id}"）
	OutRefundNo string
}

// CreateAnomalyRefundRecord 为已关闭/失败状态的支付单创建异常退款记录。
//
// 与 CreateRefundOrderTx 不同，此函数跳过 status='paid' 校验，
// 专用于"已关闭/失败订单收到微信付款"这一竞态场景。
// 幂等性由 out_refund_no UNIQUE 约束保证：INSERT 冲突时返回已有记录。
func (store *SQLStore) CreateAnomalyRefundRecord(ctx context.Context, arg CreateAnomalyRefundRecordParams) (RefundOrder, error) {
	refundOrder, err := store.CreateRefundOrder(ctx, CreateRefundOrderParams{
		PaymentOrderID: arg.PaymentOrderID,
		RefundType:     "closed_order_anomaly",
		RefundAmount:   arg.RefundAmount,
		RefundReason:   pgtype.Text{String: "已关闭订单异常到账，系统自动退款", Valid: true},
		OutRefundNo:    arg.OutRefundNo,
		Status:         "pending",
	})
	if err != nil {
		// UNIQUE 约束冲突 → 已存在，直接返回
		if ErrorCode(err) == UniqueViolation {
			existing, lookupErr := store.GetRefundOrderByOutRefundNo(ctx, arg.OutRefundNo)
			if lookupErr != nil {
				return RefundOrder{}, fmt.Errorf("lookup existing anomaly refund after conflict: %w", lookupErr)
			}
			return existing, nil
		}
		return RefundOrder{}, fmt.Errorf("create anomaly refund record: %w", err)
	}
	return refundOrder, nil
}
