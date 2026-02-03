package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
)

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

	ctx.JSON(http.StatusOK, gin.H{
		"rules": filtered,
		"count": len(filtered),
	})
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

	ctx.JSON(http.StatusOK, gin.H{
		"rule":     rule,
		"versions": filtered,
	})
}

// createOperatorRuleProxy 创建运营商规则（代理）
// @Summary 创建运营商规则
// @Tags 运营商-规则管理
// @Accept json
// @Produce json
// @Param request body createRuleRequest true "创建规则请求"
// @Success 201 {object} db.Rule
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/operators/me/rules [post]
// @Security BearerAuth
func (server *Server) createOperatorRuleProxy(ctx *gin.Context) {
	operator, ok := GetOperatorFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("operator not found in context")))
		return
	}

	var req createRuleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if req.Status == "" {
		req.Status = "draft"
	}

	payload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	rule, err := server.store.CreateRule(ctx, db.CreateRuleParams{
		Name:      req.Name,
		Category:  req.Category,
		Status:    req.Status,
		CreatedBy: pgtype.Int8{Int64: payload.UserID, Valid: true},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	_ = server.recordRuleAudit(ctx, rule.ID, 0, "create", payload.UserID, RoleOperator, map[string]interface{}{
		"name":     rule.Name,
		"category": rule.Category,
		"status":   rule.Status,
		"region":   operator.RegionID,
	})

	ctx.JSON(http.StatusCreated, rule)
}

// createOperatorRuleVersionProxy 创建运营商规则版本（代理）
// @Summary 创建运营商规则版本
// @Tags 运营商-规则管理
// @Accept json
// @Produce json
// @Param id path int true "规则ID"
// @Param request body createRuleVersionRequest true "创建规则版本请求"
// @Success 201 {object} db.RuleVersion
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/operators/me/rules/{id}/versions [post]
// @Security BearerAuth
func (server *Server) createOperatorRuleVersionProxy(ctx *gin.Context) {
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

	var req createRuleVersionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if req.Status == "" {
		req.Status = "draft"
	}
	if req.Version == 0 {
		req.Version = 1
	}

	payload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	scope, err := ensureRegionConstraint(req.Scope, operator.RegionID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	grayConfig, err := ensureRegionConstraint(req.GrayConfig, operator.RegionID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	scopeJSON, _ := json.Marshal(scope)
	conditionJSON, _ := json.Marshal(req.Condition)
	actionJSON, _ := json.Marshal(req.Action)
	grayJSON, _ := json.Marshal(grayConfig)

	params := db.CreateRuleVersionParams{
		RuleID:     ruleID,
		Version:    req.Version,
		Status:     req.Status,
		Priority:   req.Priority,
		Scope:      scopeJSON,
		Condition:  conditionJSON,
		Action:     actionJSON,
		GrayConfig: grayJSON,
		CreatedBy:  pgtype.Int8{Int64: payload.UserID, Valid: true},
	}
	if req.EffectiveAt != nil {
		params.EffectiveAt = pgtype.Timestamptz{Time: *req.EffectiveAt, Valid: true}
	}
	if req.ExpiresAt != nil {
		params.ExpiresAt = pgtype.Timestamptz{Time: *req.ExpiresAt, Valid: true}
	}

	version, err := server.store.CreateRuleVersion(ctx, params)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	_ = server.recordRuleAudit(ctx, ruleID, version.ID, "create_version", payload.UserID, RoleOperator, map[string]interface{}{
		"version": version.Version,
		"status":  version.Status,
		"region":  operator.RegionID,
	})

	ctx.JSON(http.StatusCreated, version)
}

// publishOperatorRuleProxy 发布运营商规则（代理）
// @Summary 发布运营商规则
// @Tags 运营商-规则管理
// @Accept json
// @Produce json
// @Param id path int true "规则ID"
// @Param request body publishRuleRequest true "发布请求"
// @Success 200 {object} db.Rule
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/operators/me/rules/{id}/publish [post]
// @Security BearerAuth
func (server *Server) publishOperatorRuleProxy(ctx *gin.Context) {
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

	var req publishRuleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	version, err := server.store.GetRuleVersion(ctx, req.VersionID)
	if err != nil {
		if isNotFoundError(err) || errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("rule version not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if version.RuleID != ruleID {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("rule version does not belong to rule")))
		return
	}
	if version.Status != "published" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("rule version status must be published")))
		return
	}
	if !ruleVersionMatchesRegion(version, operator.RegionID) {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("rule version out of operator region")))
		return
	}

	payload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	rule, err := server.store.UpdateRuleStatus(ctx, db.UpdateRuleStatusParams{
		ID:               ruleID,
		Status:           "active",
		CurrentVersionID: pgtype.Int8{Int64: req.VersionID, Valid: true},
	})
	if err != nil {
		if isNotFoundError(err) || errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("rule not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	_ = server.recordRuleAudit(ctx, rule.ID, req.VersionID, "publish", payload.UserID, RoleOperator, map[string]interface{}{
		"current_version_id": req.VersionID,
		"region":             operator.RegionID,
	})

	ctx.JSON(http.StatusOK, rule)
}

// rollbackOperatorRuleProxy 回滚运营商规则（代理）
// @Summary 回滚运营商规则
// @Tags 运营商-规则管理
// @Accept json
// @Produce json
// @Param id path int true "规则ID"
// @Param request body rollbackRuleRequest true "回滚请求"
// @Success 200 {object} db.Rule
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/operators/me/rules/{id}/rollback [post]
// @Security BearerAuth
func (server *Server) rollbackOperatorRuleProxy(ctx *gin.Context) {
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

	var req rollbackRuleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
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

	var version db.RuleVersion
	if req.VersionID > 0 {
		version, err = server.store.GetRuleVersion(ctx, req.VersionID)
		if err != nil {
			if isNotFoundError(err) || errors.Is(err, db.ErrRecordNotFound) {
				ctx.JSON(http.StatusNotFound, errorResponse(errors.New("rule version not found")))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		if version.RuleID != ruleID {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("rule version does not belong to rule")))
			return
		}
		if version.Status != "published" {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("rule version status must be published")))
			return
		}
		if !ruleVersionMatchesRegion(version, operator.RegionID) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("rule version out of operator region")))
			return
		}
	} else {
		versions, err := server.store.ListRuleVersionsByRule(ctx, ruleID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		found := false
		for _, candidate := range versions {
			if candidate.Status != "published" {
				continue
			}
			if rule.CurrentVersionID.Valid && candidate.ID == rule.CurrentVersionID.Int64 {
				continue
			}
			if !ruleVersionMatchesRegion(candidate, operator.RegionID) {
				continue
			}
			version = candidate
			found = true
			break
		}
		if !found {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("no rollback version available")))
			return
		}
	}

	payload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	rule, err = server.store.UpdateRuleStatus(ctx, db.UpdateRuleStatusParams{
		ID:               ruleID,
		Status:           "active",
		CurrentVersionID: pgtype.Int8{Int64: version.ID, Valid: true},
	})
	if err != nil {
		if isNotFoundError(err) || errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("rule not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	_ = server.recordRuleAudit(ctx, rule.ID, version.ID, "rollback", payload.UserID, RoleOperator, map[string]interface{}{
		"current_version_id": version.ID,
		"version":            version.Version,
		"region":             operator.RegionID,
	})

	ctx.JSON(http.StatusOK, rule)
}

// disableOperatorRuleProxy 禁用运营商规则（代理）
// @Summary 禁用运营商规则
// @Tags 运营商-规则管理
// @Accept json
// @Produce json
// @Param id path int true "规则ID"
// @Param request body disableRuleRequest false "禁用请求"
// @Success 200 {object} db.Rule
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/operators/me/rules/{id}/disable [post]
// @Security BearerAuth
func (server *Server) disableOperatorRuleProxy(ctx *gin.Context) {
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

	var req disableRuleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	versions, err := server.store.ListRuleVersionsByRule(ctx, ruleID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if !ruleVersionsMatchRegion(versions, operator.RegionID) {
		ctx.JSON(http.StatusNotFound, errorResponse(errors.New("rule not found")))
		return
	}

	payload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	rule, err := server.store.UpdateRuleStatus(ctx, db.UpdateRuleStatusParams{
		ID:               ruleID,
		Status:           "disabled",
		CurrentVersionID: pgtype.Int8{Valid: false},
	})
	if err != nil {
		if isNotFoundError(err) || errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("rule not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	_ = server.recordRuleAudit(ctx, rule.ID, 0, "disable", payload.UserID, RoleOperator, map[string]interface{}{
		"status": "disabled",
		"reason": req.Reason,
		"region": operator.RegionID,
	})

	ctx.JSON(http.StatusOK, rule)
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

	ctx.JSON(http.StatusOK, gin.H{
		"hits":  hits,
		"count": len(hits),
	})
}

func ensureRegionConstraint(input map[string]interface{}, regionID int64) (map[string]interface{}, error) {
	if regionID <= 0 {
		return nil, errors.New("invalid operator region")
	}
	if input == nil {
		input = map[string]interface{}{}
	}
	value, ok := input["region_id"]
	if !ok {
		input["region_id"] = []int64{regionID}
		return input, nil
	}
	ids, err := normalizeRegionIDs(value)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		input["region_id"] = []int64{regionID}
		return input, nil
	}
	for _, id := range ids {
		if id != regionID {
			return nil, errors.New("region_id out of operator scope")
		}
	}
	input["region_id"] = []int64{regionID}
	return input, nil
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
