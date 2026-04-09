package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/rules"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/websocket"
	"github.com/merrydance/locallife/worker"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
)

// ==================== 预定管理 ====================

// 支付模式常量
const (
	PaymentModeDeposit = "deposit" // 定金模式，到店点菜
	PaymentModeFull    = "full"    // 全款模式，在线点菜
)

// 超时和退款配置
const (
	PaymentTimeoutMinutes    = 30               // 支付超时时间：30分钟
	RefundDeadlineHours      = 2                // 退款截止：预定时间前2小时
	DefaultDepositAmount     = 100 * fenPerYuan // 默认定金：100元（分）
	ReservationDurationHours = 4                // 用餐时段时长：4小时（用于时间段冲突检测）
)

type createReservationRequest struct {
	TableID      int64             `json:"table_id" binding:"required,min=1"`
	Date         string            `json:"date" binding:"required"` // YYYY-MM-DD
	Time         string            `json:"time" binding:"required"` // HH:MM
	GuestCount   int16             `json:"guest_count" binding:"required,min=1,max=50"`
	ContactName  string            `json:"contact_name" binding:"required,max=50"`
	ContactPhone string            `json:"contact_phone" binding:"required,max=20"`
	PaymentMode  string            `json:"payment_mode" binding:"required,oneof=deposit full"`
	Notes        string            `json:"notes,omitempty" binding:"omitempty,max=500"`
	Items        []reservationItem `json:"items,omitempty" binding:"omitempty,max=50,dive"` // 全款模式预点菜品
}

// reservationItem 预点菜品项
type reservationItem struct {
	DishID   *int64 `json:"dish_id,omitempty" binding:"omitempty,min=1"`
	ComboID  *int64 `json:"combo_id,omitempty" binding:"omitempty,min=1"`
	Quantity int16  `json:"quantity" binding:"required,min=1,max=99"`
}

type reservationResponse struct {
	ID              int64                     `json:"id"`
	TableID         int64                     `json:"table_id"`
	TableNo         string                    `json:"table_no,omitempty"`
	TableType       string                    `json:"table_type,omitempty"`
	UserID          int64                     `json:"user_id"`
	MerchantID      int64                     `json:"merchant_id"`
	MerchantName    string                    `json:"merchant_name,omitempty"`
	MerchantAddress string                    `json:"merchant_address,omitempty"`
	MerchantPhone   string                    `json:"merchant_phone,omitempty"`
	ReservationDate string                    `json:"reservation_date"`
	ReservationTime string                    `json:"reservation_time"`
	GuestCount      int16                     `json:"guest_count"`
	ContactName     string                    `json:"contact_name"`
	ContactPhone    string                    `json:"contact_phone"`
	PaymentMode     string                    `json:"payment_mode"`
	DepositAmount   int64                     `json:"deposit_amount"`
	PrepaidAmount   int64                     `json:"prepaid_amount"`
	RefundDeadline  time.Time                 `json:"refund_deadline"`
	PaymentDeadline time.Time                 `json:"payment_deadline"`
	Status          string                    `json:"status"`
	Notes           *string                   `json:"notes,omitempty"`
	PaidAt          *time.Time                `json:"paid_at,omitempty"`
	ConfirmedAt     *time.Time                `json:"confirmed_at,omitempty"`
	CompletedAt     *time.Time                `json:"completed_at,omitempty"`
	CancelledAt     *time.Time                `json:"cancelled_at,omitempty"`
	CancelReason    *string                   `json:"cancel_reason,omitempty"`
	CreatedAt       time.Time                 `json:"created_at"`
	UpdatedAt       *time.Time                `json:"updated_at,omitempty"`
	Items           []reservationItemResponse `json:"items,omitempty"`
}

type reservationItemResponse struct {
	ID           int64  `json:"id"`
	DishID       *int64 `json:"dish_id,omitempty"`
	ComboID      *int64 `json:"combo_id,omitempty"`
	Name         string `json:"name,omitempty"`
	ImageAssetID *int64 `json:"image_asset_id,omitempty"`
	Quantity     int16  `json:"quantity"`
	UnitPrice    int64  `json:"unit_price"`
	TotalPrice   int64  `json:"total_price"`
	Type         string `json:"type,omitempty"`
}

type reservationListResponse struct {
	Reservations []reservationResponse `json:"reservations"`
	Total        int64                 `json:"total"`
	PageID       int32                 `json:"page_id"`
	PageSize     int32                 `json:"page_size"`
}

type reservationDishSummaryDateResponse struct {
	Date  string                       `json:"date"`
	Items []reservationDishSummaryItem `json:"items"`
}

type addDishesPaymentResponse struct {
	Message        string                `json:"message"`
	PaymentOrderID int64                 `json:"payment_order_id"`
	Amount         int64                 `json:"amount"`
	ItemsCount     int                   `json:"items_count"`
	PayParams      *miniProgramPayParams `json:"pay_params,omitempty"`
}

type addDishesSuccessResponse struct {
	Message    string `json:"message"`
	Amount     int64  `json:"amount"`
	ItemsCount int    `json:"items_count"`
	Note       string `json:"note"`
}

type modifyDishesPaymentResponse struct {
	Message        string                `json:"message"`
	PaymentOrderID int64                 `json:"payment_order_id"`
	Amount         int64                 `json:"amount"`
	ItemsCount     int                   `json:"items_count"`
	PayParams      *miniProgramPayParams `json:"pay_params,omitempty"`
}

type modifyDishesRefundResponse struct {
	Message      string `json:"message"`
	RefundAmount int64  `json:"refund_amount"`
	ItemsCount   int    `json:"items_count"`
}

type modifyDishesSuccessResponse struct {
	Message    string `json:"message"`
	ItemsCount int    `json:"items_count"`
	Delta      int64  `json:"delta"`
}

type reservationSliceResponse struct {
	Reservations []reservationResponse `json:"reservations"`
}

type reservationStatsResponse struct {
	PendingCount   int64 `json:"pending_count"`
	PaidCount      int64 `json:"paid_count"`
	ConfirmedCount int64 `json:"confirmed_count"`
	CheckedInCount int64 `json:"checked_in_count"`
	CompletedCount int64 `json:"completed_count"`
	CancelledCount int64 `json:"cancelled_count"`
	ExpiredCount   int64 `json:"expired_count"`
	NoShowCount    int64 `json:"no_show_count"`
}

func newReservationResponse(r db.TableReservation) reservationResponse {
	resp := reservationResponse{
		ID:              r.ID,
		TableID:         r.TableID,
		UserID:          r.UserID,
		MerchantID:      r.MerchantID,
		ReservationDate: r.ReservationDate.Time.Format("2006-01-02"),
		GuestCount:      r.GuestCount,
		ContactName:     r.ContactName,
		ContactPhone:    r.ContactPhone,
		PaymentMode:     r.PaymentMode,
		DepositAmount:   r.DepositAmount,
		PrepaidAmount:   r.PrepaidAmount,
		RefundDeadline:  r.RefundDeadline,
		PaymentDeadline: r.PaymentDeadline,
		Status:          r.Status,
		CreatedAt:       r.CreatedAt,
	}

	// 格式化时间
	if r.ReservationTime.Valid {
		hours := r.ReservationTime.Microseconds / 1000000 / 3600
		minutes := (r.ReservationTime.Microseconds / 1000000 % 3600) / 60
		resp.ReservationTime = time.Date(0, 1, 1, int(hours), int(minutes), 0, 0, time.UTC).Format("15:04")
	}

	if r.Notes.Valid {
		resp.Notes = &r.Notes.String
	}
	if r.PaidAt.Valid {
		resp.PaidAt = &r.PaidAt.Time
	}
	if r.ConfirmedAt.Valid {
		resp.ConfirmedAt = &r.ConfirmedAt.Time
	}
	if r.CompletedAt.Valid {
		resp.CompletedAt = &r.CompletedAt.Time
	}
	if r.CancelledAt.Valid {
		resp.CancelledAt = &r.CancelledAt.Time
	}
	if r.CancelReason.Valid {
		resp.CancelReason = &r.CancelReason.String
	}
	if r.UpdatedAt.Valid {
		resp.UpdatedAt = &r.UpdatedAt.Time
	}

	return resp
}

func newReservationWithTableResponse(r db.GetTableReservationWithTableRow) reservationResponse {
	resp := reservationResponse{
		ID:              r.ID,
		TableID:         r.TableID,
		TableNo:         r.TableNo,
		TableType:       r.TableType,
		UserID:          r.UserID,
		MerchantID:      r.MerchantID,
		ReservationDate: r.ReservationDate.Time.Format("2006-01-02"),
		GuestCount:      r.GuestCount,
		ContactName:     r.ContactName,
		ContactPhone:    r.ContactPhone,
		PaymentMode:     r.PaymentMode,
		DepositAmount:   r.DepositAmount,
		PrepaidAmount:   r.PrepaidAmount,
		RefundDeadline:  r.RefundDeadline,
		PaymentDeadline: r.PaymentDeadline,
		Status:          r.Status,
		CreatedAt:       r.CreatedAt,
	}

	// 格式化时间
	if r.ReservationTime.Valid {
		hours := r.ReservationTime.Microseconds / 1000000 / 3600
		minutes := (r.ReservationTime.Microseconds / 1000000 % 3600) / 60
		resp.ReservationTime = time.Date(0, 1, 1, int(hours), int(minutes), 0, 0, time.UTC).Format("15:04")
	}

	if r.Notes.Valid {
		resp.Notes = &r.Notes.String
	}
	if r.PaidAt.Valid {
		resp.PaidAt = &r.PaidAt.Time
	}
	if r.ConfirmedAt.Valid {
		resp.ConfirmedAt = &r.ConfirmedAt.Time
	}
	if r.CompletedAt.Valid {
		resp.CompletedAt = &r.CompletedAt.Time
	}
	if r.CancelledAt.Valid {
		resp.CancelledAt = &r.CancelledAt.Time
	}
	if r.CancelReason.Valid {
		resp.CancelReason = &r.CancelReason.String
	}
	if r.UpdatedAt.Valid {
		resp.UpdatedAt = &r.UpdatedAt.Time
	}

	return resp
}

func mapOrderItemsToReservationItems(items []db.ListOrderItemsWithDishByOrderRow) []reservationItemResponse {
	resp := make([]reservationItemResponse, 0, len(items))

	for _, item := range items {
		mapped := reservationItemResponse{
			ID:         item.ID,
			Quantity:   item.Quantity,
			UnitPrice:  item.UnitPrice,
			TotalPrice: item.Subtotal,
			Name:       item.Name,
		}

		if item.DishID.Valid {
			mapped.DishID = &item.DishID.Int64
			mapped.Type = "dish"
		}
		if item.ComboID.Valid {
			mapped.ComboID = &item.ComboID.Int64
			mapped.Type = "combo"
		}
		if item.DishImageMediaAssetID.Valid {
			mapped.ImageAssetID = &item.DishImageMediaAssetID.Int64
		}

		resp = append(resp, mapped)
	}

	return resp
}

// createReservation 创建预定 (用户)
// @Summary 创建包间预定
// @Description 用户创建包间预定，支持定金模式（到店点菜）和全款模式（在线预点菜品）
// @Tags 预定管理
// @Accept json
// @Produce json
// @Param body body createReservationRequest true "预定请求"
// @Success 200 {object} reservationResponse
// @Failure 400 {object} ErrorResponse "参数错误/桌台不可预定/时间段已被预定"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "桌台不存在"
// @Failure 409 {object} ErrorResponse "时间段已被预定"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /v1/reservations [post]
func (server *Server) createReservation(ctx *gin.Context) {
	var req createReservationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 解析日期和时间
	reservationDate, err := parseISODate(req.Date, "invalid date format, use YYYY-MM-DD")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	reservationTime, err := time.Parse("15:04", req.Time)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid time format, use HH:MM")))
		return
	}

	reservationItems := make([]logic.ReservationItemInput, len(req.Items))
	for i, item := range req.Items {
		reservationItems[i] = logic.ReservationItemInput{
			DishID:   item.DishID,
			ComboID:  item.ComboID,
			Quantity: item.Quantity,
		}
	}

	now := time.Now()
	result, err := logic.CreateReservation(ctx, server.store, logic.CreateReservationInput{
		UserID:              authPayload.UserID,
		TableID:             req.TableID,
		ReservationDate:     reservationDate,
		ReservationTime:     reservationTime,
		GuestCount:          req.GuestCount,
		ContactName:         req.ContactName,
		ContactPhone:        req.ContactPhone,
		PaymentMode:         req.PaymentMode,
		Notes:               req.Notes,
		Items:               reservationItems,
		Now:                 now,
		PaymentTimeoutMins:  PaymentTimeoutMinutes,
		RefundDeadlineHours: RefundDeadlineHours,
		DefaultDeposit:      DefaultDepositAmount,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 预订成功提醒：若用户在外卖拒绝服务名单中，通知商户后台
	if server.wsHub != nil {
		// 发送新预订通知给商户
		newReservationPayload, _ := json.Marshal(map[string]any{
			"reservation": newReservationResponse(result.Reservation),
		})
		server.wsHub.SendToMerchant(result.Reservation.MerchantID, websocket.Message{
			Type:      "reservation_new",
			Data:      newReservationPayload,
			Timestamp: time.Now(),
		})

		if server.config.RulesEngineEnabled && server.rulesEngine != nil {
			merchant, err := server.store.GetMerchant(ctx, result.Reservation.MerchantID)
			if err == nil {
				ruleInput := rules.Context{
					Domain:     rules.DomainReservation,
					RegionID:   merchant.RegionID,
					MerchantID: merchant.ID,
					UserID:     authPayload.UserID,
				}
				decision, err := server.rulesEngine.Evaluate(ctx, ruleInput)
				if err == nil {
					server.recordRuleHit(ctx, ruleInput, decision, RoleCustomer)
					if decision.Action == "alert" {
						message := decision.Reason
						if message == "" {
							message = "该顾客有多次恶意索赔记录，谨慎服务"
						}
						alertPayload, _ := json.Marshal(map[string]any{
							"user_id":        authPayload.UserID,
							"reservation_id": result.Reservation.ID,
							"merchant_id":    result.Reservation.MerchantID,
							"message":        message,
						})
						server.wsHub.SendToMerchant(result.Reservation.MerchantID, websocket.Message{
							Type:      "merchant_user_risk_alert",
							Data:      alertPayload,
							Timestamp: time.Now(),
						})
					}
				}
			}
		} else {
			block, err := server.store.GetActiveBehaviorBlocklist(ctx, db.GetActiveBehaviorBlocklistParams{
				EntityType: "user",
				EntityID:   authPayload.UserID,
			})
			if err == nil {
				alertPayload, _ := json.Marshal(map[string]any{
					"user_id":        authPayload.UserID,
					"reservation_id": result.Reservation.ID,
					"merchant_id":    result.Reservation.MerchantID,
					"reason_code":    block.ReasonCode,
					"message":        "该顾客有多次恶意索赔记录，谨慎服务",
				})
				server.wsHub.SendToMerchant(result.Reservation.MerchantID, websocket.Message{
					Type:      "merchant_user_risk_alert",
					Data:      alertPayload,
					Timestamp: time.Now(),
				})
			}
		}
	}

	// 创建支付超时任务
	// 使用 Asynq 任务队列，在 PaymentDeadline 时检查预定状态
	if server.taskDistributor != nil {
		err = server.taskDistributor.DistributeTaskReservationPaymentTimeout(
			ctx,
			&worker.PayloadReservationPaymentTimeout{ReservationID: result.Reservation.ID},
			asynq.ProcessAt(result.Reservation.PaymentDeadline),
		)
		if err != nil {
			// 任务分发失败不影响主流程，定时任务轮询超时预定作为兜底
			log.Warn().Err(err).Int64("reservation_id", result.Reservation.ID).Msg("failed to distribute reservation timeout task, non-fatal")
		}
	}

	ctx.JSON(http.StatusCreated, newReservationResponse(result.Reservation))
}

type getReservationRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// getReservation 获取预定详情（用户或商户）
// @Summary 获取预定详情
// @Description 用户查看自己的预定详情，或商户查看自己商户的预定详情
// @Tags 预定管理
// @Accept json
// @Produce json
// @Param id path int64 true "预定ID"
// @Success 201 {object} reservationResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "无权访问该预定"
// @Failure 404 {object} ErrorResponse "预定不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /v1/reservations/{id} [get]
func (server *Server) getReservation(ctx *gin.Context) {
	var req getReservationRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	reservation, err := server.store.GetTableReservationWithTable(ctx, req.ID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("reservation not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	merchant, err := server.store.GetMerchant(ctx, reservation.MerchantID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 权限验证：用户查看自己的预定，或商户查看自己商户的预定
	isOwner := reservation.UserID == authPayload.UserID
	isMerchant := false
	if !isOwner {
		merchant, err := server.resolveMerchantForUser(ctx, authPayload.UserID)
		if err == nil && merchant.ID == reservation.MerchantID {
			isMerchant = true
		}
	}

	if !isOwner && !isMerchant {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not authorized to view this reservation")))
		return
	}

	resp := newReservationWithTableResponse(reservation)
	resp.MerchantName = merchant.Name
	resp.MerchantAddress = merchant.Address
	resp.MerchantPhone = merchant.Phone

	orders, err := server.store.ListOrdersByUserWithFilters(ctx, db.ListOrdersByUserWithFiltersParams{
		UserID: reservation.UserID,
		Status: pgtype.Text{},
		OrderType: pgtype.Text{
			String: OrderTypeReservation,
			Valid:  true,
		},
		ReservationID: pgtype.Int8{
			Int64: reservation.ID,
			Valid: true,
		},
		Limit:  5,
		Offset: 0,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	for _, order := range orders {
		if order.Status == OrderStatusCancelled {
			continue
		}

		orderItems, err := server.store.ListOrderItemsWithDishByOrder(ctx, order.ID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		resp.Items = mapOrderItemsToReservationItems(orderItems)
		break
	}

	ctx.JSON(http.StatusOK, resp)
}

type listUserReservationsRequest struct {
	PageID   int32  `form:"page_id" binding:"required,min=1"`
	PageSize int32  `form:"page_size" binding:"required,min=5,max=50"`
	Status   string `form:"status" binding:"omitempty,oneof=pending paid confirmed checked_in completed cancelled expired no_show"`
}

// listUserReservations 用户查看自己的预定列表
// @Summary 获取我的预定列表
// @Description 用户查看自己的所有预定记录
// @Tags 预定管理
// @Accept json
// @Produce json
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(5) maximum(50)
// @Success 200 {object} object{reservations=[]reservationResponse}
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /v1/reservations/me [get]
func (server *Server) listUserReservations(ctx *gin.Context) {
	var req listUserReservationsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	status := pgtype.Text{}
	if req.Status != "" {
		status = pgtype.Text{String: req.Status, Valid: true}
	}

	reservations, err := server.store.ListReservationsByUserWithStatus(ctx, db.ListReservationsByUserWithStatusParams{
		UserID: authPayload.UserID,
		Status: status,
		Limit:  req.PageSize,
		Offset: pageOffset(req.PageID, req.PageSize),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := make([]reservationResponse, len(reservations))
	merchantCache := make(map[int64]db.Merchant)

	for i, r := range reservations {
		merchant, ok := merchantCache[r.MerchantID]
		if !ok {
			m, err := server.store.GetMerchant(ctx, r.MerchantID)
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
				return
			}
			merchant = m
			merchantCache[r.MerchantID] = m
		}

		resp[i] = reservationResponse{
			ID:              r.ID,
			TableID:         r.TableID,
			TableNo:         r.TableNo,
			TableType:       r.TableType,
			UserID:          r.UserID,
			MerchantID:      r.MerchantID,
			MerchantName:    merchant.Name,
			MerchantAddress: merchant.Address,
			MerchantPhone:   merchant.Phone,
			ReservationDate: r.ReservationDate.Time.Format("2006-01-02"),
			GuestCount:      r.GuestCount,
			ContactName:     r.ContactName,
			ContactPhone:    r.ContactPhone,
			PaymentMode:     r.PaymentMode,
			DepositAmount:   r.DepositAmount,
			PrepaidAmount:   r.PrepaidAmount,
			RefundDeadline:  r.RefundDeadline,
			PaymentDeadline: r.PaymentDeadline,
			Status:          r.Status,
			CreatedAt:       r.CreatedAt,
		}
		if r.ReservationTime.Valid {
			hours := r.ReservationTime.Microseconds / 1000000 / 3600
			minutes := (r.ReservationTime.Microseconds / 1000000 % 3600) / 60
			resp[i].ReservationTime = time.Date(0, 1, 1, int(hours), int(minutes), 0, 0, time.UTC).Format("15:04")
		}
	}

	var totalCount int64
	if status.Valid {
		count, err := server.store.CountReservationsByUserAndStatus(ctx, db.CountReservationsByUserAndStatusParams{
			UserID: authPayload.UserID,
			Status: status.String,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		totalCount = count
	} else {
		statuses := []string{
			ReservationStatusPending,
			ReservationStatusPaid,
			ReservationStatusConfirmed,
			ReservationStatusCheckedIn,
			ReservationStatusCompleted,
			ReservationStatusCancelled,
			ReservationStatusExpired,
			ReservationStatusNoShow,
		}
		for _, s := range statuses {
			count, err := server.store.CountReservationsByUserAndStatus(ctx, db.CountReservationsByUserAndStatusParams{
				UserID: authPayload.UserID,
				Status: s,
			})
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
				return
			}
			totalCount += count
		}
	}

	ctx.JSON(http.StatusOK, reservationListResponse{
		Reservations: resp,
		Total:        totalCount,
		PageID:       req.PageID,
		PageSize:     req.PageSize,
	})
}

type listMerchantReservationsRequest struct {
	Date     string `form:"date,omitempty"`                                                                                                             // YYYY-MM-DD
	Status   string `form:"status,omitempty" binding:"omitempty,oneof=pending paid confirmed checked_in completed cancelled expired no_show exception"` // 状态筛选
	PageID   int32  `form:"page_id" binding:"required,min=1"`
	PageSize int32  `form:"page_size" binding:"required,min=5,max=200"`
}

type listMerchantReservationDishesRequest struct {
	Date string `form:"date" binding:"required"` // YYYY-MM-DD
}

type reservationDishReference struct {
	ReservationID   int64  `json:"reservation_id"`
	ReservationTime string `json:"reservation_time"`
	TableNo         string `json:"table_no,omitempty"`
	ContactName     string `json:"contact_name,omitempty"`
	Status          string `json:"status"`
	Quantity        int16  `json:"quantity"`
}

type reservationDishSummaryItem struct {
	Type             string                     `json:"type"`
	DishID           *int64                     `json:"dish_id,omitempty"`
	ComboID          *int64                     `json:"combo_id,omitempty"`
	Name             string                     `json:"name"`
	TotalQuantity    int64                      `json:"total_quantity"`
	ReservationCount int64                      `json:"reservation_count"`
	References       []reservationDishReference `json:"references"`
}

// listMerchantReservationDishes 商户按天查看预订菜品备菜清单
// @Summary 获取商户预订菜品备菜清单
// @Description 按日期聚合预订中的菜品/套餐数量，便于后厨备菜
// @Tags 预定管理-商户
// @Accept json
// @Produce json
// @Param date query string true "日期 (YYYY-MM-DD)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "非商户"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /v1/reservations/merchant/dishes [get]
func (server *Server) listMerchantReservationDishes(ctx *gin.Context) {
	var req listMerchantReservationDishesRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	reservationDate, err := parseISODate(req.Date, "invalid date format")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.getMerchantFromContextOrStore(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	reservations, err := server.store.ListReservationsByMerchantAndDate(ctx, db.ListReservationsByMerchantAndDateParams{
		MerchantID:      merchant.ID,
		ReservationDate: pgtype.Date{Time: reservationDate, Valid: true},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	type aggValue struct {
		item    reservationDishSummaryItem
		seenRes map[int64]struct{}
	}

	agg := map[string]*aggValue{}
	for _, reservation := range reservations {
		if reservation.Status == "cancelled" || reservation.Status == "expired" || reservation.Status == "no_show" {
			continue
		}

		items, err := server.store.ListReservationItems(ctx, reservation.ID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		reservationTime := ""
		if reservation.ReservationTime.Valid {
			hours := reservation.ReservationTime.Microseconds / 1000000 / 3600
			minutes := (reservation.ReservationTime.Microseconds / 1000000 % 3600) / 60
			reservationTime = time.Date(0, 1, 1, int(hours), int(minutes), 0, 0, time.UTC).Format("15:04")
		}

		for _, item := range items {
			if !item.DishID.Valid && !item.ComboID.Valid {
				continue
			}

			var key string
			var name string
			typ := "dish"

			if item.DishID.Valid {
				key = fmt.Sprintf("dish:%d", item.DishID.Int64)
				if item.DishName.Valid {
					name = item.DishName.String
				}
			} else {
				typ = "combo"
				key = fmt.Sprintf("combo:%d", item.ComboID.Int64)
				if item.ComboName.Valid {
					name = item.ComboName.String
				}
			}

			if name == "" {
				name = "未命名"
			}

			entry, ok := agg[key]
			if !ok {
				summary := reservationDishSummaryItem{
					Type:       typ,
					Name:       name,
					References: make([]reservationDishReference, 0, 8),
				}
				if item.DishID.Valid {
					dishID := item.DishID.Int64
					summary.DishID = &dishID
				}
				if item.ComboID.Valid {
					comboID := item.ComboID.Int64
					summary.ComboID = &comboID
				}

				entry = &aggValue{
					item:    summary,
					seenRes: map[int64]struct{}{},
				}
				agg[key] = entry
			}

			entry.item.TotalQuantity += int64(item.Quantity)
			if _, seen := entry.seenRes[reservation.ID]; !seen {
				entry.item.ReservationCount++
				entry.seenRes[reservation.ID] = struct{}{}
			}
			entry.item.References = append(entry.item.References, reservationDishReference{
				ReservationID:   reservation.ID,
				ReservationTime: reservationTime,
				TableNo:         reservation.TableNo,
				ContactName:     reservation.ContactName,
				Status:          reservation.Status,
				Quantity:        item.Quantity,
			})
		}
	}

	result := make([]reservationDishSummaryItem, 0, len(agg))
	for _, value := range agg {
		sort.Slice(value.item.References, func(i, j int) bool {
			return value.item.References[i].ReservationTime < value.item.References[j].ReservationTime
		})
		result = append(result, value.item)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].TotalQuantity == result[j].TotalQuantity {
			return result[i].Name < result[j].Name
		}
		return result[i].TotalQuantity > result[j].TotalQuantity
	})

	ctx.JSON(http.StatusOK, reservationDishSummaryDateResponse{
		Date:  req.Date,
		Items: result,
	})
}

// listMerchantReservations 商户查看预定列表
// @Summary 获取商户预定列表
// @Description 商户查看自己店铺的所有预定记录，支持按日期或状态筛选
// @Tags 预定管理-商户
// @Accept json
// @Produce json
// @Param date query string false "筛选日期 (YYYY-MM-DD)"
// @Param status query string false "筛选状态" Enums(pending, paid, confirmed, checked_in, completed, cancelled, expired, no_show, exception)
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(5) maximum(50)
// @Success 200 {object} reservationListResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "非商户"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /v1/reservations/merchant [get]
func (server *Server) listMerchantReservations(ctx *gin.Context) {
	var req listMerchantReservationsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取商户ID
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.getMerchantFromContextOrStore(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	var reservations []db.ListReservationsByMerchantRow

	if req.Date != "" {
		// 按日期查询
		date, err := parseISODate(req.Date, "invalid date format")
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}
		dateReservations, err := server.store.ListReservationsByMerchantAndDate(ctx, db.ListReservationsByMerchantAndDateParams{
			MerchantID:      merchant.ID,
			ReservationDate: pgtype.Date{Time: date, Valid: true},
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		filteredReservations := make([]db.ListReservationsByMerchantAndDateRow, 0, len(dateReservations))
		for _, reservation := range dateReservations {
			if !matchesMerchantReservationStatusFilter(reservation.Status, req.Status) {
				continue
			}
			filteredReservations = append(filteredReservations, reservation)
		}

		totalCount := int64(len(filteredReservations))
		start := int(pageOffset(req.PageID, req.PageSize))
		if start > len(filteredReservations) {
			start = len(filteredReservations)
		}
		end := start + int(req.PageSize)
		if end > len(filteredReservations) {
			end = len(filteredReservations)
		}

		// 转换类型
		for _, r := range filteredReservations[start:end] {
			reservations = append(reservations, db.ListReservationsByMerchantRow{
				ID:              r.ID,
				TableID:         r.TableID,
				UserID:          r.UserID,
				MerchantID:      r.MerchantID,
				ReservationDate: r.ReservationDate,
				ReservationTime: r.ReservationTime,
				GuestCount:      r.GuestCount,
				ContactName:     r.ContactName,
				ContactPhone:    r.ContactPhone,
				PaymentMode:     r.PaymentMode,
				DepositAmount:   r.DepositAmount,
				PrepaidAmount:   r.PrepaidAmount,
				RefundDeadline:  r.RefundDeadline,
				Status:          r.Status,
				PaymentDeadline: r.PaymentDeadline,
				Notes:           r.Notes,
				PaidAt:          r.PaidAt,
				ConfirmedAt:     r.ConfirmedAt,
				CompletedAt:     r.CompletedAt,
				CancelledAt:     r.CancelledAt,
				CancelReason:    r.CancelReason,
				CreatedAt:       r.CreatedAt,
				UpdatedAt:       r.UpdatedAt,
				TableNo:         r.TableNo,
				TableType:       r.TableType,
			})
		}

		resp := make([]reservationResponse, len(reservations))
		for i, r := range reservations {
			resp[i] = reservationResponse{
				ID:              r.ID,
				TableID:         r.TableID,
				TableNo:         r.TableNo,
				TableType:       r.TableType,
				UserID:          r.UserID,
				MerchantID:      r.MerchantID,
				ReservationDate: r.ReservationDate.Time.Format("2006-01-02"),
				GuestCount:      r.GuestCount,
				ContactName:     r.ContactName,
				ContactPhone:    r.ContactPhone,
				PaymentMode:     r.PaymentMode,
				DepositAmount:   r.DepositAmount,
				PrepaidAmount:   r.PrepaidAmount,
				RefundDeadline:  r.RefundDeadline,
				PaymentDeadline: r.PaymentDeadline,
				Status:          r.Status,
				CreatedAt:       r.CreatedAt,
			}
			if r.ReservationTime.Valid {
				hours := r.ReservationTime.Microseconds / 1000000 / 3600
				minutes := (r.ReservationTime.Microseconds / 1000000 % 3600) / 60
				resp[i].ReservationTime = time.Date(0, 1, 1, int(hours), int(minutes), 0, 0, time.UTC).Format("15:04")
			}

			items, err := server.store.ListReservationItems(ctx, r.ID)
			if err == nil && len(items) > 0 {
				resp[i].Items = make([]reservationItemResponse, len(items))
				for j, item := range items {
					resp[i].Items[j] = reservationItemResponse{
						ID:         item.ID,
						Quantity:   item.Quantity,
						UnitPrice:  item.UnitPrice,
						TotalPrice: int64(item.Quantity) * item.UnitPrice,
						Type:       "dish",
					}

					if item.DishID.Valid {
						resp[i].Items[j].DishID = &item.DishID.Int64
						if item.DishName.Valid {
							resp[i].Items[j].Name = item.DishName.String
						}
						if item.DishImageMediaAssetID.Valid {
							resp[i].Items[j].ImageAssetID = &item.DishImageMediaAssetID.Int64
						}
					} else if item.ComboID.Valid {
						resp[i].Items[j].ComboID = &item.ComboID.Int64
						resp[i].Items[j].Type = "combo"
						if item.ComboName.Valid {
							resp[i].Items[j].Name = item.ComboName.String
						}
						if item.ComboImageMediaAssetID.Valid {
							resp[i].Items[j].ImageAssetID = &item.ComboImageMediaAssetID.Int64
						}
					}
				}
			}
		}

		ctx.JSON(http.StatusOK, reservationListResponse{
			Reservations: resp,
			Total:        totalCount,
			PageID:       req.PageID,
			PageSize:     req.PageSize,
		})
		return
	} else if req.Status != "" {
		if req.Status == "exception" {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("status=exception requires date filter")))
			return
		}
		// 按状态查询
		statusReservations, err := server.store.ListReservationsByMerchantAndStatus(ctx, db.ListReservationsByMerchantAndStatusParams{
			MerchantID: merchant.ID,
			Status:     req.Status,
			Limit:      req.PageSize,
			Offset:     pageOffset(req.PageID, req.PageSize),
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		// 转换类型
		for _, r := range statusReservations {
			reservations = append(reservations, db.ListReservationsByMerchantRow{
				ID:              r.ID,
				TableID:         r.TableID,
				UserID:          r.UserID,
				MerchantID:      r.MerchantID,
				ReservationDate: r.ReservationDate,
				ReservationTime: r.ReservationTime,
				GuestCount:      r.GuestCount,
				ContactName:     r.ContactName,
				ContactPhone:    r.ContactPhone,
				PaymentMode:     r.PaymentMode,
				DepositAmount:   r.DepositAmount,
				PrepaidAmount:   r.PrepaidAmount,
				RefundDeadline:  r.RefundDeadline,
				Status:          r.Status,
				PaymentDeadline: r.PaymentDeadline,
				Notes:           r.Notes,
				PaidAt:          r.PaidAt,
				ConfirmedAt:     r.ConfirmedAt,
				CompletedAt:     r.CompletedAt,
				CancelledAt:     r.CancelledAt,
				CancelReason:    r.CancelReason,
				CreatedAt:       r.CreatedAt,
				UpdatedAt:       r.UpdatedAt,
				TableNo:         r.TableNo,
				TableType:       r.TableType,
			})
		}
	} else {
		// 默认查询
		reservations, err = server.store.ListReservationsByMerchant(ctx, db.ListReservationsByMerchantParams{
			MerchantID: merchant.ID,
			Limit:      req.PageSize,
			Offset:     pageOffset(req.PageID, req.PageSize),
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}

	resp := make([]reservationResponse, len(reservations))
	for i, r := range reservations {
		resp[i] = reservationResponse{
			ID:              r.ID,
			TableID:         r.TableID,
			TableNo:         r.TableNo,
			TableType:       r.TableType,
			UserID:          r.UserID,
			MerchantID:      r.MerchantID,
			ReservationDate: r.ReservationDate.Time.Format("2006-01-02"),
			GuestCount:      r.GuestCount,
			ContactName:     r.ContactName,
			ContactPhone:    r.ContactPhone,
			PaymentMode:     r.PaymentMode,
			DepositAmount:   r.DepositAmount,
			PrepaidAmount:   r.PrepaidAmount,
			RefundDeadline:  r.RefundDeadline,
			PaymentDeadline: r.PaymentDeadline,
			Status:          r.Status,
			CreatedAt:       r.CreatedAt,
		}
		if r.ReservationTime.Valid {
			hours := r.ReservationTime.Microseconds / 1000000 / 3600
			minutes := (r.ReservationTime.Microseconds / 1000000 % 3600) / 60
			resp[i].ReservationTime = time.Date(0, 1, 1, int(hours), int(minutes), 0, 0, time.UTC).Format("15:04")
		}

		// 填充预订菜品
		items, err := server.store.ListReservationItems(ctx, r.ID)
		if err == nil && len(items) > 0 {
			resp[i].Items = make([]reservationItemResponse, len(items))
			for j, item := range items {
				resp[i].Items[j] = reservationItemResponse{
					ID:         item.ID,
					Quantity:   item.Quantity,
					UnitPrice:  item.UnitPrice,
					TotalPrice: int64(item.Quantity) * item.UnitPrice,
					Type:       "dish",
				}

				if item.DishID.Valid {
					resp[i].Items[j].DishID = &item.DishID.Int64
					if item.DishName.Valid {
						resp[i].Items[j].Name = item.DishName.String
					}
					if item.DishImageMediaAssetID.Valid {
						resp[i].Items[j].ImageAssetID = &item.DishImageMediaAssetID.Int64
					}
				} else if item.ComboID.Valid {
					resp[i].Items[j].ComboID = &item.ComboID.Int64
					resp[i].Items[j].Type = "combo"
					if item.ComboName.Valid {
						resp[i].Items[j].Name = item.ComboName.String
					}
					if item.ComboImageMediaAssetID.Valid {
						resp[i].Items[j].ImageAssetID = &item.ComboImageMediaAssetID.Int64
					}
				}
			}
		}
	}

	var totalCount int64
	if req.Date != "" {
		date, err := parseISODate(req.Date, "invalid date format")
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}
		count, err := server.store.CountReservationsByMerchantAndDate(ctx, db.CountReservationsByMerchantAndDateParams{
			MerchantID:      merchant.ID,
			ReservationDate: pgtype.Date{Time: date, Valid: true},
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		totalCount = count
	} else if req.Status != "" {
		count, err := server.store.CountReservationsByMerchantAndStatus(ctx, db.CountReservationsByMerchantAndStatusParams{
			MerchantID: merchant.ID,
			Status:     req.Status,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		totalCount = count
	} else {
		count, err := server.store.CountReservationsByMerchant(ctx, merchant.ID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		totalCount = count
	}

	ctx.JSON(http.StatusOK, reservationListResponse{
		Reservations: resp,
		Total:        totalCount,
		PageID:       req.PageID,
		PageSize:     req.PageSize,
	})
}

func matchesMerchantReservationStatusFilter(status string, filter string) bool {
	if filter == "" {
		return true
	}

	if filter == "exception" {
		return status == "cancelled" || status == "expired" || status == "no_show"
	}

	return status == filter
}

// ==================== 预定状态变更 ====================

// confirmReservation 确认预定 (商户)
// @Summary 确认预定
// @Description 商户确认已支付的预定，更新桌台状态为已占用
// @Tags 预定管理-商户
// @Accept json
// @Produce json
// @Param id path int64 true "预定ID"
// @Success 200 {object} reservationResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "非商户或预定不属于该商户"
// @Failure 404 {object} ErrorResponse "预定不存在"
// @Failure 409 {object} ErrorResponse "预定状态不允许确认"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /v1/reservations/{id}/confirm [post]
func (server *Server) confirmReservation(ctx *gin.Context) {
	var req getReservationRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 验证商户权限
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	result, err := logic.ConfirmReservation(ctx, server.store, authPayload.UserID, req.ID)
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 创建未到店提醒任务
	if server.taskDistributor != nil {
		hours := result.Reservation.ReservationTime.Microseconds / 1000000 / 3600
		minutes := (result.Reservation.ReservationTime.Microseconds / 1000000 % 3600) / 60
		alertTime := time.Date(
			result.Reservation.ReservationDate.Time.Year(), result.Reservation.ReservationDate.Time.Month(), result.Reservation.ReservationDate.Time.Day(),
			int(hours), int(minutes), 0, 0, time.Local,
		)

		_ = server.taskDistributor.DistributeTaskReservationNoShowAlert(
			ctx,
			&worker.PayloadReservationNoShowAlert{ReservationID: result.Reservation.ID},
			asynq.ProcessAt(alertTime),
		)
	}

	ctx.JSON(http.StatusOK, newReservationResponse(result.Reservation))
}

// completeReservation 完成预定 (商户) - 客人离店
// @Summary 完成预定
// @Description 商户标记预定完成（客人离店），释放桌台
// @Tags 预定管理-商户
// @Accept json
// @Produce json
// @Param id path int64 true "预定ID"
// @Success 200 {object} reservationResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "非商户或预定不属于该商户"
// @Failure 404 {object} ErrorResponse "预定不存在"
// @Failure 409 {object} ErrorResponse "预定状态不允许完成"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /v1/reservations/{id}/complete [post]
func (server *Server) completeReservation(ctx *gin.Context) {
	var req getReservationRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 验证商户权限
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	result, err := logic.CompleteReservation(ctx, server.store, authPayload.UserID, req.ID)
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newReservationResponse(result.Reservation))
}

type cancelReservationRequest struct {
	ID     int64  `uri:"id" binding:"required,min=1"`
	Reason string `json:"reason,omitempty" binding:"omitempty,max=500"`
}

// cancelReservation 取消预定 (用户或商户)
// @Summary 取消预定
// @Description 用户取消自己的预定，或商户取消店铺的预定。已支付的预定按配置策略支持截止前后差异化退款（含部分退款）。
// @Tags 预定管理
// @Accept json
// @Produce json
// @Param id path int64 true "预定ID"
// @Param body body cancelReservationRequest false "取消原因"
// @Success 200 {object} reservationResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "无权取消该预定"
// @Failure 404 {object} ErrorResponse "预定不存在"
// @Failure 409 {object} ErrorResponse "预定状态不允许取消/退款截止时间已过"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /v1/reservations/{id}/cancel [post]
func (server *Server) cancelReservation(ctx *gin.Context) {
	var uriReq struct {
		ID int64 `uri:"id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req cancelReservationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		// 允许空 body
		req = cancelReservationRequest{}
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	policy := logic.ReservationRefundPolicy{
		UserBeforeDeadlinePercent:     server.config.ReservationUserRefundPercentBeforeDeadline,
		UserAfterDeadlinePercent:      server.config.ReservationUserRefundPercentAfterDeadline,
		MerchantBeforeDeadlinePercent: server.config.ReservationMerchantRefundPercentBeforeDeadline,
		MerchantAfterDeadlinePercent:  server.config.ReservationMerchantRefundPercentAfterDeadline,
	}
	result, err := logic.CancelReservation(ctx, server.store, server.ecommerceClient, authPayload.UserID, uriReq.ID, req.Reason, policy, time.Now())
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newReservationResponse(result.Reservation))
}

// markNoShow 标记未到店 (商户)
// @Summary 标记未到店
// @Description 商户标记已支付/已确认的预定为未到店，定金/预付款将被没收
// @Tags 预定管理-商户
// @Accept json
// @Produce json
// @Param id path int64 true "预定ID"
// @Success 200 {object} reservationResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "非商户或预定不属于该商户"
// @Failure 404 {object} ErrorResponse "预定不存在"
// @Failure 409 {object} ErrorResponse "预定状态不允许标记为未到店"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /v1/reservations/{id}/no-show [post]
func (server *Server) markNoShow(ctx *gin.Context) {
	var req getReservationRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 验证商户权限
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	result, err := logic.MarkReservationNoShow(ctx, server.store, authPayload.UserID, req.ID)
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 注意：no_show 通常不退款，定金被没收

	ctx.JSON(http.StatusOK, newReservationResponse(result.Reservation))
}

// ==================== 预定统计 ====================

type getReservationStatsRequest struct {
	StartDate string `form:"start_date,omitempty"` // YYYY-MM-DD
	EndDate   string `form:"end_date,omitempty"`   // YYYY-MM-DD
}

// getReservationStats 获取预定统计 (商户)
// @Summary 获取预定统计
// @Description 商户查看各状态预定数量统计
// @Tags 预定管理-商户
// @Accept json
// @Produce json
// @Success 200 {object} reservationStatsResponse
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "非商户"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /v1/reservations/merchant/stats [get]
func (server *Server) getReservationStats(ctx *gin.Context) {
	var req getReservationStatsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 验证商户权限
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.getMerchantFromContextOrStore(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取统计
	stats, err := server.store.GetReservationStatsEnhanced(ctx, merchant.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, reservationStatsResponse{
		PendingCount:   stats.PendingCount,
		PaidCount:      stats.PaidCount,
		ConfirmedCount: stats.ConfirmedCount,
		CheckedInCount: stats.CheckedInCount,
		CompletedCount: stats.CompletedCount,
		CancelledCount: stats.CancelledCount,
		ExpiredCount:   stats.ExpiredCount,
		NoShowCount:    stats.NoShowCount,
	})
}

// ==================== 辅助函数 ====================

// addDishesToReservation 为预定追加菜品
// POST /v1/reservations/:id/add-dishes
// @Summary 为预定追加菜品
// @Description 用户可以为已确认的预定追加菜品
// @Tags 预定管理
// @Accept json
// @Produce json
// @Param id path int64 true "预定ID"
// @Param body body addDishesRequest true "追加菜品请求"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/reservations/{id}/add-dishes [post]
func (server *Server) addDishesToReservation(ctx *gin.Context) {
	var uriReq struct {
		ID int64 `uri:"id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req addDishesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	if len(req.Items) == 0 {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("at least one item is required")))
		return
	}

	// 获取当前用户
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	addItems := make([]logic.ReservationItemInput, len(req.Items))
	for i, item := range req.Items {
		addItems[i] = logic.ReservationItemInput{
			DishID:   item.DishID,
			ComboID:  item.ComboID,
			Quantity: item.Quantity,
		}
	}

	result, err := logic.AddReservationDishes(ctx, server.store, logic.AddReservationDishesInput{
		UserID:          authPayload.UserID,
		ReservationID:   uriReq.ID,
		Items:           addItems,
		Now:             time.Now(),
		EcommerceClient: server.ecommerceClient,
		ClientIP:        ctx.ClientIP(),
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if result.Payment != nil {
		server.scheduleTimeoutForPaymentOrder(ctx, *result.Payment)
		resp := addDishesPaymentResponse{
			Message:        "additional dishes added, payment required",
			PaymentOrderID: result.Payment.ID,
			Amount:         result.AddedAmount,
			ItemsCount:     len(req.Items),
		}
		if result.PayParams != nil {
			resp.PayParams = &miniProgramPayParams{
				TimeStamp: result.PayParams.TimeStamp,
				NonceStr:  result.PayParams.NonceStr,
				Package:   result.PayParams.Package,
				SignType:  result.PayParams.SignType,
				PaySign:   result.PayParams.PaySign,
			}
		}
		ctx.JSON(http.StatusOK, resp)
		return
	}

	ctx.JSON(http.StatusOK, addDishesSuccessResponse{
		Message:    "additional dishes added successfully",
		Amount:     result.AddedAmount,
		ItemsCount: len(req.Items),
		Note:       "payment will be settled on site",
	})
}

// modifyReservationDishes 预订改菜（差量合并/补单/退单）
// POST /v1/reservations/:id/modify-dishes
// @Summary 预订改菜
// @Description 用户修改预订菜品，支持差量补单或退款
// @Tags 预定管理
// @Accept json
// @Produce json
// @Param id path int64 true "预定ID"
// @Param body body modifyDishesRequest true "改菜请求"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/reservations/{id}/modify-dishes [post]
func (server *Server) modifyReservationDishes(ctx *gin.Context) {
	var uriReq struct {
		ID int64 `uri:"id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req modifyDishesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if len(req.Items) == 0 {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("at least one item is required")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	modifyItems := make([]logic.ReservationItemInput, len(req.Items))
	for i, item := range req.Items {
		modifyItems[i] = logic.ReservationItemInput{
			DishID:   item.DishID,
			ComboID:  item.ComboID,
			Quantity: item.Quantity,
		}
	}

	result, err := logic.ModifyReservationDishes(ctx, server.store, logic.ModifyReservationDishesInput{
		UserID:          authPayload.UserID,
		ReservationID:   uriReq.ID,
		Items:           modifyItems,
		Now:             time.Now(),
		EcommerceClient: server.ecommerceClient,
		ClientIP:        ctx.ClientIP(),
		TaskScheduler:   apiTaskScheduler{server: server},
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if result.Payment != nil {
		server.scheduleTimeoutForPaymentOrder(ctx, *result.Payment)
		resp := modifyDishesPaymentResponse{
			Message:        "reservation modified, payment required",
			PaymentOrderID: result.Payment.ID,
			Amount:         result.Delta,
			ItemsCount:     len(req.Items),
		}
		if result.PayParams != nil {
			resp.PayParams = &miniProgramPayParams{
				TimeStamp: result.PayParams.TimeStamp,
				NonceStr:  result.PayParams.NonceStr,
				Package:   result.PayParams.Package,
				SignType:  result.PayParams.SignType,
				PaySign:   result.PayParams.PaySign,
			}
		}
		ctx.JSON(http.StatusOK, resp)
		return
	}

	if result.RefundInitiated {
		ctx.JSON(http.StatusOK, modifyDishesRefundResponse{
			Message:      "reservation modified, refund initiated",
			RefundAmount: result.RefundAmount,
			ItemsCount:   len(req.Items),
		})
		return
	}

	ctx.JSON(http.StatusOK, modifyDishesSuccessResponse{
		Message:    "reservation modified successfully",
		ItemsCount: len(req.Items),
		Delta:      result.Delta,
	})
}

type addDishesRequest struct {
	Items []addDishItem `json:"items" binding:"required,min=1,max=50,dive"`
}

type modifyDishesRequest struct {
	Items []addDishItem `json:"items" binding:"required,min=1,max=50,dive"`
}

type addDishItem struct {
	DishID   *int64 `json:"dish_id,omitempty" binding:"omitempty,min=1"`
	ComboID  *int64 `json:"combo_id,omitempty" binding:"omitempty,min=1"`
	Quantity int16  `json:"quantity" binding:"required,min=1,max=99"`
}

// ==================== 商户代客预订 ====================

// 预订来源常量
const (
	ReservationSourceOnline   = "online"   // 线上预订
	ReservationSourcePhone    = "phone"    // 电话预订
	ReservationSourceWalkin   = "walkin"   // 现场预订
	ReservationSourceMerchant = "merchant" // 商户代订
)

type merchantCreateReservationRequest struct {
	TableID      int64  `json:"table_id" binding:"required,min=1"`
	Date         string `json:"date" binding:"required"` // YYYY-MM-DD
	Time         string `json:"time" binding:"required"` // HH:MM
	GuestCount   int16  `json:"guest_count" binding:"required,min=1,max=50"`
	ContactName  string `json:"contact_name" binding:"required,max=50"`
	ContactPhone string `json:"contact_phone" binding:"required,max=20"`
	Source       string `json:"source,omitempty" binding:"omitempty,oneof=phone walkin merchant"` // 默认 merchant
	Notes        string `json:"notes,omitempty" binding:"omitempty,max=500"`
}

// merchantCreateReservation 商户代客创建预订
// @Summary 商户代客创建预订
// @Description 商户为顾客创建预订（电话预订、现场预订等），无需支付，直接进入已确认状态
// @Tags 预定管理-商户
// @Accept json
// @Produce json
// @Param body body merchantCreateReservationRequest true "预订请求"
// @Success 200 {object} reservationResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "非商户"
// @Failure 404 {object} ErrorResponse "桌台不存在"
// @Failure 409 {object} ErrorResponse "时间段已被预定"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /v1/reservations/merchant/create [post]
func (server *Server) merchantCreateReservation(ctx *gin.Context) {
	var req merchantCreateReservationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 验证商户权限
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.getMerchantFromContextOrStore(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 解析日期和时间
	reservationDate, err := parseISODate(req.Date, "invalid date format, use YYYY-MM-DD")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	reservationTime, err := time.Parse("15:04", req.Time)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid time format, use HH:MM")))
		return
	}

	reservation, err := logic.MerchantCreateReservation(ctx, server.store, logic.MerchantCreateReservationInput{
		OperatorUserID:  authPayload.UserID,
		MerchantID:      merchant.ID,
		TableID:         req.TableID,
		ReservationDate: reservationDate,
		ReservationTime: reservationTime,
		GuestCount:      req.GuestCount,
		ContactName:     req.ContactName,
		ContactPhone:    req.ContactPhone,
		Source:          req.Source,
		Notes:           req.Notes,
		Now:             time.Now(),
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 创建未到店提醒任务
	if server.taskDistributor != nil {
		hours := reservation.ReservationTime.Microseconds / 1000000 / 3600
		minutes := (reservation.ReservationTime.Microseconds / 1000000 % 3600) / 60
		alertTime := time.Date(
			reservation.ReservationDate.Time.Year(), reservation.ReservationDate.Time.Month(), reservation.ReservationDate.Time.Day(),
			int(hours), int(minutes), 0, 0, time.Local,
		)

		_ = server.taskDistributor.DistributeTaskReservationNoShowAlert(
			ctx,
			&worker.PayloadReservationNoShowAlert{ReservationID: reservation.ID},
			asynq.ProcessAt(alertTime),
		)
	}

	ctx.JSON(http.StatusOK, newReservationResponse(reservation))
}

// ==================== 顾客到店签到 ====================

// checkInReservation 顾客到店签到
// @Summary 顾客到店签到
// @Description 顾客到店后自助签到，通知商户顾客已到
// @Tags 预定管理
// @Accept json
// @Produce json
// @Param id path int64 true "预定ID"
// @Success 200 {object} reservationResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "无权操作该预定"
// @Failure 404 {object} ErrorResponse "预定不存在"
// @Failure 409 {object} ErrorResponse "预定状态不允许签到"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /v1/reservations/{id}/checkin [post]
func (server *Server) checkInReservation(ctx *gin.Context) {
	var req getReservationRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	result, err := logic.CheckInReservation(ctx, server.store, authPayload.UserID, req.ID, time.Now(), ReservationCheckInEarlyMinutes, ReservationCheckInLateMinutes)
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 推送 WebSocket 通知给商户
	if server.wsHub != nil {
		server.wsHub.SendToMerchant(result.Reservation.MerchantID, websocket.Message{
			Type:      "reservation_checkin",
			Data:      []byte(fmt.Sprintf(`{"reservation_id":%d,"contact_name":"%s"}`, req.ID, result.Reservation.ContactName)),
			Timestamp: time.Now(),
		})
	}

	ctx.JSON(http.StatusOK, newReservationResponse(result.Reservation))
}

// ==================== 起菜通知 ====================

// startCookingReservation 通知厨房起菜
// @Summary 通知厨房起菜
// @Description 顾客或商户通知厨房开始制作预点菜品
// @Tags 预定管理
// @Accept json
// @Produce json
// @Param id path int64 true "预定ID"
// @Success 200 {object} reservationResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "无权操作该预定"
// @Failure 404 {object} ErrorResponse "预定不存在"
// @Failure 409 {object} ErrorResponse "预定状态不允许起菜"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /v1/reservations/{id}/start-cooking [post]
func (server *Server) startCookingReservation(ctx *gin.Context) {
	var req getReservationRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	result, err := logic.StartCookingReservation(ctx, server.store, authPayload.UserID, req.ID)
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 推送 WebSocket 通知给商户（厨房）
	if server.wsHub != nil {
		server.wsHub.SendToMerchant(result.Reservation.MerchantID, websocket.Message{
			Type:      "reservation_start_cooking",
			Data:      []byte(fmt.Sprintf(`{"reservation_id":%d,"contact_name":"%s"}`, req.ID, result.Reservation.ContactName)),
			Timestamp: time.Now(),
		})
	}

	ctx.JSON(http.StatusOK, newReservationResponse(result.Reservation))
}

// ==================== 商户修改预订 ====================

type updateReservationRequest struct {
	TableID      *int64  `json:"table_id,omitempty" binding:"omitempty,min=1"`
	Date         *string `json:"date,omitempty"` // YYYY-MM-DD
	Time         *string `json:"time,omitempty"` // HH:MM
	GuestCount   *int16  `json:"guest_count,omitempty" binding:"omitempty,min=1,max=50"`
	ContactName  *string `json:"contact_name,omitempty" binding:"omitempty,max=50"`
	ContactPhone *string `json:"contact_phone,omitempty" binding:"omitempty,max=20"`
	Notes        *string `json:"notes,omitempty" binding:"omitempty,max=500"`
}

// merchantUpdateReservation 商户修改预订
// @Summary 商户修改预订
// @Description 商户修改预订信息（桌台、时间、人数、联系方式等）
// @Tags 预定管理-商户
// @Accept json
// @Produce json
// @Param id path int64 true "预定ID"
// @Param body body updateReservationRequest true "修改请求"
// @Success 200 {object} reservationResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "非商户或预定不属于该商户"
// @Failure 404 {object} ErrorResponse "预定不存在"
// @Failure 409 {object} ErrorResponse "时间段已被预定"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /v1/reservations/{id}/update [put]
func (server *Server) merchantUpdateReservation(ctx *gin.Context) {
	var uriReq struct {
		ID int64 `uri:"id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req updateReservationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	var reservationDate *time.Time
	if req.Date != nil {
		date, err := parseISODate(*req.Date, "invalid date format")
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}
		reservationDate = &date
	}

	var reservationTime *time.Time
	if req.Time != nil {
		parsedTime, err := time.Parse("15:04", *req.Time)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid time format")))
			return
		}
		reservationTime = &parsedTime
	}

	updatedReservation, err := logic.MerchantUpdateReservation(ctx, server.store, logic.MerchantUpdateReservationInput{
		OperatorUserID:  authPayload.UserID,
		ReservationID:   uriReq.ID,
		TableID:         req.TableID,
		ReservationDate: reservationDate,
		ReservationTime: reservationTime,
		GuestCount:      req.GuestCount,
		ContactName:     req.ContactName,
		ContactPhone:    req.ContactPhone,
		Notes:           req.Notes,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newReservationResponse(updatedReservation))
}

// ==================== 获取今日预订 ====================

// listTodayReservations 获取今日预订列表
// @Summary 获取今日预订列表
// @Description 商户获取今日有效预订列表（已支付/已确认/已签到）
// @Tags 预定管理-商户
// @Accept json
// @Produce json
// @Success 200 {object} object{reservations=[]reservationResponse}
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "非商户"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /v1/reservations/merchant/today [get]
func (server *Server) listTodayReservations(ctx *gin.Context) {
	// 验证商户权限
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.getMerchantFromContextOrStore(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	reservations, err := server.store.ListTodayReservationsByMerchant(ctx, merchant.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := make([]reservationResponse, len(reservations))
	for i, r := range reservations {
		resp[i] = reservationResponse{
			ID:              r.ID,
			TableID:         r.TableID,
			TableNo:         r.TableNo,
			TableType:       r.TableType,
			UserID:          r.UserID,
			MerchantID:      r.MerchantID,
			ReservationDate: r.ReservationDate.Time.Format("2006-01-02"),
			GuestCount:      r.GuestCount,
			ContactName:     r.ContactName,
			ContactPhone:    r.ContactPhone,
			PaymentMode:     r.PaymentMode,
			DepositAmount:   r.DepositAmount,
			PrepaidAmount:   r.PrepaidAmount,
			Status:          r.Status,
			CreatedAt:       r.CreatedAt,
		}
		if r.ReservationTime.Valid {
			hours := r.ReservationTime.Microseconds / 1000000 / 3600
			minutes := (r.ReservationTime.Microseconds / 1000000 % 3600) / 60
			resp[i].ReservationTime = time.Date(0, 1, 1, int(hours), int(minutes), 0, 0, time.UTC).Format("15:04")
		}

		// 填充预订菜品
		items, err := server.store.ListReservationItems(ctx, r.ID)
		if err == nil && len(items) > 0 {
			resp[i].Items = make([]reservationItemResponse, len(items))
			for j, item := range items {
				resp[i].Items[j] = reservationItemResponse{
					ID:         item.ID,
					Quantity:   item.Quantity,
					UnitPrice:  item.UnitPrice,
					TotalPrice: int64(item.Quantity) * item.UnitPrice,
					Type:       "dish",
				}

				if item.DishID.Valid {
					resp[i].Items[j].DishID = &item.DishID.Int64
					if item.DishName.Valid {
						resp[i].Items[j].Name = item.DishName.String
					}
					if item.DishImageMediaAssetID.Valid {
						resp[i].Items[j].ImageAssetID = &item.DishImageMediaAssetID.Int64
					}
				} else if item.ComboID.Valid {
					resp[i].Items[j].ComboID = &item.ComboID.Int64
					resp[i].Items[j].Type = "combo"
					if item.ComboName.Valid {
						resp[i].Items[j].Name = item.ComboName.String
					}
					if item.ComboImageMediaAssetID.Valid {
						resp[i].Items[j].ImageAssetID = &item.ComboImageMediaAssetID.Int64
					}
				}
			}
		}
	}

	ctx.JSON(http.StatusOK, reservationSliceResponse{Reservations: resp})
}
