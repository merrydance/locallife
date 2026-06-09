package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrMerchantStaffMerchantMismatch = errors.New("merchant staff does not belong to merchant")
	ErrMerchantStaffOwnerMutation    = errors.New("cannot mutate merchant owner staff")
)

type AddMerchantStaffTxParams struct {
	MerchantID int64
	UserID     int64
	Role       string
	InvitedBy  pgtype.Int8
}

type AddMerchantStaffTxResult struct {
	Staff MerchantStaff
}

type AssignMerchantStaffRoleTxParams struct {
	MerchantID int64
	StaffID    int64
	Role       string
}

type AssignMerchantStaffRoleTxResult struct {
	Staff MerchantStaff
}

type RemoveMerchantStaffTxParams struct {
	MerchantID int64
	StaffID    int64
}

type RemoveMerchantStaffTxResult struct {
	Staff MerchantStaff
}

func (store *SQLStore) AddMerchantStaffTx(ctx context.Context, arg AddMerchantStaffTxParams) (AddMerchantStaffTxResult, error) {
	var result AddMerchantStaffTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		staff, err := q.CreateMerchantStaff(ctx, CreateMerchantStaffParams{
			MerchantID: arg.MerchantID,
			UserID:     arg.UserID,
			Role:       arg.Role,
			Status:     MerchantStaffStatusActive,
			InvitedBy:  arg.InvitedBy,
		})
		if err != nil {
			return err
		}

		if staff.Role != MerchantStaffRolePending {
			if err := q.ensureMerchantStaffUserRoleActive(ctx, staff.UserID, staff.MerchantID); err != nil {
				return err
			}
		}

		result.Staff = staff
		return nil
	})

	return result, err
}

func (store *SQLStore) AssignMerchantStaffRoleTx(ctx context.Context, arg AssignMerchantStaffRoleTxParams) (AssignMerchantStaffRoleTxResult, error) {
	var result AssignMerchantStaffRoleTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		staff, err := q.GetMerchantStaffForUpdate(ctx, arg.StaffID)
		if err != nil {
			return err
		}
		if staff.MerchantID != arg.MerchantID {
			return ErrMerchantStaffMerchantMismatch
		}
		if staff.Role == MerchantStaffRoleOwner {
			return ErrMerchantStaffOwnerMutation
		}

		updatedStaff, err := q.UpdateMerchantStaffRole(ctx, UpdateMerchantStaffRoleParams{
			ID:   staff.ID,
			Role: arg.Role,
		})
		if err != nil {
			return err
		}

		if updatedStaff.Role != MerchantStaffRolePending {
			if err := q.ensureMerchantStaffUserRoleActive(ctx, updatedStaff.UserID, updatedStaff.MerchantID); err != nil {
				return err
			}
		} else if err := q.disableMerchantStaffUserRoleIfNoAssignedActiveStaff(ctx, updatedStaff.UserID); err != nil {
			return err
		}

		result.Staff = updatedStaff
		return nil
	})

	return result, err
}

func (store *SQLStore) RemoveMerchantStaffTx(ctx context.Context, arg RemoveMerchantStaffTxParams) (RemoveMerchantStaffTxResult, error) {
	var result RemoveMerchantStaffTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		staff, err := q.GetMerchantStaffForUpdate(ctx, arg.StaffID)
		if err != nil {
			return err
		}
		if staff.MerchantID != arg.MerchantID {
			return ErrMerchantStaffMerchantMismatch
		}
		if staff.Role == MerchantStaffRoleOwner {
			return ErrMerchantStaffOwnerMutation
		}

		disabledStaff, err := q.SoftDeleteMerchantStaff(ctx, staff.ID)
		if err != nil {
			return err
		}

		if err := q.disableMerchantStaffUserRoleIfNoAssignedActiveStaff(ctx, disabledStaff.UserID); err != nil {
			return err
		}

		result.Staff = disabledStaff
		return nil
	})

	return result, err
}

func (q *Queries) ensureMerchantStaffUserRoleActive(ctx context.Context, userID, merchantID int64) error {
	_, err := q.UpsertUserRoleActive(ctx, UpsertUserRoleActiveParams{
		UserID:          userID,
		Role:            UserRoleMerchantStaff,
		RelatedEntityID: pgtype.Int8{Int64: merchantID, Valid: true},
	})
	return err
}

func (q *Queries) disableMerchantStaffUserRoleIfNoAssignedActiveStaff(ctx context.Context, userID int64) error {
	userRole, err := q.GetUserRoleByTypeForUpdate(ctx, GetUserRoleByTypeForUpdateParams{
		UserID: userID,
		Role:   UserRoleMerchantStaff,
	})
	if err != nil {
		if errors.Is(err, ErrRecordNotFound) {
			return nil
		}
		return err
	}

	if userRole.Status != UserRoleStatusActive {
		return nil
	}

	count, err := q.CountAssignedActiveMerchantStaffByUser(ctx, userID)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	_, err = q.UpdateUserRoleStatus(ctx, UpdateUserRoleStatusParams{
		ID:     userRole.ID,
		Status: UserRoleStatusDisabled,
	})
	return err
}
