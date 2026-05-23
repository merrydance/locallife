package db

import (
	"context"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"
)

const (
	errBaofuProfitSharingRefundStarted = "订单已有退款申请或退款成功记录，不能继续发起宝付分账"
	errBaofuProfitSharingAlreadyExists = "订单已存在宝付分账单，不能重复发起分账"
	errBaofuProfitSharingBillConflict  = "订单已存在不同的宝付分账账单"
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

func (store *SQLStore) EnsureBaofuProfitSharingBillTx(ctx context.Context, arg CreateBaofuProfitSharingOrderTxParams) (CreateBaofuProfitSharingOrderTxResult, error) {
	var result CreateBaofuProfitSharingOrderTxResult
	err := store.execTx(ctx, func(q *Queries) error {
		if _, err := q.GetPaymentOrderForUpdate(ctx, arg.ProfitSharingOrder.PaymentOrderID); err != nil {
			return err
		}

		existing, err := q.GetProfitSharingOrderByPaymentOrder(ctx, arg.ProfitSharingOrder.PaymentOrderID)
		if err == nil {
			if !baofuProfitSharingBillMatches(existing, arg) {
				return &requestError{statusCode: 409, err: errors.New(errBaofuProfitSharingBillConflict)}
			}
			result.ProfitSharingOrder = existing
			return nil
		}
		if !errors.Is(err, ErrRecordNotFound) {
			return err
		}

		created, err := createBaofuProfitSharingOrderWithLedgers(ctx, q, arg)
		if err != nil {
			return err
		}
		result = created
		return nil
	})
	return result, err
}

func createBaofuProfitSharingOrderWithLedgers(ctx context.Context, q *Queries, arg CreateBaofuProfitSharingOrderTxParams) (CreateBaofuProfitSharingOrderTxResult, error) {
	var result CreateBaofuProfitSharingOrderTxResult
	profitSharingOrder, err := q.CreateProfitSharingOrder(ctx, arg.ProfitSharingOrder)
	if err != nil {
		return result, err
	}
	result.ProfitSharingOrder = profitSharingOrder
	if arg.FeeBreakdown.CalculationVersion != "" {
		arg.FeeBreakdown.ID = profitSharingOrder.ID
		profitSharingOrder, err = q.UpdateProfitSharingOrderFeeBreakdown(ctx, arg.FeeBreakdown)
		if err != nil {
			return result, err
		}
		result.ProfitSharingOrder = profitSharingOrder
	}

	feeLedger, err := q.CreateBaofuFeeLedger(ctx, arg.PaymentFeeLedger)
	if err != nil {
		return result, err
	}
	result.PaymentFeeLedger = feeLedger

	for _, ledgerArg := range arg.OrderPaymentFeeLedgers {
		ledgerArg.PaymentOrderID = profitSharingOrder.PaymentOrderID
		ledgerArg.ProfitSharingOrderID.Int64 = profitSharingOrder.ID
		ledgerArg.ProfitSharingOrderID.Valid = true
		ledger, err := q.UpsertOrderPaymentFeeLedgerCalculated(ctx, orderPaymentFeeLedgerCalculatedParams(ledgerArg))
		if err != nil {
			return result, err
		}
		result.OrderPaymentFeeLedgers = append(result.OrderPaymentFeeLedgers, ledger)
	}
	return result, nil
}

func baofuProfitSharingBillMatches(existing ProfitSharingOrder, arg CreateBaofuProfitSharingOrderTxParams) bool {
	expected := arg.ProfitSharingOrder
	if existing.PaymentOrderID != expected.PaymentOrderID ||
		existing.MerchantID != expected.MerchantID ||
		existing.OrderSource != expected.OrderSource ||
		existing.TotalAmount != expected.TotalAmount ||
		existing.DeliveryFee != expected.DeliveryFee ||
		existing.RiderAmount != expected.RiderAmount ||
		existing.DistributableAmount != expected.DistributableAmount ||
		existing.PlatformRate != expected.PlatformRate ||
		existing.OperatorRate != expected.OperatorRate ||
		existing.PlatformCommission != expected.PlatformCommission ||
		existing.OperatorCommission != expected.OperatorCommission ||
		existing.MerchantAmount != expected.MerchantAmount ||
		existing.OutOrderNo != expected.OutOrderNo ||
		existing.PaymentFee != int64SQLArgOrDefault(expected.PaymentFee, 0) ||
		existing.PaymentFeeRateBps != int32SQLArgOrDefault(expected.PaymentFeeRateBps, 30) ||
		existing.Provider != stringSQLArgOrDefault(expected.Provider, ExternalPaymentProviderWechat) ||
		existing.Channel != stringSQLArgOrDefault(expected.Channel, PaymentChannelBaofuAggregate) {
		return false
	}
	if !pgtypeInt8Equal(existing.OperatorID, expected.OperatorID) ||
		!pgtypeInt8Equal(existing.RiderID, expected.RiderID) ||
		!pgtypeTextEqual(existing.MerchantSharingMerID, expected.MerchantSharingMerID) ||
		!pgtypeTextEqual(existing.RiderSharingMerID, expected.RiderSharingMerID) ||
		!pgtypeTextEqual(existing.OperatorSharingMerID, expected.OperatorSharingMerID) ||
		!pgtypeTextEqual(existing.PlatformSharingMerID, expected.PlatformSharingMerID) {
		return false
	}
	if arg.FeeBreakdown.CalculationVersion == "" {
		return true
	}
	return existing.CalculationVersion == arg.FeeBreakdown.CalculationVersion &&
		existing.SettlementMode == arg.FeeBreakdown.SettlementMode &&
		existing.ProviderPaymentFee == arg.FeeBreakdown.ProviderPaymentFee &&
		existing.ProviderPaymentFeeRateBps == arg.FeeBreakdown.ProviderPaymentFeeRateBps &&
		existing.ProviderPaymentFeeBaseAmount == arg.FeeBreakdown.ProviderPaymentFeeBaseAmount &&
		existing.ProviderPaymentFeeSource == arg.FeeBreakdown.ProviderPaymentFeeSource &&
		existing.MerchantPaymentFee == arg.FeeBreakdown.MerchantPaymentFee &&
		existing.MerchantPaymentFeeRateBps == arg.FeeBreakdown.MerchantPaymentFeeRateBps &&
		existing.MerchantPaymentFeeBaseAmount == arg.FeeBreakdown.MerchantPaymentFeeBaseAmount &&
		existing.RiderGrossAmount == arg.FeeBreakdown.RiderGrossAmount &&
		existing.RiderPaymentFee == arg.FeeBreakdown.RiderPaymentFee &&
		existing.RiderPaymentFeeRateBps == arg.FeeBreakdown.RiderPaymentFeeRateBps &&
		existing.RiderPaymentFeeBaseAmount == arg.FeeBreakdown.RiderPaymentFeeBaseAmount &&
		existing.CommissionBaseAmount == arg.FeeBreakdown.CommissionBaseAmount &&
		existing.PlatformReceiverAmount == arg.FeeBreakdown.PlatformReceiverAmount
}

func pgtypeInt8Equal(a, b pgtype.Int8) bool {
	if a.Valid != b.Valid {
		return false
	}
	return !a.Valid || a.Int64 == b.Int64
}

func pgtypeTextEqual(a, b pgtype.Text) bool {
	if a.Valid != b.Valid {
		return false
	}
	return !a.Valid || a.String == b.String
}

func int64SQLArgOrDefault(value interface{}, fallback int64) int64 {
	switch v := value.(type) {
	case nil:
		return fallback
	case int:
		return int64(v)
	case int8:
		return int64(v)
	case int16:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return v
	case uint:
		return int64(v)
	case uint8:
		return int64(v)
	case uint16:
		return int64(v)
	case uint32:
		return int64(v)
	case uint64:
		if v > uint64(^uint64(0)>>1) {
			return fallback
		}
		return int64(v)
	case string:
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fallback
		}
		return parsed
	default:
		return fallback
	}
}

func int32SQLArgOrDefault(value interface{}, fallback int32) int32 {
	normalized := int64SQLArgOrDefault(value, int64(fallback))
	if normalized < -2147483648 || normalized > 2147483647 {
		return fallback
	}
	return int32(normalized)
}

func stringSQLArgOrDefault(value interface{}, fallback string) string {
	if value == nil {
		return fallback
	}
	if s, ok := value.(string); ok {
		return s
	}
	return fallback
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
