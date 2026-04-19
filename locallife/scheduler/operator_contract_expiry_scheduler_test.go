package scheduler

import (
	"testing"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestDataCleanupScheduler_MarkExpiredOperators_SuspendsAndDeletesReceiver(t *testing.T) {
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
	ecommerceClient.EXPECT().DeleteProfitSharingReceiver(gomock.Any(), &wechatcontracts.DeleteReceiverRequest{
		AppID:   "wx_sp_app_123",
		Type:    wechatcontracts.ReceiverTypePersonal,
		Account: user.WechatOpenid,
	}).Return(&wechatcontracts.DeleteReceiverResponse{Type: wechatcontracts.ReceiverTypePersonal, Account: user.WechatOpenid}, nil)

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
	role := db.UserRole{ID: 202, UserID: second.UserID, Role: "operator", Status: "active"}
	firstUser := db.User{ID: first.UserID, WechatOpenid: "operator-openid-81"}
	user := db.User{ID: second.UserID, WechatOpenid: "operator-openid-82"}

	store.EXPECT().ListExpiredOperators(gomock.Any()).Return([]db.ListExpiredOperatorsRow{first, second}, nil)
	store.EXPECT().GetUser(gomock.Any(), first.UserID).Return(firstUser, nil)
	ecommerceClient.EXPECT().GetSpAppID().Return("wx_sp_app_123")
	ecommerceClient.EXPECT().DeleteProfitSharingReceiver(gomock.Any(), &wechatcontracts.DeleteReceiverRequest{
		AppID:   "wx_sp_app_123",
		Type:    wechatcontracts.ReceiverTypePersonal,
		Account: firstUser.WechatOpenid,
	}).Return(nil, assertErrScheduler("wechat delete failed"))
	store.EXPECT().
		UpdateOperatorStatus(gomock.Any(), db.UpdateOperatorStatusParams{ID: second.ID, Status: "suspended"}).
		Return(db.Operator{ID: second.ID, UserID: second.UserID, RegionID: second.RegionID, Name: second.Name, ContactName: second.ContactName, Status: "suspended"}, nil)
	store.EXPECT().
		GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{UserID: second.UserID, Role: "operator"}).
		Return(role, nil)
	store.EXPECT().
		UpdateUserRoleStatus(gomock.Any(), db.UpdateUserRoleStatusParams{ID: role.ID, Status: "suspended"}).
		Return(db.UserRole{ID: role.ID, UserID: second.UserID, Role: "operator", Status: "suspended"}, nil)
	store.EXPECT().GetUser(gomock.Any(), second.UserID).Return(user, nil)
	ecommerceClient.EXPECT().GetSpAppID().Return("wx_sp_app_123")
	ecommerceClient.EXPECT().DeleteProfitSharingReceiver(gomock.Any(), &wechatcontracts.DeleteReceiverRequest{
		AppID:   "wx_sp_app_123",
		Type:    wechatcontracts.ReceiverTypePersonal,
		Account: user.WechatOpenid,
	}).Return(&wechatcontracts.DeleteReceiverResponse{Type: wechatcontracts.ReceiverTypePersonal, Account: user.WechatOpenid}, nil)

	s.markExpiredOperators()
}

func TestDataCleanupScheduler_MarkExpiredOperators_DeleteFailureDoesNotPersistSuspendedState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	s := NewDataCleanupScheduler(store, nil, nil, ecommerceClient)

	row := db.ListExpiredOperatorsRow{ID: 21, UserID: 91, RegionID: 69, Name: "测试运营商丙", ContactName: "王运营", Status: "active", RegionName: "区域丙"}
	user := db.User{ID: row.UserID, WechatOpenid: "operator-openid-91"}

	store.EXPECT().ListExpiredOperators(gomock.Any()).Return([]db.ListExpiredOperatorsRow{row}, nil)
	store.EXPECT().GetUser(gomock.Any(), row.UserID).Return(user, nil)
	ecommerceClient.EXPECT().GetSpAppID().Return("wx_sp_app_123")
	ecommerceClient.EXPECT().DeleteProfitSharingReceiver(gomock.Any(), &wechatcontracts.DeleteReceiverRequest{
		AppID:   "wx_sp_app_123",
		Type:    wechatcontracts.ReceiverTypePersonal,
		Account: user.WechatOpenid,
	}).Return(nil, assertErrScheduler("wechat delete failed"))

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
