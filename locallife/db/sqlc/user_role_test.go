package db

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func createRandomUserRole(t *testing.T, role string) UserRole {
	user := createRandomUser(t)
	return createRandomUserRoleForUser(t, user.ID, role)
}

func createRandomUserRoleForUser(t *testing.T, userID int64, role string) UserRole {
	arg := CreateUserRoleParams{
		UserID:          userID,
		Role:            role,
		Status:          "active",
		RelatedEntityID: pgtype.Int8{},
	}

	userRole, err := testStore.CreateUserRole(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, userRole)

	require.Equal(t, arg.UserID, userRole.UserID)
	require.Equal(t, arg.Role, userRole.Role)
	require.Equal(t, "active", userRole.Status)
	require.NotZero(t, userRole.ID)
	require.NotZero(t, userRole.CreatedAt)

	return userRole
}

func TestCreateUserRole(t *testing.T) {
	createRandomUserRole(t, "customer")
}

func TestGetUserRole(t *testing.T) {
	role1 := createRandomUserRole(t, "customer")

	role2, err := testStore.GetUserRole(context.Background(), role1.ID)
	require.NoError(t, err)
	require.NotEmpty(t, role2)

	require.Equal(t, role1.ID, role2.ID)
	require.Equal(t, role1.UserID, role2.UserID)
	require.Equal(t, role1.Role, role2.Role)
}

func TestGetUserRoleByType(t *testing.T) {
	user := createRandomUser(t)
	role1 := createRandomUserRoleForUser(t, user.ID, "merchant")

	arg := GetUserRoleByTypeParams{
		UserID: user.ID,
		Role:   "merchant",
	}

	role2, err := testStore.GetUserRoleByType(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, role2)

	require.Equal(t, role1.ID, role2.ID)
	require.Equal(t, "merchant", role2.Role)
}

func TestHasRole(t *testing.T) {
	user := createRandomUser(t)

	// 创建角色前检查
	arg := HasRoleParams{
		UserID: user.ID,
		Role:   "operator",
	}
	hasRole, err := testStore.HasRole(context.Background(), arg)
	require.NoError(t, err)
	require.False(t, hasRole)

	// 创建角色
	createRandomUserRoleForUser(t, user.ID, "operator")

	// 创建角色后检查
	hasRole, err = testStore.HasRole(context.Background(), arg)
	require.NoError(t, err)
	require.True(t, hasRole)
}

func TestListUserRoles(t *testing.T) {
	user := createRandomUser(t)

	// 创建多个角色
	roles := []string{"customer", "merchant", "rider"}
	for _, role := range roles {
		createRandomUserRoleForUser(t, user.ID, role)
	}

	userRoles, err := testStore.ListUserRoles(context.Background(), user.ID)
	require.NoError(t, err)
	require.Len(t, userRoles, 3)

	for _, r := range userRoles {
		require.Equal(t, user.ID, r.UserID)
		require.Equal(t, "active", r.Status)
	}
}

func TestUpdateUserRoleStatus(t *testing.T) {
	role1 := createRandomUserRole(t, "customer")

	arg := UpdateUserRoleStatusParams{
		ID:     role1.ID,
		Status: "inactive",
	}

	role2, err := testStore.UpdateUserRoleStatus(context.Background(), arg)
	require.NoError(t, err)
	require.Equal(t, "inactive", role2.Status)
}

func TestDeleteUserRole(t *testing.T) {
	role := createRandomUserRole(t, "customer")

	err := testStore.DeleteUserRole(context.Background(), role.ID)
	require.NoError(t, err)

	// 验证已删除
	_, err = testStore.GetUserRole(context.Background(), role.ID)
	require.Error(t, err)
}

func TestCreateUserRoleWithRelatedEntity(t *testing.T) {
	user := createRandomUser(t)
	merchant := createRandomMerchantForTest(t)

	arg := CreateUserRoleParams{
		UserID:          user.ID,
		Role:            "merchant",
		Status:          "active",
		RelatedEntityID: pgtype.Int8{Int64: merchant.ID, Valid: true},
	}

	role, err := testStore.CreateUserRole(context.Background(), arg)
	require.NoError(t, err)
	require.True(t, role.RelatedEntityID.Valid)
	require.Equal(t, merchant.ID, role.RelatedEntityID.Int64)
}
