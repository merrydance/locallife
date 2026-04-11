package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// uploadReviewImage godoc
// @Summary [Deprecated] 上传评价图片
// @Description **已下线**。请改用媒体上传三步流程：POST /v1/media/upload-sessions -> 直传 OSS -> POST /v1/media/complete，然后将 media_asset_ids 提交至评价接口。
// @Tags 评价管理
// @Produce json
// @Success 410 {object} ErrorResponse "接口已停用"
// @Router /v1/reviews/images/upload [post]
// @Security BearerAuth
func (server *Server) uploadReviewImage(ctx *gin.Context) {
	ctx.JSON(http.StatusGone, errorResponse(errors.New(
		"此接口已停用。请改用媒体上传接口：POST /v1/media/upload-sessions",
	)))
}
