package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
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

const (
	merchantDepositConfigKey = "platform_rule.merchant_deposit_fen"
	riderDepositConfigKey    = "platform_rule.rider_deposit_fen"
)

type depositConfigValue struct {
	AmountFen int64 `json:"amount_fen"`
}

func (server *Server) getGlobalDepositFen(ctx *gin.Context, configKey string) (int64, bool, error) {
	config, err := server.store.GetPlatformConfig(ctx, db.GetPlatformConfigParams{
		ConfigKey: configKey,
		ScopeType: "global",
		ScopeID:   pgtype.Int8{Valid: false},
	})
	if err != nil {
		if isNotFoundError(err) {
			return 0, false, nil
		}
		return 0, false, err
	}

	if len(config.ConfigValue) == 0 {
		return 0, false, nil
	}

	var payload depositConfigValue
	if err := json.Unmarshal(config.ConfigValue, &payload); err != nil {
		return 0, false, err
	}

	return payload.AmountFen, true, nil
}

func (server *Server) upsertGlobalDepositFen(ctx *gin.Context, configKey string, amountFen int64) error {
	payload, err := json.Marshal(depositConfigValue{AmountFen: amountFen})
	if err != nil {
		return err
	}

	_, err = server.store.UpsertPlatformConfig(ctx, db.UpsertPlatformConfigParams{
		ConfigKey:   configKey,
		ConfigValue: payload,
		ScopeType:   "global",
		ScopeID:     pgtype.Int8{Valid: false},
	})
	return err
}

func (server *Server) listPlatformOperatorRules(ctx *gin.Context) {
	platformRate := int32(2)
	operatorRate := int32(3)
	merchantDeposit := int64(500000)
	riderDeposit := int64(20000)
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	if config, err := server.store.GetActiveProfitSharingConfig(ctx, db.GetActiveProfitSharingConfigParams{
		OrderSource: "takeout",
		MerchantID:  pgtype.Int8{Valid: false},
		RegionID:    pgtype.Int8{Valid: false},
	}); err == nil {
		platformRate = config.PlatformRate
		operatorRate = config.OperatorRate
	} else if !isNotFoundError(err) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	baseline, err := server.store.GetPlatformOperatorRuleBaselineFromRegion(ctx)
	if err == nil {
		merchantDeposit = baseline.MerchantDeposit
		riderDeposit = baseline.RiderDeposit
	} else if !isNotFoundError(err) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	} else {
		fallback, fallbackErr := server.store.GetPlatformOperatorRuleBaselineFromOperator(ctx)
		if fallbackErr == nil {
			merchantDeposit = fallback.MerchantDeposit
			riderDeposit = fallback.RiderDeposit
		} else if !isNotFoundError(fallbackErr) {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fallbackErr))
			return
		}
	}

	if configuredMerchantDeposit, ok, cfgErr := server.getGlobalDepositFen(ctx, merchantDepositConfigKey); cfgErr != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, cfgErr))
		return
	} else if ok {
		merchantDeposit = configuredMerchantDeposit
	}

	if configuredRiderDeposit, ok, cfgErr := server.getGlobalDepositFen(ctx, riderDepositConfigKey); cfgErr != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, cfgErr))
		return
	} else if ok {
		riderDeposit = configuredRiderDeposit
	}

	rules := []platformOperatorRuleItem{
		{
			ID:       "platform_rule_1",
			Name:     "平台佣金比例",
			Key:      "PLATFORM_COMMISSION",
			Value:    strconv.FormatFloat(float64(platformRate), 'f', 2, 64),
			Unit:     "%",
			Desc:     "平台佣金比例（分账配置来源）",
			Category: "platform",
			Editable: true,
		},
		{
			ID:       "platform_rule_1_1",
			Name:     "运营商佣金比例",
			Key:      "OPERATOR_COMMISSION",
			Value:    strconv.FormatFloat(float64(operatorRate), 'f', 2, 64),
			Unit:     "%",
			Desc:     "运营商佣金比例（分账配置来源）",
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

	_ = authPayload

	ctx.JSON(http.StatusOK, listPlatformOperatorRulesResponse{Rules: rules})
}

func (server *Server) upsertGlobalProfitSharingConfig(ctx *gin.Context, platformRate, operatorRate int32, actorID int64) error {
	configs, err := server.store.ListProfitSharingConfigs(ctx, db.ListProfitSharingConfigsParams{
		Column1: "active",
		Column2: "",
		Column3: 0,
		Column4: 0,
		Limit:   200,
		Offset:  0,
	})
	if err != nil {
		return err
	}

	updated := false
	for _, cfg := range configs {
		if cfg.MerchantID.Valid || cfg.RegionID.Valid {
			continue
		}
		if cfg.OrderSource != "all" {
			continue
		}
		_, err := server.store.UpdateProfitSharingConfigTx(ctx, db.UpdateProfitSharingConfigTxParams{
			ActorID:   actorID,
			ActorRole: RoleAdmin,
			Params: db.UpdateProfitSharingConfigParams{
				ID:           cfg.ID,
				PlatformRate: pgtype.Int4{Int32: platformRate, Valid: true},
				OperatorRate: pgtype.Int4{Int32: operatorRate, Valid: true},
			},
		})
		if err != nil {
			return err
		}
		updated = true
	}

	if updated {
		return nil
	}

	_, err = server.store.CreateProfitSharingConfigTx(ctx, db.CreateProfitSharingConfigTxParams{
		ActorID:   actorID,
		ActorRole: RoleAdmin,
		Params: db.CreateProfitSharingConfigParams{
			Status:       "active",
			OrderSource:  "all",
			PlatformRate: platformRate,
			OperatorRate: operatorRate,
			RiderEnabled: true,
			Priority:     100,
			CreatedBy:    pgtype.Int8{Int64: actorID, Valid: true},
		},
	})
	return err
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

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	switch key {
	case "PLATFORM_COMMISSION":
		if value < 0 || value > 100 {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("比例必须在 0-100 之间")))
			return
		}
		current, currentErr := server.store.GetActiveProfitSharingConfig(ctx, db.GetActiveProfitSharingConfigParams{
			OrderSource: "takeout",
			MerchantID:  pgtype.Int8{Valid: false},
			RegionID:    pgtype.Int8{Valid: false},
		})
		operatorRate := int32(3)
		if currentErr == nil {
			operatorRate = current.OperatorRate
		} else if !isNotFoundError(currentErr) {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, currentErr))
			return
		}
		if value+float64(operatorRate) > 100 {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("平台比例与运营商比例之和不能超过100")))
			return
		}
		if err := server.upsertGlobalProfitSharingConfig(ctx, int32(value), operatorRate, authPayload.UserID); err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

	case "OPERATOR_COMMISSION":
		if value < 0 || value > 100 {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("比例必须在 0-100 之间")))
			return
		}
		current, currentErr := server.store.GetActiveProfitSharingConfig(ctx, db.GetActiveProfitSharingConfigParams{
			OrderSource: "takeout",
			MerchantID:  pgtype.Int8{Valid: false},
			RegionID:    pgtype.Int8{Valid: false},
		})
		platformRate := int32(2)
		if currentErr == nil {
			platformRate = current.PlatformRate
		} else if !isNotFoundError(currentErr) {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, currentErr))
			return
		}
		if float64(platformRate)+value > 100 {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("平台比例与运营商比例之和不能超过100")))
			return
		}
		if err := server.upsertGlobalProfitSharingConfig(ctx, platformRate, int32(value), authPayload.UserID); err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

	case "MERCHANT_DEPOSIT":
		if value < 0 {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("金额不能为负数")))
			return
		}

		amountFen := yuanToFen(value)
		if err := server.upsertGlobalDepositFen(ctx, merchantDepositConfigKey, amountFen); err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
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
		if err := server.upsertGlobalDepositFen(ctx, riderDepositConfigKey, amountFen); err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
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
