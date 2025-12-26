package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ApproveMerchantApplicationTxParams contains input parameters for approving merchant application
type ApproveMerchantApplicationTxParams struct {
	ApplicationID int64
	UserID        int64
	MerchantName  string
	Phone         string
	Address       string
	Latitude      pgtype.Numeric
	Longitude     pgtype.Numeric
	RegionID      int64
	AppData       []byte // JSON 格式的申请数据
}

// ApproveMerchantApplicationTxResult contains the result of approval transaction
type ApproveMerchantApplicationTxResult struct {
	Application MerchantApplication
	Merchant    Merchant
	UserRole    UserRole
}

// ApproveMerchantApplicationTx approves a merchant application, creates merchant record,
// and assigns merchant role to user in a single transaction.
// This ensures atomicity: if any step fails, all changes are rolled back.
func (store *SQLStore) ApproveMerchantApplicationTx(ctx context.Context, arg ApproveMerchantApplicationTxParams) (ApproveMerchantApplicationTxResult, error) {
	var result ApproveMerchantApplicationTxResult

	if arg.RegionID <= 0 {
		return result, fmt.Errorf("invalid region id: %d", arg.RegionID)
	}

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// Step 1: 更新申请状态为 approved
		result.Application, err = q.ApproveMerchantApplication(ctx, arg.ApplicationID)
		if err != nil {
			return fmt.Errorf("approve application: %w", err)
		}

		// Step 2: 创建或更新商户记录
		existingMerchant, err := q.GetMerchantByOwner(ctx, arg.UserID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// 创建新商户
				result.Merchant, err = q.CreateMerchant(ctx, CreateMerchantParams{
					OwnerUserID:     arg.UserID,
					Name:            arg.MerchantName,
					Description:     pgtype.Text{},
					LogoUrl:         pgtype.Text{},
					Phone:           arg.Phone,
					Address:         arg.Address,
					Latitude:        arg.Latitude,
					Longitude:       arg.Longitude,
					Status:          "approved",
					ApplicationData: arg.AppData,
					RegionID:        arg.RegionID,
				})
			} else {
				return fmt.Errorf("get existing merchant: %w", err)
			}
		} else {
			// 更新现有商户
			result.Merchant, err = q.UpdateMerchant(ctx, UpdateMerchantParams{
				ID:        existingMerchant.ID,
				Version:   existingMerchant.Version,
				Name:      pgtype.Text{String: arg.MerchantName, Valid: true},
				Phone:     pgtype.Text{String: arg.Phone, Valid: true},
				Address:   pgtype.Text{String: arg.Address, Valid: true},
				Latitude:  arg.Latitude,
				Longitude: arg.Longitude,
				RegionID:  pgtype.Int8{Int64: arg.RegionID, Valid: true},
			})
			if err == nil {
				// 确保状态也是 approved
				result.Merchant, err = q.UpdateMerchantStatus(ctx, UpdateMerchantStatusParams{
					ID:     existingMerchant.ID,
					Status: "approved",
				})
			}
		}
		if err != nil {
			return fmt.Errorf("create/update merchant: %w", err)
		}

		// Step 3: 创建或更新用户商户角色
		// 检查是否已有该角色
		roles, err := q.ListUserRoles(ctx, arg.UserID)
		if err != nil {
			return fmt.Errorf("list user roles: %w", err)
		}

		hasMerchantRole := false
		for _, r := range roles {
			if r.Role == "merchant" {
				hasMerchantRole = true
				// 如果角色已存在但关联实体 ID 不对，或者状态不是 active，可以在这里更新
				// 但目前 CreateUserRole 足够，如果已存在则跳过
				result.UserRole = r
				break
			}
		}

		if !hasMerchantRole {
			result.UserRole, err = q.CreateUserRole(ctx, CreateUserRoleParams{
				UserID:          arg.UserID,
				Role:            "merchant",
				Status:          "active",
				RelatedEntityID: pgtype.Int8{Int64: result.Merchant.ID, Valid: true},
			})
			if err != nil {
				return fmt.Errorf("create user role: %w", err)
			}
		}

		return nil
	})

	return result, err
}

// ResetMerchantApplicationTxParams contains input parameters for resetting merchant application
type ResetMerchantApplicationTxParams struct {
	ApplicationID int64
	UserID        int64
}

// ResetMerchantApplicationTxResult contains the result of reset transaction
type ResetMerchantApplicationTxResult struct {
	Application MerchantApplication
	Merchant    Merchant
}

// ResetMerchantApplicationTx resets a merchant application to draft status
// and sets the associated merchant status to pending (if exists).
func (store *SQLStore) ResetMerchantApplicationTx(ctx context.Context, arg ResetMerchantApplicationTxParams) (ResetMerchantApplicationTxResult, error) {
	var result ResetMerchantApplicationTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// Step 1: 重置申请状态为 draft
		result.Application, err = q.ResetMerchantApplicationToDraft(ctx, arg.ApplicationID)
		if err != nil {
			return fmt.Errorf("reset application: %w", err)
		}

		// Step 2: 如果商户记录已存在，将其状态改为 pending
		merchant, err := q.GetMerchantByOwner(ctx, arg.UserID)
		if err == nil {
			result.Merchant, err = q.UpdateMerchantStatus(ctx, UpdateMerchantStatusParams{
				ID:     merchant.ID,
				Status: "pending",
			})
			if err != nil {
				return fmt.Errorf("update merchant status: %w", err)
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("get merchant: %w", err)
		}

		return nil
	})

	return result, err
}
