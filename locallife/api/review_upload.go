package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/wechat"
)

// uploadReviewImage godoc
// @Summary 上传评价图片
// @Description 上传用户评价所需图片（上传前微信图片安全检测通过才会落盘）
// @Tags 评价管理
// @Accept multipart/form-data
// @Produce json
// @Param image formData file true "图片文件"
// @Success 200 {object} uploadImageResponse "上传成功"
// @Failure 400 {object} ErrorResponse "请求参数错误或内容安全检测未通过"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 502 {object} ErrorResponse "微信接口异常"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/reviews/images/upload [post]
// @Security BearerAuth
func (server *Server) uploadReviewImage(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	file, header, err := ctx.Request.FormFile("image")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("failed to get file: %w", err)))
		return
	}
	defer file.Close()

	// 上传前做图片内容安全检测：不通过则不落库/不落盘
	if err := server.wechatClient.ImgSecCheck(ctx, file); err != nil {
		if errors.Is(err, wechat.ErrRiskyContent) {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("图片内容安全检测未通过")))
			return
		}

		// 在开发环境下，将更详细的微信错误原因透传给前端，方便排查 502/412 等问题。
		errMsg := "微信图片安全检测服务异常"
		if server.config.Environment == "development" {
			errMsg = fmt.Sprintf("微信图片安全检测失败: %v", err)
		}
		ctx.JSON(http.StatusBadGateway, errorResponse(errors.New(errMsg)))

		// 同时记录详细日志
		internalError(ctx, fmt.Errorf("wechat img sec check: %w", err))
		return
	}
	if _, err := file.Seek(0, 0); err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	uploader := util.NewFileUploader("uploads")
	relativePath, err := uploader.UploadReviewImage(authPayload.UserID, file, header)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	ctx.JSON(http.StatusOK, uploadImageResponse{ImageURL: normalizeUploadURLForClient(relativePath)})
}
