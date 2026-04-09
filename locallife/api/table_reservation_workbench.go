package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/token"
)

type merchantReservationWorkbenchRequest struct {
	Date string `form:"date" binding:"required"`
}

type merchantReservationWorkbenchSummaryResponse struct {
	ReservationCount int64 `json:"reservation_count"`
	ActiveTableCount int64 `json:"active_table_count"`
}

type merchantReservationWorkbenchStatusTotalsResponse struct {
	All       int64 `json:"all"`
	Pending   int64 `json:"pending"`
	Paid      int64 `json:"paid"`
	Confirmed int64 `json:"confirmed"`
	CheckedIn int64 `json:"checked_in"`
	Completed int64 `json:"completed"`
	Cancelled int64 `json:"cancelled"`
	Expired   int64 `json:"expired"`
	NoShow    int64 `json:"no_show"`
	Exception int64 `json:"exception"`
}

type merchantReservationPrepReferenceResponse struct {
	ReservationID   int64  `json:"reservation_id"`
	ReservationTime string `json:"reservation_time"`
	TableNo         string `json:"table_no,omitempty"`
	ContactName     string `json:"contact_name,omitempty"`
	Status          string `json:"status"`
	Quantity        int16  `json:"quantity"`
}

type merchantReservationPrepItemResponse struct {
	Type             string                                     `json:"type"`
	DishID           *int64                                     `json:"dish_id,omitempty"`
	ComboID          *int64                                     `json:"combo_id,omitempty"`
	Name             string                                     `json:"name"`
	TotalQuantity    int64                                      `json:"total_quantity"`
	ReservationCount int64                                      `json:"reservation_count"`
	References       []merchantReservationPrepReferenceResponse `json:"references"`
}

type merchantReservationPrepSummaryResponse struct {
	TableCount    int64                                 `json:"table_count"`
	DishKinds     int64                                 `json:"dish_kinds"`
	TotalQuantity int64                                 `json:"total_quantity"`
	Items         []merchantReservationPrepItemResponse `json:"items"`
}

type merchantReservationWorkbenchResponse struct {
	Date         string                                           `json:"date"`
	Summary      merchantReservationWorkbenchSummaryResponse      `json:"summary"`
	StatusTotals merchantReservationWorkbenchStatusTotalsResponse `json:"status_totals"`
	PrepSummary  merchantReservationPrepSummaryResponse           `json:"prep_summary"`
}

// getMerchantReservationWorkbench 商户预订工作台汇总
// @Summary 获取商户预订工作台汇总
// @Description 按日期返回预订工作台摘要、状态计数和备菜汇总，供商户工作台首页使用
// @Tags 预定管理-商户
// @Accept json
// @Produce json
// @Param date query string true "日期 (YYYY-MM-DD)"
// @Success 200 {object} merchantReservationWorkbenchResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "非商户"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /v1/reservations/merchant/workbench [get]
func (server *Server) getMerchantReservationWorkbench(ctx *gin.Context) {
	var req merchantReservationWorkbenchRequest
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
	result, err := logic.MerchantReservationWorkbench(ctx, server.store, logic.MerchantReservationWorkbenchInput{
		OperatorUserID:  authPayload.UserID,
		ReservationDate: reservationDate,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	prepItems := make([]merchantReservationPrepItemResponse, 0, len(result.PrepSummary.Items))
	for _, item := range result.PrepSummary.Items {
		references := make([]merchantReservationPrepReferenceResponse, 0, len(item.References))
		for _, reference := range item.References {
			references = append(references, merchantReservationPrepReferenceResponse(reference))
		}

		prepItems = append(prepItems, merchantReservationPrepItemResponse{
			Type:             item.Type,
			DishID:           item.DishID,
			ComboID:          item.ComboID,
			Name:             item.Name,
			TotalQuantity:    item.TotalQuantity,
			ReservationCount: item.ReservationCount,
			References:       references,
		})
	}

	ctx.JSON(http.StatusOK, merchantReservationWorkbenchResponse{
		Date: req.Date,
		Summary: merchantReservationWorkbenchSummaryResponse{
			ReservationCount: result.Summary.ReservationCount,
			ActiveTableCount: result.Summary.ActiveTableCount,
		},
		StatusTotals: merchantReservationWorkbenchStatusTotalsResponse(result.StatusTotals),
		PrepSummary: merchantReservationPrepSummaryResponse{
			TableCount:    result.PrepSummary.TableCount,
			DishKinds:     result.PrepSummary.DishKinds,
			TotalQuantity: result.PrepSummary.TotalQuantity,
			Items:         prepItems,
		},
	})
}
