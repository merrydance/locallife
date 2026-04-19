package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

type ApproveOperatorApplicationTxParams struct {
	ApplicationID     int64
	ReviewedBy        pgtype.Int8
	OperatorName      string
	ContactName       string
	ContactPhone      string
	ContractStartDate pgtype.Date
	ContractEndDate   pgtype.Date
	ContractYears     int32
}

type ApproveOperatorApplicationTxResult struct {
	Application OperatorApplication
	Operator    Operator
	UserRole    UserRole
}

// ApproveOperatorApplicationTx approves an operator application, creates the
// operator entity, wires the initial operator_region, and ensures the operator
// user role atomically.
func (store *SQLStore) ApproveOperatorApplicationTx(ctx context.Context, arg ApproveOperatorApplicationTxParams) (ApproveOperatorApplicationTxResult, error) {
	var result ApproveOperatorApplicationTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		result.Application, err = q.ApproveOperatorApplication(ctx, ApproveOperatorApplicationParams{
			ID:         arg.ApplicationID,
			ReviewedBy: arg.ReviewedBy,
		})
		if err != nil {
			return fmt.Errorf("approve operator application: %w", err)
		}

		result.Operator, err = q.CreateOperator(ctx, CreateOperatorParams{
			UserID:            result.Application.UserID,
			RegionID:          result.Application.RegionID,
			Name:              arg.OperatorName,
			ContactName:       arg.ContactName,
			ContactPhone:      arg.ContactPhone,
			WechatMchID:       pgtype.Text{},
			Status:            "active",
			ContractStartDate: arg.ContractStartDate,
			ContractEndDate:   arg.ContractEndDate,
			ContractYears:     arg.ContractYears,
		})
		if err != nil {
			return fmt.Errorf("create operator: %w", err)
		}

		if _, err = q.AddOperatorRegion(ctx, AddOperatorRegionParams{
			OperatorID: result.Operator.ID,
			RegionID:   result.Application.RegionID,
		}); err != nil {
			return fmt.Errorf("add initial operator region: %w", err)
		}

		result.UserRole, err = q.GetUserRoleByType(ctx, GetUserRoleByTypeParams{
			UserID: result.Application.UserID,
			Role:   "operator",
		})
		if err != nil {
			if !errors.Is(err, ErrRecordNotFound) {
				return fmt.Errorf("get operator user role: %w", err)
			}

			result.UserRole, err = q.CreateUserRole(ctx, CreateUserRoleParams{
				UserID: result.Application.UserID,
				Role:   "operator",
				Status: "active",
				RelatedEntityID: pgtype.Int8{
					Int64: result.Operator.ID,
					Valid: true,
				},
			})
			if err != nil {
				return fmt.Errorf("create operator user role: %w", err)
			}

			return nil
		}

		if result.UserRole.Status != "active" {
			result.UserRole, err = q.UpdateUserRoleStatus(ctx, UpdateUserRoleStatusParams{
				ID:     result.UserRole.ID,
				Status: "active",
			})
			if err != nil {
				return fmt.Errorf("reactivate operator user role: %w", err)
			}
		}

		return nil
	})

	return result, err
}
