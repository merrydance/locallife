/**
 * 平台管理中心
 * 提供基础统计与管理入口（不做大屏展示）
 */

import { platformDashboardService, type RealtimeDashboardData, type PlatformOverviewResponse } from '@/api/platform-dashboard'
import { responsiveBehavior } from '@/utils/responsive'

type ActionTapEvent = WechatMiniprogram.CustomEvent & {
    currentTarget: {
        dataset: {
            id?: string
            url?: string
        }
    }
}
type NavHeightEvent = WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>

interface PlatformSummaryCards {
    totalOrders: string
    totalGMV: string
    activeUsers: string
    activeMerchants: string
    onlineRiders: string
    pendingOrders: string
    pendingRiders: string
}

interface ManagementAction {
    id: string
    title: string
    desc: string
    icon: string
    url?: string
    badge?: string
}

Page({
    behaviors: [responsiveBehavior],
    data: {
        loading: true,
        refreshing: false,
        requesting: false,
        overviewData: null as PlatformOverviewResponse | null,
        realtimeData: null as RealtimeDashboardData | null,
        summaryCards: {
            totalOrders: '0',
            totalGMV: '¥0.00',
            activeUsers: '0',
            activeMerchants: '0',
            onlineRiders: '0',
            pendingOrders: '0',
            pendingRiders: '0'
        } as PlatformSummaryCards,
        actions: [
            {
                id: 'operators',
                title: '运营商管理',
                desc: '运营商入驻与区域管理',
                icon: 'root-list',
                url: '/pages/platform/operators/index'
            },
            {
                id: 'rules',
                title: '规则中心',
                desc: '平台规则与命中审计',
                icon: 'setting'
            },
            {
                id: 'groups',
                title: '集团申请审核',
                desc: '审核集团入驻申请',
                icon: 'app'
            }
        ] as ManagementAction[],
        refreshTimer: null as number | null,
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
     * 加载管理中心数据
     */
    async loadDashboardData(silent = false) {
        if (this.data.requesting) {
            return
        }

        try {
            this.setData({ requesting: true })
            if (!silent) {
                this.setData({ loading: true })
            }

            const endDate = new Date().toISOString().split('T')[0]
            const startDate = new Date(Date.now() - 30 * 24 * 60 * 60 * 1000).toISOString().split('T')[0]

            // 仅加载管理中心基础数据，避免调用不存在的接口
            const [overviewData, realtimeData] = await Promise.all([
                platformDashboardService.getPlatformOverview({ start_date: startDate, end_date: endDate }),
                platformDashboardService.getRealtimeDashboard()
            ])

            const actions = this.data.actions.map((item) => {
                return {
                    ...item,
                    badge: ''
                }
            })

            this.setData({
                overviewData,
                realtimeData,
                summaryCards: {
                    totalOrders: String(overviewData.summary.total_orders || 0),
                    totalGMV: this.formatAmount(overviewData.summary.total_gmv || 0),
                    activeUsers: String(overviewData.active_users || 0),
                    activeMerchants: String(overviewData.summary.active_merchants || 0),
                    onlineRiders: String(realtimeData.realtime_stats.online_riders || 0),
                    pendingOrders: String(realtimeData.pending_orders || 0),
                    pendingRiders: '0'
                },
                actions
            })
        } catch (error) {
            console.error('加载平台管理中心数据失败:', error)
            if (!silent) {
                wx.showToast({
                    title: '加载失败',
                    icon: 'none'
                })
            }
        } finally {
            this.setData({ requesting: false })
            if (!silent) {
                this.setData({ loading: false })
            }
        }
    },

    /**
     * 下拉刷新
     */
    async onRefresh() {
        this.setData({ refreshing: true })
        try {
            await this.loadDashboardData(true) // Silent load to avoid skeleton flash, rely on refresher spinner
        } finally {
            this.setData({ refreshing: false })
        }
    },

    /**
     * 快捷操作点击
     */
    onActionTap(e: ActionTapEvent) {
        const { id, url } = e.currentTarget.dataset

        if (id === 'rules' || id === 'groups') {
            wx.showToast({
                title: '管理功能建设中',
                icon: 'none'
            })
            return
        }

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
            this.loadDashboardData(true) // Silent refresh
        }, 30000)

        this.setData({ refreshTimer: timer as number })
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
        return `¥${(amount / 100).toFixed(2)}`
    },

    /**
     * 导航栏高度变化
     */
    onNavHeight(e: NavHeightEvent) {
        this.setData({ navBarHeight: e.detail.navBarHeight })
    }
})

