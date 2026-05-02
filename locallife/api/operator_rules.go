package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/rs/zerolog/log"
)

// ListRulesResponse 规则列表响应
type ListRulesResponse struct {
	Rules []RuleItem `json:"rules"`
}

// RuleItem 单个规则项
type RuleItem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Key      string `json:"key"`
	Value    string `json:"value"`
	Unit     string `json:"unit"`
	Desc     string `json:"desc"`
	Category string `json:"category"`
	Editable bool   `json:"editable"`
	Action   string `json:"action,omitempty"`
}

type operatorRulesQuery struct {
	RegionID int64 `form:"region_id" binding:"omitempty,gt=0"`
}

const (
	operatorRiderDepositConfigKey = "platform_rule.rider_deposit_fen"
)

type operatorDepositConfigValue struct {
	AmountFen int64 `json:"amount_fen"`
}

type weatherRuleValueSource string

const (
	weatherRuleValueSourceRegion   weatherRuleValueSource = "region"
	weatherRuleValueSourcePlatform weatherRuleValueSource = "platform"
	weatherRuleValueSourceDefault  weatherRuleValueSource = "default"
)

func (server *Server) getScopedDepositFen(ctx *gin.Context, configKey, scopeType string, scopeID pgtype.Int8) (int64, bool, error) {
	config, err := server.store.GetPlatformConfig(ctx, db.GetPlatformConfigParams{
		ConfigKey: configKey,
		ScopeType: scopeType,
		ScopeID:   scopeID,
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

	var payload operatorDepositConfigValue
	if err := json.Unmarshal(config.ConfigValue, &payload); err != nil {
		return 0, false, err
	}

	return payload.AmountFen, true, nil
}

func (server *Server) getOperatorGlobalDepositFen(ctx *gin.Context, configKey string) (int64, bool, error) {
	return server.getScopedDepositFen(ctx, configKey, db.PlatformConfigScopeGlobal, pgtype.Int8{Valid: false})
}

func (server *Server) getOperatorRiderDepositThreshold(ctx *gin.Context, regionRuleConfig *db.RegionRuleConfig) (int64, db.RiderDepositThresholdSource, error) {
	if regionRuleConfig != nil && regionRuleConfig.RiderDeposit > 0 {
		return regionRuleConfig.RiderDeposit, db.RiderDepositThresholdSourceRegion, nil
	}

	configured, ok, err := server.getOperatorGlobalDepositFen(ctx, operatorRiderDepositConfigKey)
	if err != nil {
		return 0, "", err
	}
	if ok && configured > 0 {
		return configured, db.RiderDepositThresholdSourcePlatform, nil
	}

	threshold, source := db.ResolveRiderDepositThreshold(0, false)
	return threshold, source, nil
}

func weatherRuleKeyToPlatformConfigKey(key string) (string, error) {
	switch key {
	case "WEATHER_COEFF_EXTREME":
		return "WEATHER_COEFF_EXTREME", nil
	case "WEATHER_COEFF_HEAVY":
		return "WEATHER_COEFF_HEAVY", nil
	case "WEATHER_COEFF_MODERATE":
		return "WEATHER_COEFF_MODERATE", nil
	case "WEATHER_COEFF_LIGHT":
		return "WEATHER_COEFF_LIGHT", nil
	default:
		return "", errors.New("unknown weather rule key")
	}
}

func defaultWeatherRuleValue(key string) float64 {
	switch key {
	case "WEATHER_COEFF_EXTREME":
		return 2.0
	case "WEATHER_COEFF_HEAVY":
		return 1.8
	case "WEATHER_COEFF_MODERATE":
		return 1.3
	case "WEATHER_COEFF_LIGHT":
		return 1.1
	default:
		return 1.0
	}
}

func regionWeatherRuleValue(config *db.RegionRuleConfig, key string) (float64, bool) {
	if config == nil {
		return 0, false
	}

	var value pgtype.Numeric
	switch key {
	case "WEATHER_COEFF_EXTREME":
		value = config.WeatherCoeffExtreme
	case "WEATHER_COEFF_HEAVY":
		value = config.WeatherCoeffHeavy
	case "WEATHER_COEFF_MODERATE":
		value = config.WeatherCoeffModerate
	case "WEATHER_COEFF_LIGHT":
		value = config.WeatherCoeffLight
	default:
		return 0, false
	}

	if !value.Valid {
		return 0, false
	}

	parsed, err := value.Float64Value()
	if err != nil {
		return 0, false
	}

	return parsed.Float64, true
}

func (server *Server) resolveWeatherRuleValue(ctx *gin.Context, regionID int64, regionRuleConfig *db.RegionRuleConfig, key string) (float64, weatherRuleValueSource, error) {
	if value, ok := regionWeatherRuleValue(regionRuleConfig, key); ok {
		return value, weatherRuleValueSourceRegion, nil
	}

	configKey, err := weatherRuleKeyToPlatformConfigKey(key)
	if err != nil {
		return 0, "", err
	}

	config, err := server.store.GetPlatformConfig(ctx, db.GetPlatformConfigParams{
		ConfigKey: configKey,
		ScopeType: "city",
		ScopeID:   pgtype.Int8{Int64: regionID, Valid: true},
	})
	if err == nil {
		var value float64
		if unmarshalErr := json.Unmarshal(config.ConfigValue, &value); unmarshalErr != nil {
			return 0, "", unmarshalErr
		}
		return value, weatherRuleValueSourcePlatform, nil
	}
	if !isNotFoundError(err) {
		return 0, "", err
	}

	return defaultWeatherRuleValue(key), weatherRuleValueSourceDefault, nil
}

func (server *Server) resolveOperatorRuleRegionID(ctx *gin.Context, operator db.Operator) (int64, error) {
	var query operatorRulesQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		return 0, err
	}

	if query.RegionID > 0 {
		if _, err := server.checkOperatorManagesRegion(ctx, query.RegionID); err != nil {
			return -1, err
		}
		return query.RegionID, nil
	}

	if operator.RegionID > 0 {
		return operator.RegionID, nil
	}

	return server.getOperatorRegionID(ctx)
}

// listOperatorRules 获取运营商规则配置
// @Summary 获取规则配置
// @Description 获取运营商相关的规则配置，如保证金、抽成比例等
// @Tags 运营商-规则管理
// @Accept json
// @Produce json
// @Success 200 {object} ListRulesResponse "规则列表"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /v1/operator/rules [get]
func (server *Server) listOperatorRules(ctx *gin.Context) {
	// 获取当前运营商
	operator, ok := GetOperatorFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("operator not found in context")))
		return
	}

	targetRegionID, err := server.resolveOperatorRuleRegionID(ctx, operator)
	if err != nil {
		if targetRegionID == -1 {
			ctx.JSON(http.StatusForbidden, errorResponse(err))
			return
		}
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("operator has no assigned region")))
			return
		}
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	rules := []RuleItem{}

	regionRuleConfig, configErr := server.store.GetRegionRuleConfigByRegion(ctx, targetRegionID)
	var regionRuleConfigPtr *db.RegionRuleConfig
	if configErr != nil && !isNotFoundError(configErr) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, configErr))
		return
	}
	if configErr == nil {
		regionRuleConfigPtr = &regionRuleConfig
	}
	riderDepositValue, riderDepositSource, err := server.getOperatorRiderDepositThreshold(ctx, regionRuleConfigPtr)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 运营商抽成比例直接从 profit_sharing_configs 读取，与平台侧保持同源
	operatorRateInt := int32(0)
	profitConfig, profitErr := server.store.GetActiveProfitSharingConfig(ctx, db.GetActiveProfitSharingConfigParams{
		OrderSource: "takeout",
		MerchantID:  pgtype.Int8{Valid: false},
		RegionID:    pgtype.Int8{Int64: targetRegionID, Valid: targetRegionID > 0},
	})
	if profitErr == nil {
		operatorRateInt = profitConfig.OperatorRate
	} else if !isNotFoundError(profitErr) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, profitErr))
		return
	}

	weatherExtreme, weatherExtremeSource, err := server.resolveWeatherRuleValue(ctx, targetRegionID, regionRuleConfigPtr, "WEATHER_COEFF_EXTREME")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	weatherHeavy, weatherHeavySource, err := server.resolveWeatherRuleValue(ctx, targetRegionID, regionRuleConfigPtr, "WEATHER_COEFF_HEAVY")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	weatherModerate, weatherModerateSource, err := server.resolveWeatherRuleValue(ctx, targetRegionID, regionRuleConfigPtr, "WEATHER_COEFF_MODERATE")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	weatherLight, weatherLightSource, err := server.resolveWeatherRuleValue(ctx, targetRegionID, regionRuleConfigPtr, "WEATHER_COEFF_LIGHT")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	weatherRuleDesc := func(base string, source weatherRuleValueSource) string {
		switch source {
		case weatherRuleValueSourcePlatform:
			return base + "；当前使用平台城市默认值"
		case weatherRuleValueSourceDefault:
			return base + "；当前使用系统默认值"
		default:
			return base + "；当前使用区域配置"
		}
	}

	// 1. 骑手入驻押金（区域优先，平台默认值兜底）
	riderDeposit := fenToYuanString(riderDepositValue, 2)
	riderDepositDesc := "当前区域配置，直接决定本区域骑手押金门槛"
	switch riderDepositSource {
	case db.RiderDepositThresholdSourcePlatform:
		riderDepositDesc = "当前使用平台默认值，运营商可单独修改覆盖"
	case db.RiderDepositThresholdSourceDefault:
		riderDepositDesc = "当前使用系统默认值，运营商可单独修改覆盖"
	}
	rules = append(rules, RuleItem{
		ID:       "rule_1",
		Name:     "骑手入驻押金",
		Key:      "RIDER_DEPOSIT",
		Value:    riderDeposit,
		Unit:     "元",
		Desc:     riderDepositDesc,
		Category: "delivery",
		Editable: true,
	})

	// 2. 平台抽成比例 (只读展示，平台维护)
	rules = append(rules, RuleItem{
		ID:       "rule_2",
		Name:     "运营商抽成比例",
		Key:      "PLATFORM_COMMISSION",
		Value:    fmt.Sprintf("%.1f", float64(operatorRateInt)),
		Unit:     "%",
		Desc:     "平台统一维护，运营商仅可查看",
		Category: "delivery",
		Editable: false,
	})

	// 获取运费配置
	feeConfig, deliveryFeeSource, err := server.resolveOperatorRuleDeliveryFeeConfig(ctx, targetRegionID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	deliveryFeeDesc := "当前区域运费配置，运营商可调整"
	switch deliveryFeeSource {
	case deliveryFeeConfigSourcePlatform:
		deliveryFeeDesc = "当前使用平台默认值；保存后会生成本区域独立配置"
	case deliveryFeeConfigSourceDefault:
		deliveryFeeDesc = "当前使用系统默认值；建议尽快补齐平台或区域配置"
	}

	// 3. 基础运费
	baseFee := fenToYuanString(feeConfig.BaseFee, 2)
	rules = append(rules, RuleItem{
		ID:       "rule_3",
		Name:     "基础运费",
		Key:      "BASE_DELIVERY_FEE",
		Value:    baseFee,
		Unit:     "元",
		Desc:     deliveryFeeDesc,
		Category: "delivery",
		Editable: true,
	})

	// 4. 基础距离
	rules = append(rules, RuleItem{
		ID:       "rule_4",
		Name:     "基础距离",
		Key:      "BASE_DISTANCE",
		Value:    fmt.Sprintf("%d", feeConfig.BaseDistance),
		Unit:     "米",
		Desc:     deliveryFeeDesc,
		Category: "delivery",
		Editable: true,
	})

	// 6. 超距加价
	extraFee := fenToYuanString(feeConfig.ExtraFeePerKm, 2)
	rules = append(rules, RuleItem{
		ID:       "rule_6",
		Name:     "超距加价",
		Key:      "EXTRA_FEE_PER_KM",
		Value:    extraFee,
		Unit:     "元/km",
		Desc:     deliveryFeeDesc,
		Category: "delivery",
		Editable: true,
	})

	// 8. 最低运费
	minFee := fenToYuanString(feeConfig.MinFee, 2)
	rules = append(rules, RuleItem{
		ID:       "rule_8",
		Name:     "最低运费",
		Key:      "MIN_DELIVERY_FEE",
		Value:    minFee,
		Unit:     "元",
		Desc:     deliveryFeeDesc,
		Category: "delivery",
		Editable: true,
	})

	// 9. 最高运费 (MaxFee is nullable)
	maxFeeVal := "不限"
	if feeConfig.MaxFee.Valid {
		maxFeeVal = fenToYuanString(feeConfig.MaxFee.Int64, 2)
	}
	rules = append(rules, RuleItem{
		ID:       "rule_9",
		Name:     "最高运费",
		Key:      "MAX_DELIVERY_FEE",
		Value:    maxFeeVal,
		Unit:     "元",
		Desc:     deliveryFeeDesc,
		Category: "delivery",
		Editable: true,
	})

	// 10. 货值费率
	valueRatio := pgNumericToFloat64(feeConfig.ValueRatio) * 100
	rules = append(rules, RuleItem{
		ID:       "rule_10",
		Name:     "货值费率",
		Key:      "DELIVERY_VALUE_RATIO",
		Value:    fmt.Sprintf("%.2f", valueRatio),
		Unit:     "%",
		Desc:     deliveryFeeDesc,
		Category: "delivery",
		Editable: true,
		Action:   "edit",
	})

	// 15. 时段系数配置入口（按区域管理）
	rules = append(rules, RuleItem{
		ID:       "rule_15",
		Name:     "时段系数配置",
		Key:      "PEAK_HOUR_COEFFICIENTS",
		Value:    "按区域配置",
		Unit:     "",
		Desc:     "支持按区域配置午高峰/晚高峰等时段系数",
		Category: "timeslot",
		Editable: true,
		Action:   "navigate_peak",
	})

	// 11. 恶劣天气加价倍数
	rules = append(rules, RuleItem{
		ID:       "rule_11",
		Name:     "极端天气倍数",
		Key:      "WEATHER_COEFF_EXTREME",
		Value:    fmt.Sprintf("%.2f", weatherExtreme),
		Unit:     "x",
		Desc:     weatherRuleDesc("台风/龙卷风等极端天气下的运费倍数", weatherExtremeSource),
		Category: "weather",
		Editable: true,
	})

	// 12. 暴雨/雪加价倍数
	rules = append(rules, RuleItem{
		ID:       "rule_12",
		Name:     "暴雨雪倍数",
		Key:      "WEATHER_COEFF_HEAVY",
		Value:    fmt.Sprintf("%.2f", weatherHeavy),
		Unit:     "x",
		Desc:     weatherRuleDesc("暴雨/暴雪/特大暴雨下的运费倍数", weatherHeavySource),
		Category: "weather",
		Editable: true,
	})

	// 13. 中雨/雪加价倍数
	rules = append(rules, RuleItem{
		ID:       "rule_13",
		Name:     "中雨雪倍数",
		Key:      "WEATHER_COEFF_MODERATE",
		Value:    fmt.Sprintf("%.2f", weatherModerate),
		Unit:     "x",
		Desc:     weatherRuleDesc("中雨/中雪/大雨/大雪下的运费倍数", weatherModerateSource),
		Category: "weather",
		Editable: true,
	})

	// 14. 小雨/雪加价倍数
	rules = append(rules, RuleItem{
		ID:       "rule_14",
		Name:     "小雨雪倍数",
		Key:      "WEATHER_COEFF_LIGHT",
		Value:    fmt.Sprintf("%.2f", weatherLight),
		Unit:     "x",
		Desc:     weatherRuleDesc("小雨/小雪下的运费倍数", weatherLightSource),
		Category: "weather",
		Editable: true,
	})

	// 获取最新天气系数
	weather, err := server.store.GetLatestWeatherCoefficient(ctx, targetRegionID)
	if err == nil {
		// 7. 当前天气加价系数
		coeff := pgNumericToFloat64(weather.FinalCoefficient)
		rules = append(rules, RuleItem{
			ID:       "rule_7",
			Name:     "天气加价系数",
			Key:      "WEATHER_COEFFICIENT", // 此字段通常由系统自动更新，但展示给运营商看
			Value:    fmt.Sprintf("%.2f", coeff),
			Unit:     "x",
			Desc:     fmt.Sprintf("当前天气：%s (系统自动更新)", weather.WeatherType),
			Category: "weather",
			Editable: false,
		})
	} else {
		// 暂无天气数据时显示默认值
		rules = append(rules, RuleItem{
			ID:       "rule_7",
			Name:     "天气加价系数",
			Key:      "WEATHER_COEFFICIENT",
			Value:    "1.00",
			Unit:     "x",
			Desc:     "当前天气：暂无数据 (系统自动更新)",
			Category: "weather",
			Editable: false,
		})
	}

	ctx.JSON(http.StatusOK, ListRulesResponse{Rules: rules})
}

type updateRuleRequest struct {
	Value string `json:"value" binding:"required"`
}

// updateOperatorRule 更新运营商规则配置
// @Summary 更新规则配置
// @Description 更新运营商规则配置，支持运费参数、天气系数与骑手押金；抽成、商户保证金与货值费率为平台维护只读项
// @Tags 运营商-规则管理
// @Accept json
// @Produce json
// @Param key path string true "规则Key (RIDER_DEPOSIT, BASE_DELIVERY_FEE, BASE_DISTANCE, EXTRA_FEE_PER_KM, MIN_DELIVERY_FEE, MAX_DELIVERY_FEE, DELIVERY_VALUE_RATIO, WEATHER_COEFF_EXTREME, WEATHER_COEFF_HEAVY, WEATHER_COEFF_MODERATE, WEATHER_COEFF_LIGHT)"
// @Param request body updateRuleRequest true "新值"
// @Success 200 {object} MessageResponse "更新成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Security BearerAuth
// @Router /v1/operator/rules/{key} [patch]
func (server *Server) updateOperatorRule(ctx *gin.Context) {
	key := ctx.Param("key")
	var req updateRuleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取当前运营商
	operator, ok := GetOperatorFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("operator not found in context")))
		return
	}

	targetRegionID, err := server.resolveOperatorRuleRegionID(ctx, operator)
	if err != nil {
		if targetRegionID == -1 {
			ctx.JSON(http.StatusForbidden, errorResponse(err))
			return
		}
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("operator has no assigned region")))
			return
		}
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	if key == "WEATHER_COEFFICIENT" {
		ctx.JSON(http.StatusForbidden, errorResponse(ErrWeatherCoefficientReadOnly))
		return
	}

	editableKeys := map[string]struct{}{
		"RIDER_DEPOSIT":          {},
		"BASE_DELIVERY_FEE":      {},
		"BASE_DISTANCE":          {},
		"EXTRA_FEE_PER_KM":       {},
		"MIN_DELIVERY_FEE":       {},
		"MAX_DELIVERY_FEE":       {},
		"DELIVERY_VALUE_RATIO":   {},
		"WEATHER_COEFF_EXTREME":  {},
		"WEATHER_COEFF_HEAVY":    {},
		"WEATHER_COEFF_MODERATE": {},
		"WEATHER_COEFF_LIGHT":    {},
	}

	if _, ok := editableKeys[key]; !ok {
		ctx.JSON(http.StatusForbidden, errorResponse(ErrRulePlatformOnly))
		return
	}

	auditMetadata := map[string]any{
		"key":       key,
		"value":     req.Value,
		"region_id": targetRegionID,
	}

	switch key {
	case "RIDER_DEPOSIT":
		val, err := strconv.ParseFloat(req.Value, 64)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrInvalidNumberFormat))
			return
		}
		if val < 0 {
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrAmountNegative))
			return
		}

		amountFen := yuanToFen(val)
		_, err = server.store.UpdateOperatorRules(ctx, db.UpdateOperatorRulesParams{
			ID:           operator.ID,
			RiderDeposit: pgtype.Int8{Int64: amountFen, Valid: true},
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		_, err = server.store.UpsertRegionRuleConfig(ctx, db.UpsertRegionRuleConfigParams{
			RegionID:     targetRegionID,
			RiderDeposit: pgtype.Int8{Int64: amountFen, Valid: true},
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		if err := server.syncRiderOperationalStatusesByRegion(ctx, targetRegionID); err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		auditMetadata["operator_id"] = operator.ID
		auditMetadata["value_fen"] = amountFen
		auditMetadata["region_rule_config"] = true

	case "BASE_DELIVERY_FEE", "BASE_DISTANCE", "EXTRA_FEE_PER_KM", "MIN_DELIVERY_FEE", "MAX_DELIVERY_FEE", "DELIVERY_VALUE_RATIO":
		// 1. 获取现有配置或初始化
		feeConfig, err := server.store.GetDeliveryFeeConfigByRegion(ctx, targetRegionID)
		if err != nil {
			if isNotFoundError(err) {
				seedConfig, _, resolveErr := server.resolveOperatorRuleDeliveryFeeConfig(ctx, targetRegionID)
				if resolveErr != nil {
					ctx.JSON(http.StatusInternalServerError, internalError(ctx, resolveErr))
					return
				}

				feeConfig, err = server.store.CreateDeliveryFeeConfig(ctx, db.CreateDeliveryFeeConfigParams{
					RegionID:      targetRegionID,
					BaseFee:       seedConfig.BaseFee,
					BaseDistance:  seedConfig.BaseDistance,
					ExtraFeePerKm: seedConfig.ExtraFeePerKm,
					ValueRatio:    seedConfig.ValueRatio,
					MaxFee:        seedConfig.MaxFee,
					MinFee:        seedConfig.MinFee,
					IsActive:      true,
				})
				if err != nil {
					ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
					return
				}
			} else {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
				return
			}
		}

		// 2. 准备更新参数
		arg := db.UpdateDeliveryFeeConfigParams{
			ID:       feeConfig.ID,
			IsActive: pgtype.Bool{Bool: true, Valid: true},
			// 其他字段保持原值（COALESCE in SQL）
		}

		// Handle "不限" logic for max fee if valid number or 0
		if req.Value == "不限" && key == "MAX_DELIVERY_FEE" {
			arg.MaxFee = pgtype.Int8{Valid: false}
			// Skip parsing float
		} else {
			val, err := strconv.ParseFloat(req.Value, 64)
			if err != nil {
				ctx.JSON(http.StatusBadRequest, errorResponse(ErrInvalidNumberFormat))
				return
			}

			switch key {
			case "BASE_DELIVERY_FEE":
				if val < 0 {
					ctx.JSON(http.StatusBadRequest, errorResponse(ErrAmountNegative))
					return
				}
				arg.BaseFee = pgtype.Int8{Int64: yuanToFen(val), Valid: true}
			case "BASE_DISTANCE":
				if val < 0 {
					ctx.JSON(http.StatusBadRequest, errorResponse(ErrDistanceNegative))
					return
				}
				arg.BaseDistance = pgtype.Int4{Int32: int32(val), Valid: true}
			case "EXTRA_FEE_PER_KM":
				if val < 0 {
					ctx.JSON(http.StatusBadRequest, errorResponse(ErrAmountNegative))
					return
				}
				arg.ExtraFeePerKm = pgtype.Int8{Int64: yuanToFen(val), Valid: true}
			case "MIN_DELIVERY_FEE":
				if val < 0 {
					ctx.JSON(http.StatusBadRequest, errorResponse(ErrAmountNegative))
					return
				}
				arg.MinFee = pgtype.Int8{Int64: yuanToFen(val), Valid: true}
			case "MAX_DELIVERY_FEE":
				if val <= 0 {
					// 0 or negative considered as no limit or clear limit?
					// Typical logic: 0 or -1 means no limit.
					// User might enter 0. Let's treat 0 as "No Limit" (NULL).
					arg.MaxFee = pgtype.Int8{Valid: false}
				} else {
					arg.MaxFee = pgtype.Int8{Int64: yuanToFen(val), Valid: true}
				}
			case "DELIVERY_VALUE_RATIO":
				if val < 0 || val > 100 {
					ctx.JSON(http.StatusBadRequest, errorResponse(ErrValueRateOutOfRange))
					return
				}
				arg.ValueRatio = numericFromFloat(val / 100.0)
			}
		}

		effectiveMinFee := feeConfig.MinFee
		if arg.MinFee.Valid {
			effectiveMinFee = arg.MinFee.Int64
		}

		var effectiveMaxFee *int64
		if feeConfig.MaxFee.Valid {
			currentMaxFee := feeConfig.MaxFee.Int64
			effectiveMaxFee = &currentMaxFee
		}
		if key == "MAX_DELIVERY_FEE" {
			if arg.MaxFee.Valid {
				updatedMaxFee := arg.MaxFee.Int64
				effectiveMaxFee = &updatedMaxFee
			} else {
				effectiveMaxFee = nil
			}
		}

		if err := validateMinMaxFee(effectiveMinFee, effectiveMaxFee); err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}

		_, err = server.store.UpdateDeliveryFeeConfig(ctx, arg)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

	case "WEATHER_COEFF_EXTREME", "WEATHER_COEFF_HEAVY", "WEATHER_COEFF_MODERATE", "WEATHER_COEFF_LIGHT":
		// 校验数值
		val, err := strconv.ParseFloat(req.Value, 64)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrInvalidNumberFormat))
			return
		}
		if val < 1.0 {
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrCoefficientTooLow))
			return
		}

		arg := db.UpsertRegionRuleConfigParams{
			RegionID: targetRegionID,
		}
		valNumeric := numericFromFloat(val)

		switch key {
		case "WEATHER_COEFF_EXTREME":
			arg.WeatherCoeffExtreme = valNumeric
		case "WEATHER_COEFF_HEAVY":
			arg.WeatherCoeffHeavy = valNumeric
		case "WEATHER_COEFF_MODERATE":
			arg.WeatherCoeffModerate = valNumeric
		case "WEATHER_COEFF_LIGHT":
			arg.WeatherCoeffLight = valNumeric
		}

		_, err = server.store.UpsertRegionRuleConfig(ctx, arg)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		if server.weatherCache != nil {
			if err := server.weatherCache.Delete(ctx, targetRegionID); err != nil {
				log.Warn().Err(err).Int64("region_id", targetRegionID).Str("rule_key", key).Msg("invalidate weather coefficient cache failed after operator rule update")
			}
		}

	default:
		ctx.JSON(http.StatusNotFound, errorResponse(ErrUnknownRuleKey))
		return
	}

	server.writeAuditLog(ctx, AuditLogInput{
		ActorUserID: operator.UserID,
		ActorRole:   "operator",
		Action:      "operator_rule_updated",
		TargetType:  "region",
		TargetID:    &targetRegionID,
		RegionID:    &targetRegionID,
		Metadata:    auditMetadata,
	})

	ctx.JSON(http.StatusOK, MessageResponse{Message: "修改成功"})
}
