package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/maps"
)

const tencentMapBaseURL = "https://apis.map.qq.com"
const tencentBicyclingURL = "/ws/direction/v1/bicycling/"

type reverseGeocodeResponse struct {
	Address          string `json:"address"`
	FormattedAddress string `json:"formatted_address"`
	Province         string `json:"province"`
	City             string `json:"city"`
	District         string `json:"district"`
	Street           string `json:"street"`
	StreetNumber     string `json:"street_number"`
}

type reverseGeocodeAPIResponse struct {
	Code    int                    `json:"code" example:"0"`
	Message string                 `json:"message" example:"ok"`
	Data    reverseGeocodeResponse `json:"data"`
}

func parseLatitudeLongitude(ctx *gin.Context) (float64, float64, error) {
	latStr := ctx.Query("latitude")
	lngStr := ctx.Query("longitude")
	if latStr == "" || lngStr == "" {
		return 0, 0, fmt.Errorf("latitude and longitude are required")
	}

	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid latitude: %w", err)
	}
	lng, err := strconv.ParseFloat(lngStr, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid longitude: %w", err)
	}

	if lat < -90 || lat > 90 {
		return 0, 0, fmt.Errorf("latitude out of range")
	}
	if lng < -180 || lng > 180 {
		return 0, 0, fmt.Errorf("longitude out of range")
	}

	return lat, lng, nil
}

func validateLatitudeLongitude(lat, lng float64) error {
	if lat < -90 || lat > 90 {
		return fmt.Errorf("latitude out of range")
	}
	if lng < -180 || lng > 180 {
		return fmt.Errorf("longitude out of range")
	}
	return nil
}

func parseLocationPair(value string) (float64, float64, error) {
	parts := strings.Split(value, ",")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid location format")
	}

	lat, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid latitude: %w", err)
	}
	lng, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid longitude: %w", err)
	}

	if err := validateLatitudeLongitude(lat, lng); err != nil {
		return 0, 0, err
	}

	return lat, lng, nil
}

type tencentDirectionAPIResponse struct {
	Code    int         `json:"code" example:"0"`
	Message string      `json:"message" example:"ok"`
	Data    interface{} `json:"data"`
}

// reverseGeocode uses Tencent LBS to convert (lat,lng) into an address.
// @Summary 逆地址解析
// @Description 使用腾讯 LBS 将经纬度解析为地址（服务端调用腾讯接口，不暴露 key）。
// @Tags 位置
// @Produce json
// @Param latitude query number true "纬度" example(39.908722)
// @Param longitude query number true "经度" example(116.397499)
// @Success 200 {object} reverseGeocodeAPIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/location/reverse-geocode [get]
func (server *Server) reverseGeocode(ctx *gin.Context) {
	if server.mapClient == nil {
		internalError(ctx, fmt.Errorf("tencent map client is not configured"))
		return
	}

	lat, lng, err := parseLatitudeLongitude(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if err := validateLatitudeLongitude(lat, lng); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	result, err := server.mapClient.ReverseGeocode(ctx, maps.Location{Lat: lat, Lng: lng})
	if err != nil {
		internalError(ctx, fmt.Errorf("reverse geocode: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, reverseGeocodeAPIResponse{
		Code:    0,
		Message: "ok",
		Data: reverseGeocodeResponse{
			Address:          result.Address,
			FormattedAddress: result.FormattedAddress,
			Province:         result.Province,
			City:             result.City,
			District:         result.District,
			Street:           result.Street,
			StreetNumber:     result.StreetNumber,
		},
	})
}

// proxyTencentBicyclingDirection proxies Tencent LBS bicycling route API.
// It returns Tencent's raw JSON response to minimize frontend changes.
// @Summary 腾讯骑行路线（后端代理）
// @Description 后端请求腾讯 LBS /ws/direction/v1/bicycling 并原样返回 JSON（不暴露 key）。
// @Tags 位置
// @Produce json
// @Param from query string true "起点坐标，格式: lat,lng" example("39.908722,116.397499")
// @Param to query string true "终点坐标，格式: lat,lng" example("39.914722,116.404499")
// @Param policy query integer false "路线策略（腾讯 LBS policy 参数）" example(0)
// @Success 200 {object} tencentDirectionAPIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/location/direction/bicycling [get]
func (server *Server) proxyTencentBicyclingDirection(ctx *gin.Context) {
	if server.config.TencentMapKey == "" {
		internalError(ctx, fmt.Errorf("tencent map key is not configured"))
		return
	}

	from := ctx.Query("from")
	to := ctx.Query("to")
	if from == "" || to == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("from and to are required")))
		return
	}

	if _, _, err := parseLocationPair(from); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("invalid from: %w", err)))
		return
	}
	if _, _, err := parseLocationPair(to); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("invalid to: %w", err)))
		return
	}

	params := url.Values{}
	params.Set("from", from)
	params.Set("to", to)
	params.Set("key", server.config.TencentMapKey)
	if policy := ctx.Query("policy"); policy != "" {
		params.Set("policy", policy)
	}

	reqURL := tencentMapBaseURL + tencentBicyclingURL + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		internalError(ctx, fmt.Errorf("create request: %w", err))
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		internalError(ctx, fmt.Errorf("do request: %w", err))
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		internalError(ctx, fmt.Errorf("read response: %w", err))
		return
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		internalError(ctx, fmt.Errorf("tencent direction http status=%d body=%s", resp.StatusCode, string(body)))
		return
	}

	var raw json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		internalError(ctx, fmt.Errorf("decode tencent direction response: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, tencentDirectionAPIResponse{
		Code:    0,
		Message: "ok",
		Data:    raw,
	})
}

// matchRegionID 根据经纬度匹配 region_id
// 优先使用腾讯地图逆地址解析获取 adcode 匹配，失败则回退到球面距离最近匹配
func (server *Server) matchRegionID(ctx context.Context, lat, lon float64) (int64, error) {
	// 1. 尝试通过腾讯地图 API 获取 adcode
	if server.mapClient != nil {
		res, err := server.mapClient.ReverseGeocode(ctx, maps.Location{Lat: lat, Lng: lon})
		if err == nil && res.Adcode != "" {
			// 腾讯返回的 adcode 通常是 6 位，我们的数据库中也是 6 位
			region, err := server.store.GetRegionByCode(ctx, res.Adcode)
			if err == nil {
				return region.ID, nil
			}
		}
	}

	// 2. 兜底方案：寻找距离最近的行政区中心点
	region, err := server.store.GetClosestRegion(ctx, db.GetClosestRegionParams{
		Lat: lat,
		Lon: lon,
	})
	if err != nil {
		return 0, err
	}

	return region.ID, nil
}
