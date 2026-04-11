package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
)

type ruleHitsListResponse struct {
	Hits  interface{} `json:"hits"`
	Count int         `json:"count"`
}

// listRuleHits 查询规则命中记录（平台侧）
// @Summary 查询规则命中记录
// @Description 平台管理员查询规则命中记录
// @Tags 规则引擎
// @Produce json
// @Param rule_id query int true "规则ID"
// @Param limit query int false "分页大小"
// @Param offset query int false "分页偏移"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/platform/rules/hits [get]
// @Security BearerAuth
func (server *Server) listRuleHits(ctx *gin.Context) {
	ruleIDStr := ctx.Query("rule_id")
	if ruleIDStr == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("rule_id is required")))
		return
	}
	ruleID, err := strconv.ParseInt(ruleIDStr, 10, 64)
	if err != nil || ruleID <= 0 {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid rule_id")))
		return
	}

	limit := int32(50)
	if v := ctx.Query("limit"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 32); err == nil && parsed > 0 {
			if parsed > 200 {
				parsed = 200
			}
			limit = int32(parsed)
		}
	}
	offset := int32(0)
	if v := ctx.Query("offset"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 32); err == nil && parsed >= 0 {
			offset = int32(parsed)
		}
	}

	hits, err := server.store.ListRuleHitsByRule(ctx, db.ListRuleHitsByRuleParams{
		RuleID: ruleID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, ruleHitsListResponse{Hits: hits, Count: len(hits)})
}
