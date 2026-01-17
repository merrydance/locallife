package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

// ApproveRiderApplicationTxParams contains input parameters for approving rider application
// and creating a rider record atomically.
type ApproveRiderApplicationTxParams struct {
	ApplicationID int64
	ReviewedBy    pgtype.Int8
	RiderRealName string
	RiderIDCardNo string
	RiderPhone    string
	RegionID      pgtype.Int8
}

// ApproveRiderApplicationTxResult contains the result of approval transaction
type ApproveRiderApplicationTxResult struct {
	Application RiderApplication
	Rider       Rider
}

// ApproveRiderApplicationTx approves a rider application and creates the rider record
// in a single transaction to avoid leaving an approved application without a rider.
func (store *SQLStore) ApproveRiderApplicationTx(ctx context.Context, arg ApproveRiderApplicationTxParams) (ApproveRiderApplicationTxResult, error) {
	var result ApproveRiderApplicationTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		result.Application, err = q.ApproveRiderApplication(ctx, ApproveRiderApplicationParams{
			ID:         arg.ApplicationID,
			ReviewedBy: arg.ReviewedBy,
		})
		if err != nil {
			return fmt.Errorf("approve rider application: %w", err)
		}

		// Create rider record (idempotent if already exists)
		existingRider, err := q.GetRiderByUserID(ctx, result.Application.UserID)
		if err == nil {
			result.Rider = existingRider
			return nil
		}
		if !errors.Is(err, ErrRecordNotFound) {
			return fmt.Errorf("get rider by user: %w", err)
		}

		result.Rider, err = q.CreateRider(ctx, CreateRiderParams{
			UserID:   result.Application.UserID,
			RealName: arg.RiderRealName,
			IDCardNo: arg.RiderIDCardNo,
			Phone:    arg.RiderPhone,
			RegionID: arg.RegionID,
		})
		if err != nil {
			return fmt.Errorf("create rider: %w", err)
		}

		return nil
	})

	return result, err
}
