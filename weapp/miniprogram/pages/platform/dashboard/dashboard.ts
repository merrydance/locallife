/**
 * 平台管理中心
 * 提供基础统计与管理入口（不做大屏展示）
 */

import { platformDashboardService, type RealtimeDashboardData, type PlatformOverviewResponse } from '@/api/platform-dashboard'
import { platformAlertsService } from '@/api/platform-alerts'
import { responsiveBehavior } from '@/utils/responsive'
import { getConsoleDashboardErrorState } from '../../../utils/console-dashboard'
import { wsManager, WSMessageType } from '../../../utils/websocket'
import {
    buildAbnormalRefundClipboardText,
    formatPlatformAlertTime,
    toActionableAbnormalRefundAlert,
    type ActionableAbnormalRefundAlert
} from '../../../utils/platform-alerts'

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

interface AlertFeedItem extends ActionableAbnormalRefundAlert {
    timeDisplay: string
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
        abnormalRefundAlerts: [] as AlertFeedItem[],
        alertFeedReady: false,
        _alertListeners: [] as Array<() => void>,
        refreshTimer: null as number | null,
        navBarHeight: 0
    },

    onLoad() {
        this.loadDashboardData()
        this.startAutoRefresh()
        this.initAlertFeed()
    },

    onUnload() {
        this.stopAutoRefresh()
        this.stopAlertFeed()
    },

    onShow() {
        // 页面显示时恢复自动刷新
        if (!this.data.refreshTimer) {
            this.startAutoRefresh()
        }
        if (this.data._alertListeners.length === 0) {
            this.startAlertFeed()
        }
    },

    onHide() {
        // 页面隐藏时停止自动刷新
        this.stopAutoRefresh()
        this.stopAlertFeed()
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

    startAlertFeed() {
        this.stopAlertFeed()
        wsManager.connect('/v1/platform/ws')

        const alertSub = wsManager.on(WSMessageType.ALERT, (data) => {
            const nextAlert = toActionableAbnormalRefundAlert(data)
            if (!nextAlert) {
                return
            }

            const nextItems = [
                {
                    ...nextAlert,
                    timeDisplay: formatPlatformAlertTime(nextAlert.timestamp)
                },
                ...this.data.abnormalRefundAlerts.filter((item) => item.refundOrderId !== nextAlert.refundOrderId)
            ].slice(0, 5)

            this.setData({
                abnormalRefundAlerts: nextItems,
                alertFeedReady: true
            })
        })

        this.data._alertListeners = [alertSub]
    },

    async initAlertFeed() {
        try {
            await this.loadRecentPlatformAlerts()
        } finally {
            this.startAlertFeed()
        }
    },

    async loadRecentPlatformAlerts() {
        try {
            const response = await platformAlertsService.listPlatformAlerts({ page_id: 1, page_size: 10 })
            const alerts = (response.alerts || [])
                .map((item) => toActionableAbnormalRefundAlert(item))
                .filter((item): item is ActionableAbnormalRefundAlert => !!item)
                .map((item) => ({
                    ...item,
                    timeDisplay: formatPlatformAlertTime(item.timestamp)
                }))
                .slice(0, 5)

            this.setData({
                abnormalRefundAlerts: alerts,
                alertFeedReady: true
            })
        } catch (_error) {
            this.setData({ alertFeedReady: true })
        }
    },

    stopAlertFeed() {
        if (this.data._alertListeners.length > 0) {
            this.data._alertListeners.forEach((unsubscribe) => unsubscribe())
            this.data._alertListeners = []
        }
        wsManager.disconnect()
    },

    onAbnormalRefundAlertTap(e: WechatMiniprogram.TouchEvent) {
        const { index } = e.currentTarget.dataset as { index?: number }
        const alert = typeof index === 'number' ? this.data.abnormalRefundAlerts[index] : undefined
        if (!alert) {
            return
        }

        wx.showActionSheet({
            itemList: ['查看处理说明', '复制处理参数'],
            success: ({ tapIndex }) => {
                if (tapIndex === 0) {
                    this.showAbnormalRefundGuide(alert)
                    return
                }
                this.copyAbnormalRefundAlert(alert)
            }
        })
    },

    showAbnormalRefundGuide(alert: AlertFeedItem) {
        const extraFields = alert.userBankCardRequiredFields.length > 0
            ? `\n收款到用户银行卡需补充：${alert.userBankCardRequiredFields.join('、')}`
            : ''

        wx.showModal({
            title: '异常退款处理说明',
            content:
                `该退款已进入微信异常退款人工处理流程。\n\n` +
                `接口：${alert.method} ${alert.path}\n` +
                `权限：平台管理员\n` +
                `退款单ID：${alert.refundOrderId}\n` +
                `支付单ID：${alert.paymentOrderId || '-'}\n` +
                `微信退款ID：${alert.refundId}\n` +
                `默认退款去向：${alert.defaultType}\n` +
                `支持退款去向：${alert.supportedTypes.join(' / ') || alert.defaultType}` +
                extraFields +
                `\n\n${alert.message}`,
            cancelText: '关闭',
            confirmText: '复制参数',
            success: (result) => {
                if (result.confirm) {
                    this.copyAbnormalRefundAlert(alert)
                }
            }
        })
    },

    copyAbnormalRefundAlert(alert: AlertFeedItem) {
        wx.setClipboardData({
            data: buildAbnormalRefundClipboardText(alert),
            success: () => {
                wx.showToast({ title: '处理参数已复制', icon: 'success' })
            }
        })
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

