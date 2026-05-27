import { Delivery } from '../api/delivery'

export type RiderDeliveryActionKey = 'start_pickup' | 'confirm_pickup' | 'start_delivery' | 'confirm_delivery' | ''
export type RiderDeliveryActionFeedbackMode = 'toast' | 'modal'

interface RiderDeliveryActionState {
  canUpdate: boolean
  label: string
  actionKey: RiderDeliveryActionKey
  expectedStatus: Delivery['status'] | null
  locationSource: string
}

export interface RiderDeliveryActionFeedback {
  mode: RiderDeliveryActionFeedbackMode
  title: string
  content?: string
  confirmText?: string
}

interface DeliveryActionErrorLike {
  userMessage?: unknown
  message?: unknown
  detailMessage?: unknown
  errMsg?: unknown
  data?: {
    message?: unknown
  }
  body?: {
    message?: unknown
  }
  originalError?: {
    message?: unknown
  }
}

interface RiderNavigationStopView {
  title: string
  address: string
  latitude: number
  longitude: number
}

export interface RiderDeliveryDeadlineView {
  text: string
  deadline?: string
  isOverdue: boolean
  isVeryUrgent: boolean
}

const PICKUP_STAGE_STATUSES = new Set<Delivery['status']>(['assigned', 'picking'])
const TRACKED_STATUSES = new Set<Delivery['status']>(['assigned', 'picking', 'picked', 'delivering'])
const DELIVERED_STATUSES = new Set<Delivery['status']>(['delivered', 'completed'])

function getTimestamp(timeStr?: string): number | null {
  if (!timeStr) return null
  const timestamp = new Date(timeStr).getTime()
  return Number.isFinite(timestamp) ? timestamp : null
}

function formatClock(timestamp: number): string {
  const date = new Date(timestamp)
  const hours = date.getHours().toString().padStart(2, '0')
  const minutes = date.getMinutes().toString().padStart(2, '0')
  return `${hours}:${minutes}`
}

const DELIVERY_ACTION_STATE_MAP: Partial<Record<Delivery['status'], Omit<RiderDeliveryActionState, 'canUpdate'>>> = {
  assigned: {
    label: '我已到达商家',
    actionKey: 'start_pickup',
    expectedStatus: 'picking',
    locationSource: 'rider_task_detail_start_pickup'
  },
  picking: {
    label: '确认取餐',
    actionKey: 'confirm_pickup',
    expectedStatus: 'picked',
    locationSource: 'rider_task_detail_confirm_pickup'
  },
  picked: {
    label: '开始代取',
    actionKey: 'start_delivery',
    expectedStatus: 'delivering',
    locationSource: 'rider_task_detail_start_delivery'
  },
  delivering: {
    label: '确认已送达',
    actionKey: 'confirm_delivery',
    expectedStatus: 'delivered',
    locationSource: 'rider_task_detail_confirm_delivery'
  },
  pending: {
    label: '',
    actionKey: '',
    expectedStatus: null,
    locationSource: 'rider_task_detail_pending'
  },
  delivered: {
    label: '',
    actionKey: '',
    expectedStatus: null,
    locationSource: 'rider_task_detail_delivered'
  },
  completed: {
    label: '',
    actionKey: '',
    expectedStatus: null,
    locationSource: 'rider_task_detail_completed'
  },
  cancelled: {
    label: '',
    actionKey: '',
    expectedStatus: null,
    locationSource: 'rider_task_detail_cancelled'
  },
  exception: {
    label: '',
    actionKey: '',
    expectedStatus: null,
    locationSource: 'rider_task_detail_exception'
  }
}

const DELIVERY_STEP_MAP: Partial<Record<Delivery['status'], number>> = {
  assigned: 0,
  picking: 1,
  picked: 2,
  delivering: 2,
  delivered: 3,
  completed: 3
}

function normalizeActionKey(actionKey?: string): string {
  return String(actionKey || '')
    .replace(/([a-z])([A-Z])/g, '$1_$2')
    .trim()
    .toLowerCase()
}

function isConfirmDeliveryAction(actionKey?: string): boolean {
  return normalizeActionKey(actionKey) === 'confirm_delivery'
}

function asNonEmptyString(value: unknown): string {
  return typeof value === 'string' ? value.replace(/\s+/g, ' ').trim() : ''
}

function getDeliveryActionErrorMessage(error: unknown, fallback: string): string {
  if (typeof error === 'string') {
    return asNonEmptyString(error) || fallback
  }

  if (!error || typeof error !== 'object') {
    return fallback
  }

  const knownError = error as DeliveryActionErrorLike
  const candidates = [
    knownError.userMessage,
    knownError.message,
    knownError.detailMessage,
    knownError.data?.message,
    knownError.body?.message,
    knownError.originalError?.message,
    knownError.errMsg
  ]

  for (const candidate of candidates) {
    const message = asNonEmptyString(candidate)
    if (message) {
      return message
    }
  }

  return fallback
}

function normalizeDeliveryDistanceMessage(message: string): string {
  return message
    .replace(/代取地址/g, '用户位置点')
    .replace(/收货地址/g, '用户位置点')
    .replace(/送达地址/g, '用户位置点')
}

function isDeliveryDistanceBlocked(message: string): boolean {
  const normalized = message.toLowerCase()
  return (
    (
      message.includes('距离代取地址') ||
      message.includes('距离收货地址') ||
      message.includes('距离送达地址') ||
      message.includes('距离用户位置点')
    ) &&
    (message.includes('确认送达') || message.includes('需在') || normalized.includes('distance'))
  )
}

function isDeliveryLocationBlocked(message: string): boolean {
  return (
    message.includes('骑手定位缺失') ||
    message.includes('骑手定位已过期') ||
    message.includes('定位获取失败') ||
    message.includes('开启定位权限') ||
    message.includes('刷新定位')
  )
}

function isDropoffLocationMissing(message: string): boolean {
  return message.includes('收货位置缺失') || message.includes('送达位置缺失')
}

export function isRiderDeliveryTrackedStatus(status?: Delivery['status']): boolean {
  return !!status && TRACKED_STATUSES.has(status)
}

export function isExpectedDeliveryStatusReached(status: Delivery['status'], expectedStatus: Delivery['status']): boolean {
  if (expectedStatus === 'delivered') {
    return DELIVERED_STATUSES.has(status)
  }

  return status === expectedStatus
}

export function getRiderDeliveryActionState(status: Delivery['status']): RiderDeliveryActionState {
  const config = DELIVERY_ACTION_STATE_MAP[status]

  if (!config || !config.actionKey || !config.expectedStatus) {
    return {
      canUpdate: false,
      label: '',
      actionKey: '',
      expectedStatus: null,
      locationSource: DELIVERY_ACTION_STATE_MAP[status]?.locationSource || 'rider_task_detail_action'
    }
  }

  return {
    canUpdate: true,
    ...config
  }
}

export function buildRiderDeliveryActionConfirmFeedback(
  actionKey: string,
  actionLabel: string
): RiderDeliveryActionFeedback {
  if (isConfirmDeliveryAction(actionKey)) {
    return {
      mode: 'modal',
      title: '确认送达',
      content: '请确认已到达用户位置点并完成交付；未到达时无法送达。',
      confirmText: '确认送达'
    }
  }

  return {
    mode: 'modal',
    title: '状态更新',
    content: `确定已完成 ${actionLabel.replace('我已', '')} 吗？`,
    confirmText: '确定'
  }
}

export function buildRiderDeliveryActionFailureFeedback(
  error: unknown,
  actionKey: string,
  fallback: string
): RiderDeliveryActionFeedback {
  const message = getDeliveryActionErrorMessage(error, fallback)

  if (!isConfirmDeliveryAction(actionKey)) {
    return {
      mode: 'toast',
      title: message || fallback
    }
  }

  if (isDeliveryDistanceBlocked(message)) {
    return {
      mode: 'modal',
      title: '暂未到达送达点',
      content: normalizeDeliveryDistanceMessage(message),
      confirmText: '知道了'
    }
  }

  if (isDeliveryLocationBlocked(message)) {
    return {
      mode: 'modal',
      title: '定位未同步',
      content: message,
      confirmText: '知道了'
    }
  }

  if (isDropoffLocationMissing(message)) {
    return {
      mode: 'modal',
      title: '送达位置缺失',
      content: message,
      confirmText: '知道了'
    }
  }

  return {
    mode: 'toast',
    title: message || fallback
  }
}

export function getRiderDeliveryStep(status?: string): number {
  const normalizedStatus = String(status || '').trim().toLowerCase() as Delivery['status'] | ''
  if (!normalizedStatus) {
    return 0
  }

  return DELIVERY_STEP_MAP[normalizedStatus] ?? 0
}

export function getRiderDeliveryDeadline(delivery: Pick<Delivery, 'status' | 'estimated_pickup_at' | 'estimated_delivery_at'>): string | undefined {
  return PICKUP_STAGE_STATUSES.has(delivery.status) ? delivery.estimated_pickup_at : delivery.estimated_delivery_at
}

export function buildRiderDeliveryDeadlineView(
  delivery: Pick<Delivery, 'status' | 'estimated_pickup_at' | 'estimated_delivery_at' | 'delivered_at' | 'completed_at'>,
  now: number = Date.now()
): RiderDeliveryDeadlineView {
  const deadline = getRiderDeliveryDeadline(delivery)
  const deadlineTimestamp = getTimestamp(deadline)

  if (DELIVERED_STATUSES.has(delivery.status)) {
    const deliveredTimestamp = getTimestamp(delivery.delivered_at || delivery.completed_at)
    if (!deliveredTimestamp) {
      return { text: '已送达', deadline, isOverdue: false, isVeryUrgent: false }
    }

    const isLate = deadlineTimestamp !== null && deliveredTimestamp > deadlineTimestamp
    return {
      text: isLate ? '超时送达' : `${formatClock(deliveredTimestamp)} 送达`,
      deadline,
      isOverdue: isLate,
      isVeryUrgent: false
    }
  }

  if (deadlineTimestamp === null) {
    return { text: '尽快送达', deadline, isOverdue: false, isVeryUrgent: false }
  }

  const diff = deadlineTimestamp - now
  if (diff < 0) {
    return { text: '已超时', deadline, isOverdue: true, isVeryUrgent: false }
  }

  const clock = formatClock(deadlineTimestamp)
  return {
    text: diff < 60 * 60 * 1000 ? `剩 ${Math.max(1, Math.floor(diff / 60000))} 分钟 (${clock})` : `${clock} 前`,
    deadline,
    isOverdue: false,
    isVeryUrgent: diff < 15 * 60 * 1000
  }
}

export function isRiderDeliveryOverdue(delivery: Pick<Delivery, 'status' | 'estimated_pickup_at' | 'estimated_delivery_at' | 'delivered_at' | 'completed_at'>): boolean {
  return buildRiderDeliveryDeadlineView(delivery).isOverdue
}

export function getRiderNavigationNextStop(delivery: Pick<Delivery, 'status' | 'pickup_address' | 'pickup_latitude' | 'pickup_longitude' | 'delivery_address' | 'delivery_latitude' | 'delivery_longitude'>): RiderNavigationStopView {
  if (PICKUP_STAGE_STATUSES.has(delivery.status)) {
    return {
      title: '下一站 · 商家',
      address: delivery.pickup_address,
      latitude: delivery.pickup_latitude,
      longitude: delivery.pickup_longitude
    }
  }

  return {
    title: '下一站 · 顾客',
    address: delivery.delivery_address,
    latitude: delivery.delivery_latitude,
    longitude: delivery.delivery_longitude
  }
}
