package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

const operatorNotificationCategoryDispatchTimeout = "dispatch_timeout"

type listOperatorNotificationsRequest struct {
	IsRead   *bool   `form:"is_read"`
	Category *string `form:"category" binding:"omitempty,oneof=dispatch_timeout system"`
	Limit    int32   `form:"limit,default=20" binding:"min=1,max=100"`
	Offset   int32   `form:"offset,default=0" binding:"min=0"`
}

type operatorNotificationResponse struct {
	ID          int64      `json:"id"`
	Type        string     `json:"type"`
	Category    string     `json:"category"`
	Level       string     `json:"level,omitempty"`
	Title       string     `json:"title"`
	Content     string     `json:"content"`
	Summary     string     `json:"summary,omitempty"`
	RelatedType *string    `json:"related_type,omitempty"`
	RelatedID   *int64     `json:"related_id,omitempty"`
	RegionID    *int64     `json:"region_id,omitempty"`
	RegionName  *string    `json:"region_name,omitempty"`
	WaitMinutes *int32     `json:"wait_minutes,omitempty"`
	IsRead      bool       `json:"is_read"`
	ReadAt      *time.Time `json:"read_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

type listOperatorNotificationsResponse struct {
	Notifications []operatorNotificationResponse `json:"notifications"`
	Total         int64                          `json:"total"`
	PageID        int32                          `json:"page_id"`
	PageSize      int32                          `json:"page_size"`
}

type operatorNotificationSummaryResponse struct {
	UnreadCount        int64                         `json:"unread_count"`
	LatestNotification *operatorNotificationResponse `json:"latest_notification,omitempty"`
}

func decodeNotificationExtraData(raw []byte) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var extra map[string]any
	if err := json.Unmarshal(raw, &extra); err != nil {
		return nil
	}
	return extra
}

func stringExtra(extra map[string]any, key string) string {
	if extra == nil {
		return ""
	}
	value, ok := extra[key]
	if !ok {
		return ""
	}
	parsed, ok := value.(string)
	if !ok {
		return ""
	}
	return parsed
}

func int64Extra(extra map[string]any, key string) *int64 {
	if extra == nil {
		return nil
	}
	value, ok := extra[key]
	if !ok {
		return nil
	}
	switch typed := value.(type) {
	case float64:
		parsed := int64(typed)
		return &parsed
	case int64:
		parsed := typed
		return &parsed
	case int:
		parsed := int64(typed)
		return &parsed
	default:
		return nil
	}
}

func int32Extra(extra map[string]any, key string) *int32 {
	value := int64Extra(extra, key)
	if value == nil {
		return nil
	}
	parsed := int32(*value)
	return &parsed
}

func newOperatorNotificationResponse(n db.Notification) operatorNotificationResponse {
	extra := decodeNotificationExtraData(n.ExtraData)
	resp := operatorNotificationResponse{
		ID:        n.ID,
		Type:      n.Type,
		Category:  stringExtra(extra, "category"),
		Level:     stringExtra(extra, "level"),
		Title:     n.Title,
		Content:   n.Content,
		Summary:   stringExtra(extra, "summary"),
		IsRead:    n.IsRead,
		CreatedAt: n.CreatedAt,
	}

	if resp.Category == "" {
		resp.Category = operatorNotificationCategoryDispatchTimeout
	}
	if n.RelatedType.Valid {
		resp.RelatedType = &n.RelatedType.String
	}
	if n.RelatedID.Valid {
		relatedID := n.RelatedID.Int64
		resp.RelatedID = &relatedID
	}
	if n.ReadAt.Valid {
		resp.ReadAt = &n.ReadAt.Time
	}
	if n.ExpiresAt.Valid {
		resp.ExpiresAt = &n.ExpiresAt.Time
	}
	resp.RegionID = int64Extra(extra, "region_id")
	resp.WaitMinutes = int32Extra(extra, "wait_minutes")
	if regionName := stringExtra(extra, "region_name"); regionName != "" {
		resp.RegionName = &regionName
	}

	return resp
}

func currentOperatorNotificationUserID(ctx *gin.Context) (int64, bool) {
	operator, ok := GetOperatorFromContext(ctx)
	if !ok {
		return 0, false
	}
	return operator.UserID, true
}

// listOperatorNotifications godoc
// @Summary 获取运营商通知列表
// @Description 获取当前运营商的提醒列表，支持按已读状态和分类筛选，支持分页
// @Tags 运营商通知
// @Accept json
// @Produce json
// @Param is_read query bool false "筛选读取状态: true-已读, false-未读"
// @Param category query string false "筛选分类: dispatch_timeout/system"
// @Param limit query int false "每页数量(默认20, 最大100)"
// @Param offset query int false "分页偏移量(默认0)"
// @Success 200 {object} listOperatorNotificationsResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/operators/me/notifications [get]
// @Security BearerAuth
func (server *Server) listOperatorNotifications(ctx *gin.Context) {
	userID, ok := currentOperatorNotificationUserID(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, errorResponse(errors.New("operator context missing")))
		return
	}

	var req listOperatorNotificationsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	listParams := db.ListOperatorNotificationsParams{
		UserID: userID,
		Limit:  req.Limit,
		Offset: req.Offset,
	}
	countParams := db.CountOperatorNotificationsParams{UserID: userID}

	if req.IsRead != nil {
		value := pgtype.Bool{Bool: *req.IsRead, Valid: true}
		listParams.IsRead = value
		countParams.IsRead = value
	}
	if req.Category != nil {
		value := pgtype.Text{String: *req.Category, Valid: true}
		listParams.Category = value
		countParams.Category = value
	}

	notifications, err := server.store.ListOperatorNotifications(ctx, listParams)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	totalCount, err := server.store.CountOperatorNotifications(ctx, countParams)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	responseList := make([]operatorNotificationResponse, len(notifications))
	for i, notification := range notifications {
		responseList[i] = newOperatorNotificationResponse(notification)
	}

	pageID := int32(1)
	if req.Limit > 0 {
		pageID = req.Offset/req.Limit + 1
	}

	ctx.JSON(http.StatusOK, listOperatorNotificationsResponse{
		Notifications: responseList,
		Total:         totalCount,
		PageID:        pageID,
		PageSize:      req.Limit,
	})
}

// getOperatorNotificationSummary godoc
// @Summary 获取运营商通知摘要
// @Description 获取当前运营商未读数和最近一条提醒
// @Tags 运营商通知
// @Accept json
// @Produce json
// @Success 200 {object} operatorNotificationSummaryResponse
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/operators/me/notifications/summary [get]
// @Security BearerAuth
func (server *Server) getOperatorNotificationSummary(ctx *gin.Context) {
	userID, ok := currentOperatorNotificationUserID(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, errorResponse(errors.New("operator context missing")))
		return
	}

	unreadCount, err := server.store.CountOperatorNotifications(ctx, db.CountOperatorNotificationsParams{
		UserID: userID,
		IsRead: pgtype.Bool{Bool: false, Valid: true},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	latestNotifications, err := server.store.ListOperatorNotifications(ctx, db.ListOperatorNotificationsParams{
		UserID: userID,
		Limit:  1,
		Offset: 0,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	response := operatorNotificationSummaryResponse{UnreadCount: unreadCount}
	if len(latestNotifications) > 0 {
		latest := newOperatorNotificationResponse(latestNotifications[0])
		response.LatestNotification = &latest
	}

	ctx.JSON(http.StatusOK, response)
}

// getOperatorNotification godoc
// @Summary 获取运营商通知详情
// @Description 获取当前运营商的一条提醒详情
// @Tags 运营商通知
// @Accept json
// @Produce json
// @Param id path int true "通知ID"
// @Success 200 {object} operatorNotificationResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "通知不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/operators/me/notifications/{id} [get]
// @Security BearerAuth
func (server *Server) getOperatorNotification(ctx *gin.Context) {
	userID, ok := currentOperatorNotificationUserID(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, errorResponse(errors.New("operator context missing")))
		return
	}

	var uri markNotificationAsReadURI
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	notification, err := server.store.GetOperatorNotification(ctx, db.GetOperatorNotificationParams{
		ID:     uri.ID,
		UserID: userID,
	})
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("notification not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newOperatorNotificationResponse(notification))
}

// markOperatorNotificationAsRead godoc
// @Summary 标记运营商通知已读
// @Description 将指定运营商提醒标记为已读，仅能操作自己的提醒
// @Tags 运营商通知
// @Accept json
// @Produce json
// @Param id path int true "通知ID"
// @Success 200 {object} operatorNotificationResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "通知不存在或已读"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/operators/me/notifications/{id}/read [put]
// @Security BearerAuth
func (server *Server) markOperatorNotificationAsRead(ctx *gin.Context) {
	userID, ok := currentOperatorNotificationUserID(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, errorResponse(errors.New("operator context missing")))
		return
	}

	var uri markNotificationAsReadURI
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	notification, err := server.store.MarkOperatorNotificationAsRead(ctx, db.MarkOperatorNotificationAsReadParams{
		ID:     uri.ID,
		UserID: userID,
	})
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("notification not found or already read")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newOperatorNotificationResponse(notification))
}

// markAllOperatorNotificationsAsRead godoc
// @Summary 全部标记运营商通知已读
// @Description 将当前运营商的所有未读提醒标记为已读
// @Tags 运营商通知
// @Accept json
// @Produce json
// @Success 200 {object} markAllAsReadResponse
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/operators/me/notifications/read-all [put]
// @Security BearerAuth
func (server *Server) markAllOperatorNotificationsAsRead(ctx *gin.Context) {
	userID, ok := currentOperatorNotificationUserID(ctx)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, errorResponse(errors.New("operator context missing")))
		return
	}

	err := server.store.MarkAllOperatorNotificationsAsRead(ctx, userID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, markAllAsReadResponse{Success: true})
}
