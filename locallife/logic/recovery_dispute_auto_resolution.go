package logic

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

type AutomaticRecoveryDisputeResolution struct {
	Status      string
	ReviewNotes string
	DecisionID  pgtype.Int8
}

type AutomaticRecoveryDisputeResolutionResult struct {
	Resolution   AutomaticRecoveryDisputeResolution
	ReviewResult db.ReviewRecoveryDisputeWithCompensationTxResult
}

func EvaluateAutomaticRecoveryDisputeResolution(ctx context.Context, store db.Store, recoveryDispute db.RecoveryDispute) (AutomaticRecoveryDisputeResolution, error) {
	disputeCtx, err := getRecoveryDisputeContext(ctx, store, recoveryDispute.ClaimID)
	if err != nil {
		return AutomaticRecoveryDisputeResolution{}, err
	}

	decisions, err := store.ListBehaviorDecisionsByOrder(ctx, pgtype.Int8{Int64: disputeCtx.OrderID, Valid: true})
	if err != nil {
		return AutomaticRecoveryDisputeResolution{}, err
	}

	return DeriveAutomaticRecoveryDisputeResolution(recoveryDispute, decisions), nil
}

func ResolveRecoveryDisputeAutomatically(ctx context.Context, store db.Store, recoveryDispute db.RecoveryDispute) (AutomaticRecoveryDisputeResolutionResult, error) {
	resolution, err := EvaluateAutomaticRecoveryDisputeResolution(ctx, store, recoveryDispute)
	if err != nil {
		return AutomaticRecoveryDisputeResolutionResult{}, err
	}

	reviewResult, err := store.ReviewRecoveryDisputeWithCompensationTx(ctx, db.ReviewRecoveryDisputeWithCompensationTxParams{
		ID:                 recoveryDispute.ID,
		Status:             resolution.Status,
		DecisionID:         resolution.DecisionID,
		ReviewerID:         pgtype.Int8{},
		ReviewNotes:        pgtype.Text{String: resolution.ReviewNotes, Valid: true},
		CompensationAmount: pgtype.Int8{},
	})
	if err != nil {
		return AutomaticRecoveryDisputeResolutionResult{}, err
	}

	return AutomaticRecoveryDisputeResolutionResult{
		Resolution:   resolution,
		ReviewResult: reviewResult,
	}, nil
}

func DeriveAutomaticRecoveryDisputeResolution(recoveryDispute db.RecoveryDispute, decisions []db.BehaviorDecision) AutomaticRecoveryDisputeResolution {
	resolution := AutomaticRecoveryDisputeResolution{
		Status:      "rejected",
		ReviewNotes: "系统复核确认最新行为判责仍指向当前申诉方，维持原判。",
	}

	latest, ok := findDecisionForRecoveryDispute(recoveryDispute, decisions)
	if !ok {
		resolution.ReviewNotes = "系统未找到当前索赔对应的行为判责快照，维持原判。"
		return resolution
	}

	resolution.DecisionID = pgtype.Int8{Int64: latest.ID, Valid: true}

	effectiveStatus := latest.EffectiveStatus
	if effectiveStatus == "" {
		effectiveStatus = db.BehaviorEffectiveStatusEffective
	}
	if effectiveStatus != db.BehaviorEffectiveStatusEffective {
		resolution.Status = "approved"
		resolution.ReviewNotes = "系统复核发现相关行为判责已失效，自动撤销原追偿安排。"
		return resolution
	}

	if latest.ResponsibleParty != recoveryDispute.AppellantType {
		resolution.Status = "approved"
		resolution.ReviewNotes = "系统复核发现最新行为判责已不再指向当前申诉方，自动撤销原追偿安排。"
		return resolution
	}

	if latest.CompensationSource == "platform" {
		resolution.Status = "approved"
		resolution.ReviewNotes = "系统复核发现当前行为判责已转为平台承担，自动撤销原追偿安排。"
		return resolution
	}

	if latest.DecisionMode.Valid && latest.DecisionMode.String == db.BehaviorDecisionModeUserRestricted {
		resolution.Status = "approved"
		resolution.ReviewNotes = "系统复核发现当前行为判责已转为平台承担，自动撤销原追偿安排。"
	}

	return resolution
}

func findDecisionForRecoveryDispute(recoveryDispute db.RecoveryDispute, decisions []db.BehaviorDecision) (db.BehaviorDecision, bool) {
	for _, decision := range decisions {
		if decision.ClaimID.Valid && decision.ClaimID.Int64 == recoveryDispute.ClaimID {
			return decision, true
		}
	}

	return db.BehaviorDecision{}, false
}
