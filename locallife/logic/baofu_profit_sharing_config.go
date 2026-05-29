package logic

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

type BaofuProfitSharingConfigResolution struct {
	PlatformRateBps int32
	OperatorRateBps int32
	OperatorID      int64
}

func ResolveBaofuProfitSharingConfig(ctx context.Context, store db.Store, orderSource string, merchant db.Merchant) (BaofuProfitSharingConfigResolution, error) {
	return resolveBaofuProfitSharingConfig(ctx, store, orderSource, merchant, true)
}

func ResolveBaofuProfitSharingConfigStrict(ctx context.Context, store db.Store, orderSource string, merchant db.Merchant) (BaofuProfitSharingConfigResolution, error) {
	return resolveBaofuProfitSharingConfig(ctx, store, orderSource, merchant, false)
}

// resolveBaofuProfitSharingConfig resolves the active rate config and optional operator receiver.
func resolveBaofuProfitSharingConfig(ctx context.Context, store db.Store, orderSource string, merchant db.Merchant, allowMissingOperator bool) (BaofuProfitSharingConfigResolution, error) {
	if store == nil || merchant.ID <= 0 || strings.TrimSpace(orderSource) == "" {
		return BaofuProfitSharingConfigResolution{}, ErrBaofuProfitSharingInvalidAmount
	}
	config, err := store.GetActiveProfitSharingConfig(ctx, db.GetActiveProfitSharingConfigParams{
		OrderSource: strings.TrimSpace(orderSource),
		MerchantID:  pgtype.Int8{Int64: merchant.ID, Valid: true},
		RegionID:    pgtype.Int8{Int64: merchant.RegionID, Valid: merchant.RegionID > 0},
	})
	if err != nil {
		return BaofuProfitSharingConfigResolution{}, fmt.Errorf("get active profit sharing config: %w", err)
	}

	resolution := BaofuProfitSharingConfigResolution{
		PlatformRateBps: profitSharingPercentToBps(config.PlatformRate),
		OperatorRateBps: profitSharingPercentToBps(config.OperatorRate),
	}
	if merchant.RegionID <= 0 || config.OperatorRate <= 0 {
		return resolution, nil
	}

	operator, err := store.GetActiveOperatorByRegion(ctx, merchant.RegionID)
	if err != nil {
		if allowMissingOperator && errors.Is(err, db.ErrRecordNotFound) {
			return resolution, nil
		}
		return BaofuProfitSharingConfigResolution{}, fmt.Errorf("get active operator for baofu profit sharing: %w", err)
	}
	resolution.OperatorID = operator.ID
	return resolution, nil
}

func BaofuProfitSharingOrderSource(order db.Order) string {
	if order.ReservationID.Valid && order.OrderType == orderTypeDineIn {
		return db.OrderTypeReservation
	}
	return order.OrderType
}
