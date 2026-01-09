import { request } from '../utils/request'

export interface RiderOrderDTO {
  id: string
  status: string
  fee: number // Rider's earning in cents
  merchant_name: string
  merchant_address: string
  merchant_phone?: string
  customer_name?: string
  customer_address: string
  customer_phone?: string
  distance_to_shop: number // meters
  distance_to_deliver: number // meters
  expect_pickup_time: string
  expect_deliver_time: string
  created_at: string
  items?: string[]
}

export interface DepositInfo {
  amount: number
  status: string // PAID, UNPAID, REFUNDED
  paid_at?: string
}

export interface RiderDashboardDTO {
  rider_id: string
  active_tasks: RiderOrderDTO[]
  deposit: DepositInfo
  last_event_at: string
}

export interface RiderMetricsDTO {
  completed_orders: number
  online_hours: number
  rating: number // 0-100
  total_earnings: number // cents
}

/**
 * Get Rider Dashboard Data (Active tasks, deposit info)
 */
export function getRiderDashboard() {
  return request<RiderDashboardDTO>({
    url: '/rider/dashboard',
    method: 'GET'
  })
}

/**
 * Get Rider Metrics (Today's stats)
 */
export function getRiderMetrics() {
  return request<RiderMetricsDTO>({
    url: '/rider/metrics/today',
    method: 'GET'
  })
}

/**
 * Get Available Orders (Pool)
 */
export function getAvailableOrders(page: number = 1, pageSize: number = 20) {
  return request<PagingData<RiderOrderDTO>>({
    url: '/rider/orders/available',
    method: 'GET',
    data: { page, page_size: pageSize }
  })
}

/**
 * Accept an order
 */
export function acceptOrder(orderId: string) {
  return request<void>({
    url: `/rider/orders/${orderId}/accept`,
    method: 'POST'
  })
}

/**
 * Pickup an order
 */
export function pickupOrder(orderId: string) {
  return request<void>({
    url: `/rider/orders/${orderId}/pickup`,
    method: 'POST'
  })
}

/**
 * Deliver an order
 */
export function deliverOrder(orderId: string) {
  return request<void>({
    url: `/rider/orders/${orderId}/deliver`,
    method: 'POST'
  })
}

/**
 * Set Rider Online
 */
export function setRiderOnline(mode: string = 'DELIVERY') {
  return request<void>({
    url: '/rider/online',
    method: 'POST',
    data: { mode }
  })
}

/**
 * Set Rider Offline
 */
export function setRiderOffline() {
  return request<void>({
    url: '/rider/offline',
    method: 'POST'
  })
}

