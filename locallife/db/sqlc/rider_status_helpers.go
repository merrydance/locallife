package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

type riderDepositConfigValue struct {
	AmountFen int64 `json:"amount_fen"`
}

type riderDepositThresholdReader interface {
	GetPlatformConfig(ctx context.Context, arg GetPlatformConfigParams) (PlatformConfig, error)
	GetActiveOperatorByRegion(ctx context.Context, regionID int64) (Operator, error)
}

type riderOperationalStatusReconciler interface {
	riderDepositThresholdReader
	ListRiderActiveDeliveries(ctx context.Context, riderID pgtype.Int8) ([]Delivery, error)
	UpdateRiderStatus(ctx context.Context, arg UpdateRiderStatusParams) (Rider, error)
	UpdateRiderOnlineStatus(ctx context.Context, arg UpdateRiderOnlineStatusParams) (Rider, error)
}

func maybeSetRiderOfflineWhenNotEligible(ctx context.Context, q riderOperationalStatusReconciler, rider Rider) (Rider, error) {
	if rider.Status == RiderStatusActive || !rider.IsOnline {
		return rider, nil
	}

	activeDeliveries, err := q.ListRiderActiveDeliveries(ctx, pgtype.Int8{Int64: rider.ID, Valid: true})
	if err != nil {
		return rider, fmt.Errorf("list rider active deliveries: %w", err)
	}
	if len(activeDeliveries) > 0 {
		return rider, nil
	}

	updatedRider, err := q.UpdateRiderOnlineStatus(ctx, UpdateRiderOnlineStatusParams{
		ID:       rider.ID,
		IsOnline: false,
	})
	if err != nil {
		return rider, fmt.Errorf("set rider offline after status reconcile: %w", err)
	}

	return updatedRider, nil
}

func GetEffectiveRiderDepositThreshold(ctx context.Context, q riderDepositThresholdReader, regionID pgtype.Int8) (int64, error) {
	if regionID.Valid && regionID.Int64 > 0 {
		operator, err := q.GetActiveOperatorByRegion(ctx, regionID.Int64)
		if err == nil {
			if operator.RiderDeposit > 0 {
				return operator.RiderDeposit, nil
			}
		} else if err != ErrRecordNotFound {
			return 0, fmt.Errorf("get active operator by region: %w", err)
		}
	}

	config, err := q.GetPlatformConfig(ctx, GetPlatformConfigParams{
		ConfigKey: PlatformConfigKeyRiderDepositFen,
		ScopeType: PlatformConfigScopeGlobal,
		ScopeID:   pgtype.Int8{Valid: false},
	})
	if err != nil {
		if err == ErrRecordNotFound {
			return DefaultRiderDepositThresholdFen, nil
		}
		return 0, fmt.Errorf("get rider deposit platform config: %w", err)
	}

	if len(config.ConfigValue) == 0 {
		return DefaultRiderDepositThresholdFen, nil
	}

	var payload riderDepositConfigValue
	if err := json.Unmarshal(config.ConfigValue, &payload); err != nil {
		return 0, fmt.Errorf("decode rider deposit platform config: %w", err)
	}
	if payload.AmountFen <= 0 {
		return DefaultRiderDepositThresholdFen, nil
	}

	return payload.AmountFen, nil
}

func ReconcileRiderOperationalStatus(ctx context.Context, q riderOperationalStatusReconciler, rider Rider) (Rider, error) {
	if rider.Status == RiderStatusSuspended {
		if !rider.IsOnline {
			return rider, nil
		}

		updatedRider, err := q.UpdateRiderOnlineStatus(ctx, UpdateRiderOnlineStatusParams{
			ID:       rider.ID,
			IsOnline: false,
		})
		if err != nil {
			return rider, fmt.Errorf("set suspended rider offline: %w", err)
		}

		return updatedRider, nil
	}
	if rider.Status != RiderStatusApproved && rider.Status != RiderStatusActive {
		return rider, fmt.Errorf("unsupported rider status for deposit transition: %s", rider.Status)
	}

	threshold, err := GetEffectiveRiderDepositThreshold(ctx, q, rider.RegionID)
	if err != nil {
		return rider, err
	}

	desiredStatus := RiderStatusApproved
	if rider.DepositAmount >= threshold {
		desiredStatus = RiderStatusActive
	}

	updatedRider := rider
	if rider.Status != desiredStatus {
		var err error
		updatedRider, err = q.UpdateRiderStatus(ctx, UpdateRiderStatusParams{
			ID:     rider.ID,
			Status: desiredStatus,
		})
		if err != nil {
			return rider, fmt.Errorf("update rider status to %s: %w", desiredStatus, err)
		}
	}

	if desiredStatus != RiderStatusActive {
		updatedRider, err = maybeSetRiderOfflineWhenNotEligible(ctx, q, updatedRider)
		if err != nil {
			return rider, err
		}
	}

	return updatedRider, nil
}
