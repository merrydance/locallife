package db

import "context"

// CloseCombinedPaymentOrderTxParams 合单支付关闭事务参数
type CloseCombinedPaymentOrderTxParams struct {
	CombinedPaymentOrderID int64
	SubOrderOutTradeNos    []string
}

// CloseCombinedPaymentOrderTxResult 合单支付关闭事务结果
type CloseCombinedPaymentOrderTxResult struct {
	CombinedPaymentOrder CombinedPaymentOrder
}

// CloseCombinedPaymentOrderTx 在单个数据库事务中原子关闭合单支付订单及其所有子支付单。
func (store *SQLStore) CloseCombinedPaymentOrderTx(ctx context.Context, arg CloseCombinedPaymentOrderTxParams) (CloseCombinedPaymentOrderTxResult, error) {
	var result CloseCombinedPaymentOrderTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error
		result.CombinedPaymentOrder, err = q.UpdateCombinedPaymentOrderToClosed(ctx, arg.CombinedPaymentOrderID)
		if err != nil {
			return err
		}

		for _, outTradeNo := range arg.SubOrderOutTradeNos {
			paymentOrder, err := q.GetPaymentOrderByOutTradeNo(ctx, outTradeNo)
			if err != nil {
				continue // 子单未找到则跳过
			}
			if paymentOrder.Status == "pending" {
				if _, err := q.UpdatePaymentOrderToClosed(ctx, paymentOrder.ID); err != nil {
					return err
				}
			}
		}

		return nil
	})

	return result, err
}
