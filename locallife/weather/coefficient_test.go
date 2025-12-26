package weather

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCalculateCoefficient_SunnyWeather(t *testing.T) {
	// 晴天
	weather := &WeatherNow{
		Icon: "100",
		Temp: "25",
		Text: "晴",
	}

	coef := CalculateCoefficient(weather, nil)

	require.Equal(t, "sunny", coef.WeatherType)
	require.Equal(t, 1.0, coef.Coefficient)
	require.Equal(t, 1.0, coef.WarningCoefficient)
	require.False(t, coef.SuspendDelivery)
	require.Equal(t, 25, coef.Temperature)
}

func TestCalculateCoefficient_LightRain(t *testing.T) {
	// 小雨
	weather := &WeatherNow{
		Icon: "300",
		Temp: "18",
		Text: "小雨",
	}

	coef := CalculateCoefficient(weather, nil)

	require.Equal(t, "light_rain", coef.WeatherType)
	require.Equal(t, 1.1, coef.Coefficient) // 小雨 1.1 倍
	require.False(t, coef.SuspendDelivery)
}

func TestCalculateCoefficient_HeavyRain(t *testing.T) {
	// 暴雨
	weather := &WeatherNow{
		Icon: "310",
		Temp: "20",
		Text: "暴雨",
	}

	coef := CalculateCoefficient(weather, nil)

	require.Equal(t, "heavy_rain", coef.WeatherType)
	require.Equal(t, 1.5, coef.Coefficient) // 暴雨 1.5 倍
	require.False(t, coef.SuspendDelivery)
}

func TestCalculateCoefficient_SnowyWeather(t *testing.T) {
	// 小雪 + 低温
	weather := &WeatherNow{
		Icon: "400",
		Temp: "-2",
		Text: "小雪",
	}

	coef := CalculateCoefficient(weather, nil)

	require.Equal(t, "light_snow", coef.WeatherType)
	require.InDelta(t, 1.2, coef.Coefficient, 0.001) // 小雪 1.1 + 低温 0.1 = 1.2
}

func TestCalculateCoefficient_WithYellowWarning(t *testing.T) {
	weather := &WeatherNow{
		Icon: "100",
		Temp: "30",
		Text: "晴",
	}

	warnings := []WarningAlert{
		{
			Severity:  "Minor",
			EventType: WarningEvent{Name: "高温"},
			Color:     WarningColor{Code: "yellow"},
			Headline:  "高温黄色预警",
		},
	}

	coef := CalculateCoefficient(weather, warnings)

	require.Equal(t, 1.0, coef.Coefficient)        // 晴天无加价
	require.Equal(t, 1.2, coef.WarningCoefficient) // 黄色预警 1.2
	require.Equal(t, "高温", coef.WarningType)
	require.False(t, coef.SuspendDelivery)
}

func TestCalculateCoefficient_WithOrangeWarning(t *testing.T) {
	weather := &WeatherNow{
		Icon: "305",
		Temp: "22",
		Text: "中雨",
	}

	warnings := []WarningAlert{
		{
			Severity:  "Moderate",
			EventType: WarningEvent{Name: "暴雨"},
			Color:     WarningColor{Code: "orange"},
			Headline:  "暴雨橙色预警",
		},
	}

	coef := CalculateCoefficient(weather, warnings)

	require.Equal(t, 1.3, coef.Coefficient)        // 中雨 1.3 倍
	require.Equal(t, 1.3, coef.WarningCoefficient) // 橙色预警 1.3
	require.False(t, coef.SuspendDelivery)
}

func TestCalculateCoefficient_WithRedWarning(t *testing.T) {
	weather := &WeatherNow{
		Icon: "310",
		Temp: "25",
		Text: "暴雨",
	}

	warnings := []WarningAlert{
		{
			Severity:  "Severe",
			EventType: WarningEvent{Name: "暴雨"},
			Color:     WarningColor{Code: "red"},
			Headline:  "暴雨红色预警",
		},
	}

	coef := CalculateCoefficient(weather, warnings)

	require.Equal(t, 1.5, coef.Coefficient)        // 暴雨 1.5 倍
	require.Equal(t, 2.0, coef.WarningCoefficient) // 红色预警 2.0
	require.True(t, coef.SuspendDelivery)          // 红色预警暂停配送
	require.Equal(t, "暴雨", coef.WarningType)
}

func TestCalculateCoefficient_ExtremeWeather(t *testing.T) {
	// 台风
	weather := &WeatherNow{
		Icon: "900",
		Temp: "28",
		Text: "台风",
	}

	coef := CalculateCoefficient(weather, nil)

	require.Equal(t, "extreme", coef.WeatherType)
	require.Equal(t, 2.0, coef.Coefficient) // 极端天气 2.0 倍
	require.True(t, coef.SuspendDelivery)   // 极端天气暂停配送
}

func TestCalculateCoefficient_CloudyWeather(t *testing.T) {
	// 多云
	weather := &WeatherNow{
		Icon: "101",
		Temp: "22",
		Text: "多云",
	}

	coef := CalculateCoefficient(weather, nil)

	require.Equal(t, "cloudy", coef.WeatherType)
	require.Equal(t, 1.0, coef.Coefficient) // 多云无加价
}

func TestCalculateCoefficient_HighWind(t *testing.T) {
	// 晴天 + 大风
	weather := &WeatherNow{
		Icon:      "100",
		Temp:      "25",
		Text:      "晴",
		WindScale: "7",
	}

	coef := CalculateCoefficient(weather, nil)

	require.Equal(t, "sunny", coef.WeatherType)
	require.Equal(t, 1.15, coef.Coefficient) // 晴天 1.0 + 大风 0.15 = 1.15
}

func TestFinalCoefficient(t *testing.T) {
	coef := &WeatherCoefficient{
		Coefficient:        1.3,
		WarningCoefficient: 1.2,
	}

	// 1.3 * 1.2 = 1.56
	require.InDelta(t, 1.56, coef.FinalCoefficient(), 0.001)
}
