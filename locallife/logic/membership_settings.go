package logic

import (
	"context"
	"errors"
	"net/http"

	db "github.com/merrydance/locallife/db/sqlc"
)

type MembershipSettingsResult struct {
	MerchantID          int64
	BalanceUsableScenes []string
	BonusUsableScenes   []string
	AllowWithVoucher    bool
	AllowWithDiscount   bool
	MaxDeductionPercent int32
}

type UpdateMembershipSettingsInput struct {
	OwnerUserID         int64
	BalanceUsableScenes []string
	BonusUsableScenes   []string
	AllowWithVoucher    *bool
	AllowWithDiscount   *bool
	MaxDeductionPercent *int32
}

func defaultMembershipSettings(merchantID int64) MembershipSettingsResult {
	return MembershipSettingsResult{
		MerchantID:          merchantID,
		BalanceUsableScenes: []string{"dine_in", "takeaway"},
		BonusUsableScenes:   []string{"dine_in"},
		AllowWithVoucher:    true,
		AllowWithDiscount:   true,
		MaxDeductionPercent: 100,
	}
}

func settingsResultFromModel(settings db.MerchantMembershipSetting) MembershipSettingsResult {
	return MembershipSettingsResult{
		MerchantID:          settings.MerchantID,
		BalanceUsableScenes: sanitizeMembershipUsableScenes(settings.BalanceUsableScenes),
		BonusUsableScenes:   sanitizeMembershipUsableScenes(settings.BonusUsableScenes),
		AllowWithVoucher:    settings.AllowWithVoucher,
		AllowWithDiscount:   settings.AllowWithDiscount,
		MaxDeductionPercent: settings.MaxDeductionPercent,
	}
}

func GetMembershipSettingsForOwner(ctx context.Context, store db.Store, ownerUserID int64) (MembershipSettingsResult, error) {
	merchant, err := resolveMerchantForUser(ctx, store, ownerUserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return MembershipSettingsResult{}, NewRequestError(http.StatusNotFound, errors.New("merchant not found"))
		}
		return MembershipSettingsResult{}, err
	}
	if merchant.OwnerUserID != ownerUserID {
		return MembershipSettingsResult{}, NewRequestError(http.StatusForbidden, errors.New("merchant owner required"))
	}

	settings, err := store.GetMerchantMembershipSettings(ctx, merchant.ID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return defaultMembershipSettings(merchant.ID), nil
		}
		return MembershipSettingsResult{}, err
	}

	return settingsResultFromModel(settings), nil
}

func UpdateMembershipSettingsForOwner(ctx context.Context, store db.Store, input UpdateMembershipSettingsInput) (MembershipSettingsResult, error) {
	merchant, err := resolveMerchantForUser(ctx, store, input.OwnerUserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return MembershipSettingsResult{}, NewRequestError(http.StatusNotFound, errors.New("merchant not found"))
		}
		return MembershipSettingsResult{}, err
	}
	if merchant.OwnerUserID != input.OwnerUserID {
		return MembershipSettingsResult{}, NewRequestError(http.StatusForbidden, errors.New("merchant owner required"))
	}

	defaults := defaultMembershipSettings(merchant.ID)
	balanceScenes := defaults.BalanceUsableScenes
	bonusScenes := defaults.BonusUsableScenes
	allowVoucher := defaults.AllowWithVoucher
	allowDiscount := defaults.AllowWithDiscount
	maxPercent := defaults.MaxDeductionPercent

	if input.BalanceUsableScenes != nil {
		balanceScenes = sanitizeMembershipUsableScenes(input.BalanceUsableScenes)
	}
	if input.BonusUsableScenes != nil {
		bonusScenes = sanitizeMembershipUsableScenes(input.BonusUsableScenes)
	}
	if input.AllowWithVoucher != nil {
		allowVoucher = *input.AllowWithVoucher
	}
	if input.AllowWithDiscount != nil {
		allowDiscount = *input.AllowWithDiscount
	}
	if input.MaxDeductionPercent != nil {
		maxPercent = *input.MaxDeductionPercent
	}

	settings, err := store.UpsertMerchantMembershipSettings(ctx, db.UpsertMerchantMembershipSettingsParams{
		MerchantID:          merchant.ID,
		BalanceUsableScenes: balanceScenes,
		BonusUsableScenes:   bonusScenes,
		AllowWithVoucher:    allowVoucher,
		AllowWithDiscount:   allowDiscount,
		MaxDeductionPercent: maxPercent,
	})
	if err != nil {
		return MembershipSettingsResult{}, err
	}

	return settingsResultFromModel(settings), nil
}
