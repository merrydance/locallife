package db

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func upsertRoleForExistingUser(t *testing.T, userID int64, role string) {
	t.Helper()

	ctx := context.Background()

	_, err := testStore.GetUser(ctx, userID)
	require.NoErrorf(t, err, "user %d does not exist; this test only updates existing users", userID)

	existingRole, err := testStore.GetUserRoleByType(ctx, GetUserRoleByTypeParams{
		UserID: userID,
		Role:   role,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			created, createErr := testStore.CreateUserRole(ctx, CreateUserRoleParams{
				UserID:          userID,
				Role:            role,
				Status:          "active",
				RelatedEntityID: pgtype.Int8{},
			})
			require.NoErrorf(t, createErr, "create role %s for user %d", role, userID)
			require.Equal(t, "active", created.Status)
			return
		}
		require.NoErrorf(t, err, "query role %s for user %d", role, userID)
	}

	if existingRole.Status != "active" {
		updated, updateErr := testStore.UpdateUserRoleStatus(ctx, UpdateUserRoleStatusParams{
			ID:     existingRole.ID,
			Status: "active",
		})
		require.NoErrorf(t, updateErr, "activate role %s for user %d", role, userID)
		require.Equal(t, "active", updated.Status)
	}
}

func TestAssignExistingUsersRoles(t *testing.T) {
	upsertRoleForExistingUser(t, 1, "operator")
	upsertRoleForExistingUser(t, 2, "admin")

	ctx := context.Background()

	opRole, err := testStore.GetUserRoleByType(ctx, GetUserRoleByTypeParams{UserID: 1, Role: "operator"})
	require.NoError(t, err)
	require.Equal(t, "active", opRole.Status)

	adminRole, err := testStore.GetUserRoleByType(ctx, GetUserRoleByTypeParams{UserID: 2, Role: "admin"})
	require.NoError(t, err)
	require.Equal(t, "active", adminRole.Status)
}
