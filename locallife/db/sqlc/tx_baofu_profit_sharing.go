package db

import "context"

// CreateBaofuProfitSharingOrderTxParams contains the durable share order and the
// matching merchant-borne payment fee ledger row that must commit atomically.
type CreateBaofuProfitSharingOrderTxParams struct {
	ProfitSharingOrder CreateProfitSharingOrderParams
	PaymentFeeLedger   CreateBaofuFeeLedgerParams
}

type CreateBaofuProfitSharingOrderTxResult struct {
	ProfitSharingOrder ProfitSharingOrder
	PaymentFeeLedger   BaofuFeeLedger
}

func (store *SQLStore) CreateBaofuProfitSharingOrderTx(ctx context.Context, arg CreateBaofuProfitSharingOrderTxParams) (CreateBaofuProfitSharingOrderTxResult, error) {
	var result CreateBaofuProfitSharingOrderTxResult
	err := store.execTx(ctx, func(q *Queries) error {
		profitSharingOrder, err := q.CreateProfitSharingOrder(ctx, arg.ProfitSharingOrder)
		if err != nil {
			return err
		}
		result.ProfitSharingOrder = profitSharingOrder

		feeLedger, err := q.CreateBaofuFeeLedger(ctx, arg.PaymentFeeLedger)
		if err != nil {
			return err
		}
		result.PaymentFeeLedger = feeLedger
		return nil
	})
	return result, err
}
