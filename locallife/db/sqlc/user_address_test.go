package db

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func createRandomUserAddress(t *testing.T, user User) UserAddress {
	// 创建有效的经纬度 (北京附近)
	var longitude, latitude pgtype.Numeric
	err := longitude.Scan("116.4074")
	require.NoError(t, err)
	err = latitude.Scan("39.9042")
	require.NoError(t, err)

	arg := CreateUserAddressParams{
		UserID:        user.ID,
		RegionID:      1, // 假设1是有效的区域ID
		DetailAddress: util.RandomString(20),
		ContactName:   util.RandomOwner(),
		ContactPhone:  fmt.Sprintf("139%08d", util.RandomInt(10000000, 99999999)),
		Longitude:     longitude,
		Latitude:      latitude,
		IsDefault:     false,
	}

	address, err := testStore.CreateUserAddress(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, address)

	require.Equal(t, arg.UserID, address.UserID)
	require.Equal(t, arg.ContactName, address.ContactName)
	require.Equal(t, arg.ContactPhone, address.ContactPhone)
	require.Equal(t, arg.RegionID, address.RegionID)
	require.Equal(t, arg.DetailAddress, address.DetailAddress)
	require.Equal(t, arg.IsDefault, address.IsDefault)
	require.NotZero(t, address.ID)
	require.NotZero(t, address.CreatedAt)

	return address
}

func TestCreateUserAddress(t *testing.T) {
	user := createRandomUser(t)
	createRandomUserAddress(t, user)
}

func TestGetUserAddress(t *testing.T) {
	user := createRandomUser(t)
	address1 := createRandomUserAddress(t, user)

	address2, err := testStore.GetUserAddress(context.Background(), address1.ID)
	require.NoError(t, err)
	require.NotEmpty(t, address2)

	require.Equal(t, address1.ID, address2.ID)
	require.Equal(t, address1.UserID, address2.UserID)
	require.Equal(t, address1.ContactName, address2.ContactName)
	require.Equal(t, address1.ContactPhone, address2.ContactPhone)
}

func TestListUserAddresses(t *testing.T) {
	user := createRandomUser(t)

	// 创建5个地址
	for i := 0; i < 5; i++ {
		createRandomUserAddress(t, user)
	}

	addresses, err := testStore.ListUserAddresses(context.Background(), user.ID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(addresses), 5)

	for _, address := range addresses {
		require.NotEmpty(t, address)
		require.Equal(t, user.ID, address.UserID)
	}
}

func TestUpdateUserAddress(t *testing.T) {
	user := createRandomUser(t)
	address1 := createRandomUserAddress(t, user)

	newContactName := util.RandomOwner()
	newContactPhone := fmt.Sprintf("139%08d", util.RandomInt(10000000, 99999999))
	newDetailAddress := util.RandomString(30)

	arg := UpdateUserAddressParams{
		ID:     address1.ID,
		UserID: user.ID,
		ContactName: pgtype.Text{
			String: newContactName,
			Valid:  true,
		},
		ContactPhone: pgtype.Text{
			String: newContactPhone,
			Valid:  true,
		},
		DetailAddress: pgtype.Text{
			String: newDetailAddress,
			Valid:  true,
		},
	}

	address2, err := testStore.UpdateUserAddress(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, address2)

	require.Equal(t, address1.ID, address2.ID)
	require.Equal(t, newContactName, address2.ContactName)
	require.Equal(t, newContactPhone, address2.ContactPhone)
	require.Equal(t, newDetailAddress, address2.DetailAddress)
}

func TestSetDefaultAddress(t *testing.T) {
	user := createRandomUser(t)
	address1 := createRandomUserAddress(t, user)
	address2 := createRandomUserAddress(t, user)

	// 先清除所有默认地址
	err := testStore.SetDefaultAddress(context.Background(), user.ID)
	require.NoError(t, err)

	// 设置address2为默认地址
	updatedAddress2, err := testStore.SetAddressAsDefault(context.Background(), SetAddressAsDefaultParams{
		ID:     address2.ID,
		UserID: user.ID,
	})
	require.NoError(t, err)
	require.True(t, updatedAddress2.IsDefault)

	// 验证address1不是默认地址
	updatedAddress1, err := testStore.GetUserAddress(context.Background(), address1.ID)
	require.NoError(t, err)
	require.False(t, updatedAddress1.IsDefault)
}

func TestDeleteUserAddress(t *testing.T) {
	user := createRandomUser(t)
	address := createRandomUserAddress(t, user)

	err := testStore.DeleteUserAddress(context.Background(), DeleteUserAddressParams{
		ID:     address.ID,
		UserID: user.ID,
	})
	require.NoError(t, err)

	// 验证地址已被删除
	deletedAddress, err := testStore.GetUserAddress(context.Background(), address.ID)
	require.Error(t, err)
	require.Empty(t, deletedAddress)
}
