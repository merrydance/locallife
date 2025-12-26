package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func createRandomUser(t *testing.T) User {
	// 使用 RandomString 生成唯一标识符，避免并发测试中手机号重复
	uniqueID := util.RandomString(12)
	arg := CreateUserParams{
		WechatOpenid: fmt.Sprintf("wx_openid_%s", uniqueID),
		WechatUnionid: pgtype.Text{
			String: fmt.Sprintf("wx_unionid_%s", uniqueID),
			Valid:  true,
		},
		FullName: util.RandomOwner(),
		Phone: pgtype.Text{
			String: fmt.Sprintf("138%s", util.RandomString(8)),
			Valid:  true,
		},
		AvatarUrl: pgtype.Text{
			String: "https://example.com/avatar.jpg",
			Valid:  true,
		},
	}

	user, err := testStore.CreateUser(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, user)

	require.Equal(t, arg.WechatOpenid, user.WechatOpenid)
	require.Equal(t, arg.FullName, user.FullName)
	require.NotZero(t, user.ID)
	require.NotZero(t, user.CreatedAt)

	return user
}

func TestCreateUser(t *testing.T) {
	createRandomUser(t)
}

func TestGetUser(t *testing.T) {
	user1 := createRandomUser(t)
	user2, err := testStore.GetUser(context.Background(), user1.ID)
	require.NoError(t, err)
	require.NotEmpty(t, user2)

	require.Equal(t, user1.ID, user2.ID)
	require.Equal(t, user1.WechatOpenid, user2.WechatOpenid)
	require.Equal(t, user1.FullName, user2.FullName)
	require.WithinDuration(t, user1.CreatedAt, user2.CreatedAt, time.Second)
}

func TestGetUserByWechatOpenID(t *testing.T) {
	user1 := createRandomUser(t)
	user2, err := testStore.GetUserByWechatOpenID(context.Background(), user1.WechatOpenid)
	require.NoError(t, err)
	require.NotEmpty(t, user2)

	require.Equal(t, user1.ID, user2.ID)
	require.Equal(t, user1.WechatOpenid, user2.WechatOpenid)
}

func TestGetUserByPhone(t *testing.T) {
	user1 := createRandomUser(t)
	user2, err := testStore.GetUserByPhone(context.Background(), user1.Phone)
	require.NoError(t, err)
	require.NotEmpty(t, user2)

	require.Equal(t, user1.ID, user2.ID)
	require.Equal(t, user1.Phone, user2.Phone)
}

func TestUpdateUserFullName(t *testing.T) {
	user1 := createRandomUser(t)

	newFullName := util.RandomOwner()
	arg := UpdateUserParams{
		ID: user1.ID,
		FullName: pgtype.Text{
			String: newFullName,
			Valid:  true,
		},
	}

	user2, err := testStore.UpdateUser(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, user2)

	require.Equal(t, user1.ID, user2.ID)
	require.Equal(t, user1.WechatOpenid, user2.WechatOpenid)
	require.Equal(t, newFullName, user2.FullName)
}

func TestUpdateUserPhone(t *testing.T) {
	user1 := createRandomUser(t)

	newPhone := fmt.Sprintf("139%08d", util.RandomInt(10000000, 99999999))
	arg := UpdateUserParams{
		ID: user1.ID,
		Phone: pgtype.Text{
			String: newPhone,
			Valid:  true,
		},
	}

	user2, err := testStore.UpdateUser(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, user2)

	require.Equal(t, user1.ID, user2.ID)
	require.Equal(t, newPhone, user2.Phone.String)
	require.Equal(t, user1.FullName, user2.FullName)
}

func TestUpdateUserAvatar(t *testing.T) {
	user1 := createRandomUser(t)

	newAvatar := "https://example.com/new-avatar.jpg"
	arg := UpdateUserParams{
		ID: user1.ID,
		AvatarUrl: pgtype.Text{
			String: newAvatar,
			Valid:  true,
		},
	}

	user2, err := testStore.UpdateUser(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, user2)

	require.Equal(t, user1.ID, user2.ID)
	require.Equal(t, newAvatar, user2.AvatarUrl.String)
	require.Equal(t, user1.FullName, user2.FullName)
}
