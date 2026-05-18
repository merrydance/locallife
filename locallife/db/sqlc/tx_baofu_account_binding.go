package db

import "context"

// MarkBaofuAccountBindingActiveWithFeeLedgerTxParams groups the active account
// transition with the platform-borne account-opening fee ledger row.
type MarkBaofuAccountBindingActiveWithFeeLedgerTxParams struct {
	ActiveBinding        MarkBaofuAccountBindingActiveParams
	AccountOpenFeeLedger CreateBaofuFeeLedgerParams
}

type MarkBaofuAccountBindingActiveWithFeeLedgerTxResult struct {
	Binding              BaofuAccountBinding
	AccountOpenFeeLedger BaofuFeeLedger
}

func (store *SQLStore) MarkBaofuAccountBindingActiveWithFeeLedgerTx(ctx context.Context, arg MarkBaofuAccountBindingActiveWithFeeLedgerTxParams) (MarkBaofuAccountBindingActiveWithFeeLedgerTxResult, error) {
	var result MarkBaofuAccountBindingActiveWithFeeLedgerTxResult
	err := store.execTx(ctx, func(q *Queries) error {
		binding, err := q.MarkBaofuAccountBindingActive(ctx, arg.ActiveBinding)
		if err != nil {
			return err
		}
		result.Binding = binding

		feeLedger, err := q.CreateBaofuFeeLedger(ctx, arg.AccountOpenFeeLedger)
		if err != nil {
			return err
		}
		result.AccountOpenFeeLedger = feeLedger
		return nil
	})
	return result, err
}
