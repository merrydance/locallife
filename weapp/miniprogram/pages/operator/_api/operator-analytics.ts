/**
 * 运营商数据统计接口
 * 后端契约是统计字段的唯一真值，页面层不得补造未返回的健康度/洞察字段。
 */

import { request } from '../../../utils/request'

// ==================== 区域统计相关类型 ====================

/** 实时统计响应 - 对齐 api.operatorRealtimeStatsResponse */
export interface OperatorRealtimeStatsResponse {
    active_merchant_count: number
    active_rider_count: number
    pending_merchant_count: number
    pending_rider_count: number
}

/** 区域统计响应 - 对齐 api.regionStatsResponse */
export interface OperatorRegionStatsResponse {
    region_id: number
    region_name: string
    merchant_count: number
    total_orders: number
    total_gmv: number
    total_commission: number
}

/** 趋势分析响应 - 对齐后端 /v1/operator/trend/daily（数组） */
export type OperatorTrendDailyResponse = OperatorDailyTrendItem[]

/** 日趋势项 - 基于swagger api.operatorDailyTrendItem */
export interface OperatorDailyTrendItem {
    date: string
    merchant_count: number
    active_merchant_count: number
    rider_count: number
    online_rider_count: number
    order_count: number
    completed_order_count: number
    cancelled_order_count: number
    total_gmv: number
    avg_order_value: number
    completion_rate: number
    total_commission: number
    operator_income: number  // 运营商当日可得金额（后端计算）
    weather?: string
    special_events?: string[]
}

// ==================== 运营商数据统计服务类 ====================

/**
 * 运营商数据统计服务
 * 提供区域统计、趋势分析等功能
 */
export class OperatorAnalyticsService {
    /**
     * 获取区域统计数据
     * @param regionId 区域ID
     * @param startDate 开始日期
     * @param endDate 结束日期
     */
    async getRegionStats(regionId: number, startDate: string, endDate: string): Promise<OperatorRegionStatsResponse> {
        return request({
            url: `/v1/operator/regions/${regionId}/stats`,
            method: 'GET',
            data: {
                start_date: startDate,
                end_date: endDate
            }
        })
    }

    /**
     * 获取日趋势分析数据
     * @param regionId 区域ID（可选）
     * @param startDate 开始日期
     * @param endDate 结束日期
     */
    async getDailyTrend(regionId?: number, startDate?: string, endDate?: string): Promise<OperatorTrendDailyResponse> {
        const data: Record<string, string | number> = {}
        if (typeof regionId === 'number') data.region_id = regionId
        if (startDate) data.start_date = startDate
        if (endDate) data.end_date = endDate

        return request({
            url: '/v1/operator/trend/daily',
            method: 'GET',
            data
        })
    }

    /**
     * 获取实时统计数据
     */
    async getRealtimeStats(regionId?: number): Promise<OperatorRealtimeStatsResponse> {
        return request({
            url: '/v1/operator/stats/realtime',
            method: 'GET',
            data: regionId ? { region_id: regionId } : undefined
        })
    }
}

export const operatorAnalyticsService = new OperatorAnalyticsService()
