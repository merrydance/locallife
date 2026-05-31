package logic

import (
	"context"
	"errors"
	"net/http"
	"strings"

	db "github.com/merrydance/locallife/db/sqlc"
)

type AdjustMemberBalanceInput struct {
	MerchantID       int64
	TargetMerchantID int64
	UserID           int64
	Amount           int64
	Notes            string
	IdempotencyKey   string
}

type AdjustMemberBalanceResult struct {
	Membership db.MerchantMembership
	User       db.User
}

func AdjustMemberBalance(ctx context.Context, store db.Store, input AdjustMemberBalanceInput) (AdjustMemberBalanceResult, error) {
	var result AdjustMemberBalanceResult

	if input.Amount == 0 {
		return result, NewRequestError(http.StatusBadRequest, errors.New("amount cannot be zero"))
	}
	idempotencyKey := strings.TrimSpace(input.IdempotencyKey)
	if idempotencyKey == "" {
		return result, NewRequestError(http.StatusBadRequest, errors.New("idempotency key is required"))
	}
	if input.MerchantID != input.TargetMerchantID {
		return result, NewRequestError(http.StatusForbidden, errors.New("not authorized for this merchant"))
	}

	membership, err := store.GetMembershipByMerchantAndUser(ctx, db.GetMembershipByMerchantAndUserParams{
		MerchantID: input.TargetMerchantID,
		UserID:     input.UserID,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("membership not found"))
		}
		return result, err
	}

	adjusted, err := store.AdjustMemberBalanceTx(ctx, db.AdjustMemberBalanceTxParams{
		MembershipID:   membership.ID,
		Amount:         input.Amount,
		Notes:          strings.TrimSpace(input.Notes),
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		if errors.Is(err, db.ErrMembershipBalanceInsufficient) {
			return result, NewRequestErrorWithCause(http.StatusBadRequest, errors.New("会员余额不足"), err)
		}
		if errors.Is(err, db.ErrMembershipAdjustmentIdempotencyConflict) {
			return result, NewRequestErrorWithCause(http.StatusConflict, errors.New("idempotency key already used by a different membership adjustment request"), err)
		}
		return result, err
	}

	user, err := store.GetUser(ctx, input.UserID)
	if err != nil {
		return result, err
	}

	result.Membership = adjusted.Membership
	result.User = user
	return result, nil
}
