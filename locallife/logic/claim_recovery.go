package logic

import (
	"context"
	"errors"
	"net/http"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
)

type MerchantClaimRecoveryInput struct {
	ClaimID    int64
	MerchantID int64
}

type RiderClaimRecoveryInput struct {
	ClaimID int64
	RiderID int64
}

type OperatorClaimRecoveryInput struct {
	ClaimID  int64
	RegionID int64
}

type PayMerchantClaimRecoveryInput struct {
	ClaimID    int64
	MerchantID int64
	Now        time.Time
}

type PayRiderClaimRecoveryInput struct {
	ClaimID int64
	RiderID int64
}

type WaiveClaimRecoveryInput struct {
	ClaimID  int64
	RegionID int64
	Now      time.Time
}

func GetClaimRecoveryForMerchant(ctx context.Context, store db.Store, input MerchantClaimRecoveryInput) (db.ClaimRecovery, error) {
	claimInfo, err := store.GetClaimForAppeal(ctx, input.ClaimID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.ClaimRecovery{}, NewRequestError(http.StatusNotFound, errors.New("claim not found or not eligible for recovery"))
		}
		return db.ClaimRecovery{}, err
	}

	if claimInfo.MerchantID != input.MerchantID {
		return db.ClaimRecovery{}, NewRequestError(http.StatusForbidden, errors.New("this claim does not belong to your merchant"))
	}

	recovery, err := store.GetClaimRecoveryByClaimID(ctx, input.ClaimID)
	if err != nil {
		return db.ClaimRecovery{}, NewRequestError(http.StatusNotFound, errors.New("claim recovery not found"))
	}

	return recovery, nil
}

func GetClaimRecoveryForRider(ctx context.Context, store db.Store, input RiderClaimRecoveryInput) (db.ClaimRecovery, error) {
	claimInfo, err := store.GetClaimForAppeal(ctx, input.ClaimID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.ClaimRecovery{}, NewRequestError(http.StatusNotFound, errors.New("claim not found or not eligible for recovery"))
		}
		return db.ClaimRecovery{}, err
	}

	if !claimInfo.RiderID.Valid || claimInfo.RiderID.Int64 != input.RiderID {
		return db.ClaimRecovery{}, NewRequestError(http.StatusForbidden, errors.New("this claim does not belong to your rider"))
	}

	recovery, err := store.GetClaimRecoveryByClaimID(ctx, input.ClaimID)
	if err != nil {
		return db.ClaimRecovery{}, NewRequestError(http.StatusNotFound, errors.New("claim recovery not found"))
	}

	return recovery, nil
}

func GetClaimRecoveryForOperator(ctx context.Context, store db.Store, input OperatorClaimRecoveryInput) (db.ClaimRecovery, error) {
	claimInfo, err := store.GetClaimForAppeal(ctx, input.ClaimID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.ClaimRecovery{}, NewRequestError(http.StatusNotFound, errors.New("claim not found or not eligible for recovery"))
		}
		return db.ClaimRecovery{}, err
	}

	if claimInfo.RegionID != input.RegionID {
		return db.ClaimRecovery{}, NewRequestError(http.StatusForbidden, errors.New("operator does not manage this region"))
	}

	recovery, err := store.GetClaimRecoveryByClaimID(ctx, input.ClaimID)
	if err != nil {
		return db.ClaimRecovery{}, NewRequestError(http.StatusNotFound, errors.New("claim recovery not found"))
	}

	return recovery, nil
}

func PayMerchantClaimRecovery(ctx context.Context, store db.Store, input PayMerchantClaimRecoveryInput) (db.ClaimRecovery, error) {
	claimInfo, err := store.GetClaimForAppeal(ctx, input.ClaimID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.ClaimRecovery{}, NewRequestError(http.StatusNotFound, errors.New("claim not found or not eligible for recovery"))
		}
		return db.ClaimRecovery{}, err
	}

	if claimInfo.MerchantID != input.MerchantID {
		return db.ClaimRecovery{}, NewRequestError(http.StatusForbidden, errors.New("this claim does not belong to your merchant"))
	}

	recovery, err := store.GetClaimRecoveryByClaimID(ctx, input.ClaimID)
	if err != nil {
		return db.ClaimRecovery{}, NewRequestError(http.StatusNotFound, errors.New("claim recovery not found"))
	}

	if !recovery.RecoveryTarget.Valid || recovery.RecoveryTarget.String != "merchant" {
		return db.ClaimRecovery{}, NewRequestError(http.StatusBadRequest, errors.New("recovery target mismatch"))
	}

	updated, err := store.MarkClaimRecoveryPaid(ctx, recovery.ID)
	if err != nil {
		return db.ClaimRecovery{}, err
	}

	if err := store.UnsuspendMerchantTakeout(ctx, input.MerchantID); err != nil {
		return db.ClaimRecovery{}, err
	}

	return updated, nil
}

func PayRiderClaimRecovery(ctx context.Context, store db.Store, input PayRiderClaimRecoveryInput) (db.ClaimRecovery, error) {
	claimInfo, err := store.GetClaimForAppeal(ctx, input.ClaimID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.ClaimRecovery{}, NewRequestError(http.StatusNotFound, errors.New("claim not found or not eligible for recovery"))
		}
		return db.ClaimRecovery{}, err
	}

	if !claimInfo.RiderID.Valid || claimInfo.RiderID.Int64 != input.RiderID {
		return db.ClaimRecovery{}, NewRequestError(http.StatusForbidden, errors.New("this claim does not belong to your rider"))
	}

	recovery, err := store.GetClaimRecoveryByClaimID(ctx, input.ClaimID)
	if err != nil {
		return db.ClaimRecovery{}, NewRequestError(http.StatusNotFound, errors.New("claim recovery not found"))
	}

	if !recovery.RecoveryTarget.Valid || recovery.RecoveryTarget.String != "rider" {
		return db.ClaimRecovery{}, NewRequestError(http.StatusBadRequest, errors.New("recovery target mismatch"))
	}

	updated, err := store.MarkClaimRecoveryPaid(ctx, recovery.ID)
	if err != nil {
		return db.ClaimRecovery{}, err
	}

	if err := store.UnsuspendRider(ctx, input.RiderID); err != nil {
		return db.ClaimRecovery{}, err
	}

	return updated, nil
}

func WaiveClaimRecovery(ctx context.Context, store db.Store, input WaiveClaimRecoveryInput) (db.ClaimRecovery, error) {
	claimInfo, err := store.GetClaimForAppeal(ctx, input.ClaimID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.ClaimRecovery{}, NewRequestError(http.StatusNotFound, errors.New("claim not found or not eligible for recovery"))
		}
		return db.ClaimRecovery{}, err
	}

	if claimInfo.RegionID != input.RegionID {
		return db.ClaimRecovery{}, NewRequestError(http.StatusForbidden, errors.New("operator does not manage this region"))
	}

	recovery, err := store.GetClaimRecoveryByClaimID(ctx, input.ClaimID)
	if err != nil {
		return db.ClaimRecovery{}, NewRequestError(http.StatusNotFound, errors.New("claim recovery not found"))
	}

	updated, err := store.MarkClaimRecoveryWaived(ctx, recovery.ID)
	if err != nil {
		return db.ClaimRecovery{}, err
	}

	if updated.RecoveryTarget.Valid && updated.RecoveryTarget.String == "merchant" {
		order, orderErr := store.GetOrder(ctx, updated.OrderID)
		if orderErr != nil {
			return db.ClaimRecovery{}, orderErr
		}
		if err := store.UnsuspendMerchantTakeout(ctx, order.MerchantID); err != nil {
			return db.ClaimRecovery{}, err
		}
	}

	if updated.RecoveryTarget.Valid && updated.RecoveryTarget.String == "rider" {
		delivery, deliveryErr := store.GetDeliveryByOrderID(ctx, updated.OrderID)
		if deliveryErr != nil {
			return db.ClaimRecovery{}, deliveryErr
		}
		if delivery.RiderID.Valid {
			if err := store.UnsuspendRider(ctx, delivery.RiderID.Int64); err != nil {
				return db.ClaimRecovery{}, err
			}
		}
	}

	return updated, nil
}
