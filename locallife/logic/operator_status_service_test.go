package logic

import (
	"context"
	"errors"
	"testing"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestOperatorStatusService_UpdateStatus_SuspendWritesAbsentReceiverIntent(t *testing.T) {
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
	updatedOperator := operator
	updatedOperator.Status = operatorStatusSuspended
	role := db.UserRole{ID: 261, UserID: operator.UserID, Role: operatorRole, Status: operatorStatusActive}
	user := db.User{ID: operator.UserID, WechatOpenid: "operator-openid-161"}

	store.EXPECT().
		GetUserRoleByType(gomock.Any(), db.GetUserRoleByTypeParams{UserID: operator.UserID, Role: operatorRole}).
		Return(role, nil)
	store.EXPECT().
		UpdateUserRoleStatus(gomock.Any(), db.UpdateUserRoleStatusParams{ID: role.ID, Status: operatorStatusSuspended}).
		Return(db.UserRole{ID: role.ID, UserID: operator.UserID, Role: operatorRole, Status: operatorStatusSuspended}, nil)
	store.EXPECT().
		UpdateOperatorStatus(gomock.Any(), db.UpdateOperatorStatusParams{ID: operator.ID, Status: operatorStatusSuspended}).
		Return(updatedOperator, nil)
	expectLogicOperatorReceiverTargetIntent(t, store, ecommerceClient, updatedOperator, user, db.ProfitSharingReceiverDesiredStateAbsent)

	result, err := service.UpdateStatus(context.Background(), operator, operatorStatusSuspended)
	require.NoError(t, err)
	require.Equal(t, operatorStatusSuspended, result.Status)
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

func expectLogicOperatorReceiverTargetIntent(
	t *testing.T,
	store *mockdb.MockStore,
	ecommerceClient *mockwechat.MockEcommerceClientInterface,
	operator db.Operator,
	user db.User,
	desiredState string,
) {
	t.Helper()

	store.EXPECT().GetUser(gomock.Any(), operator.UserID).Return(user, nil)
	ecommerceClient.EXPECT().GetSpAppID().Return("wx_sp_app_123")
	store.EXPECT().UpsertProfitSharingReceiverTarget(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.UpsertProfitSharingReceiverTargetParams) (db.ProfitSharingReceiverTarget, error) {
		require.Equal(t, db.ExternalPaymentProviderWechat, arg.Provider)
		require.Equal(t, db.PaymentChannelEcommerce, arg.Channel)
		require.Equal(t, db.ProfitSharingReceiverOwnerTypeOperator, arg.OwnerType)
		require.Equal(t, operator.ID, arg.OwnerID)
		require.Equal(t, db.ProfitSharingReceiverTypePersonalOpenID, arg.ReceiverType)
		require.Equal(t, "wx_sp_app_123", arg.Appid)
		require.Equal(t, desiredState, arg.DesiredState)
		require.NotEmpty(t, arg.AccountHash)
		require.NotContains(t, arg.AccountHash, user.WechatOpenid)
		require.NotContains(t, arg.DisplayNameHash.String, operator.ContactName)
		return db.ProfitSharingReceiverTarget{ID: 1, DesiredState: desiredState, SyncStatus: db.ProfitSharingReceiverSyncStatusPending}, nil
	})
}
