package db

import "context"

// CreateBaofuProfitSharingOrderTxParams contains the durable share order and the
// matching merchant-borne payment fee ledger row that must commit atomically.
type CreateBaofuProfitSharingOrderTxParams struct {
	ProfitSharingOrder     CreateProfitSharingOrderParams
	FeeBreakdown           UpdateProfitSharingOrderFeeBreakdownParams
	PaymentFeeLedger       CreateBaofuFeeLedgerParams
	OrderPaymentFeeLedgers []CreateOrderPaymentFeeLedgerParams
}

type CreateBaofuProfitSharingOrderTxResult struct {
	ProfitSharingOrder     ProfitSharingOrder
	PaymentFeeLedger       BaofuFeeLedger
	OrderPaymentFeeLedgers []OrderPaymentFeeLedger
}

func (store *SQLStore) CreateBaofuProfitSharingOrderTx(ctx context.Context, arg CreateBaofuProfitSharingOrderTxParams) (CreateBaofuProfitSharingOrderTxResult, error) {
	var result CreateBaofuProfitSharingOrderTxResult
	err := store.execTx(ctx, func(q *Queries) error {
		profitSharingOrder, err := q.CreateProfitSharingOrder(ctx, arg.ProfitSharingOrder)
		if err != nil {
			return err
		}
		result.ProfitSharingOrder = profitSharingOrder
		if arg.FeeBreakdown.CalculationVersion != "" {
			arg.FeeBreakdown.ID = profitSharingOrder.ID
			profitSharingOrder, err = q.UpdateProfitSharingOrderFeeBreakdown(ctx, arg.FeeBreakdown)
			if err != nil {
				return err
			}
			result.ProfitSharingOrder = profitSharingOrder
		}

		feeLedger, err := q.CreateBaofuFeeLedger(ctx, arg.PaymentFeeLedger)
		if err != nil {
			return err
		}
		result.PaymentFeeLedger = feeLedger

		for _, ledgerArg := range arg.OrderPaymentFeeLedgers {
			ledgerArg.PaymentOrderID = profitSharingOrder.PaymentOrderID
			ledgerArg.ProfitSharingOrderID.Int64 = profitSharingOrder.ID
			ledgerArg.ProfitSharingOrderID.Valid = true
			ledger, err := q.UpsertOrderPaymentFeeLedgerCalculated(ctx, orderPaymentFeeLedgerCalculatedParams(ledgerArg))
			if err != nil {
				return err
			}
			result.OrderPaymentFeeLedgers = append(result.OrderPaymentFeeLedgers, ledger)
		}
		return nil
	})
	return result, err
}

func orderPaymentFeeLedgerCalculatedParams(arg CreateOrderPaymentFeeLedgerParams) UpsertOrderPaymentFeeLedgerCalculatedParams {
	return UpsertOrderPaymentFeeLedgerCalculatedParams{
		Provider:              arg.Provider,
		Channel:               arg.Channel,
		PaymentOrderID:        arg.PaymentOrderID,
		ProfitSharingOrderID:  arg.ProfitSharingOrderID,
		FeeType:               arg.FeeType,
		PayerType:             arg.PayerType,
		PayerID:               arg.PayerID,
		PayeeType:             arg.PayeeType,
		BaseAmount:            arg.BaseAmount,
		RateBps:               arg.RateBps,
		Amount:                arg.Amount,
		AmountSource:          arg.AmountSource,
		ExternalPaymentFactID: arg.ExternalPaymentFactID,
		Status:                arg.Status,
		CalculationVersion:    arg.CalculationVersion,
	}
}
