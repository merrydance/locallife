package maps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// ErrGeocodeNoResult 表示地理编码未返回结果
var ErrGeocodeNoResult = errors.New("geocode no result")

// OSMClient 使用自建的 OSRM + Nominatim 服务实现 TencentMapClientInterface
// 兼容接口：路径规划、距离矩阵、地理编码、逆地理编码。
// 注意：OSRM 使用经度,纬度顺序；现有 Location 使用 Lat/Lng，内部会按需要转换。
type OSMClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewOSMClient(baseURL string) *OSMClient {
	clean := strings.TrimRight(baseURL, "/")
	return &OSMClient{
		baseURL: clean,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// OSRM route 响应
type osrmRouteResponse struct {
	Code   string `json:"code"`
	Routes []struct {
		Distance float64 `json:"distance"`
		Duration float64 `json:"duration"`
	} `json:"routes"`
}

// OSRM table 响应
type osrmTableResponse struct {
	Code      string      `json:"code"`
	Distances [][]float64 `json:"distances"`
	Durations [][]float64 `json:"durations"`
}

// Nominatim search 响应（仅取首条）
type nominatimSearchItem struct {
	Lat         string `json:"lat"`
	Lon         string `json:"lon"`
	DisplayName string `json:"display_name"`
}

// Nominatim reverse 响应
type nominatimReverseResponse struct {
	DisplayName string `json:"display_name"`
	Address     struct {
		State    string `json:"state"`
		City     string `json:"city"`
		County   string `json:"county"`
		Town     string `json:"town"`
		Village  string `json:"village"`
		Suburb   string `json:"suburb"`
		Road     string `json:"road"`
		House    string `json:"house_number"`
		Postcode string `json:"postcode"`
		Country  string `json:"country"`
	} `json:"address"`
}

// ===== 路径规划 =====

func (c *OSMClient) GetBicyclingRoute(ctx context.Context, from, to Location) (*RouteResult, error) {
	return c.getRoute(ctx, "cycling", from, to)
}

func (c *OSMClient) GetWalkingRoute(ctx context.Context, from, to Location) (*RouteResult, error) {
	return c.getRoute(ctx, "foot", from, to)
}

func (c *OSMClient) GetDrivingRoute(ctx context.Context, from, to Location) (*RouteResult, error) {
	return c.getRoute(ctx, "driving", from, to)
}

func (c *OSMClient) getRoute(ctx context.Context, profile string, from, to Location) (*RouteResult, error) {
	// 微信小程序使用 GCJ-02，OSRM 需要 WGS84
	wgsFromLat, wgsFromLng := GCJ02ToWGS84(from.Lat, from.Lng)
	wgsToLat, wgsToLng := GCJ02ToWGS84(to.Lat, to.Lng)
	coords := fmt.Sprintf("%f,%f;%f,%f", wgsFromLng, wgsFromLat, wgsToLng, wgsToLat)
	endpoint := fmt.Sprintf("%s/route/v1/%s/%s?overview=false", c.baseURL, profile, coords)
	body, err := c.doRequest(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	var resp osrmRouteResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("osrm route unmarshal: %w", err)
	}
	if resp.Code != "Ok" || len(resp.Routes) == 0 {
		return nil, fmt.Errorf("osrm route error: %s", resp.Code)
	}
	r := resp.Routes[0]
	return &RouteResult{Distance: int(r.Distance), Duration: int(r.Duration)}, nil
}

// ===== 距离矩阵 =====

// 仅支持从多个源到多个目的，通过 OSRM table。froms first, tos second，用 sources/destinations 索引。
func (c *OSMClient) GetDistanceMatrix(ctx context.Context, froms, tos []Location, mode string) (*DistanceMatrixResult, error) {
	if len(froms) == 0 || len(tos) == 0 {
		return nil, fmt.Errorf("distance matrix requires at least one source and destination")
	}
	profile := "driving"
	switch mode {
	case "bicycling":
		profile = "cycling"
	case "walking":
		profile = "foot"
	}
	coords := make([]string, 0, len(froms)+len(tos))
	for _, loc := range append(froms, tos...) {
		wgsLat, wgsLng := GCJ02ToWGS84(loc.Lat, loc.Lng)
		coords = append(coords, fmt.Sprintf("%f,%f", wgsLng, wgsLat))
	}
	coordStr := strings.Join(coords, ";")
	// sources: 0..len(froms)-1, destinations: len(froms)..len(all)-1
	sources := make([]string, len(froms))
	for i := range froms {
		sources[i] = fmt.Sprintf("%d", i)
	}
	dests := make([]string, len(tos))
	for i := range tos {
		dests[i] = fmt.Sprintf("%d", len(froms)+i)
	}
	params := url.Values{}
	params.Set("sources", strings.Join(sources, ";"))
	params.Set("destinations", strings.Join(dests, ";"))
	params.Set("annotations", "distance,duration")
	endpoint := fmt.Sprintf("%s/table/v1/%s/%s?%s", c.baseURL, profile, coordStr, params.Encode())
	body, err := c.doRequest(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	var resp osrmTableResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("osrm table unmarshal: %w", err)
	}
	if resp.Code != "Ok" {
		return nil, fmt.Errorf("osrm table error: %s", resp.Code)
	}
	// 组装与现有结构一致
	result := &DistanceMatrixResult{Rows: make([]DistanceMatrixRow, len(froms))}
	for i := 0; i < len(froms); i++ {
		row := DistanceMatrixRow{Elements: make([]DistanceMatrixElement, len(tos))}
		for j := 0; j < len(tos); j++ {
			if i < len(resp.Distances) && j < len(resp.Distances[i]) {
				dist := resp.Distances[i][j]
				dur := resp.Durations[i][j]
				row.Elements[j] = DistanceMatrixElement{Distance: int(dist), Duration: int(dur)}
			}
		}
		result.Rows[i] = row
	}
	return result, nil
}

// ===== 地理编码 =====

func (c *OSMClient) Geocode(ctx context.Context, address string) (*GeocodeResult, error) {
	params := url.Values{}
	params.Set("q", address)
	params.Set("format", "json")
	params.Set("limit", "1")
	params.Set("accept-language", "zh-CN")
	// 使用 search.php 显式请求，避免 MultiViews 协商导致 406
	endpoint := fmt.Sprintf("%s/search.php?%s", c.baseURL, params.Encode())
	body, err := c.doRequest(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	var items []nominatimSearchItem
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, fmt.Errorf("nominatim search unmarshal: %w", err)
	}
	if len(items) == 0 {
		log.Warn().Str("address", address).Str("url", endpoint).Msg("nominatim search returned empty result")
		return nil, ErrGeocodeNoResult
	}
	lat, lng, err := parseLatLng(items[0].Lat, items[0].Lon)
	if err != nil {
		return nil, err
	}
	gcjLat, gcjLng := WGS84ToGCJ02(lat, lng)
	return &GeocodeResult{Location: Location{Lat: gcjLat, Lng: gcjLng}, Address: items[0].DisplayName}, nil
}

func (c *OSMClient) ReverseGeocode(ctx context.Context, location Location) (*ReverseGeocodeResult, error) {
	// 输入为 GCJ-02，Nominatim 需要 WGS84
	wgsLat, wgsLng := GCJ02ToWGS84(location.Lat, location.Lng)
	params := url.Values{}
	params.Set("lat", fmt.Sprintf("%f", wgsLat))
	params.Set("lon", fmt.Sprintf("%f", wgsLng))
	params.Set("format", "jsonv2")
	params.Set("addressdetails", "1")
	endpoint := fmt.Sprintf("%s/reverse.php?%s", c.baseURL, params.Encode())
	body, err := c.doRequest(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	var resp nominatimReverseResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("nominatim reverse unmarshal: %w", err)
	}
	return &ReverseGeocodeResult{
		Address:          resp.DisplayName,
		FormattedAddress: resp.DisplayName,
		Province:         firstNonEmpty(resp.Address.State, resp.Address.City, resp.Address.County),
		City:             firstNonEmpty(resp.Address.City, resp.Address.County, resp.Address.Town),
		District:         firstNonEmpty(resp.Address.County, resp.Address.Town, resp.Address.Village, resp.Address.Suburb),
		Street:           firstNonEmpty(resp.Address.Road),
		StreetNumber:     firstNonEmpty(resp.Address.House),
		Adcode:           resp.Address.Postcode,
	}, nil
}

// ===== 工具函数 =====

func (c *OSMClient) doRequest(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "*/*")
	// Nominatim requires a valid User-Agent per usage policy to avoid 406
	req.Header.Set("User-Agent", "locallife/1.0 (contact: support@locallife)")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		log.Warn().
			Int("status", resp.StatusCode).
			Str("url", url).
			Str("user_agent", req.UserAgent()).
			Str("body", string(b)).
			Msg("osm request failed")
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, string(b))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	return body, nil
}

func parseLatLng(latStr, lngStr string) (float64, float64, error) {
	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse lat: %w", err)
	}
	lng, err := strconv.ParseFloat(lngStr, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse lng: %w", err)
	}
	return lat, lng, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// 确保兼容现有接口
var _ TencentMapClientInterface = (*OSMClient)(nil)
