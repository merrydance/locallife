package logic

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

type CreateMerchantRecoveryDisputeInput struct {
	MerchantID        int64
	ClaimID           int64
	Reason            string
	DisputeWindowDays int
	Now               time.Time
}

type CreateRiderRecoveryDisputeInput struct {
	RiderID           int64
	ClaimID           int64
	Reason            string
	DisputeWindowDays int
	Now               time.Time
}

type CreateRiderRecoveryDisputeResult struct {
	RecoveryDispute db.RecoveryDispute
	AlreadyExists   bool
}

type recoveryDisputeContext struct {
	ClaimID        int64
	OrderID        int64
	MerchantID     int64
	RegionID       int64
	RiderID        pgtype.Int8
	ClaimCreatedAt time.Time
	HasRecovery    bool
}

func CreateMerchantRecoveryDispute(ctx context.Context, store db.Store, input CreateMerchantRecoveryDisputeInput) (db.RecoveryDispute, error) {
	disputeCtx, err := getRecoveryDisputeContext(ctx, store, input.ClaimID, "merchant")
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.RecoveryDispute{}, NewRequestError(http.StatusNotFound, errors.New("claim not found or not eligible for recovery dispute"))
		}
		return db.RecoveryDispute{}, err
	}

	if disputeCtx.MerchantID != input.MerchantID {
		return db.RecoveryDispute{}, NewRequestError(http.StatusForbidden, errors.New("this claim does not belong to your merchant"))
	}

	now, windowDays := normalizeRecoveryDisputeWindow(input.Now, input.DisputeWindowDays)
	recoveryDisputeDeadline := disputeCtx.ClaimCreatedAt.Add(time.Duration(windowDays) * 24 * time.Hour)
	if now.After(recoveryDisputeDeadline) {
		return db.RecoveryDispute{}, NewRequestError(http.StatusBadRequest, errors.New("申诉窗口期已过（索赔后7天内可申诉）"))
	}

	exists, err := store.CheckRecoveryDisputeExists(ctx, db.CheckRecoveryDisputeExistsParams{
		ClaimID:       input.ClaimID,
		AppellantType: "merchant",
	})
	if err != nil {
		return db.RecoveryDispute{}, err
	}
	if exists {
		return db.RecoveryDispute{}, NewRequestError(http.StatusConflict, errors.New("recovery dispute already exists for this claim"))
	}

	result, err := store.CreateRecoveryDisputeWithRecoveryTx(ctx, db.CreateRecoveryDisputeWithRecoveryTxParams{
		ClaimID:        input.ClaimID,
		RecoveryTarget: "merchant",
		AppellantType:  "merchant",
		AppellantID:    input.MerchantID,
		Reason:         input.Reason,
		RegionID:       disputeCtx.RegionID,
	})
	if err != nil {
		if db.ErrorCode(err) == db.UniqueViolation {
			return db.RecoveryDispute{}, NewRequestError(http.StatusConflict, errors.New("recovery dispute already exists for this claim"))
		}
		return db.RecoveryDispute{}, err
	}

	return result.RecoveryDispute, nil
}

func CreateRiderRecoveryDispute(ctx context.Context, store db.Store, input CreateRiderRecoveryDisputeInput) (CreateRiderRecoveryDisputeResult, error) {
	var result CreateRiderRecoveryDisputeResult

	disputeCtx, err := getRecoveryDisputeContext(ctx, store, input.ClaimID, "rider")
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("claim not found or not eligible for recovery dispute"))
		}
		return result, err
	}

	if !disputeCtx.RiderID.Valid || disputeCtx.RiderID.Int64 != input.RiderID {
		return result, NewRequestError(http.StatusForbidden, errors.New("this claim is not related to your deliveries"))
	}

	now, windowDays := normalizeRecoveryDisputeWindow(input.Now, input.DisputeWindowDays)
	recoveryDisputeDeadline := disputeCtx.ClaimCreatedAt.Add(time.Duration(windowDays) * 24 * time.Hour)
	if now.After(recoveryDisputeDeadline) {
		return result, NewRequestError(http.StatusBadRequest, errors.New("申诉窗口期已过（索赔后7天内可申诉）"))
	}

	exists, err := store.CheckRecoveryDisputeExists(ctx, db.CheckRecoveryDisputeExistsParams{
		ClaimID:       input.ClaimID,
		AppellantType: "rider",
	})
	if err != nil {
		return result, err
	}

	if exists {
		recoveryDispute, err := store.GetRecoveryDisputeByClaim(ctx, db.GetRecoveryDisputeByClaimParams{
			ClaimID:       input.ClaimID,
			AppellantType: "rider",
		})
		if err != nil {
			return result, err
		}
		if recoveryDispute.AppellantType != "rider" || recoveryDispute.AppellantID != input.RiderID {
			return result, NewRequestError(http.StatusConflict, errors.New("recovery dispute already exists for this claim"))
		}
		result.RecoveryDispute = recoveryDispute
		result.AlreadyExists = true
		return result, nil
	}

	txResult, err := store.CreateRecoveryDisputeWithRecoveryTx(ctx, db.CreateRecoveryDisputeWithRecoveryTxParams{
		ClaimID:        input.ClaimID,
		RecoveryTarget: "rider",
		AppellantType:  "rider",
		AppellantID:    input.RiderID,
		Reason:         input.Reason,
		RegionID:       disputeCtx.RegionID,
	})
	if err != nil {
		if db.ErrorCode(err) == db.UniqueViolation {
			recoveryDispute, getErr := store.GetRecoveryDisputeByClaim(ctx, db.GetRecoveryDisputeByClaimParams{
				ClaimID:       input.ClaimID,
				AppellantType: "rider",
			})
			if getErr != nil {
				return result, getErr
			}
			if recoveryDispute.AppellantType != "rider" || recoveryDispute.AppellantID != input.RiderID {
				return result, NewRequestError(http.StatusConflict, errors.New("recovery dispute already exists for this claim"))
			}
			result.RecoveryDispute = recoveryDispute
			result.AlreadyExists = true
			return result, nil
		}
		return result, err
	}

	result.RecoveryDispute = txResult.RecoveryDispute
	return result, nil
}

func getRecoveryDisputeContext(ctx context.Context, store db.Store, claimID int64, recoveryTarget string) (recoveryDisputeContext, error) {
	recoveryCtx, err := store.GetClaimRecoveryContextByClaimIDAndTarget(ctx, db.GetClaimRecoveryContextByClaimIDAndTargetParams{
		ClaimID:        claimID,
		RecoveryTarget: pgtype.Text{String: recoveryTarget, Valid: recoveryTarget != ""},
	})
	if err == nil {
		return recoveryDisputeContext{
			ClaimID:        recoveryCtx.ClaimID,
			OrderID:        recoveryCtx.OrderID,
			MerchantID:     recoveryCtx.MerchantID,
			RegionID:       recoveryCtx.RegionID,
			RiderID:        recoveryCtx.RiderID,
			ClaimCreatedAt: recoveryCtx.ClaimCreatedAt,
			HasRecovery:    true,
		}, nil
	}
	if errors.Is(err, db.ErrRecordNotFound) {
		return recoveryDisputeContext{}, db.ErrRecordNotFound
	}

	return recoveryDisputeContext{}, err
}

func normalizeRecoveryDisputeWindow(now time.Time, windowDays int) (time.Time, int) {
	if windowDays <= 0 {
		windowDays = defaultRecoveryDisputeWindowDays
	}
	if now.IsZero() {
		now = time.Now()
	}
	return now, windowDays
}
