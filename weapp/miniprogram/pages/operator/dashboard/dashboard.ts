/**
 * 运营商工作台
 * 提供区域管理、商户管理、骑手管理、数据统计等功能入口
 */

import { getOperatorDashboard } from '@/api/operator-basic-management'
import { getMerchantManagementDashboard } from '@/api/operator-merchant-management'
import { getRiderManagementDashboard } from '@/api/operator-rider-management'
import { getOperatorAnalyticsDashboard } from '@/api/operator-analytics'
import type {
    OperatorResponse,
    OperatorFinanceOverviewResponse,
    RegionStatsResponse
} from '@/api/operator-basic-management'

interface QuickActionDataset {
    url?: string
}

Page({
    data: {
        loading: false,
        initialLoading: true,
        error: null as string | null,
        navBarHeight: 88,
        refreshing: false,

        // 运营商信息
        operatorInfo: null as OperatorResponse | null,

        // 财务概览
        financeOverview: null as OperatorFinanceOverviewResponse | null,

        // 区域统计
        regionStats: [] as RegionStatsResponse[],
        selectedRegionId: 0,

        // 商户摘要
        merchantSummary: {
            total: 0,
            active: 0,
            suspended: 0,
            pending: 0
        },

        // 骑手摘要
        riderSummary: {
            total: 0,
            active: 0,
            online: 0,
            suspended: 0,
            pending: 0
        },

        // 申诉摘要
        appealSummary: {
            totalAppeals: 0,
            pendingAppeals: 0,
            avgResolutionTime: 0,
            satisfactionRate: 0
        },

        // 快捷入口
        quickActions: [
            { id: 'merchants', icon: 'shop', label: '商户管理', url: '/pages/operator/merchants/index' },
            { id: 'riders', icon: 'user', label: '骑手管理', url: '/pages/operator/riders/index' },
            { id: 'analytics', icon: 'chart', label: '数据分析', url: '/pages/operator/analytics/index' },
            { id: 'appeals', icon: 'service', label: '客诉处理', url: '/pages/operator/appeal/list/index' }
        ]
    },

    onLoad() {
        this.loadDashboardData()
    },

    onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight: number }>) {
        this.setData({ navBarHeight: e.detail.navBarHeight })
    },

    onPullDownRefresh() {
        this.setData({ refreshing: true })
        this.loadDashboardData().finally(() => {
            this.setData({ refreshing: false })
            wx.stopPullDownRefresh()
        })
    },

    /**
     * 加载工作台数据
     */
    async loadDashboardData() {
        if (this.data.loading && !this.data.initialLoading) return
        this.setData({ loading: true, error: null })

        try {
            // 并行加载所有数据
            const [
                dashboardData,
                merchantData,
                riderData,
                analyticsData
            ] = await Promise.all([
                getOperatorDashboard(),
                getMerchantManagementDashboard(),
                getRiderManagementDashboard(),
                getOperatorAnalyticsDashboard()
            ])

            this.setData({
                operatorInfo: dashboardData.operatorInfo,
                financeOverview: dashboardData.financeOverview,
                regionStats: dashboardData.regionStats,
                selectedRegionId: dashboardData.regionStats[0]?.region_id || 0,
                merchantSummary: merchantData.merchantSummary,
                riderSummary: riderData.riderSummary,
                appealSummary: analyticsData.appealSummary,
                loading: false,
                initialLoading: false
            })
        } catch (error) {
            console.error('加载工作台数据失败:', error)
            this.setData({ 
                loading: false, 
                initialLoading: false,
                error: '加载工作台数据失败'
            })
        }
    },

    onRetry() {
        this.loadDashboardData()
    },

    /**
     * 切换区域
     */
    onRegionChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
        const regionId = parseInt(e.detail.value)
        this.setData({ selectedRegionId: regionId })
        this.loadRegionData(regionId)
    },

    /**
     * 加载指定区域的数据
     */
    async loadRegionData(regionId: number) {
        try {
            wx.showLoading({ title: '加载中...' })

            const [merchantData, riderData] = await Promise.all([
                getMerchantManagementDashboard(regionId),
                getRiderManagementDashboard(regionId)
            ])

            this.setData({
                merchantSummary: merchantData.merchantSummary,
                riderSummary: riderData.riderSummary
            })
        } catch (error) {
            console.error('加载区域数据失败:', error)
            wx.showToast({
                title: '加载失败',
                icon: 'none'
            })
        } finally {
            wx.hideLoading()
        }
    },

    /**
     * 快捷入口点击
     */
    onQuickActionTap(e: WechatMiniprogram.TouchEvent) {
        const { url } = e.currentTarget.dataset as QuickActionDataset
        if (!url) return
        wx.navigateTo({ url })
    },

    /**
     * 查看财务详情
     */
    onFinanceDetailTap() {
        wx.navigateTo({
            url: '/pages/operator/finance/withdraw/index'
        })
    },

    /**
     * 查看区域详情
     */
    onRegionDetailTap() {
        const { selectedRegionId } = this.data
        if (selectedRegionId) {
            wx.navigateTo({
                url: `/pages/operator/region/config?id=${selectedRegionId}`
            })
        }
    },

    /**
     * 格式化金额
     */
    formatAmount(amount: number): string {
        return `¥${(amount / 100).toFixed(2)}`
    },

    /**
     * 格式化百分比
     */
    formatPercentage(value: number): string {
        return `${value.toFixed(1)}%`
    }
})
