package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/maps"
)

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

var _ reverseGeocodeAPIResponse

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

type routeAPIResponse struct {
	Code    int              `json:"code" example:"0"`
	Message string           `json:"message" example:"ok"`
	Data    maps.RouteResult `json:"data"`
}

var _ routeAPIResponse

// reverseGeocode uses self-hosted OSM (Nominatim) to convert (lat,lng) into an address.
// @Summary 逆地址解析
// @Description 使用自建 OSM Nominatim 将经纬度解析为地址。
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
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("map client is not configured")))
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
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("reverse geocode: %w", err)))
		return
	}

	ctx.JSON(http.StatusOK, reverseGeocodeResponse{
		Address:          result.Address,
		FormattedAddress: result.FormattedAddress,
		Province:         result.Province,
		City:             result.City,
		District:         result.District,
		Street:           result.Street,
		StreetNumber:     result.StreetNumber,
	})
}

// getBicyclingRoute uses self-hosted OSRM to return cycling route.
// @Summary 自建 OSM 骑行路线
// @Description 调用自建 OSRM /route 获取骑行距离与耗时。
// @Tags 位置
// @Produce json
// @Param from query string true "起点坐标，格式: lat,lng" example("39.908722,116.397499")
// @Param to query string true "终点坐标，格式: lat,lng" example("39.914722,116.404499")
// @Success 200 {object} routeAPIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/location/direction/bicycling [get]
func (server *Server) getBicyclingRoute(ctx *gin.Context) {
	if server.mapClient == nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("map client is not configured")))
		return
	}

	from := ctx.Query("from")
	to := ctx.Query("to")
	if from == "" || to == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("from and to are required")))
		return
	}

	fromLat, fromLng, err := parseLocationPair(from)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("invalid from: %w", err)))
		return
	}
	toLat, toLng, err := parseLocationPair(to)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("invalid to: %w", err)))
		return
	}

	route, err := server.mapClient.GetBicyclingRoute(ctx, maps.Location{Lat: fromLat, Lng: fromLng}, maps.Location{Lat: toLat, Lng: toLng})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("bicycling route: %w", err)))
		return
	}

	ctx.JSON(http.StatusOK, route)
}

// matchRegionID 根据经纬度匹配 region_id
// 优先使用自建 OSM 逆地址解析获取 adcode 匹配，失败则回退到球面距离最近匹配
func (server *Server) matchRegionID(ctx context.Context, lat, lon float64) (int64, error) {
	// 1. 尝试通过地图 API 获取 adcode
	if server.mapClient != nil {
		res, err := server.mapClient.ReverseGeocode(ctx, maps.Location{Lat: lat, Lng: lon})
		if err == nil {
			if res.Adcode != "" {
				// 先尝试 provider + adcode 映射匹配
				if res.Provider != "" {
					region, err := server.store.GetRegionByProviderCode(ctx, db.GetRegionByProviderCodeParams{
						Provider:     res.Provider,
						ExternalCode: res.Adcode,
					})
					if err == nil {
						return region.ID, nil
					}
				}
				// 再尝试 adcode 精确匹配（部分地图服务可返回行政区划码）
				region, err := server.store.GetRegionByCode(ctx, res.Adcode)
				if err == nil {
					return region.ID, nil
				}
			}

			// OSM 逆地理编码的 adcode 可能是邮编，改用名称匹配作为兜底
			var cityRegion *db.Region
			if res.City != "" {
				if city, err := server.store.GetRegionByNameAndLevel(ctx, db.GetRegionByNameAndLevelParams{
					Name:  res.City,
					Level: 2,
				}); err == nil {
					cityRegion = &city
				}
			}

			if res.District != "" {
				if cityRegion != nil {
					if district, err := server.store.GetRegionByNameAndParent(ctx, db.GetRegionByNameAndParentParams{
						Name:     res.District,
						ParentID: pgtype.Int8{Int64: cityRegion.ID, Valid: true},
					}); err == nil {
						return district.ID, nil
					}
				}

				if district, err := server.store.GetRegionByNameAndLevel(ctx, db.GetRegionByNameAndLevelParams{
					Name:  res.District,
					Level: 3,
				}); err == nil {
					return district.ID, nil
				}
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
