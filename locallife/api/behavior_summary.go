package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
)

type behaviorSummary struct {
	EntityType     string  `json:"entity_type"`
	EntityID       int64   `json:"entity_id"`
	TotalOrders    int32   `json:"total_orders"`
	AbnormalClaims int32   `json:"abnormal_claims"`
	AbnormalRate   float64 `json:"abnormal_rate"`
}

type behaviorSummaryResponse struct {
	OrderID  int64             `json:"order_id"`
	Window   behaviorDateRange `json:"window"`
	User     behaviorSummary   `json:"user"`
	Merchant behaviorSummary   `json:"merchant"`
	Rider    *behaviorSummary  `json:"rider,omitempty"`
}

type behaviorDateRange struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

func parseBehaviorDateRange(ctx *gin.Context, defaultDays int) (time.Time, time.Time, error) {
	layout := "2006-01-02"
	endDateStr := ctx.Query("end_date")
	startDateStr := ctx.Query("start_date")

	endDate := time.Now()
	if endDateStr != "" {
		parsed, err := time.Parse(layout, endDateStr)
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("invalid end_date")
		}
		endDate = parsed
	}

	startDate := endDate.AddDate(0, 0, -defaultDays)
	if startDateStr != "" {
		parsed, err := time.Parse(layout, startDateStr)
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("invalid start_date")
		}
		startDate = parsed
	}

	if startDate.After(endDate) {
		return time.Time{}, time.Time{}, errors.New("start_date must be before end_date")
	}

	return startDate, endDate, nil
}

func buildBehaviorSummary(ctx *gin.Context, store db.Store, entityType string, entityID int64, startDate, endDate time.Time) (behaviorSummary, error) {
	startDateParam := pgtype.Date{Time: startDate, Valid: true}
	endDateParam := pgtype.Date{Time: endDate, Valid: true}
	row, err := store.GetAbnormalStatsSummary(ctx, db.GetAbnormalStatsSummaryParams{
		EntityType: entityType,
		EntityID:   entityID,
		StatDate:   startDateParam,
		StatDate_2: endDateParam,
	})
	if err != nil {
		return behaviorSummary{}, err
	}

	ratio := 0.0
	if row.TotalOrders > 0 {
		ratio = float64(row.AbnormalClaims) / float64(row.TotalOrders)
	}

	return behaviorSummary{
		EntityType:     entityType,
		EntityID:       entityID,
		TotalOrders:    row.TotalOrders,
		AbnormalClaims: row.AbnormalClaims,
		AbnormalRate:   ratio,
	}, nil
}

// getMerchantClaimBehaviorSummary 商户查看索赔相关行为回溯摘要
func (server *Server) getMerchantClaimBehaviorSummary(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, err := server.getMerchantFromUserID(ctx, authPayload.UserID)
	if err != nil {
		return
	}

	orderIDStr := ctx.Query("order_id")
	if orderIDStr == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("order_id is required")))
		return
	}
	orderID, err := strconv.ParseInt(orderIDStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid order_id")))
		return
	}

	order, err := server.store.GetOrder(ctx, orderID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("order not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if order.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("order does not belong to merchant")))
		return
	}

	startDate, endDate, err := parseBehaviorDateRange(ctx, 30)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	userSummary, err := buildBehaviorSummary(ctx, server.store, "user", order.UserID, startDate, endDate)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	merchantSummary, err := buildBehaviorSummary(ctx, server.store, "merchant", merchant.ID, startDate, endDate)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	var riderSummary *behaviorSummary
	if delivery, err := server.store.GetDeliveryByOrderID(ctx, order.ID); err == nil && delivery.RiderID.Valid {
		summary, err := buildBehaviorSummary(ctx, server.store, "rider", delivery.RiderID.Int64, startDate, endDate)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		riderSummary = &summary
	}

	ctx.JSON(http.StatusOK, behaviorSummaryResponse{
		OrderID: order.ID,
		Window: behaviorDateRange{
			StartDate: startDate.Format("2006-01-02"),
			EndDate:   endDate.Format("2006-01-02"),
		},
		User:     userSummary,
		Merchant: merchantSummary,
		Rider:    riderSummary,
	})
}

// getRiderClaimBehaviorSummary 骑手查看索赔相关行为回溯摘要
func (server *Server) getRiderClaimBehaviorSummary(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	rider, err := server.getRiderFromUserID(ctx, authPayload.UserID)
	if err != nil {
		return
	}

	orderIDStr := ctx.Query("order_id")
	if orderIDStr == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("order_id is required")))
		return
	}
	orderID, err := strconv.ParseInt(orderIDStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid order_id")))
		return
	}

	order, err := server.store.GetOrder(ctx, orderID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("order not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	delivery, err := server.store.GetDeliveryByOrderID(ctx, order.ID)
	if err != nil || !delivery.RiderID.Valid || delivery.RiderID.Int64 != rider.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("order does not belong to rider")))
		return
	}

	startDate, endDate, err := parseBehaviorDateRange(ctx, 30)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	userSummary, err := buildBehaviorSummary(ctx, server.store, "user", order.UserID, startDate, endDate)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	merchantSummary, err := buildBehaviorSummary(ctx, server.store, "merchant", order.MerchantID, startDate, endDate)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	riderSummary, err := buildBehaviorSummary(ctx, server.store, "rider", rider.ID, startDate, endDate)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, behaviorSummaryResponse{
		OrderID: order.ID,
		Window: behaviorDateRange{
			StartDate: startDate.Format("2006-01-02"),
			EndDate:   endDate.Format("2006-01-02"),
		},
		User:     userSummary,
		Merchant: merchantSummary,
		Rider:    &riderSummary,
	})
}
