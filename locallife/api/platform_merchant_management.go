package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
)

func platformComplaintCategoriesFromMerchantRows(rows []db.ListPlatformMerchantComplaintCategoriesRow) []platformComplaintCategory {
	items := make([]platformComplaintCategory, 0, len(rows))
	for _, row := range rows {
		items = append(items, platformComplaintCategory{Category: row.ClaimType, Count: row.Count})
	}
	return items
}

func (server *Server) listPlatformMerchants(ctx *gin.Context) {
	var req platformEntityListRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	normalizePlatformPagination(&req)

	rows, err := server.store.ListPlatformMerchantCards(ctx, db.ListPlatformMerchantCardsParams{
		Limit:  req.Limit,
		Offset: pageOffset(req.Page, req.Limit),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	total, err := server.store.CountPlatformMerchants(ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	merchants := make([]platformMerchantCard, 0, len(rows))
	for _, row := range rows {
		merchants = append(merchants, platformMerchantCard{
			ID:             row.ID,
			Name:           row.Name,
			RegionID:       row.RegionID,
			RegionName:     stringFromPgText(row.RegionName),
			Status:         row.Status,
			IsOpen:         row.IsOpen,
			MonthOrders:    row.MonthOrders,
			ComplaintCount: row.ComplaintCount,
		})
	}

	ctx.JSON(http.StatusOK, platformMerchantListResponse{
		Merchants: merchants,
		Total:     total,
		Page:      req.Page,
		Limit:     req.Limit,
		HasMore:   platformHasMore(req.Page, req.Limit, len(merchants), total),
	})
}

func (server *Server) getPlatformMerchantDetail(ctx *gin.Context) {
	var uri platformMerchantIDRequest
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	row, err := server.store.GetPlatformMerchantDetail(ctx, uri.MerchantID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrMerchantNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	categoryRows, err := server.store.ListPlatformMerchantComplaintCategories(ctx, uri.MerchantID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, platformMerchantDetailResponse{
		ID:   row.ID,
		Name: row.Name,
		Basic: platformMerchantBasicInfo{
			Name:       row.Name,
			Phone:      row.Phone,
			Address:    row.Address,
			RegionID:   row.RegionID,
			RegionName: stringFromPgText(row.RegionName),
			Status:     row.Status,
			IsOpen:     row.IsOpen,
		},
		OrderStats: platformOrderStats{
			TotalOrders:     row.TotalOrders,
			TotalIncome:     row.TotalIncome,
			LastMonthOrders: row.MonthOrders,
			LastMonthIncome: row.MonthIncome,
		},
		Service: platformServiceStats{
			ComplaintCount:      row.ComplaintCount,
			ComplaintCategories: platformComplaintCategoriesFromMerchantRows(categoryRows),
		},
		CreatedAt:  row.CreatedAt,
		CanSuspend: row.Status == db.MerchantStatusActive,
		CanResume:  row.Status == db.MerchantStatusSuspended,
	})
}

func (server *Server) updatePlatformMerchantStatus(ctx *gin.Context, targetStatus string) {
	var uri platformMerchantIDRequest
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	currentMerchant, err := server.store.GetMerchant(ctx, uri.MerchantID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrMerchantNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if targetStatus == db.MerchantStatusSuspended && currentMerchant.Status != db.MerchantStatusActive {
		ctx.JSON(http.StatusConflict, errorResponse(errPlatformEntityStatusConflict))
		return
	}
	if targetStatus == db.MerchantStatusActive && currentMerchant.Status != db.MerchantStatusSuspended {
		ctx.JSON(http.StatusConflict, errorResponse(errPlatformEntityStatusConflict))
		return
	}

	merchant, err := server.store.UpdateMerchantStatus(ctx, db.UpdateMerchantStatusParams{
		ID:     uri.MerchantID,
		Status: targetStatus,
	})
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrMerchantNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, platformMerchantStatusResponse{ID: merchant.ID, Status: merchant.Status})
}

func (server *Server) suspendPlatformMerchant(ctx *gin.Context) {
	server.updatePlatformMerchantStatus(ctx, db.MerchantStatusSuspended)
}

func (server *Server) resumePlatformMerchant(ctx *gin.Context) {
	server.updatePlatformMerchantStatus(ctx, db.MerchantStatusActive)
}
