package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/token"
)

type cartPackagingOptionResponse struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Price       int64  `json:"price"`
	IsEnabled   bool   `json:"is_enabled"`
	SortOrder   int16  `json:"sort_order"`
}

type cartPackagingResponse struct {
	Enabled          bool                          `json:"enabled"`
	Required         bool                          `json:"required"`
	Applicable       bool                          `json:"applicable"`
	SelectedOptionID *int64                        `json:"selected_option_id"`
	SelectionVersion int64                         `json:"selection_version"`
	Options          []cartPackagingOptionResponse `json:"options"`
}

type cartPackagingSelectionResponse struct {
	SelectedOptionID *int64 `json:"selected_option_id"`
	SelectionVersion int64  `json:"selection_version"`
}

type packagingPreviewResponse struct {
	Enabled          bool   `json:"enabled"`
	Required         bool   `json:"required"`
	Applicable       bool   `json:"applicable"`
	SelectedOptionID *int64 `json:"selected_option_id"`
	SelectionVersion int64  `json:"selection_version"`
	Fee              int64  `json:"fee"`
}

type putCartPackagingSelectionRequest struct {
	MerchantID        int64  `json:"merchant_id" binding:"required,min=1"`
	OrderType         string `json:"order_type"`
	TableID           *int64 `json:"table_id" binding:"omitempty,min=1"`
	ReservationID     *int64 `json:"reservation_id" binding:"omitempty,min=1"`
	PackagingOptionID int64  `json:"packaging_option_id" binding:"required,min=1"`
}

type deleteCartPackagingSelectionRequest struct {
	MerchantID    int64  `json:"merchant_id" binding:"required,min=1"`
	OrderType     string `json:"order_type"`
	TableID       *int64 `json:"table_id" binding:"omitempty,min=1"`
	ReservationID *int64 `json:"reservation_id" binding:"omitempty,min=1"`
}

func (server *Server) attachCartPackagingState(ctx context.Context, userID int64, cart *db.Cart, resp *logic.CartResponse) error {
	if resp == nil {
		return nil
	}
	state, err := logic.NewPackagingService(server.store).ResolveCartPackagingState(ctx, logic.ResolveCartPackagingStateInput{
		UserID:     userID,
		MerchantID: resp.MerchantID,
		OrderType:  resp.OrderType,
		Cart:       cart,
	})
	if err != nil {
		return err
	}
	resp.Packaging = state
	return nil
}

// putCartPackagingSelection godoc
// @Summary 选择购物车包装方式
// @Description 为当前用户在指定商户和订单场景下的购物车选择包装方式；重复选择同一包装方式不会递增版本
// @Tags 购物车
// @Accept json
// @Produce json
// @Param request body putCartPackagingSelectionRequest true "包装方式选择请求"
// @Success 200 {object} cartPackagingSelectionResponse "包装方式选择状态"
// @Failure 400 {object} ErrorResponse "请求参数错误/包装方式不可用"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "购物车不属于当前用户"
// @Failure 404 {object} ErrorResponse "购物车不存在"
// @Failure 409 {object} ErrorResponse "购物车状态冲突"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/cart/packaging-selection [put]
// @Security BearerAuth
func (server *Server) putCartPackagingSelection(ctx *gin.Context) {
	var req putCartPackagingSelectionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if req.OrderType == "" {
		req.OrderType = db.OrderTypeTakeout
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	result, err := logic.NewPackagingService(server.store).SelectCartPackagingOption(ctx, logic.CartPackagingSelectionInput{
		UserID:            authPayload.UserID,
		MerchantID:        req.MerchantID,
		OrderType:         req.OrderType,
		TableID:           req.TableID,
		ReservationID:     req.ReservationID,
		PackagingOptionID: req.PackagingOptionID,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("put cart packaging selection: %w", err)))
		return
	}

	ctx.JSON(http.StatusOK, toCartPackagingSelectionResponse(result))
}

// deleteCartPackagingSelection godoc
// @Summary 清除购物车包装方式
// @Description 清除当前用户在指定商户和订单场景下的购物车包装方式；重复清除保持同一版本
// @Tags 购物车
// @Accept json
// @Produce json
// @Param request body deleteCartPackagingSelectionRequest true "清除包装方式请求"
// @Success 200 {object} cartPackagingSelectionResponse "包装方式选择状态"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "购物车不属于当前用户"
// @Failure 404 {object} ErrorResponse "购物车不存在"
// @Failure 409 {object} ErrorResponse "购物车状态冲突"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/cart/packaging-selection [delete]
// @Security BearerAuth
func (server *Server) deleteCartPackagingSelection(ctx *gin.Context) {
	var req deleteCartPackagingSelectionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if req.OrderType == "" {
		req.OrderType = db.OrderTypeTakeout
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	result, err := logic.NewPackagingService(server.store).ClearCartPackagingSelection(ctx, logic.CartPackagingSelectionInput{
		UserID:        authPayload.UserID,
		MerchantID:    req.MerchantID,
		OrderType:     req.OrderType,
		TableID:       req.TableID,
		ReservationID: req.ReservationID,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("delete cart packaging selection: %w", err)))
		return
	}

	ctx.JSON(http.StatusOK, toCartPackagingSelectionResponse(result))
}

func toCartPackagingSelectionResponse(result logic.CartPackagingSelectionResult) cartPackagingSelectionResponse {
	return cartPackagingSelectionResponse{
		SelectedOptionID: result.SelectedOptionID,
		SelectionVersion: result.SelectionVersion,
	}
}

func toPackagingPreviewResponse(state logic.CartPackagingState, fee int64) packagingPreviewResponse {
	return packagingPreviewResponse{
		Enabled:          state.Enabled,
		Required:         state.Required,
		Applicable:       state.Applicable,
		SelectedOptionID: state.SelectedOptionID,
		SelectionVersion: state.SelectionVersion,
		Fee:              fee,
	}
}
