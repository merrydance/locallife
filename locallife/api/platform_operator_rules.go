package api

import (
	"encoding/json"
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
	riderDepositConfigKey                   = "platform_rule.rider_deposit_fen"
	platformOperationalConfigsSuccessorPath = "/v1/platform/operational-configs"
)

type depositConfigValue struct {
	AmountFen int64 `json:"amount_fen"`
}

func (server *Server) getGlobalDepositFen(ctx *gin.Context, configKey string) (int64, bool, error) {
	config, err := server.store.GetPlatformConfig(ctx, db.GetPlatformConfigParams{
		ConfigKey: configKey,
		ScopeType: db.PlatformConfigScopeGlobal,
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
		ScopeType:   db.PlatformConfigScopeGlobal,
		ScopeID:     pgtype.Int8{Valid: false},
	})
	return err
}

func markPlatformOperatorRulesDeprecated(ctx *gin.Context) {
	ctx.Header("Deprecation", "true")
	ctx.Header("Link", "<"+platformOperationalConfigsSuccessorPath+">; rel=\"successor-version\"")
	ctx.Header("X-Deprecated-Route", "/v1/platform/operator-rules")
}

// listPlatformOperationalConfigs 获取平台运营配置列表
// @Summary 获取平台运营配置列表
// @Description 获取平台维护的运营真实配置项，包括平台佣金、运营商佣金、骑手押金与运费默认值。
// @Tags Platform
// @Produce json
// @Security BearerAuth
// @Success 200 {object} listPlatformOperatorRulesResponse "配置列表"
// @Failure 401 {object} errorRes "未授权"
// @Failure 403 {object} errorRes "权限不足"
// @Failure 500 {object} errorRes "服务器内部错误"
// @Router /v1/platform/operational-configs [get]
func (server *Server) listPlatformOperationalConfigs(ctx *gin.Context) {
	server.renderPlatformOperationalConfigs(ctx)
}

// listPlatformOperatorRules 获取平台运营配置列表（兼容旧路径）
// @Summary [Deprecated] 获取平台运营配置列表（兼容路径）
// @Description 获取平台维护的运营真实配置项，包括平台佣金、运营商佣金、骑手押金与运费默认值。请迁移到 /v1/platform/operational-configs。
// @Tags Platform
// @Produce json
// @Security BearerAuth
// @Success 200 {object} listPlatformOperatorRulesResponse "配置列表"
// @Failure 401 {object} errorRes "未授权"
// @Failure 403 {object} errorRes "权限不足"
// @Failure 500 {object} errorRes "服务器内部错误"
// @Router /v1/platform/operator-rules [get]
func (server *Server) listPlatformOperatorRules(ctx *gin.Context) {
	markPlatformOperatorRulesDeprecated(ctx)
	server.renderPlatformOperationalConfigs(ctx)
}

func (server *Server) renderPlatformOperationalConfigs(ctx *gin.Context) {
	platformRate := int32(2)
	operatorRate := int32(3)
	riderDeposit := int64(db.DefaultRiderDepositThresholdFen)
	deliveryFeeDefault := defaultDeliveryFeeConfig()

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

	if configuredRiderDeposit, ok, cfgErr := server.getGlobalDepositFen(ctx, riderDepositConfigKey); cfgErr != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, cfgErr))
		return
	} else if ok {
		riderDeposit = configuredRiderDeposit
	}

	if configuredDeliveryFee, ok, cfgErr := server.getGlobalDeliveryFeeDefaultConfig(ctx); cfgErr != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, cfgErr))
		return
	} else if ok {
		deliveryFeeDefault = configuredDeliveryFee
	}

	deliveryValueRatio := DefaultValueRatio * 100
	if deliveryFeeDefault.ValueRatio.Valid {
		if value, err := deliveryFeeDefault.ValueRatio.Float64Value(); err == nil {
			deliveryValueRatio = value.Float64 * 100
		}
	}

	deliveryMaxFee := "不限"
	if deliveryFeeDefault.MaxFee.Valid {
		deliveryMaxFee = fenToYuanString(deliveryFeeDefault.MaxFee.Int64, 2)
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
			ID:       "platform_rule_3",
			Name:     "骑手入驻押金",
			Key:      "RIDER_DEPOSIT",
			Value:    fenToYuanString(riderDeposit, 2),
			Unit:     "元",
			Desc:     "平台默认值；仅在运营商未单独配置时生效",
			Category: "platform",
			Editable: true,
		},
		{
			ID:       "platform_rule_4",
			Name:     "基础运费",
			Key:      "BASE_DELIVERY_FEE",
			Value:    fenToYuanString(deliveryFeeDefault.BaseFee, 2),
			Unit:     "元",
			Desc:     "平台默认值；仅在区域未配置运费时生效",
			Category: "delivery",
			Editable: true,
		},
		{
			ID:       "platform_rule_5",
			Name:     "基础距离",
			Key:      "BASE_DISTANCE",
			Value:    strconv.FormatInt(int64(deliveryFeeDefault.BaseDistance), 10),
			Unit:     "米",
			Desc:     "平台默认值；仅在区域未配置运费时生效",
			Category: "delivery",
			Editable: true,
		},
		{
			ID:       "platform_rule_6",
			Name:     "超距加价",
			Key:      "EXTRA_FEE_PER_KM",
			Value:    fenToYuanString(deliveryFeeDefault.ExtraFeePerKm, 2),
			Unit:     "元/km",
			Desc:     "平台默认值；仅在区域未配置运费时生效",
			Category: "delivery",
			Editable: true,
		},
		{
			ID:       "platform_rule_7",
			Name:     "最低运费",
			Key:      "MIN_DELIVERY_FEE",
			Value:    fenToYuanString(deliveryFeeDefault.MinFee, 2),
			Unit:     "元",
			Desc:     "平台默认值；仅在区域未配置运费时生效",
			Category: "delivery",
			Editable: true,
		},
		{
			ID:       "platform_rule_8",
			Name:     "最高运费",
			Key:      "MAX_DELIVERY_FEE",
			Value:    deliveryMaxFee,
			Unit:     "元",
			Desc:     "平台默认值；仅在区域未配置运费时生效",
			Category: "delivery",
			Editable: true,
		},
		{
			ID:       "platform_rule_9",
			Name:     "货值费率",
			Key:      "DELIVERY_VALUE_RATIO",
			Value:    strconv.FormatFloat(deliveryValueRatio, 'f', 2, 64),
			Unit:     "%",
			Desc:     "平台默认值；仅在区域未配置运费时生效",
			Category: "delivery",
			Editable: true,
		},
	}

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

// updatePlatformOperationalConfig 更新平台运营配置项
// @Summary 更新平台运营配置项
// @Description 更新平台维护的运营真实配置项。
// @Tags Platform
// @Accept json
// @Produce json
// @Param key path string true "配置Key (PLATFORM_COMMISSION, OPERATOR_COMMISSION, RIDER_DEPOSIT, BASE_DELIVERY_FEE, BASE_DISTANCE, EXTRA_FEE_PER_KM, MIN_DELIVERY_FEE, MAX_DELIVERY_FEE, DELIVERY_VALUE_RATIO)"
// @Param request body updatePlatformOperatorRuleRequest true "新值"
// @Security BearerAuth
// @Success 200 {object} MessageResponse "更新成功"
// @Failure 400 {object} errorRes "请求参数错误"
// @Failure 401 {object} errorRes "未授权"
// @Failure 403 {object} errorRes "权限不足"
// @Failure 404 {object} errorRes "配置不存在"
// @Failure 500 {object} errorRes "服务器内部错误"
// @Router /v1/platform/operational-configs/{key} [patch]
func (server *Server) updatePlatformOperationalConfig(ctx *gin.Context) {
	server.applyPlatformOperationalConfigUpdate(ctx)
}

// updatePlatformOperatorRule 更新平台运营配置项（兼容旧路径）
// @Summary [Deprecated] 更新平台运营配置项（兼容路径）
// @Description 更新平台维护的运营真实配置项。请迁移到 /v1/platform/operational-configs/{key}。
// @Tags Platform
// @Accept json
// @Produce json
// @Param key path string true "配置Key (PLATFORM_COMMISSION, OPERATOR_COMMISSION, RIDER_DEPOSIT, BASE_DELIVERY_FEE, BASE_DISTANCE, EXTRA_FEE_PER_KM, MIN_DELIVERY_FEE, MAX_DELIVERY_FEE, DELIVERY_VALUE_RATIO)"
// @Param request body updatePlatformOperatorRuleRequest true "新值"
// @Security BearerAuth
// @Success 200 {object} MessageResponse "更新成功"
// @Failure 400 {object} errorRes "请求参数错误"
// @Failure 401 {object} errorRes "未授权"
// @Failure 403 {object} errorRes "权限不足"
// @Failure 404 {object} errorRes "配置不存在"
// @Failure 500 {object} errorRes "服务器内部错误"
// @Router /v1/platform/operator-rules/{key} [patch]
func (server *Server) updatePlatformOperatorRule(ctx *gin.Context) {
	markPlatformOperatorRulesDeprecated(ctx)
	server.applyPlatformOperationalConfigUpdate(ctx)
}

func (server *Server) applyPlatformOperationalConfigUpdate(ctx *gin.Context) {
	key := ctx.Param("key")

	var req updatePlatformOperatorRuleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	value := 0.0
	if !(key == "MAX_DELIVERY_FEE" && req.Value == "不限") {
		parsedValue, err := strconv.ParseFloat(req.Value, 64)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrInvalidNumberFormat))
			return
		}
		value = parsedValue
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	switch key {
	case "PLATFORM_COMMISSION":
		if value < 0 || value > 100 {
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrRatioOutOfRange))
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
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrProfitShareExceedsLimit))
			return
		}
		if err := server.upsertGlobalProfitSharingConfig(ctx, int32(value), operatorRate, authPayload.UserID); err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

	case "OPERATOR_COMMISSION":
		if value < 0 || value > 100 {
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrRatioOutOfRange))
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
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrProfitShareExceedsLimit))
			return
		}
		if err := server.upsertGlobalProfitSharingConfig(ctx, platformRate, int32(value), authPayload.UserID); err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

	case "RIDER_DEPOSIT":
		if value < 0 {
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrAmountNegative))
			return
		}

		amountFen := yuanToFen(value)
		if err := server.upsertGlobalDepositFen(ctx, riderDepositConfigKey, amountFen); err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		if err := server.syncAllRiderOperationalStatuses(ctx); err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

	case "BASE_DELIVERY_FEE", "BASE_DISTANCE", "EXTRA_FEE_PER_KM", "MIN_DELIVERY_FEE", "MAX_DELIVERY_FEE", "DELIVERY_VALUE_RATIO":
		config, ok, cfgErr := server.getGlobalDeliveryFeeDefaultConfig(ctx)
		if cfgErr != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, cfgErr))
			return
		}
		if !ok {
			config = defaultDeliveryFeeConfig()
		}

		switch key {
		case "BASE_DELIVERY_FEE":
			if value < 0 {
				ctx.JSON(http.StatusBadRequest, errorResponse(ErrAmountNegative))
				return
			}
			config.BaseFee = yuanToFen(value)
		case "BASE_DISTANCE":
			if value < 0 {
				ctx.JSON(http.StatusBadRequest, errorResponse(ErrDistanceNegative))
				return
			}
			config.BaseDistance = int32(value)
		case "EXTRA_FEE_PER_KM":
			if value < 0 {
				ctx.JSON(http.StatusBadRequest, errorResponse(ErrAmountNegative))
				return
			}
			config.ExtraFeePerKm = yuanToFen(value)
		case "MIN_DELIVERY_FEE":
			if value < 0 {
				ctx.JSON(http.StatusBadRequest, errorResponse(ErrAmountNegative))
				return
			}
			config.MinFee = yuanToFen(value)
		case "MAX_DELIVERY_FEE":
			if req.Value == "不限" || value <= 0 {
				config.MaxFee = pgtype.Int8{Valid: false}
			} else {
				config.MaxFee = pgtype.Int8{Int64: yuanToFen(value), Valid: true}
			}
		case "DELIVERY_VALUE_RATIO":
			if value < 0 || value > 100 {
				ctx.JSON(http.StatusBadRequest, errorResponse(ErrValueRateOutOfRange))
				return
			}
			config.ValueRatio = numericFromFloat(value / 100.0)
		}

		var maxFee *int64
		if config.MaxFee.Valid {
			currentMaxFee := config.MaxFee.Int64
			maxFee = &currentMaxFee
		}
		if err := validateMinMaxFee(config.MinFee, maxFee); err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}

		if err := server.upsertGlobalDeliveryFeeDefaultConfig(ctx, config); err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

	default:
		ctx.JSON(http.StatusNotFound, errorResponse(ErrUnknownRuleKey))
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
