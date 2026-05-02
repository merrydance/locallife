package logic

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
)

const (
	operatorRole            = "operator"
	operatorStatusActive    = "active"
	operatorStatusSuspended = "suspended"
)

var ErrUnsupportedOperatorStatus = errors.New("unsupported operator status")

type OperatorActiveRegionConflictError struct {
	RegionID         int64
	ActiveOperatorID int64
}

func (e *OperatorActiveRegionConflictError) Error() string {
	return fmt.Sprintf("region %d already has active operator %d", e.RegionID, e.ActiveOperatorID)
}

type OperatorStatusService struct {
	store db.Store
}

func NewOperatorStatusService(store db.Store, ecommerceClient wechat.EcommerceClientInterface) *OperatorStatusService {
	_ = ecommerceClient
	return &OperatorStatusService{
		store: store,
	}
}

func (s *OperatorStatusService) UpdateStatus(ctx context.Context, operator db.Operator, targetStatus string) (db.Operator, error) {
	if targetStatus != operatorStatusActive && targetStatus != operatorStatusSuspended {
		return db.Operator{}, ErrUnsupportedOperatorStatus
	}

	if targetStatus == operatorStatusActive {
		if err := s.ensureManagedRegionsHaveNoOtherActiveOperator(ctx, operator); err != nil {
			return db.Operator{}, err
		}
	} else {
		if err := s.ensureOperatorRoleStatus(ctx, operator, targetStatus); err != nil {
			return db.Operator{}, err
		}
	}

	updatedOperator := operator
	if operator.Status != targetStatus {
		var err error
		updatedOperator, err = s.store.UpdateOperatorStatus(ctx, db.UpdateOperatorStatusParams{
			ID:     operator.ID,
			Status: targetStatus,
		})
		if err != nil {
			return db.Operator{}, fmt.Errorf("update operator status: %w", err)
		}
		updatedOperator.Status = targetStatus
	}

	if targetStatus == operatorStatusActive {
		if err := s.ensureOperatorRoleStatus(ctx, updatedOperator, targetStatus); err != nil {
			return db.Operator{}, err
		}
	}

	return updatedOperator, nil
}

func (s *OperatorStatusService) ensureManagedRegionsHaveNoOtherActiveOperator(ctx context.Context, operator db.Operator) error {
	regions, err := s.store.ListOperatorRegions(ctx, operator.ID)
	if err != nil {
		return fmt.Errorf("list operator regions: %w", err)
	}

	regionIDs := make(map[int64]struct{}, len(regions)+1)
	for _, region := range regions {
		regionIDs[region.RegionID] = struct{}{}
	}
	if operator.RegionID > 0 {
		regionIDs[operator.RegionID] = struct{}{}
	}

	for regionID := range regionIDs {
		activeOperator, getErr := s.store.GetActiveOperatorByRegion(ctx, regionID)
		if getErr == nil {
			if activeOperator.ID != operator.ID {
				return &OperatorActiveRegionConflictError{RegionID: regionID, ActiveOperatorID: activeOperator.ID}
			}
			continue
		}
		if !errors.Is(getErr, db.ErrRecordNotFound) {
			return fmt.Errorf("get active operator by region %d: %w", regionID, getErr)
		}
	}

	return nil
}

func (s *OperatorStatusService) ensureOperatorRoleStatus(ctx context.Context, operator db.Operator, targetStatus string) error {
	role, err := s.store.GetUserRoleByType(ctx, db.GetUserRoleByTypeParams{
		UserID: operator.UserID,
		Role:   operatorRole,
	})
	if err != nil {
		if !errors.Is(err, db.ErrRecordNotFound) {
			return fmt.Errorf("get operator user role: %w", err)
		}
		if targetStatus != operatorStatusActive {
			return nil
		}

		_, createErr := s.store.CreateUserRole(ctx, db.CreateUserRoleParams{
			UserID: operator.UserID,
			Role:   operatorRole,
			Status: operatorStatusActive,
			RelatedEntityID: pgtype.Int8{
				Int64: operator.ID,
				Valid: true,
			},
		})
		if createErr != nil {
			return fmt.Errorf("create operator user role: %w", createErr)
		}
		return nil
	}

	if role.Status == targetStatus {
		return nil
	}

	_, err = s.store.UpdateUserRoleStatus(ctx, db.UpdateUserRoleStatusParams{
		ID:     role.ID,
		Status: targetStatus,
	})
	if err != nil {
		return fmt.Errorf("update operator user role status: %w", err)
	}

	return nil
}
