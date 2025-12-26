/**
 * 超管平台管理大屏
 * 提供实时数据监控、平台概览、排行榜、快捷管理入口
 */

import { getPlatformDashboardData } from '@/api/platform-dashboard'
import { getPlatformManagementDashboard } from '@/api/platform-management'
import type {
    RealtimeDashboardData,
    PlatformOverviewResponse,
    MerchantRankingRow,
    RiderRankingRow
} from '@/api/platform-dashboard'
import { responsiveBehavior } from '@/utils/responsive'

Page({
    behaviors: [responsiveBehavior],
    data: {
        loading: true,
        refreshing: false,

        // 实时数据
        realtimeData: null as RealtimeDashboardData | null,

        // 平台概览
        overviewData: null as PlatformOverviewResponse | null,

        // 排行榜
        merchantRanking: [] as MerchantRankingRow[],
        riderRanking: [] as RiderRankingRow[],

        // 待审核数量
        pendingMerchants: 0,
        pendingRiders: 0,

        // 日期范围选择
        dateRangeOptions: [
            { label: '今日', value: 'today', days: 1 },
            { label: '近7天', value: 'week', days: 7 },
            { label: '近30天', value: 'month', days: 30 },
            { label: '近90天', value: 'quarter', days: 90 }
        ],
        selectedDateRange: 2, // 默认近30天

        // 自动刷新定时器
        refreshTimer: null as number | null,

        // 导航栏高度
        navBarHeight: 0
    },

    onLoad() {
        this.loadDashboardData()
        this.startAutoRefresh()
    },

    onUnload() {
        this.stopAutoRefresh()
    },

    onShow() {
        // 页面显示时恢复自动刷新
        if (!this.data.refreshTimer) {
            this.startAutoRefresh()
        }
    },

    onHide() {
        // 页面隐藏时停止自动刷新
        this.stopAutoRefresh()
    },

    /**
     * 加载大屏数据
     */
    async loadDashboardData() {
        try {
            this.setData({ loading: true })

            // 并行加载所有数据
            const [dashboardData, managementData] = await Promise.all([
                getPlatformDashboardData(),
                getPlatformManagementDashboard()
            ])

            this.setData({
                realtimeData: dashboardData.realtime,
                overviewData: dashboardData.overview,
                merchantRanking: dashboardData.merchantRanking.slice(0, 10),
                riderRanking: dashboardData.riderRanking.slice(0, 10),
                pendingMerchants: managementData.merchantApplications.pending.length,
                pendingRiders: managementData.riderApplications.pending.length
            })
        } catch (error) {
            console.error('加载大屏数据失败:', error)
            wx.showToast({
                title: '加载失败',
                icon: 'none'
            })
        } finally {
            this.setData({ loading: false })
        }
    },

    /**
     * 下拉刷新
     */
    async onRefresh() {
        this.setData({ refreshing: true })
        try {
            await this.loadDashboardData()
        } finally {
            this.setData({ refreshing: false })
        }
    },

    /**
     * 切换日期范围
     */
    onDateRangeChange(e: any) {
        const index = parseInt(e.detail.value)
        this.setData({ selectedDateRange: index })
        this.loadDashboardData()
    },

    /**
     * 查看更多商户
     */
    onViewMoreMerchants() {
        wx.navigateTo({
            url: '/pages/platform/merchants/ranking'
        })
    },

    /**
     * 查看更多骑手
     */
    onViewMoreRiders() {
        wx.navigateTo({
            url: '/pages/platform/riders/ranking'
        })
    },

    /**
     * 快捷操作点击
     */
    onActionTap(e: any) {
        const { url } = e.currentTarget.dataset
        if (url) {
            wx.navigateTo({ url })
        } else {
            wx.showToast({
                title: '功能开发中',
                icon: 'none'
            })
        }
    },

    /**
     * 启动自动刷新
     */
    startAutoRefresh() {
        // 每30秒自动刷新实时数据
        const timer = setInterval(() => {
            this.loadDashboardData()
        }, 30000)

        this.setData({ refreshTimer: timer as any })
    },

    /**
     * 停止自动刷新
     */
    stopAutoRefresh() {
        if (this.data.refreshTimer) {
            clearInterval(this.data.refreshTimer)
            this.setData({ refreshTimer: null })
        }
    },

    /**
     * 格式化金额
     */
    formatAmount(amount: number): string {
        if (amount >= 100000000) { // 1亿以上
            return `¥${(amount / 100000000).toFixed(2)}亿`
        } else if (amount >= 10000000) { // 1000万以上
            return `¥${(amount / 10000000).toFixed(2)}千万`
        } else if (amount >= 1000000) { // 100万以上
            return `¥${(amount / 1000000).toFixed(2)}百万`
        } else if (amount >= 10000) { // 1万以上
            return `¥${(amount / 10000).toFixed(2)}万`
        } else {
            return `¥${(amount / 100).toFixed(2)}`
        }
    },

    formatPercentage(value: number): string {
        const sign = value >= 0 ? '+' : ''
        return `${sign}${value.toFixed(1)}%`
    },

    /**
     * 导航栏高度变化
     */
    onNavHeight(e: any) {
        this.setData({ navBarHeight: e.detail.navBarHeight })
    }
})
