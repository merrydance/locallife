package api

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/algorithm"
	db "github.com/merrydance/locallife/db/sqlc"
)

type claimFinalAdjudicatorPlatformConfig struct {
	Enabled     bool                                  `json:"enabled"`
	GrayRegions []int64                               `json:"gray_regions"`
	Thresholds  algorithm.ClaimFinalAdjudicatorConfig `json:"thresholds"`
}

type claimFinalAdjudicatorFactSnapshot struct {
	OrderID              int64                       `json:"order_id"`
	ClaimType            string                      `json:"claim_type"`
	ClaimAmount          int64                       `json:"claim_amount"`
	BaseResponsibleParty string                      `json:"base_responsible_party"`
	ResponsibleParty     string                      `json:"responsible_party"`
	DecisionMode         string                      `json:"decision_mode"`
	CompensationSource   string                      `json:"compensation_source"`
	User                 algorithm.PartyWindowStats  `json:"user"`
	Rider                *algorithm.PartyWindowStats `json:"rider,omitempty"`
	Merchant             algorithm.PartyWindowStats  `json:"merchant"`
	ReasonCodes          []string                    `json:"reason_codes"`
}

func resolveClaimAddressID(order db.Order) pgtype.Int8 {
	if !order.AddressID.Valid {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: order.AddressID.Int64, Valid: true}
}

func loadClaimFinalAdjudicatorConfig(ctx *gin.Context, store db.Store, regionID int64) (algorithm.ClaimFinalAdjudicatorConfig, bool, error) {
	config := algorithm.DefaultClaimFinalAdjudicatorConfig()
	platformConfig, err := store.GetPlatformConfig(ctx, db.GetPlatformConfigParams{
		ConfigKey: "claim_final_adjudicator",
		ScopeType: "global",
		ScopeID:   pgtype.Int8{Valid: false},
	})
	if err != nil {
		if isNotFoundError(err) {
			return config, false, nil
		}
		return config, false, fmt.Errorf("load claim final adjudicator platform config: %w", err)
	}
	if len(platformConfig.ConfigValue) == 0 {
		return config, false, nil
	}

	var payload claimFinalAdjudicatorPlatformConfig
	if err := json.Unmarshal(platformConfig.ConfigValue, &payload); err != nil {
		return config, false, fmt.Errorf("decode claim final adjudicator platform config: %w", err)
	}
	if !payload.Enabled {
		return config, false, nil
	}
	if len(payload.GrayRegions) > 0 {
		matched := false
		for _, id := range payload.GrayRegions {
			if id == regionID {
				matched = true
				break
			}
		}
		if !matched {
			return config, false, nil
		}
	}
	return mergeClaimFinalAdjudicatorConfig(config, payload.Thresholds), true, nil
}

func mergeClaimFinalAdjudicatorConfig(base algorithm.ClaimFinalAdjudicatorConfig, override algorithm.ClaimFinalAdjudicatorConfig) algorithm.ClaimFinalAdjudicatorConfig {
	if override.MinUserOrders30d > 0 {
		base.MinUserOrders30d = override.MinUserOrders30d
	}
	if override.MinUserClaims30d > 0 {
		base.MinUserClaims30d = override.MinUserClaims30d
	}
	if override.UserClaimRate30d > 0 {
		base.UserClaimRate30d = override.UserClaimRate30d
	}
	if override.UserClaims7d > 0 {
		base.UserClaims7d = override.UserClaims7d
	}
	if override.UserClaimRate7d > 0 {
		base.UserClaimRate7d = override.UserClaimRate7d
	}
	if override.MinRiderOrders30d > 0 {
		base.MinRiderOrders30d = override.MinRiderOrders30d
	}
	if override.RiderAbnormalRate30d > 0 {
		base.RiderAbnormalRate30d = override.RiderAbnormalRate30d
	}
	if override.MinMerchantOrders30d > 0 {
		base.MinMerchantOrders30d = override.MinMerchantOrders30d
	}
	if override.MerchantAbnormalRate30d > 0 {
		base.MerchantAbnormalRate30d = override.MerchantAbnormalRate30d
	}
	return base
}

func loadClaimFinalPartyStats(ctx *gin.Context, store db.Store, entityType string, entityID int64, start7d, start30d, endDate pgtype.Date) (algorithm.PartyWindowStats, error) {
	stats := algorithm.PartyWindowStats{EntityType: entityType, EntityID: entityID}
	if entityID == 0 {
		return stats, nil
	}
	summary7d, err := store.GetAbnormalStatsSummary(ctx, db.GetAbnormalStatsSummaryParams{
		EntityType: entityType,
		EntityID:   entityID,
		StatDate:   start7d,
		StatDate_2: endDate,
	})
	if err != nil {
		return stats, fmt.Errorf("load %s 7d abnormal stats: %w", entityType, err)
	}
	stats.TotalOrders7d = summary7d.TotalOrders
	stats.AbnormalClaims7d = summary7d.AbnormalClaims

	summary30d, err := store.GetAbnormalStatsSummary(ctx, db.GetAbnormalStatsSummaryParams{
		EntityType: entityType,
		EntityID:   entityID,
		StatDate:   start30d,
		StatDate_2: endDate,
	})
	if err != nil {
		return stats, fmt.Errorf("load %s 30d abnormal stats: %w", entityType, err)
	}
	stats.TotalOrders30d = summary30d.TotalOrders
	stats.AbnormalClaims30d = summary30d.AbnormalClaims
	stats.NetAbnormalClaims30d = summary30d.AbnormalClaims
	return stats, nil
}

func enrichClaimFinalUserBehaviorStats(ctx *gin.Context, store db.Store, stats *algorithm.PartyWindowStats, deviceFingerprint string, addressID pgtype.Int8, now time.Time) error {
	if stats == nil || stats.EntityID == 0 {
		return nil
	}
	effectSummary, err := store.GetBehaviorEffectSummary(ctx, db.GetBehaviorEffectSummaryParams{
		EntityType: "user",
		EntityID:   stats.EntityID,
		StartAt:    now.AddDate(0, 0, -30),
		EndAt:      now,
	})
	if err != nil {
		return fmt.Errorf("load user behavior effect summary: %w", err)
	}
	stats.MaliciousConfirmedClaims = effectSummary.MaliciousConfirmedClaims
	stats.NetAbnormalClaims30d = int32Saturating(effectSummary.EffectiveClaims)

	if deviceFingerprint != "" {
		userIDs, err := store.GetUsersByDeviceFingerprint(ctx, pgtype.Text{String: deviceFingerprint, Valid: true})
		if err != nil {
			return fmt.Errorf("load shared device graph: %w", err)
		}
		stats.SharedDeviceOtherUsers = countClaimFinalOtherUsers(userIDs, stats.EntityID)
	}
	if addressID.Valid {
		rows, err := store.GetUsersByAddressID(ctx, addressID.Int64)
		if err != nil {
			return fmt.Errorf("load shared address graph: %w", err)
		}
		var otherUsers int32
		for _, row := range rows {
			if row.UserID != stats.EntityID {
				otherUsers++
			}
		}
		stats.SharedAddressOtherUsers = otherUsers
	}
	return nil
}

func enrichClaimFinalLiabilityStats(ctx *gin.Context, store db.Store, stats *algorithm.PartyWindowStats, now time.Time) error {
	if stats == nil || stats.EntityID == 0 {
		return nil
	}
	effectSummary, err := store.GetBehaviorEffectSummary(ctx, db.GetBehaviorEffectSummaryParams{
		EntityType: stats.EntityType,
		EntityID:   stats.EntityID,
		StartAt:    now.AddDate(0, 0, -30),
		EndAt:      now,
	})
	if err != nil {
		return fmt.Errorf("load %s behavior effect summary: %w", stats.EntityType, err)
	}
	if effectSummary.EffectiveLiabilityClaims > int64(stats.AbnormalClaims30d) {
		stats.AbnormalClaims30d = int32Saturating(effectSummary.EffectiveLiabilityClaims)
	}
	return nil
}

func countClaimFinalOtherUsers(userIDs []int64, currentUserID int64) int32 {
	var count int32
	seen := make(map[int64]struct{}, len(userIDs))
	for _, userID := range userIDs {
		if userID == currentUserID {
			continue
		}
		if _, exists := seen[userID]; exists {
			continue
		}
		seen[userID] = struct{}{}
		count++
	}
	return count
}

func int32Saturating(value int64) int32 {
	if value > 2147483647 {
		return 2147483647
	}
	if value < -2147483648 {
		return -2147483648
	}
	return int32(value)
}

func applyClaimFinalAdjudication(decision *algorithm.Decision, adjudication algorithm.ClaimFinalAdjudicationResult, scoreBreakdown []byte, factSnapshot []byte) {
	if decision == nil || adjudication.DecisionMode == "" {
		return
	}
	decision.Type = adjudication.DecisionMode
	decision.Approved = true
	decision.Reason = adjudication.Reason
	decision.BehaviorStatus = adjudication.BehaviorStatus
	decision.CompensationSource = adjudication.CompensationSource
	decision.ScoreBreakdown = scoreBreakdown
	decision.FactSnapshot = factSnapshot
	if adjudication.DecisionMode == algorithm.DecisionModeUserRestricted {
		decision.Warning = adjudication.Reason
	}
}
