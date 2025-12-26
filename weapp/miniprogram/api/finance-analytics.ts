/**
 * 财务和统计分析接口重构 (Task 2.6)
 * 基于swagger.json完全重构，移除所有没有后端支持的旧功能
 * 包含：财务概览、财务明细、统计分析、客户分析、菜品分析
 */

import { request } from '../utils/request'

// ==================== 数据类型定义 ====================

/** 结算状态枚举 */
export type SettlementStatus = 'pending' | 'processing' | 'finished' | 'failed'

/** 排序字段枚举 */
export type CustomerOrderBy = 'total_orders' | 'total_amount' | 'last_order_at'

// ==================== 财务管理相关类型 ====================

/** 财务概览响应 - 基于swagger api.financeOverviewResponse */
export interface FinanceOverviewResponse {
    total_gmv: number
    total_income: number
    net_income: number
    pending_income: number
    total_service_fee: number
    total_platform_fee: number
    total_operator_fee: number
    total_promotion_exp: number
    completed_orders: number
    pending_orders: number
    promotion_orders: number
}

/** 财务查询参数 */
export interface FinanceQueryParams extends Record<string, unknown> {
    start_date: string
    end_date: string
}

/** 财务订单查询参数 */
export interface FinanceOrdersParams extends FinanceQueryParams {
    page?: number
    limit?: number
}

/** 财务结算查询参数 */
export interface FinanceSettlementsParams extends FinanceQueryParams {
    status?: SettlementStatus
    page?: number
    limit?: number
}

// ==================== 统计分析相关类型 ====================

/** 商户概览响应 - 基于swagger api.merchantOverviewResponse */
export interface MerchantOverviewResponse {
    total_days: number
    total_orders: number
    total_sales: number
    total_commission: number
    avg_daily_sales: number
}

/** 日报统计行 - 基于swagger api.dailyStatRow */
export interface DailyStatRow {
    date: string
    order_count: number
    total_sales: number
    commission: number
    takeout_orders: number
    dine_in_orders: number
}

/** 统计查询参数 */
export interface StatsQueryParams extends Record<string, unknown> {
    start_date: string
    end_date: string
}

// ==================== 客户分析相关类型 ====================

/** 客户列表查询参数 */
export interface CustomersQueryParams extends Record<string, unknown> {
    order_by?: CustomerOrderBy
    page?: number
    limit?: number
}

/** 复购率响应 - 基于swagger api.merchantRepurchaseRateResponse */
export interface MerchantRepurchaseRateResponse {
    total_users: number
    repeat_users: number
    repurchase_rate: number
    avg_orders_per_user: number
}

// ==================== 菜品分析相关类型 ====================

/** 热门菜品行 - 基于swagger api.topSellingDishRow */
export interface TopSellingDishRow {
    dish_id: number
    dish_name: string
    dish_price: number
    total_sold: number
    total_revenue: number
}

/** 菜品分类统计行 - 基于swagger api.dishCategoryStatsRow */
export interface DishCategoryStatsRow {
    category_name: string
    order_count: number
    total_quantity: number
    total_sales: number
}

/** 热门菜品查询参数 */
export interface TopDishesParams extends StatsQueryParams {
    limit?: number
}

// ==================== 财务管理服务类 ====================

/**
 * 财务管理服务
 * 提供财务概览、明细查询、结算记录等功能
 */
export class FinanceManagementService {
    /**
     * 获取财务概览
     * @param params 查询参数
     */
    async getFinanceOverview(params: FinanceQueryParams): Promise<FinanceOverviewResponse> {
        return request({
            url: '/v1/merchant/finance/overview',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取每日财务汇总
     * @param params 查询参数
     */
    async getDailyFinanceSummary(params: FinanceQueryParams): Promise<any[]> {
        return request({
            url: '/v1/merchant/finance/daily',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取订单收入明细
     * @param params 查询参数
     */
    async getFinanceOrders(params: FinanceOrdersParams): Promise<any> {
        return request({
            url: '/v1/merchant/finance/orders',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取结算记录
     * @param params 查询参数
     */
    async getSettlements(params: FinanceSettlementsParams): Promise<any> {
        return request({
            url: '/v1/merchant/finance/settlements',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取促销支出明细
     * @param params 查询参数
     */
    async getPromotionExpenses(params: FinanceQueryParams): Promise<any[]> {
        return request({
            url: '/v1/merchant/finance/promotions',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取服务费明细
     * @param params 查询参数
     */
    async getServiceFees(params: FinanceQueryParams): Promise<any[]> {
        return request({
            url: '/v1/merchant/finance/service-fees',
            method: 'GET',
            data: params
        })
    }
}

// ==================== 统计分析服务类 ====================

/**
 * 统计分析服务
 * 提供商户统计概览、日报、时段分析等功能
 */
export class StatsAnalyticsService {
    /**
     * 获取商户概览统计
     * @param params 查询参数
     */
    async getMerchantOverview(params: StatsQueryParams): Promise<MerchantOverviewResponse> {
        return request({
            url: '/v1/merchant/stats/overview',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取商户日报统计
     * @param params 查询参数
     */
    async getDailyStats(params: StatsQueryParams): Promise<DailyStatRow[]> {
        return request({
            url: '/v1/merchant/stats/daily',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取小时分布统计
     * @param params 查询参数
     */
    async getHourlyStats(params: StatsQueryParams): Promise<any[]> {
        return request({
            url: '/v1/merchant/stats/hourly',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取订单来源统计
     * @param params 查询参数
     */
    async getSourceStats(params: StatsQueryParams): Promise<any[]> {
        return request({
            url: '/v1/merchant/stats/sources',
            method: 'GET',
            data: params
        })
    }
}

// ==================== 客户分析服务类 ====================

/**
 * 客户分析服务
 * 提供客户列表、复购率分析等功能
 */
export class CustomerAnalyticsService {
    /**
     * 获取商户顾客列表
     * @param params 查询参数
     */
    async getCustomers(params: CustomersQueryParams): Promise<any> {
        return request({
            url: '/v1/merchant/stats/customers',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取顾客详情
     * @param userId 用户ID
     */
    async getCustomerDetail(userId: number): Promise<any> {
        return request({
            url: `/v1/merchant/stats/customers/${userId}`,
            method: 'GET'
        })
    }

    /**
     * 获取复购率统计
     * @param params 查询参数
     */
    async getRepurchaseRate(params: StatsQueryParams): Promise<MerchantRepurchaseRateResponse> {
        return request({
            url: '/v1/merchant/stats/repurchase',
            method: 'GET',
            data: params
        })
    }
}

// ==================== 菜品分析服务类 ====================

/**
 * 菜品分析服务
 * 提供热门菜品、分类统计等功能
 */
export class DishAnalyticsService {
    /**
     * 获取菜品销量排行
     * @param params 查询参数
     */
    async getTopSellingDishes(params: TopDishesParams): Promise<TopSellingDishRow[]> {
        return request({
            url: '/v1/merchant/stats/dishes/top',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取菜品分类统计
     * @param params 查询参数
     */
    async getCategoryStats(params: StatsQueryParams): Promise<DishCategoryStatsRow[]> {
        return request({
            url: '/v1/merchant/stats/categories',
            method: 'GET',
            data: params
        })
    }
}

// ==================== 数据适配器 ====================

/**
 * 财务和统计分析数据适配器
 * 处理前端数据格式与后端API数据格式的转换
 */
export class FinanceAnalyticsAdapter {
    /**
     * 适配财务概览响应数据
     */
    static adaptFinanceOverviewResponse(data: FinanceOverviewResponse): {
        totalGmv: number
        totalIncome: number
        netIncome: number
        pendingIncome: number
        totalServiceFee: number
        totalPlatformFee: number
        totalOperatorFee: number
        totalPromotionExp: number
        completedOrders: number
        pendingOrders: number
        promotionOrders: number
    } {
        return {
            totalGmv: data.total_gmv,
            totalIncome: data.total_income,
            netIncome: data.net_income,
            pendingIncome: data.pending_income,
            totalServiceFee: data.total_service_fee,
            totalPlatformFee: data.total_platform_fee,
            totalOperatorFee: data.total_operator_fee,
            totalPromotionExp: data.total_promotion_exp,
            completedOrders: data.completed_orders,
            pendingOrders: data.pending_orders,
            promotionOrders: data.promotion_orders
        }
    }

    /**
     * 适配商户概览响应数据
     */
    static adaptMerchantOverviewResponse(data: MerchantOverviewResponse): {
        totalDays: number
        totalOrders: number
        totalSales: number
        totalCommission: number
        avgDailySales: number
    } {
        return {
            totalDays: data.total_days,
            totalOrders: data.total_orders,
            totalSales: data.total_sales,
            totalCommission: data.total_commission,
            avgDailySales: data.avg_daily_sales
        }
    }

    /**
     * 适配日报统计数据
     */
    static adaptDailyStatRow(data: DailyStatRow): {
        date: string
        orderCount: number
        totalSales: number
        commission: number
        takeoutOrders: number
        dineInOrders: number
    } {
        return {
            date: data.date,
            orderCount: data.order_count,
            totalSales: data.total_sales,
            commission: data.commission,
            takeoutOrders: data.takeout_orders,
            dineInOrders: data.dine_in_orders
        }
    }

    /**
     * 适配复购率响应数据
     */
    static adaptRepurchaseRateResponse(data: MerchantRepurchaseRateResponse): {
        totalUsers: number
        repeatUsers: number
        repurchaseRate: number
        avgOrdersPerUser: number
    } {
        return {
            totalUsers: data.total_users,
            repeatUsers: data.repeat_users,
            repurchaseRate: data.repurchase_rate,
            avgOrdersPerUser: data.avg_orders_per_user
        }
    }

    /**
     * 适配热门菜品数据
     */
    static adaptTopSellingDishRow(data: TopSellingDishRow): {
        dishId: number
        dishName: string
        dishPrice: number
        totalSold: number
        totalRevenue: number
    } {
        return {
            dishId: data.dish_id,
            dishName: data.dish_name,
            dishPrice: data.dish_price,
            totalSold: data.total_sold,
            totalRevenue: data.total_revenue
        }
    }

    /**
     * 适配菜品分类统计数据
     */
    static adaptDishCategoryStatsRow(data: DishCategoryStatsRow): {
        categoryName: string
        orderCount: number
        totalQuantity: number
        totalSales: number
    } {
        return {
            categoryName: data.category_name,
            orderCount: data.order_count,
            totalQuantity: data.total_quantity,
            totalSales: data.total_sales
        }
    }
}

// ==================== 导出服务实例 ====================

export const financeManagementService = new FinanceManagementService()
export const statsAnalyticsService = new StatsAnalyticsService()
export const customerAnalyticsService = new CustomerAnalyticsService()
export const dishAnalyticsService = new DishAnalyticsService()

// ==================== 便捷函数 ====================

/**
 * 获取完整的商户分析报告
 * @param startDate 开始日期
 * @param endDate 结束日期
 */
export async function getComprehensiveAnalytics(startDate: string, endDate: string): Promise<{
    financeOverview: FinanceOverviewResponse
    merchantOverview: MerchantOverviewResponse
    dailyStats: DailyStatRow[]
    repurchaseRate: MerchantRepurchaseRateResponse
    topDishes: TopSellingDishRow[]
    categoryStats: DishCategoryStatsRow[]
}> {
    const params = { start_date: startDate, end_date: endDate }

    const [
        financeOverview,
        merchantOverview,
        dailyStats,
        repurchaseRate,
        topDishes,
        categoryStats
    ] = await Promise.all([
        financeManagementService.getFinanceOverview(params),
        statsAnalyticsService.getMerchantOverview(params),
        statsAnalyticsService.getDailyStats(params),
        customerAnalyticsService.getRepurchaseRate(params),
        dishAnalyticsService.getTopSellingDishes({ ...params, limit: 10 }),
        dishAnalyticsService.getCategoryStats(params)
    ])

    return {
        financeOverview,
        merchantOverview,
        dailyStats,
        repurchaseRate,
        topDishes,
        categoryStats
    }
}

/**
 * 计算财务指标
 * @param financeData 财务数据
 */
export function calculateFinanceMetrics(financeData: FinanceOverviewResponse): {
    profitMargin: number
    serviceFeeRate: number
    promotionRate: number
    avgOrderValue: number
} {
    const totalOrders = financeData.completed_orders + financeData.pending_orders

    return {
        // 利润率 = 净收入 / 总GMV
        profitMargin: financeData.total_gmv > 0 ? (financeData.net_income / financeData.total_gmv) * 100 : 0,

        // 服务费率 = 总服务费 / 总GMV
        serviceFeeRate: financeData.total_gmv > 0 ? (financeData.total_service_fee / financeData.total_gmv) * 100 : 0,

        // 促销支出率 = 促销支出 / 总GMV
        promotionRate: financeData.total_gmv > 0 ? (financeData.total_promotion_exp / financeData.total_gmv) * 100 : 0,

        // 平均订单价值 = 总GMV / 总订单数
        avgOrderValue: totalOrders > 0 ? financeData.total_gmv / totalOrders : 0
    }
}

/**
 * 分析销售趋势
 * @param dailyStats 日报统计数据
 */
export function analyzeSalesTrend(dailyStats: DailyStatRow[]): {
    trend: 'up' | 'down' | 'stable'
    growthRate: number
    peakDay: DailyStatRow | null
    avgDailySales: number
} {
    if (dailyStats.length < 2) {
        return {
            trend: 'stable',
            growthRate: 0,
            peakDay: dailyStats[0] || null,
            avgDailySales: dailyStats[0]?.total_sales || 0
        }
    }

    // 计算增长率（最后一天相比第一天）
    const firstDay = dailyStats[0]
    const lastDay = dailyStats[dailyStats.length - 1]
    const growthRate = firstDay.total_sales > 0
        ? ((lastDay.total_sales - firstDay.total_sales) / firstDay.total_sales) * 100
        : 0

    // 确定趋势
    let trend: 'up' | 'down' | 'stable' = 'stable'
    if (growthRate > 5) trend = 'up'
    else if (growthRate < -5) trend = 'down'

    // 找到销售额最高的一天
    const peakDay = dailyStats.reduce((max, current) =>
        current.total_sales > max.total_sales ? current : max
    )

    // 计算平均日销售额
    const avgDailySales = dailyStats.reduce((sum, day) => sum + day.total_sales, 0) / dailyStats.length

    return {
        trend,
        growthRate,
        peakDay,
        avgDailySales
    }
}

/**
 * 生成经营建议
 * @param analytics 分析数据
 */
export function generateBusinessSuggestions(analytics: {
    financeOverview: FinanceOverviewResponse
    repurchaseRate: MerchantRepurchaseRateResponse
    topDishes: TopSellingDishRow[]
    categoryStats: DishCategoryStatsRow[]
}): string[] {
    const suggestions: string[] = []
    const { financeOverview, repurchaseRate, topDishes, categoryStats } = analytics

    // 财务建议
    const metrics = calculateFinanceMetrics(financeOverview)
    if (metrics.profitMargin < 10) {
        suggestions.push('利润率偏低，建议优化成本结构或调整菜品定价')
    }
    if (metrics.promotionRate > 15) {
        suggestions.push('促销支出占比较高，建议评估促销活动效果')
    }

    // 复购率建议
    if (repurchaseRate.repurchase_rate < 30) {
        suggestions.push('客户复购率较低，建议加强会员营销和客户关系维护')
    }

    // 菜品建议
    if (topDishes.length > 0) {
        const topDish = topDishes[0]
        suggestions.push(`${topDish.dish_name}是您的招牌菜品，建议重点推广`)
    }

    // 分类建议
    if (categoryStats.length > 0) {
        const topCategory = categoryStats.reduce((max, current) =>
            current.total_sales > max.total_sales ? current : max
        )
        suggestions.push(`${topCategory.category_name}分类销售表现最佳，建议丰富该分类菜品`)
    }

    return suggestions
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
 * @param rate 比率
 * @param decimals 小数位数
 */
export function formatPercentage(rate: number, decimals: number = 1): string {
    return `${rate.toFixed(decimals)}%`
}

/**
 * 计算同比增长率
 * @param current 当前值
 * @param previous 上期值
 */
export function calculateGrowthRate(current: number, previous: number): number {
    if (previous === 0) return current > 0 ? 100 : 0
    return ((current - previous) / previous) * 100
}