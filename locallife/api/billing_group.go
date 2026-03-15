package api

import (
	"errors"
	"net/http"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"

	"github.com/gin-gonic/gin"
)

type createBillingGroupRequest struct {
	DiningSessionID int64 `json:"dining_session_id" binding:"required,min=1"`
}

type billingGroupURIRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

type listBillingGroupsRequest struct {
	DiningSessionID int64 `form:"dining_session_id" binding:"required,min=1"`
}

type billingGroupListResponse struct {
	BillingGroups []billingGroupResponse `json:"billing_groups"`
	Total         int64                  `json:"total"`
}

type billingGroupOrderResponse struct {
	ID             int64   `json:"id"`
	BillingGroupID int64   `json:"billing_group_id"`
	OrderID        int64   `json:"order_id"`
	Amount         int64   `json:"amount"`
	Status         string  `json:"status"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      *string `json:"updated_at,omitempty"`
}

type billingGroupOrderListResponse struct {
	Orders []billingGroupOrderResponse `json:"orders"`
	Total  int64                       `json:"total"`
}

func newBillingGroupOrderResponse(o db.BillingGroupOrder) billingGroupOrderResponse {
	resp := billingGroupOrderResponse{
		ID:             o.ID,
		BillingGroupID: o.BillingGroupID,
		OrderID:        o.OrderID,
		Amount:         o.Amount,
		Status:         o.Status,
		CreatedAt:      o.CreatedAt.Format(timeLayout),
	}
	if o.UpdatedAt.Valid {
		t := o.UpdatedAt.Time.Format(timeLayout)
		resp.UpdatedAt = &t
	}
	return resp
}

// createBillingGroup 创建账单组（用于单独结算/拼桌）
// @Summary 创建账单组
// @Tags 账单组
// @Accept json
// @Produce json
// @Param request body createBillingGroupRequest true "创建账单组请求"
// @Success 200 {object} billingGroupResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/billing-groups [post]
func (server *Server) createBillingGroup(ctx *gin.Context) {
	var req createBillingGroupRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	session, err := server.store.GetDiningSession(ctx, req.DiningSessionID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("dining session not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if session.Status != "open" {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("dining session is not open")))
		return
	}

	billingGroup, err := server.store.CreateBillingGroup(ctx, db.CreateBillingGroupParams{
		DiningSessionID: session.ID,
		Status:          "open",
		IsDefault:       false,
		TotalAmount:     0,
		PaidAmount:      0,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if _, err := server.store.CreateBillingGroupMember(ctx, db.CreateBillingGroupMemberParams{
		BillingGroupID: billingGroup.ID,
		UserID:         authPayload.UserID,
		Role:           "owner",
	}); err != nil && db.ErrorCode(err) != db.UniqueViolation {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newBillingGroupResponse(billingGroup))
}

// joinBillingGroup 加入账单组
// @Summary 加入账单组
// @Tags 账单组
// @Produce json
// @Param id path int true "账单组ID"
// @Success 200 {object} billingGroupResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/billing-groups/{id}/join [post]
func (server *Server) joinBillingGroup(ctx *gin.Context) {
	var req billingGroupURIRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	billingGroup, err := server.store.GetBillingGroup(ctx, req.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("billing group not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if billingGroup.Status == "closed" {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("billing group is closed")))
		return
	}

	session, err := server.store.GetDiningSession(ctx, billingGroup.DiningSessionID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("dining session not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if session.Status != "open" {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("dining session is not open")))
		return
	}

	if session.UserID != authPayload.UserID {
		defaultGroup, err := server.store.GetDefaultBillingGroupBySession(ctx, session.ID)
		if err != nil {
			if isNotFoundError(err) {
				ctx.JSON(http.StatusForbidden, errorResponse(errors.New("access denied")))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		if _, err := server.store.GetActiveBillingGroupMember(ctx, db.GetActiveBillingGroupMemberParams{
			BillingGroupID: defaultGroup.ID,
			UserID:         authPayload.UserID,
		}); err != nil {
			if isNotFoundError(err) {
				ctx.JSON(http.StatusForbidden, errorResponse(errors.New("access denied")))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}

	if _, err := server.store.CreateBillingGroupMember(ctx, db.CreateBillingGroupMemberParams{
		BillingGroupID: billingGroup.ID,
		UserID:         authPayload.UserID,
		Role:           "member",
	}); err != nil && db.ErrorCode(err) != db.UniqueViolation {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newBillingGroupResponse(billingGroup))
}

// listBillingGroups 列出会话下的账单组
// @Summary 账单组列表
// @Tags 账单组
// @Produce json
// @Param dining_session_id query int true "用餐会话ID"
// @Success 200 {object} billingGroupListResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/billing-groups [get]
func (server *Server) listBillingGroups(ctx *gin.Context) {
	var req listBillingGroupsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	session, err := server.store.GetDiningSession(ctx, req.DiningSessionID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("dining session not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if session.UserID != authPayload.UserID {
		defaultGroup, err := server.store.GetDefaultBillingGroupBySession(ctx, session.ID)
		if err != nil {
			if isNotFoundError(err) {
				ctx.JSON(http.StatusForbidden, errorResponse(errors.New("access denied")))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		if _, err := server.store.GetActiveBillingGroupMember(ctx, db.GetActiveBillingGroupMemberParams{
			BillingGroupID: defaultGroup.ID,
			UserID:         authPayload.UserID,
		}); err != nil {
			if isNotFoundError(err) {
				ctx.JSON(http.StatusForbidden, errorResponse(errors.New("access denied")))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}

	groups, err := server.store.ListBillingGroupsBySession(ctx, req.DiningSessionID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := billingGroupListResponse{BillingGroups: make([]billingGroupResponse, 0, len(groups))}
	for _, g := range groups {
		resp.BillingGroups = append(resp.BillingGroups, newBillingGroupResponse(g))
	}
	resp.Total = int64(len(resp.BillingGroups))

	ctx.JSON(http.StatusOK, resp)
}

// listBillingGroupOrders 列出账单组订单
// @Summary 账单组订单列表
// @Tags 账单组
// @Produce json
// @Param id path int true "账单组ID"
// @Success 200 {object} billingGroupOrderListResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /v1/billing-groups/{id}/orders [get]
func (server *Server) listBillingGroupOrders(ctx *gin.Context) {
	var req billingGroupURIRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	billingGroup, err := server.store.GetBillingGroup(ctx, req.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("billing group not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	session, err := server.store.GetDiningSession(ctx, billingGroup.DiningSessionID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("dining session not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if session.UserID != authPayload.UserID {
		if _, err := server.store.GetActiveBillingGroupMember(ctx, db.GetActiveBillingGroupMemberParams{
			BillingGroupID: billingGroup.ID,
			UserID:         authPayload.UserID,
		}); err != nil {
			if isNotFoundError(err) {
				ctx.JSON(http.StatusForbidden, errorResponse(errors.New("access denied")))
				return
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}

	orders, err := server.store.ListBillingGroupOrdersByGroup(ctx, req.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := billingGroupOrderListResponse{Orders: make([]billingGroupOrderResponse, 0, len(orders))}
	for _, o := range orders {
		resp.Orders = append(resp.Orders, newBillingGroupOrderResponse(o))
	}
	resp.Total = int64(len(resp.Orders))

	ctx.JSON(http.StatusOK, resp)
}
