package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/rs/zerolog/log"
)

type operatorRealtimeStatsResponse struct {
	ActiveMerchantCount  int32 `json:"active_merchant_count"`
	ActiveRiderCount     int32 `json:"active_rider_count"`
	PendingMerchantCount int32 `json:"pending_merchant_count"`
	PendingRiderCount    int32 `json:"pending_rider_count"`
}

// getOperatorRealtimeStats 获取运营商实时统计数据
// @Summary 获取实时统计
// @Description 获取当前活跃的商户数、骑手数以及待审核数量
// @Tags 运营商数据统计
// @Accept json
// @Produce json
// @Success 200 {object} operatorRealtimeStatsResponse "实时统计数据"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Security BearerAuth
// @Router /v1/operator/stats/realtime [get]
func (server *Server) getOperatorRealtimeStats(ctx *gin.Context) {
	// 优先使用前端传入的 region_id（区域切换场景），无则取默认区域
	var regionID int64
	if qRegionID := ctx.Query("region_id"); qRegionID != "" {
		parsed, err := strconv.ParseInt(qRegionID, 10, 64)
		if err != nil || parsed <= 0 {
			ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("invalid region_id: %s", qRegionID)))
			return
		}
		if _, err := server.checkOperatorManagesRegion(ctx, parsed); err != nil {
			ctx.JSON(http.StatusForbidden, errorResponse(err))
			return
		}
		regionID = parsed
	}
	if regionID == 0 {
		var err error
		regionID, err = server.getOperatorRegionID(ctx)
		if err != nil {
			ctx.JSON(http.StatusForbidden, errorResponse(err))
			return
		}
	}

	// 并发获取各项数据
	type result struct {
		count int32
		err   error
	}

	activeMerchantsChan := make(chan result)
	activeRidersChan := make(chan result)
	pendingMerchantsChan := make(chan result)
	pendingRidersChan := make(chan result)

	// 1. 活跃商户数 (状态为 approved)
	go func() {
		count, err := server.store.CountMerchantsByRegionWithStatus(ctx, db.CountMerchantsByRegionWithStatusParams{
			RegionID: regionID,
			Column2:  db.MerchantStatusApproved,
		})
		activeMerchantsChan <- result{count: int32(count), err: err}
	}()

	// 2. 活跃骑手数 (状态为 active)
	go func() {
		count, err := server.store.CountRidersByRegionWithStatus(ctx, db.CountRidersByRegionWithStatusParams{
			RegionID: pgtype.Int8{Int64: regionID, Valid: true},
			Status:   db.RiderStatusActive,
		})
		activeRidersChan <- result{count: int32(count), err: err}
	}()

	// 3. 待审核商户数
	go func() {
		count, err := server.store.CountMerchantsByRegionWithStatus(ctx, db.CountMerchantsByRegionWithStatusParams{
			RegionID: regionID,
			Column2:  "pending",
		})
		pendingMerchantsChan <- result{count: int32(count), err: err}
	}()

	// 4. 待审核骑手数
	go func() {
		count, err := server.store.CountRidersByRegionWithStatus(ctx, db.CountRidersByRegionWithStatusParams{
			RegionID: pgtype.Int8{Int64: regionID, Valid: true},
			Status:   db.RiderStatusPendingApproval,
		})
		pendingRidersChan <- result{count: int32(count), err: err}
	}()

	// 收集结果
	activeMerchantsRes := <-activeMerchantsChan
	activeRidersRes := <-activeRidersChan
	pendingMerchantsRes := <-pendingMerchantsChan
	pendingRidersRes := <-pendingRidersChan

	// 任一子查询失败则记录日志并返回错误，避免静默返回不可信 0 值
	for _, r := range []struct {
		label string
		res   result
	}{
		{"active_merchants", activeMerchantsRes},
		{"active_riders", activeRidersRes},
		{"pending_merchants", pendingMerchantsRes},
		{"pending_riders", pendingRidersRes},
	} {
		if r.res.err != nil {
			log.Error().Err(r.res.err).Str("query", r.label).Msg("realtime stats query failed")
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, r.res.err))
			return
		}
	}

	response := operatorRealtimeStatsResponse{
		ActiveMerchantCount:  activeMerchantsRes.count,
		ActiveRiderCount:     activeRidersRes.count,
		PendingMerchantCount: pendingMerchantsRes.count,
		PendingRiderCount:    pendingRidersRes.count,
	}

	ctx.JSON(http.StatusOK, response)
}
