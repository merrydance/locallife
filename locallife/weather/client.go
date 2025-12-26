package weather

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// QweatherClient 和风天气 API 客户端接口
type QweatherClient interface {
	// LookupCity 城市搜索，返回和风天气 LocationID
	LookupCity(ctx context.Context, location, adm string) (*CityLookupResponse, error)
	// GetWeatherNow 获取实时天气
	GetWeatherNow(ctx context.Context, locationID string) (*WeatherNowResponse, error)
	// GetWeatherWarning 获取天气预警（使用经纬度）
	GetWeatherWarning(ctx context.Context, lat, lon float64) (*WeatherWarningResponse, error)
}

// qweatherClient 和风天气客户端实现
type qweatherClient struct {
	apiKey     string
	apiHost    string
	httpClient *http.Client
}

// NewQweatherClient 创建和风天气客户端
func NewQweatherClient(apiKey, apiHost string) QweatherClient {
	return &qweatherClient{
		apiKey:  apiKey,
		apiHost: apiHost,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ==================== 响应结构体 ====================

// CityLookupResponse 城市搜索响应
type CityLookupResponse struct {
	Code     string         `json:"code"`
	Location []CityLocation `json:"location"`
}

// CityLocation 城市位置信息
type CityLocation struct {
	ID      string `json:"id"`   // LocationID
	Name    string `json:"name"` // 城市名
	Lat     string `json:"lat"`  // 纬度
	Lon     string `json:"lon"`  // 经度
	Adm2    string `json:"adm2"` // 上级行政区（市）
	Adm1    string `json:"adm1"` // 一级行政区（省）
	Country string `json:"country"`
}

// WeatherNowResponse 实时天气响应
type WeatherNowResponse struct {
	Code       string     `json:"code"`
	UpdateTime string     `json:"updateTime"`
	Now        WeatherNow `json:"now"`
}

// WeatherNow 实时天气数据
type WeatherNow struct {
	ObsTime   string `json:"obsTime"`   // 观测时间
	Temp      string `json:"temp"`      // 温度 ℃
	FeelsLike string `json:"feelsLike"` // 体感温度
	Icon      string `json:"icon"`      // 天气图标代码
	Text      string `json:"text"`      // 天气状况文字（晴、多云、雨等）
	Wind360   string `json:"wind360"`   // 风向角度
	WindDir   string `json:"windDir"`   // 风向
	WindScale string `json:"windScale"` // 风力等级
	WindSpeed string `json:"windSpeed"` // 风速 km/h
	Humidity  string `json:"humidity"`  // 相对湿度 %
	Precip    string `json:"precip"`    // 降水量 mm
	Pressure  string `json:"pressure"`  // 大气压强 hPa
	Vis       string `json:"vis"`       // 能见度 km
	Cloud     string `json:"cloud"`     // 云量 %
	Dew       string `json:"dew"`       // 露点温度
}

// WeatherWarningResponse 天气预警响应
type WeatherWarningResponse struct {
	Metadata WarningMetadata `json:"metadata"`
	Alerts   []WarningAlert  `json:"alerts"`
}

// WarningMetadata 预警元数据
type WarningMetadata struct {
	ZeroResult bool `json:"zeroResult"` // true 表示无预警
}

// WarningAlert 预警信息
type WarningAlert struct {
	ID         string       `json:"id"`
	SenderName string       `json:"senderName"`
	IssuedTime string       `json:"issuedTime"`
	EventType  WarningEvent `json:"eventType"`
	Severity   string       `json:"severity"` // minor/moderate/severe/extreme
	Color      WarningColor `json:"color"`
	ExpireTime string       `json:"expireTime"`
	Headline   string       `json:"headline"`
}

// WarningEvent 预警事件类型
type WarningEvent struct {
	Name string `json:"name"` // 大风、暴雨、台风等
	Code string `json:"code"`
}

// WarningColor 预警颜色
type WarningColor struct {
	Code string `json:"code"` // blue/yellow/orange/red
}

// ==================== API 调用实现 ====================

// doRequest 发送 HTTP 请求
func (c *qweatherClient) doRequest(ctx context.Context, path string, params url.Values) ([]byte, error) {
	// 构建完整 URL
	fullURL := fmt.Sprintf("%s%s", c.apiHost, path)
	if len(params) > 0 {
		fullURL = fmt.Sprintf("%s?%s", fullURL, params.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	// 设置请求头
	req.Header.Set("X-QW-Api-Key", c.apiKey)
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// 处理 Gzip 压缩
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("gzip reader failed: %w", err)
		}
		defer gzReader.Close()
		reader = gzReader
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read body failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// LookupCity 城市搜索
func (c *qweatherClient) LookupCity(ctx context.Context, location, adm string) (*CityLookupResponse, error) {
	params := url.Values{}
	params.Set("location", location)
	if adm != "" {
		params.Set("adm", adm)
	}
	params.Set("range", "cn") // 仅搜索中国
	params.Set("number", "1") // 只返回1个结果

	body, err := c.doRequest(ctx, "/geo/v2/city/lookup", params)
	if err != nil {
		return nil, err
	}

	var resp CityLookupResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal city lookup response failed: %w", err)
	}

	if resp.Code != "200" {
		return nil, fmt.Errorf("city lookup API error: code=%s", resp.Code)
	}

	return &resp, nil
}

// GetWeatherNow 获取实时天气
func (c *qweatherClient) GetWeatherNow(ctx context.Context, locationID string) (*WeatherNowResponse, error) {
	params := url.Values{}
	params.Set("location", locationID)

	body, err := c.doRequest(ctx, "/v7/weather/now", params)
	if err != nil {
		return nil, err
	}

	var resp WeatherNowResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal weather now response failed: %w", err)
	}

	if resp.Code != "200" {
		return nil, fmt.Errorf("weather now API error: code=%s", resp.Code)
	}

	return &resp, nil
}

// GetWeatherWarning 获取天气预警
func (c *qweatherClient) GetWeatherWarning(ctx context.Context, lat, lon float64) (*WeatherWarningResponse, error) {
	// 新版预警 API 使用路径参数
	path := fmt.Sprintf("/weatheralert/v1/current/%.2f/%.2f", lat, lon)

	body, err := c.doRequest(ctx, path, nil)
	if err != nil {
		return nil, err
	}

	var resp WeatherWarningResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal weather warning response failed: %w", err)
	}

	return &resp, nil
}
