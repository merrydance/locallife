/**
 * 平台管理中心
 * 提供基础统计与管理入口（不做大屏展示）
 */

import { platformDashboardService, type RealtimeDashboardData, type PlatformOverviewResponse } from '@/api/platform-dashboard'
import { responsiveBehavior } from '@/utils/responsive'
import { getConsoleDashboardErrorState } from '../../../utils/console-dashboard'

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

interface StateCardItem {
    label: string
    value: string
    theme?: 'success' | 'warning' | 'danger'
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
        error: null as string | null,
        errorCanRetry: true,
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
                title: '运营配置',
                desc: '平台真实运营参数配置',
                icon: 'setting',
                url: '/pages/platform/operational-configs/index'
            },
            {
                id: 'groups',
                title: '集团申请审核',
                desc: '审核集团入驻申请',
                icon: 'app',
                url: '/pages/platform/groups/index'
            },
            {
                id: 'riders',
                title: '骑手管理',
                desc: '骑手审核与状态管理',
                icon: 'undertake-delivery',
                url: '/pages/platform/riders/index'
            }
        ] as ManagementAction[],
        realtimeCards: [
            { label: '在线骑手', value: '0', theme: 'success' },
            { label: '待处理订单', value: '0', theme: 'warning' },
            { label: '待审骑手', value: '0', theme: 'danger' }
        ] as StateCardItem[],
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
            this.setData({ requesting: true, error: null })
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

            const overviewSummary = overviewData.summary || {
                total_orders: overviewData.total_orders || 0,
                total_gmv: overviewData.total_gmv || 0,
                active_merchants: overviewData.active_merchants || 0
            }

            const realtimeStats = realtimeData.realtime_stats || {
                online_riders: 0
            }

            const pendingOrders = realtimeData.pending_orders || 0
            const onlineRiders = realtimeStats.online_riders || 0

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
                    totalOrders: String(overviewSummary.total_orders || 0),
                    totalGMV: this.formatAmount(overviewSummary.total_gmv || 0),
                    activeUsers: String(overviewData.active_users || 0),
                    activeMerchants: String(overviewSummary.active_merchants || 0),
                    onlineRiders: String(onlineRiders),
                    pendingOrders: String(pendingOrders),
                    pendingRiders: '0'
                },
                realtimeCards: [
                    { label: '在线骑手', value: String(onlineRiders), theme: 'success' },
                    { label: '待处理订单', value: String(pendingOrders), theme: 'warning' },
                    { label: '待审骑手', value: '0', theme: 'danger' }
                ],
                actions
            })
        } catch (error) {
            console.error('加载平台管理中心数据失败:', error)
            const errorState = getConsoleDashboardErrorState('platform', error, '平台管理中心暂时无法加载，请稍后重试。')
            const hasLoadedDashboard = !!this.data.overviewData || !!this.data.realtimeData
            const shouldSurfaceError = !silent || !hasLoadedDashboard || !errorState.canRetry

            if (shouldSurfaceError) {
                this.setData({ error: errorState.message, errorCanRetry: errorState.canRetry })
            }

            if (!silent && errorState.canRetry) {
                wx.showToast({
                    title: '加载失败，请重试',
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
            await this.loadDashboardData(true)
        } finally {
            this.setData({ refreshing: false })
        }
    },

    onRetry() {
        this.loadDashboardData()
    },

    /**
     * 快捷操作点击
     */
    onActionTap(e: ActionTapEvent) {
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

