package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
)

const baofuWithdrawalSubmittedSyncMessage = "提现申请已提交，结果正在确认，请勿重复提交"

func isBaofuWithdrawalCreateSubmitted(result logic.BaofuCreateWithdrawalResult) bool {
	return result.WithdrawalOrder.ID != 0 &&
		strings.TrimSpace(result.SyncState) == "unknown" &&
		result.WithdrawalOrder.Status == db.BaofuWithdrawalStatusProcessing
}

func (server *Server) respondBaofuWithdrawalCreateSubmitted(ctx *gin.Context, result logic.BaofuCreateWithdrawalResult) bool {
	if !isBaofuWithdrawalCreateSubmitted(result) {
		return false
	}
	message := strings.TrimSpace(result.UserMessage)
	if message == "" {
		message = baofuWithdrawalSubmittedSyncMessage
	}
	item := newBaofuWithdrawalItem(result.WithdrawalOrder)
	item.SyncState = "unknown"
	item.SyncMessage = message
	ctx.JSON(http.StatusAccepted, baofuWithdrawalCreateResponse{
		Withdrawal: item,
		Message:    message,
	})
	return true
}
