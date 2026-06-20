package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

const operatorRuleWriteProxyDisabledMessage = "operator rule write proxy is disabled"

func rejectOperatorRuleWriteProxy(ctx *gin.Context) {
	ctx.JSON(http.StatusForbidden, errorResponse(errors.New(operatorRuleWriteProxyDisabledMessage)))
}

// listOperatorRulesProxy 运营商规则列表（代理）
// @Summary 运营商规则列表
// @Description 仅返回当前运营商可见的规则（按 region 过滤）
// @Tags 运营商-规则管理
// @Produce json
// @Param limit query int false "分页大小"
// @Param offset query int false "分页偏移"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/operators/me/rules [get]
// @Security BearerAuth
func (server *Server) listOperatorRulesProxy(ctx *gin.Context) {
	operator, ok := GetOperatorFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("operator not found in context")))
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

	allRules, err := server.store.ListRules(ctx, db.ListRulesParams{Limit: limit, Offset: offset})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	filtered := make([]db.Rule, 0, len(allRules))
	for _, rule := range allRules {
		versions, err := server.store.ListRuleVersionsByRule(ctx, rule.ID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		if ruleVersionsMatchRegion(versions, operator.RegionID) {
			filtered = append(filtered, rule)
		}
	}

	ctx.JSON(http.StatusOK, rulesListResponse{Rules: filtered, Count: len(filtered)})
}

// getOperatorRuleProxy 获取运营商规则详情（代理）
// @Summary 获取运营商规则详情
// @Tags 运营商-规则管理
// @Produce json
// @Param id path int true "规则ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/operators/me/rules/{id} [get]
// @Security BearerAuth
func (server *Server) getOperatorRuleProxy(ctx *gin.Context) {
	operator, ok := GetOperatorFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("operator not found in context")))
		return
	}

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

	filtered := filterRuleVersionsByRegion(versions, operator.RegionID)
	if len(filtered) == 0 {
		ctx.JSON(http.StatusNotFound, errorResponse(errors.New("rule not found")))
		return
	}

	ctx.JSON(http.StatusOK, ruleDetailWithVersionsResponse{Rule: rule, Versions: filtered})
}

// createOperatorRuleProxy 创建运营商规则（代理，已关闭）
// @Summary 创建运营商规则（已关闭）
// @Description 运营商规则引擎写入当前不向运营商端开放；规则发布、回滚、禁用由平台规则治理入口承接。
// @Tags 运营商-规则管理
// @Produce json
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /v1/operators/me/rules [post]
// @Security BearerAuth
func (server *Server) createOperatorRuleProxy(ctx *gin.Context) {
	rejectOperatorRuleWriteProxy(ctx)
}

// createOperatorRuleVersionProxy 创建运营商规则版本（代理，已关闭）
// @Summary 创建运营商规则版本（已关闭）
// @Description 运营商规则引擎写入当前不向运营商端开放；规则版本变更必须走平台规则治理入口。
// @Tags 运营商-规则管理
// @Produce json
// @Param id path int true "规则ID"
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /v1/operators/me/rules/{id}/versions [post]
// @Security BearerAuth
func (server *Server) createOperatorRuleVersionProxy(ctx *gin.Context) {
	rejectOperatorRuleWriteProxy(ctx)
}

// publishOperatorRuleProxy 发布运营商规则（代理，已关闭）
// @Summary 发布运营商规则（已关闭）
// @Description 运营商规则引擎写入当前不向运营商端开放；规则发布必须走平台规则治理入口。
// @Tags 运营商-规则管理
// @Produce json
// @Param id path int true "规则ID"
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /v1/operators/me/rules/{id}/publish [post]
// @Security BearerAuth
func (server *Server) publishOperatorRuleProxy(ctx *gin.Context) {
	rejectOperatorRuleWriteProxy(ctx)
}

// rollbackOperatorRuleProxy 回滚运营商规则（代理，已关闭）
// @Summary 回滚运营商规则（已关闭）
// @Description 运营商规则引擎写入当前不向运营商端开放；规则回滚必须走平台规则治理入口。
// @Tags 运营商-规则管理
// @Produce json
// @Param id path int true "规则ID"
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /v1/operators/me/rules/{id}/rollback [post]
// @Security BearerAuth
func (server *Server) rollbackOperatorRuleProxy(ctx *gin.Context) {
	rejectOperatorRuleWriteProxy(ctx)
}

// disableOperatorRuleProxy 禁用运营商规则（代理，已关闭）
// @Summary 禁用运营商规则（已关闭）
// @Description 运营商规则引擎写入当前不向运营商端开放；规则禁用必须走平台规则治理入口。
// @Tags 运营商-规则管理
// @Produce json
// @Param id path int true "规则ID"
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /v1/operators/me/rules/{id}/disable [post]
// @Security BearerAuth
func (server *Server) disableOperatorRuleProxy(ctx *gin.Context) {
	rejectOperatorRuleWriteProxy(ctx)
}

// listOperatorRuleHitsProxy 查询运营商规则命中记录（代理）
// @Summary 查询运营商规则命中记录
// @Tags 运营商-规则管理
// @Produce json
// @Param rule_id query int true "规则ID"
// @Param limit query int false "分页大小"
// @Param offset query int false "分页偏移"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/operators/me/rules/hits [get]
// @Security BearerAuth
func (server *Server) listOperatorRuleHitsProxy(ctx *gin.Context) {
	operator, ok := GetOperatorFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("operator not found in context")))
		return
	}

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

	hits, err := server.store.ListRuleHitsByRuleAndRegion(ctx, db.ListRuleHitsByRuleAndRegionParams{
		RuleID:   ruleID,
		RegionID: pgtype.Int8{Int64: operator.RegionID, Valid: true},
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, ruleHitsListResponse{Hits: hits, Count: len(hits)})
}

func normalizeRegionIDs(value interface{}) ([]int64, error) {
	switch v := value.(type) {
	case float64:
		return []int64{int64(v)}, nil
	case int64:
		return []int64{v}, nil
	case int:
		return []int64{int64(v)}, nil
	case []interface{}:
		ids := make([]int64, 0, len(v))
		for _, item := range v {
			if f, ok := item.(float64); ok {
				ids = append(ids, int64(f))
				continue
			}
			if i, ok := item.(int64); ok {
				ids = append(ids, i)
				continue
			}
			if i, ok := item.(int); ok {
				ids = append(ids, int64(i))
				continue
			}
			return nil, errors.New("invalid region_id type")
		}
		return ids, nil
	case []int64:
		return v, nil
	case []int:
		ids := make([]int64, 0, len(v))
		for _, id := range v {
			ids = append(ids, int64(id))
		}
		return ids, nil
	default:
		return nil, errors.New("invalid region_id type")
	}
}

func ruleVersionsMatchRegion(versions []db.RuleVersion, regionID int64) bool {
	for _, version := range versions {
		if ruleVersionMatchesRegion(version, regionID) {
			return true
		}
	}
	return false
}

func filterRuleVersionsByRegion(versions []db.RuleVersion, regionID int64) []db.RuleVersion {
	filtered := make([]db.RuleVersion, 0, len(versions))
	for _, version := range versions {
		if ruleVersionMatchesRegion(version, regionID) {
			filtered = append(filtered, version)
		}
	}
	return filtered
}

func ruleVersionMatchesRegion(version db.RuleVersion, regionID int64) bool {
	if regionID <= 0 {
		return false
	}
	if ruleVersionRegionMatch(version.Scope, regionID) {
		return true
	}
	return ruleVersionRegionMatch(version.GrayConfig, regionID)
}

func ruleVersionRegionMatch(payload []byte, regionID int64) bool {
	if len(payload) == 0 {
		return false
	}
	data := map[string]interface{}{}
	if err := json.Unmarshal(payload, &data); err != nil {
		return false
	}
	value, ok := data["region_id"]
	if !ok {
		return false
	}
	ids, err := normalizeRegionIDs(value)
	if err != nil {
		return false
	}
	for _, id := range ids {
		if id == regionID {
			return true
		}
	}
	return false
}
