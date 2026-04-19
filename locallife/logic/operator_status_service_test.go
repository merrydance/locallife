package logic

import (
	"context"
	"errors"
	"testing"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestOperatorStatusService_UpdateStatus_SuspendDeleteFailureDoesNotPersistLocalState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	service := NewOperatorStatusService(store, ecommerceClient)

	operator := db.Operator{
		ID:          61,
		UserID:      161,
		RegionID:    71,
		Name:        "运营商甲",
		ContactName: "运营商甲",
		Status:      operatorStatusActive,
	}
	user := db.User{ID: operator.UserID, WechatOpenid: "operator-openid-161"}

	store.EXPECT().GetUser(gomock.Any(), operator.UserID).Return(user, nil)
	ecommerceClient.EXPECT().GetSpAppID().Return("wx_sp_app_123")
	ecommerceClient.EXPECT().DeleteProfitSharingReceiver(gomock.Any(), &wechatcontracts.DeleteReceiverRequest{
		AppID:   "wx_sp_app_123",
		Type:    wechatcontracts.ReceiverTypePersonal,
		Account: user.WechatOpenid,
	}).Return(nil, errors.New("wechat delete failed"))

	_, err := service.UpdateStatus(context.Background(), operator, operatorStatusSuspended)
	require.Error(t, err)

	var receiverSyncErr *OperatorReceiverSyncError
	require.ErrorAs(t, err, &receiverSyncErr)
	require.Equal(t, "delete", receiverSyncErr.Action)
}

func TestOperatorStatusService_UpdateStatus_SuspendRoleFailureDoesNotPersistOperatorStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	service := NewOperatorStatusService(store, ecommerceClient)

	operator := db.Operator{
		ID:          62,
		UserID:      162,
		RegionID:    72,
		Name:        "运营商乙",
		ContactName: "运营商乙",
		Status:      operatorStatusActive,
	}
	role := db.UserRole{ID: 262, UserID: operator.UserID, Role: operatorRole, Status: operatorStatusActive}
	user := db.User{ID: operator.UserID, WechatOpenid: "operator-openid-162"}

	store.EXPECT().GetUser(gomock.Any(), operator.UserID).Return(user, nil)
	ecommerceClient.EXPECT().GetSpAppID().Return("wx_sp_app_123")
	ecommerceClient.EXPECT().DeleteProfitSharingReceiver(gomock.Any(), &wechatcontracts.DeleteReceiverRequest{
		AppID:   "wx_sp_app_123",
		Type:    wechatcontracts.ReceiverTypePersonal,
		Account: user.WechatOpenid,
	}).Return(&wechatcontracts.DeleteReceiverResponse{Type: wechatcontracts.ReceiverTypePersonal, Account: user.WechatOpenid}, nil)
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
