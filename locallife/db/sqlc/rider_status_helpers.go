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

type RiderDepositThresholdSource string

const (
	RiderDepositThresholdSourceOperator RiderDepositThresholdSource = "operator"
	RiderDepositThresholdSourcePlatform RiderDepositThresholdSource = "platform"
	RiderDepositThresholdSourceDefault  RiderDepositThresholdSource = "default"
)

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

func ResolveRiderDepositThreshold(operatorDeposit int64, platformDeposit int64, hasPlatformConfig bool) (int64, RiderDepositThresholdSource) {
	if operatorDeposit > 0 {
		if operatorDeposit != DefaultRiderDepositThresholdFen {
			return operatorDeposit, RiderDepositThresholdSourceOperator
		}
		if hasPlatformConfig && platformDeposit > 0 {
			return platformDeposit, RiderDepositThresholdSourcePlatform
		}
		return operatorDeposit, RiderDepositThresholdSourceDefault
	}

	if hasPlatformConfig && platformDeposit > 0 {
		return platformDeposit, RiderDepositThresholdSourcePlatform
	}

	return DefaultRiderDepositThresholdFen, RiderDepositThresholdSourceDefault
}

func decodeRiderDepositConfigAmount(config PlatformConfig) (int64, bool, error) {
	if len(config.ConfigValue) == 0 {
		return 0, false, nil
	}

	var payload riderDepositConfigValue
	if err := json.Unmarshal(config.ConfigValue, &payload); err != nil {
		return 0, false, fmt.Errorf("decode rider deposit config: %w", err)
	}
	if payload.AmountFen <= 0 {
		return 0, false, nil
	}

	return payload.AmountFen, true, nil
}

func GetEffectiveRiderDepositThreshold(ctx context.Context, q riderDepositThresholdReader, regionID pgtype.Int8) (int64, error) {
	operatorDeposit := int64(0)
	operatorID := int64(0)
	if regionID.Valid && regionID.Int64 > 0 {
		operator, err := q.GetActiveOperatorByRegion(ctx, regionID.Int64)
		if err == nil {
			operatorID = operator.ID
			operatorDeposit = operator.RiderDeposit
			if operatorID > 0 && operatorDeposit == DefaultRiderDepositThresholdFen {
				config, configErr := q.GetPlatformConfig(ctx, GetPlatformConfigParams{
					ConfigKey: PlatformConfigKeyRiderDepositFen,
					ScopeType: PlatformConfigScopeOperator,
					ScopeID:   pgtype.Int8{Int64: operatorID, Valid: true},
				})
				if configErr == nil {
					amountFen, configured, decodeErr := decodeRiderDepositConfigAmount(config)
					if decodeErr != nil {
						return 0, decodeErr
					}
					if configured {
						return amountFen, nil
					}
				} else if configErr != ErrRecordNotFound {
					return 0, fmt.Errorf("get operator-scoped rider deposit config: %w", configErr)
				}
			}
			if operatorDeposit > 0 && operatorDeposit != DefaultRiderDepositThresholdFen {
				return operatorDeposit, nil
			}
		} else if err != ErrRecordNotFound {
			return 0, fmt.Errorf("get active operator by region: %w", err)
		}
	}

	platformDeposit := int64(0)
	hasPlatformConfig := false
	config, err := q.GetPlatformConfig(ctx, GetPlatformConfigParams{
		ConfigKey: PlatformConfigKeyRiderDepositFen,
		ScopeType: PlatformConfigScopeGlobal,
		ScopeID:   pgtype.Int8{Valid: false},
	})
	if err != nil {
		if err == ErrRecordNotFound {
			threshold, _ := ResolveRiderDepositThreshold(operatorDeposit, 0, false)
			return threshold, nil
		}
		return 0, fmt.Errorf("get rider deposit platform config: %w", err)
	}

	amountFen, configured, decodeErr := decodeRiderDepositConfigAmount(config)
	if decodeErr != nil {
		return 0, decodeErr
	}
	if configured {
		platformDeposit = amountFen
		hasPlatformConfig = true
	}

	threshold, _ := ResolveRiderDepositThreshold(operatorDeposit, platformDeposit, hasPlatformConfig)
	return threshold, nil
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
