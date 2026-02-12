package logic

import (
	"context"
	"errors"
	"net/http"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
)

// TakeoutSuspensionInfo holds takeout suspension details for a merchant.
type TakeoutSuspensionInfo struct {
	Reason string
	Until  time.Time
}

// ValidateMerchantForOrder ensures the merchant is active and open for ordering.
func ValidateMerchantForOrder(ctx context.Context, store db.Store, merchantID int64) (db.Merchant, error) {
	merchant, err := store.GetMerchant(ctx, merchantID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.Merchant{}, NewRequestError(http.StatusNotFound, errors.New("merchant not found"))
		}
		return db.Merchant{}, err
	}

	if merchant.Status != "active" {
		return db.Merchant{}, NewRequestError(http.StatusBadRequest, errors.New("merchant is not active"))
	}
	if !merchant.IsOpen {
		return db.Merchant{}, NewRequestError(http.StatusBadRequest, errors.New("商户已打烊，暂时无法接单"))
	}

	return merchant, nil
}

// GetTakeoutSuspension returns takeout suspension info if ordering is suspended.
func GetTakeoutSuspension(ctx context.Context, store db.Store, merchantID int64) (*TakeoutSuspensionInfo, error) {
	profile, err := store.GetMerchantProfile(ctx, merchantID)
	if err != nil {
		return nil, nil
	}
	if !profile.IsTakeoutSuspended {
		return nil, nil
	}

	return &TakeoutSuspensionInfo{
		Reason: profile.TakeoutSuspendReason.String,
		Until:  profile.TakeoutSuspendUntil.Time,
	}, nil
}

// ValidateTableOwnership ensures the table exists and belongs to the merchant.
func ValidateTableOwnership(ctx context.Context, store db.Store, merchantID, tableID int64) error {
	table, err := store.GetTable(ctx, tableID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return NewRequestError(http.StatusNotFound, errors.New("table not found"))
		}
		return err
	}
	if table.MerchantID != merchantID {
		return NewRequestError(http.StatusBadRequest, errors.New("table does not belong to this merchant"))
	}

	return nil
}

// CheckTakeoutBlocklist returns true if the user is blocked from takeout orders.
func CheckTakeoutBlocklist(ctx context.Context, store db.Store, userID int64) (bool, error) {
	block, err := store.GetActiveBehaviorBlocklist(ctx, db.GetActiveBehaviorBlocklistParams{
		EntityType: "user",
		EntityID:   userID,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}

	if block.BlockUntil.Valid && time.Now().After(block.BlockUntil.Time) {
		_ = store.UpdateBehaviorBlocklistStatus(ctx, db.UpdateBehaviorBlocklistStatusParams{
			ID:     block.ID,
			Status: "expired",
		})
		return false, nil
	}

	return true, nil
}
