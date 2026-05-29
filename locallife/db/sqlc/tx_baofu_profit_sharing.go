package db

import (
	"context"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"
)

const (
	errBaofuProfitSharingRefundStarted    = "订单已有未完成退款申请，不能继续发起宝付分账"
	errBaofuProfitSharingAlreadyExists    = "订单已存在宝付分账单，不能重复发起分账"
	errBaofuProfitSharingBillConflict     = "订单已存在不同的宝付分账账单"
	errBaofuProfitSharingNotReady         = "宝付分账单当前状态不允许发起分账"
	errBaofuProfitSharingBillAmountStale  = "宝付分账账单金额与退款后净额不一致"
	errBaofuProfitSharingNetAmountInvalid = "订单退款后净额不能继续发起宝付分账"
	errBaofuProfitSharingRefundScope      = "宝付分账成功退款净额口径仅适用于预订支付单"
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

type PrepareBaofuProfitSharingCommandTxParams struct {
	ProfitSharingOrderID int64
}

type PrepareBaofuProfitSharingCommandTxResult struct {
	ProfitSharingOrder ProfitSharingOrder
	PaymentOrder       PaymentOrder
}

func (store *SQLStore) CreateBaofuProfitSharingOrderTx(ctx context.Context, arg CreateBaofuProfitSharingOrderTxParams) (CreateBaofuProfitSharingOrderTxResult, error) {
	var result CreateBaofuProfitSharingOrderTxResult
	err := store.execTx(ctx, func(q *Queries) error {
		paymentOrder, err := q.GetPaymentOrderForUpdate(ctx, arg.ProfitSharingOrder.PaymentOrderID)
		if err != nil {
			return err
		}
		occupiedRefundAmount, err := q.GetTotalActiveRefundedByPaymentOrder(ctx, arg.ProfitSharingOrder.PaymentOrderID)
		if err != nil {
			return err
		}
		if occupiedRefundAmount > 0 {
			return &requestError{statusCode: 400, err: errors.New(errBaofuProfitSharingRefundStarted)}
		}
		successRefundAmount, err := q.GetTotalSuccessfulRefundedByPaymentOrder(ctx, arg.ProfitSharingOrder.PaymentOrderID)
		if err != nil {
			return err
		}
		if successRefundAmount > 0 && !baofuPaymentOrderAllowsSuccessfulRefundNetShare(paymentOrder) {
			return &requestError{statusCode: 400, err: errors.New(errBaofuProfitSharingRefundScope)}
		}
		if paymentOrder.Amount-successRefundAmount <= 0 {
			return &requestError{statusCode: 400, err: errors.New(errBaofuProfitSharingNetAmountInvalid)}
		}
		if arg.ProfitSharingOrder.TotalAmount != paymentOrder.Amount-successRefundAmount {
			return &requestError{statusCode: 409, err: errors.New(errBaofuProfitSharingBillAmountStale)}
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
		paymentOrder, err := q.GetPaymentOrderForUpdate(ctx, arg.ProfitSharingOrder.PaymentOrderID)
		if err != nil {
			return err
		}
		occupiedRefundAmount, err := q.GetTotalActiveRefundedByPaymentOrder(ctx, arg.ProfitSharingOrder.PaymentOrderID)
		if err != nil {
			return err
		}
		if occupiedRefundAmount > 0 {
			return &requestError{statusCode: 400, err: errors.New(errBaofuProfitSharingRefundStarted)}
		}
		successRefundAmount, err := q.GetTotalSuccessfulRefundedByPaymentOrder(ctx, arg.ProfitSharingOrder.PaymentOrderID)
		if err != nil {
			return err
		}
		if successRefundAmount > 0 && !baofuPaymentOrderAllowsSuccessfulRefundNetShare(paymentOrder) {
			return &requestError{statusCode: 400, err: errors.New(errBaofuProfitSharingRefundScope)}
		}
		if paymentOrder.Amount-successRefundAmount <= 0 {
			return &requestError{statusCode: 400, err: errors.New(errBaofuProfitSharingNetAmountInvalid)}
		}
		if arg.ProfitSharingOrder.TotalAmount != paymentOrder.Amount-successRefundAmount {
			return &requestError{statusCode: 409, err: errors.New(errBaofuProfitSharingBillAmountStale)}
		}

		existing, err := q.GetProfitSharingOrderByPaymentOrder(ctx, arg.ProfitSharingOrder.PaymentOrderID)
		if err == nil {
			if !baofuProfitSharingBillMatches(existing, arg) {
				if existing.Provider != ExternalPaymentProviderBaofu ||
					existing.Channel != PaymentChannelBaofuAggregate ||
					(existing.Status != ProfitSharingOrderStatusPending && existing.Status != ProfitSharingOrderStatusFailed) ||
					arg.FeeBreakdown.CalculationVersion == "" {
					return &requestError{statusCode: 409, err: errors.New(errBaofuProfitSharingBillConflict)}
				}
				refreshed, refreshErr := refreshBaofuProfitSharingBillWithLedgers(ctx, q, existing.ID, arg)
				if refreshErr != nil {
					return refreshErr
				}
				result = refreshed
				return nil
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

func (store *SQLStore) PrepareBaofuProfitSharingCommandTx(ctx context.Context, arg PrepareBaofuProfitSharingCommandTxParams) (PrepareBaofuProfitSharingCommandTxResult, error) {
	var result PrepareBaofuProfitSharingCommandTxResult
	err := store.execTx(ctx, func(q *Queries) error {
		profitSharingOrder, err := q.GetProfitSharingOrderForUpdate(ctx, arg.ProfitSharingOrderID)
		if err != nil {
			return err
		}
		if profitSharingOrder.Provider != ExternalPaymentProviderBaofu ||
			profitSharingOrder.Channel != PaymentChannelBaofuAggregate ||
			(profitSharingOrder.Status != ProfitSharingOrderStatusPending && profitSharingOrder.Status != ProfitSharingOrderStatusFailed) {
			return &requestError{statusCode: 400, err: errors.New(errBaofuProfitSharingNotReady)}
		}

		paymentOrder, err := q.GetPaymentOrderForUpdate(ctx, profitSharingOrder.PaymentOrderID)
		if err != nil {
			return err
		}
		if paymentOrder.Status != "paid" ||
			paymentOrder.PaymentChannel != PaymentChannelBaofuAggregate ||
			!paymentOrder.RequiresProfitSharing {
			return &requestError{statusCode: 400, err: errors.New(errBaofuProfitSharingNotReady)}
		}

		occupiedRefundAmount, err := q.GetTotalActiveRefundedByPaymentOrder(ctx, paymentOrder.ID)
		if err != nil {
			return err
		}
		if occupiedRefundAmount > 0 {
			return &requestError{statusCode: 400, err: errors.New(errBaofuProfitSharingRefundStarted)}
		}
		successRefundAmount, err := q.GetTotalSuccessfulRefundedByPaymentOrder(ctx, paymentOrder.ID)
		if err != nil {
			return err
		}
		netAmount := paymentOrder.Amount - successRefundAmount
		if successRefundAmount > 0 && !baofuPaymentOrderAllowsSuccessfulRefundNetShare(paymentOrder) {
			return &requestError{statusCode: 400, err: errors.New(errBaofuProfitSharingRefundScope)}
		}
		if netAmount <= 0 {
			return &requestError{statusCode: 400, err: errors.New(errBaofuProfitSharingNetAmountInvalid)}
		}
		if profitSharingOrder.TotalAmount != netAmount {
			return &requestError{statusCode: 409, err: errors.New(errBaofuProfitSharingBillAmountStale)}
		}

		profitSharingOrder, err = q.UpdateProfitSharingOrderToProcessing(ctx, UpdateProfitSharingOrderToProcessingParams{
			ID:             profitSharingOrder.ID,
			SharingOrderID: profitSharingOrder.SharingOrderID,
		})
		if err != nil {
			return err
		}
		result.ProfitSharingOrder = profitSharingOrder
		result.PaymentOrder = paymentOrder
		return nil
	})
	return result, err
}

func baofuPaymentOrderAllowsSuccessfulRefundNetShare(paymentOrder PaymentOrder) bool {
	return paymentOrder.BusinessType == ExternalPaymentBusinessOwnerReservation ||
		paymentOrder.BusinessType == "reservation_addon"
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

func refreshBaofuProfitSharingBillWithLedgers(ctx context.Context, q *Queries, existingID int64, arg CreateBaofuProfitSharingOrderTxParams) (CreateBaofuProfitSharingOrderTxResult, error) {
	var result CreateBaofuProfitSharingOrderTxResult
	refreshArg := UpdateBaofuPendingProfitSharingBillSnapshotParams{
		ID:                           existingID,
		TotalAmount:                  arg.ProfitSharingOrder.TotalAmount,
		DeliveryFee:                  arg.ProfitSharingOrder.DeliveryFee,
		RiderID:                      arg.ProfitSharingOrder.RiderID,
		RiderAmount:                  arg.ProfitSharingOrder.RiderAmount,
		DistributableAmount:          arg.ProfitSharingOrder.DistributableAmount,
		PlatformRate:                 arg.ProfitSharingOrder.PlatformRate,
		OperatorRate:                 arg.ProfitSharingOrder.OperatorRate,
		PlatformCommission:           arg.ProfitSharingOrder.PlatformCommission,
		OperatorCommission:           arg.ProfitSharingOrder.OperatorCommission,
		MerchantAmount:               arg.ProfitSharingOrder.MerchantAmount,
		PaymentFee:                   arg.ProfitSharingOrder.PaymentFee,
		PaymentFeeRateBps:            arg.ProfitSharingOrder.PaymentFeeRateBps,
		MerchantSharingMerID:         arg.ProfitSharingOrder.MerchantSharingMerID,
		RiderSharingMerID:            arg.ProfitSharingOrder.RiderSharingMerID,
		OperatorSharingMerID:         arg.ProfitSharingOrder.OperatorSharingMerID,
		PlatformSharingMerID:         arg.ProfitSharingOrder.PlatformSharingMerID,
		SharingDetailSnapshot:        bytesSQLArgOrDefault(arg.ProfitSharingOrder.SharingDetailSnapshot, []byte(`{}`)),
		CalculationVersion:           arg.FeeBreakdown.CalculationVersion,
		SettlementMode:               arg.FeeBreakdown.SettlementMode,
		ProviderPaymentFee:           arg.FeeBreakdown.ProviderPaymentFee,
		ProviderPaymentFeeRateBps:    arg.FeeBreakdown.ProviderPaymentFeeRateBps,
		ProviderPaymentFeeBaseAmount: arg.FeeBreakdown.ProviderPaymentFeeBaseAmount,
		ProviderPaymentFeeSource:     arg.FeeBreakdown.ProviderPaymentFeeSource,
		MerchantPaymentFee:           arg.FeeBreakdown.MerchantPaymentFee,
		MerchantPaymentFeeRateBps:    arg.FeeBreakdown.MerchantPaymentFeeRateBps,
		MerchantPaymentFeeBaseAmount: arg.FeeBreakdown.MerchantPaymentFeeBaseAmount,
		RiderGrossAmount:             arg.FeeBreakdown.RiderGrossAmount,
		RiderPaymentFee:              arg.FeeBreakdown.RiderPaymentFee,
		RiderPaymentFeeRateBps:       arg.FeeBreakdown.RiderPaymentFeeRateBps,
		RiderPaymentFeeBaseAmount:    arg.FeeBreakdown.RiderPaymentFeeBaseAmount,
		CommissionBaseAmount:         arg.FeeBreakdown.CommissionBaseAmount,
		PlatformReceiverAmount:       arg.FeeBreakdown.PlatformReceiverAmount,
	}
	profitSharingOrder, err := q.UpdateBaofuPendingProfitSharingBillSnapshot(ctx, refreshArg)
	if err != nil {
		return result, err
	}
	result.ProfitSharingOrder = profitSharingOrder

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
		!pgtypeTextEqual(existing.MerchantSharingMerID, expected.MerchantSharingMerID) ||
		!pgtypeTextEqual(existing.OperatorSharingMerID, expected.OperatorSharingMerID) ||
		!pgtypeTextEqual(existing.PlatformSharingMerID, expected.PlatformSharingMerID) {
		return false
	}
	riderAssignedAfterBillCreation := !expected.RiderID.Valid && existing.RiderID.Valid
	if !riderAssignedAfterBillCreation {
		if existing.RiderAmount != expected.RiderAmount ||
			!pgtypeInt8Equal(existing.RiderID, expected.RiderID) ||
			!pgtypeTextEqual(existing.RiderSharingMerID, expected.RiderSharingMerID) {
			return false
		}
	}
	if arg.FeeBreakdown.CalculationVersion == "" {
		return true
	}
	baseFeeBreakdownMatches := existing.CalculationVersion == arg.FeeBreakdown.CalculationVersion &&
		existing.SettlementMode == arg.FeeBreakdown.SettlementMode &&
		existing.ProviderPaymentFee == arg.FeeBreakdown.ProviderPaymentFee &&
		existing.ProviderPaymentFeeRateBps == arg.FeeBreakdown.ProviderPaymentFeeRateBps &&
		existing.ProviderPaymentFeeBaseAmount == arg.FeeBreakdown.ProviderPaymentFeeBaseAmount &&
		existing.ProviderPaymentFeeSource == arg.FeeBreakdown.ProviderPaymentFeeSource &&
		existing.MerchantPaymentFee == arg.FeeBreakdown.MerchantPaymentFee &&
		existing.MerchantPaymentFeeRateBps == arg.FeeBreakdown.MerchantPaymentFeeRateBps &&
		existing.MerchantPaymentFeeBaseAmount == arg.FeeBreakdown.MerchantPaymentFeeBaseAmount &&
		existing.CommissionBaseAmount == arg.FeeBreakdown.CommissionBaseAmount
	if !baseFeeBreakdownMatches {
		return false
	}
	if riderAssignedAfterBillCreation {
		return existing.RiderGrossAmount == arg.FeeBreakdown.RiderGrossAmount &&
			existing.RiderPaymentFeeRateBps == arg.FeeBreakdown.RiderPaymentFeeRateBps
	}
	return existing.RiderGrossAmount == arg.FeeBreakdown.RiderGrossAmount &&
		existing.RiderPaymentFee == arg.FeeBreakdown.RiderPaymentFee &&
		existing.RiderPaymentFeeRateBps == arg.FeeBreakdown.RiderPaymentFeeRateBps &&
		existing.RiderPaymentFeeBaseAmount == arg.FeeBreakdown.RiderPaymentFeeBaseAmount &&
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

func bytesSQLArgOrDefault(value interface{}, fallback []byte) []byte {
	if value == nil {
		return fallback
	}
	switch v := value.(type) {
	case []byte:
		return v
	case string:
		return []byte(v)
	default:
		return fallback
	}
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
