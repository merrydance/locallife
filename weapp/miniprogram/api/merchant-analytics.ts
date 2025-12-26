/**
 * 商户BI分析接口
 * 基于swagger.json完全重构，包含销售统计、财务分析、客户分析等
 */

import { request } from '../utils/request'

// ==================== 商户小时统计数据类型定义 ====================

/**
 * 商户小时统计行 - 对齐 api.merchantHourlyStatsRow
 */
export interface MerchantHourlyStatsRow {
    avg_order_amount?: number                    // 平均订单金额（分）
    hour?: number                                // 小时（0-23）
    order_count?: number                         // 订单数
}

/**
 * 商户订单来源统计行 - 对齐 api.merchantOrderSourceStatsRow
 */
export interface MerchantOrderSourceStatsRow {
    order_count?: number                         // 订单数
    order_type?: string                          // 订单类型
    total_sales?: number                         // 总销售额（分）
}

// ==================== 统计概览数据类型定义 ====================

/**
 * 统计概览响应
 */
export interface StatsOverviewResponse {
    total_orders: number                         // 总订单数
    total_revenue: number                        // 总营收（分）
    total_customers: number                      // 总客户数
    avg_order_value: number                      // 平均订单价值（分）
    completion_rate: number                      // 完成率
    growth_rate: number                          // 增长率
}

/**
 * 每日统计响应
 */
export interface DailyStatsResponse {
    date: string                                 // 日期
    orders: number                               // 订单数
    revenue: number                              // 营收（分）
    customers: number                            // 客户数
    avg_order_value: number                      // 平均订单价值（分）
}

/**
 * 小时统计响应
 */
export interface HourlyStatsResponse {
    hour: number                                 // 小时（0-23）
    orders: number                               // 订单数
    revenue: number                              // 营收（分）
}

// ==================== 菜品分析数据类型定义 ====================

/**
 * 热门菜品响应
 */
export interface TopDishResponse {
    dish_id: number                              // 菜品ID
    dish_name: string                            // 菜品名称
    sales_count: number                          // 销量
    revenue: number                              // 营收（分）
    rank: number                                 // 排名
}

/**
 * 分类统计响应
 */
export interface CategoryStatsResponse {
    category_id: number                          // 分类ID
    category_name: string                        // 分类名称
    sales_count: number                          // 销量
    revenue: number                              // 营收（分）
    percentage: number                           // 占比
}

// ==================== 客户分析数据类型定义 ====================

/**
 * 客户统计响应
 */
export interface CustomerStatsResponse {
    user_id: number                              // 用户ID
    username: string                             // 用户名
    total_orders: number                         // 总订单数
    total_spent: number                          // 总消费（分）
    avg_order_value: number                      // 平均订单价值（分）
    last_order_date: string                      // 最后下单日期
}

/**
 * 复购率统计响应
 */
export interface RepurchaseStatsResponse {
    total_customers: number                      // 总客户数
    repurchase_customers: number                 // 复购客户数
    repurchase_rate: number                      // 复购率
    avg_repurchase_interval: number              // 平均复购间隔（天）
}

/**
 * 订单来源统计响应
 */
export interface OrderSourceStatsResponse {
    source: string                               // 来源
    orders: number                               // 订单数
    revenue: number                              // 营收（分）
    percentage: number                           // 占比
}

// ==================== 财务分析数据类型定义 ====================

/**
 * 财务概览响应 - 对齐 api.financeOverviewResponse
 */
export interface FinanceOverviewResponse {
    completed_orders?: number                    // 订单统计
    net_income?: number                          // 汇总
    pending_income?: number                      // 待结算收入
    pending_orders?: number                      // 待处理订单数
    promotion_orders?: number                    // 满返支出统计
    total_gmv?: number                           // 金额统计（分）
    total_income?: number                        // 商户净收入
    total_operator_fee?: number                  // 运营商服务费
    total_platform_fee?: number                  // 平台服务费
    total_promotion_exp?: number                 // 满返支出总额
    total_service_fee?: number                   // 总服务费（平台+运营商）
}

/**
 * 每日财务响应
 */
export interface DailyFinanceResponse {
    date: string                                 // 日期
    revenue: number                              // 营收（分）
    refunds: number                              // 退款（分）
    service_fees: number                         // 服务费（分）
    net_revenue: number                          // 净营收（分）
}

/**
 * 订单财务明细响应
 */
export interface OrderFinanceResponse {
    order_id: number                             // 订单ID
    order_no: string                             // 订单编号
    order_date: string                           // 订单日期
    total_amount: number                         // 订单金额（分）
    service_fee: number                          // 服务费（分）
    net_amount: number                           // 净收入（分）
    status: string                               // 订单状态
}

/**
 * 结算记录响应
 */
export interface SettlementResponse {
    id: number                                   // 结算ID
    settlement_date: string                      // 结算日期
    amount: number                               // 结算金额（分）
    status: string                               // 结算状态
    orders_count: number                         // 订单数量
}

// ==================== 统计分析服务 ====================

/**
 * 统计分析服务
 */
export class MerchantStatsService {

    /**
     * 获取统计概览
     * GET /v1/merchant/stats/overview
     */
    static async getStatsOverview(params: {
        start_date: string                         // 开始日期（YYYY-MM-DD）
        end_date: string                           // 结束日期（YYYY-MM-DD）
    }): Promise<StatsOverviewResponse> {
        return await request({
            url: '/v1/merchant/stats/overview',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取每日统计
     * GET /v1/merchant/stats/daily
     */
    static async getDailyStats(params: {
        start_date: string                         // 开始日期（YYYY-MM-DD）
        end_date: string                           // 结束日期（YYYY-MM-DD）
    }): Promise<DailyStatsResponse[]> {
        return await request({
            url: '/v1/merchant/stats/daily',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取小时统计
     * GET /v1/merchant/stats/hourly
     */
    static async getHourlyStats(params: {
        date: string                               // 日期（YYYY-MM-DD）
    }): Promise<HourlyStatsResponse[]> {
        return await request({
            url: '/v1/merchant/stats/hourly',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取热门菜品
     * GET /v1/merchant/stats/dishes/top
     */
    static async getTopDishes(params: {
        start_date: string                         // 开始日期（YYYY-MM-DD）
        end_date: string                           // 结束日期（YYYY-MM-DD）
        limit?: number                             // 返回数量（默认10）
    }): Promise<TopDishResponse[]> {
        return await request({
            url: '/v1/merchant/stats/dishes/top',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取分类统计
     * GET /v1/merchant/stats/categories
     */
    static async getCategoryStats(params: {
        start_date: string                         // 开始日期（YYYY-MM-DD）
        end_date: string                           // 结束日期（YYYY-MM-DD）
    }): Promise<CategoryStatsResponse[]> {
        return await request({
            url: '/v1/merchant/stats/categories',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取客户统计
     * GET /v1/merchant/stats/customers
     */
    static async getCustomerStats(params: {
        start_date: string                         // 开始日期（YYYY-MM-DD）
        end_date: string                           // 结束日期（YYYY-MM-DD）
        page_id: number                            // 页码
        page_size: number                          // 每页数量
    }): Promise<CustomerStatsResponse[]> {
        return await request({
            url: '/v1/merchant/stats/customers',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取复购率统计
     * GET /v1/merchant/stats/repurchase
     */
    static async getRepurchaseStats(params: {
        start_date: string                         // 开始日期（YYYY-MM-DD）
        end_date: string                           // 结束日期（YYYY-MM-DD）
    }): Promise<RepurchaseStatsResponse> {
        return await request({
            url: '/v1/merchant/stats/repurchase',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取订单来源统计
     * GET /v1/merchant/stats/sources
     */
    static async getOrderSourceStats(params: {
        start_date: string                         // 开始日期（YYYY-MM-DD）
        end_date: string                           // 结束日期（YYYY-MM-DD）
    }): Promise<OrderSourceStatsResponse[]> {
        return await request({
            url: '/v1/merchant/stats/sources',
            method: 'GET',
            data: params
        })
    }
}

// ==================== 财务分析服务 ====================

/**
 * 财务分析服务
 */
export class MerchantFinanceService {

    /**
     * 获取财务概览
     * GET /v1/merchant/finance/overview
     */
    static async getFinanceOverview(params: {
        start_date: string                         // 开始日期（YYYY-MM-DD）
        end_date: string                           // 结束日期（YYYY-MM-DD）
    }): Promise<FinanceOverviewResponse> {
        return await request({
            url: '/v1/merchant/finance/overview',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取每日财务
     * GET /v1/merchant/finance/daily
     */
    static async getDailyFinance(params: {
        start_date: string                         // 开始日期（YYYY-MM-DD）
        end_date: string                           // 结束日期（YYYY-MM-DD）
    }): Promise<DailyFinanceResponse[]> {
        return await request({
            url: '/v1/merchant/finance/daily',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取订单财务明细
     * GET /v1/merchant/finance/orders
     */
    static async getOrderFinance(params: {
        start_date: string                         // 开始日期（YYYY-MM-DD）
        end_date: string                           // 结束日期（YYYY-MM-DD）
        page_id: number                            // 页码
        page_size: number                          // 每页数量
    }): Promise<OrderFinanceResponse[]> {
        return await request({
            url: '/v1/merchant/finance/orders',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取结算记录
     * GET /v1/merchant/finance/settlements
     */
    static async getSettlements(params: {
        start_date: string                         // 开始日期（YYYY-MM-DD）
        end_date: string                           // 结束日期（YYYY-MM-DD）
        page_id: number                            // 页码
        page_size: number                          // 每页数量
    }): Promise<SettlementResponse[]> {
        return await request({
            url: '/v1/merchant/finance/settlements',
            method: 'GET',
            data: params
        })
    }
}

// ==================== 分析数据适配器 ====================

/**
 * 分析数据适配器
 */
export class AnalyticsAdapter {

    /**
     * 格式化金额显示（分转元）
     */
    static formatAmount(amountInCents: number): string {
        return (amountInCents / 100).toFixed(2)
    }

    /**
     * 格式化百分比
     */
    static formatPercentage(value: number): string {
        return `${(value * 100).toFixed(1)}%`
    }

    /**
     * 格式化增长率
     */
    static formatGrowthRate(rate: number): string {
        const sign = rate >= 0 ? '+' : ''
        return `${sign}${(rate * 100).toFixed(1)}%`
    }

    /**
     * 计算环比增长
     */
    static calculateGrowth(current: number, previous: number): number {
        if (previous === 0) return current > 0 ? 1 : 0
        return (current - previous) / previous
    }

    /**
     * 格式化日期范围
     */
    static formatDateRange(startDate: string, endDate: string): string {
        return `${startDate} 至 ${endDate}`
    }

    /**
     * 获取增长率颜色
     */
    static getGrowthColor(rate: number): string {
        if (rate > 0) return '#52c41a'  // 绿色
        if (rate < 0) return '#ff4d4f'  // 红色
        return '#999'                    // 灰色
    }

    /**
     * 转换为图表数据格式
     */
    static toChartData(data: DailyStatsResponse[]): {
        labels: string[]
        datasets: number[][]
    } {
        return {
            labels: data.map(d => d.date),
            datasets: [
                data.map(d => d.orders),
                data.map(d => d.revenue / 100)
            ]
        }
    }

    /**
     * 转换为饼图数据格式
     */
    static toPieChartData(data: CategoryStatsResponse[]): {
        labels: string[]
        values: number[]
    } {
        return {
            labels: data.map(d => d.category_name),
            values: data.map(d => d.percentage)
        }
    }
}

// ==================== 导出默认服务 ====================

export default {
    MerchantStatsService,
    MerchantFinanceService,
    AnalyticsAdapter
}
