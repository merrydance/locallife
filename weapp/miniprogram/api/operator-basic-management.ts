/**
 * 运营商基础管理接口重构 (Task 4.1)
 * 基于swagger.json完全重构，移除所有没有后端支持的旧功能
 * 包含：区域管理、财务概览、佣金管理
 */

import { request } from '../utils/request'

// ==================== 数据类型定义 ====================

/** 区域状态枚举 */
export type RegionStatus = 'active' | 'inactive' | 'pending'
export type RegionStatusTheme = 'primary' | 'default' | 'warning'

/** 运营商状态枚举 */
export type OperatorStatus = 'active' | 'suspended' | 'pending'

// ==================== 区域管理相关类型 ====================

/** 区域信息响应 - 对齐 api.regionResponse */
export interface RegionResponse {
    code: string
    id: number
    latitude: string
    level: number
    longitude: string
    name: string
    parent_id?: number
    status?: RegionStatus
    operator_id?: number
    created_at?: string
    updated_at?: string
}

/** 区域统计响应 - 对齐 api.regionStatsResponse */
export interface RegionStatsResponse {
    merchant_count: number
    region_id: number
    region_name: string
    total_commission: number
    total_gmv: number
    total_orders: number
    completed_order_count?: number
    order_count?: number
    completion_rate?: number
    active_merchant_count?: number
    rider_count?: number
    active_rider_count?: number
    avg_order_value?: number
    created_at?: string
}

/** 区域查询参数 */
export interface RegionQueryParams extends Record<string, unknown> {
    parent_id?: number
    level?: number
    status?: RegionStatus
    keyword?: string
    page?: number
    limit?: number
}

// ==================== 财务概览相关类型 ====================

/** 运营商财务概览响应 - 对齐 api.operatorFinanceOverviewResponse */
export interface OperatorFinanceOverviewResponse {
    current_month: {
        pending_commission: number    // 待分账佣金
        settled_commission: number    // 已完成分账佣金
        total_commission: number      // 平台佣金
        operator_income: number       // 运营商可得金额（佣金 * 分成比例）
        total_gmv: number             // 区域总交易额
        total_orders: number          // 订单数
    }
    region_id: number                 // 区域ID
    region_name: string               // 区域名称
    operator_share_ratio: number      // 运营商分成比例（如 0.6 表示 60%）
    total: {
        settled_commission: number    // 已结算
        total_commission: number      // 累计平台佣金
        operator_income: number       // 累计运营商可得金额
        total_gmv: number             // 累计交易额
    }
}

/** 佣金明细响应 - 对齐 api.operatorCommissionResponse */
export interface OperatorCommissionResponse {
    items: OperatorCommissionItem[]
    limit: number
    page: number
    summary: {
        total_commission: number
        total_gmv: number
        total_orders: number
    }
    total: number
    id?: number
    operator_id?: number
    order_id?: number
    merchant_id?: number
    commission_amount?: number
    commission_rate?: number
    order_amount?: number
    settlement_status?: 'pending' | 'settled' | 'cancelled'
    settlement_date?: string
    created_at?: string
}

/** 佣金明细项 - 对齐 api.operatorCommissionItem */
export interface OperatorCommissionItem {
    commission: number                // 佣金金额
    commission_rate: string           // 如 "3%"
    date: string
    order_count: number
    total_gmv: number
}

/** 佣金查询参数 */
export interface CommissionQueryParams extends Record<string, unknown> {
    start_date?: string
    end_date?: string
    merchant_id?: number
    settlement_status?: 'pending' | 'settled' | 'cancelled'
    page?: number
    limit?: number
}

// ==================== 运营商信息相关类型 ====================

/** 运营商信息响应 - 基于swagger api.operatorResponse */
export interface OperatorResponse {
    id: number
    user_id: number
    name: string
    phone: string
    email?: string
    region_ids: number[]
    status: OperatorStatus
    commission_rate: number
    created_at: string
    updated_at: string
}

/** 运营商更新请求 */
export interface UpdateOperatorRequest extends Record<string, unknown> {
    name?: string
    phone?: string
    email?: string
    commission_rate?: number
}

export type OperatorFoodSafetyCaseStatus = 'merchant-suspended' | 'investigating' | 'resolved'
export type OperatorFoodSafetyCaseStatusTheme = 'danger' | 'warning' | 'success'

export interface OperatorFoodSafetyCaseItem {
    id: number
    merchant_id: number
    region_id: number
    primary_product_key: string
    primary_product_label: string
    status: OperatorFoodSafetyCaseStatus
    trigger_reason: string
    investigation_report?: string
    merchant_rectification_report?: string
    resolution?: string
    suspended_at: string
    resolved_at?: string
    created_at: string
    updated_at: string
}

export interface OperatorFoodSafetyIncidentItem {
    id: number
    order_id: number
    merchant_id: number
    user_id: number
    incident_type: string
    description: string
    status: OperatorFoodSafetyCaseStatus
    primary_product_key: string
    primary_product_label: string
    case_id?: number
    created_at: string
    resolved_at?: string
}

export interface OperatorFoodSafetyCaseListResponse {
    items: OperatorFoodSafetyCaseItem[]
    page: number
    limit: number
    has_more: boolean
    total: number
}

export interface OperatorFoodSafetyCaseDetailResponse {
    case: OperatorFoodSafetyCaseItem
    incidents: OperatorFoodSafetyIncidentItem[]
}

export interface InvestigateOperatorFoodSafetyCaseRequest extends Record<string, unknown> {
    investigation_report: string
}

export interface ResolveOperatorFoodSafetyCaseRequest extends Record<string, unknown> {
    investigation_report?: string
    merchant_rectification_report: string
    resolution: string
}

export function getFoodSafetyCaseStatusLabel(status: OperatorFoodSafetyCaseStatus): string {
    const labels: Record<OperatorFoodSafetyCaseStatus, string> = {
        'merchant-suspended': '待调查',
        investigating: '调查中',
        resolved: '已结案'
    }
    return labels[status] || status
}

export function getFoodSafetyCaseStatusTheme(status: OperatorFoodSafetyCaseStatus): OperatorFoodSafetyCaseStatusTheme {
    const themes: Record<OperatorFoodSafetyCaseStatus, OperatorFoodSafetyCaseStatusTheme> = {
        'merchant-suspended': 'danger',
        investigating: 'warning',
        resolved: 'success'
    }
    return themes[status] || 'warning'
}

export function getFoodSafetyCaseStatusDisplay(status: OperatorFoodSafetyCaseStatus): {
    label: string
    theme: OperatorFoodSafetyCaseStatusTheme
    isActive: boolean
    isResolved: boolean
} {
    return {
        label: getFoodSafetyCaseStatusLabel(status),
        theme: getFoodSafetyCaseStatusTheme(status),
        isActive: status !== 'resolved',
        isResolved: status === 'resolved'
    }
}

// ==================== 运营商基础管理服务类 ====================

/**
 * 运营商基础管理服务
 * 提供区域管理、财务概览、运营商信息管理等功能
 */
export class OperatorBasicManagementService {
    /**
     * 获取运营商管理的区域列表
     * @param params 查询参数
     */
    async getOperatorRegions(params?: RegionQueryParams): Promise<{
        regions: RegionResponse[]
        total: number
        page: number
        limit: number
        has_more: boolean
    }> {
        return request({
            url: '/v1/operator/regions',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取指定区域的统计数据
     * @param regionId 区域ID
     * @param startDate 开始日期
     * @param endDate 结束日期
     */
    async getRegionStats(regionId: number, startDate?: string, endDate?: string): Promise<RegionStatsResponse> {
        // 如果未提供日期，默认为最近30天
        const end = endDate || new Date().toISOString().split('T')[0]
        const start = startDate || new Date(Date.now() - 30 * 24 * 60 * 60 * 1000).toISOString().split('T')[0]

        return request({
            url: `/v1/operator/regions/${regionId}/stats`,
            method: 'GET',
            data: {
                start_date: start,
                end_date: end
            }
        })
    }

    /**
     * 获取运营商财务概览
     * @param startDate 开始日期
     * @param endDate 结束日期
     */
    async getFinanceOverview(startDate?: string, endDate?: string, regionId?: number): Promise<OperatorFinanceOverviewResponse> {
        const data: Record<string, string | number> = {}
        if (startDate) data.start_date = startDate
        if (endDate) data.end_date = endDate
        if (regionId) data.region_id = regionId

        return request({
            url: '/v1/operators/me/finance/overview',
            method: 'GET',
            data
        })
    }

    /**
     * 获取佣金明细列表
     * @param params 查询参数
     */
    async getCommissionList(params: CommissionQueryParams): Promise<{
        commissions: OperatorCommissionResponse[]
        total: number
        page: number
        limit: number
        has_more: boolean
    }> {
        // 确保有日期参数，默认为最近30天
        const end = params.end_date || new Date().toISOString().split('T')[0]
        const start = params.start_date || new Date(Date.now() - 30 * 24 * 60 * 60 * 1000).toISOString().split('T')[0]

        return request({
            url: '/v1/operators/me/commission',
            method: 'GET',
            data: {
                ...params,
                start_date: start,
                end_date: end
            }
        })
    }

    /**
     * 获取运营商信息
     */
    async getOperatorInfo(): Promise<OperatorResponse> {
        return request({
            url: '/v1/operators/me',
            method: 'GET'
        })
    }

    /**
     * 更新运营商信息
     * @param updateData 更新数据
     */
    async updateOperatorInfo(updateData: UpdateOperatorRequest): Promise<OperatorResponse> {
        return request({
            url: '/v1/operators/me',
            method: 'PATCH',
            data: updateData
        })
    }

    async getFoodSafetyCases(params?: {
        page?: number
        limit?: number
        status?: OperatorFoodSafetyCaseStatus
    }): Promise<OperatorFoodSafetyCaseListResponse> {
        const query: {
            page?: number
            limit?: number
            status?: OperatorFoodSafetyCaseStatus
        } = {
            page: params?.page,
            limit: params?.limit
        }
        if (params?.status) {
            query.status = params.status
        }

        return request({
            url: '/v1/operator/food-safety/cases',
            method: 'GET',
            data: query
        })
    }

    async getFoodSafetyCaseDetail(caseId: number): Promise<OperatorFoodSafetyCaseDetailResponse> {
        return request({
            url: `/v1/operator/food-safety/cases/${caseId}`,
            method: 'GET'
        })
    }

    async investigateFoodSafetyCase(caseId: number, data: InvestigateOperatorFoodSafetyCaseRequest): Promise<OperatorFoodSafetyCaseItem> {
        return request({
            url: `/v1/operator/food-safety/cases/${caseId}/investigate`,
            method: 'POST',
            data
        })
    }

    async resolveFoodSafetyCase(caseId: number, data: ResolveOperatorFoodSafetyCaseRequest): Promise<OperatorFoodSafetyCaseItem> {
        return request({
            url: `/v1/operator/food-safety/cases/${caseId}/resolve`,
            method: 'POST',
            data
        })
    }
}

// ==================== 区域统计分析服务类 ====================

/**
 * 区域统计分析服务
 * 提供区域数据分析、对比等功能
 */
export class RegionAnalyticsService {
    /**
     * 计算区域绩效指标
     * @param stats 区域统计数据
     */
    calculateRegionPerformance(stats: RegionStatsResponse): {
        merchantDensity: number
        riderDensity: number
        orderDensity: number
        avgOrderValue: number
        completionRate: number
        commissionRate: number
        performanceScore: number
        performanceLevel: 'excellent' | 'good' | 'average' | 'poor'
    } {
        const activeMerchantCount = stats.active_merchant_count ?? 0
        const activeRiderCount = stats.active_rider_count ?? 0
        const completedOrderCount = stats.completed_order_count ?? 0
        const orderCount = stats.order_count ?? stats.total_orders ?? 0
        const completionRate = stats.completion_rate ?? 0

        const merchantDensity = activeMerchantCount / Math.max(stats.merchant_count ?? 0, 1)
        const riderDensity = activeRiderCount / Math.max(stats.rider_count ?? 0, 1)
        const orderDensity = completedOrderCount / Math.max(orderCount, 1)
        const avgOrderValue = stats.total_gmv / Math.max(completedOrderCount, 1)
        const commissionRate = stats.total_commission / Math.max(stats.total_gmv ?? 0, 1)

        // 计算综合绩效分数 (0-100)
        const performanceScore = Math.min(100, Math.round(
            merchantDensity * 20 +
            riderDensity * 20 +
            orderDensity * 25 +
            (completionRate / 100) * 25 +
            Math.min(commissionRate * 1000, 10) // 佣金率权重较小
        ))

        let performanceLevel: 'excellent' | 'good' | 'average' | 'poor' = 'poor'
        if (performanceScore >= 80) performanceLevel = 'excellent'
        else if (performanceScore >= 65) performanceLevel = 'good'
        else if (performanceScore >= 50) performanceLevel = 'average'

        return {
            merchantDensity,
            riderDensity,
            orderDensity,
            avgOrderValue,
            completionRate,
            commissionRate,
            performanceScore,
            performanceLevel
        }
    }

    /**
     * 对比多个区域的绩效
     * @param regionStats 多个区域的统计数据
     */
    compareRegionPerformance(regionStats: RegionStatsResponse[]): {
        bestRegion: RegionStatsResponse | null
        worstRegion: RegionStatsResponse | null
        avgPerformance: number
        regionRankings: Array<{
            region: RegionStatsResponse
            performance: ReturnType<RegionAnalyticsService['calculateRegionPerformance']>
            rank: number
        }>
    } {
        if (regionStats.length === 0) {
            return {
                bestRegion: null,
                worstRegion: null,
                avgPerformance: 0,
                regionRankings: []
            }
        }

        const regionRankings = regionStats
            .map((region) => ({
                region,
                performance: this.calculateRegionPerformance(region),
                rank: 0
            }))
            .sort((a, b) => b.performance.performanceScore - a.performance.performanceScore)
            .map((item, index) => ({ ...item, rank: index + 1 }))

        const avgPerformance = regionRankings.reduce(
            (sum, item) => sum + item.performance.performanceScore, 0
        ) / regionRankings.length

        return {
            bestRegion: regionRankings[0]?.region || null,
            worstRegion: regionRankings[regionRankings.length - 1]?.region || null,
            avgPerformance,
            regionRankings
        }
    }

    /**
     * 分析区域增长趋势
     * @param currentStats 当前统计数据
     * @param previousStats 上期统计数据
     */
    analyzeRegionGrowth(currentStats: RegionStatsResponse, previousStats: RegionStatsResponse): {
        merchantGrowth: number
        riderGrowth: number
        orderGrowth: number
        gmvGrowth: number
        commissionGrowth: number
        overallGrowth: number
        growthTrend: 'up' | 'down' | 'stable'
    } {
        const merchantGrowth = this.calculateGrowthRate(
            currentStats.active_merchant_count ?? 0,
            previousStats.active_merchant_count ?? 0
        )
        const riderGrowth = this.calculateGrowthRate(
            currentStats.active_rider_count ?? 0,
            previousStats.active_rider_count ?? 0
        )
        const orderGrowth = this.calculateGrowthRate(
            currentStats.completed_order_count ?? 0,
            previousStats.completed_order_count ?? 0
        )
        const gmvGrowth = this.calculateGrowthRate(
            currentStats.total_gmv ?? 0,
            previousStats.total_gmv ?? 0
        )
        const commissionGrowth = this.calculateGrowthRate(
            currentStats.total_commission ?? 0,
            previousStats.total_commission ?? 0
        )

        const overallGrowth = (merchantGrowth + riderGrowth + orderGrowth + gmvGrowth + commissionGrowth) / 5

        let growthTrend: 'up' | 'down' | 'stable' = 'stable'
        if (overallGrowth > 5) growthTrend = 'up'
        else if (overallGrowth < -5) growthTrend = 'down'

        return {
            merchantGrowth,
            riderGrowth,
            orderGrowth,
            gmvGrowth,
            commissionGrowth,
            overallGrowth,
            growthTrend
        }
    }

    /**
     * 计算增长率
     * @param current 当前值
     * @param previous 上期值
     */
    private calculateGrowthRate(current: number, previous: number): number {
        if (previous === 0) return current > 0 ? 100 : 0
        return ((current - previous) / previous) * 100
    }
}

// ==================== 数据适配器 ====================

/**
 * 运营商基础管理数据适配器
 * 处理前端数据格式与后端API数据格式的转换
 */
export class OperatorBasicManagementAdapter {
    /**
     * 适配区域响应数据
     */
    static adaptRegionResponse(data: RegionResponse): {
        id: number
        name: string
        code: string
        parentId?: number
        level: number
        status: RegionStatus
        status_label: string
        status_theme: RegionStatusTheme
        is_active: boolean
        operatorId?: number
        createdAt: string
        updatedAt: string
    } {
        const status = data.status ?? 'pending'
        const statusDisplay = getRegionStatusDisplay(status)
        return {
            id: data.id,
            name: data.name,
            code: data.code,
            parentId: data.parent_id,
            level: data.level,
            status,
            status_label: statusDisplay.label,
            status_theme: statusDisplay.theme,
            is_active: statusDisplay.isActive,
            operatorId: data.operator_id,
            createdAt: data.created_at ?? '',
            updatedAt: data.updated_at ?? ''
        }
    }

    /**
     * 适配区域统计响应数据
     */
    static adaptRegionStatsResponse(data: RegionStatsResponse): {
        regionId: number
        regionName: string
        merchantCount: number
        activeMerchantCount: number
        riderCount: number
        activeRiderCount: number
        orderCount: number
        completedOrderCount: number
        totalGmv: number
        totalCommission: number
        avgOrderValue: number
        completionRate: number
        createdAt: string
    } {
        return {
            regionId: data.region_id,
            regionName: data.region_name,
            merchantCount: data.merchant_count ?? 0,
            activeMerchantCount: data.active_merchant_count ?? 0,
            riderCount: data.rider_count ?? 0,
            activeRiderCount: data.active_rider_count ?? 0,
            orderCount: data.order_count ?? data.total_orders ?? 0,
            completedOrderCount: data.completed_order_count ?? 0,
            totalGmv: data.total_gmv ?? 0,
            totalCommission: data.total_commission ?? 0,
            avgOrderValue: data.avg_order_value ?? 0,
            completionRate: data.completion_rate ?? 0,
            createdAt: data.created_at ?? ''
        }
    }

    /**
     * 适配财务概览响应数据
     */
    static adaptFinanceOverviewResponse(data: OperatorFinanceOverviewResponse): {
        totalCommission: number
        todayCommission: number
        weekCommission: number
        monthCommission: number
        pendingSettlement: number
        settledAmount: number
        commissionRate: number
        merchantCount: number
        activeMerchantCount: number
        orderCount: number
        gmv: number
    } {
        return {
            totalCommission: data.total?.total_commission ?? 0,
            todayCommission: 0,
            weekCommission: 0,
            monthCommission: data.current_month?.total_commission ?? 0,
            pendingSettlement: data.current_month?.pending_commission ?? 0,
            settledAmount: data.total?.settled_commission ?? 0,
            commissionRate: data.operator_share_ratio ?? 0,
            merchantCount: 0,
            activeMerchantCount: 0,
            orderCount: data.current_month?.total_orders ?? 0,
            gmv: data.total?.total_gmv ?? 0
        }
    }

    /**
     * 适配佣金响应数据
     */
    static adaptCommissionResponse(data: OperatorCommissionResponse): {
        id: number
        operatorId: number
        orderId: number
        merchantId: number
        commissionAmount: number
        commissionRate: number
        orderAmount: number
        settlementStatus: 'pending' | 'settled' | 'cancelled'
        settlementDate?: string
        createdAt: string
    } {
        return {
            id: data.id ?? 0,
            operatorId: data.operator_id ?? 0,
            orderId: data.order_id ?? 0,
            merchantId: data.merchant_id ?? 0,
            commissionAmount: data.commission_amount ?? 0,
            commissionRate: data.commission_rate ?? 0,
            orderAmount: data.order_amount ?? 0,
            settlementStatus: data.settlement_status ?? 'pending',
            settlementDate: data.settlement_date,
            createdAt: data.created_at ?? ''
        }
    }

    /**
     * 适配运营商响应数据
     */
    static adaptOperatorResponse(data: OperatorResponse): {
        id: number
        userId: number
        name: string
        phone: string
        email?: string
        regionIds: number[]
        status: OperatorStatus
        commissionRate: number
        createdAt: string
        updatedAt: string
    } {
        return {
            id: data.id,
            userId: data.user_id,
            name: data.name,
            phone: data.phone,
            email: data.email,
            regionIds: data.region_ids,
            status: data.status,
            commissionRate: data.commission_rate,
            createdAt: data.created_at,
            updatedAt: data.updated_at
        }
    }
}

// ==================== 导出服务实例 ====================

export const operatorBasicManagementService = new OperatorBasicManagementService()
export const regionAnalyticsService = new RegionAnalyticsService()

// ==================== 便捷函数 ====================

/**
 * 获取运营商工作台数据
 */
export async function getOperatorDashboard(): Promise<{
    operatorInfo: OperatorResponse
    financeOverview: OperatorFinanceOverviewResponse
    regionStats: RegionStatsResponse[]
    regionPerformance: ReturnType<RegionAnalyticsService['compareRegionPerformance']>
    recentCommissions: OperatorCommissionResponse[]
}> {
    const [operatorInfo, financeOverview, regions] = await Promise.all([
        operatorBasicManagementService.getOperatorInfo(),
        operatorBasicManagementService.getFinanceOverview(),
        operatorBasicManagementService.getOperatorRegions({ limit: 100 })
    ])

    // 获取各区域统计数据（单个区域失败不影响整体，兼容 ES2018）
    const regionStatsPromises = regions.regions.map((region) =>
        operatorBasicManagementService.getRegionStats(region.id).catch(() => null)
    )
    const regionStatsRaw = await Promise.all(regionStatsPromises)
    const regionStats = regionStatsRaw.filter((s): s is RegionStatsResponse => s !== null)

    // 分析区域绩效
    const regionPerformance = regionAnalyticsService.compareRegionPerformance(regionStats)

    // 获取最近的佣金记录
    const commissionResult = await operatorBasicManagementService.getCommissionList({
        page: 1,
        limit: 10
    })

    return {
        operatorInfo,
        financeOverview,
        regionStats,
        regionPerformance,
        recentCommissions: commissionResult.commissions
    }
}

/**
 * 格式化区域状态显示
 * @param status 区域状态
 */
export function formatRegionStatus(status: RegionStatus): string {
    const statusMap: Record<RegionStatus, string> = {
        active: '正常',
        inactive: '停用',
        pending: '待审核'
    }
    return statusMap[status] || status
}

export function getRegionStatusDisplay(status?: RegionStatus | string) {
    const normalizedStatus = (status || 'pending') as RegionStatus
    const themeMap: Record<RegionStatus, RegionStatusTheme> = {
        active: 'primary',
        inactive: 'default',
        pending: 'warning'
    }

    return {
        normalizedStatus,
        label: normalizedStatus === 'active' ? '运营中' : normalizedStatus === 'inactive' ? '已停用' : '待审核',
        theme: themeMap[normalizedStatus] || 'default',
        isActive: normalizedStatus === 'active'
    }
}

/**
 * 格式化运营商状态显示
 * @param status 运营商状态
 */
export function formatOperatorStatus(status: OperatorStatus): string {
    const statusMap: Record<OperatorStatus, string> = {
        active: '正常',
        suspended: '暂停',
        pending: '待审核'
    }
    return statusMap[status] || status
}

/**
 * 格式化佣金结算状态显示
 * @param status 结算状态
 */
export function formatSettlementStatus(status: 'pending' | 'settled' | 'cancelled'): string {
    const statusMap = {
        pending: '待结算',
        settled: '已结算',
        cancelled: '已取消'
    }
    return statusMap[status] || status
}

/**
 * 格式化金额显示
 * @param amount 金额（分）
 * @param showUnit 是否显示单位
 */
export function formatAmount(amount: number, showUnit: boolean = true): string {
    const yuan = (amount / 100).toFixed(2)
    return showUnit ? `¥${yuan}` : yuan
}

/**
 * 格式化百分比显示
 * @param value 数值
 * @param decimals 小数位数
 */
export function formatPercentage(value: number, decimals: number = 1): string {
    return `${value.toFixed(decimals)}%`
}

/**
 * 格式化增长率显示
 * @param growth 增长率
 * @param showSign 是否显示正负号
 */
export function formatGrowthRate(growth: number, showSign: boolean = true): string {
    const sign = showSign && growth > 0 ? '+' : ''
    return `${sign}${growth.toFixed(1)}%`
}

/**
 * 验证区域查询参数
 * @param params 查询参数
 */
export function validateRegionQueryParams(params: RegionQueryParams): { valid: boolean, message?: string } {
    if (params.page && params.page < 1) {
        return { valid: false, message: '页码必须大于0' }
    }

    if (params.limit && (params.limit < 1 || params.limit > 100)) {
        return { valid: false, message: '每页数量必须在1-100之间' }
    }

    if (params.level && (params.level < 1 || params.level > 5)) {
        return { valid: false, message: '区域级别必须在1-5之间' }
    }

    return { valid: true }
}

/**
 * 验证佣金查询参数
 * @param params 查询参数
 */
export function validateCommissionQueryParams(params: CommissionQueryParams): { valid: boolean, message?: string } {
    if (params.start_date && params.end_date) {
        const startDate = new Date(params.start_date)
        const endDate = new Date(params.end_date)

        if (startDate > endDate) {
            return { valid: false, message: '开始日期不能晚于结束日期' }
        }

        const daysDiff = (endDate.getTime() - startDate.getTime()) / (1000 * 60 * 60 * 24)
        if (daysDiff > 365) {
            return { valid: false, message: '查询时间范围不能超过365天' }
        }
    }

    if (params.page && params.page < 1) {
        return { valid: false, message: '页码必须大于0' }
    }

    if (params.limit && (params.limit < 1 || params.limit > 100)) {
        return { valid: false, message: '每页数量必须在1-100之间' }
    }

    return { valid: true }
}