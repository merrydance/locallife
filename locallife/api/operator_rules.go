package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

// ListRulesResponse 规则列表响应
type ListRulesResponse struct {
	Rules []RuleItem `json:"rules"`
}

// RuleItem 单个规则项
type RuleItem struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Key   string `json:"key"`
	Value string `json:"value"`
	Unit  string `json:"unit"`
	Desc  string `json:"desc"`
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

	rules := []RuleItem{}

	// 1. 商户入驻保证金 (从 operator 表获取, 单位: 分 -> 元)
	// MerchantDeposit is int64 (fen)
	merchantDeposit := fmt.Sprintf("%.2f", float64(operator.MerchantDeposit)/100.0)

	rules = append(rules, RuleItem{
		ID:    "rule_1",
		Name:  "商户入驻保证金",
		Key:   "MERCHANT_DEPOSIT",
		Value: merchantDeposit,
		Unit:  "元",
		Desc:  "商户入驻需缴纳的保证金金额",
	})

	// 2. 骑手入驻押金 (从 operator 表获取, 单位: 分 -> 元)
	// RiderDeposit is int64 (fen)
	riderDeposit := fmt.Sprintf("%.2f", float64(operator.RiderDeposit)/100.0)

	rules = append(rules, RuleItem{
		ID:    "rule_2",
		Name:  "骑手入驻押金",
		Key:   "RIDER_DEPOSIT",
		Value: riderDeposit,
		Unit:  "元",
		Desc:  "骑手接单前需缴纳的押金金额",
	})

	// 3. 平台抽成比例 (From Operator DB)
	// commission_rate 是 numeric
	commissionRate := pgNumericToFloat64(operator.CommissionRate) * 100
	rules = append(rules, RuleItem{
		ID:    "rule_3",
		Name:  "平台抽成比例",
		Key:   "PLATFORM_COMMISSION",
		Value: fmt.Sprintf("%.1f", commissionRate),
		Unit:  "%",
		Desc:  "每笔订单平台收取的服务费比例",
	})

	// 获取运费配置
	feeConfig, err := server.store.GetActiveDeliveryFeeConfigByRegion(ctx, operator.RegionID)
	if err == nil {
		// 4. 基础运费
		// BaseFee 是分，转换为元
		baseFee := float64(feeConfig.BaseFee) / 100.0
		rules = append(rules, RuleItem{
			ID:    "rule_4",
			Name:  "基础运费",
			Key:   "BASE_DELIVERY_FEE",
			Value: fmt.Sprintf("%.2f", baseFee),
			Unit:  "元",
			Desc:  "配送的基础费用（含基础距离）",
		})

		// 5. 基础距离
		rules = append(rules, RuleItem{
			ID:    "rule_5",
			Name:  "基础距离",
			Key:   "BASE_DISTANCE",
			Value: fmt.Sprintf("%d", feeConfig.BaseDistance),
			Unit:  "米",
			Desc:  "基础运费包含的配送距离",
		})

		// 6. 超距加价
		// ExtraFeePerKm 是分，转换为元
		extraFee := float64(feeConfig.ExtraFeePerKm) / 100.0
		rules = append(rules, RuleItem{
			ID:    "rule_6",
			Name:  "超距加价",
			Key:   "EXTRA_FEE_PER_KM",
			Value: fmt.Sprintf("%.2f", extraFee),
			Unit:  "元/km",
			Desc:  "超出基础距离后每公里的加价",
		})

		// 8. 最低运费
		minFee := float64(feeConfig.MinFee) / 100.0
		rules = append(rules, RuleItem{
			ID:    "rule_8",
			Name:  "最低运费",
			Key:   "MIN_DELIVERY_FEE",
			Value: fmt.Sprintf("%.2f", minFee),
			Unit:  "元",
			Desc:  "配送费的最低下限",
		})

		// 9. 最高运费 (MaxFee is nullable)
		maxFeeVal := "不限"
		if feeConfig.MaxFee.Valid {
			maxFeeVal = fmt.Sprintf("%.2f", float64(feeConfig.MaxFee.Int64)/100.0)
		}
		rules = append(rules, RuleItem{
			ID:    "rule_9",
			Name:  "最高运费",
			Key:   "MAX_DELIVERY_FEE",
			Value: maxFeeVal,
			Unit:  "元",
			Desc:  "配送费的最高上限（0或不填代表不限）",
		})

		// 10. 货值费率
		valueRatio := pgNumericToFloat64(feeConfig.ValueRatio) * 100
		rules = append(rules, RuleItem{
			ID:    "rule_10",
			Name:  "货值费率",
			Key:   "DELIVERY_VALUE_RATIO",
			Value: fmt.Sprintf("%.2f", valueRatio),
			Unit:  "%",
			Desc:  "按商品金额收取的保险/服务费比例",
		})

	} else {
		// 未配置时显示默认值或提示
		if isNotFoundError(err) {
			rules = append(rules, RuleItem{
				ID:    "rule_4",
				Name:  "基础运费",
				Key:   "BASE_DELIVERY_FEE",
				Value: "未配置",
				Unit:  "元",
				Desc:  "配送的基础费用",
			})
		}
	}

	// 11. 恶劣天气加价倍数
	weatherExtreme := pgNumericToFloat64(operator.WeatherCoeffExtreme)
	rules = append(rules, RuleItem{
		ID:    "rule_11",
		Name:  "极端天气倍数",
		Key:   "WEATHER_COEFF_EXTREME",
		Value: fmt.Sprintf("%.2f", weatherExtreme),
		Unit:  "x",
		Desc:  "台风/龙卷风等极端天气下的运费倍数",
	})

	// 12. 暴雨/雪加价倍数
	weatherHeavy := pgNumericToFloat64(operator.WeatherCoeffHeavy)
	rules = append(rules, RuleItem{
		ID:    "rule_12",
		Name:  "暴雨雪倍数",
		Key:   "WEATHER_COEFF_HEAVY",
		Value: fmt.Sprintf("%.2f", weatherHeavy),
		Unit:  "x",
		Desc:  "暴雨/暴雪/特大暴雨下的运费倍数",
	})

	// 13. 中雨/雪加价倍数
	weatherModerate := pgNumericToFloat64(operator.WeatherCoeffModerate)
	rules = append(rules, RuleItem{
		ID:    "rule_13",
		Name:  "中雨雪倍数",
		Key:   "WEATHER_COEFF_MODERATE",
		Value: fmt.Sprintf("%.2f", weatherModerate),
		Unit:  "x",
		Desc:  "中雨/中雪/大雨/大雪下的运费倍数",
	})

	// 14. 小雨/雪加价倍数
	weatherLight := pgNumericToFloat64(operator.WeatherCoeffLight)
	rules = append(rules, RuleItem{
		ID:    "rule_14",
		Name:  "小雨雪倍数",
		Key:   "WEATHER_COEFF_LIGHT",
		Value: fmt.Sprintf("%.2f", weatherLight),
		Unit:  "x",
		Desc:  "小雨/小雪下的运费倍数",
	})

	// 获取最新天气系数
	weather, err := server.store.GetLatestWeatherCoefficient(ctx, operator.RegionID)
	if err == nil {
		// 7. 当前天气加价系数
		coeff := pgNumericToFloat64(weather.FinalCoefficient)
		rules = append(rules, RuleItem{
			ID:    "rule_7",
			Name:  "天气加价系数",
			Key:   "WEATHER_COEFFICIENT", // 此字段通常由系统自动更新，但展示给运营商看
			Value: fmt.Sprintf("%.2f", coeff),
			Unit:  "x",
			Desc:  fmt.Sprintf("当前天气：%s (系统自动更新)", weather.WeatherType),
		})
	} else {
		// 暂无天气数据时显示默认值
		rules = append(rules, RuleItem{
			ID:    "rule_7",
			Name:  "天气加价系数",
			Key:   "WEATHER_COEFFICIENT",
			Value: "1.00",
			Unit:  "x",
			Desc:  "当前天气：暂无数据 (系统自动更新)",
		})
	}

	ctx.JSON(http.StatusOK, ListRulesResponse{Rules: rules})
}

type updateRuleRequest struct {
	Value string `json:"value" binding:"required"`
}

// updateOperatorRule 更新运营商规则配置
// @Summary 更新规则配置
// @Description 更新运营商规则配置，支持【平台抽成比例】、【商户保证金】、【骑手押金】
// @Tags 运营商-规则管理
// @Accept json
// @Produce json
// @Param key path string true "规则Key (MERCHANT_DEPOSIT, RIDER_DEPOSIT, PLATFORM_COMMISSION)"
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

	// 如果运营商未关联区域，无法设置区域级配置
	if operator.RegionID == 0 {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("operator has no assigned region")))
		return
	}

	switch key {
	case "PLATFORM_COMMISSION":
		// 解析新值
		rate, err := strconv.ParseFloat(req.Value, 64)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("无效的数值格式")))
			return
		}
		if rate < 0 || rate > 100 {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("比例必须在 0-100 之间")))
			return
		}

		// 更新数据库
		// 前端传入的是百分比 (e.g. 15)，DB 存储的是小数 (e.g. 0.15)
		newRate := rate / 100.0
		arg := db.UpdateOperatorParams{
			ID:             operator.ID,
			CommissionRate: float64ToPgNumeric(newRate),
		}
		_, err = server.store.UpdateOperator(ctx, arg)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

	case "MERCHANT_DEPOSIT", "RIDER_DEPOSIT":
		// 校验数值
		val, err := strconv.ParseFloat(req.Value, 64)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("无效的数值格式")))
			return
		}
		if val < 0 {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("金额不能为负数")))
			return
		}

		// Update operators table
		// Convert to fen (int64)
		valInt := int64(val * 100)
		arg := db.UpdateOperatorRulesParams{
			ID: operator.ID,
		}

		if key == "MERCHANT_DEPOSIT" {
			arg.MerchantDeposit = pgtype.Int8{Int64: valInt, Valid: true}
		} else {
			arg.RiderDeposit = pgtype.Int8{Int64: valInt, Valid: true}
		}

		_, err = server.store.UpdateOperatorRules(ctx, arg)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

	case "BASE_DELIVERY_FEE", "BASE_DISTANCE", "EXTRA_FEE_PER_KM", "MIN_DELIVERY_FEE", "MAX_DELIVERY_FEE", "DELIVERY_VALUE_RATIO":
		// 1. 获取现有配置或初始化
		feeConfig, err := server.store.GetActiveDeliveryFeeConfigByRegion(ctx, operator.RegionID)
		if err != nil {
			if isNotFoundError(err) {
				// 如果不存在，需要先创建
				// 这里简化逻辑，如果不存在则初始化默认值
				feeConfig, err = server.store.CreateDeliveryFeeConfig(ctx, db.CreateDeliveryFeeConfigParams{
					RegionID:      operator.RegionID,
					BaseFee:       DefaultBaseFee,
					BaseDistance:  DefaultBaseDistance,
					ExtraFeePerKm: DefaultExtraFeePerKm,
					ValueRatio:    float64ToPgNumeric(DefaultValueRatio),
					MinFee:        DefaultMinFee,
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
				ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("无效的数值格式")))
				return
			}

			switch key {
			case "BASE_DELIVERY_FEE":
				if val < 0 {
					ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("金额不能为负数")))
					return
				}
				arg.BaseFee = pgtype.Int8{Int64: int64(val * 100), Valid: true}
			case "BASE_DISTANCE":
				if val < 0 {
					ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("距离不能为负数")))
					return
				}
				arg.BaseDistance = pgtype.Int4{Int32: int32(val), Valid: true}
			case "EXTRA_FEE_PER_KM":
				if val < 0 {
					ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("金额不能为负数")))
					return
				}
				arg.ExtraFeePerKm = pgtype.Int8{Int64: int64(val * 100), Valid: true}
			case "MIN_DELIVERY_FEE":
				if val < 0 {
					ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("金额不能为负数")))
					return
				}
				arg.MinFee = pgtype.Int8{Int64: int64(val * 100), Valid: true}
			case "MAX_DELIVERY_FEE":
				if val <= 0 {
					// 0 or negative considered as no limit or clear limit?
					// Typical logic: 0 or -1 means no limit.
					// User might enter 0. Let's treat 0 as "No Limit" (NULL).
					arg.MaxFee = pgtype.Int8{Valid: false}
				} else {
					arg.MaxFee = pgtype.Int8{Int64: int64(val * 100), Valid: true}
				}
			case "DELIVERY_VALUE_RATIO":
				if val < 0 || val > 100 {
					ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("比例必须在 0-100 之间")))
					return
				}
				// Percent to ratio: 1% -> 0.01
				arg.ValueRatio = float64ToPgNumeric(val / 100.0)
			}
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
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("无效的数值格式")))
			return
		}
		if val < 1.0 {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("系数不能小于 1.0")))
			return
		}

		// Update operators table
		arg := db.UpdateOperatorRulesParams{
			ID: operator.ID,
		}
		valNumeric := float64ToPgNumeric(val)

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

		_, err = server.store.UpdateOperatorRules(ctx, arg)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

	case "WEATHER_COEFFICIENT":
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("天气系数由系统根据实时天气自动更新，无法手动修改")))
		return

	default:
		ctx.JSON(http.StatusNotFound, errorResponse(errors.New("未知的规则 Key")))
		return
	}

	server.writeAuditLog(ctx, auditLogInput{
		ActorUserID: operator.UserID,
		ActorRole:   "operator",
		Action:      "operator_rule_updated",
		TargetType:  "operator",
		TargetID:    &operator.ID,
		RegionID:    &operator.RegionID,
		Metadata: map[string]any{
			"key":   key,
			"value": req.Value,
		},
	})

	ctx.JSON(http.StatusOK, MessageResponse{Message: "修改成功"})
}

func float64ToPgNumeric(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(fmt.Sprintf("%f", f))
	return n
}
