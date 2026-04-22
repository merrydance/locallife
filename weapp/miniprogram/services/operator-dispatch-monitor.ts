import {
  operatorDispatchMonitorService,
  type OperatorPendingDispatchItem,
  type OperatorPendingDispatchSummary
} from '../api/operator-dispatch-monitor'
import { formatPrice } from '../utils/util'

export interface OperatorDispatchMonitorSummaryView {
  regionId: number
  regionName: string
  pendingTotal: number
  timeoutOver3mTotal: number
  oldestWaitSeconds: number
  oldestWaitText: string
  latestRefreshText: string
}

export interface OperatorPendingDispatchView extends OperatorPendingDispatchItem {
  waitText: string
  deliveryFeeText: string
  expectedPickupText: string
}

export interface OperatorDispatchMonitorPageData {
  summary: OperatorDispatchMonitorSummaryView
  items: OperatorPendingDispatchView[]
  total: number
  page: number
  pageSize: number
  hasMore: boolean
}

function pad(value: number): string {
  return String(value).padStart(2, '0')
}

function formatTime(iso?: string): string {
  if (!iso) {
    return '刚刚'
  }

  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) {
    return '刚刚'
  }

  const now = new Date()
  const diffMinutes = Math.max(0, Math.floor((now.getTime() - date.getTime()) / 60000))
  if (diffMinutes < 1) {
    return '刚刚'
  }
  if (diffMinutes < 60) {
    return `${diffMinutes} 分钟前`
  }

  const sameDay = now.getFullYear() === date.getFullYear()
    && now.getMonth() === date.getMonth()
    && now.getDate() === date.getDate()

  if (sameDay) {
    return `${pad(date.getHours())}:${pad(date.getMinutes())}`
  }

  return `${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function formatWaitText(waitSeconds: number): string {
  const seconds = Math.max(0, Number(waitSeconds || 0))
  if (seconds < 60) {
    return `${seconds} 秒`
  }

  const minutes = Math.floor(seconds / 60)
  const remainingSeconds = seconds % 60
  if (!remainingSeconds) {
    return `${minutes} 分钟`
  }

  return `${minutes} 分 ${remainingSeconds} 秒`
}

function formatExpectedPickup(iso?: string): string {
  if (!iso) {
    return '未提供'
  }

  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) {
    return '未提供'
  }

  return `${pad(date.getHours())}:${pad(date.getMinutes())}`
}

export function adaptOperatorPendingDispatchSummary(summary: OperatorPendingDispatchSummary): OperatorDispatchMonitorSummaryView {
  return {
    regionId: Number(summary.region_id || 0),
    regionName: summary.region_name || '当前区域',
    pendingTotal: Number(summary.pending_total || 0),
    timeoutOver3mTotal: Number(summary.timeout_over_3m_total || 0),
    oldestWaitSeconds: Number(summary.oldest_wait_seconds || 0),
    oldestWaitText: formatWaitText(Number(summary.oldest_wait_seconds || 0)),
    latestRefreshText: formatTime(summary.latest_refresh_at)
  }
}

export function adaptOperatorPendingDispatchView(item: OperatorPendingDispatchItem): OperatorPendingDispatchView {
  return {
    ...item,
    waitText: formatWaitText(Number(item.wait_seconds || 0)),
    deliveryFeeText: formatPrice(Number(item.delivery_fee || 0)),
    expectedPickupText: formatExpectedPickup(item.expected_pickup_at)
  }
}

export async function loadOperatorDispatchMonitorPageData(params: {
  regionId: number
  pageId: number
  pageSize: number
}): Promise<OperatorDispatchMonitorPageData> {
  const [summary, list] = await Promise.all([
    operatorDispatchMonitorService.getSummary(params.regionId),
    operatorDispatchMonitorService.getPendingDispatches(params.regionId, {
      page_id: params.pageId,
      page_size: params.pageSize
    })
  ])

  return {
    summary: adaptOperatorPendingDispatchSummary(summary),
    items: (list.items || []).map(adaptOperatorPendingDispatchView),
    total: Number(list.total || 0),
    page: Number(list.page || params.pageId),
    pageSize: Number(list.pageSize || params.pageSize),
    hasMore: Boolean(list.hasMore)
  }
}