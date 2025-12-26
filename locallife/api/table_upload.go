package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/wechat"
)

// uploadTableImage godoc
// @Summary 上传桌台/包间图片
// @Description 上传桌台/包间展示图片（上传前微信图片安全检测通过才会落盘）
// @Tags 桌台管理
// @Accept multipart/form-data
// @Produce json
// @Param image formData file true "图片文件"
// @Success 200 {object} uploadImageResponse "上传成功"
// @Failure 400 {object} ErrorResponse "请求参数错误或内容安全检测未通过"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "非商户"
// @Failure 502 {object} ErrorResponse "微信接口异常"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/tables/images/upload [post]
// @Security BearerAuth
func (server *Server) uploadTableImage(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get merchant by owner: %w", err)))
		return
	}

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
		ctx.JSON(http.StatusBadGateway, internalError(ctx, fmt.Errorf("wechat img sec check: %w", err)))
		return
	}
	if _, err := file.Seek(0, 0); err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	uploader := util.NewFileUploader("uploads")
	relativePath, err := uploader.UploadPublicMerchantAssetImage(merchant.ID, "tables", file, header)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	ctx.JSON(http.StatusOK, uploadImageResponse{ImageURL: normalizeUploadURLForClient(relativePath)})
}
