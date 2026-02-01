package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
)

type withdrawOperatorRequest struct {
	Amount int64 `json:"amount" binding:"required,min=100"` // 最小提现1元 (100分)
}

// withdrawOperator 运营商提现
// @Summary 运营商提现
// @Description 运营商申请提现到绑定的微信支付账户
// @Tags 运营商财务
// @Accept json
// @Produce json
// @Param request body withdrawOperatorRequest true "提现请求"
// @Success 200 {object} MessageResponse "提现申请提交成功"
// @Failure 400 {object} ErrorResponse "参数错误或余额不足"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Security BearerAuth
// @Router /v1/operator/finance/withdraw [post]
func (server *Server) withdrawOperator(ctx *gin.Context) {
	var req withdrawOperatorRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 1. 获取运营商ID
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	userID := authPayload.UserID

	// 2. 验证运营商状态
	// 注意: 实际项目中应将 getOperatorFromUserID 优化为只需一次查询即可获取所需状态和余额
	// 这里假设 getOperatorFromUserID 已经检查了基本存在性
	operator, err := server.getOperatorFromUserID(ctx, userID)
	if err != nil {
		// getOperatorFromUserID 内部已经处理了错误响应
		return
	}

	if operator.Status != "active" {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("operator is not active")))
		return
	}
	// 3. 检查是否绑定了提现账户
	if len(operator.WalletAccount) == 0 || string(operator.WalletAccount) == "{}" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("wallet account not bound")))
		return
	}

	// 4. 执行提现事务 (包含检查余额、扣减余额、创建记录)
	txArg := db.WithdrawOperatorTxParams{
		OperatorID: operator.ID,
		Amount:     req.Amount,
		Channel:    "wechat",
	}

	_, err = server.store.WithdrawOperatorTx(ctx, txArg)
	if err != nil {
		if errors.Is(err, db.ErrInsufficientBalance) {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}

		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	ctx.JSON(http.StatusOK, MessageResponse{
		Message: "withdrawal request submitted successfully",
	})
}
