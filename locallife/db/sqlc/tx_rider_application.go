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
			if existingRider.Status == "pending" || existingRider.Status == "pending_bindbank" {
				updatedRider, updateErr := q.UpdateRiderStatus(ctx, UpdateRiderStatusParams{
					ID:     existingRider.ID,
					Status: "active",
				})
				if updateErr != nil {
					return fmt.Errorf("update existing rider status: %w", updateErr)
				}
				result.Rider = updatedRider
			} else {
				result.Rider = existingRider
			}
		} else if !errors.Is(err, ErrRecordNotFound) {
			return fmt.Errorf("get rider by user: %w", err)
		} else {
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

			result.Rider, err = q.UpdateRiderStatus(ctx, UpdateRiderStatusParams{
				ID:     result.Rider.ID,
				Status: "active",
			})
			if err != nil {
				return fmt.Errorf("update rider status to active: %w", err)
			}
		}

		existingRole, err := q.GetUserRoleByType(ctx, GetUserRoleByTypeParams{
			UserID: result.Application.UserID,
			Role:   "rider",
		})
		if err != nil {
			if !errors.Is(err, ErrRecordNotFound) {
				return fmt.Errorf("get rider user role: %w", err)
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
		}

		if existingRole.Status == "active" && existingRole.RelatedEntityID.Valid && existingRole.RelatedEntityID.Int64 == result.Rider.ID {
			return nil
		}

		if err := q.DeleteUserRoleByUserAndRole(ctx, DeleteUserRoleByUserAndRoleParams{
			UserID: result.Application.UserID,
			Role:   "rider",
		}); err != nil {
			return fmt.Errorf("delete stale rider user role: %w", err)
		}

		_, err = q.CreateUserRole(ctx, CreateUserRoleParams{
			UserID:          result.Application.UserID,
			Role:            "rider",
			Status:          "active",
			RelatedEntityID: pgtype.Int8{Int64: result.Rider.ID, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("recreate rider user role: %w", err)
		}

		return nil
	})

	return result, err
}
