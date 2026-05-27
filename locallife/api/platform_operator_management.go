package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
)

func platformComplaintCategoriesFromOperatorRows(rows []db.ListPlatformOperatorComplaintCategoriesRow) []platformComplaintCategory {
	items := make([]platformComplaintCategory, 0, len(rows))
	for _, row := range rows {
		items = append(items, platformComplaintCategory{Category: row.ClaimType, Count: row.Count})
	}
	return items
}

func (server *Server) listPlatformOperators(ctx *gin.Context) {
	var req platformEntityListRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	normalizePlatformPagination(&req)

	rows, err := server.store.ListPlatformOperatorCards(ctx, db.ListPlatformOperatorCardsParams{
		Limit:  req.Limit,
		Offset: pageOffset(req.Page, req.Limit),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	total, err := server.store.CountPlatformOperators(ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	operators := make([]platformOperatorCard, 0, len(rows))
	for _, row := range rows {
		operators = append(operators, platformOperatorCard{
			ID:             row.ID,
			Name:           row.Name,
			Status:         row.Status,
			RegionCount:    row.RegionCount,
			MerchantCount:  row.MerchantCount,
			ComplaintCount: row.ComplaintCount,
		})
	}

	ctx.JSON(http.StatusOK, platformOperatorListResponse{
		Operators: operators,
		Total:     total,
		Page:      req.Page,
		Limit:     req.Limit,
		HasMore:   platformHasMore(req.Page, req.Limit, len(operators), total),
	})
}

func (server *Server) getPlatformOperatorDetail(ctx *gin.Context) {
	var uri platformOperatorIDRequest
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	row, err := server.store.GetPlatformOperatorDetail(ctx, uri.OperatorID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrOperatorNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	regionRows, err := server.store.ListPlatformOperatorRegions(ctx, uri.OperatorID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	categoryRows, err := server.store.ListPlatformOperatorComplaintCategories(ctx, uri.OperatorID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	regions := make([]platformOperatorRegion, 0, len(regionRows))
	for _, region := range regionRows {
		regions = append(regions, platformOperatorRegion{
			RegionID:   region.RegionID,
			RegionName: region.RegionName,
			Status:     region.Status,
		})
	}

	ctx.JSON(http.StatusOK, platformOperatorDetailResponse{
		ID:            row.ID,
		Name:          row.Name,
		ContactName:   row.ContactName,
		ContactPhone:  row.ContactPhone,
		Status:        row.Status,
		RegionID:      row.RegionID,
		RegionName:    stringFromPgText(row.RegionName),
		RegionCount:   row.RegionCount,
		MerchantCount: row.MerchantCount,
		Regions:       regions,
		OrderStats: platformOrderStats{
			LastMonthOrders: row.MonthOrders,
			LastMonthIncome: row.MonthRevenue,
		},
		Service: platformServiceStats{
			ComplaintCount:      row.ComplaintCount,
			ComplaintCategories: platformComplaintCategoriesFromOperatorRows(categoryRows),
		},
		CreatedAt:  row.CreatedAt,
		CanSuspend: row.Status == "active",
		CanResume:  row.Status == "suspended",
	})
}
