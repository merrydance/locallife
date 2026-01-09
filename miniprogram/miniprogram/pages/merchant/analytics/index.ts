/**
 * 经营统计页面
 * 集成全部 9 个后端统计 API，提供全面的数据分析
 */

import { request } from '@/utils/request'

// 类型定义
interface DailyStatRow {
    date: string
    order_count: number
    total_sales: number
    commission: number
    takeout_orders: number
    dine_in_orders: number
}

interface OverviewResponse {
    total_days: number
    total_orders: number
    total_sales: number
    total_commission: number
    avg_daily_sales: number
}

interface TopDishRow {
    dish_id: number
    dish_name: string
    dish_price: number
    total_sold: number
    total_revenue: number
}

interface HourlyStatsRow {
    hour: number
    order_count: number
    avg_order_amount: number
}

interface OrderSourceStatsRow {
    order_type: string
    order_count: number
    total_sales: number
}

interface RepurchaseRateResponse {
    total_users: number
    repeat_users: number
    repurchase_rate: number
    avg_orders_per_user: number
}

interface CategoryStatsRow {
    category_name: string
    order_count: number
    total_sales: number
    total_quantity: number
}

interface CustomerStatRow {
    user_id: number
    full_name: string
    phone: string
    avatar_url: string
    total_orders: number
    total_amount: number
    avg_order_amount: number
    first_order_at: string
    last_order_at: string
}

// 统计服务
const StatsService = {
    async getOverview(startDate: string, endDate: string): Promise<OverviewResponse> {
        return request({ url: '/v1/merchant/stats/overview', method: 'GET', data: { start_date: startDate, end_date: endDate } })
    },
    async getDailyStats(startDate: string, endDate: string): Promise<DailyStatRow[]> {
        return request({ url: '/v1/merchant/stats/daily', method: 'GET', data: { start_date: startDate, end_date: endDate } })
    },
    async getTopDishes(startDate: string, endDate: string, limit = 10): Promise<TopDishRow[]> {
        return request({ url: '/v1/merchant/stats/dishes/top', method: 'GET', data: { start_date: startDate, end_date: endDate, limit } })
    },
    async getHourlyStats(startDate: string, endDate: string): Promise<HourlyStatsRow[]> {
        return request({ url: '/v1/merchant/stats/hourly', method: 'GET', data: { start_date: startDate, end_date: endDate } })
    },
    async getOrderSourceStats(startDate: string, endDate: string): Promise<OrderSourceStatsRow[]> {
        return request({ url: '/v1/merchant/stats/sources', method: 'GET', data: { start_date: startDate, end_date: endDate } })
    },
    async getRepurchaseRate(startDate: string, endDate: string): Promise<RepurchaseRateResponse> {
        return request({ url: '/v1/merchant/stats/repurchase', method: 'GET', data: { start_date: startDate, end_date: endDate } })
    },
    async getCategoryStats(startDate: string, endDate: string): Promise<CategoryStatsRow[]> {
        return request({ url: '/v1/merchant/stats/categories', method: 'GET', data: { start_date: startDate, end_date: endDate } })
    },
    async getCustomers(orderBy = 'total_amount', page = 1, limit = 10): Promise<CustomerStatRow[]> {
        return request({ url: '/v1/merchant/stats/customers', method: 'GET', data: { order_by: orderBy, page, limit } })
    }
}

Page({
    data: {
        sidebarCollapsed: false,
        loading: true,

        // 日期范围
        dateRange: 'week' as 'today' | 'week' | 'month',
        startDate: '',
        endDate: '',

        // 概览数据
        overview: null as OverviewResponse | null,

        // 日报数据
        dailyStats: [] as DailyStatRow[],

        // 热门菜品
        topDishes: [] as TopDishRow[],

        // 时段分析
        hourlyStats: [] as HourlyStatsRow[],

        // 订单来源
        sourceStats: [] as OrderSourceStatsRow[],

        // 复购率
        repurchaseRate: null as RepurchaseRateResponse | null,

        // 分类销售
        categoryStats: [] as CategoryStatsRow[],

        // 顾客排行
        topCustomers: [] as CustomerStatRow[]
    },

    onLoad() {
        this.setDateRange('week')
    },

    onSidebarCollapse(e: any) {
        this.setData({ sidebarCollapsed: e.detail.collapsed })
    },

    // 设置日期范围
    setDateRange(range: 'today' | 'week' | 'month') {
        const today = new Date()
        let startDate = this.formatDate(today)
        const endDate = this.formatDate(today)

        if (range === 'week') {
            const weekAgo = new Date(today)
            weekAgo.setDate(weekAgo.getDate() - 6)
            startDate = this.formatDate(weekAgo)
        } else if (range === 'month') {
            const monthAgo = new Date(today)
            monthAgo.setDate(monthAgo.getDate() - 29)
            startDate = this.formatDate(monthAgo)
        }

        this.setData({ dateRange: range, startDate, endDate })
        this.loadAllData()
    },

    onDateRangeChange(e: any) {
        const range = e.currentTarget.dataset.range
        this.setDateRange(range)
    },

    formatDate(date: Date): string {
        const year = date.getFullYear()
        const month = ('0' + (date.getMonth() + 1)).slice(-2)
        const day = ('0' + date.getDate()).slice(-2)
        return `${year}-${month}-${day}`
    },

    // 加载所有数据
    async loadAllData() {
        const { startDate, endDate } = this.data
        this.setData({ loading: true })

        try {
            const [overview, dailyStats, topDishes, hourlyStats, sourceStats, repurchaseRate, categoryStats, topCustomers] = await Promise.all([
                StatsService.getOverview(startDate, endDate),
                StatsService.getDailyStats(startDate, endDate),
                StatsService.getTopDishes(startDate, endDate, 10),
                StatsService.getHourlyStats(startDate, endDate),
                StatsService.getOrderSourceStats(startDate, endDate),
                StatsService.getRepurchaseRate(startDate, endDate),
                StatsService.getCategoryStats(startDate, endDate),
                StatsService.getCustomers('total_amount', 1, 10)
            ])

            this.setData({
                overview,
                dailyStats: dailyStats || [],
                topDishes: topDishes || [],
                hourlyStats: hourlyStats || [],
                sourceStats: sourceStats || [],
                repurchaseRate,
                categoryStats: categoryStats || [],
                topCustomers: topCustomers || [],
                loading: false
            })
        } catch (error: any) {
            console.error('加载统计数据失败:', error)
            wx.showToast({ title: '加载失败', icon: 'none' })
            this.setData({ loading: false })
        }
    },

    // 格式化金额（分转元）
    formatAmount(fen: number): string {
        return (fen / 100).toFixed(2)
    },

    // 订单类型中文
    formatOrderType(type: string): string {
        const map: Record<string, string> = {
            'takeout': '外卖',
            'dine_in': '堂食',
            'pickup': '自取'
        }
        return map[type] || type
    }
})
