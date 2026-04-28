package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
)

type listProfitSharingReceiverLifecycleTargetsRequest struct {
	OwnerType  string `form:"owner_type" binding:"omitempty,oneof=operator rider"`
	OwnerID    int64  `form:"owner_id" binding:"omitempty,min=1"`
	SyncStatus string `form:"sync_status" binding:"omitempty,oneof=pending processing synced failed skipped"`
	Page       int32  `form:"page" binding:"omitempty,min=1"`
	Limit      int32  `form:"limit" binding:"omitempty,min=1,max=100"`
}

type profitSharingReceiverLifecycleTargetItem struct {
	ID               int64   `json:"id"`
	Provider         string  `json:"provider"`
	Channel          string  `json:"channel"`
	OwnerType        string  `json:"owner_type"`
	OwnerID          int64   `json:"owner_id"`
	ReceiverType     string  `json:"receiver_type"`
	AppID            string  `json:"appid"`
	DesiredState     string  `json:"desired_state"`
	SyncStatus       string  `json:"sync_status"`
	AttemptCount     int32   `json:"attempt_count"`
	NextRetryAt      *string `json:"next_retry_at,omitempty"`
	LastErrorCode    *string `json:"last_error_code,omitempty"`
	LastErrorMessage *string `json:"last_error_message,omitempty"`
	LastAttemptAt    *string `json:"last_attempt_at,omitempty"`
	SyncedAt         *string `json:"synced_at,omitempty"`
	SkippedAt        *string `json:"skipped_at,omitempty"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
}

type listProfitSharingReceiverLifecycleTargetsResponse struct {
	Items      []profitSharingReceiverLifecycleTargetItem `json:"items"`
	Total      int64                                      `json:"total"`
	Page       int32                                      `json:"page"`
	Limit      int32                                      `json:"limit"`
	TotalPages int64                                      `json:"total_pages"`
	HasMore    bool                                       `json:"has_more"`
}

type getProfitSharingReceiverLifecycleTargetURIRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

type getProfitSharingReceiverLifecycleTargetResponse struct {
	Target profitSharingReceiverLifecycleTargetItem `json:"target"`
}

type listProfitSharingReceiverLifecycleAttemptsRequest struct {
	Page  int32 `form:"page" binding:"omitempty,min=1"`
	Limit int32 `form:"limit" binding:"omitempty,min=1,max=100"`
}

type profitSharingReceiverLifecycleAttemptItem struct {
	ID                int64   `json:"id"`
	TargetID          int64   `json:"target_id"`
	Action            string  `json:"action"`
	Status            string  `json:"status"`
	IdempotentSuccess bool    `json:"idempotent_success"`
	ErrorCode         *string `json:"error_code,omitempty"`
	ErrorMessage      *string `json:"error_message,omitempty"`
	StartedAt         string  `json:"started_at"`
	FinishedAt        *string `json:"finished_at,omitempty"`
	CreatedAt         string  `json:"created_at"`
}

type listProfitSharingReceiverLifecycleAttemptsResponse struct {
	Target     profitSharingReceiverLifecycleTargetItem    `json:"target"`
	Items      []profitSharingReceiverLifecycleAttemptItem `json:"items"`
	Total      int64                                       `json:"total"`
	Page       int32                                       `json:"page"`
	Limit      int32                                       `json:"limit"`
	TotalPages int64                                       `json:"total_pages"`
	HasMore    bool                                        `json:"has_more"`
}

func newProfitSharingReceiverLifecycleTargetItem(target db.ProfitSharingReceiverTarget) profitSharingReceiverLifecycleTargetItem {
	return profitSharingReceiverLifecycleTargetItem{
		ID:               target.ID,
		Provider:         target.Provider,
		Channel:          target.Channel,
		OwnerType:        target.OwnerType,
		OwnerID:          target.OwnerID,
		ReceiverType:     target.ReceiverType,
		AppID:            target.Appid,
		DesiredState:     target.DesiredState,
		SyncStatus:       target.SyncStatus,
		AttemptCount:     target.AttemptCount,
		NextRetryAt:      profitSharingReceiverLifecycleTimePtr(target.NextRetryAt),
		LastErrorCode:    profitSharingReceiverLifecycleTextPtr(target.LastErrorCode),
		LastErrorMessage: profitSharingReceiverLifecycleTextPtr(target.LastErrorMessage),
		LastAttemptAt:    profitSharingReceiverLifecycleTimePtr(target.LastAttemptAt),
		SyncedAt:         profitSharingReceiverLifecycleTimePtr(target.SyncedAt),
		SkippedAt:        profitSharingReceiverLifecycleTimePtr(target.SkippedAt),
		CreatedAt:        target.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        target.UpdatedAt.Format(time.RFC3339),
	}
}

func newProfitSharingReceiverLifecycleAttemptItem(attempt db.ProfitSharingReceiverAttempt) profitSharingReceiverLifecycleAttemptItem {
	return profitSharingReceiverLifecycleAttemptItem{
		ID:                attempt.ID,
		TargetID:          attempt.TargetID,
		Action:            attempt.Action,
		Status:            attempt.Status,
		IdempotentSuccess: attempt.IdempotentSuccess,
		ErrorCode:         profitSharingReceiverLifecycleTextPtr(attempt.ErrorCode),
		ErrorMessage:      profitSharingReceiverLifecycleTextPtr(attempt.ErrorMessage),
		StartedAt:         attempt.StartedAt.Format(time.RFC3339),
		FinishedAt:        profitSharingReceiverLifecycleTimePtr(attempt.FinishedAt),
		CreatedAt:         attempt.CreatedAt.Format(time.RFC3339),
	}
}

func profitSharingReceiverLifecycleTextPtr(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	text := value.String
	return &text
}

func profitSharingReceiverLifecycleTimePtr(value pgtype.Timestamptz) *string {
	if !value.Valid {
		return nil
	}
	formatted := value.Time.Format(time.RFC3339)
	return &formatted
}

func profitSharingReceiverLifecycleTotalPages(total int64, limit int32) int64 {
	if total == 0 || limit <= 0 {
		return 0
	}
	return (total + int64(limit) - 1) / int64(limit)
}

func profitSharingReceiverLifecycleTextFilter(value string) pgtype.Text {
	if value == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: value, Valid: true}
}

func profitSharingReceiverLifecycleOwnerIDFilter(value int64) pgtype.Int8 {
	if value == 0 {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: value, Valid: true}
}

// listProfitSharingReceiverLifecycleTargets 获取分账接收方生命周期 target 列表
// @Summary 获取分账接收方生命周期 target 列表
// @Description 平台只读查询 receiver lifecycle target，用于排查 pending/failed/synced 状态；响应不返回 openid、receiver name、account hash、display name hash 或微信原始 payload
// @Tags Platform
// @Accept json
// @Produce json
// @Param owner_type query string false "owner 类型" Enums(operator,rider)
// @Param owner_id query int false "owner ID"
// @Param sync_status query string false "同步状态" Enums(pending,processing,synced,failed,skipped)
// @Param page query int false "页码" default(1) minimum(1)
// @Param limit query int false "每页数量" default(20) minimum(1) maximum(100)
// @Security BearerAuth
// @Success 200 {object} listProfitSharingReceiverLifecycleTargetsResponse "target 列表"
// @Failure 400 {object} errorRes "请求参数错误"
// @Failure 401 {object} errorRes "未授权"
// @Failure 403 {object} errorRes "权限不足"
// @Failure 500 {object} errorRes "服务器内部错误"
// @Router /v1/platform/profit-sharing/receiver-lifecycle/targets [get]
func (server *Server) listProfitSharingReceiverLifecycleTargets(ctx *gin.Context) {
	var req listProfitSharingReceiverLifecycleTargetsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	setDefaultProfitSharingReceiverLifecyclePagination(&req.Page, &req.Limit)

	params := db.ListProfitSharingReceiverTargetsParams{
		OwnerType:   profitSharingReceiverLifecycleTextFilter(req.OwnerType),
		OwnerID:     profitSharingReceiverLifecycleOwnerIDFilter(req.OwnerID),
		SyncStatus:  profitSharingReceiverLifecycleTextFilter(req.SyncStatus),
		OffsetCount: pageOffset(req.Page, req.Limit),
		LimitCount:  req.Limit,
	}
	targets, err := server.store.ListProfitSharingReceiverTargets(ctx, params)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	total, err := server.store.CountProfitSharingReceiverTargets(ctx, db.CountProfitSharingReceiverTargetsParams{
		OwnerType:  params.OwnerType,
		OwnerID:    params.OwnerID,
		SyncStatus: params.SyncStatus,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	server.writeProfitSharingReceiverLifecycleReadAudit(ctx, "profit_sharing_receiver_lifecycle_targets_viewed", "profit_sharing_receiver_targets", nil, map[string]any{
		"owner_type":  req.OwnerType,
		"owner_id":    req.OwnerID,
		"sync_status": req.SyncStatus,
		"page":        req.Page,
		"limit":       req.Limit,
	})

	items := make([]profitSharingReceiverLifecycleTargetItem, len(targets))
	for i, target := range targets {
		items[i] = newProfitSharingReceiverLifecycleTargetItem(target)
	}

	ctx.JSON(http.StatusOK, listProfitSharingReceiverLifecycleTargetsResponse{
		Items:      items,
		Total:      total,
		Page:       req.Page,
		Limit:      req.Limit,
		TotalPages: profitSharingReceiverLifecycleTotalPages(total, req.Limit),
		HasMore:    int64(req.Page*req.Limit) < total,
	})
}

// getProfitSharingReceiverLifecycleTarget 获取分账接收方生命周期 target 详情
// @Summary 获取分账接收方生命周期 target 详情
// @Description 平台只读查询单个 receiver lifecycle target；响应不返回 openid、receiver name、account hash、display name hash 或微信原始 payload
// @Tags Platform
// @Accept json
// @Produce json
// @Param id path int true "target ID"
// @Security BearerAuth
// @Success 200 {object} getProfitSharingReceiverLifecycleTargetResponse "target 详情"
// @Failure 400 {object} errorRes "请求参数错误"
// @Failure 401 {object} errorRes "未授权"
// @Failure 403 {object} errorRes "权限不足"
// @Failure 404 {object} errorRes "target不存在"
// @Failure 500 {object} errorRes "服务器内部错误"
// @Router /v1/platform/profit-sharing/receiver-lifecycle/targets/{id} [get]
func (server *Server) getProfitSharingReceiverLifecycleTarget(ctx *gin.Context) {
	var uriReq getProfitSharingReceiverLifecycleTargetURIRequest
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	target, err := server.store.GetProfitSharingReceiverTarget(ctx, uriReq.ID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("profit sharing receiver target not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	server.writeProfitSharingReceiverLifecycleReadAudit(ctx, "profit_sharing_receiver_lifecycle_target_viewed", "profit_sharing_receiver_target", &target.ID, map[string]any{
		"owner_type":    target.OwnerType,
		"owner_id":      target.OwnerID,
		"desired_state": target.DesiredState,
		"sync_status":   target.SyncStatus,
	})

	ctx.JSON(http.StatusOK, getProfitSharingReceiverLifecycleTargetResponse{Target: newProfitSharingReceiverLifecycleTargetItem(target)})
}

// listProfitSharingReceiverLifecycleAttempts 获取分账接收方生命周期 target 执行记录
// @Summary 获取分账接收方生命周期 target 执行记录
// @Description 平台只读查询单个 receiver lifecycle target 的 attempts；错误字段为服务端脱敏摘要，不包含微信原始 payload
// @Tags Platform
// @Accept json
// @Produce json
// @Param id path int true "target ID"
// @Param page query int false "页码" default(1) minimum(1)
// @Param limit query int false "每页数量" default(20) minimum(1) maximum(100)
// @Security BearerAuth
// @Success 200 {object} listProfitSharingReceiverLifecycleAttemptsResponse "attempt 列表"
// @Failure 400 {object} errorRes "请求参数错误"
// @Failure 401 {object} errorRes "未授权"
// @Failure 403 {object} errorRes "权限不足"
// @Failure 404 {object} errorRes "target不存在"
// @Failure 500 {object} errorRes "服务器内部错误"
// @Router /v1/platform/profit-sharing/receiver-lifecycle/targets/{id}/attempts [get]
func (server *Server) listProfitSharingReceiverLifecycleAttempts(ctx *gin.Context) {
	var uriReq getProfitSharingReceiverLifecycleTargetURIRequest
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	var req listProfitSharingReceiverLifecycleAttemptsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	setDefaultProfitSharingReceiverLifecyclePagination(&req.Page, &req.Limit)

	target, err := server.store.GetProfitSharingReceiverTarget(ctx, uriReq.ID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("profit sharing receiver target not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	attempts, err := server.store.ListProfitSharingReceiverAttemptsByTargetPaginated(ctx, db.ListProfitSharingReceiverAttemptsByTargetPaginatedParams{
		TargetID:    target.ID,
		OffsetCount: pageOffset(req.Page, req.Limit),
		LimitCount:  req.Limit,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	total, err := server.store.CountProfitSharingReceiverAttemptsByTarget(ctx, target.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	server.writeProfitSharingReceiverLifecycleReadAudit(ctx, "profit_sharing_receiver_lifecycle_attempts_viewed", "profit_sharing_receiver_target", &target.ID, map[string]any{
		"owner_type":  target.OwnerType,
		"owner_id":    target.OwnerID,
		"sync_status": target.SyncStatus,
		"page":        req.Page,
		"limit":       req.Limit,
	})

	items := make([]profitSharingReceiverLifecycleAttemptItem, len(attempts))
	for i, attempt := range attempts {
		items[i] = newProfitSharingReceiverLifecycleAttemptItem(attempt)
	}

	ctx.JSON(http.StatusOK, listProfitSharingReceiverLifecycleAttemptsResponse{
		Target:     newProfitSharingReceiverLifecycleTargetItem(target),
		Items:      items,
		Total:      total,
		Page:       req.Page,
		Limit:      req.Limit,
		TotalPages: profitSharingReceiverLifecycleTotalPages(total, req.Limit),
		HasMore:    int64(req.Page*req.Limit) < total,
	})
}

func setDefaultProfitSharingReceiverLifecyclePagination(page *int32, limit *int32) {
	if *page == 0 {
		*page = 1
	}
	if *limit == 0 {
		*limit = 20
	}
}

func (server *Server) writeProfitSharingReceiverLifecycleReadAudit(ctx *gin.Context, action string, targetType string, targetID *int64, metadata map[string]any) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	server.writeAuditLog(ctx, AuditLogInput{
		ActorUserID: authPayload.UserID,
		ActorRole:   "platform",
		Action:      action,
		TargetType:  targetType,
		TargetID:    targetID,
		Metadata:    metadata,
	})
}
