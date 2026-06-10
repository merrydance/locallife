package db

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestCheckUserHasMerchantAccessRejectsPendingStaffRole(t *testing.T) {
	ctx := context.Background()
	merchant := createRandomMerchantForTest(t)
	user := createRandomUser(t)

	_, err := testStore.CreateMerchantStaff(ctx, CreateMerchantStaffParams{
		MerchantID: merchant.ID,
		UserID:     user.ID,
		Role:       "pending",
		Status:     MerchantStaffStatusActive,
		InvitedBy:  pgtype.Int8{Int64: merchant.OwnerUserID, Valid: true},
	})
	require.NoError(t, err)

	hasAccess, err := testStore.CheckUserHasMerchantAccess(ctx, CheckUserHasMerchantAccessParams{
		MerchantID: merchant.ID,
		UserID:     user.ID,
	})
	require.NoError(t, err)
	require.False(t, hasAccess)
}

func TestGetUserMerchantRoleReturnsPendingForAssignmentDisplay(t *testing.T) {
	ctx := context.Background()
	merchant := createRandomMerchantForTest(t)
	user := createRandomUser(t)

	_, err := testStore.CreateMerchantStaff(ctx, CreateMerchantStaffParams{
		MerchantID: merchant.ID,
		UserID:     user.ID,
		Role:       MerchantStaffRolePending,
		Status:     MerchantStaffStatusActive,
		InvitedBy:  pgtype.Int8{Int64: merchant.OwnerUserID, Valid: true},
	})
	require.NoError(t, err)

	role, err := testStore.GetUserMerchantRole(ctx, GetUserMerchantRoleParams{
		MerchantID: merchant.ID,
		UserID:     user.ID,
	})
	require.NoError(t, err)
	require.Equal(t, MerchantStaffRolePending, role)
}

func TestCountMerchantStaffExcludesPendingStaffRole(t *testing.T) {
	ctx := context.Background()
	merchant := createRandomMerchantForTest(t)
	pendingUser := createRandomUser(t)
	assignedUser := createRandomUser(t)

	_, err := testStore.CreateMerchantStaff(ctx, CreateMerchantStaffParams{
		MerchantID: merchant.ID,
		UserID:     pendingUser.ID,
		Role:       MerchantStaffRolePending,
		Status:     MerchantStaffStatusActive,
		InvitedBy:  pgtype.Int8{Int64: merchant.OwnerUserID, Valid: true},
	})
	require.NoError(t, err)
	_, err = testStore.CreateMerchantStaff(ctx, CreateMerchantStaffParams{
		MerchantID: merchant.ID,
		UserID:     assignedUser.ID,
		Role:       MerchantStaffRoleCashier,
		Status:     MerchantStaffStatusActive,
		InvitedBy:  pgtype.Int8{Int64: merchant.OwnerUserID, Valid: true},
	})
	require.NoError(t, err)

	count, err := testStore.CountMerchantStaff(ctx, merchant.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)
}

func TestCheckUserHasMerchantAccessAllowsAssignedActiveStaffRole(t *testing.T) {
	ctx := context.Background()
	merchant := createRandomMerchantForTest(t)
	user := createRandomUser(t)

	_, err := testStore.CreateMerchantStaff(ctx, CreateMerchantStaffParams{
		MerchantID: merchant.ID,
		UserID:     user.ID,
		Role:       "cashier",
		Status:     MerchantStaffStatusActive,
		InvitedBy:  pgtype.Int8{Int64: merchant.OwnerUserID, Valid: true},
	})
	require.NoError(t, err)

	hasAccess, err := testStore.CheckUserHasMerchantAccess(ctx, CheckUserHasMerchantAccessParams{
		MerchantID: merchant.ID,
		UserID:     user.ID,
	})
	require.NoError(t, err)
	require.True(t, hasAccess)
}

func TestAddMerchantStaffTxDoesNotGrantCoarseRoleForPendingStaff(t *testing.T) {
	ctx := context.Background()
	merchant := createRandomMerchantForTest(t)
	user := createRandomUser(t)

	result, err := testStore.AddMerchantStaffTx(ctx, AddMerchantStaffTxParams{
		MerchantID: merchant.ID,
		UserID:     user.ID,
		Role:       MerchantStaffRolePending,
		InvitedBy:  pgtype.Int8{Int64: merchant.OwnerUserID, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, MerchantStaffRolePending, result.Staff.Role)
	require.Equal(t, MerchantStaffStatusActive, result.Staff.Status)

	_, err = testStore.GetUserRoleByType(ctx, GetUserRoleByTypeParams{
		UserID: user.ID,
		Role:   UserRoleMerchantStaff,
	})
	require.ErrorIs(t, err, ErrRecordNotFound)
}

func TestAddMerchantStaffTxReactivatesDisabledStaffAsPendingWithoutCoarseRole(t *testing.T) {
	ctx := context.Background()
	merchant := createRandomMerchantForTest(t)
	user := createRandomUser(t)

	disabledStaff, err := testStore.CreateMerchantStaff(ctx, CreateMerchantStaffParams{
		MerchantID: merchant.ID,
		UserID:     user.ID,
		Role:       MerchantStaffRoleCashier,
		Status:     MerchantStaffStatusDisabled,
		InvitedBy:  pgtype.Int8{Int64: merchant.OwnerUserID, Valid: true},
	})
	require.NoError(t, err)
	_, err = testStore.CreateUserRole(ctx, CreateUserRoleParams{
		UserID:          user.ID,
		Role:            UserRoleMerchantStaff,
		Status:          UserRoleStatusDisabled,
		RelatedEntityID: pgtype.Int8{Int64: merchant.ID, Valid: true},
	})
	require.NoError(t, err)

	result, err := testStore.AddMerchantStaffTx(ctx, AddMerchantStaffTxParams{
		MerchantID: merchant.ID,
		UserID:     user.ID,
		Role:       MerchantStaffRolePending,
		InvitedBy:  pgtype.Int8{Int64: merchant.OwnerUserID, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, disabledStaff.ID, result.Staff.ID)
	require.Equal(t, MerchantStaffRolePending, result.Staff.Role)
	require.Equal(t, MerchantStaffStatusActive, result.Staff.Status)

	userRole, err := testStore.GetUserRoleByType(ctx, GetUserRoleByTypeParams{
		UserID: user.ID,
		Role:   UserRoleMerchantStaff,
	})
	require.NoError(t, err)
	require.Equal(t, UserRoleStatusDisabled, userRole.Status)
}

func TestAddMerchantStaffTxGrantsCoarseRoleForAssignedStaff(t *testing.T) {
	ctx := context.Background()
	merchant := createRandomMerchantForTest(t)
	user := createRandomUser(t)

	result, err := testStore.AddMerchantStaffTx(ctx, AddMerchantStaffTxParams{
		MerchantID: merchant.ID,
		UserID:     user.ID,
		Role:       MerchantStaffRoleCashier,
		InvitedBy:  pgtype.Int8{Int64: merchant.OwnerUserID, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, MerchantStaffRoleCashier, result.Staff.Role)

	userRole, err := testStore.GetUserRoleByType(ctx, GetUserRoleByTypeParams{
		UserID: user.ID,
		Role:   UserRoleMerchantStaff,
	})
	require.NoError(t, err)
	require.Equal(t, UserRoleStatusActive, userRole.Status)
	require.True(t, userRole.RelatedEntityID.Valid)
	require.Equal(t, merchant.ID, userRole.RelatedEntityID.Int64)
}

func TestAddMerchantStaffTxReactivatesDisabledStaffWithAssignedRole(t *testing.T) {
	ctx := context.Background()
	merchant := createRandomMerchantForTest(t)
	user := createRandomUser(t)

	disabledStaff, err := testStore.CreateMerchantStaff(ctx, CreateMerchantStaffParams{
		MerchantID: merchant.ID,
		UserID:     user.ID,
		Role:       MerchantStaffRoleCashier,
		Status:     MerchantStaffStatusDisabled,
		InvitedBy:  pgtype.Int8{Int64: merchant.OwnerUserID, Valid: true},
	})
	require.NoError(t, err)
	_, err = testStore.CreateUserRole(ctx, CreateUserRoleParams{
		UserID:          user.ID,
		Role:            UserRoleMerchantStaff,
		Status:          UserRoleStatusDisabled,
		RelatedEntityID: pgtype.Int8{Int64: merchant.ID, Valid: true},
	})
	require.NoError(t, err)

	result, err := testStore.AddMerchantStaffTx(ctx, AddMerchantStaffTxParams{
		MerchantID: merchant.ID,
		UserID:     user.ID,
		Role:       MerchantStaffRoleManager,
		InvitedBy:  pgtype.Int8{Int64: merchant.OwnerUserID, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, disabledStaff.ID, result.Staff.ID)
	require.Equal(t, MerchantStaffRoleManager, result.Staff.Role)
	require.Equal(t, MerchantStaffStatusActive, result.Staff.Status)

	userRole, err := testStore.GetUserRoleByType(ctx, GetUserRoleByTypeParams{
		UserID: user.ID,
		Role:   UserRoleMerchantStaff,
	})
	require.NoError(t, err)
	require.Equal(t, UserRoleStatusActive, userRole.Status)
	require.True(t, userRole.RelatedEntityID.Valid)
	require.Equal(t, merchant.ID, userRole.RelatedEntityID.Int64)
}

func TestAddMerchantStaffTxRejectsExistingActiveOrPendingStaff(t *testing.T) {
	tests := []struct {
		name string
		role string
	}{
		{name: "active assigned", role: MerchantStaffRoleCashier},
		{name: "active pending", role: MerchantStaffRolePending},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			merchant := createRandomMerchantForTest(t)
			user := createRandomUser(t)

			existingStaff, err := testStore.CreateMerchantStaff(ctx, CreateMerchantStaffParams{
				MerchantID: merchant.ID,
				UserID:     user.ID,
				Role:       tc.role,
				Status:     MerchantStaffStatusActive,
				InvitedBy:  pgtype.Int8{Int64: merchant.OwnerUserID, Valid: true},
			})
			require.NoError(t, err)

			_, err = testStore.AddMerchantStaffTx(ctx, AddMerchantStaffTxParams{
				MerchantID: merchant.ID,
				UserID:     user.ID,
				Role:       MerchantStaffRoleManager,
				InvitedBy:  pgtype.Int8{Int64: merchant.OwnerUserID, Valid: true},
			})
			require.ErrorIs(t, err, ErrMerchantStaffAlreadyExists)

			staff, err := testStore.GetMerchantStaff(ctx, GetMerchantStaffParams{
				MerchantID: merchant.ID,
				UserID:     user.ID,
			})
			require.NoError(t, err)
			require.Equal(t, existingStaff.ID, staff.ID)
			require.Equal(t, tc.role, staff.Role)
			require.Equal(t, MerchantStaffStatusActive, staff.Status)
		})
	}
}

func TestAddMerchantStaffTxReactivatesDisabledCoarseRole(t *testing.T) {
	ctx := context.Background()
	merchant := createRandomMerchantForTest(t)
	previousMerchant := createRandomMerchantForTest(t)
	user := createRandomUser(t)

	disabledRole, err := testStore.CreateUserRole(ctx, CreateUserRoleParams{
		UserID:          user.ID,
		Role:            UserRoleMerchantStaff,
		Status:          UserRoleStatusDisabled,
		RelatedEntityID: pgtype.Int8{Int64: previousMerchant.ID, Valid: true},
	})
	require.NoError(t, err)

	_, err = testStore.AddMerchantStaffTx(ctx, AddMerchantStaffTxParams{
		MerchantID: merchant.ID,
		UserID:     user.ID,
		Role:       MerchantStaffRoleManager,
		InvitedBy:  pgtype.Int8{Int64: merchant.OwnerUserID, Valid: true},
	})
	require.NoError(t, err)

	userRole, err := testStore.GetUserRoleByType(ctx, GetUserRoleByTypeParams{
		UserID: user.ID,
		Role:   UserRoleMerchantStaff,
	})
	require.NoError(t, err)
	require.Equal(t, disabledRole.ID, userRole.ID)
	require.Equal(t, UserRoleStatusActive, userRole.Status)
	require.True(t, userRole.RelatedEntityID.Valid)
	require.Equal(t, merchant.ID, userRole.RelatedEntityID.Int64)
}

func TestAssignMerchantStaffRoleTxActivatesCoarseRoleForPendingStaff(t *testing.T) {
	ctx := context.Background()
	merchant := createRandomMerchantForTest(t)
	user := createRandomUser(t)

	pendingStaff, err := testStore.CreateMerchantStaff(ctx, CreateMerchantStaffParams{
		MerchantID: merchant.ID,
		UserID:     user.ID,
		Role:       MerchantStaffRolePending,
		Status:     MerchantStaffStatusActive,
		InvitedBy:  pgtype.Int8{Int64: merchant.OwnerUserID, Valid: true},
	})
	require.NoError(t, err)

	result, err := testStore.AssignMerchantStaffRoleTx(ctx, AssignMerchantStaffRoleTxParams{
		MerchantID: merchant.ID,
		StaffID:    pendingStaff.ID,
		Role:       MerchantStaffRoleChef,
	})
	require.NoError(t, err)
	require.Equal(t, MerchantStaffRoleChef, result.Staff.Role)
	require.Equal(t, MerchantStaffStatusActive, result.Staff.Status)

	userRole, err := testStore.GetUserRoleByType(ctx, GetUserRoleByTypeParams{
		UserID: user.ID,
		Role:   UserRoleMerchantStaff,
	})
	require.NoError(t, err)
	require.Equal(t, UserRoleStatusActive, userRole.Status)
}

func TestAssignMerchantStaffRoleTxReactivatesDisabledCoarseRole(t *testing.T) {
	ctx := context.Background()
	merchant := createRandomMerchantForTest(t)
	user := createRandomUser(t)

	pendingStaff, err := testStore.CreateMerchantStaff(ctx, CreateMerchantStaffParams{
		MerchantID: merchant.ID,
		UserID:     user.ID,
		Role:       MerchantStaffRolePending,
		Status:     MerchantStaffStatusActive,
		InvitedBy:  pgtype.Int8{Int64: merchant.OwnerUserID, Valid: true},
	})
	require.NoError(t, err)
	_, err = testStore.CreateUserRole(ctx, CreateUserRoleParams{
		UserID:          user.ID,
		Role:            UserRoleMerchantStaff,
		Status:          UserRoleStatusDisabled,
		RelatedEntityID: pgtype.Int8{Int64: merchant.ID, Valid: true},
	})
	require.NoError(t, err)

	_, err = testStore.AssignMerchantStaffRoleTx(ctx, AssignMerchantStaffRoleTxParams{
		MerchantID: merchant.ID,
		StaffID:    pendingStaff.ID,
		Role:       MerchantStaffRoleChef,
	})
	require.NoError(t, err)

	userRole, err := testStore.GetUserRoleByType(ctx, GetUserRoleByTypeParams{
		UserID: user.ID,
		Role:   UserRoleMerchantStaff,
	})
	require.NoError(t, err)
	require.Equal(t, UserRoleStatusActive, userRole.Status)
}

func TestRemoveMerchantStaffTxDisablesCoarseRoleWhenNoAssignedStaffRemain(t *testing.T) {
	ctx := context.Background()
	merchant := createRandomMerchantForTest(t)
	user := createRandomUser(t)

	added, err := testStore.AddMerchantStaffTx(ctx, AddMerchantStaffTxParams{
		MerchantID: merchant.ID,
		UserID:     user.ID,
		Role:       MerchantStaffRoleCashier,
		InvitedBy:  pgtype.Int8{Int64: merchant.OwnerUserID, Valid: true},
	})
	require.NoError(t, err)

	result, err := testStore.RemoveMerchantStaffTx(ctx, RemoveMerchantStaffTxParams{
		MerchantID: merchant.ID,
		StaffID:    added.Staff.ID,
	})
	require.NoError(t, err)
	require.Equal(t, MerchantStaffStatusDisabled, result.Staff.Status)

	userRole, err := testStore.GetUserRoleByType(ctx, GetUserRoleByTypeParams{
		UserID: user.ID,
		Role:   UserRoleMerchantStaff,
	})
	require.NoError(t, err)
	require.Equal(t, UserRoleStatusDisabled, userRole.Status)
}

func TestRemoveMerchantStaffTxDisablesCoarseRoleWhenOnlyPendingStaffRemain(t *testing.T) {
	ctx := context.Background()
	assignedMerchant := createRandomMerchantForTest(t)
	pendingMerchant := createRandomMerchantForTest(t)
	user := createRandomUser(t)

	added, err := testStore.AddMerchantStaffTx(ctx, AddMerchantStaffTxParams{
		MerchantID: assignedMerchant.ID,
		UserID:     user.ID,
		Role:       MerchantStaffRoleCashier,
		InvitedBy:  pgtype.Int8{Int64: assignedMerchant.OwnerUserID, Valid: true},
	})
	require.NoError(t, err)
	_, err = testStore.AddMerchantStaffTx(ctx, AddMerchantStaffTxParams{
		MerchantID: pendingMerchant.ID,
		UserID:     user.ID,
		Role:       MerchantStaffRolePending,
		InvitedBy:  pgtype.Int8{Int64: pendingMerchant.OwnerUserID, Valid: true},
	})
	require.NoError(t, err)

	_, err = testStore.RemoveMerchantStaffTx(ctx, RemoveMerchantStaffTxParams{
		MerchantID: assignedMerchant.ID,
		StaffID:    added.Staff.ID,
	})
	require.NoError(t, err)

	userRole, err := testStore.GetUserRoleByType(ctx, GetUserRoleByTypeParams{
		UserID: user.ID,
		Role:   UserRoleMerchantStaff,
	})
	require.NoError(t, err)
	require.Equal(t, UserRoleStatusDisabled, userRole.Status)
}

func TestRemoveMerchantStaffTxKeepsCoarseRoleWhenOtherAssignedStaffRemain(t *testing.T) {
	ctx := context.Background()
	merchant1 := createRandomMerchantForTest(t)
	merchant2 := createRandomMerchantForTest(t)
	user := createRandomUser(t)

	added1, err := testStore.AddMerchantStaffTx(ctx, AddMerchantStaffTxParams{
		MerchantID: merchant1.ID,
		UserID:     user.ID,
		Role:       MerchantStaffRoleCashier,
		InvitedBy:  pgtype.Int8{Int64: merchant1.OwnerUserID, Valid: true},
	})
	require.NoError(t, err)
	_, err = testStore.AddMerchantStaffTx(ctx, AddMerchantStaffTxParams{
		MerchantID: merchant2.ID,
		UserID:     user.ID,
		Role:       MerchantStaffRoleChef,
		InvitedBy:  pgtype.Int8{Int64: merchant2.OwnerUserID, Valid: true},
	})
	require.NoError(t, err)

	_, err = testStore.RemoveMerchantStaffTx(ctx, RemoveMerchantStaffTxParams{
		MerchantID: merchant1.ID,
		StaffID:    added1.Staff.ID,
	})
	require.NoError(t, err)

	userRole, err := testStore.GetUserRoleByType(ctx, GetUserRoleByTypeParams{
		UserID: user.ID,
		Role:   UserRoleMerchantStaff,
	})
	require.NoError(t, err)
	require.Equal(t, UserRoleStatusActive, userRole.Status)
}

func TestMerchantStaffTxRejectsOwnerMutation(t *testing.T) {
	ctx := context.Background()
	merchant := createRandomMerchantForTest(t)

	ownerStaff, err := testStore.CreateMerchantStaff(ctx, CreateMerchantStaffParams{
		MerchantID: merchant.ID,
		UserID:     merchant.OwnerUserID,
		Role:       MerchantStaffRoleOwner,
		Status:     MerchantStaffStatusActive,
		InvitedBy:  pgtype.Int8{Valid: false},
	})
	require.NoError(t, err)

	_, err = testStore.AssignMerchantStaffRoleTx(ctx, AssignMerchantStaffRoleTxParams{
		MerchantID: merchant.ID,
		StaffID:    ownerStaff.ID,
		Role:       MerchantStaffRoleManager,
	})
	require.ErrorIs(t, err, ErrMerchantStaffOwnerMutation)

	_, err = testStore.RemoveMerchantStaffTx(ctx, RemoveMerchantStaffTxParams{
		MerchantID: merchant.ID,
		StaffID:    ownerStaff.ID,
	})
	require.ErrorIs(t, err, ErrMerchantStaffOwnerMutation)
}
