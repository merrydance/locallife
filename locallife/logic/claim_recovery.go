package logic

import (
	"context"
	"errors"
	"net/http"

	db "github.com/merrydance/locallife/db/sqlc"
)

type MerchantClaimRecoveryInput struct {
	RecoveryID int64
	MerchantID int64
}

type RiderClaimRecoveryInput struct {
	RecoveryID int64
	RiderID    int64
}

type OperatorClaimRecoveryInput struct {
	RecoveryID int64
	RegionID   int64
	RegionIDs  []int64
}

type ClaimRecoveryMerchantResult struct {
	Recovery          db.ClaimRecovery
	ReleaseVisibility *ClaimRecoveryReleaseVisibility
}

func GetClaimRecoveryForMerchant(ctx context.Context, store db.Store, input MerchantClaimRecoveryInput) (ClaimRecoveryMerchantResult, error) {
	recoveryCtx, err := getClaimRecoveryContextByID(ctx, store, input.RecoveryID)
	if err != nil {
		return ClaimRecoveryMerchantResult{}, err
	}

	if recoveryCtx.MerchantID != input.MerchantID {
		return ClaimRecoveryMerchantResult{}, NewRequestError(http.StatusForbidden, errors.New("this claim does not belong to your merchant"))
	}
	if recoveryCtx.RecoveryTarget.String != "merchant" {
		return ClaimRecoveryMerchantResult{}, NewRequestError(http.StatusForbidden, errors.New("this claim recovery does not belong to your merchant"))
	}

	recovery := claimRecoveryFromContextByID(recoveryCtx)
	return ClaimRecoveryMerchantResult{
		Recovery:          recovery,
		ReleaseVisibility: resolveMerchantClaimRecoveryReleaseVisibility(ctx, store, recovery),
	}, nil
}

func GetClaimRecoveryForRider(ctx context.Context, store db.Store, input RiderClaimRecoveryInput) (db.ClaimRecovery, error) {
	recoveryCtx, err := getClaimRecoveryContextByID(ctx, store, input.RecoveryID)
	if err != nil {
		return db.ClaimRecovery{}, err
	}

	if !recoveryCtx.RiderID.Valid || recoveryCtx.RiderID.Int64 != input.RiderID {
		return db.ClaimRecovery{}, NewRequestError(http.StatusForbidden, errors.New("this claim does not belong to your rider"))
	}
	if recoveryCtx.RecoveryTarget.String != "rider" {
		return db.ClaimRecovery{}, NewRequestError(http.StatusForbidden, errors.New("this claim recovery does not belong to your rider"))
	}

	return claimRecoveryFromContextByID(recoveryCtx), nil
}

func GetClaimRecoveryForOperator(ctx context.Context, store db.Store, input OperatorClaimRecoveryInput) (db.ClaimRecovery, error) {
	recoveryCtx, err := getClaimRecoveryContextByID(ctx, store, input.RecoveryID)
	if err != nil {
		return db.ClaimRecovery{}, err
	}

	if !operatorManagesClaimRecoveryRegion(recoveryCtx.RegionID, input.RegionID, input.RegionIDs) {
		return db.ClaimRecovery{}, NewRequestError(http.StatusForbidden, errors.New("operator does not manage this region"))
	}

	return claimRecoveryFromContextByID(recoveryCtx), nil
}

func operatorManagesClaimRecoveryRegion(claimRegionID int64, fallbackRegionID int64, managedRegionIDs []int64) bool {
	if len(managedRegionIDs) == 0 {
		return claimRegionID == fallbackRegionID
	}

	for _, regionID := range managedRegionIDs {
		if regionID == claimRegionID {
			return true
		}
	}

	return false
}
