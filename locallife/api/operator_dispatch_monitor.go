package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
)

const operatorPendingDispatchStatus = "pending"

type operatorPendingDispatchRegionURI struct {
	RegionID int64 `uri:"region_id" binding:"required,min=1"`
}

type listOperatorPendingDispatchesQuery struct {
	Page  int32 `form:"page,default=1" binding:"min=1"`
	Limit int32 `form:"limit,default=20" binding:"min=1,max=100"`
}

type operatorPendingDispatchSummaryResponse struct {
	RegionID           int64     `json:"region_id"`
	RegionName         string    `json:"region_name"`
	PendingTotal       int64     `json:"pending_total"`
	TimeoutOver3mTotal int64     `json:"timeout_over_3m_total"`
	OldestWaitSeconds  int64     `json:"oldest_wait_seconds"`
	LatestRefreshAt    time.Time `json:"latest_refresh_at"`
}

type operatorPendingDispatchItemResponse struct {
	DeliveryID       int64      `json:"delivery_id"`
	OrderID          int64      `json:"order_id"`
	OrderNo          string     `json:"order_no"`
	MerchantID       int64      `json:"merchant_id"`
	MerchantName     string     `json:"merchant_name"`
	RegionID         int64      `json:"region_id"`
	RegionName       string     `json:"region_name"`
	WaitSeconds      int64      `json:"wait_seconds"`
	DeliveryFee      int64      `json:"delivery_fee"`
	ExpectedPickupAt *time.Time `json:"expected_pickup_at,omitempty"`
	IsTimeoutOver3m  bool       `json:"is_timeout_over_3m"`
}

type listOperatorPendingDispatchesResponse struct {
	Items []operatorPendingDispatchItemResponse `json:"items"`
	Total int64                                 `json:"total"`
	Page  int32                                 `json:"page"`
	Limit int32                                 `json:"limit"`
}

func newOperatorPendingDispatchItemResponse(item db.ListOperatorPendingDispatchesRow) operatorPendingDispatchItemResponse {
	response := operatorPendingDispatchItemResponse{
		DeliveryID:      item.DeliveryID,
		OrderID:         item.OrderID,
		OrderNo:         item.OrderNo,
		MerchantID:      item.MerchantID,
		MerchantName:    item.MerchantName,
		RegionID:        item.RegionID,
		RegionName:      item.RegionName,
		WaitSeconds:     item.WaitSeconds,
		DeliveryFee:     item.DeliveryFee,
		IsTimeoutOver3m: item.IsTimeoutOverThreshold,
	}
	if item.ExpectedPickupAt.Valid {
		response.ExpectedPickupAt = &item.ExpectedPickupAt.Time
	}
	return response
}

// getOperatorPendingDispatchSummary godoc
// @Summary 获取运营区域待接单摘要
// @Description 获取指定运营区域当前待接单总数、超时数量和最长等待时长
// @Tags 运营商数据统计
// @Accept json
// @Produce json
// @Param region_id path int true "区域ID"
// @Success 200 {object} operatorPendingDispatchSummaryResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 403 {object} ErrorResponse "无权限访问该区域"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Security BearerAuth
// @Router /v1/operator/regions/{region_id}/delivery-pool/summary [get]
func (server *Server) getOperatorPendingDispatchSummary(ctx *gin.Context) {
	var uri operatorPendingDispatchRegionURI
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	if _, err := server.checkOperatorManagesRegion(ctx, uri.RegionID); err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(err))
		return
	}

	summary, err := server.store.GetOperatorPendingDispatchSummary(ctx, db.GetOperatorPendingDispatchSummaryParams{
		RegionID:      uri.RegionID,
		Status:        operatorPendingDispatchStatus,
		TimeoutBefore: time.Now().Add(-3 * time.Minute),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	oldestWaitSeconds, err := normalizeInt64Result(summary.OldestWaitSeconds)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, operatorPendingDispatchSummaryResponse{
		RegionID:           summary.RegionID,
		RegionName:         summary.RegionName,
		PendingTotal:       summary.PendingTotal,
		TimeoutOver3mTotal: summary.TimeoutOverThresholdTotal,
		OldestWaitSeconds:  oldestWaitSeconds,
		LatestRefreshAt:    summary.LatestRefreshAt,
	})
}

// listOperatorPendingDispatches godoc
// @Summary 获取运营区域待接单列表
// @Description 获取指定运营区域当前待接单列表，支持分页
// @Tags 运营商数据统计
// @Accept json
// @Produce json
// @Param region_id path int true "区域ID"
// @Param page query int false "页码"
// @Param limit query int false "每页数量"
// @Success 200 {object} listOperatorPendingDispatchesResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 403 {object} ErrorResponse "无权限访问该区域"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Security BearerAuth
// @Router /v1/operator/regions/{region_id}/delivery-pool [get]
func (server *Server) listOperatorPendingDispatches(ctx *gin.Context) {
	var uri operatorPendingDispatchRegionURI
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var query listOperatorPendingDispatchesQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	if _, err := server.checkOperatorManagesRegion(ctx, uri.RegionID); err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(err))
		return
	}

	offset := (query.Page - 1) * query.Limit
	timeoutBefore := time.Now().Add(-3 * time.Minute)
	items, err := server.store.ListOperatorPendingDispatches(ctx, db.ListOperatorPendingDispatchesParams{
		RegionID:      uri.RegionID,
		Status:        operatorPendingDispatchStatus,
		TimeoutBefore: timeoutBefore,
		Limit:         query.Limit,
		Offset:        offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	total, err := server.store.CountOperatorPendingDispatches(ctx, db.CountOperatorPendingDispatchesParams{
		RegionID: uri.RegionID,
		Status:   operatorPendingDispatchStatus,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	responseItems := make([]operatorPendingDispatchItemResponse, len(items))
	for index, item := range items {
		responseItems[index] = newOperatorPendingDispatchItemResponse(item)
	}

	ctx.JSON(http.StatusOK, listOperatorPendingDispatchesResponse{
		Items: responseItems,
		Total: total,
		Page:  query.Page,
		Limit: query.Limit,
	})
}
