package db

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
)

// CreateRefundOrderTxParams 创建退款单事务输入
type CreateRefundOrderTxParams struct {
	PaymentOrderID int64
	RefundType     string
	RefundAmount   int64
	RefundReason   string
	OutRefundNo    string
}

// CreateRefundOrderTxResult 创建退款单事务结果
type CreateRefundOrderTxResult struct {
	RefundOrder  RefundOrder
	PaymentOrder PaymentOrder
}

// CreateRefundOrderTx 以单一事务原子性地校验退款金额并创建退款单。
//
// 修复 #5（并发超退窗口）：
//   - 使用 GetPaymentOrderForUpdate 对支付单行加排他锁
//   - 在持锁状态下读取累计已退款额
//   - 校验通过后才创建退款单
//
// 任何并发的同支付单退款请求都会在行锁处串行化，消除竞态条件。
func (store *SQLStore) CreateRefundOrderTx(ctx context.Context, arg CreateRefundOrderTxParams) (CreateRefundOrderTxResult, error) {
	var result CreateRefundOrderTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		// 对支付单行加排他锁，串行化同一支付单的并发退款请求
		paymentOrder, err := q.GetPaymentOrderForUpdate(ctx, arg.PaymentOrderID)
		if err != nil {
			if errors.Is(err, ErrRecordNotFound) {
				return &requestError{statusCode: http.StatusNotFound, err: errors.New("payment order not found")}
			}
			return fmt.Errorf("lock payment order: %w", err)
		}
		result.PaymentOrder = paymentOrder

		if paymentOrder.Status != "paid" {
			return &requestError{statusCode: http.StatusBadRequest, err: errors.New("payment order is not paid")}
		}

		// 在持锁状态下统计已占用退款额度（pending/processing/success 都占用额度）
		alreadyRefunded, err := q.GetTotalRefundedByPaymentOrder(ctx, arg.PaymentOrderID)
		if err != nil {
			return fmt.Errorf("get total refunded: %w", err)
		}
		if alreadyRefunded+arg.RefundAmount > paymentOrder.Amount {
			return &requestError{
				statusCode: http.StatusBadRequest,
				err: fmt.Errorf("refund amount %d + already refunded %d exceeds payment amount %d",
					arg.RefundAmount, alreadyRefunded, paymentOrder.Amount),
			}
		}

		refundOrder, err := q.CreateRefundOrder(ctx, CreateRefundOrderParams{
			PaymentOrderID: arg.PaymentOrderID,
			RefundType:     arg.RefundType,
			RefundAmount:   arg.RefundAmount,
			RefundReason:   pgtype.Text{String: arg.RefundReason, Valid: arg.RefundReason != ""},
			OutRefundNo:    arg.OutRefundNo,
			Status:         "pending",
		})
		if err != nil {
			return fmt.Errorf("create refund order: %w", err)
		}
		result.RefundOrder = refundOrder
		return nil
	})

	return result, err
}

// requestError 是一个内部错误类型，携带 HTTP 状态码以便上层转换为 API 错误
type requestError struct {
	statusCode int
	err        error
}

func (e *requestError) Error() string { return e.err.Error() }
func (e *requestError) Unwrap() error { return e.err }

// IsRefundRequestError 判断事务错误是否为业务校验失败并返回 HTTP 状态码
func IsRefundRequestError(err error) (int, bool) {
	var re *requestError
	if errors.As(err, &re) {
		return re.statusCode, true
	}
	return 0, false
}
