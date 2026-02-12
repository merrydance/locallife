package logic

import (
	"context"
	"errors"
	"net/http"

	db "github.com/merrydance/locallife/db/sqlc"
)

type MembershipRechargeInput struct {
	UserID         int64
	MembershipID   int64
	RechargeAmount int64
}

type MembershipRechargeResult struct {
	Membership     db.MerchantMembership
	BonusAmount    int64
	RechargeRuleID *int64
}

func PrepareMembershipRecharge(ctx context.Context, store db.Store, input MembershipRechargeInput) (MembershipRechargeResult, error) {
	var result MembershipRechargeResult

	membership, err := store.GetMerchantMembership(ctx, input.MembershipID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("membership not found"))
		}
		return result, err
	}

	if membership.UserID != input.UserID {
		return result, NewRequestError(http.StatusForbidden, errors.New("not authorized"))
	}

	matchingRule, err := store.GetMatchingRechargeRule(ctx, db.GetMatchingRechargeRuleParams{
		MerchantID:     membership.MerchantID,
		RechargeAmount: input.RechargeAmount,
	})
	if err == nil {
		result.BonusAmount = matchingRule.BonusAmount
		result.RechargeRuleID = &matchingRule.ID
	} else if !errors.Is(err, db.ErrRecordNotFound) {
		return result, err
	}

	result.Membership = membership
	return result, nil
}
