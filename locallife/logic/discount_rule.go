package logic

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

type CreateDiscountRuleInput struct {
	MerchantID             int64
	Name                   string
	Description            string
	MinOrderAmount         int64
	DiscountAmount         int64
	CanStackWithVoucher    bool
	CanStackWithMembership bool
	StackingGroup          *string
	ValidFrom              time.Time
	ValidUntil             time.Time
}

type DiscountRuleAccessInput struct {
	MerchantID int64
	RuleID     int64
}

type ListMerchantDiscountRulesInput struct {
	MerchantID       int64
	TargetMerchantID int64
	Limit            int32
	Offset           int32
}

type CountMerchantDiscountRulesInput struct {
	MerchantID       int64
	TargetMerchantID int64
}

type ListActiveDiscountRulesInput struct {
	MerchantID       int64
	TargetMerchantID int64
}

type ApplicableDiscountRulesInput struct {
	MerchantID       int64
	TargetMerchantID int64
	OrderAmount      int64
}

type BestDiscountRuleInput struct {
	MerchantID       int64
	TargetMerchantID int64
	OrderAmount      int64
}

type UpdateDiscountRuleInput struct {
	MerchantID             int64
	RuleID                 int64
	Name                   *string
	Description            *string
	MinOrderAmount         *int64
	DiscountAmount         *int64
	CanStackWithVoucher    *bool
	CanStackWithMembership *bool
	StackingGroup          *string
	ValidFrom              *time.Time
	ValidUntil             *time.Time
	IsActive               *bool
}

type DeleteDiscountRuleInput struct {
	MerchantID int64
	RuleID     int64
}

func CreateDiscountRule(ctx context.Context, store db.Store, input CreateDiscountRuleInput) (db.DiscountRule, error) {
	if err := validateDiscountRuleValues(input.ValidFrom, input.ValidUntil, input.MinOrderAmount, input.DiscountAmount); err != nil {
		return db.DiscountRule{}, err
	}

	description := pgtype.Text{String: input.Description, Valid: input.Description != ""}
	stackingGroup := pgtype.Text{Valid: false}
	if input.StackingGroup != nil && *input.StackingGroup != "" {
		stackingGroup = pgtype.Text{String: *input.StackingGroup, Valid: true}
	}

	rule, err := store.CreateDiscountRule(ctx, db.CreateDiscountRuleParams{
		MerchantID:             input.MerchantID,
		Name:                   input.Name,
		Description:            description,
		MinOrderAmount:         input.MinOrderAmount,
		DiscountAmount:         input.DiscountAmount,
		CanStackWithVoucher:    input.CanStackWithVoucher,
		CanStackWithMembership: input.CanStackWithMembership,
		StackingGroup:          stackingGroup,
		ValidFrom:              input.ValidFrom,
		ValidUntil:             input.ValidUntil,
		IsActive:               true,
	})
	if err != nil {
		return db.DiscountRule{}, err
	}

	return rule, nil
}

func GetDiscountRuleForMerchant(ctx context.Context, store db.Store, input DiscountRuleAccessInput) (db.DiscountRule, error) {
	rule, err := store.GetDiscountRule(ctx, input.RuleID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.DiscountRule{}, NewRequestError(http.StatusNotFound, errors.New("discount rule not found"))
		}
		return db.DiscountRule{}, err
	}

	if rule.MerchantID != input.MerchantID {
		return db.DiscountRule{}, NewRequestError(http.StatusForbidden, errors.New("insufficient permissions for this merchant"))
	}

	return rule, nil
}

func ListMerchantDiscountRules(ctx context.Context, store db.Store, input ListMerchantDiscountRulesInput) ([]db.DiscountRule, error) {
	if input.TargetMerchantID != input.MerchantID {
		return nil, NewRequestError(http.StatusForbidden, errors.New("insufficient permissions for this merchant"))
	}

	rules, err := store.ListMerchantDiscountRules(ctx, db.ListMerchantDiscountRulesParams{
		MerchantID: input.TargetMerchantID,
		Limit:      input.Limit,
		Offset:     input.Offset,
	})
	if err != nil {
		return nil, err
	}

	return rules, nil
}

func CountMerchantDiscountRules(ctx context.Context, store db.Store, input CountMerchantDiscountRulesInput) (int64, error) {
	if input.TargetMerchantID != input.MerchantID {
		return 0, NewRequestError(http.StatusForbidden, errors.New("insufficient permissions for this merchant"))
	}

	count, err := store.CountMerchantDiscountRules(ctx, input.TargetMerchantID)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func ListActiveDiscountRules(ctx context.Context, store db.Store, input ListActiveDiscountRulesInput) ([]db.DiscountRule, error) {
	if input.TargetMerchantID != input.MerchantID {
		return nil, NewRequestError(http.StatusForbidden, errors.New("insufficient permissions for this merchant"))
	}

	rules, err := store.ListActiveDiscountRules(ctx, input.TargetMerchantID)
	if err != nil {
		return nil, err
	}

	return rules, nil
}

func GetApplicableDiscountRules(ctx context.Context, store db.Store, input ApplicableDiscountRulesInput) ([]db.DiscountRule, error) {
	if input.TargetMerchantID != input.MerchantID {
		return nil, NewRequestError(http.StatusForbidden, errors.New("insufficient permissions for this merchant"))
	}

	rules, err := store.GetApplicableDiscountRules(ctx, db.GetApplicableDiscountRulesParams{
		MerchantID:     input.TargetMerchantID,
		MinOrderAmount: input.OrderAmount,
	})
	if err != nil {
		return nil, err
	}

	return rules, nil
}

func GetBestDiscountRule(ctx context.Context, store db.Store, input BestDiscountRuleInput) (db.DiscountRule, error) {
	if input.TargetMerchantID != input.MerchantID {
		return db.DiscountRule{}, NewRequestError(http.StatusForbidden, errors.New("insufficient permissions for this merchant"))
	}

	rule, err := store.GetBestDiscountRule(ctx, db.GetBestDiscountRuleParams{
		MerchantID:     input.TargetMerchantID,
		MinOrderAmount: input.OrderAmount,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.DiscountRule{}, NewRequestError(http.StatusNotFound, errors.New("no applicable discount rule found"))
		}
		return db.DiscountRule{}, err
	}

	return rule, nil
}

func UpdateDiscountRuleForMerchant(ctx context.Context, store db.Store, input UpdateDiscountRuleInput) (db.DiscountRule, error) {
	rule, err := store.GetDiscountRule(ctx, input.RuleID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.DiscountRule{}, NewRequestError(http.StatusNotFound, errors.New("discount rule not found"))
		}
		return db.DiscountRule{}, err
	}

	if rule.MerchantID != input.MerchantID {
		return db.DiscountRule{}, NewRequestError(http.StatusForbidden, errors.New("not authorized"))
	}

	effectiveValidFrom := rule.ValidFrom
	effectiveValidUntil := rule.ValidUntil
	effectiveMinOrderAmount := rule.MinOrderAmount
	effectiveDiscountAmount := rule.DiscountAmount
	if input.ValidFrom != nil {
		effectiveValidFrom = *input.ValidFrom
	}
	if input.ValidUntil != nil {
		effectiveValidUntil = *input.ValidUntil
	}
	if input.MinOrderAmount != nil {
		effectiveMinOrderAmount = *input.MinOrderAmount
	}
	if input.DiscountAmount != nil {
		effectiveDiscountAmount = *input.DiscountAmount
	}
	if err := validateDiscountRuleValues(effectiveValidFrom, effectiveValidUntil, effectiveMinOrderAmount, effectiveDiscountAmount); err != nil {
		return db.DiscountRule{}, err
	}

	arg := db.UpdateDiscountRuleParams{ID: input.RuleID}
	if input.Name != nil {
		arg.Name = pgtype.Text{String: *input.Name, Valid: true}
	}
	if input.Description != nil {
		arg.Description = pgtype.Text{String: *input.Description, Valid: true}
	}
	if input.MinOrderAmount != nil {
		arg.MinOrderAmount = pgtype.Int8{Int64: *input.MinOrderAmount, Valid: true}
	}
	if input.DiscountAmount != nil {
		arg.DiscountAmount = pgtype.Int8{Int64: *input.DiscountAmount, Valid: true}
	}
	if input.CanStackWithVoucher != nil {
		arg.CanStackWithVoucher = pgtype.Bool{Bool: *input.CanStackWithVoucher, Valid: true}
	}
	if input.CanStackWithMembership != nil {
		arg.CanStackWithMembership = pgtype.Bool{Bool: *input.CanStackWithMembership, Valid: true}
	}
	if input.StackingGroup != nil {
		arg.StackingGroup = pgtype.Text{String: *input.StackingGroup, Valid: *input.StackingGroup != ""}
	}
	if input.ValidFrom != nil {
		arg.ValidFrom = pgtype.Timestamptz{Time: *input.ValidFrom, Valid: true}
	}
	if input.ValidUntil != nil {
		arg.ValidUntil = pgtype.Timestamptz{Time: *input.ValidUntil, Valid: true}
	}
	if input.IsActive != nil {
		arg.IsActive = pgtype.Bool{Bool: *input.IsActive, Valid: true}
	}

	updated, err := store.UpdateDiscountRule(ctx, arg)
	if err != nil {
		return db.DiscountRule{}, err
	}

	return updated, nil
}

func DeleteDiscountRuleForMerchant(ctx context.Context, store db.Store, input DeleteDiscountRuleInput) error {
	rule, err := store.GetDiscountRule(ctx, input.RuleID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return NewRequestError(http.StatusNotFound, errors.New("discount rule not found"))
		}
		return err
	}

	if rule.MerchantID != input.MerchantID {
		return NewRequestError(http.StatusForbidden, errors.New("not authorized"))
	}

	if err := store.DeleteDiscountRule(ctx, input.RuleID); err != nil {
		return err
	}

	return nil
}

func validateDiscountRuleValues(validFrom, validUntil time.Time, minOrderAmount, discountAmount int64) error {
	if minOrderAmount <= 0 {
		return NewRequestError(http.StatusBadRequest, errors.New("min_order_amount must be greater than zero"))
	}
	if discountAmount <= 0 {
		return NewRequestError(http.StatusBadRequest, errors.New("discount_amount must be greater than zero"))
	}
	if !validUntil.After(validFrom) {
		return NewRequestError(http.StatusBadRequest, errors.New("valid_until must be after valid_from"))
	}
	if discountAmount >= minOrderAmount {
		return NewRequestError(http.StatusBadRequest, errors.New("discount_amount must be less than min_order_amount"))
	}
	return nil
}
