package db

import (
	"context"
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

		_, err = q.CreateUserRole(ctx, CreateUserRoleParams{
			UserID:          result.Application.UserID,
			Role:            "rider",
			Status:          "active",
			RelatedEntityID: pgtype.Int8{Int64: result.Rider.ID, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("create rider user role: %w", err)
		}

		return nil
	})

	return result, err
}
