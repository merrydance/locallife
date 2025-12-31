package maps

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	baseURL = "https://apis.map.qq.com"

	// 路径规划
	bicyclingURL = "/ws/direction/v1/bicycling/" // 骑行
	walkingURL   = "/ws/direction/v1/walking/"   // 步行
	drivingURL   = "/ws/direction/v1/driving/"   // 驾车

	// 距离矩阵
	distanceMatrixURL = "/ws/distance/v1/matrix/"

	// 地理编码
	geocodeURL        = "/ws/geocoder/v1/" // 地址转坐标
	reverseGeocodeURL = "/ws/geocoder/v1/" // 坐标转地址
)

// TencentMapClient 腾讯地图客户端
type TencentMapClient struct {
	key        string
	httpClient *http.Client
}

// TencentMapClientInterface 腾讯地图客户端接口
type TencentMapClientInterface interface {
	// 路径规划
	GetBicyclingRoute(ctx context.Context, from, to Location) (*RouteResult, error)
	GetWalkingRoute(ctx context.Context, from, to Location) (*RouteResult, error)
	GetDrivingRoute(ctx context.Context, from, to Location) (*RouteResult, error)

	// 距离矩阵（批量计算）
	GetDistanceMatrix(ctx context.Context, froms, tos []Location, mode string) (*DistanceMatrixResult, error)

	// 地理编码
	Geocode(ctx context.Context, address string) (*GeocodeResult, error)
	ReverseGeocode(ctx context.Context, location Location) (*ReverseGeocodeResult, error)
}

// Location 位置坐标
type Location struct {
	Lat float64 `json:"lat"` // 纬度
	Lng float64 `json:"lng"` // 经度
}

// String 返回 "纬度,经度" 格式
func (l Location) String() string {
	return fmt.Sprintf("%f,%f", l.Lat, l.Lng)
}

// RouteResult 路径规划结果
type RouteResult struct {
	Distance int `json:"distance"` // 距离（米）
	Duration int `json:"duration"` // 时间（秒）
}

// DistanceMatrixResult 距离矩阵结果
type DistanceMatrixResult struct {
	Rows []DistanceMatrixRow `json:"rows"`
}

// DistanceMatrixRow 距离矩阵行
type DistanceMatrixRow struct {
	Elements []DistanceMatrixElement `json:"elements"`
}

// DistanceMatrixElement 距离矩阵元素
type DistanceMatrixElement struct {
	Distance int `json:"distance"` // 距离（米）
	Duration int `json:"duration"` // 时间（秒）
}

// GeocodeResult 地理编码结果
type GeocodeResult struct {
	Location Location `json:"location"`
	Address  string   `json:"address"`
}

// ReverseGeocodeResult 逆地理编码结果
type ReverseGeocodeResult struct {
	Address          string `json:"address"`
	FormattedAddress string `json:"formatted_address"`
	Province         string `json:"province"`
	City             string `json:"city"`
	District         string `json:"district"`
	Street           string `json:"street"`
	StreetNumber     string `json:"street_number"`
	Adcode           string `json:"adcode"`
}

// NewTencentMapClient 创建腾讯地图客户端
func NewTencentMapClient(key string) *TencentMapClient {
	return &TencentMapClient{
		key: key,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ==================== API 响应结构 ====================

type apiResponse struct {
	Status  int             `json:"status"`
	Message string          `json:"message"`
	Result  json.RawMessage `json:"result"`
}

type routeAPIResult struct {
	Routes []struct {
		Distance int `json:"distance"`
		Duration int `json:"duration"`
	} `json:"routes"`
}

type distanceMatrixAPIResult struct {
	Rows []struct {
		Elements []struct {
			Distance int `json:"distance"`
			Duration int `json:"duration"`
		} `json:"elements"`
	} `json:"rows"`
}

type geocodeAPIResult struct {
	Location struct {
		Lat float64 `json:"lat"`
		Lng float64 `json:"lng"`
	} `json:"location"`
	Address string `json:"address"`
}

type reverseGeocodeAPIResult struct {
	Address          string `json:"address"`
	FormattedAddress struct {
		Recommend string `json:"recommend"`
	} `json:"formatted_addresses"`
	AddressComponent struct {
		Province     string `json:"province"`
		City         string `json:"city"`
		District     string `json:"district"`
		Street       string `json:"street"`
		StreetNumber string `json:"street_number"`
	} `json:"address_component"`
	AdInfo struct {
		Adcode string `json:"adcode"`
	} `json:"ad_info"`
}

// ==================== 路径规划 ====================

// GetBicyclingRoute 获取骑行路线（外卖骑手用）
func (c *TencentMapClient) GetBicyclingRoute(ctx context.Context, from, to Location) (*RouteResult, error) {
	return c.getRoute(ctx, bicyclingURL, from, to)
}

// GetWalkingRoute 获取步行路线
func (c *TencentMapClient) GetWalkingRoute(ctx context.Context, from, to Location) (*RouteResult, error) {
	return c.getRoute(ctx, walkingURL, from, to)
}

// GetDrivingRoute 获取驾车路线
func (c *TencentMapClient) GetDrivingRoute(ctx context.Context, from, to Location) (*RouteResult, error) {
	return c.getRoute(ctx, drivingURL, from, to)
}

func (c *TencentMapClient) getRoute(ctx context.Context, apiURL string, from, to Location) (*RouteResult, error) {
	params := url.Values{}
	params.Set("from", from.String())
	params.Set("to", to.String())
	params.Set("key", c.key)

	reqURL := baseURL + apiURL + "?" + params.Encode()

	body, err := c.doRequest(ctx, reqURL)
	if err != nil {
		return nil, err
	}

	var result routeAPIResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal result: %w", err)
	}

	if len(result.Routes) == 0 {
		return nil, fmt.Errorf("no route found")
	}

	return &RouteResult{
		Distance: result.Routes[0].Distance,
		Duration: result.Routes[0].Duration,
	}, nil
}

// ==================== 距离矩阵 ====================

// GetDistanceMatrix 批量计算距离（多对多）
// mode: "bicycling" | "walking" | "driving"
func (c *TencentMapClient) GetDistanceMatrix(ctx context.Context, froms, tos []Location, mode string) (*DistanceMatrixResult, error) {
	// 构建 from 和 to 参数
	var fromStrs, toStrs []string
	for _, loc := range froms {
		fromStrs = append(fromStrs, loc.String())
	}
	for _, loc := range tos {
		toStrs = append(toStrs, loc.String())
	}

	params := url.Values{}
	params.Set("from", joinLocations(fromStrs))
	params.Set("to", joinLocations(toStrs))
	params.Set("mode", mode)
	params.Set("key", c.key)

	reqURL := baseURL + distanceMatrixURL + "?" + params.Encode()

	body, err := c.doRequest(ctx, reqURL)
	if err != nil {
		return nil, err
	}

	var result distanceMatrixAPIResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal result: %w", err)
	}

	// 转换结果
	matrixResult := &DistanceMatrixResult{
		Rows: make([]DistanceMatrixRow, len(result.Rows)),
	}
	for i, row := range result.Rows {
		matrixResult.Rows[i].Elements = make([]DistanceMatrixElement, len(row.Elements))
		for j, elem := range row.Elements {
			matrixResult.Rows[i].Elements[j] = DistanceMatrixElement{
				Distance: elem.Distance,
				Duration: elem.Duration,
			}
		}
	}

	return matrixResult, nil
}

func joinLocations(locs []string) string {
	result := ""
	for i, loc := range locs {
		if i > 0 {
			result += ";"
		}
		result += loc
	}
	return result
}

// ==================== 地理编码 ====================

// Geocode 地址转坐标
func (c *TencentMapClient) Geocode(ctx context.Context, address string) (*GeocodeResult, error) {
	params := url.Values{}
	params.Set("address", address)
	params.Set("key", c.key)

	reqURL := baseURL + geocodeURL + "?" + params.Encode()

	body, err := c.doRequest(ctx, reqURL)
	if err != nil {
		return nil, err
	}

	var result geocodeAPIResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal result: %w", err)
	}

	return &GeocodeResult{
		Location: Location{
			Lat: result.Location.Lat,
			Lng: result.Location.Lng,
		},
		Address: result.Address,
	}, nil
}

// ReverseGeocode 坐标转地址
func (c *TencentMapClient) ReverseGeocode(ctx context.Context, location Location) (*ReverseGeocodeResult, error) {
	params := url.Values{}
	params.Set("location", location.String())
	params.Set("key", c.key)

	reqURL := baseURL + reverseGeocodeURL + "?" + params.Encode()

	body, err := c.doRequest(ctx, reqURL)
	if err != nil {
		return nil, err
	}

	var result reverseGeocodeAPIResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal result: %w", err)
	}

	return &ReverseGeocodeResult{
		Address:          result.Address,
		FormattedAddress: result.FormattedAddress.Recommend,
		Province:         result.AddressComponent.Province,
		City:             result.AddressComponent.City,
		District:         result.AddressComponent.District,
		Street:           result.AddressComponent.Street,
		StreetNumber:     result.AddressComponent.StreetNumber,
		Adcode:           result.AdInfo.Adcode,
	}, nil
}

// ==================== HTTP 请求 ====================

func (c *TencentMapClient) doRequest(ctx context.Context, reqURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// 调试日志：打印响应体（隐藏 key）
	debugURL := reqURL
	if idx := len(debugURL) - 40; idx > 0 {
		// 简单隐藏 key
		debugURL = debugURL[:idx] + "key=***"
	}
	fmt.Printf("[TencentMap] Request: %s\n", debugURL)
	fmt.Printf("[TencentMap] Response: %s\n", string(body))

	var apiResp apiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if apiResp.Status != 0 {
		return nil, fmt.Errorf("API error: %s (status=%d)", apiResp.Message, apiResp.Status)
	}

	return apiResp.Result, nil
}

// 确保实现接口
var _ TencentMapClientInterface = (*TencentMapClient)(nil)
