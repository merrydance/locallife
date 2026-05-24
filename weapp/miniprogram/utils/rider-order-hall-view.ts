import type { RecommendedOrder } from '../api/delivery'

export type RecommendedOrderCardView = RecommendedOrder & {
  pickup_address: string
  delivery_address: string
  distance_km: string
  pickup_distance_km: string
  route_distance_km: string
  estimated_duration: number
  deadline_desc: string
  expected_delivery_time_label: string
  is_very_urgent: boolean
}

function toTimestamp(value?: string): number | null {
  if (!value) {
    return null
  }

  const timestamp = new Date(value).getTime()
  return Number.isFinite(timestamp) && timestamp > 0 ? timestamp : null
}

function formatClock(timestamp: number): string {
  const date = new Date(timestamp)
  const hours = date.getHours().toString().padStart(2, '0')
  const minutes = date.getMinutes().toString().padStart(2, '0')
  return `${hours}:${minutes}`
}

function formatExpectedDeliveryDeadline(expectedDeliveryAt?: string, now: number = Date.now()): string {
  const expectedTimestamp = toTimestamp(expectedDeliveryAt)
  if (expectedTimestamp === null) {
    return '尽快接单'
  }

  const diff = expectedTimestamp - now
  if (diff <= 0) {
    return '已超时'
  }

  if (diff < 60 * 60 * 1000) {
    return `剩 ${Math.max(1, Math.ceil(diff / 60000))} 分钟`
  }

  return `${formatClock(expectedTimestamp)} 前`
}

export function buildRecommendedOrderCardView(
  order: RecommendedOrder,
  now: number = Date.now()
): RecommendedOrderCardView {
  const expectedTimestamp = toTimestamp(order.expected_delivery_at)

  return {
    ...order,
    pickup_address: order.merchant_address || '取餐地址待同步',
    delivery_address: order.customer_address || '送达地址待同步',
    distance_km: ((order.distance || 0) / 1000).toFixed(1),
    pickup_distance_km: ((order.distance_to_pickup || 0) / 1000).toFixed(1),
    route_distance_km: ((order.distance || 0) / 1000).toFixed(1),
    estimated_duration: order.estimated_minutes || 0,
    deadline_desc: formatExpectedDeliveryDeadline(order.expected_delivery_at, now),
    expected_delivery_time_label: expectedTimestamp === null ? '尽快' : formatClock(expectedTimestamp),
    is_very_urgent: expectedTimestamp !== null && expectedTimestamp > now && expectedTimestamp - now < 15 * 60 * 1000
  }
}
