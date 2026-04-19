package logic

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
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

type MerchantMembershipRechargeInput struct {
	MerchantID       int64
	TargetMerchantID int64
	UserID           int64
	RechargeAmount   int64
	Notes            string
	IdempotencyKey   string
}

type MerchantMembershipRechargeResult struct {
	Membership     db.MerchantMembership
	User           db.User
	Transaction    db.MembershipTransaction
	BonusAmount    int64
	RechargeRuleID *int64
}

const membershipRechargeIdempotencyPrefix = "[merchant_recharge_idempotency:"

func StripMembershipTransactionSystemNotes(notes string) string {
	trimmed := strings.TrimSpace(notes)
	if !strings.HasPrefix(trimmed, membershipRechargeIdempotencyPrefix) {
		return notes
	}

	endIndex := strings.Index(trimmed, "]")
	if endIndex < 0 {
		return notes
	}

	return strings.TrimSpace(trimmed[endIndex+1:])
}

func buildMerchantRechargeStoredNotes(notes string) string {
	return strings.TrimSpace(notes)
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

	bonusAmount, rechargeRuleID, err := resolveMembershipRechargeRule(ctx, store, membership.MerchantID, input.RechargeAmount)
	if err != nil {
		return result, err
	}

	result.Membership = membership
	result.BonusAmount = bonusAmount
	result.RechargeRuleID = rechargeRuleID
	return result, nil
}

func RecordMembershipRechargeForMerchant(ctx context.Context, store db.Store, input MerchantMembershipRechargeInput) (MerchantMembershipRechargeResult, error) {
	var result MerchantMembershipRechargeResult

	if input.RechargeAmount <= 0 {
		return result, NewRequestError(http.StatusBadRequest, errors.New("recharge amount must be greater than zero"))
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

	user, err := store.GetUser(ctx, input.UserID)
	if err != nil {
		return result, err
	}

	result, err = loadMerchantRechargeByIdempotencyKey(ctx, store, membership, user, input.RechargeAmount, strings.TrimSpace(input.Notes), idempotencyKey)
	if err == nil {
		return result, nil
	}
	if !errors.Is(err, db.ErrRecordNotFound) {
		return result, err
	}

	bonusAmount, rechargeRuleID, err := resolveMembershipRechargeRule(ctx, store, membership.MerchantID, input.RechargeAmount)
	if err != nil {
		return result, err
	}

	rechargeResult, err := store.RechargeTx(ctx, db.RechargeTxParams{
		MembershipID:   membership.ID,
		RechargeAmount: input.RechargeAmount,
		BonusAmount:    bonusAmount,
		RechargeRuleID: rechargeRuleID,
		Notes:          buildMerchantRechargeStoredNotes(input.Notes),
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		if db.ErrorCode(err) == db.UniqueViolation {
			result, duplicateErr := loadMerchantRechargeByIdempotencyKey(ctx, store, membership, user, input.RechargeAmount, strings.TrimSpace(input.Notes), idempotencyKey)
			if duplicateErr == nil {
				return result, nil
			}
			if !errors.Is(duplicateErr, db.ErrRecordNotFound) {
				return result, duplicateErr
			}
		}
		return result, err
	}

	result.Membership = rechargeResult.Membership
	result.User = user
	result.Transaction = rechargeResult.Transaction
	result.BonusAmount = bonusAmount
	result.RechargeRuleID = rechargeRuleID
	return result, nil
}

func loadMerchantRechargeByIdempotencyKey(
	ctx context.Context,
	store db.Store,
	membership db.MerchantMembership,
	user db.User,
	rechargeAmount int64,
	notes string,
	idempotencyKey string,
) (MerchantMembershipRechargeResult, error) {
	var result MerchantMembershipRechargeResult

	existingTransaction, err := store.GetMembershipRechargeTransactionByIdempotencyKey(ctx, db.GetMembershipRechargeTransactionByIdempotencyKeyParams{
		MembershipID:   membership.ID,
		IdempotencyKey: pgtype.Text{String: idempotencyKey, Valid: true},
	})
	if err != nil {
		return result, err
	}
	if existingTransaction.PrincipalAmount != rechargeAmount || StripMembershipTransactionSystemNotes(existingTransaction.Notes.String) != notes {
		return result, NewRequestError(http.StatusConflict, errors.New("idempotency key already used by a different membership recharge request"))
	}

	latestMembership, err := store.GetMerchantMembership(ctx, membership.ID)
	if err != nil {
		return result, err
	}

	result.Membership = latestMembership
	result.User = user
	result.Transaction = existingTransaction
	result.BonusAmount = existingTransaction.BonusAmount
	if existingTransaction.RechargeRuleID.Valid {
		rechargeRuleID := existingTransaction.RechargeRuleID.Int64
		result.RechargeRuleID = &rechargeRuleID
	}
	return result, nil
}

func resolveMembershipRechargeRule(ctx context.Context, store db.Store, merchantID, rechargeAmount int64) (int64, *int64, error) {
	matchingRule, err := store.GetMatchingRechargeRule(ctx, db.GetMatchingRechargeRuleParams{
		MerchantID:     merchantID,
		RechargeAmount: rechargeAmount,
	})
	if err == nil {
		rechargeRuleID := matchingRule.ID
		return matchingRule.BonusAmount, &rechargeRuleID, nil
	} else if !errors.Is(err, db.ErrRecordNotFound) {
		return 0, nil, err
	}

	return 0, nil, nil
}
