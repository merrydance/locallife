package db

import (
	"context"
	"errors"
)

type MarkMerchantBaofuAccountOpeningReadyTxParams struct {
	PaymentConfig UpsertMerchantPaymentConfigParams
	Flow          MarkBaofuAccountOpeningFlowReadyParams
}

type MarkMerchantBaofuAccountOpeningReadyTxResult struct {
	PaymentConfig MerchantPaymentConfig
	Flow          BaofuAccountOpeningFlow
	Merchant      Merchant
}

func (store *SQLStore) MarkMerchantBaofuAccountOpeningReadyTx(ctx context.Context, arg MarkMerchantBaofuAccountOpeningReadyTxParams) (MarkMerchantBaofuAccountOpeningReadyTxResult, error) {
	var result MarkMerchantBaofuAccountOpeningReadyTxResult
	err := store.execTx(ctx, func(q *Queries) error {
		cfg, err := q.UpsertMerchantPaymentConfig(ctx, arg.PaymentConfig)
		if err != nil {
			return err
		}
		result.PaymentConfig = cfg

		flow, err := q.MarkBaofuAccountOpeningFlowReady(ctx, arg.Flow)
		if err != nil {
			return err
		}
		result.Flow = flow

		merchant, err := q.ActivateApprovedMerchant(ctx, arg.PaymentConfig.MerchantID)
		if err != nil {
			if errors.Is(err, ErrRecordNotFound) {
				merchant, err = q.GetMerchant(ctx, arg.PaymentConfig.MerchantID)
			}
			if err != nil {
				return err
			}
		}
		result.Merchant = merchant
		return nil
	})
	return result, err
}
