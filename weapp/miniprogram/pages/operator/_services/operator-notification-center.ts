import {
  operatorNotificationService,
  type OperatorNotification,
  type OperatorNotificationCategory
} from '../_api/operator-notification'

export interface OperatorNotificationSummaryCard {
  unreadCount: number
  latestTitle: string
  latestSummary: string
  latestCreatedAt: string
  empty: boolean
}

export interface OperatorNotificationView extends OperatorNotification {
  timeDisplay: string
  summaryText: string
  regionText: string
  waitText: string
}

export interface OperatorNotificationListPageData {
  notifications: OperatorNotificationView[]
  unreadCount: number
  total: number
  nextPage: number
  hasMore: boolean
}

export type OperatorNotificationFilterCategory = OperatorNotificationCategory | ''

export function formatOperatorNotificationTime(iso: string): string {
  const date = new Date(iso)
  const now = new Date()
  const diffMinutes = Math.floor((now.getTime() - date.getTime()) / 60000)

  if (diffMinutes < 1) {
    return '刚刚'
  }
  if (diffMinutes < 60) {
    return `${diffMinutes}分钟前`
  }

  const diffHours = Math.floor(diffMinutes / 60)
  if (diffHours < 24) {
    return `${diffHours}小时前`
  }

  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hour = String(date.getHours()).padStart(2, '0')
  const minute = String(date.getMinutes()).padStart(2, '0')
  return `${month}-${day} ${hour}:${minute}`
}

export function buildOperatorNotificationSummary(item?: Partial<OperatorNotification> | null): string {
  if (!item) {
    return '暂无待处理提醒'
  }

  const regionName = String(item.region_name || '')
  const waitMinutes = Number(item.wait_minutes || 0)
  if (regionName && waitMinutes > 0) {
    return `${regionName}有订单已等待 ${waitMinutes} 分钟，建议尽快联系骑手接单。`
  }

  if (item.summary) {
    return item.summary
  }

  return item.content || '有新的待接单提醒'
}

export function adaptOperatorNotificationView(item: OperatorNotification): OperatorNotificationView {
  return {
    ...item,
    timeDisplay: formatOperatorNotificationTime(item.created_at),
    summaryText: buildOperatorNotificationSummary(item),
    regionText: item.region_name ? `区域：${item.region_name}` : '区域待确认',
    waitText: item.wait_minutes ? `已等待 ${item.wait_minutes} 分钟` : ''
  }
}

export async function loadOperatorNotificationSummaryCard(): Promise<OperatorNotificationSummaryCard> {
  const summary = await operatorNotificationService.getSummary()
  const latest = summary.latest_notification

  return {
    unreadCount: Number(summary.unread_count || 0),
    latestTitle: latest?.title || '暂无待接单提醒',
    latestSummary: buildOperatorNotificationSummary(latest),
    latestCreatedAt: latest?.created_at ? formatOperatorNotificationTime(latest.created_at) : '',
    empty: !latest
  }
}

export async function loadOperatorNotificationListPageData(params: {
  pageId: number
  pageSize: number
  category?: OperatorNotificationCategory
  includeSummary?: boolean
  fallbackUnreadCount?: number
}): Promise<OperatorNotificationListPageData> {
  const [result, summary] = await Promise.all([
    operatorNotificationService.getNotifications({
      page_id: params.pageId,
      page_size: params.pageSize,
      category: params.category
    }),
    params.includeSummary
      ? operatorNotificationService.getSummary().catch(() => ({ unread_count: params.fallbackUnreadCount || 0 }))
      : Promise.resolve({ unread_count: params.fallbackUnreadCount || 0 })
  ])

  const notifications = (result.notifications || []).map(adaptOperatorNotificationView)
  const total = Number(result.total || notifications.length)

  return {
    notifications,
    unreadCount: Number(summary.unread_count || 0),
    total,
    nextPage: params.pageId + 1,
    hasMore: notifications.length < total
  }
}

export async function markOperatorNotificationAsRead(id: number): Promise<void> {
  await operatorNotificationService.markAsRead(id)
}

export async function markAllOperatorNotificationsAsRead(): Promise<void> {
  await operatorNotificationService.markAllAsRead()
}

export async function loadOperatorNotificationDetail(id: number): Promise<OperatorNotificationView> {
  const detail = await operatorNotificationService.getDetail(id)
  if (!detail.is_read) {
    operatorNotificationService.markAsRead(id).catch(() => null)
  }

  return adaptOperatorNotificationView({
    ...detail,
    is_read: true
  } as OperatorNotification)
}
