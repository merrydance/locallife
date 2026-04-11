package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
)

type merchantUserRiskResponse struct {
	UserID       int64   `json:"user_id"`
	HasBlock     bool    `json:"has_block"`
	ReasonCode   *string `json:"reason_code,omitempty"`
	BlockUntil   *string `json:"block_until,omitempty"`
	ReminderText *string `json:"reminder_text,omitempty"`
}

// getMerchantUserRisk 查询顾客风险提示（商户后台）
// @Summary 查询顾客风险提示
// @Description 商户查看顾客是否存在异常索赔记录，返回提示信息
// @Tags 商户风控
// @Accept json
// @Produce json
// @Param id path int true "用户ID"
// @Success 200 {object} merchantUserRiskResponse
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "非商户用户"
// @Failure 500 {object} ErrorResponse "内部错误"
// @Router /v1/merchant/risk/users/{id} [get]
// @Security BearerAuth
func (server *Server) getMerchantUserRisk(ctx *gin.Context) {
	userIDStr := ctx.Param("id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid user id")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	if _, err := server.getMerchantFromUserID(ctx, authPayload.UserID); err != nil {
		return
	}

	block, err := server.store.GetActiveBehaviorBlocklist(ctx, db.GetActiveBehaviorBlocklistParams{
		EntityType: "user",
		EntityID:   userID,
	})
	if err != nil {
		if isNotFoundError(err) || errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusOK, merchantUserRiskResponse{UserID: userID, HasBlock: false})
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	var blockUntil *string
	if block.BlockUntil.Valid {
		value := block.BlockUntil.Time.Format(time.RFC3339)
		blockUntil = &value
	}
	reminder := "该顾客有多次恶意索赔记录，谨慎服务"

	ctx.JSON(http.StatusOK, merchantUserRiskResponse{
		UserID:       userID,
		HasBlock:     true,
		ReasonCode:   &block.ReasonCode,
		BlockUntil:   blockUntil,
		ReminderText: &reminder,
	})
}
