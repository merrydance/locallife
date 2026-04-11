package api

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/rules"
	"github.com/rs/zerolog/log"
)

func (server *Server) recordRuleHit(ctx *gin.Context, input rules.Context, decision rules.Decision, actorRole string) {
	if server == nil || server.store == nil {
		return
	}
	if decision.RuleID <= 0 {
		return
	}

	inputs, err := json.Marshal(input)
	if err != nil {
		log.Warn().Err(err).Msg("rule_hit: marshal inputs")
		return
	}
	outputs, err := json.Marshal(decision)
	if err != nil {
		log.Warn().Err(err).Msg("rule_hit: marshal outputs")
		return
	}

	params := db.CreateRuleHitParams{
		RuleID:   decision.RuleID,
		Domain:   string(input.Domain),
		Decision: decision.Action,
		Inputs:   inputs,
		Outputs:  outputs,
	}
	if decision.Reason != "" {
		params.Reason = pgtype.Text{String: decision.Reason, Valid: true}
	}
	if decision.RuleVersionID > 0 {
		params.RuleVersionID = pgtype.Int8{Int64: decision.RuleVersionID, Valid: true}
	}
	if actorRole != "" {
		params.ActorRole = pgtype.Text{String: actorRole, Valid: true}
	}
	if input.UserID > 0 {
		params.ActorID = pgtype.Int8{Int64: input.UserID, Valid: true}
	}
	if input.RegionID > 0 {
		params.RegionID = pgtype.Int8{Int64: input.RegionID, Valid: true}
	}
	if input.MerchantID > 0 {
		params.MerchantID = pgtype.Int8{Int64: input.MerchantID, Valid: true}
	}

	if _, err := server.store.CreateRuleHit(ctx, params); err != nil {
		log.Warn().Err(err).Int64("rule_id", decision.RuleID).Msg("rule_hit: create")
	}
}
