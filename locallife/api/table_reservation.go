package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/rules"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/websocket"
	"github.com/merrydance/locallife/wechat"
	"github.com/merrydance/locallife/worker"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
)

// ==================== 预定管理 ====================

// 预定状态常量

const (
	reservationActionConfirm      = "confirm"
	reservationActionComplete     = "complete"
	reservationActionCancel       = "cancel"
	reservationActionNoShow       = "no_show"
	reservationActionCheckIn      = "check_in"
	reservationActionStartCooking = "start_cooking"
)

func isReservationStatusAllowed(status string, action string) bool {
	switch action {
	case reservationActionConfirm:
		return status == ReservationStatusPaid
	case reservationActionComplete:
		return status == ReservationStatusConfirmed || status == ReservationStatusCheckedIn
	case reservationActionCancel:
		return status == ReservationStatusPending || status == ReservationStatusPaid || status == ReservationStatusConfirmed
	case reservationActionNoShow:
		return status == ReservationStatusPaid || status == ReservationStatusConfirmed
	case reservationActionCheckIn:
		return status == ReservationStatusPaid || status == ReservationStatusConfirmed
	case reservationActionStartCooking:
		return status == ReservationStatusConfirmed || status == ReservationStatusCheckedIn
	default:
		return false
	}
}

// 支付模式常量
const (
	PaymentModeDeposit = "deposit" // 定金模式，到店点菜
	PaymentModeFull    = "full"    // 全款模式，在线点菜
)

// 超时和退款配置
const (
	PaymentTimeoutMinutes    = 30    // 支付超时时间：30分钟
	RefundDeadlineHours      = 2     // 退款截止：预定时间前2小时
	DefaultDepositAmount     = 10000 // 默认定金：100元（分）
	ReservationDurationHours = 4     // 用餐时段时长：4小时（用于时间段冲突检测）
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
	ID         int64  `json:"id"`
	DishID     *int64 `json:"dish_id,omitempty"`
	ComboID    *int64 `json:"combo_id,omitempty"`
	Name       string `json:"name,omitempty"`
	ImageURL   string `json:"image_url,omitempty"`
	Quantity   int16  `json:"quantity"`
	UnitPrice  int64  `json:"unit_price"`
	TotalPrice int64  `json:"total_price"`
	Type       string `json:"type,omitempty"`
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
		if item.DishImageUrl.Valid {
			mapped.ImageURL = normalizeUploadURLForClient(item.DishImageUrl.String)
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

	// 检查预定时间是否在未来
	now := time.Now()
	reservationDateTime := time.Date(
		reservationDate.Year(), reservationDate.Month(), reservationDate.Day(),
		reservationTime.Hour(), reservationTime.Minute(), 0, 0, time.Local,
	)
	if reservationDateTime.Before(now) {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("所选时段已过，请选择其他时间")))
		return
	}

	// 获取桌台信息
	table, err := server.store.GetTable(ctx, req.TableID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("table not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 只有包间可以预定
	if table.TableType != "room" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("只有包间可以预订")))
		return
	}

	// 检查桌台状态
	if table.Status == "disabled" {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("table is disabled and cannot be reserved")))
		return
	}

	// 检查人数是否超过容量
	if req.GuestCount > table.Capacity {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("guest count exceeds table capacity")))
		return
	}

	conflict, err := server.checkReservationConflict(ctx, req.TableID, table.MerchantID, reservationDate, reservationTime, 0)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if conflict {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("该时间段已被预订，请选择其他时间")))
		return
	}

	pgDate := pgtype.Date{Time: reservationDate, Valid: true}
	pgTime := pgtype.Time{
		Microseconds: int64(reservationTime.Hour()*3600+reservationTime.Minute()*60) * 1000000,
		Valid:        true,
	}

	// 计算金额和验证菜品
	var depositAmount, prepaidAmount int64
	var validatedItems []validatedReservationItem

	if req.PaymentMode == PaymentModeDeposit {
		// 定金模式：使用包间最低消费作为定金，如果没有则使用默认定金
		if table.MinimumSpend.Valid && table.MinimumSpend.Int64 > 0 {
			depositAmount = table.MinimumSpend.Int64
		} else {
			depositAmount = DefaultDepositAmount
		}
	} else {
		// 全款模式：如果预点了菜品，则验证菜品和金额
		if len(req.Items) > 0 {
			// 验证并计算菜品金额（同时获取菜品单价）
			validatedItems, prepaidAmount, err = server.validateReservationItems(ctx, table.MerchantID, req.Items)
			if err != nil {
				ctx.JSON(http.StatusBadRequest, errorResponse(err))
				return
			}

			// 全款模式如果预点了菜品，至少要达到包间最低消费
			if table.MinimumSpend.Valid && prepaidAmount < table.MinimumSpend.Int64 {
				ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("预点菜品金额 %d 分未达到包间最低消费 %d 分", prepaidAmount, table.MinimumSpend.Int64)))
				return
			}
		}
		// 如果没有预点菜品，允许创建预订（prepaidAmount 为 0），用户将在 30 分钟内去点菜支付
	}

	// 计算支付截止时间和退款截止时间
	paymentDeadline := now.Add(PaymentTimeoutMinutes * time.Minute)
	refundDeadline := reservationDateTime.Add(-RefundDeadlineHours * time.Hour)

	// 准备创建预定参数
	arg := db.CreateTableReservationParams{
		TableID:         req.TableID,
		UserID:          authPayload.UserID,
		MerchantID:      table.MerchantID,
		ReservationDate: pgDate,
		ReservationTime: pgTime,
		GuestCount:      req.GuestCount,
		ContactName:     req.ContactName,
		ContactPhone:    req.ContactPhone,
		PaymentMode:     req.PaymentMode,
		DepositAmount:   depositAmount,
		PrepaidAmount:   prepaidAmount,
		RefundDeadline:  refundDeadline,
		PaymentDeadline: paymentDeadline,
		Status:          ReservationStatusPending,
	}

	if req.Notes != "" {
		arg.Notes = pgtype.Text{String: req.Notes, Valid: true}
	}

	// 构建事务参数（包含菜品项）
	txArg := db.CreateReservationTxParams{
		CreateTableReservationParams: arg,
		AfterLock: func(ctx context.Context, q *db.Queries) error {
			// P0-002 修复：在锁内进行二次冲突校验
			// 1. 获取已有预订
			existingReservations, err := q.ListReservationsByTableAndDate(ctx, db.ListReservationsByTableAndDateParams{
				TableID:         req.TableID,
				ReservationDate: pgDate,
			})
			if err != nil {
				return fmt.Errorf("failed to list existing reservations: %w", err)
			}

			// 2. 获取商户营业时间配置 (用于冲突判断规则)
			businessHours, err := q.ListMerchantBusinessHours(ctx, table.MerchantID)
			config := util.DefaultConfig
			if err == nil {
				dayOfWeek := int32(reservationDate.Weekday())
				var todayHours []db.MerchantBusinessHour
				for _, bh := range businessHours {
					if bh.SpecialDate.Valid && bh.SpecialDate.Time.Format("2006-01-02") == reservationDate.Format("2006-01-02") {
						todayHours = append(todayHours, bh)
					}
				}
				if len(todayHours) == 0 {
					for _, bh := range businessHours {
						if !bh.SpecialDate.Valid && bh.DayOfWeek == dayOfWeek {
							todayHours = append(todayHours, bh)
						}
					}
				}

				if len(todayHours) > 0 {
					h1 := todayHours[0]
					config.LunchStart = int(h1.OpenTime.Microseconds/1000000/3600*100) + int(h1.OpenTime.Microseconds/1000000%3600/60)
					config.LunchEnd = int(h1.CloseTime.Microseconds/1000000/3600*100) + int(h1.CloseTime.Microseconds/1000000%3600/60)
					config.DinnerStart = 0
					config.DinnerEnd = 0

					if len(todayHours) > 1 {
						h2 := todayHours[1]
						config.DinnerStart = int(h2.OpenTime.Microseconds/1000000/3600*100) + int(h2.OpenTime.Microseconds/1000000%3600/60)
						config.DinnerEnd = int(h2.CloseTime.Microseconds/1000000/3600*100) + int(h2.CloseTime.Microseconds/1000000%3600/60)
					} else if config.LunchStart >= 1500 {
						config.DinnerStart = config.LunchStart
						config.DinnerEnd = config.LunchEnd
						config.LunchStart = 0
						config.LunchEnd = 0
					}
				}
			}

			// 3. 检查冲突
			newDateTime := time.Date(reservationDate.Year(), reservationDate.Month(), reservationDate.Day(), reservationTime.Hour(), reservationTime.Minute(), 0, 0, reservationDate.Location())

			for _, r := range existingReservations {
				if r.Status == ReservationStatusCancelled || r.Status == ReservationStatusExpired || r.Status == ReservationStatusNoShow {
					continue
				}
				if !r.ReservationTime.Valid {
					continue
				}

				existingTime := util.CombineDateAndTime(r.ReservationDate.Time, r.ReservationTime.Microseconds)

				if util.AreReservationsConflictingWithConfig(newDateTime, existingTime, config) {
					return fmt.Errorf("该时间段刚刚被抢订，请选择其他时间")
				}
			}
			return nil
		},
	}

	// 全款模式添加菜品明细
	if len(validatedItems) > 0 {
		txArg.Items = make([]db.ReservationItemInput, len(validatedItems))
		for i, item := range validatedItems {
			txArg.Items[i] = db.ReservationItemInput{
				DishID:    item.DishID,
				ComboID:   item.ComboID,
				Quantity:  item.Quantity,
				UnitPrice: item.UnitPrice,
			}
		}
	}

	// 使用事务创建预定和菜品明细
	result, err := server.store.CreateReservationTx(ctx, txArg)
	if err != nil {
		if strings.Contains(err.Error(), "刚被抢订") { // Handle custom error
			ctx.JSON(http.StatusConflict, errorResponse(errors.New("该时间段刚刚被抢订，请选择其他时间")))
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
			asynq.ProcessAt(paymentDeadline),
		)
		if err != nil {
			// 任务分发失败不影响主流程，记录日志
			// 可以通过定时任务轮询处理超时预定作为兜底
		}
	}

	ctx.JSON(http.StatusOK, newReservationResponse(result.Reservation))
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
// @Success 200 {object} reservationResponse
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
		merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
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

	ctx.JSON(http.StatusOK, gin.H{
		"reservations": resp,
		"total":        totalCount,
		"total_count":  totalCount,
		"page_id":      req.PageID,
		"page_size":    req.PageSize,
	})
}

type listMerchantReservationsRequest struct {
	Date     string `form:"date,omitempty"`                                                                                        // YYYY-MM-DD
	Status   string `form:"status,omitempty" binding:"omitempty,oneof=pending paid confirmed completed cancelled expired no_show"` // 状态筛选
	PageID   int32  `form:"page_id" binding:"required,min=1"`
	PageSize int32  `form:"page_size" binding:"required,min=5,max=200"`
}

// listMerchantReservations 商户查看预定列表
// @Summary 获取商户预定列表
// @Description 商户查看自己店铺的所有预定记录，支持按日期或状态筛选
// @Tags 预定管理-商户
// @Accept json
// @Produce json
// @Param date query string false "筛选日期 (YYYY-MM-DD)"
// @Param status query string false "筛选状态" Enums(pending, paid, confirmed, completed, cancelled, expired, no_show)
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(5) maximum(50)
// @Success 200 {object} object{reservations=[]reservationResponse}
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
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
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
		// 转换类型
		for _, r := range dateReservations {
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
	} else if req.Status != "" {
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
					if item.DishImageUrl.Valid {
						resp[i].Items[j].ImageURL = normalizeUploadURLForClient(item.DishImageUrl.String)
					}
				} else if item.ComboID.Valid {
					resp[i].Items[j].ComboID = &item.ComboID.Int64
					resp[i].Items[j].Type = "combo"
					if item.ComboName.Valid {
						resp[i].Items[j].Name = item.ComboName.String
					}
					if item.ComboImageUrl.Valid {
						resp[i].Items[j].ImageURL = normalizeUploadURLForClient(item.ComboImageUrl.String)
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

	ctx.JSON(http.StatusOK, gin.H{
		"reservations": resp,
		"total":        totalCount,
		"total_count":  totalCount,
		"page_id":      req.PageID,
		"page_size":    req.PageSize,
	})
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
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取预定并锁定
	reservation, err := server.store.GetTableReservationForUpdate(ctx, req.ID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("reservation not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证所有权
	if reservation.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("reservation does not belong to your merchant")))
		return
	}

	// 检查状态
	if !isReservationStatusAllowed(reservation.Status, reservationActionConfirm) {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("reservation is not paid")))
		return
	}

	// 使用事务更新预定状态和桌台状态
	result, err := server.store.ConfirmReservationTx(ctx, db.ConfirmReservationTxParams{
		ReservationID: req.ID,
		TableID:       reservation.TableID,
	})
	if err != nil {
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
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取预定并锁定
	reservation, err := server.store.GetTableReservationForUpdate(ctx, req.ID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("reservation not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证所有权
	if reservation.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("reservation does not belong to your merchant")))
		return
	}

	// 检查状态
	if !isReservationStatusAllowed(reservation.Status, reservationActionComplete) {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("reservation is not confirmed or checked in")))
		return
	}

	// 获取桌台当前预定ID
	table, err := server.store.GetTable(ctx, reservation.TableID)
	var currentReservationID pgtype.Int8
	if err == nil {
		currentReservationID = table.CurrentReservationID
	}

	// 使用事务更新预定状态和释放桌台
	result, err := server.store.CompleteReservationTx(ctx, db.CompleteReservationTxParams{
		ReservationID:        req.ID,
		TableID:              reservation.TableID,
		CurrentReservationID: currentReservationID,
	})
	if err != nil {
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
// @Description 用户取消自己的预定，或商户取消店铺的预定。已支付的预定在退款截止时间前可全额退款。
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

	// 获取预定并锁定
	reservation, err := server.store.GetTableReservationForUpdate(ctx, uriReq.ID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("reservation not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证权限：用户或商户
	isOwner := reservation.UserID == authPayload.UserID
	isMerchant := false

	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err == nil && merchant.ID == reservation.MerchantID {
		isMerchant = true
	}

	if !isOwner && !isMerchant {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not authorized to cancel this reservation")))
		return
	}

	// 检查状态：只有 pending, paid, confirmed 可以取消
	if !isReservationStatusAllowed(reservation.Status, reservationActionCancel) {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("预约状态不允许取消")))
		return
	}

	// 用户取消时检查退款截止时间
	if isOwner && !isMerchant {
		if reservation.Status == ReservationStatusPaid || reservation.Status == ReservationStatusConfirmed {
			if time.Now().After(reservation.RefundDeadline) {
				ctx.JSON(http.StatusConflict, errorResponse(errors.New("refund deadline passed, please contact merchant")))
				return
			}
		}
	}

	// 获取桌台信息以判断是否需要释放
	table, err := server.store.GetTable(ctx, reservation.TableID)
	var currentReservationID pgtype.Int8
	if err == nil {
		currentReservationID = table.CurrentReservationID
	}

	// P1-029 修复：使用事务执行取消操作，同时释放库存（原子性保证）
	result, err := server.store.CancelReservationTx(ctx, db.CancelReservationTxParams{
		ReservationID:        uriReq.ID,
		TableID:              reservation.TableID,
		CancelReason:         req.Reason,
		CurrentReservationID: currentReservationID,
		ReleaseInventory:     true, // P1-029: 在同一事务中释放库存
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 处理退款逻辑（退款逻辑在事务外执行，因为涉及外部 API 调用）
	// 取消预定时需要根据以下条件处理退款：
	// 1. 状态为 pending：无需退款（未支付）
	// 2. 状态为 paid/confirmed 且在 refund_deadline 之前：全额退款
	// 3. 状态为 paid/confirmed 且在 refund_deadline 之后：根据商户政策处理（可能部分退款或不退）
	if reservation.Status == ReservationStatusPaid || reservation.Status == ReservationStatusConfirmed {
		// 查找该预定的支付订单
		paymentOrder, err := server.store.GetLatestPaymentOrderByReservation(ctx, db.GetLatestPaymentOrderByReservationParams{
			ReservationID: pgtype.Int8{Int64: reservation.ID, Valid: true},
			BusinessType:  BusinessTypeReservation,
		})
		if err == nil && paymentOrder.Status == PaymentStatusPaid {
			// 计算退款金额
			var refundAmount int64
			if time.Now().Before(reservation.RefundDeadline) {
				// 退款截止前，全额退款
				refundAmount = paymentOrder.Amount
			} else {
				// 退款截止后，根据商户政策处理（这里暂不退款，实际可配置）
				refundAmount = 0
			}

			if refundAmount > 0 && server.paymentClient != nil {
				// 生成退款单号
				outRefundNo := generateOutRefundNo()

				refundType := paymentOrder.PaymentType
				if refundType == PaymentTypeNative {
					refundType = PaymentTypeMiniProgram
				}

				// 创建退款订单
				refundOrder, err := server.store.CreateRefundOrder(ctx, db.CreateRefundOrderParams{
					PaymentOrderID: paymentOrder.ID,
					RefundType:     refundType,
					RefundAmount:   refundAmount,
					RefundReason:   pgtype.Text{String: "预定取消退款", Valid: true},
					OutRefundNo:    outRefundNo,
					Status:         "pending",
				})
				if err == nil {
					// 调用微信退款 API
					wxRefund, err := server.paymentClient.CreateRefund(ctx, &wechat.RefundRequest{
						OutTradeNo:   paymentOrder.OutTradeNo,
						OutRefundNo:  outRefundNo,
						Reason:       "预定取消退款",
						RefundAmount: refundAmount,
						TotalAmount:  paymentOrder.Amount,
					})
					if err != nil {
						server.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID)
					} else if wxRefund.Status == wechat.RefundStatusSuccess {
						server.store.UpdateRefundOrderToSuccess(ctx, refundOrder.ID)
						server.store.UpdatePaymentOrderToRefunded(ctx, paymentOrder.ID)
					} else if wxRefund.Status == wechat.RefundStatusProcessing {
						server.store.UpdateRefundOrderToProcessing(ctx, db.UpdateRefundOrderToProcessingParams{
							ID:       refundOrder.ID,
							RefundID: pgtype.Text{String: wxRefund.RefundID, Valid: true},
						})
					}
				}
			}
		}
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
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取预定并锁定
	reservation, err := server.store.GetTableReservationForUpdate(ctx, req.ID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("reservation not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证所有权
	if reservation.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("reservation does not belong to your merchant")))
		return
	}

	// 检查状态：只有 paid 或 confirmed 可以标记为未到店
	if !isReservationStatusAllowed(reservation.Status, reservationActionNoShow) {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("only paid or confirmed reservations can be marked as no-show")))
		return
	}

	// 获取桌台信息以判断是否需要释放
	table, err := server.store.GetTable(ctx, reservation.TableID)
	var currentReservationID pgtype.Int8
	if err == nil {
		currentReservationID = table.CurrentReservationID
	}

	// 使用事务执行标记未到店操作：更新预定状态 + 释放桌台
	result, err := server.store.MarkNoShowTx(ctx, db.MarkNoShowTxParams{
		ReservationID:        req.ID,
		TableID:              reservation.TableID,
		CurrentReservationID: currentReservationID,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if err := server.store.ReleaseReservationInventoryTx(ctx, db.ReleaseReservationInventoryTxParams{
		ReservationID: reservation.ID,
	}); err != nil {
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
// @Success 200 {object} object{pending_count=int64,paid_count=int64,confirmed_count=int64,completed_count=int64,cancelled_count=int64,expired_count=int64,no_show_count=int64}
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
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
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

	ctx.JSON(http.StatusOK, gin.H{
		"pending_count":    stats.PendingCount,
		"paid_count":       stats.PaidCount,
		"confirmed_count":  stats.ConfirmedCount,
		"checked_in_count": stats.CheckedInCount,
		"completed_count":  stats.CompletedCount,
		"cancelled_count":  stats.CancelledCount,
		"expired_count":    stats.ExpiredCount,
		"no_show_count":    stats.NoShowCount,
	})
}

// ==================== 辅助函数 ====================

// validatedReservationItem 验证后的预定菜品项（包含单价）
type validatedReservationItem struct {
	DishID    *int64
	ComboID   *int64
	Quantity  int16
	UnitPrice int64
}

// validateReservationItems 验证预定菜品并返回带单价的菜品列表
// 返回: 菜品列表、总金额、错误
func (server *Server) validateReservationItems(ctx *gin.Context, merchantID int64, items []reservationItem) ([]validatedReservationItem, int64, error) {
	var total int64 = 0
	validatedItems := make([]validatedReservationItem, 0, len(items))

	for _, item := range items {
		if item.DishID == nil && item.ComboID == nil {
			return nil, 0, errors.New("每个菜品项必须指定 dish_id 或 combo_id")
		}
		if item.DishID != nil && item.ComboID != nil {
			return nil, 0, errors.New("每个菜品项只能指定 dish_id 或 combo_id 之一")
		}

		var unitPrice int64

		if item.DishID != nil {
			// 查询菜品
			dish, err := server.store.GetDish(ctx, *item.DishID)
			if err != nil {
				if isNotFoundError(err) {
					return nil, 0, fmt.Errorf("菜品 %d 不存在", *item.DishID)
				}
				return nil, 0, err
			}
			// 验证菜品属于该商户
			if dish.MerchantID != merchantID {
				return nil, 0, fmt.Errorf("菜品 %s 不属于该商户", dish.Name)
			}
			// 验证菜品上架
			if !dish.IsOnline {
				return nil, 0, fmt.Errorf("菜品 %s 已下架", dish.Name)
			}
			unitPrice = dish.Price
		} else if item.ComboID != nil {
			// 查询套餐
			combo, err := server.store.GetComboSet(ctx, *item.ComboID)
			if err != nil {
				if isNotFoundError(err) {
					return nil, 0, fmt.Errorf("套餐 %d 不存在", *item.ComboID)
				}
				return nil, 0, err
			}
			// 验证套餐属于该商户
			if combo.MerchantID != merchantID {
				return nil, 0, fmt.Errorf("套餐 %s 不属于该商户", combo.Name)
			}
			// 验证套餐上架
			if !combo.IsOnline {
				return nil, 0, fmt.Errorf("套餐 %s 已下架", combo.Name)
			}
			unitPrice = combo.ComboPrice
		}

		validatedItems = append(validatedItems, validatedReservationItem{
			DishID:    item.DishID,
			ComboID:   item.ComboID,
			Quantity:  item.Quantity,
			UnitPrice: unitPrice,
		})
		total += unitPrice * int64(item.Quantity)
	}

	return validatedItems, total, nil
}

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

	// 获取预定信息
	reservation, err := server.store.GetTableReservationForUpdate(ctx, uriReq.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("reservation not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证权限（只有预定用户可以加菜）
	if reservation.UserID != authPayload.UserID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("you can only add dishes to your own reservation")))
		return
	}

	// 验证预定状态（只有已支付或已确认的预定可以加菜）
	if reservation.Status != ReservationStatusPaid && reservation.Status != ReservationStatusConfirmed {
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("cannot add dishes to reservation in %s status", reservation.Status)))
		return
	}

	// 转换请求项以复用验证逻辑
	addItems := make([]reservationItem, len(req.Items))
	for i, item := range req.Items {
		addItems[i] = reservationItem{
			DishID:   item.DishID,
			ComboID:  item.ComboID,
			Quantity: item.Quantity,
		}
	}

	// 验证并计算追加菜品金额（带单价）
	validatedItems, addedAmount, err := server.validateReservationItems(ctx, reservation.MerchantID, addItems)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 记录追加菜品到 reservation_items，便于详情页展示
	for _, item := range validatedItems {
		var dishID, comboID pgtype.Int8
		if item.DishID != nil {
			dishID = pgtype.Int8{Int64: *item.DishID, Valid: true}
		}
		if item.ComboID != nil {
			comboID = pgtype.Int8{Int64: *item.ComboID, Valid: true}
		}

		_, err := server.store.CreateReservationItem(ctx, db.CreateReservationItemParams{
			ReservationID: reservation.ID,
			DishID:        dishID,
			ComboID:       comboID,
			Quantity:      item.Quantity,
			UnitPrice:     item.UnitPrice,
			TotalPrice:    item.UnitPrice * int64(item.Quantity),
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}

	// 如果是全款预付模式，需要创建补差价支付订单
	if reservation.PaymentMode == PaymentModeFull {
		// 创建支付订单（out_trade_no 碰撞重试）
		var paymentOrder db.PaymentOrder
		var outTradeNo string
		expiresAt := time.Now().Add(30 * time.Minute)
		for attempt := 1; attempt <= outTradeNoMaxRetry; attempt++ {
			outTradeNo = generateOutTradeNoWithPrefix("RA")
			paymentOrder, err = server.store.CreatePaymentOrder(ctx, db.CreatePaymentOrderParams{
				UserID:        authPayload.UserID,
				ReservationID: pgtype.Int8{Int64: reservation.ID, Valid: true},
				PaymentType:   PaymentTypeMiniProgram,
				BusinessType:  BusinessTypeReservationAddon,
				Amount:        addedAmount,
				OutTradeNo:    outTradeNo,
				ExpiresAt:     pgtype.Timestamptz{Time: expiresAt, Valid: true},
			})
			if err == nil {
				break
			}
			if isOutTradeNoConflict(err) && attempt < outTradeNoMaxRetry {
				if !sleepWithContext(ctx.Request.Context(), outTradeNoRetryBaseBack*time.Duration(attempt)) {
					ctx.JSON(http.StatusRequestTimeout, errorResponse(errors.New("request canceled")))
					return
				}
				continue
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		ctx.JSON(http.StatusOK, gin.H{
			"message":          "additional dishes added, payment required",
			"payment_order_id": paymentOrder.ID,
			"amount":           addedAmount,
			"items_count":      len(req.Items),
		})
		return
	}

	// 定金模式：直接记录追加菜品，到店结算
	ctx.JSON(http.StatusOK, gin.H{
		"message":     "additional dishes added successfully",
		"amount":      addedAmount,
		"items_count": len(req.Items),
		"note":        "payment will be settled on site",
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

	reservation, err := server.store.GetTableReservationForUpdate(ctx, uriReq.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("reservation not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if reservation.UserID != authPayload.UserID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("you can only modify your own reservation")))
		return
	}

	if reservation.Status != ReservationStatusPaid && reservation.Status != ReservationStatusConfirmed && reservation.Status != ReservationStatusCheckedIn {
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("cannot modify reservation in %s status", reservation.Status)))
		return
	}

	if reservation.CookingStartedAt.Valid {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("cooking already started, modification is not allowed")))
		return
	}

	currentTotal, err := server.store.SumReservationItemsTotal(ctx, reservation.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	modifyItems := make([]reservationItem, len(req.Items))
	for i, item := range req.Items {
		modifyItems[i] = reservationItem{
			DishID:   item.DishID,
			ComboID:  item.ComboID,
			Quantity: item.Quantity,
		}
	}

	validatedItems, newTotal, err := server.validateReservationItems(ctx, reservation.MerchantID, modifyItems)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	delta := newTotal - currentTotal

	createItems := make([]db.CreateReservationItemParams, 0, len(validatedItems))
	for _, item := range validatedItems {
		var dishID, comboID pgtype.Int8
		if item.DishID != nil {
			dishID = pgtype.Int8{Int64: *item.DishID, Valid: true}
		}
		if item.ComboID != nil {
			comboID = pgtype.Int8{Int64: *item.ComboID, Valid: true}
		}
		createItems = append(createItems, db.CreateReservationItemParams{
			ReservationID: reservation.ID,
			DishID:        dishID,
			ComboID:       comboID,
			Quantity:      item.Quantity,
			UnitPrice:     item.UnitPrice,
			TotalPrice:    item.UnitPrice * int64(item.Quantity),
		})
	}

	if _, err := server.store.ReplaceReservationItemsTx(ctx, db.ReplaceReservationItemsTxParams{
		ReservationID: reservation.ID,
		Items:         createItems,
	}); err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if reservation.PaymentMode == PaymentModeFull && delta != 0 {
		if delta > 0 {
			var paymentOrder db.PaymentOrder
			var outTradeNo string
			expiresAt := time.Now().Add(30 * time.Minute)
			for attempt := 1; attempt <= outTradeNoMaxRetry; attempt++ {
				outTradeNo = generateOutTradeNoWithPrefix("RA")
				paymentOrder, err = server.store.CreatePaymentOrder(ctx, db.CreatePaymentOrderParams{
					UserID:        authPayload.UserID,
					ReservationID: pgtype.Int8{Int64: reservation.ID, Valid: true},
					PaymentType:   PaymentTypeMiniProgram,
					BusinessType:  BusinessTypeReservationAddon,
					Amount:        delta,
					OutTradeNo:    outTradeNo,
					ExpiresAt:     pgtype.Timestamptz{Time: expiresAt, Valid: true},
				})
				if err == nil {
					break
				}
				if isOutTradeNoConflict(err) && attempt < outTradeNoMaxRetry {
					if !sleepWithContext(ctx.Request.Context(), outTradeNoRetryBaseBack*time.Duration(attempt)) {
						ctx.JSON(http.StatusRequestTimeout, errorResponse(errors.New("request canceled")))
						return
					}
					continue
				}
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
				return
			}

			ctx.JSON(http.StatusOK, gin.H{
				"message":          "reservation modified, payment required",
				"payment_order_id": paymentOrder.ID,
				"amount":           delta,
				"items_count":      len(req.Items),
			})
			return
		}

		refundAmount := -delta
		if refundAmount > reservation.PrepaidAmount {
			refundAmount = reservation.PrepaidAmount
		}
		if refundAmount > 0 && server.paymentClient != nil {
			paymentOrder, err := server.store.GetLatestPaymentOrderByReservation(ctx, db.GetLatestPaymentOrderByReservationParams{
				ReservationID: pgtype.Int8{Int64: reservation.ID, Valid: true},
				BusinessType:  BusinessTypeReservation,
			})
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
				return
			}
			if paymentOrder.Status == PaymentStatusPaid {
				outRefundNo := generateOutRefundNo()
				refundOrder, err := server.store.CreateRefundOrder(ctx, db.CreateRefundOrderParams{
					PaymentOrderID: paymentOrder.ID,
					RefundType:     "partial",
					RefundAmount:   refundAmount,
					RefundReason:   pgtype.Text{String: "预定改菜退款", Valid: true},
					OutRefundNo:    outRefundNo,
					Status:         "pending",
				})
				if err != nil {
					ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
					return
				}

				wxRefund, err := server.paymentClient.CreateRefund(ctx, &wechat.RefundRequest{
					OutTradeNo:   paymentOrder.OutTradeNo,
					OutRefundNo:  outRefundNo,
					Reason:       "预定改菜退款",
					RefundAmount: refundAmount,
					TotalAmount:  paymentOrder.Amount,
				})
				if err != nil {
					server.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID)
					ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
					return
				}
				if wxRefund.Status == wechat.RefundStatusSuccess {
					server.store.UpdateRefundOrderToSuccess(ctx, refundOrder.ID)
					_, _ = server.store.AddReservationPrepaidAmount(ctx, db.AddReservationPrepaidAmountParams{
						ID:            reservation.ID,
						PrepaidAmount: -refundAmount,
					})
				}
			}

			ctx.JSON(http.StatusOK, gin.H{
				"message":       "reservation modified, refund initiated",
				"refund_amount": refundAmount,
				"items_count":   len(req.Items),
			})
			return
		}
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message":     "reservation modified successfully",
		"items_count": len(req.Items),
		"delta":       delta,
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
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
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

	// 获取桌台信息
	table, err := server.store.GetTable(ctx, req.TableID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("table not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证桌台属于该商户
	if table.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("table does not belong to your merchant")))
		return
	}

	// 检查桌台状态
	if table.Status == "disabled" {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("table is disabled and cannot be reserved")))
		return
	}

	// 检查人数是否超过容量
	if req.GuestCount > table.Capacity {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("guest count exceeds table capacity")))
		return
	}

	conflict, err := server.checkReservationConflict(ctx, req.TableID, merchant.ID, reservationDate, reservationTime, 0)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if conflict {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("该时间段已被预订，请选择其他时间")))
		return
	}

	pgDate := pgtype.Date{Time: reservationDate, Valid: true}
	pgTime := pgtype.Time{
		Microseconds: int64(reservationTime.Hour()*3600+reservationTime.Minute()*60) * 1000000,
		Valid:        true,
	}

	// 来源默认为 merchant
	source := req.Source
	if source == "" {
		source = ReservationSourceMerchant
	}

	// 创建预订（商户代订无需支付，直接确认）
	reservation, err := server.store.CreateTableReservationByMerchant(ctx, db.CreateTableReservationByMerchantParams{
		TableID:         req.TableID,
		UserID:          authPayload.UserID, // 商户用户ID
		MerchantID:      merchant.ID,
		ReservationDate: pgDate,
		ReservationTime: pgTime,
		GuestCount:      req.GuestCount,
		ContactName:     req.ContactName,
		ContactPhone:    req.ContactPhone,
		PaymentMode:     PaymentModeDeposit, // 商户代订默认到店结算
		DepositAmount:   0,                  // 商户代订无定金
		PrepaidAmount:   0,
		RefundDeadline:  time.Now(),                           // 无退款期限
		PaymentDeadline: time.Now().Add(365 * 24 * time.Hour), // 无支付期限
		Notes:           pgtype.Text{String: req.Notes, Valid: req.Notes != ""},
		Source:          pgtype.Text{String: source, Valid: true},
	})
	if err != nil {
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

	// 获取预定
	reservation, err := server.store.GetTableReservationForUpdate(ctx, req.ID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("reservation not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证权限：顾客或商户
	isOwner := reservation.UserID == authPayload.UserID
	isMerchant := false
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err == nil && merchant.ID == reservation.MerchantID {
		isMerchant = true
	}

	if !isOwner && !isMerchant {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not authorized to check in this reservation")))
		return
	}

	// 检查状态：只有已支付或已确认的预定可以签到
	if !isReservationStatusAllowed(reservation.Status, reservationActionCheckIn) {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("only paid or confirmed reservations can be checked in")))
		return
	}

	// P1-023 修复：签到时间窗口检查（使用常量替代硬编码）
	if reservation.ReservationDate.Valid && reservation.ReservationTime.Valid {
		hours := reservation.ReservationTime.Microseconds / 1000000 / 3600
		minutes := (reservation.ReservationTime.Microseconds / 1000000 % 3600) / 60
		reservationDateTime := time.Date(
			reservation.ReservationDate.Time.Year(),
			reservation.ReservationDate.Time.Month(),
			reservation.ReservationDate.Time.Day(),
			int(hours), int(minutes), 0, 0, time.Local,
		)

		now := time.Now()
		earlyLimit := reservationDateTime.Add(-time.Duration(ReservationCheckInEarlyMinutes) * time.Minute)
		lateLimit := reservationDateTime.Add(time.Duration(ReservationCheckInLateMinutes) * time.Minute)

		if now.Before(earlyLimit) {
			ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf(
				"尚未到签到时间，请在预订时间前%d分钟内签到", ReservationCheckInEarlyMinutes,
			)))
			return
		}
		if now.After(lateLimit) {
			ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf(
				"已超过签到时间%d分钟，请联系商户", ReservationCheckInLateMinutes,
			)))
			return
		}
	}

	// 更新状态为已签到
	updatedReservation, err := server.store.UpdateReservationToCheckedIn(ctx, req.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 推送 WebSocket 通知给商户
	if server.wsHub != nil {
		server.wsHub.SendToMerchant(reservation.MerchantID, websocket.Message{
			Type:      "reservation_checkin",
			Data:      []byte(fmt.Sprintf(`{"reservation_id":%d,"contact_name":"%s"}`, req.ID, reservation.ContactName)),
			Timestamp: time.Now(),
		})
	}

	ctx.JSON(http.StatusOK, newReservationResponse(updatedReservation))
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

	// 获取预定
	reservation, err := server.store.GetTableReservationForUpdate(ctx, req.ID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("reservation not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证权限：顾客或商户
	isOwner := reservation.UserID == authPayload.UserID
	isMerchant := false
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err == nil && merchant.ID == reservation.MerchantID {
		isMerchant = true
	}

	if !isOwner && !isMerchant {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not authorized to start cooking for this reservation")))
		return
	}

	// 检查状态：只有已确认或已签到的预定可以起菜
	if !isReservationStatusAllowed(reservation.Status, reservationActionStartCooking) {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("only confirmed or checked-in reservations can start cooking")))
		return
	}

	// 更新起菜时间
	updatedReservation, err := server.store.UpdateReservationCookingStarted(ctx, req.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 推送 WebSocket 通知给商户（厨房）
	if server.wsHub != nil {
		server.wsHub.SendToMerchant(reservation.MerchantID, websocket.Message{
			Type:      "reservation_start_cooking",
			Data:      []byte(fmt.Sprintf(`{"reservation_id":%d,"contact_name":"%s"}`, req.ID, reservation.ContactName)),
			Timestamp: time.Now(),
		})
	}

	ctx.JSON(http.StatusOK, newReservationResponse(updatedReservation))
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

	// 验证商户权限
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取预定
	reservation, err := server.store.GetTableReservationForUpdate(ctx, uriReq.ID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("reservation not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证所有权
	if reservation.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("reservation does not belong to your merchant")))
		return
	}

	// 只能修改未完成的预订
	if reservation.Status == ReservationStatusCompleted ||
		reservation.Status == ReservationStatusCancelled ||
		reservation.Status == ReservationStatusExpired {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("cannot modify completed, cancelled or expired reservations")))
		return
	}

	// 构建更新参数
	updateParams := db.UpdateReservationParams{
		ID: uriReq.ID,
	}

	if req.TableID != nil {
		updateParams.TableID = pgtype.Int8{Int64: *req.TableID, Valid: true}
	}
	if req.Date != nil {
		date, err := parseISODate(*req.Date, "invalid date format")
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}
		updateParams.ReservationDate = pgtype.Date{Time: date, Valid: true}
	}
	if req.Time != nil {
		t, err := time.Parse("15:04", *req.Time)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid time format")))
			return
		}
		updateParams.ReservationTime = pgtype.Time{
			Microseconds: int64(t.Hour()*3600+t.Minute()*60) * 1000000,
			Valid:        true,
		}
	}
	if req.GuestCount != nil {
		updateParams.GuestCount = pgtype.Int2{Int16: *req.GuestCount, Valid: true}
	}
	if req.ContactName != nil {
		updateParams.ContactName = pgtype.Text{String: *req.ContactName, Valid: true}
	}
	if req.ContactPhone != nil {
		updateParams.ContactPhone = pgtype.Text{String: *req.ContactPhone, Valid: true}
	}
	if req.Notes != nil {
		updateParams.Notes = pgtype.Text{String: *req.Notes, Valid: true}
	}

	// 如果更换了桌台或时间，需要检查可用性
	if req.TableID != nil || req.Date != nil || req.Time != nil {
		checkTableID := reservation.TableID
		if req.TableID != nil {
			checkTableID = *req.TableID
		}

		targetDate := reservation.ReservationDate.Time
		if req.Date != nil {
			date, _ := parseISODate(*req.Date, "invalid date format")
			targetDate = date
		}

		finalTime := time.Date(0, 1, 1, int(reservation.ReservationTime.Microseconds/1000000/3600), int((reservation.ReservationTime.Microseconds/1000000%3600)/60), 0, 0, time.UTC)
		if req.Time != nil {
			finalTime, _ = time.Parse("15:04", *req.Time)
		}

		conflict, err := server.checkReservationConflict(ctx, checkTableID, reservation.MerchantID, targetDate, finalTime, reservation.ID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		if conflict {
			ctx.JSON(http.StatusConflict, errorResponse(errors.New("该时间段已被预订，请选择其他时间")))
			return
		}
	}

	updatedReservation, err := server.store.UpdateReservation(ctx, updateParams)
	if err != nil {
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
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
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
					if item.DishImageUrl.Valid {
						resp[i].Items[j].ImageURL = normalizeUploadURLForClient(item.DishImageUrl.String)
					}
				} else if item.ComboID.Valid {
					resp[i].Items[j].ComboID = &item.ComboID.Int64
					resp[i].Items[j].Type = "combo"
					if item.ComboName.Valid {
						resp[i].Items[j].Name = item.ComboName.String
					}
					if item.ComboImageUrl.Valid {
						resp[i].Items[j].ImageURL = normalizeUploadURLForClient(item.ComboImageUrl.String)
					}
				}
			}
		}
	}

	ctx.JSON(http.StatusOK, gin.H{"reservations": resp})
}

func (server *Server) checkReservationConflict(ctx *gin.Context, tableID int64, merchantID int64, date time.Time, newTime time.Time, excludeID int64) (bool, error) {
	pgDate := pgtype.Date{Time: date, Valid: true}
	reservations, err := server.store.ListReservationsByTableAndDate(ctx, db.ListReservationsByTableAndDateParams{
		TableID:         tableID,
		ReservationDate: pgDate,
	})
	if err != nil {
		return false, err
	}

	// 获取商户营业时间以确定冲突规则
	businessHours, err := server.store.ListMerchantBusinessHours(ctx, merchantID)
	config := util.DefaultConfig
	if err == nil {
		dayOfWeek := int32(date.Weekday())
		var todayHours []db.MerchantBusinessHour
		for _, bh := range businessHours {
			if bh.SpecialDate.Valid && bh.SpecialDate.Time.Format("2006-01-02") == date.Format("2006-01-02") {
				todayHours = append(todayHours, bh)
			}
		}
		if len(todayHours) == 0 {
			for _, bh := range businessHours {
				if !bh.SpecialDate.Valid && bh.DayOfWeek == dayOfWeek {
					todayHours = append(todayHours, bh)
				}
			}
		}

		if len(todayHours) > 0 {
			h1 := todayHours[0]
			config.LunchStart = int(h1.OpenTime.Microseconds/1000000/3600*100) + int(h1.OpenTime.Microseconds/1000000%3600/60)
			config.LunchEnd = int(h1.CloseTime.Microseconds/1000000/3600*100) + int(h1.CloseTime.Microseconds/1000000%3600/60)
			config.DinnerStart = 0
			config.DinnerEnd = 0

			if len(todayHours) > 1 {
				h2 := todayHours[1]
				config.DinnerStart = int(h2.OpenTime.Microseconds/1000000/3600*100) + int(h2.OpenTime.Microseconds/1000000%3600/60)
				config.DinnerEnd = int(h2.CloseTime.Microseconds/1000000/3600*100) + int(h2.CloseTime.Microseconds/1000000%3600/60)
			} else if config.LunchStart >= 1500 {
				// 如果只有一个段，且是下午 15:00 之后开始，视为晚餐
				config.DinnerStart = config.LunchStart
				config.DinnerEnd = config.LunchEnd
				config.LunchStart = 0
				config.LunchEnd = 0
			}
		}
	}

	for _, r := range reservations {
		if r.ID == excludeID {
			continue
		}
		if r.Status == ReservationStatusCancelled || r.Status == ReservationStatusExpired || r.Status == ReservationStatusNoShow {
			continue
		}
		if !r.ReservationTime.Valid {
			continue
		}

		existingTime := util.CombineDateAndTime(r.ReservationDate.Time, r.ReservationTime.Microseconds)
		newDateTime := time.Date(date.Year(), date.Month(), date.Day(), newTime.Hour(), newTime.Minute(), 0, 0, date.Location())

		if util.AreReservationsConflictingWithConfig(newDateTime, existingTime, config) {
			return true, nil
		}
	}

	return false, nil
}
