package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
)

type rulesListResponse struct {
	Rules interface{} `json:"rules"`
	Count int         `json:"count"`
}

type ruleDetailWithVersionsResponse struct {
	Rule     interface{} `json:"rule"`
	Versions interface{} `json:"versions"`
}

// listRules 列出规则
// @Summary 列出规则
// @Tags 规则引擎
// @Produce json
// @Param limit query int false "分页大小"
// @Param offset query int false "分页偏移"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/platform/rules [get]
// @Security BearerAuth
func (server *Server) listRules(ctx *gin.Context) {
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

	rulesList, err := server.store.ListRules(ctx, db.ListRulesParams{Limit: limit, Offset: offset})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, rulesListResponse{Rules: rulesList, Count: len(rulesList)})
}

// getRule 获取规则详情
// @Summary 获取规则详情
// @Tags 规则引擎
// @Produce json
// @Param id path int true "规则ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/platform/rules/{id} [get]
// @Security BearerAuth
func (server *Server) getRule(ctx *gin.Context) {
	ruleID, err := parseIDParam(ctx, "id")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	rule, err := server.store.GetRule(ctx, ruleID)
	if err != nil {
		if isNotFoundError(err) || errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("rule not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	versions, err := server.store.ListRuleVersionsByRule(ctx, ruleID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, ruleDetailWithVersionsResponse{Rule: rule, Versions: versions})
}
