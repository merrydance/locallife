package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/websocket"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
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

	table, err := server.store.GetTable(ctx, req.TableID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("table not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	activeReservation, err := server.findActiveReservationForTable(ctx, table.ID, time.Now())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if activeReservation == nil {
		ctx.JSON(http.StatusOK, diningSessionPrecheckResponse{TableID: table.ID, Reserved: false})
		return
	}

	if activeReservation.UserID != authPayload.UserID {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("table is reserved and unavailable")))
		return
	}

	resp := diningSessionPrecheckResponse{
		TableID:            table.ID,
		Reserved:           true,
		ReservationID:      &activeReservation.ID,
		IsReservationOwner: true,
	}
	pm := activeReservation.PaymentMode
	resp.PaymentMode = &pm
	paidAmount := activeReservation.PrepaidAmount
	if pm == PaymentModeDeposit {
		paidAmount = activeReservation.DepositAmount
	}
	resp.PaidAmount = &paidAmount

	if order, err := server.store.GetLatestOrderByReservation(ctx, pgtype.Int8{Int64: activeReservation.ID, Valid: true}); err == nil {
		resp.OrderID = &order.ID
		resp.OrderStatus = &order.Status
		if order.FulfillmentStatus != "" {
			fs := order.FulfillmentStatus
			resp.OrderFulfillmentStatus = &fs
		}
	} else if !isNotFoundError(err) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
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

	// 基础校验：桌台存在
	table, err := server.store.GetTable(ctx, req.TableID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("table not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	activeReservation, err := server.findActiveReservationForTable(ctx, table.ID, now)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 如果带预订，验证归属与状态
	var reservation *db.TableReservation
	if req.ReservationID != nil {
		res, err := server.store.GetTableReservation(ctx, *req.ReservationID)
		if err != nil {
			if isNotFoundError(err) {
				ctx.JSON(http.StatusNotFound, errorResponse(errors.New("reservation not found")))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		reservation = &res
	} else if activeReservation != nil && activeReservation.UserID == authPayload.UserID {
		reservation = activeReservation
	}

	if activeReservation != nil && (reservation == nil || activeReservation.ID != reservation.ID) {
		if activeReservation.UserID != authPayload.UserID {
			ctx.JSON(http.StatusConflict, errorResponse(errors.New("table is reserved and unavailable")))
			return
		}
	}

	if reservation != nil {
		if reservation.UserID != authPayload.UserID {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("reservation does not belong to you")))
			return
		}
		if reservation.TableID != req.TableID {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("table does not match reservation")))
			return
		}
		if reservation.Status != ReservationStatusPaid && reservation.Status != ReservationStatusConfirmed && reservation.Status != ReservationStatusCheckedIn {
			ctx.JSON(http.StatusConflict, errorResponse(errors.New("reservation is not ready for dining")))
			return
		}

		// 预订签到时间窗口：预订时间前后各30分钟
		scheduledAt := combineDateAndTime(reservation.ReservationDate.Time, reservation.ReservationTime)
		if now.Before(scheduledAt.Add(-30 * time.Minute)) {
			ctx.JSON(http.StatusConflict, errorResponse(errors.New("too early to check in for reservation")))
			return
		}
		if now.After(scheduledAt.Add(30 * time.Minute)) {
			ctx.JSON(http.StatusConflict, errorResponse(errors.New("reservation check-in window has passed")))
			return
		}
	}

	if reservation == nil {
		if !table.AccessCodeHash.Valid || strings.TrimSpace(table.AccessCodeHash.String) == "" {
			ctx.JSON(http.StatusConflict, errorResponse(errors.New("table access code is not configured")))
			return
		}
		if req.TableCode == nil || strings.TrimSpace(*req.TableCode) == "" {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("table access code is required")))
			return
		}
		if err := util.CheckPassword(*req.TableCode, table.AccessCodeHash.String); err != nil {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("invalid table access code")))
			return
		}
	}

	// 已有开放会话直接返回（按预订优先，其次按桌台）
	if reservation != nil {
		if existing, err := server.store.GetActiveDiningSessionByReservation(ctx, pgtype.Int8{Int64: reservation.ID, Valid: true}); err == nil {
			billingGroup, err := server.getOrCreateDefaultBillingGroup(ctx, existing, authPayload.UserID)
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
				return
			}
			ctx.JSON(http.StatusOK, openDiningSessionResponse{
				Session:      newDiningSessionResponse(existing),
				BillingGroup: newBillingGroupResponse(billingGroup),
			})
			return
		} else if !isNotFoundError(err) {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}
	if existing, err := server.store.GetActiveDiningSessionByTable(ctx, req.TableID); err == nil {
		// 若桌上已有开放会话但绑定的预订与请求不一致，视为冲突
		if reservation != nil {
			if !existing.ReservationID.Valid || existing.ReservationID.Int64 != reservation.ID {
				ctx.JSON(http.StatusConflict, errorResponse(errors.New("table already has an active session")))
				return
			}
		} else if existing.ReservationID.Valid {
			ctx.JSON(http.StatusConflict, errorResponse(errors.New("table already has an active session")))
			return
		}
		billingGroup, err := server.getOrCreateDefaultBillingGroup(ctx, existing, authPayload.UserID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		ctx.JSON(http.StatusOK, openDiningSessionResponse{
			Session:      newDiningSessionResponse(existing),
			BillingGroup: newBillingGroupResponse(billingGroup),
		})
		return
	} else if !isNotFoundError(err) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resID := pgtype.Int8{Valid: false}
	if reservation != nil {
		resID = pgtype.Int8{Int64: reservation.ID, Valid: true}
	}

	var activateOrder *db.ActivateOrderInput
	if reservation != nil {
		order, err := server.store.GetLatestOrderByReservation(ctx, resID)
		if err != nil && !isNotFoundError(err) {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		if err == nil {
			if order.Status != OrderStatusPaid {
				ctx.JSON(http.StatusConflict, errorResponse(errors.New("reservation order is not paid")))
				return
			}

			newFulfillment := order.FulfillmentStatus
			if order.FulfillmentStatus == FulfillmentStatusScheduled {
				newFulfillment = FulfillmentStatusPendingKitchen
			}

			activateOrder = &db.ActivateOrderInput{
				OrderID:              order.ID,
				Status:               order.Status,
				NewFulfillmentStatus: pgtype.Text{String: newFulfillment, Valid: true},
			}
		}
	}

	txResult, err := server.store.OpenDiningSessionTx(ctx, db.OpenDiningSessionTxParams{
		TableID:                table.ID,
		MerchantID:             table.MerchantID,
		UserID:                 authPayload.UserID,
		ReservationID:          resID,
		ImportReservationItems: reservation != nil,
		ActivateOrder:          activateOrder,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	session := txResult.Session
	if txResult.ActivatedOrder != nil {
		session.ActiveOrderID = pgtype.Int8{Int64: txResult.ActivatedOrder.ID, Valid: true}
	}

	// 到店扫码开台提醒：若用户在外卖拒绝服务名单中，通知商户后台
	if server.wsHub != nil {
		block, err := server.store.GetActiveBehaviorBlocklist(ctx, db.GetActiveBehaviorBlocklistParams{
			EntityType: "user",
			EntityID:   authPayload.UserID,
		})
		if err == nil {
			alertPayload, _ := json.Marshal(map[string]any{
				"user_id":      authPayload.UserID,
				"session_id":   session.ID,
				"reservation_id": func() *int64 {
					if session.ReservationID.Valid {
						id := session.ReservationID.Int64
						return &id
					}
					return nil
				}(),
				"table_id":     session.TableID,
				"merchant_id":  session.MerchantID,
				"reason_code":  block.ReasonCode,
				"message":      "该顾客有多次恶意索赔记录，谨慎服务",
			})
			server.wsHub.SendToMerchant(session.MerchantID, websocket.Message{
				Type:      "merchant_user_risk_alert",
				Data:      alertPayload,
				Timestamp: time.Now(),
			})
		}
	}

	ctx.JSON(http.StatusOK, openDiningSessionResponse{
		Session:       newDiningSessionResponse(session),
		BillingGroup:  newBillingGroupResponse(txResult.BillingGroup),
		CartID:        txResult.CartID,
		ImportedItems: txResult.ImportedItems,
	})
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

	session, err := server.store.GetDiningSession(ctx, uriReq.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("dining session not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if session.Status != "open" {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("dining session is not open")))
		return
	}

	if req.ToTableID == session.TableID {
		table, err := server.store.GetTable(ctx, session.TableID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		ctx.JSON(http.StatusOK, transferDiningSessionResponse{
			Session:   newDiningSessionResponse(session),
			FromTable: newTableResponse(table),
			ToTable:   newTableResponse(table),
		})
		return
	}

	isOwner := session.UserID == authPayload.UserID
	isMerchant := false
	if merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID); err == nil && merchant.ID == session.MerchantID {
		isMerchant = true
	} else if err != nil && !isNotFoundError(err) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if !isMerchant {
		if staff, err := server.store.GetMerchantStaff(ctx, db.GetMerchantStaffParams{
			MerchantID: session.MerchantID,
			UserID:     authPayload.UserID,
		}); err == nil {
			if staff.Status == "active" {
				isMerchant = true
			}
		} else if !isNotFoundError(err) {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}
	if !isOwner && !isMerchant {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not authorized to transfer dining session")))
		return
	}

	toTable, err := server.store.GetTable(ctx, req.ToTableID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("table not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if toTable.MerchantID != session.MerchantID {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("table does not belong to session merchant")))
		return
	}
	if toTable.Status == "disabled" {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("target table is disabled")))
		return
	}

	if session.ReservationID.Valid && !isMerchant {
		res, err := server.store.GetTableReservation(ctx, session.ReservationID.Int64)
		if err != nil {
			if isNotFoundError(err) {
				ctx.JSON(http.StatusNotFound, errorResponse(errors.New("reservation not found")))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		if res.UserID != authPayload.UserID {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("reservation does not belong to you")))
			return
		}
	}

	activeReservation, err := server.findActiveReservationForTable(ctx, toTable.ID, time.Now())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if activeReservation != nil {
		if !session.ReservationID.Valid || activeReservation.ID != session.ReservationID.Int64 {
			ctx.JSON(http.StatusConflict, errorResponse(errors.New("target table is reserved and unavailable")))
			return
		}
	}

	if !isMerchant && !session.ReservationID.Valid {
		if req.TableCode == nil || strings.TrimSpace(*req.TableCode) == "" {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("table access code is required")))
			return
		}
		if !toTable.AccessCodeHash.Valid || strings.TrimSpace(toTable.AccessCodeHash.String) == "" {
			ctx.JSON(http.StatusConflict, errorResponse(errors.New("table access code is not configured")))
			return
		}
		if err := util.CheckPassword(*req.TableCode, toTable.AccessCodeHash.String); err != nil {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("invalid table access code")))
			return
		}
	}

	var reason pgtype.Text
	if req.Reason != nil && strings.TrimSpace(*req.Reason) != "" {
		reason = pgtype.Text{String: strings.TrimSpace(*req.Reason), Valid: true}
	}

	result, err := server.store.TransferDiningSessionTableTx(ctx, db.TransferDiningSessionTableTxParams{
		SessionID:      session.ID,
		ToTableID:      req.ToTableID,
		OperatorUserID: authPayload.UserID,
		Reason:         reason,
	})
	if err != nil {
		switch {
		case errors.Is(err, db.ErrDiningSessionNotFound):
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("dining session not found")))
			return
		case errors.Is(err, db.ErrDiningSessionNotOpen):
			ctx.JSON(http.StatusConflict, errorResponse(errors.New("dining session is not open")))
			return
		case errors.Is(err, db.ErrTargetTableDisabled):
			ctx.JSON(http.StatusConflict, errorResponse(errors.New("target table is disabled")))
			return
		case errors.Is(err, db.ErrTargetTableOccupied):
			ctx.JSON(http.StatusConflict, errorResponse(errors.New("target table is occupied")))
			return
		case errors.Is(err, db.ErrTargetTableReserved):
			ctx.JSON(http.StatusConflict, errorResponse(errors.New("target table is reserved")))
			return
		case errors.Is(err, db.ErrReservationMismatch):
			ctx.JSON(http.StatusConflict, errorResponse(errors.New("reservation mismatch")))
			return
		default:
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}

	if server.wsHub != nil {
		fromPayload, _ := json.Marshal(map[string]any{
			"id":       result.FromTable.ID,
			"table_no": result.FromTable.TableNo,
			"status":   result.FromTable.Status,
		})
		server.wsHub.SendToMerchant(result.Session.MerchantID, websocket.Message{
			Type:      "table_status_change",
			Data:      fromPayload,
			Timestamp: time.Now(),
		})

		toPayload, _ := json.Marshal(map[string]any{
			"id":       result.ToTable.ID,
			"table_no": result.ToTable.TableNo,
			"status":   result.ToTable.Status,
		})
		server.wsHub.SendToMerchant(result.Session.MerchantID, websocket.Message{
			Type:      "table_status_change",
			Data:      toPayload,
			Timestamp: time.Now(),
		})

		transferPayload, _ := json.Marshal(map[string]any{
			"session_id":   result.Session.ID,
			"from_table_id": result.FromTable.ID,
			"to_table_id":   result.ToTable.ID,
			"operator_id":   authPayload.UserID,
		})
		server.wsHub.SendToMerchant(result.Session.MerchantID, websocket.Message{
			Type:      "table_transfer",
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

func (server *Server) getOrCreateDefaultBillingGroup(ctx *gin.Context, session db.DiningSession, userID int64) (db.BillingGroup, error) {
	billingGroup, err := server.store.GetDefaultBillingGroupBySession(ctx, session.ID)
	if err != nil {
		if !isNotFoundError(err) {
			return db.BillingGroup{}, err
		}

		billingGroup, err = server.store.CreateBillingGroup(ctx, db.CreateBillingGroupParams{
			DiningSessionID: session.ID,
			Status:          "open",
			IsDefault:       true,
			TotalAmount:     0,
			PaidAmount:      0,
		})
		if err != nil {
			if db.ErrorCode(err) != db.UniqueViolation {
				return db.BillingGroup{}, err
			}
			billingGroup, err = server.store.GetDefaultBillingGroupBySession(ctx, session.ID)
			if err != nil {
				return db.BillingGroup{}, err
			}
		}
	}

	role := "member"
	if session.UserID == userID {
		role = "owner"
	}
	if _, err := server.store.GetActiveBillingGroupMember(ctx, db.GetActiveBillingGroupMemberParams{
		BillingGroupID: billingGroup.ID,
		UserID:         userID,
	}); err != nil {
		if !isNotFoundError(err) {
			return db.BillingGroup{}, err
		}
		if _, err := server.store.CreateBillingGroupMember(ctx, db.CreateBillingGroupMemberParams{
			BillingGroupID: billingGroup.ID,
			UserID:         userID,
			Role:           role,
		}); err != nil && db.ErrorCode(err) != db.UniqueViolation {
			return db.BillingGroup{}, err
		}
	}

	return billingGroup, nil
}

func (server *Server) findActiveReservationForTable(ctx *gin.Context, tableID int64, now time.Time) (*db.TableReservation, error) {
	date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	reservations, err := server.store.ListReservationsByTableAndDate(ctx, db.ListReservationsByTableAndDateParams{
		TableID: tableID,
		ReservationDate: pgtype.Date{
			Time:  date,
			Valid: true,
		},
	})
	if err != nil {
		return nil, err
	}

	for _, r := range reservations {
		if r.Status != ReservationStatusPending && r.Status != ReservationStatusPaid && r.Status != ReservationStatusConfirmed && r.Status != ReservationStatusCheckedIn {
			continue
		}
		if !r.ReservationTime.Valid {
			continue
		}
		start := combineDateAndTime(r.ReservationDate.Time, r.ReservationTime)
		windowStart := start.Add(-30 * time.Minute)
		end := start.Add(time.Duration(ReservationDurationHours) * time.Hour)
		if now.Before(windowStart) || now.After(end) {
			continue
		}
		res := r
		return &res, nil
	}

	return nil, nil
}

func combineDateAndTime(d time.Time, t pgtype.Time) time.Time {
	if !t.Valid {
		return d
	}
	hours := t.Microseconds / int64(time.Microsecond) / int64(time.Hour)
	minutes := (t.Microseconds / int64(time.Microsecond) % int64(time.Hour)) / int64(time.Minute)
	return time.Date(d.Year(), d.Month(), d.Day(), int(hours), int(minutes), 0, 0, d.Location())
}
