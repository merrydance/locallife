package logic

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

type AutomaticAppealResolution struct {
	Status      string
	ReviewNotes string
	DecisionID  pgtype.Int8
}

type AutomaticAppealResolutionResult struct {
	Resolution   AutomaticAppealResolution
	ReviewResult db.ReviewAppealWithCompensationTxResult
}

func EvaluateAutomaticAppealResolution(ctx context.Context, store db.Store, appeal db.Appeal) (AutomaticAppealResolution, error) {
	claimInfo, err := store.GetClaimForAppeal(ctx, appeal.ClaimID)
	if err != nil {
		return AutomaticAppealResolution{}, err
	}

	decisions, err := store.ListBehaviorDecisionsByOrder(ctx, pgtype.Int8{Int64: claimInfo.OrderID, Valid: true})
	if err != nil {
		return AutomaticAppealResolution{}, err
	}

	return DeriveAutomaticAppealResolution(appeal, decisions), nil
}

func ResolveAppealAutomatically(ctx context.Context, store db.Store, appeal db.Appeal) (AutomaticAppealResolutionResult, error) {
	resolution, err := EvaluateAutomaticAppealResolution(ctx, store, appeal)
	if err != nil {
		return AutomaticAppealResolutionResult{}, err
	}

	reviewResult, err := store.ReviewAppealWithCompensationTx(ctx, db.ReviewAppealWithCompensationTxParams{
		ID:                 appeal.ID,
		Status:             resolution.Status,
		ReviewerID:         pgtype.Int8{},
		ReviewNotes:        pgtype.Text{String: resolution.ReviewNotes, Valid: true},
		CompensationAmount: pgtype.Int8{},
	})
	if err != nil {
		return AutomaticAppealResolutionResult{}, err
	}

	return AutomaticAppealResolutionResult{
		Resolution:   resolution,
		ReviewResult: reviewResult,
	}, nil
}

func DeriveAutomaticAppealResolution(appeal db.Appeal, decisions []db.BehaviorDecision) AutomaticAppealResolution {
	resolution := AutomaticAppealResolution{
		Status:      "rejected",
		ReviewNotes: "系统复核确认最新行为判责仍指向当前申诉方，维持原判。",
	}

	latest, ok := findDecisionForAppeal(appeal, decisions)
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

	if latest.ResponsibleParty != appeal.AppellantType {
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

func BuildBehaviorAppealEvidence(appeal db.Appeal) string {
	return "appeal_id=" + strconv.FormatInt(appeal.ID, 10) + ",claim_id=" + strconv.FormatInt(appeal.ClaimID, 10)
}

func findDecisionForAppeal(appeal db.Appeal, decisions []db.BehaviorDecision) (db.BehaviorDecision, bool) {
	for _, decision := range decisions {
		if decision.ClaimID.Valid && decision.ClaimID.Int64 == appeal.ClaimID {
			return decision, true
		}
	}

	return db.BehaviorDecision{}, false
}
