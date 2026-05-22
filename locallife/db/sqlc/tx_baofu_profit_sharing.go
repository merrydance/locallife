package db

import (
	"context"
	"errors"
)

const (
	errBaofuProfitSharingRefundStarted = "订单已有退款申请或退款成功记录，不能继续发起宝付分账"
	errBaofuProfitSharingAlreadyExists = "订单已存在宝付分账单，不能重复发起分账"
)

// CreateBaofuProfitSharingOrderTxParams contains the durable share order and
// matching Baofu fee ledger rows that must commit atomically.
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
		if _, err := q.GetPaymentOrderForUpdate(ctx, arg.ProfitSharingOrder.PaymentOrderID); err != nil {
			return err
		}
		occupiedRefundAmount, err := q.GetTotalRefundedByPaymentOrder(ctx, arg.ProfitSharingOrder.PaymentOrderID)
		if err != nil {
			return err
		}
		if occupiedRefundAmount > 0 {
			return &requestError{statusCode: 400, err: errors.New(errBaofuProfitSharingRefundStarted)}
		}
		if _, err := q.GetProfitSharingOrderByPaymentOrder(ctx, arg.ProfitSharingOrder.PaymentOrderID); err == nil {
			return &requestError{statusCode: 400, err: errors.New(errBaofuProfitSharingAlreadyExists)}
		} else if !errors.Is(err, ErrRecordNotFound) {
			return err
		}

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
