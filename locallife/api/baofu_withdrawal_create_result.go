package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/merrydance/locallife/logic"
)

func (server *Server) respondBaofuWithdrawalCreateResultError(ctx *gin.Context, result logic.BaofuCreateWithdrawalResult, err error) bool {
	switch {
	case errors.Is(err, logic.ErrBaofuWithdrawCreateRejected):
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("提现申请未被受理，请刷新余额后重试")))
		return true
	case errors.Is(err, logic.ErrBaofuWithdrawCreateResultUnknown) && result.WithdrawalOrder.ID != 0:
		item := newBaofuWithdrawalItem(result.WithdrawalOrder)
		item.SyncState = "unknown"
		item.SyncMessage = "提现申请已提交，结果正在确认，请勿重复提交"
		ctx.JSON(http.StatusAccepted, baofuWithdrawalCreateResponse{
			Withdrawal: item,
			Message:    "提现申请已提交，结果正在确认，请勿重复提交",
		})
		return true
	default:
		return false
	}
}
