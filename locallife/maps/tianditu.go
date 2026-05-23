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
)

const (
	tiandituDefaultBaseURL = "https://api.tianditu.gov.cn"
	tiandituGeocoderURL    = "/geocoder"
)

var (
	ErrRouteUnsupported          = errors.New("route unsupported by provider")
	ErrDistanceMatrixUnsupported = errors.New("distance matrix unsupported by provider")
)

// TiandituMapClient 天地图客户端（仅用于地理编码/逆地理编码）。
// 当前运行时统一使用腾讯 LBS；该实现保留为历史兼容能力。
type TiandituMapClient struct {
	key        string
	baseURL    string
	httpClient *http.Client
}

func NewTiandituMapClient(key, baseURL string) *TiandituMapClient {
	clean := strings.TrimRight(baseURL, "/")
	if clean == "" {
		clean = tiandituDefaultBaseURL
	}
	return &TiandituMapClient{
		key:     key,
		baseURL: clean,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *TiandituMapClient) GetBicyclingRoute(ctx context.Context, from, to Location) (*RouteResult, error) {
	return nil, ErrRouteUnsupported
}

func (c *TiandituMapClient) GetWalkingRoute(ctx context.Context, from, to Location) (*RouteResult, error) {
	return nil, ErrRouteUnsupported
}

func (c *TiandituMapClient) GetDrivingRoute(ctx context.Context, from, to Location) (*RouteResult, error) {
	return nil, ErrRouteUnsupported
}

func (c *TiandituMapClient) GetDistanceMatrix(ctx context.Context, froms, tos []Location, mode string) (*DistanceMatrixResult, error) {
	return nil, ErrDistanceMatrixUnsupported
}

type tiandituGeocodeResponse struct {
	Status string `json:"status"`
	Msg    string `json:"msg"`
	Result struct {
		FormattedAddress string `json:"formatted_address"`
		Location         struct {
			Lon string `json:"lon"`
			Lat string `json:"lat"`
		} `json:"location"`
		AddressComponent struct {
			Province     string `json:"province"`
			City         string `json:"city"`
			County       string `json:"county"`
			Street       string `json:"street"`
			StreetNumber string `json:"street_number"`
			Adcode       string `json:"adcode"`
		} `json:"addressComponent"`
	} `json:"result"`
}

// Geocode 地址转坐标
func (c *TiandituMapClient) Geocode(ctx context.Context, address string) (*GeocodeResult, error) {
	params := url.Values{}
	params.Set("tk", c.key)
	params.Set("ds", fmt.Sprintf(`{"keyWord":"%s"}`, escapeJSONString(address)))

	reqURL := c.baseURL + tiandituGeocoderURL + "?" + params.Encode()
	body, err := c.doRequest(ctx, reqURL)
	if err != nil {
		return nil, err
	}

	var resp tiandituGeocodeResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("tianditu geocode unmarshal: %w", err)
	}
	if resp.Status != "0" {
		return nil, fmt.Errorf("tianditu geocode failed: %s", resp.Msg)
	}

	lat, lng, err := parseStringLatLng(resp.Result.Location.Lat, resp.Result.Location.Lon)
	if err != nil {
		return nil, err
	}

	return &GeocodeResult{
		Location: Location{Lat: lat, Lng: lng},
		Address:  resp.Result.FormattedAddress,
	}, nil
}

// ReverseGeocode 坐标转地址
func (c *TiandituMapClient) ReverseGeocode(ctx context.Context, location Location) (*ReverseGeocodeResult, error) {
	params := url.Values{}
	params.Set("tk", c.key)
	params.Set("postStr", fmt.Sprintf(`{"lon":%f,"lat":%f,"ver":1}`, location.Lng, location.Lat))

	reqURL := c.baseURL + tiandituGeocoderURL + "?" + params.Encode()
	body, err := c.doRequest(ctx, reqURL)
	if err != nil {
		return nil, err
	}

	var resp tiandituGeocodeResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("tianditu reverse geocode unmarshal: %w", err)
	}
	if resp.Status != "0" {
		return nil, fmt.Errorf("tianditu reverse geocode failed: %s", resp.Msg)
	}

	return &ReverseGeocodeResult{
		Provider:         MapProviderTianditu,
		Address:          resp.Result.FormattedAddress,
		FormattedAddress: resp.Result.FormattedAddress,
		Province:         resp.Result.AddressComponent.Province,
		City:             resp.Result.AddressComponent.City,
		District:         resp.Result.AddressComponent.County,
		Street:           resp.Result.AddressComponent.Street,
		StreetNumber:     resp.Result.AddressComponent.StreetNumber,
		Adcode:           resp.Result.AddressComponent.Adcode,
	}, nil
}

func (c *TiandituMapClient) doRequest(ctx context.Context, reqURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, string(b))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	return body, nil
}

func parseStringLatLng(latStr, lngStr string) (float64, float64, error) {
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

func escapeJSONString(input string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"\"", "\\\"",
		"\n", "\\n",
		"\r", "\\r",
		"\t", "\\t",
	)
	return replacer.Replace(input)
}

var _ TencentMapClientInterface = (*TiandituMapClient)(nil)
