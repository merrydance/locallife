package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/merrydance/locallife/logic"
)

type platformBaofuSettlementStatusResponse struct {
	SettlementAccount  *baofuSettlementReadinessResponse `json:"settlement_account"`
	MaskedContractNo   string                            `json:"masked_contract_no,omitempty"`
	MaskedSharingMerID string                            `json:"masked_sharing_mer_id,omitempty"`
}

// getPlatformBaofuSettlementStatus 查询平台宝付结算账户状态
// @Summary 查询平台宝付结算账户状态
// @Description 管理员查询平台佣金接收方宝付二级户开通状态；响应只返回产品状态和脱敏账户标识，不暴露宝付账户号、分账接收方标识或上游原始数据
// @Tags 平台财务
// @Produce json
// @Success 200 {object} platformBaofuSettlementStatusResponse
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /v1/platform/finance/settlement-account/status [get]
func (server *Server) getPlatformBaofuSettlementStatus(ctx *gin.Context) {
	binding, found, err := server.getPlatformBaofuSettlementBinding(ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	service := logic.NewBaofuAccountService(nil, nil)
	readiness := service.ReadinessFromBinding(binding, found)
	ctx.JSON(http.StatusOK, platformBaofuSettlementStatusResponse{
		SettlementAccount:  newBaofuSettlementReadinessResponse(readiness),
		MaskedContractNo:   maskedBaofuIdentifier(binding.ContractNo.String),
		MaskedSharingMerID: maskedBaofuIdentifier(binding.SharingMerID.String),
	})
}

func maskedBaofuIdentifier(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 6 {
		if value == "" {
			return ""
		}
		return "***"
	}
	return value[:3] + "****" + value[len(value)-3:]
}
