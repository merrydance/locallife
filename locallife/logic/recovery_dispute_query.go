package logic

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

const defaultRecoveryDisputeWindowDays = 7

type ListMerchantRecoveryDisputesInput struct {
	MerchantID int64
	Status     string
	Limit      int32
	Offset     int32
}

type MerchantRecoveryDisputesResult struct {
	Disputes []db.ListMerchantRecoveryDisputesForMerchantRow
	Total    int64
}

type GetMerchantRecoveryDisputeDetailInput struct {
	MerchantID int64
	DisputeID  int64
}

type ListRiderRecoveryDisputesInput struct {
	RiderID int64
	Status  string
	Limit   int32
	Offset  int32
}

type RiderRecoveryDisputesResult struct {
	Disputes []db.ListRiderRecoveryDisputesRow
	Total    int64
}

type GetRiderRecoveryDisputeDetailInput struct {
	RiderID   int64
	DisputeID int64
}

func ListMerchantRecoveryDisputes(ctx context.Context, store db.Store, input ListMerchantRecoveryDisputesInput) (MerchantRecoveryDisputesResult, error) {
	var result MerchantRecoveryDisputesResult

	disputes, err := store.ListMerchantRecoveryDisputesForMerchant(ctx, db.ListMerchantRecoveryDisputesForMerchantParams{
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

	total, err := store.CountMerchantRecoveryDisputesForMerchant(ctx, db.CountMerchantRecoveryDisputesForMerchantParams{
		AppellantID: input.MerchantID,
		Status: pgtype.Text{
			String: input.Status,
			Valid:  input.Status != "",
		},
	})
	if err != nil {
		total = int64(len(disputes))
	}

	result.Disputes = disputes
	result.Total = total
	return result, nil
}

func GetMerchantRecoveryDisputeDetail(ctx context.Context, store db.Store, input GetMerchantRecoveryDisputeDetailInput) (db.GetMerchantRecoveryDisputeDetailRow, error) {
	dispute, err := store.GetMerchantRecoveryDisputeDetail(ctx, db.GetMerchantRecoveryDisputeDetailParams{
		ID:          input.DisputeID,
		AppellantID: input.MerchantID,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.GetMerchantRecoveryDisputeDetailRow{}, NewRequestError(http.StatusNotFound, errors.New("recovery dispute not found"))
		}
		return db.GetMerchantRecoveryDisputeDetailRow{}, err
	}

	return dispute, nil
}

func ListRiderRecoveryDisputes(ctx context.Context, store db.Store, input ListRiderRecoveryDisputesInput) (RiderRecoveryDisputesResult, error) {
	var result RiderRecoveryDisputesResult

	disputes, err := store.ListRiderRecoveryDisputes(ctx, db.ListRiderRecoveryDisputesParams{
		AppellantID: input.RiderID,
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

	total, err := store.CountRiderRecoveryDisputes(ctx, db.CountRiderRecoveryDisputesParams{
		AppellantID: input.RiderID,
		Status: pgtype.Text{
			String: input.Status,
			Valid:  input.Status != "",
		},
	})
	if err != nil {
		total = int64(len(disputes))
	}

	result.Disputes = disputes
	result.Total = total
	return result, nil
}

func GetRiderRecoveryDisputeDetail(ctx context.Context, store db.Store, input GetRiderRecoveryDisputeDetailInput) (db.GetRiderRecoveryDisputeDetailRow, error) {
	dispute, err := store.GetRiderRecoveryDisputeDetail(ctx, db.GetRiderRecoveryDisputeDetailParams{
		ID:          input.DisputeID,
		AppellantID: input.RiderID,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.GetRiderRecoveryDisputeDetailRow{}, NewRequestError(http.StatusNotFound, errors.New("recovery dispute not found"))
		}
		return db.GetRiderRecoveryDisputeDetailRow{}, err
	}

	return dispute, nil
}
