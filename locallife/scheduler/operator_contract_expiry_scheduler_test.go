package scheduler

import (
	"testing"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestDataCleanupScheduler_MarkExpiredOperators_SuspendsWithoutReceiverTarget(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	s := NewDataCleanupScheduler(store, nil, nil)

	row := db.ListExpiredOperatorsRow{
		ID:          18,
		UserID:      81,
		RegionID:    66,
		Name:        "测试运营商",
		ContactName: "张运营",
		Status:      "active",
		RegionName:  "测试区域",
	}
	role := db.UserRole{ID: 201, UserID: row.UserID, Role: "operator", Status: "active"}

	store.EXPECT().ListExpiredOperators(gomock.Any()).Return([]db.ListExpiredOperatorsRow{row}, nil)
	store.EXPECT().
		UpdateOperatorStatus(gomock.Any(), db.UpdateOperatorStatusParams{ID: row.ID, Status: "suspended"}).
		Return(db.Operator{ID: row.ID, UserID: row.UserID, RegionID: row.RegionID, Name: row.Name, ContactName: row.ContactName, Status: "suspended"}, nil)
	store.EXPECT().
		GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{UserID: row.UserID, Role: "operator"}).
		Return(role, nil)
	store.EXPECT().
		UpdateUserRoleStatus(gomock.Any(), db.UpdateUserRoleStatusParams{ID: role.ID, Status: "suspended"}).
		Return(db.UserRole{ID: role.ID, UserID: row.UserID, Role: "operator", Status: "suspended"}, nil)

	s.markExpiredOperators()
}

func TestDataCleanupScheduler_MarkExpiredOperators_ContinuesAfterFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	s := NewDataCleanupScheduler(store, nil, nil)

	first := db.ListExpiredOperatorsRow{ID: 18, UserID: 81, RegionID: 66, Name: "测试运营商甲", ContactName: "张运营", Status: "active", RegionName: "区域甲"}
	second := db.ListExpiredOperatorsRow{ID: 19, UserID: 82, RegionID: 67, Name: "测试运营商乙", ContactName: "李运营", Status: "active", RegionName: "区域乙"}
	firstRole := db.UserRole{ID: 201, UserID: first.UserID, Role: "operator", Status: "active"}
	secondRole := db.UserRole{ID: 202, UserID: second.UserID, Role: "operator", Status: "active"}

	store.EXPECT().ListExpiredOperators(gomock.Any()).Return([]db.ListExpiredOperatorsRow{first, second}, nil)
	store.EXPECT().GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{UserID: first.UserID, Role: "operator"}).Return(firstRole, nil)
	store.EXPECT().UpdateUserRoleStatus(gomock.Any(), db.UpdateUserRoleStatusParams{ID: firstRole.ID, Status: "suspended"}).Return(db.UserRole{}, assertErrScheduler("update role failed"))
	store.EXPECT().GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{UserID: second.UserID, Role: "operator"}).Return(secondRole, nil)
	store.EXPECT().UpdateUserRoleStatus(gomock.Any(), db.UpdateUserRoleStatusParams{ID: secondRole.ID, Status: "suspended"}).Return(db.UserRole{ID: secondRole.ID, UserID: second.UserID, Role: "operator", Status: "suspended"}, nil)
	store.EXPECT().
		UpdateOperatorStatus(gomock.Any(), db.UpdateOperatorStatusParams{ID: second.ID, Status: "suspended"}).
		Return(db.Operator{ID: second.ID, UserID: second.UserID, RegionID: second.RegionID, Name: second.Name, ContactName: second.ContactName, Status: "suspended"}, nil)

	s.markExpiredOperators()
}

func TestDataCleanupScheduler_MarkExpiredOperators_KeepsLocalSuspensionWithoutReceiverTarget(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	s := NewDataCleanupScheduler(store, nil, nil)

	row := db.ListExpiredOperatorsRow{ID: 21, UserID: 91, RegionID: 69, Name: "测试运营商丙", ContactName: "王运营", Status: "active", RegionName: "区域丙"}
	role := db.UserRole{ID: 203, UserID: row.UserID, Role: "operator", Status: "active"}

	store.EXPECT().ListExpiredOperators(gomock.Any()).Return([]db.ListExpiredOperatorsRow{row}, nil)
	store.EXPECT().GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{UserID: row.UserID, Role: "operator"}).Return(role, nil)
	store.EXPECT().UpdateUserRoleStatus(gomock.Any(), db.UpdateUserRoleStatusParams{ID: role.ID, Status: "suspended"}).Return(db.UserRole{ID: role.ID, UserID: row.UserID, Role: "operator", Status: "suspended"}, nil)
	store.EXPECT().UpdateOperatorStatus(gomock.Any(), db.UpdateOperatorStatusParams{ID: row.ID, Status: "suspended"}).Return(db.Operator{ID: row.ID, UserID: row.UserID, RegionID: row.RegionID, Name: row.Name, ContactName: row.ContactName, Status: "suspended"}, nil)

	s.markExpiredOperators()
}

func TestDataCleanupScheduler_MarkExpiredOperators_NoExpiredOperators(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	s := NewDataCleanupScheduler(store, nil, nil)
	store.EXPECT().ListExpiredOperators(gomock.Any()).Return([]db.ListExpiredOperatorsRow{}, nil)

	s.markExpiredOperators()
	require.True(t, true)
}

type schedulerTestError string

func (e schedulerTestError) Error() string {
	return string(e)
}

func assertErrScheduler(message string) error {
	return schedulerTestError(message)
}
