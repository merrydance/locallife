import type { RiderStatus } from '../api/rider'
import type {
  RiderWorkbenchDeliveryItem,
  RiderWorkbenchSectionStatus,
  RiderWorkbenchSummaryResponse
} from '../api/rider-workbench'
import type { DashboardDeliveryView, TagTheme } from '../utils/rider-dashboard-runtime'
import { getDeliveryStatusDisplay } from '../api/delivery'
import { formatPriceNoSymbol } from '../utils/util'
import { resolveStatusTagTheme } from '../utils/status-tag'
import { buildRiderDepositWorkbenchSummaryView } from './rider-deposit-finance'

export interface RiderWorkbenchMetricView {
  key: string
  label: string
  value: string
  note: string
}


export interface RiderWorkbenchRiskView {
  key: string
  label: string
  value: string
  note: string
  highlight: boolean
  highlightClass: string
  actionText: string
}

export interface RiderWorkbenchDashboardView {
  riderStatus: RiderStatus
  metrics: RiderWorkbenchMetricView[]
  risks: RiderWorkbenchRiskView[]
  currentDelivery: DashboardDeliveryView | null
  availableOrderCount: number
  activeDeliveryCount: number
  todayCompletedDeliveries: number
  todayIncomeDisplay: string
  unavailableText: string
}

const sectionLabelMap: Record<string, string> = {
  rider_status: '骑手状态',
  current_deliveries: '当前任务',
  order_pool: '抢单大厅',
  today: '今日经营',
  income: '配送费结算',
  deposit: '押金摘要',
  claims: '追偿待处理',
  notifications: '未读通知'
}

function buildUnavailableText(sections: RiderWorkbenchSectionStatus[]): string {
  const unavailable = (sections || []).filter((section) => !section.available)
  if (!unavailable.length) {
    return ''
  }

  return unavailable
    .map((section) => section.message || `${sectionLabelMap[section.section] || section.section}暂不可用`)
    .join('；')
}

function toRiderStatus(summary: RiderWorkbenchSummaryResponse): RiderStatus {
  const status = summary.rider_status
  return {
    status: status.status,
    is_online: status.is_online,
    online_status: status.online_status,
    active_deliveries: status.active_deliveries,
    current_longitude: status.current_longitude,
    current_latitude: status.current_latitude,
    location_updated_at: status.location_updated_at,
    can_go_online: status.can_go_online,
    can_go_offline: status.can_go_offline,
    online_block_reason: status.online_block_reason
  }
}

function buildDashboardDelivery(item: RiderWorkbenchDeliveryItem): DashboardDeliveryView {
  const statusDisplay = getDeliveryStatusDisplay(item.status)
  const isAssignedStage = item.status === 'assigned' || item.status === 'picking'

  return {
    id: item.id,
    order_id: item.order_id,
    status: item.status,
    delivery_fee: item.delivery_fee,
    rider_earnings: item.rider_earnings,
    pickup_address: item.pickup_address,
    pickup_longitude: 0,
    pickup_latitude: 0,
    delivery_address: item.delivery_address,
    delivery_longitude: 0,
    delivery_latitude: 0,
    estimated_pickup_at: item.estimated_pickup_at,
    estimated_delivery_at: item.estimated_delivery_at,
    picked_at: item.picked_at,
    delivered_at: item.delivered_at,
    created_at: item.created_at,
    status_desc: statusDisplay.text || item.status,
    status_tag_theme: (isAssignedStage ? resolveStatusTagTheme('warning') : resolveStatusTagTheme('success')) as TagTheme,
    deadline_desc: '',
    is_overdue: false,
    is_very_urgent: false,
    is_pickup_finished: item.status === 'picked' || item.status === 'delivering',
    can_start_pickup: item.status === 'assigned',
    can_confirm_pickup: item.status === 'picking',
    can_start_delivery: item.status === 'picked',
    can_confirm_delivery: item.status === 'delivering',
    is_action_loading: false
  }
}

export function buildRiderWorkbenchDashboardView(summary: RiderWorkbenchSummaryResponse): RiderWorkbenchDashboardView {
  const availableOrderCount = Number(summary.order_pool?.available_count || 0)
  const activeDeliveryCount = Number(summary.current_deliveries?.active_count || 0)
  const todayCompletedDeliveries = Number(summary.today?.completed_deliveries || 0)
  const todayIncome = Number(summary.income?.total_rider_income || 0)
  const pendingIncome = Number(summary.income?.pending_rider_amount || 0) + Number(summary.income?.processing_rider_amount || 0)
  const failedIncomeCount = Number(summary.income?.failed_count || 0)
  const pendingClaims = Number(summary.claims?.pending_action_count || 0)
  const unreadNotifications = Number(summary.notifications?.unread_count || 0)
  const deposit = summary.deposit || {
    total_deposit: 0,
    frozen_deposit: 0,
    delivery_frozen_deposit: 0,
    deposit_refund_processing_amount: 0,
    available_deposit: 0,
    threshold_amount: 0
  }
  const depositSummary = buildRiderDepositWorkbenchSummaryView({
    availableDeposit: deposit.available_deposit || 0,
    deliveryFrozenDeposit: deposit.delivery_frozen_deposit || 0,
    withdrawalProcessingAmount: deposit.deposit_refund_processing_amount || 0,
    thresholdAmount: deposit.threshold_amount || 0,
    activeDeliveries: activeDeliveryCount,
    canGoOnline: !!summary.rider_status?.can_go_online,
    onlineBlockReason: summary.rider_status?.online_block_reason || ''
  })

  return {
    riderStatus: toRiderStatus(summary),
    metrics: [
      {
        key: 'today_completed',
        label: '今日完成',
        value: String(todayCompletedDeliveries),
        note: summary.today?.date || '今日'
      },
      {
        key: 'today_income',
        label: '到账金额',
        value: `¥${formatPriceNoSymbol(todayIncome)}`,
        note: `${summary.income?.total_deliveries || 0} 单已到账`
      },
      {
        key: 'order_pool',
        label: '可抢订单',
        value: String(availableOrderCount),
        note: activeDeliveryCount > 0 ? `当前 ${activeDeliveryCount} 个任务` : '空闲可接单'
      }
    ],
    risks: [
      {
        key: 'deposit',
        label: '可用押金',
        value: depositSummary.value,
        note: depositSummary.note,
        highlight: depositSummary.highlight,
        highlightClass: depositSummary.highlightClass,
        actionText: depositSummary.actionText
      },
      {
        key: 'claims',
        label: '待处理追偿',
        value: String(pendingClaims),
        note: pendingClaims > 0 ? '需及时处理' : '暂无待处理',
        highlight: pendingClaims > 0,
        highlightClass: pendingClaims > 0 ? 'is-highlight' : '',
        actionText: ''
      },
      {
        key: 'income_pending',
        label: '待结算/分账中',
        value: `¥${formatPriceNoSymbol(pendingIncome)}`,
        note: failedIncomeCount > 0 ? `${failedIncomeCount} 单待平台处理` : '分账完成后入账',
        highlight: failedIncomeCount > 0,
        highlightClass: failedIncomeCount > 0 ? 'is-highlight' : '',
        actionText: ''
      },
      {
        key: 'notifications',
        label: '未读通知',
        value: String(unreadNotifications),
        note: unreadNotifications > 0 ? '有新提醒' : '暂无新提醒',
        highlight: unreadNotifications > 0,
        highlightClass: unreadNotifications > 0 ? 'is-highlight' : '',
        actionText: ''
      }
    ],
    currentDelivery: (summary.current_deliveries?.items || [])[0]
      ? buildDashboardDelivery(summary.current_deliveries.items[0])
      : null,
    availableOrderCount,
    activeDeliveryCount,
    todayCompletedDeliveries,
    todayIncomeDisplay: formatPriceNoSymbol(todayIncome),
    unavailableText: buildUnavailableText(summary.sections || [])
  }
}