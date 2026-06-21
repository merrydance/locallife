package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
)

type listOperatorProfitSharingConfigsRequest struct {
	RegionID    int64  `form:"region_id" binding:"omitempty,min=1"`
	Status      string `form:"status"`
	OrderSource string `form:"order_source"`
	MerchantID  int64  `form:"merchant_id"`
	Page        int32  `form:"page" binding:"omitempty,min=1"`
	Limit       int32  `form:"limit" binding:"omitempty,min=1,max=200"`
}

type listOperatorProfitSharingConfigsResponse struct {
	Items []profitSharingConfigResponse `json:"items"`
	Page  int32                         `json:"page"`
	Limit int32                         `json:"limit"`
}

// listOperatorProfitSharingConfigs 获取运营商可见的分账规则配置
// @Summary 获取分账规则配置
// @Description 运营商查看本区域及全局分账规则配置
// @Tags 运营商数据统计
// @Accept json
// @Produce json
// @Param region_id query int false "区域ID；不传时聚合当前运营商全部可管区域"
// @Param status query string false "状态"
// @Param order_source query string false "订单来源"
// @Param merchant_id query int false "商户ID"
// @Param page query int false "页码" default(1) minimum(1)
// @Param limit query int false "每页数量" default(20) minimum(1) maximum(200)
// @Security BearerAuth
// @Success 200 {object} listOperatorProfitSharingConfigsResponse "配置列表"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/operators/me/profit-sharing/configs [get]
func (server *Server) listOperatorProfitSharingConfigs(ctx *gin.Context) {
	var req listOperatorProfitSharingConfigsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if req.Page == 0 {
		req.Page = 1
	}
	if req.Limit == 0 {
		req.Limit = 20
	}

	selection, err := server.resolveOperatorRegionSelection(ctx)
	if err != nil {
		server.respondOperatorRegionSelectionError(ctx, err)
		return
	}

	configs, err := server.store.ListProfitSharingConfigsForRegions(ctx, db.ListProfitSharingConfigsForRegionsParams{
		Status:      req.Status,
		OrderSource: req.OrderSource,
		RegionIds:   selection.RegionIDs,
		MerchantID:  req.MerchantID,
		Limit:       req.Limit,
		Offset:      pageOffset(req.Page, req.Limit),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	items := make([]profitSharingConfigResponse, len(configs))
	for i, config := range configs {
		items[i] = newProfitSharingConfigResponse(config)
	}

	ctx.JSON(http.StatusOK, listOperatorProfitSharingConfigsResponse{
		Items: items,
		Page:  req.Page,
		Limit: req.Limit,
	})
}
