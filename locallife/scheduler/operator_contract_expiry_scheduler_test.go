package scheduler

import (
	"context"
	"testing"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestDataCleanupScheduler_MarkExpiredOperators_SuspendsAndWritesAbsentReceiverIntent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	s := NewDataCleanupScheduler(store, nil, nil, ecommerceClient)

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
	user := db.User{ID: row.UserID, WechatOpenid: "operator-openid-81"}

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
	store.EXPECT().GetUser(gomock.Any(), row.UserID).Return(user, nil)
	ecommerceClient.EXPECT().GetSpAppID().Return("wx_sp_app_123")
	expectSchedulerOperatorReceiverTargetIntent(t, store, row.ID, user.WechatOpenid, db.ProfitSharingReceiverDesiredStateAbsent)

	s.markExpiredOperators()
}

func TestDataCleanupScheduler_MarkExpiredOperators_ContinuesAfterFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	s := NewDataCleanupScheduler(store, nil, nil, ecommerceClient)

	first := db.ListExpiredOperatorsRow{ID: 18, UserID: 81, RegionID: 66, Name: "测试运营商甲", ContactName: "张运营", Status: "active", RegionName: "区域甲"}
	second := db.ListExpiredOperatorsRow{ID: 19, UserID: 82, RegionID: 67, Name: "测试运营商乙", ContactName: "李运营", Status: "active", RegionName: "区域乙"}
	firstRole := db.UserRole{ID: 201, UserID: first.UserID, Role: "operator", Status: "active"}
	secondRole := db.UserRole{ID: 202, UserID: second.UserID, Role: "operator", Status: "active"}
	firstUser := db.User{ID: first.UserID, WechatOpenid: "operator-openid-81"}
	secondUser := db.User{ID: second.UserID, WechatOpenid: "operator-openid-82"}

	store.EXPECT().ListExpiredOperators(gomock.Any()).Return([]db.ListExpiredOperatorsRow{first, second}, nil)
	store.EXPECT().GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{UserID: first.UserID, Role: "operator"}).Return(firstRole, nil)
	store.EXPECT().UpdateUserRoleStatus(gomock.Any(), db.UpdateUserRoleStatusParams{ID: firstRole.ID, Status: "suspended"}).Return(db.UserRole{ID: firstRole.ID, UserID: first.UserID, Role: "operator", Status: "suspended"}, nil)
	store.EXPECT().UpdateOperatorStatus(gomock.Any(), db.UpdateOperatorStatusParams{ID: first.ID, Status: "suspended"}).Return(db.Operator{ID: first.ID, UserID: first.UserID, RegionID: first.RegionID, Name: first.Name, ContactName: first.ContactName, Status: "suspended"}, nil)
	ecommerceClient.EXPECT().GetSpAppID().Return("wx_sp_app_123")
	store.EXPECT().GetUser(gomock.Any(), first.UserID).Return(firstUser, nil)
	store.EXPECT().UpsertProfitSharingReceiverTarget(gomock.Any(), gomock.Any()).Return(db.ProfitSharingReceiverTarget{}, assertErrScheduler("receiver intent failed"))
	store.EXPECT().GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{UserID: second.UserID, Role: "operator"}).Return(secondRole, nil)
	store.EXPECT().UpdateUserRoleStatus(gomock.Any(), db.UpdateUserRoleStatusParams{ID: secondRole.ID, Status: "suspended"}).Return(db.UserRole{ID: secondRole.ID, UserID: second.UserID, Role: "operator", Status: "suspended"}, nil)
	store.EXPECT().
		UpdateOperatorStatus(gomock.Any(), db.UpdateOperatorStatusParams{ID: second.ID, Status: "suspended"}).
		Return(db.Operator{ID: second.ID, UserID: second.UserID, RegionID: second.RegionID, Name: second.Name, ContactName: second.ContactName, Status: "suspended"}, nil)
	ecommerceClient.EXPECT().GetSpAppID().Return("wx_sp_app_123")
	store.EXPECT().GetUser(gomock.Any(), second.UserID).Return(secondUser, nil)
	expectSchedulerOperatorReceiverTargetIntent(t, store, second.ID, secondUser.WechatOpenid, db.ProfitSharingReceiverDesiredStateAbsent)

	s.markExpiredOperators()
}

func TestDataCleanupScheduler_MarkExpiredOperators_ReceiverIntentFailureKeepsLocalSuspension(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	s := NewDataCleanupScheduler(store, nil, nil, ecommerceClient)

	row := db.ListExpiredOperatorsRow{ID: 21, UserID: 91, RegionID: 69, Name: "测试运营商丙", ContactName: "王运营", Status: "active", RegionName: "区域丙"}
	role := db.UserRole{ID: 203, UserID: row.UserID, Role: "operator", Status: "active"}
	user := db.User{ID: row.UserID, WechatOpenid: "operator-openid-91"}

	store.EXPECT().ListExpiredOperators(gomock.Any()).Return([]db.ListExpiredOperatorsRow{row}, nil)
	store.EXPECT().GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{UserID: row.UserID, Role: "operator"}).Return(role, nil)
	store.EXPECT().UpdateUserRoleStatus(gomock.Any(), db.UpdateUserRoleStatusParams{ID: role.ID, Status: "suspended"}).Return(db.UserRole{ID: role.ID, UserID: row.UserID, Role: "operator", Status: "suspended"}, nil)
	store.EXPECT().UpdateOperatorStatus(gomock.Any(), db.UpdateOperatorStatusParams{ID: row.ID, Status: "suspended"}).Return(db.Operator{ID: row.ID, UserID: row.UserID, RegionID: row.RegionID, Name: row.Name, ContactName: row.ContactName, Status: "suspended"}, nil)
	ecommerceClient.EXPECT().GetSpAppID().Return("wx_sp_app_123")
	store.EXPECT().GetUser(gomock.Any(), row.UserID).Return(user, nil)
	store.EXPECT().UpsertProfitSharingReceiverTarget(gomock.Any(), gomock.Any()).Return(db.ProfitSharingReceiverTarget{}, assertErrScheduler("receiver intent failed"))

	s.markExpiredOperators()
}

func TestDataCleanupScheduler_MarkExpiredOperators_SkipsWithoutEcommerceClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	s := NewDataCleanupScheduler(store, nil, nil)

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

func expectSchedulerOperatorReceiverTargetIntent(t *testing.T, store *mockdb.MockStore, operatorID int64, openID string, desiredState string) {
	t.Helper()

	store.EXPECT().UpsertProfitSharingReceiverTarget(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.UpsertProfitSharingReceiverTargetParams) (db.ProfitSharingReceiverTarget, error) {
		require.Equal(t, db.ExternalPaymentProviderWechat, arg.Provider)
		require.Equal(t, db.PaymentChannelEcommerce, arg.Channel)
		require.Equal(t, db.ProfitSharingReceiverOwnerTypeOperator, arg.OwnerType)
		require.Equal(t, operatorID, arg.OwnerID)
		require.Equal(t, db.ProfitSharingReceiverTypePersonalOpenID, arg.ReceiverType)
		require.Equal(t, "wx_sp_app_123", arg.Appid)
		require.Equal(t, desiredState, arg.DesiredState)
		require.NotEmpty(t, arg.AccountHash)
		require.NotContains(t, arg.AccountHash, openID)
		return db.ProfitSharingReceiverTarget{ID: operatorID + 1000, OwnerID: operatorID, DesiredState: desiredState, SyncStatus: db.ProfitSharingReceiverSyncStatusPending}, nil
	})
}
