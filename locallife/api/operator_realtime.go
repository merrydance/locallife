package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
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
	// 获取运营商管理的区域ID
	regionID, err := server.getOperatorRegionID(ctx)
	if err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(err))
		return
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
			Column2:  "approved",
		})
		activeMerchantsChan <- result{count: int32(count), err: err}
	}()

	// 2. 活跃骑手数 (状态为 active)
	go func() {
		count, err := server.store.CountRidersByRegionWithStatus(ctx, db.CountRidersByRegionWithStatusParams{
			RegionID: pgtype.Int8{Int64: regionID, Valid: true},
			Status:   "active",
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
			Status:   "pending",
		})
		pendingRidersChan <- result{count: int32(count), err: err}
	}()

	// 收集结果
	activeMerchantsRes := <-activeMerchantsChan
	activeRidersRes := <-activeRidersChan
	pendingMerchantsRes := <-pendingMerchantsChan
	pendingRidersRes := <-pendingRidersChan

	// 错误处理 (记录日志，返回0或报错，这里选择宽容处理，出错返回0)
	response := operatorRealtimeStatsResponse{
		ActiveMerchantCount:  activeMerchantsRes.count,
		ActiveRiderCount:     activeRidersRes.count,
		PendingMerchantCount: pendingMerchantsRes.count,
		PendingRiderCount:    pendingRidersRes.count,
	}

	ctx.JSON(http.StatusOK, response)
}
