package logic

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

type CreateRechargeRuleInput struct {
	MerchantID     int64
	RechargeAmount int64
	BonusAmount    int64
	ValidFrom      time.Time
	ValidUntil     time.Time
}

type MerchantRechargeRulesInput struct {
	MerchantID       int64
	TargetMerchantID int64
}

type UpdateRechargeRuleInput struct {
	MerchantID       int64
	TargetMerchantID int64
	RuleID           int64
	RechargeAmount   *int64
	BonusAmount      *int64
	IsActive         *bool
	ValidFrom        *time.Time
	ValidUntil       *time.Time
}

type DeleteRechargeRuleInput struct {
	MerchantID       int64
	TargetMerchantID int64
	RuleID           int64
}

func CreateRechargeRule(ctx context.Context, store db.Store, input CreateRechargeRuleInput) (db.RechargeRule, error) {
	if input.ValidUntil.Before(input.ValidFrom) {
		return db.RechargeRule{}, NewRequestError(http.StatusBadRequest, errors.New("valid_until must be after valid_from"))
	}

	rule, err := store.CreateRechargeRule(ctx, db.CreateRechargeRuleParams{
		MerchantID:     input.MerchantID,
		RechargeAmount: input.RechargeAmount,
		BonusAmount:    input.BonusAmount,
		IsActive:       true,
		ValidFrom:      input.ValidFrom,
		ValidUntil:     input.ValidUntil,
	})
	if err != nil {
		return db.RechargeRule{}, err
	}

	return rule, nil
}

func ListMerchantRechargeRules(ctx context.Context, store db.Store, input MerchantRechargeRulesInput) ([]db.RechargeRule, error) {
	if input.MerchantID != input.TargetMerchantID {
		return nil, NewRequestError(http.StatusForbidden, errors.New("not authorized for this merchant"))
	}

	rules, err := store.ListMerchantRechargeRules(ctx, input.TargetMerchantID)
	if err != nil {
		return nil, err
	}

	return rules, nil
}

func ListActiveRechargeRules(ctx context.Context, store db.Store, input MerchantRechargeRulesInput) ([]db.RechargeRule, error) {
	if input.MerchantID != input.TargetMerchantID {
		return nil, NewRequestError(http.StatusForbidden, errors.New("not authorized for this merchant"))
	}

	rules, err := store.ListActiveRechargeRules(ctx, input.TargetMerchantID)
	if err != nil {
		return nil, err
	}

	return rules, nil
}

func GetPublicRechargeRules(ctx context.Context, store db.Store, merchantID int64) ([]db.RechargeRule, error) {
	if _, err := store.GetMerchant(ctx, merchantID); err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return nil, NewRequestError(http.StatusNotFound, errors.New("merchant not found"))
		}
		return nil, err
	}

	rules, err := store.ListActiveRechargeRules(ctx, merchantID)
	if err != nil {
		return nil, err
	}

	return rules, nil
}

func UpdateRechargeRuleForMerchant(ctx context.Context, store db.Store, input UpdateRechargeRuleInput) (db.RechargeRule, error) {
	rule, err := store.GetRechargeRule(ctx, input.RuleID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.RechargeRule{}, NewRequestError(http.StatusNotFound, errors.New("rule not found"))
		}
		return db.RechargeRule{}, err
	}

	if rule.MerchantID != input.TargetMerchantID {
		return db.RechargeRule{}, NewRequestError(http.StatusNotFound, errors.New("rule not found"))
	}

	if rule.MerchantID != input.MerchantID {
		return db.RechargeRule{}, NewRequestError(http.StatusForbidden, errors.New("not authorized"))
	}

	effectiveValidFrom := rule.ValidFrom
	effectiveValidUntil := rule.ValidUntil
	if input.ValidFrom != nil {
		effectiveValidFrom = *input.ValidFrom
	}
	if input.ValidUntil != nil {
		effectiveValidUntil = *input.ValidUntil
	}
	if effectiveValidUntil.Before(effectiveValidFrom) {
		return db.RechargeRule{}, NewRequestError(http.StatusBadRequest, errors.New("valid_until must be after valid_from"))
	}

	arg := db.UpdateRechargeRuleParams{ID: input.RuleID}
	if input.RechargeAmount != nil {
		arg.RechargeAmount = pgtype.Int8{Int64: *input.RechargeAmount, Valid: true}
	}
	if input.BonusAmount != nil {
		arg.BonusAmount = pgtype.Int8{Int64: *input.BonusAmount, Valid: true}
	}
	if input.IsActive != nil {
		arg.IsActive = pgtype.Bool{Bool: *input.IsActive, Valid: true}
	}
	if input.ValidFrom != nil {
		arg.ValidFrom = pgtype.Timestamptz{Time: *input.ValidFrom, Valid: true}
	}
	if input.ValidUntil != nil {
		arg.ValidUntil = pgtype.Timestamptz{Time: *input.ValidUntil, Valid: true}
	}

	updated, err := store.UpdateRechargeRule(ctx, arg)
	if err != nil {
		return db.RechargeRule{}, err
	}

	return updated, nil
}

func DeleteRechargeRuleForMerchant(ctx context.Context, store db.Store, input DeleteRechargeRuleInput) error {
	rule, err := store.GetRechargeRule(ctx, input.RuleID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return NewRequestError(http.StatusNotFound, errors.New("rule not found"))
		}
		return err
	}

	if rule.MerchantID != input.TargetMerchantID {
		return NewRequestError(http.StatusNotFound, errors.New("rule not found"))
	}

	if rule.MerchantID != input.MerchantID {
		return NewRequestError(http.StatusForbidden, errors.New("not authorized"))
	}

	if err := store.DeleteRechargeRule(ctx, input.RuleID); err != nil {
		return err
	}

	return nil
}
