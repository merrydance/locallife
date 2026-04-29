import type { OrderCardViewModel } from '../adapters/order-card'
import type { CombinedPaymentOrderResponse } from '../api/payment'
import type { OrderResponse, OrderStatus, OrderType } from '../api/order'
import Navigation from './navigation'
import { getCombinedPaymentFollowupMessage, isCombinedPaymentSuccessful, shouldRecreateCombinedPayment } from '../api/payment'

export const STATUS_TABS = [
  { label: '全部', value: '' },
  { label: '待支付', value: 'pending' },
  { label: '制作中', value: 'preparing' },
  { label: '配送中', value: 'delivering' },
  { label: '已完成', value: 'completed' },
  { label: '已取消', value: 'cancelled' }
]

export const CANCEL_REASONS = [
  '不想要了',
  '信息填写错误',
  '商品价格较贵',
  '配送时间太长',
  '其他原因'
]

export const ORDER_REQUEST_DEDUP_MS = 800
export type OrderTypeFilter = OrderType | ''
export type OrderStatusFilter = OrderStatus | ''

const VALID_ORDER_TYPES: OrderType[] = ['takeout', 'reservation', 'dine_in', 'takeaway']
const VALID_STATUS_FILTERS: OrderStatus[] = ['pending', 'preparing', 'delivering', 'completed', 'cancelled']

export const normalizeOrderType = (value?: string): OrderTypeFilter => {
  if (value && VALID_ORDER_TYPES.includes(value as OrderType)) {
    return value as OrderType
  }
  return ''
}

export const normalizeOrderStatusFilter = (value?: string): OrderStatusFilter => {
  if (!value || value === 'all') {
    return ''
  }
  if (VALID_STATUS_FILTERS.includes(value as OrderStatus)) {
    return value as OrderStatus
  }
  return ''
}

export const normalizeSelectMode = (value?: string): boolean => {
  return value === '1' || value === 'true'
}

export const getDatasetId = (event: WechatMiniprogram.BaseEvent): number | null => {
  const dataset = event.currentTarget.dataset as { id?: string | number }
  const id = dataset.id
  const numericId = typeof id === 'number' ? id : Number(id)
  return Number.isFinite(numericId) ? numericId : null
}

export const isOrderResponse = (value: unknown): value is OrderResponse => {
  return !!value && typeof value === 'object' && 'id' in value && 'order_no' in value
}

export const isWechatPayCancelled = (error: unknown): boolean => {
  const wxError = error as { errMsg?: string }
  return !!wxError?.errMsg?.includes('cancel')
}

export const navigateToCombinedPaymentSuccess = (combinedPayment: CombinedPaymentOrderResponse, orderIds: number[]) => {
  const firstOrderId = combinedPayment.sub_orders?.[0]?.order_id || orderIds[0]
  Navigation.toPaymentResult({
    status: 'paid',
    businessId: firstOrderId,
    businessType: 'order',
    orderNo: combinedPayment.combine_out_trade_no || String(firstOrderId),
    amount: (combinedPayment.total_amount / 100).toFixed(2)
  })
}

export const getCombinedPaymentToastMessage = (combinedPayment: CombinedPaymentOrderResponse): string => {
  const baseMessage = getCombinedPaymentFollowupMessage(combinedPayment)
  if (shouldRecreateCombinedPayment(combinedPayment)) {
    return baseMessage
  }

  return `${baseMessage}，订单列表稍后会自动同步。`
}

export const getSharedCombinedPaymentID = (orders: OrderCardViewModel[], orderIds: number[]): number | null => {
  const selectedOrders = orders.filter((order) => orderIds.includes(order.id))
  if (selectedOrders.length === 0) {
    return null
  }

  const firstPaymentID = selectedOrders[0].paymentContext?.combined_payment_id
  if (!firstPaymentID) {
    return null
  }

  return selectedOrders.every((order) => order.paymentContext?.combined_payment_id === firstPaymentID)
    ? firstPaymentID
    : null
}

export const isCombinedPaymentReady = (combinedPayment: CombinedPaymentOrderResponse) => {
  if (isCombinedPaymentSuccessful(combinedPayment)) {
    return 'completed' as const
  }
  if (shouldRecreateCombinedPayment(combinedPayment)) {
    return 'fallback' as const
  }
  return 'handled' as const
}