import type { PlatformOverviewResponse, RealtimeDashboardData } from '../api/platform-dashboard'

export type PlatformDashboardTheme = 'success' | 'warning' | 'danger' | 'primary' | 'default'

export interface PlatformDashboardMetricCard {
    key: string
    label: string
    value: string
    note: string
    theme: PlatformDashboardTheme
}

export interface PlatformDashboardRiskItem {
    key: string
    title: string
    desc: string
    value: string
    theme: PlatformDashboardTheme
    url?: string
}

export interface PlatformDashboardEntry {
    id: string
    title: string
    desc: string
    icon: string
    url: string
}

export interface PlatformDashboardEntryGroup {
    title: string
    items: PlatformDashboardEntry[]
}

export interface PlatformDashboardStatus {
    summary: string
    syncText: string
    healthText: string
    healthTheme: PlatformDashboardTheme
}

export interface PlatformDashboardView {
    todayCards: PlatformDashboardMetricCard[]
    riskItems: PlatformDashboardRiskItem[]
    realtimeRows: PlatformDashboardMetricCard[]
    cumulativeCards: PlatformDashboardMetricCard[]
    entryGroups: PlatformDashboardEntryGroup[]
    operationsStatus: PlatformDashboardStatus
}

export interface BuildPlatformDashboardViewInput {
    overviewData: PlatformOverviewResponse | null
    realtimeData: RealtimeDashboardData | null
    abnormalRefundCount: number
    alertFeedReady: boolean
}

const REVIEW_QUEUE_URL = '/pages/platform/riders/index'

function toNumber(value: unknown): number {
    return typeof value === 'number' && Number.isFinite(value) ? value : 0
}

function formatInteger(value: number): string {
    return Math.max(0, Math.round(value)).toLocaleString('zh-CN')
}

function formatAmount(amountInCents: number): string {
    return `¥${(Math.max(0, amountInCents) / 100).toLocaleString('zh-CN', {
        minimumFractionDigits: 2,
        maximumFractionDigits: 2
    })}`
}

function formatSyncText(timestamp?: number): string {
    if (!timestamp) {
        return '刚刚更新'
    }

    const date = new Date(timestamp)
    if (Number.isNaN(date.getTime())) {
        return '刚刚更新'
    }

    const hours = String(date.getHours()).padStart(2, '0')
    const minutes = String(date.getMinutes()).padStart(2, '0')
    return `${hours}:${minutes} 更新`
}

function getOverviewSummary(overviewData: PlatformOverviewResponse | null) {
    return overviewData?.summary || {
        total_orders: overviewData?.total_orders || 0,
        total_gmv: overviewData?.total_gmv || 0,
        completion_rate: 0,
        active_merchants: overviewData?.active_merchants || 0,
        active_riders: 0,
        avg_order_value: 0
    }
}

function getTodayStats(realtimeData: RealtimeDashboardData | null) {
    return realtimeData?.today_stats || {
        new_users: 0,
        new_merchants: 0,
        gmv: realtimeData?.gmv_24h || 0,
        order_count: realtimeData?.orders_24h || 0,
        total_orders: realtimeData?.orders_24h || 0,
        completed_orders: 0,
        cancelled_orders: 0,
        total_gmv: realtimeData?.gmv_24h || 0,
        avg_order_value: 0,
        completion_rate: 0,
        new_riders: 0
    }
}

function getRealtimeStats(realtimeData: RealtimeDashboardData | null) {
    return realtimeData?.realtime_stats || {
        online_riders: 0,
        online_merchants: 0,
        orders_per_minute: 0,
        online_users: 0,
        active_orders: 0,
        gmv_per_minute: 0
    }
}

function getActiveOrders(realtimeData: RealtimeDashboardData | null): number {
    const realtimeStats = getRealtimeStats(realtimeData)
    const activeOrders = toNumber(realtimeStats.active_orders)
    if (activeOrders > 0) {
        return activeOrders
    }

    return toNumber(realtimeData?.pending_orders)
        + toNumber(realtimeData?.preparing_orders)
        + toNumber(realtimeData?.ready_orders)
        + toNumber(realtimeData?.delivering_orders)
}

function buildTodayCards(realtimeData: RealtimeDashboardData | null): PlatformDashboardMetricCard[] {
    const todayStats = getTodayStats(realtimeData)
    const realtimeStats = getRealtimeStats(realtimeData)
    const orders24h = toNumber(realtimeData?.orders_24h) || toNumber(todayStats.order_count) || toNumber(todayStats.total_orders)
    const gmv24h = toNumber(realtimeData?.gmv_24h) || toNumber(todayStats.gmv) || toNumber(todayStats.total_gmv)
    const activeOrders = getActiveOrders(realtimeData)
    const onlineRiders = toNumber(realtimeStats.online_riders)

    return [
        {
            key: 'orders24h',
            label: '今日订单',
            value: formatInteger(orders24h),
            note: `${formatInteger(toNumber(todayStats.completed_orders))} 单已完成`,
            theme: 'primary'
        },
        {
            key: 'gmv24h',
            label: '今日GMV',
            value: formatAmount(gmv24h),
            note: `${formatInteger(toNumber(todayStats.new_users))} 位新用户`,
            theme: 'success'
        },
        {
            key: 'activeOrders',
            label: '履约中',
            value: formatInteger(activeOrders),
            note: `${formatInteger(toNumber(realtimeData?.delivering_orders))} 单配送中`,
            theme: activeOrders > 0 ? 'warning' : 'default'
        },
        {
            key: 'onlineRiders',
            label: '在线骑手',
            value: formatInteger(onlineRiders),
            note: `${formatInteger(toNumber(realtimeStats.online_merchants))} 家商户在线`,
            theme: onlineRiders > 0 ? 'success' : 'warning'
        }
    ]
}

function buildRiskItems(input: BuildPlatformDashboardViewInput): PlatformDashboardRiskItem[] {
    const realtimeData = input.realtimeData
    const pendingOrders = toNumber(realtimeData?.pending_orders)
    const abnormalRefundCount = Math.max(0, input.abnormalRefundCount)
    const risks: PlatformDashboardRiskItem[] = []

    if (abnormalRefundCount > 0 || input.alertFeedReady) {
        risks.push({
            key: 'abnormalRefunds',
            title: abnormalRefundCount > 0 ? '异常退款待处理' : '异常退款',
            desc: abnormalRefundCount > 0 ? '需平台管理员介入' : '当前没有待人工介入的异常退款',
            value: abnormalRefundCount > 0 ? formatInteger(abnormalRefundCount) : '0',
            theme: abnormalRefundCount > 0 ? 'danger' : 'success',
            url: '/pages/platform/finance/reconciliation/index'
        })
    }

    risks.push({
        key: 'pendingOrders',
        title: '待接单订单',
        desc: pendingOrders > 0 ? '关注履约压力与骑手供给' : '当前待接单压力稳定',
        value: formatInteger(pendingOrders),
        theme: pendingOrders > 0 ? 'warning' : 'success'
    })

    risks.push({
        key: 'reviewQueue',
        title: '经营实体待巡检',
        desc: '骑手、商户、运营商状态集中查看',
        value: '点入查看',
        theme: 'warning',
        url: REVIEW_QUEUE_URL
    })

    return risks
}

function buildRealtimeRows(realtimeData: RealtimeDashboardData | null): PlatformDashboardMetricCard[] {
    const realtimeStats = getRealtimeStats(realtimeData)
    return [
        {
            key: 'onlineMerchants',
            label: '在线商户',
            value: formatInteger(toNumber(realtimeStats.online_merchants)),
            note: '供给侧在线',
            theme: 'success'
        },
        {
            key: 'pendingOrders',
            label: '待接单',
            value: formatInteger(toNumber(realtimeData?.pending_orders)),
            note: '需关注接单效率',
            theme: toNumber(realtimeData?.pending_orders) > 0 ? 'warning' : 'success'
        },
        {
            key: 'preparingOrders',
            label: '制作中',
            value: formatInteger(toNumber(realtimeData?.preparing_orders)),
            note: '商户侧处理中',
            theme: 'primary'
        },
        {
            key: 'deliveringOrders',
            label: '配送中',
            value: formatInteger(toNumber(realtimeData?.delivering_orders)),
            note: '骑手侧履约',
            theme: 'primary'
        }
    ]
}

function buildCumulativeCards(overviewData: PlatformOverviewResponse | null): PlatformDashboardMetricCard[] {
    const summary = getOverviewSummary(overviewData)
    return [
        {
            key: 'totalOrders',
            label: '累计订单',
            value: formatInteger(toNumber(summary.total_orders)),
            note: `完成率 ${toNumber(summary.completion_rate).toFixed(1)}%`,
            theme: 'primary'
        },
        {
            key: 'totalGMV',
            label: '累计GMV',
            value: formatAmount(toNumber(summary.total_gmv)),
            note: `客单 ${formatAmount(toNumber(summary.avg_order_value))}`,
            theme: 'success'
        },
        {
            key: 'activeUsers',
            label: '活跃用户',
            value: formatInteger(toNumber(overviewData?.active_users)),
            note: '近30天',
            theme: 'default'
        },
        {
            key: 'activeMerchants',
            label: '活跃商户',
            value: formatInteger(toNumber(summary.active_merchants)),
            note: '近30天',
            theme: 'default'
        }
    ]
}

function buildEntryGroups(): PlatformDashboardEntryGroup[] {
    return [
        {
            title: '经营实体',
            items: [
                {
                    id: 'riders',
                    title: '骑手管理',
                    desc: '活跃与投诉',
                    icon: 'undertake-delivery',
                    url: '/pages/platform/riders/index'
                },
                {
                    id: 'merchants',
                    title: '商户管理',
                    desc: '经营与服务',
                    icon: 'shop',
                    url: '/pages/platform/merchants/index'
                },
                {
                    id: 'operators',
                    title: '运营商管理',
                    desc: '区域与商户',
                    icon: 'root-list',
                    url: '/pages/platform/operators/index'
                },
                {
                    id: 'groups',
                    title: '集团申请',
                    desc: '集团入驻审核',
                    icon: 'app',
                    url: '/pages/platform/groups/index'
                }
            ]
        },
        {
            title: '资金结算',
            items: [
                {
                    id: 'settlement-account',
                    title: '结算账户',
                    desc: '开户与状态同步',
                    icon: 'wallet',
                    url: '/pages/platform/finance/settlement-account/index'
                },
                {
                    id: 'withdrawals',
                    title: '提现',
                    desc: '余额与提现记录',
                    icon: 'money',
                    url: '/pages/platform/finance/withdrawals/index'
                },
                {
                    id: 'reconciliation',
                    title: '对账账单',
                    desc: '分账与异常计数',
                    icon: 'chart-bar',
                    url: '/pages/platform/finance/reconciliation/index'
                }
            ]
        },
        {
            title: '平台配置',
            items: [
                {
                    id: 'rules',
                    title: '运营配置',
                    desc: '佣金与骑手押金',
                    icon: 'setting',
                    url: '/pages/platform/operational-configs/index'
                }
            ]
        }
    ]
}

function buildOperationsStatus(realtimeData: RealtimeDashboardData | null, alertCount: number): PlatformDashboardStatus {
    const todayStats = getTodayStats(realtimeData)
    const orders24h = toNumber(realtimeData?.orders_24h) || toNumber(todayStats.order_count) || toNumber(todayStats.total_orders)
    const activeOrders = getActiveOrders(realtimeData)
    const pendingOrders = toNumber(realtimeData?.pending_orders)
    const hasRisk = alertCount > 0 || pendingOrders > 0

    return {
        summary: `今日 ${formatInteger(orders24h)} 单，履约中 ${formatInteger(activeOrders)} 单`,
        syncText: formatSyncText(realtimeData?.timestamp),
        healthText: hasRisk ? '有待处理事项' : '运行平稳',
        healthTheme: hasRisk ? 'warning' : 'success'
    }
}

export function buildPlatformDashboardView(input: BuildPlatformDashboardViewInput): PlatformDashboardView {
    return {
        todayCards: buildTodayCards(input.realtimeData),
        riskItems: buildRiskItems(input),
        realtimeRows: buildRealtimeRows(input.realtimeData),
        cumulativeCards: buildCumulativeCards(input.overviewData),
        entryGroups: buildEntryGroups(),
        operationsStatus: buildOperationsStatus(input.realtimeData, input.abnormalRefundCount)
    }
}
