package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	gorilla_websocket "github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/websocket"
	"github.com/rs/zerolog/log"
)

type notificationResponse struct {
	ID          int64          `json:"id"`
	UserID      int64          `json:"user_id"`
	Type        string         `json:"type"`
	Title       string         `json:"title"`
	Content     string         `json:"content"`
	RelatedType *string        `json:"related_type,omitempty"`
	RelatedID   *int64         `json:"related_id,omitempty"`
	ExtraData   map[string]any `json:"extra_data,omitempty"`
	IsRead      bool           `json:"is_read"`
	ReadAt      *time.Time     `json:"read_at,omitempty"`
	IsPushed    bool           `json:"is_pushed"`
	PushedAt    *time.Time     `json:"pushed_at,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	ExpiresAt   *time.Time     `json:"expires_at,omitempty"`
}

func newNotificationResponse(n db.Notification) notificationResponse {
	resp := notificationResponse{
		ID:        n.ID,
		UserID:    n.UserID,
		Type:      n.Type,
		Title:     n.Title,
		Content:   n.Content,
		IsRead:    n.IsRead,
		IsPushed:  n.IsPushed,
		CreatedAt: n.CreatedAt,
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

	if n.PushedAt.Valid {
		resp.PushedAt = &n.PushedAt.Time
	}

	if n.ExpiresAt.Valid {
		resp.ExpiresAt = &n.ExpiresAt.Time
	}

	if len(n.ExtraData) > 0 {
		var extraData map[string]any
		if err := json.Unmarshal(n.ExtraData, &extraData); err == nil {
			resp.ExtraData = extraData
		}
	}

	return resp
}

type listNotificationsRequest struct {
	IsRead *bool   `form:"is_read"`
	Type   *string `form:"type" binding:"omitempty,oneof=order payment delivery system food_safety"`
	Limit  int32   `form:"limit,default=20" binding:"min=1,max=100"`
	Offset int32   `form:"offset,default=0" binding:"min=0"`
}

type listNotificationsResponse struct {
	Notifications []notificationResponse `json:"notifications"`
	TotalCount    int64                  `json:"total_count"`
}

// listNotifications godoc
// @Summary 获取通知列表
// @Description 获取当前用户的通知列表，支持按读取状态和通知类型筛选，支持分页
// @Tags 通知管理
// @Accept json
// @Produce json
// @Param is_read query bool false "筛选读取状态: true-已读, false-未读"
// @Param type query string false "筛选通知类型: order/payment/delivery/system/food_safety"
// @Param limit query int false "每页数量(默认20, 最大100)"
// @Param offset query int false "分页偏移量(默认0)"
// @Success 200 {object} listNotificationsResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/notifications [get]
// @Security BearerAuth
func (server *Server) listNotifications(ctx *gin.Context) {
	var req listNotificationsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 构建查询参数
	listParams := db.ListUserNotificationsParams{
		UserID: authPayload.UserID,
		Limit:  req.Limit,
		Offset: req.Offset,
	}

	if req.IsRead != nil {
		listParams.IsRead = pgtype.Bool{Bool: *req.IsRead, Valid: true}
	}

	if req.Type != nil {
		listParams.Type = pgtype.Text{String: *req.Type, Valid: true}
	}

	// 查询通知列表
	notifications, err := server.store.ListUserNotifications(ctx, listParams)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 查询总数
	countParams := db.CountUserNotificationsParams{
		UserID: authPayload.UserID,
		IsRead: listParams.IsRead,
		Type:   listParams.Type,
	}

	totalCount, err := server.store.CountUserNotifications(ctx, countParams)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 转换响应
	responseList := make([]notificationResponse, len(notifications))
	for i, n := range notifications {
		responseList[i] = newNotificationResponse(n)
	}

	ctx.JSON(http.StatusOK, listNotificationsResponse{
		Notifications: responseList,
		TotalCount:    totalCount,
	})
}

type unreadCountResponse struct {
	Count int64 `json:"count"`
}

// getUnreadCount godoc
// @Summary 获取未读通知数量
// @Description 获取当前用户的未读通知数量
// @Tags 通知管理
// @Accept json
// @Produce json
// @Success 200 {object} unreadCountResponse
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/notifications/unread/count [get]
// @Security BearerAuth
func (server *Server) getUnreadCount(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	count, err := server.store.CountUnreadNotifications(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, unreadCountResponse{Count: count})
}

type markNotificationAsReadURI struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// markNotificationAsRead godoc
// @Summary 标记通知已读
// @Description 将指定通知标记为已读状态，仅能操作自己的通知
// @Tags 通知管理
// @Accept json
// @Produce json
// @Param id path int true "通知ID"
// @Success 200 {object} notificationResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "通知不存在或已读"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/notifications/{id}/read [put]
// @Security BearerAuth
func (server *Server) markNotificationAsRead(ctx *gin.Context) {
	var uri markNotificationAsReadURI
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 标记已读（Store层会检查用户权限）
	notification, err := server.store.MarkNotificationAsRead(ctx, db.MarkNotificationAsReadParams{
		ID:     uri.ID,
		UserID: authPayload.UserID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("notification not found or already read")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newNotificationResponse(notification))
}

type markAllAsReadResponse struct {
	Success bool `json:"success"`
}

// markAllAsRead godoc
// @Summary 全部标记已读
// @Description 将当前用户的所有未读通知标记为已读
// @Tags 通知管理
// @Accept json
// @Produce json
// @Success 200 {object} markAllAsReadResponse
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/notifications/read-all [put]
// @Security BearerAuth
func (server *Server) markAllAsRead(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	err := server.store.MarkAllNotificationsAsRead(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, markAllAsReadResponse{Success: true})
}

type deleteNotificationURI struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// deleteNotification godoc
// @Summary 删除通知
// @Description 删除指定通知，仅能删除自己的通知
// @Tags 通知管理
// @Accept json
// @Produce json
// @Param id path int true "通知ID"
// @Success 204 "无返回内容"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "通知不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/notifications/{id} [delete]
// @Security BearerAuth
func (server *Server) deleteNotification(ctx *gin.Context) {
	var uri deleteNotificationURI
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 删除通知（Store层会检查用户权限）
	err := server.store.DeleteNotification(ctx, db.DeleteNotificationParams{
		ID:     uri.ID,
		UserID: authPayload.UserID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("notification not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.Status(http.StatusNoContent)
}

type notificationPreferencesResponse struct {
	UserID                      int64     `json:"user_id"`
	EnableOrderNotifications    bool      `json:"enable_order_notifications"`
	EnablePaymentNotifications  bool      `json:"enable_payment_notifications"`
	EnableDeliveryNotifications bool      `json:"enable_delivery_notifications"`
	EnableSystemNotifications   bool      `json:"enable_system_notifications"`
	DoNotDisturbStart           *string   `json:"do_not_disturb_start,omitempty"`
	DoNotDisturbEnd             *string   `json:"do_not_disturb_end,omitempty"`
	CreatedAt                   time.Time `json:"created_at"`
	UpdatedAt                   time.Time `json:"updated_at"`
}

func newNotificationPreferencesResponse(p db.UserNotificationPreference) notificationPreferencesResponse {
	resp := notificationPreferencesResponse{
		UserID:                      p.UserID,
		EnableOrderNotifications:    p.EnableOrderNotifications,
		EnablePaymentNotifications:  p.EnablePaymentNotifications,
		EnableDeliveryNotifications: p.EnableDeliveryNotifications,
		EnableSystemNotifications:   p.EnableSystemNotifications,
		CreatedAt:                   p.CreatedAt,
	}

	// 处理UpdatedAt
	if p.UpdatedAt.Valid {
		resp.UpdatedAt = p.UpdatedAt.Time
	} else {
		resp.UpdatedAt = p.CreatedAt
	}

	// 处理DoNotDisturbStart (pgtype.Time以微秒存储)
	if p.DoNotDisturbStart.Valid {
		seconds := p.DoNotDisturbStart.Microseconds / 1000000
		hours := seconds / 3600
		minutes := (seconds % 3600) / 60
		secs := seconds % 60
		timeStr := time.Date(0, 1, 1, int(hours), int(minutes), int(secs), 0, time.UTC).Format("15:04:05")
		resp.DoNotDisturbStart = &timeStr
	}

	// 处理DoNotDisturbEnd (pgtype.Time以微秒存储)
	if p.DoNotDisturbEnd.Valid {
		seconds := p.DoNotDisturbEnd.Microseconds / 1000000
		hours := seconds / 3600
		minutes := (seconds % 3600) / 60
		secs := seconds % 60
		timeStr := time.Date(0, 1, 1, int(hours), int(minutes), int(secs), 0, time.UTC).Format("15:04:05")
		resp.DoNotDisturbEnd = &timeStr
	}

	return resp
}

// getNotificationPreferences godoc
// @Summary 获取通知偏好设置
// @Description 获取用户的通知偏好设置，如不存在则自动创建默认设置
// @Tags 通知管理
// @Accept json
// @Produce json
// @Success 200 {object} notificationPreferencesResponse
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/notifications/preferences [get]
// @Security BearerAuth
func (server *Server) getNotificationPreferences(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	prefs, err := server.store.GetOrCreateUserNotificationPreferences(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newNotificationPreferencesResponse(prefs))
}

type updateNotificationPreferencesRequest struct {
	EnableOrderNotifications    *bool   `json:"enable_order_notifications"`
	EnablePaymentNotifications  *bool   `json:"enable_payment_notifications"`
	EnableDeliveryNotifications *bool   `json:"enable_delivery_notifications"`
	EnableSystemNotifications   *bool   `json:"enable_system_notifications"`
	DoNotDisturbStart           *string `json:"do_not_disturb_start" binding:"omitempty,len=8"` // HH:MM:SS
	DoNotDisturbEnd             *string `json:"do_not_disturb_end" binding:"omitempty,len=8"`   // HH:MM:SS
}

// updateNotificationPreferences godoc
// @Summary 更新通知偏好设置
// @Description 更新用户的通知偏好设置，包括各类通知开关和免打扰时段
// @Tags 通知管理
// @Accept json
// @Produce json
// @Param request body updateNotificationPreferencesRequest true "偏好设置更新内容"
// @Success 200 {object} notificationPreferencesResponse
// @Failure 400 {object} ErrorResponse "参数错误或时间格式错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/notifications/preferences [put]
// @Security BearerAuth
func (server *Server) updateNotificationPreferences(ctx *gin.Context) {
	var req updateNotificationPreferencesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 构建更新参数
	params := db.UpdateUserNotificationPreferencesParams{
		UserID: authPayload.UserID,
	}

	if req.EnableOrderNotifications != nil {
		params.EnableOrderNotifications = pgtype.Bool{Bool: *req.EnableOrderNotifications, Valid: true}
	}

	if req.EnablePaymentNotifications != nil {
		params.EnablePaymentNotifications = pgtype.Bool{Bool: *req.EnablePaymentNotifications, Valid: true}
	}

	if req.EnableDeliveryNotifications != nil {
		params.EnableDeliveryNotifications = pgtype.Bool{Bool: *req.EnableDeliveryNotifications, Valid: true}
	}

	if req.EnableSystemNotifications != nil {
		params.EnableSystemNotifications = pgtype.Bool{Bool: *req.EnableSystemNotifications, Valid: true}
	}

	if req.DoNotDisturbStart != nil {
		t, err := time.Parse("15:04:05", *req.DoNotDisturbStart)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid do_not_disturb_start format, expected HH:MM:SS")))
			return
		}
		params.DoNotDisturbStart = pgtype.Time{Microseconds: int64(t.Hour()*3600+t.Minute()*60+t.Second()) * 1000000, Valid: true}
	}

	if req.DoNotDisturbEnd != nil {
		t, err := time.Parse("15:04:05", *req.DoNotDisturbEnd)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid do_not_disturb_end format, expected HH:MM:SS")))
			return
		}
		params.DoNotDisturbEnd = pgtype.Time{Microseconds: int64(t.Hour()*3600+t.Minute()*60+t.Second()) * 1000000, Valid: true}
	}

	// 更新偏好设置
	prefs, err := server.store.UpdateUserNotificationPreferences(ctx, params)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newNotificationPreferencesResponse(prefs))
}

var upgrader = gorilla_websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 生产环境需要验证Origin
	},
}

// handleWebSocket godoc
// @Summary WebSocket连接端点
// @Description 将HTTP连接升级为WebSocket，用于实时通知推送，仅骑手和商户可用
// @Tags 通知管理
// @Accept json
// @Produce json
// @Param token query string false "Authentication token (required if Authorization header is missing)"
// @Success 101 "协议升级成功"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "仅骑手和商户可连接"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/ws [get]
// @Security BearerAuth
func (server *Server) handleWebSocket(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 查询用户角色
	roles, err := server.store.ListUserRoles(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查是否为骑手或商户
	var clientType websocket.ClientType
	var entityID int64

	for _, role := range roles {
		if role.Role == "rider" && role.RelatedEntityID.Valid {
			clientType = websocket.ClientTypeRider
			entityID = role.RelatedEntityID.Int64
			break
		} else if role.Role == "merchant" && role.RelatedEntityID.Valid {
			clientType = websocket.ClientTypeMerchant
			entityID = role.RelatedEntityID.Int64
			break
		}
	}

	if entityID == 0 {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("only riders and merchants can establish WebSocket connection")))
		return
	}

	// 骑手必须上线才能建立WebSocket连接
	if clientType == websocket.ClientTypeRider {
		rider, err := server.store.GetRider(ctx, entityID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		if !rider.IsOnline {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("请先上线才能接收实时订单推送")))
			return
		}
	}

	// 升级到WebSocket
	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		log.Error().Err(err).Msg("WebSocket upgrade failed")
		return
	}

	// 创建客户端并注册
	client := websocket.NewClient(server.wsHub, conn, websocket.ClientInfo{
		UserID:     authPayload.UserID,
		ClientType: clientType,
		EntityID:   entityID,
	})

	server.wsHub.Register(client)

	// 启动读写协程
	go client.WritePump()
	go client.ReadPump()
}

// handlePlatformWebSocket godoc
// @Summary 平台运营WebSocket连接端点
// @Description 将HTTP连接升级为WebSocket，用于平台运营人员接收实时告警推送
// @Tags 通知管理
// @Accept json
// @Produce json
// @Param token query string false "Authentication token (required if Authorization header is missing)"
// @Success 101 "协议升级成功"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "仅平台运营人员可连接"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/platform/ws [get]
// @Security BearerAuth
func (server *Server) handlePlatformWebSocket(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 查询用户角色，检查是否为平台运营人员
	roles, err := server.store.ListUserRoles(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查是否有平台角色（admin、operator、finance等）
	isPlatformUser := false
	for _, role := range roles {
		switch role.Role {
		case "admin", "operator", "platform_admin", "platform_operator", "finance":
			isPlatformUser = true
		}
		if isPlatformUser {
			break
		}
	}

	if !isPlatformUser {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("only platform operators can establish this WebSocket connection")))
		return
	}

	// 升级到WebSocket
	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		log.Error().Err(err).Msg("Platform WebSocket upgrade failed")
		return
	}

	// 创建客户端并注册（使用用户ID作为实体ID）
	client := websocket.NewClient(server.wsHub, conn, websocket.ClientInfo{
		UserID:     authPayload.UserID,
		ClientType: websocket.ClientTypePlatform,
		EntityID:   authPayload.UserID, // 平台用户使用 user_id 作为实体ID
	})

	server.wsHub.Register(client)

	// 启动读写协程
	go client.WritePump()
	go client.ReadPump()
}
