package logic

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/rs/zerolog/log"
)

const (
	claimRecoveryReleaseVisibilityPending  = "pending"
	claimRecoveryReleaseVisibilityReleased = "released"
	claimRecoveryReleaseVisibilityRetrying = "retrying"
	claimRecoveryReleaseVisibilitySyncing  = "syncing"
)

const (
	claimRecoveryReleasePendingMessage = "追偿已结清，服务限制正在解除，可稍后刷新查看"
	claimRecoveryReleaseRetryMessage   = "追偿已结清，服务限制解除正在重试，可稍后刷新查看"
	claimRecoveryReleaseSyncingMessage = "追偿已结清，服务限制状态正在同步，可稍后刷新查看"
)

type ClaimRecoveryReleaseVisibility struct {
	Status  string
	Message string
}

type claimRecoveryReleaseActionVisibilityDetail struct {
	Action       string `json:"action"`
	ClaimID      int64  `json:"claim_id"`
	RecoveryID   int64  `json:"recovery_id"`
	OrderID      int64  `json:"order_id"`
	TargetEntity string `json:"target_entity"`
}

func resolveMerchantClaimRecoveryReleaseVisibility(ctx context.Context, store db.Store, recovery db.ClaimRecovery) *ClaimRecoveryReleaseVisibility {
	if recovery.Status != db.ClaimRecoveryStatusPaid && recovery.Status != db.ClaimRecoveryStatusWaived {
		return nil
	}
	if !recovery.RecoveryTarget.Valid || recovery.RecoveryTarget.String != "merchant" {
		return nil
	}
	checkedDecisionIDs := make(map[int64]struct{}, 1)
	var action *db.BehaviorAction
	if recovery.DecisionID.Valid {
		checkedDecisionIDs[recovery.DecisionID.Int64] = struct{}{}
		var ok bool
		action, ok = lookupClaimRecoveryReleaseActionByDecision(ctx, store, recovery, recovery.DecisionID.Int64)
		if !ok {
			return newClaimRecoveryReleaseVisibilitySyncing()
		}
	}

	if action == nil {
		var ok bool
		action, ok = lookupClaimRecoveryReleaseActionByOrderDecisions(ctx, store, recovery, checkedDecisionIDs)
		if !ok {
			return newClaimRecoveryReleaseVisibilitySyncing()
		}
	}

	if action == nil {
		return newClaimRecoveryReleaseVisibilitySyncing()
	}

	return claimRecoveryReleaseVisibilityFromAction(action)
}

func claimRecoveryReleaseVisibilityFromAction(action *db.BehaviorAction) *ClaimRecoveryReleaseVisibility {
	switch strings.ToLower(strings.TrimSpace(action.Status)) {
	case "created", "running":
		return &ClaimRecoveryReleaseVisibility{
			Status:  claimRecoveryReleaseVisibilityPending,
			Message: claimRecoveryReleasePendingMessage,
		}
	case "failed":
		return &ClaimRecoveryReleaseVisibility{
			Status:  claimRecoveryReleaseVisibilityRetrying,
			Message: claimRecoveryReleaseRetryMessage,
		}
	case "success":
		return &ClaimRecoveryReleaseVisibility{
			Status: claimRecoveryReleaseVisibilityReleased,
		}
	default:
		return newClaimRecoveryReleaseVisibilitySyncing()
	}
}

func findClaimRecoveryReleaseAction(actions []db.BehaviorAction, recovery db.ClaimRecovery) *db.BehaviorAction {
	var matched *db.BehaviorAction
	for i := range actions {
		action := &actions[i]
		if action.ActionType != "release" || action.TargetEntity != recovery.RecoveryTarget.String {
			continue
		}

		var detail claimRecoveryReleaseActionVisibilityDetail
		if err := json.Unmarshal(action.Detail, &detail); err != nil {
			continue
		}
		if detail.Action != "release_recovery_suspension" {
			continue
		}
		if detail.RecoveryID != recovery.ID || detail.ClaimID != recovery.ClaimID || detail.OrderID != recovery.OrderID {
			continue
		}
		if detail.TargetEntity != "" && detail.TargetEntity != recovery.RecoveryTarget.String {
			continue
		}
		matched = action
	}
	return matched
}

func lookupClaimRecoveryReleaseActionByDecision(ctx context.Context, store db.Store, recovery db.ClaimRecovery, decisionID int64) (*db.BehaviorAction, bool) {
	actions, err := store.ListBehaviorActionsByDecision(ctx, decisionID)
	if err != nil {
		log.Warn().
			Err(err).
			Int64("recovery_id", recovery.ID).
			Int64("decision_id", decisionID).
			Msg("claim recovery release visibility lookup failed")
		return nil, false
	}
	return findClaimRecoveryReleaseAction(actions, recovery), true
}

func lookupClaimRecoveryReleaseActionByOrderDecisions(ctx context.Context, store db.Store, recovery db.ClaimRecovery, checkedDecisionIDs map[int64]struct{}) (*db.BehaviorAction, bool) {
	if recovery.OrderID <= 0 {
		return nil, true
	}

	decisions, err := store.ListBehaviorDecisionsByOrder(ctx, pgtype.Int8{Int64: recovery.OrderID, Valid: true})
	if err != nil {
		log.Warn().
			Err(err).
			Int64("recovery_id", recovery.ID).
			Int64("order_id", recovery.OrderID).
			Msg("claim recovery release visibility order decision lookup failed")
		return nil, false
	}

	for _, decision := range decisions {
		if _, checked := checkedDecisionIDs[decision.ID]; checked {
			continue
		}
		action, ok := lookupClaimRecoveryReleaseActionByDecision(ctx, store, recovery, decision.ID)
		if !ok {
			return nil, false
		}
		if action != nil {
			return action, true
		}
	}
	return nil, true
}

func newClaimRecoveryReleaseVisibilitySyncing() *ClaimRecoveryReleaseVisibility {
	return &ClaimRecoveryReleaseVisibility{
		Status:  claimRecoveryReleaseVisibilitySyncing,
		Message: claimRecoveryReleaseSyncingMessage,
	}
}
