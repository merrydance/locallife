package logic

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

const defaultAppealWindowDays = 7

type CreateMerchantAppealInput struct {
	MerchantID       int64
	ClaimID          int64
	Reason           string
	AppealWindowDays int
	Now              time.Time
}

type CreateRiderAppealInput struct {
	RiderID          int64
	ClaimID          int64
	Reason           string
	AppealWindowDays int
	Now              time.Time
}

type CreateRiderAppealResult struct {
	Appeal        db.Appeal
	AlreadyExists bool
}

type ListMerchantAppealsInput struct {
	MerchantID int64
	Status     string
	Limit      int32
	Offset     int32
}

type MerchantAppealsResult struct {
	Appeals []db.ListMerchantAppealsForMerchantRow
	Total   int64
}

type GetMerchantAppealDetailInput struct {
	MerchantID int64
	AppealID   int64
}

type ListRiderAppealsInput struct {
	RiderID int64
	Limit   int32
	Offset  int32
}

type RiderAppealsResult struct {
	Appeals []db.ListRiderAppealsRow
	Total   int64
}

type GetRiderAppealDetailInput struct {
	RiderID  int64
	AppealID int64
}

func CreateMerchantAppeal(ctx context.Context, store db.Store, input CreateMerchantAppealInput) (db.Appeal, error) {
	claimInfo, err := store.GetClaimForAppeal(ctx, input.ClaimID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.Appeal{}, NewRequestError(http.StatusNotFound, errors.New("claim not found or not eligible for appeal"))
		}
		return db.Appeal{}, err
	}

	if claimInfo.MerchantID != input.MerchantID {
		return db.Appeal{}, NewRequestError(http.StatusForbidden, errors.New("this claim does not belong to your merchant"))
	}

	windowDays := input.AppealWindowDays
	if windowDays <= 0 {
		windowDays = defaultAppealWindowDays
	}

	now := input.Now
	if now.IsZero() {
		now = time.Now()
	}

	appealDeadline := claimInfo.CreatedAt.Add(time.Duration(windowDays) * 24 * time.Hour)
	if now.After(appealDeadline) {
		return db.Appeal{}, NewRequestError(http.StatusBadRequest, errors.New("申诉窗口期已过（索赔后7天内可申诉）"))
	}

	exists, err := store.CheckAppealExists(ctx, db.CheckAppealExistsParams{
		ClaimID:       input.ClaimID,
		AppellantType: "merchant",
	})
	if err != nil {
		return db.Appeal{}, err
	}
	if exists {
		return db.Appeal{}, NewRequestError(http.StatusConflict, errors.New("appeal already exists for this claim"))
	}

	appeal, err := store.CreateAppeal(ctx, db.CreateAppealParams{
		ClaimID:       input.ClaimID,
		AppellantType: "merchant",
		AppellantID:   input.MerchantID,
		Reason:        input.Reason,
		RegionID:      claimInfo.RegionID,
	})
	if err != nil {
		return db.Appeal{}, err
	}

	if recovery, recoveryErr := store.GetClaimRecoveryByClaimID(ctx, input.ClaimID); recoveryErr == nil {
		_, _ = store.MarkClaimRecoveryAppealed(ctx, recovery.ID)
	}

	return appeal, nil
}

func ListMerchantAppeals(ctx context.Context, store db.Store, input ListMerchantAppealsInput) (MerchantAppealsResult, error) {
	var result MerchantAppealsResult

	appeals, err := store.ListMerchantAppealsForMerchant(ctx, db.ListMerchantAppealsForMerchantParams{
		AppellantID: input.MerchantID,
		Status: pgtype.Text{
			String: input.Status,
			Valid:  input.Status != "",
		},
		Limit:  input.Limit,
		Offset: input.Offset,
	})
	if err != nil {
		return result, err
	}

	total, err := store.CountMerchantAppealsForMerchant(ctx, db.CountMerchantAppealsForMerchantParams{
		AppellantID: input.MerchantID,
		Status: pgtype.Text{
			String: input.Status,
			Valid:  input.Status != "",
		},
	})
	if err != nil {
		total = int64(len(appeals))
	}

	result.Appeals = appeals
	result.Total = total
	return result, nil
}

func GetMerchantAppealDetail(ctx context.Context, store db.Store, input GetMerchantAppealDetailInput) (db.GetMerchantAppealDetailRow, error) {
	appeal, err := store.GetMerchantAppealDetail(ctx, db.GetMerchantAppealDetailParams{
		ID:          input.AppealID,
		AppellantID: input.MerchantID,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.GetMerchantAppealDetailRow{}, NewRequestError(http.StatusNotFound, errors.New("appeal not found"))
		}
		return db.GetMerchantAppealDetailRow{}, err
	}

	return appeal, nil
}

func CreateRiderAppeal(ctx context.Context, store db.Store, input CreateRiderAppealInput) (CreateRiderAppealResult, error) {
	var result CreateRiderAppealResult

	claimInfo, err := store.GetClaimForAppeal(ctx, input.ClaimID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("claim not found or not eligible for appeal"))
		}
		return result, err
	}

	if !claimInfo.RiderID.Valid || claimInfo.RiderID.Int64 != input.RiderID {
		return result, NewRequestError(http.StatusForbidden, errors.New("this claim is not related to your deliveries"))
	}

	windowDays := input.AppealWindowDays
	if windowDays <= 0 {
		windowDays = defaultAppealWindowDays
	}

	now := input.Now
	if now.IsZero() {
		now = time.Now()
	}

	appealDeadline := claimInfo.CreatedAt.Add(time.Duration(windowDays) * 24 * time.Hour)
	if now.After(appealDeadline) {
		return result, NewRequestError(http.StatusBadRequest, errors.New("申诉窗口期已过（索赔后7天内可申诉）"))
	}

	exists, err := store.CheckAppealExists(ctx, db.CheckAppealExistsParams{
		ClaimID:       input.ClaimID,
		AppellantType: "rider",
	})
	if err != nil {
		return result, err
	}

	if exists {
		appeal, err := store.GetAppealByClaim(ctx, db.GetAppealByClaimParams{
			ClaimID:       input.ClaimID,
			AppellantType: "rider",
		})
		if err != nil {
			return result, err
		}
		if appeal.AppellantType != "rider" || appeal.AppellantID != input.RiderID {
			return result, NewRequestError(http.StatusConflict, errors.New("appeal already exists for this claim"))
		}
		result.Appeal = appeal
		result.AlreadyExists = true
		return result, nil
	}

	appeal, err := store.CreateAppeal(ctx, db.CreateAppealParams{
		ClaimID:       input.ClaimID,
		AppellantType: "rider",
		AppellantID:   input.RiderID,
		Reason:        input.Reason,
		RegionID:      claimInfo.RegionID,
	})
	if err != nil {
		return result, err
	}

	if recovery, recoveryErr := store.GetClaimRecoveryByClaimID(ctx, input.ClaimID); recoveryErr == nil {
		_, _ = store.MarkClaimRecoveryAppealed(ctx, recovery.ID)
	}

	result.Appeal = appeal
	return result, nil
}

func ListRiderAppeals(ctx context.Context, store db.Store, input ListRiderAppealsInput) (RiderAppealsResult, error) {
	var result RiderAppealsResult

	appeals, err := store.ListRiderAppeals(ctx, db.ListRiderAppealsParams{
		AppellantID: input.RiderID,
		Limit:       input.Limit,
		Offset:      input.Offset,
	})
	if err != nil {
		return result, err
	}

	total, err := store.CountRiderAppeals(ctx, input.RiderID)
	if err != nil {
		total = int64(len(appeals))
	}

	result.Appeals = appeals
	result.Total = total
	return result, nil
}

func GetRiderAppealDetail(ctx context.Context, store db.Store, input GetRiderAppealDetailInput) (db.GetRiderAppealDetailRow, error) {
	appeal, err := store.GetRiderAppealDetail(ctx, db.GetRiderAppealDetailParams{
		ID:          input.AppealID,
		AppellantID: input.RiderID,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.GetRiderAppealDetailRow{}, NewRequestError(http.StatusNotFound, errors.New("appeal not found"))
		}
		return db.GetRiderAppealDetailRow{}, err
	}

	return appeal, nil
}
