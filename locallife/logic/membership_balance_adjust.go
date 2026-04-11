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
		MembershipID: membership.ID,
		Amount:       input.Amount,
		Notes:        input.Notes,
	})
	if err != nil {
		if strings.Contains(err.Error(), "余额不足") {
			return result, NewRequestError(http.StatusBadRequest, err)
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
