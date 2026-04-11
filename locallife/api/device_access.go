package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/merrydance/locallife/token"
)

type merchantDeviceAccessResponse struct {
	MerchantID   int64    `json:"merchant_id"`
	MerchantName string   `json:"merchant_name"`
	StaffRole    string   `json:"staff_role"`
	CanManage    bool     `json:"can_manage"`
	AllowedRoles []string `json:"allowed_roles"`
	BlockReason  string   `json:"block_reason,omitempty"`
}

var merchantDeviceManageAllowedRoles = []string{"owner", "manager"}

// getMerchantDeviceAccess 获取商户设备管理能力
// @Summary 获取商户设备管理能力
// @Description 返回当前商户关联用户是否可以管理打印设备与后厨协同配置，供小程序入口与页面守卫统一使用
// @Tags 商户设备管理
// @Accept json
// @Produce json
// @Success 200 {object} merchantDeviceAccessResponse "成功返回设备管理能力"
// @Failure 400 {object} ErrorResponse "商户选择错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "非商户或未关联商户"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /v1/merchant/devices/access [get]
func (server *Server) getMerchantDeviceAccess(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchant, staffRole, err := server.resolveMerchantStaffIdentity(ctx, authPayload.UserID)
	if err != nil {
		if isMerchantSelectionRequiredError(err) {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}
		if isNotFoundError(err) || err.Error() == "you are not a staff of this merchant" {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("you are not associated with any merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	response := merchantDeviceAccessResponse{
		MerchantID:   merchant.ID,
		MerchantName: merchant.Name,
		StaffRole:    staffRole,
		AllowedRoles: merchantDeviceManageAllowedRoles,
	}

	switch {
	case merchant.Status != "active" && merchant.Status != "approved":
		response.BlockReason = "商户账号当前不可用，暂时不能管理打印设备和后厨协同设置"
	case merchant.RegionID == 0:
		response.BlockReason = "商户区域尚未完成配置，暂时不能管理打印设备和后厨协同设置"
	case staffRole == "owner" || staffRole == "manager":
		response.CanManage = true
	default:
		response.BlockReason = "打印设备和后厨协同设置仅支持老板或店长管理"
	}

	ctx.JSON(http.StatusOK, response)
}
