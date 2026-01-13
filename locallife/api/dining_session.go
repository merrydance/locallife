package api

import (
	"errors"
	"net/http"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
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
	TableID       int64  `json:"table_id" binding:"required,min=1"`
	ReservationID *int64 `json:"reservation_id,omitempty" binding:"omitempty,min=1"`
}

type openDiningSessionResponse struct {
	Session       diningSessionResponse `json:"session"`
	CartID        *int64                `json:"cart_id,omitempty"`
	ImportedItems int                   `json:"imported_items"`
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
		if errors.Is(err, pgx.ErrNoRows) {
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
	} else if !errors.Is(err, pgx.ErrNoRows) {
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
		if errors.Is(err, pgx.ErrNoRows) {
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
			if errors.Is(err, pgx.ErrNoRows) {
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

	// 已有开放会话直接返回（按预订优先，其次按桌台）
	if reservation != nil {
		if existing, err := server.store.GetActiveDiningSessionByReservation(ctx, pgtype.Int8{Int64: reservation.ID, Valid: true}); err == nil {
			ctx.JSON(http.StatusOK, openDiningSessionResponse{Session: newDiningSessionResponse(existing)})
			return
		} else if !errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}
	if existing, err := server.store.GetActiveDiningSessionByTable(ctx, req.TableID); err == nil {
		// 若桌上已有开放会话且未绑定本预订，视为冲突
		if reservation == nil || !existing.ReservationID.Valid || existing.ReservationID.Int64 != reservation.ID {
			ctx.JSON(http.StatusConflict, errorResponse(errors.New("table already has an active session")))
			return
		}
		ctx.JSON(http.StatusOK, openDiningSessionResponse{Session: newDiningSessionResponse(existing)})
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
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
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
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

	ctx.JSON(http.StatusOK, openDiningSessionResponse{
		Session:       newDiningSessionResponse(session),
		CartID:        txResult.CartID,
		ImportedItems: txResult.ImportedItems,
	})
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
