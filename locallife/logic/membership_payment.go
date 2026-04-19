package logic

import (
	"context"
	"errors"
	"net/http"

	db "github.com/merrydance/locallife/db/sqlc"
)

// MembershipPaymentInput defines the input for validating membership payment usage.
type MembershipPaymentInput struct {
	UserID             int64
	MerchantID         int64
	OrderType          string
	RulesEngineEnabled bool
}

// ValidateMembershipPayment validates whether membership balance can be used.
func ValidateMembershipPayment(ctx context.Context, store db.Store, input MembershipPaymentInput) (*db.MerchantMembership, error) {
	if !IsMembershipBalanceSupportedOrderType(input.OrderType) {
		return nil, NewRequestError(http.StatusBadRequest, errors.New("仅堂食和外带自取支持余额支付"))
	}

	membership, err := store.GetMembershipByMerchantAndUser(ctx, db.GetMembershipByMerchantAndUserParams{
		MerchantID: input.MerchantID,
		UserID:     input.UserID,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return nil, NewRequestError(http.StatusBadRequest, errors.New("会员卡不存在"))
		}
		return nil, err
	}

	settings, err := store.GetMerchantMembershipSettings(ctx, input.MerchantID)
	if err == nil {
		sceneAllowed := false
		for _, scene := range sanitizeMembershipUsableScenes(settings.BalanceUsableScenes) {
			if scene == input.OrderType {
				sceneAllowed = true
				break
			}
		}
		if !sceneAllowed && !input.RulesEngineEnabled {
			return nil, NewRequestError(http.StatusBadRequest, errors.New("该商户暂不支持余额支付"))
		}
	}

	if membership.Balance <= 0 {
		return nil, NewRequestError(http.StatusBadRequest, errors.New("会员余额不足"))
	}

	return &membership, nil
}
