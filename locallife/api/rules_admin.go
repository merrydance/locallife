package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
)

type createRuleRequest struct {
	Name     string `json:"name" binding:"required"`
	Category string `json:"category" binding:"required"`
	Status   string `json:"status"`
}

type createRuleVersionRequest struct {
	Version     int32                  `json:"version"`
	Status      string                 `json:"status"`
	Priority    int32                  `json:"priority"`
	Scope       map[string]interface{} `json:"scope"`
	Condition   map[string]interface{} `json:"condition"`
	Action      map[string]interface{} `json:"action"`
	GrayConfig  map[string]interface{} `json:"gray_config"`
	EffectiveAt *time.Time             `json:"effective_at"`
	ExpiresAt   *time.Time             `json:"expires_at"`
}

type publishRuleRequest struct {
	VersionID int64 `json:"version_id" binding:"required,min=1"`
}

type disableRuleRequest struct {
	Reason string `json:"reason"`
}

type rollbackRuleRequest struct {
	VersionID int64 `json:"version_id"`
}

// createRule 创建规则（草案）
func (server *Server) createRule(ctx *gin.Context) {
	var req createRuleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if req.Status == "" {
		req.Status = "draft"
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	rule, err := server.store.CreateRule(ctx, db.CreateRuleParams{
		Name:      req.Name,
		Category:  req.Category,
		Status:    req.Status,
		CreatedBy: pgtype.Int8{Int64: authPayload.UserID, Valid: true},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	_ = server.recordRuleAudit(ctx, rule.ID, 0, "create", authPayload.UserID, RoleAdmin, map[string]interface{}{
		"name":     rule.Name,
		"category": rule.Category,
		"status":   rule.Status,
	})

	ctx.JSON(http.StatusCreated, rule)
}

// createRuleVersion 创建规则版本（草案）
func (server *Server) createRuleVersion(ctx *gin.Context) {
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

	scope, _ := json.Marshal(req.Scope)
	condition, _ := json.Marshal(req.Condition)
	action, _ := json.Marshal(req.Action)
	grayConfig, _ := json.Marshal(req.GrayConfig)

	params := db.CreateRuleVersionParams{
		RuleID:     ruleID,
		Version:    req.Version,
		Status:     req.Status,
		Priority:   req.Priority,
		Scope:      scope,
		Condition:  condition,
		Action:     action,
		GrayConfig: grayConfig,
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

	_ = server.recordRuleAudit(ctx, ruleID, version.ID, "create_version", payload.UserID, RoleAdmin, map[string]interface{}{
		"version": version.Version,
		"status":  version.Status,
	})

	ctx.JSON(http.StatusCreated, version)
}

// publishRule 绑定当前版本（草案）
func (server *Server) publishRule(ctx *gin.Context) {
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

	_ = server.recordRuleAudit(ctx, rule.ID, req.VersionID, "publish", payload.UserID, RoleAdmin, map[string]interface{}{
		"current_version_id": req.VersionID,
	})

	ctx.JSON(http.StatusOK, rule)
}

// disableRule 禁用规则（草案）
func (server *Server) disableRule(ctx *gin.Context) {
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

	_ = server.recordRuleAudit(ctx, rule.ID, 0, "disable", payload.UserID, RoleAdmin, map[string]interface{}{
		"status": "disabled",
		"reason": req.Reason,
	})

	ctx.JSON(http.StatusOK, rule)
}

// rollbackRule 回滚到指定版本（草案）
func (server *Server) rollbackRule(ctx *gin.Context) {
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

	_ = server.recordRuleAudit(ctx, rule.ID, version.ID, "rollback", payload.UserID, RoleAdmin, map[string]interface{}{
		"current_version_id": version.ID,
		"version":            version.Version,
	})

	ctx.JSON(http.StatusOK, rule)
}

func (server *Server) recordRuleAudit(ctx *gin.Context, ruleID int64, versionID int64, action string, actorID int64, actorRole string, detail map[string]interface{}) error {
	if server == nil || server.store == nil {
		return errors.New("store not initialized")
	}
	payload, _ := json.Marshal(detail)
	params := db.CreateRuleAuditParams{
		RuleID: ruleID,
		Action: action,
		Detail: payload,
	}
	if versionID > 0 {
		params.RuleVersionID = pgtype.Int8{Int64: versionID, Valid: true}
	}
	if actorID > 0 {
		params.ActorID = pgtype.Int8{Int64: actorID, Valid: true}
	}
	if actorRole != "" {
		params.ActorRole = pgtype.Text{String: actorRole, Valid: true}
	}
	_, err := server.store.CreateRuleAudit(ctx, params)
	return err
}

func parseIDParam(ctx *gin.Context, key string) (int64, error) {
	value := ctx.Param(key)
	if value == "" {
		return 0, errors.New("missing id")
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, errors.New("invalid id")
	}
	return parsed, nil
}
