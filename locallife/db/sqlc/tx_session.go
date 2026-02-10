package db

import (
	"context"
	"fmt"
	"time"
)

// ==================== 刷新会话事务 ====================

// RefreshSessionTxParams contains the input parameters for refreshing a session
type RefreshSessionTxParams struct {
	RefreshToken         string
	RefreshTokenFallback string
	// 新 token 信息（由调用方生成）
	NewAccessToken        string
	NewRefreshToken       string
	AccessTokenExpiresAt  time.Time
	RefreshTokenExpiresAt time.Time
}

// RefreshSessionTxResult contains the result of the refresh session transaction
type RefreshSessionTxResult struct {
	Session Session
}

// RefreshSessionTx atomically locks the session, validates it, and updates tokens.
// P1-012 修复：使用 FOR UPDATE 防止并发刷新导致多个有效 token
func (store *SQLStore) RefreshSessionTx(ctx context.Context, arg RefreshSessionTxParams) (RefreshSessionTxResult, error) {
	var result RefreshSessionTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		// 1. 使用 FOR UPDATE 锁定会话记录，防止并发刷新
		session, err := q.GetSessionByRefreshTokenForUpdate(ctx, GetSessionByRefreshTokenForUpdateParams{
			RefreshToken:         arg.RefreshToken,
			RefreshTokenFallback: arg.RefreshTokenFallback,
		})
		if err != nil {
			return fmt.Errorf("session not found: %w", err)
		}

		// 2. 验证会话有效性
		if session.IsRevoked {
			return fmt.Errorf("session is revoked")
		}
		if time.Now().After(session.RefreshTokenExpiresAt) {
			return fmt.Errorf("refresh token expired")
		}

		// 3. 原子更新 token
		result.Session, err = q.UpdateSessionTokens(ctx, UpdateSessionTokensParams{
			ID:                    session.ID,
			AccessToken:           arg.NewAccessToken,
			RefreshToken:          arg.NewRefreshToken,
			AccessTokenExpiresAt:  arg.AccessTokenExpiresAt,
			RefreshTokenExpiresAt: arg.RefreshTokenExpiresAt,
		})
		if err != nil {
			return fmt.Errorf("update session tokens: %w", err)
		}

		return nil
	})

	return result, err
}
