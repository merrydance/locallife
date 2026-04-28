package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
)

type listAdminGroupApplicationsRequest struct {
	Status string `form:"status" binding:"omitempty,oneof=draft submitted approved rejected"`
	Page   int32  `form:"page" binding:"omitempty,min=1"`
	Limit  int32  `form:"limit" binding:"omitempty,min=1,max=100"`
}

type listAdminGroupApplicationsResponse struct {
	Applications []groupApplicationResponse `json:"applications"`
	Total        int64                      `json:"total"`
	Page         int32                      `json:"page"`
	Limit        int32                      `json:"limit"`
	HasMore      bool                       `json:"has_more"`
}

type adminGroupApplicationIDRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

func (server *Server) listGroupApplicationsAdmin(ctx *gin.Context) {
	var req listAdminGroupApplicationsRequest
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

	sqlStore, ok := server.store.(*db.SQLStore)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("store does not support group admin queries")))
		return
	}

	apps, err := sqlStore.ListGroupApplicationsAdmin(ctx, db.ListGroupApplicationsAdminParams{
		Status: req.Status,
		Limit:  req.Limit,
		Offset: pageOffset(req.Page, req.Limit),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	total, err := sqlStore.CountGroupApplicationsAdmin(ctx, req.Status)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	items := make([]groupApplicationResponse, 0, len(apps))
	for _, app := range apps {
		resp, err := newGroupApplicationResponse(app)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		items = append(items, resp)
	}

	hasMore := int64(req.Page*req.Limit) < total
	ctx.JSON(http.StatusOK, listAdminGroupApplicationsResponse{
		Applications: items,
		Total:        total,
		Page:         req.Page,
		Limit:        req.Limit,
		HasMore:      hasMore,
	})
}

func (server *Server) getGroupApplicationAdmin(ctx *gin.Context) {
	var uriReq adminGroupApplicationIDRequest
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	app, err := server.store.GetGroupApplication(ctx, uriReq.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("application not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	server.writeGroupApplicationResponse(ctx, http.StatusOK, app)
}
