/**
 * 财务管理页面
 * 集成全部 6 个后端财务 API，提供全面的财务分析
 */

import { request } from '@/utils/request'

// 类型定义
interface FinanceOverviewResponse {
    completed_orders: number
    pending_orders: number
    total_gmv: number
    total_income: number
    total_platform_fee: number
    total_operator_fee: number
    total_service_fee: number
    pending_income: number
    promotion_orders: number
    total_promotion_exp: number
    net_income: number
}

interface FinanceOrderItem {
    id: number
    payment_order_id: number
    order_id: number
    order_source: string
    total_amount: number
    platform_fee: number
    operator_fee: number
    merchant_amount: number
    status: string
    created_at: string
    finished_at?: string
}

interface ServiceFeeItem {
    date: string
    order_source: string
    order_count: number
    total_amount: number
    platform_fee: number
    operator_fee: number
    total_fee: number
}

interface PromotionExpenseItem {
    id: number
    order_no: string
    order_type: string
    subtotal: number
    delivery_fee: number
    delivery_fee_discount: number
    total_amount: number
    created_at: string
    completed_at?: string
}

interface DailyFinanceItem {
    date: string
    order_count: number
    total_gmv: number
    merchant_income: number
    total_fee: number
}

interface SettlementItem {
    id: number
    payment_order_id: number
    order_source: string
    total_amount: number
    platform_fee: number
    operator_fee: number
    merchant_amount: number
    sharing_order_id?: string
    status: string
    created_at: string
    finished_at?: string
}

// 财务服务
const FinanceService = {
    async getOverview(startDate: string, endDate: string): Promise<FinanceOverviewResponse> {
        return request({ url: '/v1/merchant/finance/overview', method: 'GET', data: { start_date: startDate, end_date: endDate } })
    },
    async getOrders(startDate: string, endDate: string, page = 1, limit = 20): Promise<FinanceOrderItem[]> {
        return request({ url: '/v1/merchant/finance/orders', method: 'GET', data: { start_date: startDate, end_date: endDate, page, limit } })
    },
    async getServiceFees(startDate: string, endDate: string): Promise<ServiceFeeItem[]> {
        return request({ url: '/v1/merchant/finance/service-fees', method: 'GET', data: { start_date: startDate, end_date: endDate } })
    },
    async getPromotions(startDate: string, endDate: string, page = 1, limit = 20): Promise<PromotionExpenseItem[]> {
        return request({ url: '/v1/merchant/finance/promotions', method: 'GET', data: { start_date: startDate, end_date: endDate, page, limit } })
    },
    async getDailyFinance(startDate: string, endDate: string): Promise<DailyFinanceItem[]> {
        return request({ url: '/v1/merchant/finance/daily', method: 'GET', data: { start_date: startDate, end_date: endDate } })
    },
    async getSettlements(startDate: string, endDate: string, status?: string, page = 1, limit = 20): Promise<SettlementItem[]> {
        const data: any = { start_date: startDate, end_date: endDate, page, limit }
        if (status) data.status = status
        return request({ url: '/v1/merchant/finance/settlements', method: 'GET', data })
    }
}

Page({
    data: {
        sidebarCollapsed: false,
        loading: true,

        // 日期范围
        dateRange: 'month' as 'week' | 'month',
        startDate: '',
        endDate: '',

        // Tab
        activeTab: 'overview' as 'overview' | 'daily' | 'orders' | 'fees' | 'promotions' | 'settlements',

        // 数据
        overview: null as FinanceOverviewResponse | null,
        dailyFinance: [] as DailyFinanceItem[],
        orders: [] as FinanceOrderItem[],
        serviceFees: [] as ServiceFeeItem[],
        promotions: [] as PromotionExpenseItem[],
        settlements: [] as SettlementItem[]
    },

    onLoad() {
        this.setDateRange('month')
    },

    onSidebarCollapse(e: any) {
        this.setData({ sidebarCollapsed: e.detail.collapsed })
    },

    // 设置日期范围
    setDateRange(range: 'week' | 'month') {
        const today = new Date()
        const endDate = this.formatDate(today)
        let startDate: string

        if (range === 'week') {
            const weekAgo = new Date(today)
            weekAgo.setDate(weekAgo.getDate() - 6)
            startDate = this.formatDate(weekAgo)
        } else {
            const monthAgo = new Date(today)
            monthAgo.setDate(monthAgo.getDate() - 29)
            startDate = this.formatDate(monthAgo)
        }

        this.setData({ dateRange: range, startDate, endDate })
        this.loadData()
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

    // 切换 Tab
    onTabChange(e: any) {
        const tab = e.currentTarget.dataset.tab
        this.setData({ activeTab: tab })
        this.loadTabData(tab)
    },

    // 加载当前 Tab 数据
    async loadTabData(tab: string) {
        const { startDate, endDate } = this.data
        this.setData({ loading: true })

        try {
            switch (tab) {
                case 'overview':
                    const overview = await FinanceService.getOverview(startDate, endDate)
                    this.setData({ overview, loading: false })
                    break
                case 'daily':
                    const dailyFinance = await FinanceService.getDailyFinance(startDate, endDate)
                    this.setData({ dailyFinance: dailyFinance || [], loading: false })
                    break
                case 'orders':
                    const orders = await FinanceService.getOrders(startDate, endDate)
                    this.setData({ orders: orders || [], loading: false })
                    break
                case 'fees':
                    const serviceFees = await FinanceService.getServiceFees(startDate, endDate)
                    this.setData({ serviceFees: serviceFees || [], loading: false })
                    break
                case 'promotions':
                    const promotions = await FinanceService.getPromotions(startDate, endDate)
                    this.setData({ promotions: promotions || [], loading: false })
                    break
                case 'settlements':
                    const settlements = await FinanceService.getSettlements(startDate, endDate)
                    this.setData({ settlements: settlements || [], loading: false })
                    break
            }
        } catch (error: any) {
            console.error('加载财务数据失败:', error)
            wx.showToast({ title: '加载失败', icon: 'none' })
            this.setData({ loading: false })
        }
    },

    // 加载初始数据
    async loadData() {
        await this.loadTabData(this.data.activeTab)
    },

    // 格式化状态
    formatStatus(status: string): string {
        const map: Record<string, string> = {
            'pending': '待处理',
            'processing': '处理中',
            'finished': '已完成',
            'failed': '失败'
        }
        return map[status] || status
    }
})
