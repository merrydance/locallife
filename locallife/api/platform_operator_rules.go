package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/merrydance/locallife/token"
)

type platformOperatorRuleItem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Key      string `json:"key"`
	Value    string `json:"value"`
	Unit     string `json:"unit"`
	Desc     string `json:"desc"`
	Category string `json:"category"`
	Editable bool   `json:"editable"`
}

type listPlatformOperatorRulesResponse struct {
	Rules []platformOperatorRuleItem `json:"rules"`
}

type updatePlatformOperatorRuleRequest struct {
	Value string `json:"value" binding:"required"`
}

func (server *Server) listPlatformOperatorRules(ctx *gin.Context) {
	commissionRate := numericFromFloat(0.03)
	merchantDeposit := int64(500000)
	riderDeposit := int64(20000)

	baseline, err := server.store.GetPlatformOperatorRuleBaselineFromRegion(ctx)
	if err == nil {
		commissionRate = baseline.CommissionRate
		merchantDeposit = baseline.MerchantDeposit
		riderDeposit = baseline.RiderDeposit
	} else if !isNotFoundError(err) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	} else {
		fallback, fallbackErr := server.store.GetPlatformOperatorRuleBaselineFromOperator(ctx)
		if fallbackErr == nil {
			commissionRate = fallback.CommissionRate
			merchantDeposit = fallback.MerchantDeposit
			riderDeposit = fallback.RiderDeposit
		} else if !isNotFoundError(fallbackErr) {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fallbackErr))
			return
		}
	}

	commissionPercent := pgNumericToFloat64(commissionRate) * 100
	rules := []platformOperatorRuleItem{
		{
			ID:       "platform_rule_1",
			Name:     "平台抽成比例",
			Key:      "PLATFORM_COMMISSION",
			Value:    strconv.FormatFloat(commissionPercent, 'f', 2, 64),
			Unit:     "%",
			Desc:     "平台对订单收取的服务费比例（全局生效）",
			Category: "platform",
			Editable: true,
		},
		{
			ID:       "platform_rule_2",
			Name:     "商户入驻保证金",
			Key:      "MERCHANT_DEPOSIT",
			Value:    fenToYuanString(merchantDeposit, 2),
			Unit:     "元",
			Desc:     "商户入驻需缴纳的保证金（全局生效）",
			Category: "platform",
			Editable: true,
		},
		{
			ID:       "platform_rule_3",
			Name:     "骑手入驻押金",
			Key:      "RIDER_DEPOSIT",
			Value:    fenToYuanString(riderDeposit, 2),
			Unit:     "元",
			Desc:     "骑手接单前需缴纳的押金（全局生效）",
			Category: "platform",
			Editable: true,
		},
	}

	ctx.JSON(http.StatusOK, listPlatformOperatorRulesResponse{Rules: rules})
}

func (server *Server) updatePlatformOperatorRule(ctx *gin.Context) {
	key := ctx.Param("key")

	var req updatePlatformOperatorRuleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	value, err := strconv.ParseFloat(req.Value, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("无效的数值格式")))
		return
	}

	switch key {
	case "PLATFORM_COMMISSION":
		if value < 0 || value > 100 {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("比例必须在 0-100 之间")))
			return
		}

		rate := numericFromFloat(value / 100.0)
		if err := server.store.UpdateAllRegionRuleConfigCommissionRate(ctx, rate); err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		if err := server.store.UpdateAllOperatorsCommissionRate(ctx, rate); err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

	case "MERCHANT_DEPOSIT":
		if value < 0 {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("金额不能为负数")))
			return
		}

		amountFen := yuanToFen(value)
		if err := server.store.UpdateAllRegionRuleConfigMerchantDeposit(ctx, amountFen); err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		if err := server.store.UpdateAllOperatorsMerchantDeposit(ctx, amountFen); err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

	case "RIDER_DEPOSIT":
		if value < 0 {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("金额不能为负数")))
			return
		}

		amountFen := yuanToFen(value)
		if err := server.store.UpdateAllRegionRuleConfigRiderDeposit(ctx, amountFen); err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		if err := server.store.UpdateAllOperatorsRiderDeposit(ctx, amountFen); err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

	default:
		ctx.JSON(http.StatusNotFound, errorResponse(errors.New("未知规则Key")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	server.writeAuditLog(ctx, AuditLogInput{
		ActorUserID: authPayload.UserID,
		ActorRole:   RoleAdmin,
		Action:      "platform_operator_rule_updated",
		TargetType:  "platform_rule",
		RegionID:    nil,
		Metadata: map[string]any{
			"key":   key,
			"value": req.Value,
		},
	})

	ctx.JSON(http.StatusOK, MessageResponse{Message: "规则更新成功"})
}
