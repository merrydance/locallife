package db

import (
	"context"
	"fmt"
)

type ApproveOperatorRegionApplicationTxResult struct {
	Application    OperatorRegionApplication
	OperatorRegion OperatorRegion
}

func (store *SQLStore) ApproveOperatorRegionApplicationTx(ctx context.Context, applicationID int64) (ApproveOperatorRegionApplicationTxResult, error) {
	var result ApproveOperatorRegionApplicationTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		result.Application, err = q.ApproveOperatorRegionApplication(ctx, applicationID)
		if err != nil {
			return fmt.Errorf("approve operator region application: %w", err)
		}

		result.OperatorRegion, err = q.AddOperatorRegion(ctx, AddOperatorRegionParams{
			OperatorID: result.Application.OperatorID,
			RegionID:   result.Application.RegionID,
		})
		if err != nil {
			return fmt.Errorf("add operator region: %w", err)
		}

		return nil
	})

	return result, err
}
