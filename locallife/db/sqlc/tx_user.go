package db

import (
	"context"
	"fmt"
)

// CreateUserTxParams contains the input parameters for creating a new user with role
type CreateUserTxParams struct {
	WechatOpenid string
	FullName     string
	DefaultRole  string // default role to assign, e.g. "customer"
}

// CreateUserTxResult contains the result of user creation transaction
type CreateUserTxResult struct {
	User     User
	UserRole UserRole
}

// CreateUserTx creates a new user and assigns a default role in a single transaction.
// This ensures atomicity: if role creation fails, the user is not created either.
func (store *SQLStore) CreateUserTx(ctx context.Context, arg CreateUserTxParams) (CreateUserTxResult, error) {
	var result CreateUserTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// Step 1: Create user
		result.User, err = q.CreateUser(ctx, CreateUserParams{
			WechatOpenid: arg.WechatOpenid,
			FullName:     arg.FullName,
		})
		if err != nil {
			return fmt.Errorf("create user: %w", err)
		}

		// Step 2: Create default user role
		result.UserRole, err = q.CreateUserRole(ctx, CreateUserRoleParams{
			UserID: result.User.ID,
			Role:   arg.DefaultRole,
			Status: "active",
		})
		if err != nil {
			return fmt.Errorf("create user role: %w", err)
		}

		return nil
	})

	return result, err
}
