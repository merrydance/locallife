package weather

import (
	"strconv"
	"strings"
)

// WeatherCoefficient 天气系数计算结果
type WeatherCoefficient struct {
	Coefficient        float64 // 基础天气系数
	WarningCoefficient float64 // 预警系数
	SuspendDelivery    bool    // 是否暂停配送
	WeatherType        string  // 天气类型：sunny/cloudy/rainy/snowy/extreme
	Temperature        int     // 温度
	Humidity           int     // 湿度
	WindScale          int     // 风力等级
	Precipitation      float64 // 降水量
	Visibility         int     // 能见度
	WarningType        string  // 预警类型
	WarningLevel       string  // 预警等级：blue/yellow/orange/red
	WarningText        string  // 预警内容
}

// CalculateCoefficient 根据天气数据计算运费系数
func CalculateCoefficient(weather *WeatherNow, warnings []WarningAlert) *WeatherCoefficient {
	result := &WeatherCoefficient{
		Coefficient:        1.00,
		WarningCoefficient: 1.00,
		SuspendDelivery:    false,
		WeatherType:        "sunny",
	}

	if weather == nil {
		return result
	}

	// 解析天气数据
	temp, _ := strconv.Atoi(weather.Temp)
	humidity, _ := strconv.Atoi(weather.Humidity)
	windScale, _ := strconv.Atoi(weather.WindScale)
	precip, _ := strconv.ParseFloat(weather.Precip, 64)
	vis, _ := strconv.Atoi(weather.Vis)

	result.Temperature = temp
	result.Humidity = humidity
	result.WindScale = windScale
	result.Precipitation = precip
	result.Visibility = vis

	// 根据天气文字判断天气类型
	result.WeatherType = classifyWeatherType(weather.Text)

	// 计算基础天气系数
	result.Coefficient = calculateBaseCoefficient(result.WeatherType, temp, windScale, vis)

	// 极端天气暂停配送
	if result.WeatherType == "extreme" {
		result.SuspendDelivery = true
	}

	// 计算预警系数
	if len(warnings) > 0 {
		result.WarningCoefficient, result.SuspendDelivery, result.WarningType, result.WarningLevel, result.WarningText = calculateWarningCoefficient(warnings)
	}

	return result
}

// classifyWeatherType 根据天气文字分类天气类型
func classifyWeatherType(text string) string {
	text = strings.ToLower(text)

	// 极端天气
	extremeKeywords := []string{"台风", "龙卷", "冰雹", "暴风", "沙尘暴"}
	for _, kw := range extremeKeywords {
		if strings.Contains(text, kw) {
			return "extreme"
		}
	}

	// 雪天
	if strings.Contains(text, "雪") {
		if strings.Contains(text, "暴雪") || strings.Contains(text, "大雪") {
			return "heavy_snow"
		}
		if strings.Contains(text, "中雪") {
			return "moderate_snow"
		}
		return "light_snow"
	}

	// 雨天
	if strings.Contains(text, "雨") {
		if strings.Contains(text, "暴雨") || strings.Contains(text, "大暴雨") || strings.Contains(text, "特大暴雨") {
			return "heavy_rain"
		}
		if strings.Contains(text, "大雨") {
			return "moderate_rain"
		}
		if strings.Contains(text, "中雨") {
			return "moderate_rain"
		}
		return "light_rain"
	}

	// 多云/阴天
	if strings.Contains(text, "阴") || strings.Contains(text, "多云") {
		return "cloudy"
	}

	// 晴天
	return "sunny"
}

// calculateBaseCoefficient 计算基础天气系数
func calculateBaseCoefficient(weatherType string, temp, windScale, visibility int) float64 {
	coefficient := 1.00

	// 根据天气类型调整
	switch weatherType {
	case "extreme":
		coefficient = 2.00 // 极端天气，系数最高
	case "heavy_rain", "heavy_snow":
		coefficient = 1.50 // 暴雨/暴雪
	case "moderate_rain", "moderate_snow":
		coefficient = 1.30 // 中雨/中雪、大雨/大雪
	case "light_rain", "light_snow":
		coefficient = 1.10 // 小雨/小雪
	case "cloudy":
		coefficient = 1.00 // 多云/阴天，正常
	case "sunny":
		coefficient = 1.00 // 晴天，正常
	}

	// 高温补贴 (>35℃)
	if temp > 35 {
		coefficient += 0.10
	}

	// 低温补贴 (<0℃)
	if temp < 0 {
		coefficient += 0.10
	}

	// 大风补贴 (≥6级)
	if windScale >= 6 {
		coefficient += 0.15
	}

	// 低能见度补贴 (<3km)
	if visibility > 0 && visibility < 3 {
		coefficient += 0.10
	}

	return coefficient
}

// calculateWarningCoefficient 计算预警系数
func calculateWarningCoefficient(warnings []WarningAlert) (coefficient float64, suspend bool, warningType, warningLevel, warningText string) {
	coefficient = 1.00
	suspend = false

	// 找出最严重的预警
	for _, warning := range warnings {
		level := strings.ToLower(warning.Color.Code)
		eventName := warning.EventType.Name

		var currentCoef float64
		var currentSuspend bool

		switch level {
		case "red":
			// 红色预警，暂停配送
			currentCoef = 2.00
			currentSuspend = true
		case "orange":
			// 橙色预警
			currentCoef = 1.30
		case "yellow":
			// 黄色预警
			currentCoef = 1.20
		case "blue":
			// 蓝色预警
			currentCoef = 1.10
		default:
			currentCoef = 1.00
		}

		// 特定极端天气类型直接暂停配送
		if containsExtremeEvent(eventName) {
			currentSuspend = true
			currentCoef = 2.00
		}

		// 取最高系数
		if currentCoef > coefficient {
			coefficient = currentCoef
			suspend = currentSuspend
			warningType = eventName
			warningLevel = level
			warningText = warning.Headline
		}
	}

	return
}

// containsExtremeEvent 判断是否是极端天气事件
func containsExtremeEvent(eventName string) bool {
	extremeEvents := []string{"台风", "龙卷风", "冰雹", "暴风雪", "沙尘暴", "大暴雨", "特大暴雨"}
	for _, event := range extremeEvents {
		if strings.Contains(eventName, event) {
			return true
		}
	}
	return false
}

// FinalCoefficient 计算最终运费系数（基础系数 × 预警系数）
func (c *WeatherCoefficient) FinalCoefficient() float64 {
	return c.Coefficient * c.WarningCoefficient
}
