package logic

import (
	"context"
	"errors"
	"testing"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestOperatorStatusService_UpdateStatus_SuspendsOperatorAndRoleWithoutReceiverTarget(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	service := NewOperatorStatusService(store)

	operator := db.Operator{
		ID:          61,
		UserID:      161,
		RegionID:    71,
		Name:        "运营商甲",
		ContactName: "运营商甲",
		Status:      operatorStatusActive,
	}
	updatedOperator := operator
	updatedOperator.Status = operatorStatusSuspended
	role := db.UserRole{ID: 261, UserID: operator.UserID, Role: operatorRole, Status: operatorStatusActive}

	store.EXPECT().
		GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{UserID: operator.UserID, Role: operatorRole}).
		Return(role, nil)
	store.EXPECT().
		UpdateUserRoleStatus(gomock.Any(), db.UpdateUserRoleStatusParams{ID: role.ID, Status: operatorStatusSuspended}).
		Return(db.UserRole{ID: role.ID, UserID: operator.UserID, Role: operatorRole, Status: operatorStatusSuspended}, nil)
	store.EXPECT().
		UpdateOperatorStatus(gomock.Any(), db.UpdateOperatorStatusParams{ID: operator.ID, Status: operatorStatusSuspended}).
		Return(updatedOperator, nil)

	result, err := service.UpdateStatus(context.Background(), operator, operatorStatusSuspended)
	require.NoError(t, err)
	require.Equal(t, operatorStatusSuspended, result.Status)
}

func TestOperatorStatusService_UpdateStatus_SuspendRoleFailureDoesNotPersistOperatorStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	service := NewOperatorStatusService(store)

	operator := db.Operator{
		ID:          62,
		UserID:      162,
		RegionID:    72,
		Name:        "运营商乙",
		ContactName: "运营商乙",
		Status:      operatorStatusActive,
	}
	role := db.UserRole{ID: 262, UserID: operator.UserID, Role: operatorRole, Status: operatorStatusActive}

	store.EXPECT().
		GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{UserID: operator.UserID, Role: operatorRole}).
		Return(role, nil)
	store.EXPECT().
		UpdateUserRoleStatus(gomock.Any(), db.UpdateUserRoleStatusParams{ID: role.ID, Status: operatorStatusSuspended}).
		Return(db.UserRole{}, errors.New("update role failed"))

	_, err := service.UpdateStatus(context.Background(), operator, operatorStatusSuspended)
	require.Error(t, err)
	require.Contains(t, err.Error(), "update operator user role status")
}

func TestOperatorStatusService_UpdateStatus_RepeatedSuspendDoesNotRewriteOperatorStatusOrReceiverTarget(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	service := NewOperatorStatusService(store)

	operator := db.Operator{
		ID:          63,
		UserID:      163,
		RegionID:    73,
		Name:        "运营商丙",
		ContactName: "运营商丙",
		Status:      operatorStatusSuspended,
	}
	role := db.UserRole{ID: 263, UserID: operator.UserID, Role: operatorRole, Status: operatorStatusSuspended}

	store.EXPECT().
		GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{UserID: operator.UserID, Role: operatorRole}).
		Return(role, nil)

	result, err := service.UpdateStatus(context.Background(), operator, operatorStatusSuspended)
	require.NoError(t, err)
	require.Equal(t, operatorStatusSuspended, result.Status)
}
