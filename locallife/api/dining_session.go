package api

import (
	"encoding/json"
	"net/http"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/websocket"

	"github.com/gin-gonic/gin"
)

const timeLayout = "2006-01-02 15:04:05"

type diningSessionResponse struct {
	ID            int64   `json:"id"`
	MerchantID    int64   `json:"merchant_id"`
	TableID       int64   `json:"table_id"`
	ReservationID *int64  `json:"reservation_id,omitempty"`
	UserID        int64   `json:"user_id"`
	ActiveOrderID *int64  `json:"active_order_id,omitempty"`
	Status        string  `json:"status"`
	OpenedAt      string  `json:"opened_at"`
	ClosedAt      *string `json:"closed_at,omitempty"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     *string `json:"updated_at,omitempty"`
}

type openDiningSessionRequest struct {
	TableID       int64   `json:"table_id" binding:"required,min=1"`
	ReservationID *int64  `json:"reservation_id,omitempty" binding:"omitempty,min=1"`
	TableCode     *string `json:"table_code,omitempty" binding:"omitempty,min=4,max=32"`
}

type openDiningSessionResponse struct {
	Session       diningSessionResponse `json:"session"`
	BillingGroup  billingGroupResponse  `json:"billing_group"`
	CartID        *int64                `json:"cart_id,omitempty"`
	ImportedItems int                   `json:"imported_items"`
}

type transferDiningSessionRequest struct {
	ToTableID int64   `json:"to_table_id" binding:"required,min=1"`
	TableCode *string `json:"table_code,omitempty" binding:"omitempty,min=4,max=32"`
	Reason    *string `json:"reason,omitempty" binding:"omitempty,max=200"`
}

type transferDiningSessionResponse struct {
	Session   diningSessionResponse `json:"session"`
	FromTable tableResponse         `json:"from_table"`
	ToTable   tableResponse         `json:"to_table"`
}

type billingGroupResponse struct {
	ID              int64   `json:"id"`
	DiningSessionID int64   `json:"dining_session_id"`
	Status          string  `json:"status"`
	IsDefault       bool    `json:"is_default"`
	TotalAmount     int64   `json:"total_amount"`
	PaidAmount      int64   `json:"paid_amount"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       *string `json:"updated_at,omitempty"`
	ClosedAt        *string `json:"closed_at,omitempty"`
}

func newBillingGroupResponse(bg db.BillingGroup) billingGroupResponse {
	resp := billingGroupResponse{
		ID:              bg.ID,
		DiningSessionID: bg.DiningSessionID,
		Status:          bg.Status,
		IsDefault:       bg.IsDefault,
		TotalAmount:     bg.TotalAmount,
		PaidAmount:      bg.PaidAmount,
		CreatedAt:       bg.CreatedAt.Format(timeLayout),
	}
	if bg.UpdatedAt.Valid {
		t := bg.UpdatedAt.Time.Format(timeLayout)
		resp.UpdatedAt = &t
	}
	if bg.ClosedAt.Valid {
		t := bg.ClosedAt.Time.Format(timeLayout)
		resp.ClosedAt = &t
	}
	return resp
}

type precheckDiningSessionRequest struct {
	TableID int64 `form:"table_id" binding:"required,min=1"`
}

type diningSessionPrecheckResponse struct {
	TableID                int64   `json:"table_id"`
	Reserved               bool    `json:"reserved"`
	ReservationID          *int64  `json:"reservation_id,omitempty"`
	IsReservationOwner     bool    `json:"is_reservation_owner"`
	PaymentMode            *string `json:"payment_mode,omitempty"`
	PaidAmount             *int64  `json:"paid_amount,omitempty"`
	OrderID                *int64  `json:"order_id,omitempty"`
	OrderStatus            *string `json:"order_status,omitempty"`
	OrderFulfillmentStatus *string `json:"order_fulfillment_status,omitempty"`
}

func newDiningSessionResponse(s db.DiningSession) diningSessionResponse {
	resp := diningSessionResponse{
		ID:         s.ID,
		MerchantID: s.MerchantID,
		TableID:    s.TableID,
		UserID:     s.UserID,
		Status:     s.Status,
		OpenedAt:   s.OpenedAt.Format(timeLayout),
		CreatedAt:  s.CreatedAt.Format(timeLayout),
	}
	if s.ReservationID.Valid {
		resp.ReservationID = &s.ReservationID.Int64
	}
	if s.ActiveOrderID.Valid {
		resp.ActiveOrderID = &s.ActiveOrderID.Int64
	}
	if s.ClosedAt.Valid {
		t := s.ClosedAt.Time.Format(timeLayout)
		resp.ClosedAt = &t
	}
	if s.UpdatedAt.Valid {
		t := s.UpdatedAt.Time.Format(timeLayout)
		resp.UpdatedAt = &t
	}
	return resp
}

// precheckDiningSession 预检查桌台时段预订占用
// @Summary 用餐会话预检
// @Description 扫码后检查桌台当前时段是否被预订，返回预订及订单信息
// @Tags 用餐会话
// @Produce json
// @Param table_id query int true "桌台ID"
// @Success 200 {object} diningSessionPrecheckResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/dining-sessions/precheck [get]
func (server *Server) precheckDiningSession(ctx *gin.Context) {
	var req precheckDiningSessionRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	result, err := logic.PrecheckDiningSession(ctx, server.store, logic.DiningSessionPrecheckInput{
		UserID:  authPayload.UserID,
		TableID: req.TableID,
		Now:     time.Now(),
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if !result.Reserved || result.Reservation == nil {
		ctx.JSON(http.StatusOK, diningSessionPrecheckResponse{TableID: result.Table.ID, Reserved: false})
		return
	}

	resp := diningSessionPrecheckResponse{
		TableID:            result.Table.ID,
		Reserved:           true,
		ReservationID:      &result.Reservation.ID,
		IsReservationOwner: result.IsReservationOwner,
		PaymentMode:        result.PaymentMode,
		PaidAmount:         result.PaidAmount,
	}
	if result.Order != nil {
		resp.OrderID = &result.Order.ID
		resp.OrderStatus = &result.Order.Status
		if result.Order.FulfillmentStatus != "" {
			fs := result.Order.FulfillmentStatus
			resp.OrderFulfillmentStatus = &fs
		}
	}

	ctx.JSON(http.StatusOK, resp)
}

// openDiningSession 创建或返回当前桌台/预订的开放用餐会话
// @Summary 开启用餐会话（堂食/预订到店）
// @Description 扫码后为桌台/预订创建开放会话；若已存在开放会话则直接返回
// @Tags 用餐会话
// @Accept json
// @Produce json
// @Param request body openDiningSessionRequest true "开台请求"
// @Success 200 {object} openDiningSessionResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/dining-sessions/open [post]
func (server *Server) openDiningSession(ctx *gin.Context) {
	var req openDiningSessionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	now := time.Now()
	result, err := logic.OpenDiningSession(ctx, server.store, logic.OpenDiningSessionInput{
		UserID:              authPayload.UserID,
		TableID:             req.TableID,
		ReservationID:       req.ReservationID,
		TableCode:           req.TableCode,
		Now:                 now,
		CheckInEarlyMinutes: ReservationCheckInEarlyMinutes,
		CheckInLateMinutes:  ReservationCheckInLateMinutes,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	session := result.Session
	response := openDiningSessionResponse{
		Session:       newDiningSessionResponse(session),
		BillingGroup:  newBillingGroupResponse(result.BillingGroup),
		CartID:        result.CartID,
		ImportedItems: result.ImportedItems,
	}

	if result.Existing {
		ctx.JSON(http.StatusOK, response)
		return
	}

	// 到店扫码开台提醒：若用户在外卖拒绝服务名单中，通知商户后台
	if server.wsHub != nil {
		block, err := server.store.GetActiveBehaviorBlocklist(ctx, db.GetActiveBehaviorBlocklistParams{
			EntityType: "user",
			EntityID:   authPayload.UserID,
		})
		if err == nil {
			alertPayload, _ := json.Marshal(map[string]any{
				"user_id":    authPayload.UserID,
				"session_id": session.ID,
				"reservation_id": func() *int64 {
					if session.ReservationID.Valid {
						id := session.ReservationID.Int64
						return &id
					}
					return nil
				}(),
				"table_id":    session.TableID,
				"merchant_id": session.MerchantID,
				"reason_code": block.ReasonCode,
				"message":     "该顾客有多次恶意索赔记录，谨慎服务",
			})
			server.wsHub.SendToMerchant(session.MerchantID, websocket.Message{
				Type:      EventMerchantUserRiskAlert,
				Data:      alertPayload,
				Timestamp: time.Now(),
			})
		}

		// 推送桌台状态变更
		tableData, _ := json.Marshal(map[string]any{
			"id":     session.TableID,
			"status": TableStatusOccupied,
		})
		server.wsHub.SendToMerchant(session.MerchantID, websocket.Message{
			Type:      EventTableStatusChange,
			Data:      tableData,
			Timestamp: time.Now(),
		})

		// 如果关联了预订，推送预订状态变更 (checked_in)
		if session.ReservationID.Valid {
			if updatedRes, err := server.store.GetTableReservationWithTable(ctx, session.ReservationID.Int64); err == nil {
				// 获取关联的订单项 (为了完整性，虽然列表可能只需要基本信息)
				// 简单起见，这里复用 newReservationWithTableResponse
				resResp := newReservationWithTableResponse(updatedRes)
				// 尝试获取 Items (可选，为了列表视图完整性)
				if items, err := server.store.ListReservationItems(ctx, updatedRes.ID); err == nil {
					resResp.Items = make([]reservationItemResponse, len(items))
					for i, item := range items {
						resResp.Items[i] = reservationItemResponse{
							ID:         item.ID,
							Quantity:   item.Quantity,
							UnitPrice:  item.UnitPrice,
							TotalPrice: int64(item.Quantity) * item.UnitPrice,
							Type:       "dish", // Default
							Name:       item.DishName.String,
						}
						// 简单的映射，不像 listMerchantReservations 那么细致，主要为了状态更新
						if item.DishImageUrl.Valid {
							resResp.Items[i].ImageURL = normalizeUploadURLForClient(item.DishImageUrl.String)
						}
					}
				}

				resPayload, _ := json.Marshal(map[string]any{
					"reservation": resResp,
				})
				server.wsHub.SendToMerchant(session.MerchantID, websocket.Message{
					Type:      EventReservationUpdate,
					Data:      resPayload,
					Timestamp: time.Now(),
				})
			}
		}
	}

	ctx.JSON(http.StatusOK, response)
}

// transferDiningSessionTable 转台/换桌
// @Summary 转台（换桌）
// @Description 将开放用餐会话从一个桌台转移到另一个桌台，支持商户与C端扫码
// @Tags 用餐会话
// @Accept json
// @Produce json
// @Param id path int64 true "用餐会话ID"
// @Param request body transferDiningSessionRequest true "转台请求"
// @Success 200 {object} transferDiningSessionResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/dining-sessions/{id}/transfer-table [post]
func (server *Server) transferDiningSessionTable(ctx *gin.Context) {
	var uriReq struct {
		ID int64 `uri:"id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req transferDiningSessionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	result, err := logic.TransferDiningSessionTable(ctx, server.store, logic.TransferDiningSessionTableInput{
		SessionID: uriReq.ID,
		ToTableID: req.ToTableID,
		UserID:    authPayload.UserID,
		TableCode: req.TableCode,
		Reason:    req.Reason,
		Now:       time.Now(),
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if result.SameTable {
		ctx.JSON(http.StatusOK, transferDiningSessionResponse{
			Session:   newDiningSessionResponse(result.Session),
			FromTable: newTableResponse(result.FromTable),
			ToTable:   newTableResponse(result.ToTable),
		})
		return
	}

	if server.wsHub != nil {
		fromPayload, _ := json.Marshal(map[string]any{
			"id":       result.FromTable.ID,
			"table_no": result.FromTable.TableNo,
			"status":   result.FromTable.Status,
		})
		server.wsHub.SendToMerchant(result.Session.MerchantID, websocket.Message{
			Type:      EventTableStatusChange,
			Data:      fromPayload,
			Timestamp: time.Now(),
		})

		toPayload, _ := json.Marshal(map[string]any{
			"id":       result.ToTable.ID,
			"table_no": result.ToTable.TableNo,
			"status":   result.ToTable.Status,
		})
		server.wsHub.SendToMerchant(result.Session.MerchantID, websocket.Message{
			Type:      EventTableStatusChange,
			Data:      toPayload,
			Timestamp: time.Now(),
		})

		transferPayload, _ := json.Marshal(map[string]any{
			"session_id":    result.Session.ID,
			"from_table_id": result.FromTable.ID,
			"to_table_id":   result.ToTable.ID,
			"operator_id":   authPayload.UserID,
		})
		server.wsHub.SendToMerchant(result.Session.MerchantID, websocket.Message{
			Type:      EventTableTransfer,
			Data:      transferPayload,
			Timestamp: time.Now(),
		})
	}

	ctx.JSON(http.StatusOK, transferDiningSessionResponse{
		Session:   newDiningSessionResponse(result.Session),
		FromTable: newTableResponse(result.FromTable),
		ToTable:   newTableResponse(result.ToTable),
	})
}

type checkoutDiningSessionRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// checkoutDiningSession godoc
// @Summary 结束就餐会话（结账离店）
// @Description 商家手动结束就餐会话，关闭账单组并释放桌位。
// @Tags 就餐会话
// @Accept json
// @Produce json
// @Param id path int true "会话ID"
// @Success 200 {object} diningSessionResponse "会话已关闭"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "权限不足"
// @Failure 404 {object} ErrorResponse "找不到就餐会话"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/dining-sessions/{id}/checkout [post]
// @Security BearerAuth
func (server *Server) checkoutDiningSession(ctx *gin.Context) {
	var req checkoutDiningSessionRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	result, err := logic.CheckoutDiningSession(ctx, server.store, logic.CheckoutDiningSessionInput{
		SessionID: req.ID,
		UserID:    authPayload.UserID,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 推送 WebSocket 消息通知相关方桌位已释放
	if server.wsHub != nil {
		tableData, _ := json.Marshal(map[string]any{
			"id":     result.Session.TableID,
			"status": TableStatusAvailable,
			"event":  EventSessionClosed,
		})
		server.wsHub.SendToMerchant(result.Merchant.ID, websocket.Message{
			Type:      EventTableStatusChange,
			Data:      tableData,
			Timestamp: time.Now(),
		})

		// 如果关联了预订，推送预订状态变更 (completed)
		if result.Session.ReservationID.Valid {
			if updatedRes, err := server.store.GetTableReservationWithTable(ctx, result.Session.ReservationID.Int64); err == nil {
				resResp := newReservationWithTableResponse(updatedRes)
				resPayload, _ := json.Marshal(map[string]any{
					"reservation": resResp,
				})
				server.wsHub.SendToMerchant(result.Merchant.ID, websocket.Message{
					Type:      EventReservationUpdate,
					Data:      resPayload,
					Timestamp: time.Now(),
				})
			}
		}
	}

	ctx.JSON(http.StatusOK, newDiningSessionResponse(result.Session))
}
