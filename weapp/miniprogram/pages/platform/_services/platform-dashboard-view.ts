import type { PlatformOverviewResponse, RealtimeDashboardData } from '../_api/platform-dashboard'

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

function getActiveOrders(realtimeData: RealtimeDashboardData | null): number {
    return toNumber(realtimeData?.pending_orders)
        + toNumber(realtimeData?.preparing_orders)
        + toNumber(realtimeData?.ready_orders)
        + toNumber(realtimeData?.delivering_orders)
}

function buildTodayCards(realtimeData: RealtimeDashboardData | null): PlatformDashboardMetricCard[] {
    const orders24h = toNumber(realtimeData?.orders_24h)
    const gmv24h = toNumber(realtimeData?.gmv_24h)
    const activeOrders = getActiveOrders(realtimeData)
    const activeUsers24h = toNumber(realtimeData?.active_users_24h)
    const activeMerchants24h = toNumber(realtimeData?.active_merchants_24h)

    return [
        {
            key: 'orders24h',
            label: '近24h订单',
            value: formatInteger(orders24h),
            note: `${formatInteger(activeOrders)} 单履约中`,
            theme: 'primary'
        },
        {
            key: 'gmv24h',
            label: '近24hGMV',
            value: formatAmount(gmv24h),
            note: `${formatInteger(activeMerchants24h)} 家商户活跃`,
            theme: 'success'
        },
        {
            key: 'activeOrders',
            label: '履约中',
            value: formatInteger(activeOrders),
            note: `${formatInteger(toNumber(realtimeData?.delivering_orders))} 单代取中`,
            theme: activeOrders > 0 ? 'warning' : 'default'
        },
        {
            key: 'activeUsers24h',
            label: '24h活跃用户',
            value: formatInteger(activeUsers24h),
            note: `${formatInteger(activeMerchants24h)} 家商户活跃`,
            theme: activeUsers24h > 0 ? 'success' : 'default'
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
    return [
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
            key: 'readyOrders',
            label: '待取餐',
            value: formatInteger(toNumber(realtimeData?.ready_orders)),
            note: '等待骑手取餐',
            theme: toNumber(realtimeData?.ready_orders) > 0 ? 'warning' : 'success'
        },
        {
            key: 'deliveringOrders',
            label: '代取中',
            value: formatInteger(toNumber(realtimeData?.delivering_orders)),
            note: '骑手侧履约',
            theme: 'primary'
        }
    ]
}

function buildCumulativeCards(overviewData: PlatformOverviewResponse | null): PlatformDashboardMetricCard[] {
    return [
        {
            key: 'totalOrders',
            label: '累计订单',
            value: formatInteger(toNumber(overviewData?.total_orders)),
            note: '近30天订单',
            theme: 'primary'
        },
        {
            key: 'totalGMV',
            label: '累计GMV',
            value: formatAmount(toNumber(overviewData?.total_gmv)),
            note: '近30天GMV',
            theme: 'success'
        },
        {
            key: 'totalCommission',
            label: '平台佣金',
            value: formatAmount(toNumber(overviewData?.total_commission)),
            note: '近30天佣金',
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
            value: formatInteger(toNumber(overviewData?.active_merchants)),
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
    const orders24h = toNumber(realtimeData?.orders_24h)
    const activeOrders = getActiveOrders(realtimeData)
    const pendingOrders = toNumber(realtimeData?.pending_orders)
    const hasRisk = alertCount > 0 || pendingOrders > 0

    return {
        summary: `近24h ${formatInteger(orders24h)} 单，履约中 ${formatInteger(activeOrders)} 单`,
        syncText: '自动刷新',
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
