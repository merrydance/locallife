package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// uploadDishImage godoc
// @Summary [Deprecated] 上传菜品图片
// @Description **已下线**。请改用媒体上传三步流程：POST /v1/media/upload-sessions -> 直传 OSS -> POST /v1/media/complete，然后将 media_asset_id 提交至菜品接口。
// @Tags 菜品管理
// @Produce json
// @Success 410 {object} ErrorResponse "接口已停用"
// @Router /v1/dishes/images/upload [post]
// @Security BearerAuth
func (server *Server) uploadDishImage(ctx *gin.Context) {
	ctx.JSON(http.StatusGone, errorResponse(errors.New(
		"此接口已停用。请改用媒体上传接口：POST /v1/media/upload-sessions",
	)))
}
