import type { DeliveryProgressView, DeliveryResponse } from '../_main_shared/api/delivery'
import type { OrderResponse, OrderStatus, OrderType } from '../_main_shared/api/order'

export type CustomerOrderStatusGroup =
  | 'pending'
  | 'preparing'
  | 'ready'
  | 'delivering'
  | 'completed'
  | 'cancelled'

export type CustomerOrderStatusIcon = 'time' | 'timer' | 'cart' | 'check-circle' | 'close-circle'
export type CustomerDeliveryRemainingRouteStage = 'pickup' | 'delivery' | 'none'

export interface CustomerOrderStatusView {
  rawStatus: string
  label: string
  group: CustomerOrderStatusGroup
  className: string
  color: string
  description: string
  icon: CustomerOrderStatusIcon
  canTrack: boolean
  canConfirmReceipt: boolean
  shouldPoll: boolean
  isLocationTracked: boolean
  remainingRouteStage: CustomerDeliveryRemainingRouteStage
}

type OrderStatusSource = Pick<
  OrderResponse,
  'status' | 'status_hint' | 'order_type' | 'fulfillment_status' | 'order_no' | 'merchant_name' | 'cancel_reason'
>

type DeliveryStatus = DeliveryResponse['status']

const STATUS_META: Record<CustomerOrderStatusGroup, { color: string, icon: CustomerOrderStatusIcon }> = {
  pending: { color: '#E34D59', icon: 'timer' },
  preparing: { color: '#0052D9', icon: 'time' },
  ready: { color: '#0052D9', icon: 'time' },
  delivering: { color: '#0052D9', icon: 'cart' },
  completed: { color: '#00A870', icon: 'check-circle' },
  cancelled: { color: '#999999', icon: 'close-circle' }
}

const DELIVERY_TO_ORDER_STATUS: Record<DeliveryStatus, OrderStatus> = {
  pending: 'ready',
  assigned: 'courier_accepted',
  picking: 'courier_accepted',
  picked: 'picked',
  delivering: 'delivering',
  delivered: 'rider_delivered',
  completed: 'user_delivered',
  cancelled: 'cancelled',
  exception: 'ready'
}

export function buildCustomerOrderStatusView(order: OrderStatusSource): CustomerOrderStatusView {
  return buildView({
    status: order.status,
    orderType: order.order_type,
    fulfillmentStatus: order.fulfillment_status,
    statusHint: order.status_hint,
    orderNo: order.order_no,
    merchantName: order.merchant_name,
    cancelReason: order.cancel_reason
  })
}

export function buildCustomerDeliveryTrackingStatusView(delivery: DeliveryResponse): CustomerOrderStatusView {
  const orderStatus = normalizeOrderStatus(delivery.order_status) || DELIVERY_TO_ORDER_STATUS[delivery.status] || 'ready'

  return buildView({
    status: orderStatus,
    orderType: 'takeout',
    fulfillmentStatus: delivery.fulfillment_status,
    deliveryStatus: delivery.status
  })
}

export function shouldPollCustomerDeliveryTrackingState(status?: DeliveryStatus): boolean {
  if (!status) return false
  return status === 'pending'
    || status === 'assigned'
    || status === 'picking'
    || status === 'picked'
    || status === 'delivering'
}

export function buildCustomerOrderDeliveryProgress(delivery: DeliveryResponse, formatTime: (timeStr: string) => string): DeliveryProgressView[] {
  const statusView = buildCustomerDeliveryTrackingStatusView(delivery)
  const status = delivery.status
  const isAtLeastAssigned = status === 'assigned'
    || status === 'picking'
    || status === 'picked'
    || status === 'delivering'
    || status === 'delivered'
    || status === 'completed'
  const isAtLeastPicked = status === 'picked'
    || status === 'delivering'
    || status === 'delivered'
    || status === 'completed'
  const isAtLeastDelivering = status === 'delivering'
    || status === 'delivered'
    || status === 'completed'
  const isDelivered = status === 'delivered' || status === 'completed'

  return [
    {
      title: '商家已接单',
      time: delivery.created_at ? formatTime(delivery.created_at) : '',
      done: true,
      active: status === 'pending'
    },
    {
      title: '骑手已接单',
      time: delivery.assigned_at ? formatTime(delivery.assigned_at) : '',
      done: !!delivery.assigned_at || isAtLeastAssigned,
      active: status === 'assigned' || status === 'picking'
    },
    {
      title: '骑手已取餐',
      time: delivery.picked_at ? formatTime(delivery.picked_at) : '',
      done: !!delivery.picked_at || isAtLeastPicked,
      active: status === 'picked'
    },
    {
      title: '代取中',
      time: '',
      done: isAtLeastDelivering,
      active: status === 'delivering'
    },
    {
      title: statusView.label,
      time: delivery.delivered_at ? formatTime(delivery.delivered_at) : '',
      done: !!delivery.delivered_at || isDelivered,
      active: status === 'delivered'
    }
  ]
}

function buildView(input: {
  status: OrderStatus
  orderType: OrderType
  fulfillmentStatus?: string
  statusHint?: string
  deliveryStatus?: DeliveryStatus
  orderNo?: string
  merchantName?: string
  cancelReason?: string
}): CustomerOrderStatusView {
  const group = getStatusGroup(input.status, input.orderType, input.fulfillmentStatus)
  const meta = STATUS_META[group]
  const label = (input.statusHint && input.statusHint.trim()) || getStatusLabel(input.status, input.orderType, input.fulfillmentStatus, input.deliveryStatus)
  const canTrack = input.orderType === 'takeout' && isTrackableOrderStatus(input.status)
  const canConfirmReceipt = input.status === 'rider_delivered' || input.deliveryStatus === 'delivered'
  const shouldPoll = input.orderType === 'takeout' && shouldPollStatus(input.status, input.deliveryStatus)
  const isLocationTracked = isLocationTrackedStatus(input.status, input.deliveryStatus)
  const remainingRouteStage = getRemainingRouteStage(input.deliveryStatus)

  return {
    rawStatus: input.status,
    label,
    group,
    className: `class-${group}`,
    color: meta.color,
    description: getStatusDescription(input, label),
    icon: meta.icon,
    canTrack,
    canConfirmReceipt,
    shouldPoll,
    isLocationTracked,
    remainingRouteStage
  }
}

function getRemainingRouteStage(status?: DeliveryStatus): CustomerDeliveryRemainingRouteStage {
  if (status === 'assigned' || status === 'picking') {
    return 'pickup'
  }
  if (status === 'picked' || status === 'delivering') {
    return 'delivery'
  }
  return 'none'
}

function getStatusGroup(status: OrderStatus, orderType: OrderType, fulfillmentStatus?: string): CustomerOrderStatusGroup {
  if (status === 'cancelled') return 'cancelled'
  if (status === 'completed' || status === 'user_delivered') return 'completed'
  if (status === 'courier_accepted' || status === 'picked' || status === 'delivering' || status === 'rider_delivered') {
    return 'delivering'
  }
  if (status === 'ready') return 'ready'
  if (status === 'pending') return 'pending'
  if (orderType === 'reservation' && status === 'paid' && !isActiveFulfillmentStatus(fulfillmentStatus)) {
    return 'pending'
  }
  return 'preparing'
}

function getStatusLabel(status: OrderStatus, orderType: OrderType, fulfillmentStatus?: string, deliveryStatus?: DeliveryStatus): string {
  if (deliveryStatus === 'exception') return '代取异常'

  if (orderType === 'reservation' && status === 'paid' && !isActiveFulfillmentStatus(fulfillmentStatus)) {
    return '等待制作'
  }

  if (status === 'ready') {
    if (orderType === 'takeaway') return '请到店取餐'
    if (orderType === 'dine_in' || orderType === 'reservation') return '已出餐/已上齐'
    return '等待跑腿接单'
  }

  const labels: Record<OrderStatus, string> = {
    pending: '待支付',
    paid: '商家已接单',
    preparing: '制作中',
    ready: '等待跑腿接单',
    courier_accepted: '骑手已接单',
    picked: '骑手已取餐',
    delivering: '代取中',
    rider_delivered: '已送达待确认',
    user_delivered: '已送达',
    completed: '已完成',
    cancelled: '已取消'
  }
  return labels[status] || status
}

function getStatusDescription(input: {
  status: OrderStatus
  orderType: OrderType
  deliveryStatus?: DeliveryStatus
  orderNo?: string
  merchantName?: string
  cancelReason?: string
}, label: string): string {
  if (input.cancelReason && input.status === 'cancelled') return input.cancelReason

  if (input.orderType === 'reservation') return '预订点菜订单'
  if (input.orderType === 'dine_in') return '堂食订单'
  if (input.orderType === 'takeaway') return input.status === 'ready' ? '餐品已备好，请到店取餐' : '打包自取订单'

  const descriptions: Partial<Record<OrderStatus, string>> = {
    pending: '订单等待支付，请在15分钟内完成',
    paid: '商家已收到订单，正在确认',
    preparing: '商家正在制作您的餐品',
    ready: '商家已备餐，等待跑腿接单',
    courier_accepted: '骑手已接单，正在前往取餐',
    picked: '骑手已取餐，正在送往收货地址',
    delivering: '骑手正在代取中，请耐心等待',
    rider_delivered: '订单已送达，请确认收餐',
    user_delivered: '订单已送达',
    completed: input.merchantName ? `感谢您对${input.merchantName}的信任` : '订单已完成，感谢您的惠顾',
    cancelled: '订单已取消'
  }

  return descriptions[input.status] || (input.orderNo ? `订单编号: ${input.orderNo}` : label)
}

function shouldPollStatus(status: OrderStatus, deliveryStatus?: DeliveryStatus): boolean {
  if (deliveryStatus) {
    return deliveryStatus === 'pending'
      || deliveryStatus === 'assigned'
      || deliveryStatus === 'picking'
      || deliveryStatus === 'picked'
      || deliveryStatus === 'delivering'
  }
  return status === 'ready'
    || status === 'courier_accepted'
    || status === 'picked'
    || status === 'delivering'
}

function isLocationTrackedStatus(status: OrderStatus, deliveryStatus?: DeliveryStatus): boolean {
  if (deliveryStatus) {
    return deliveryStatus === 'assigned'
      || deliveryStatus === 'picking'
      || deliveryStatus === 'picked'
      || deliveryStatus === 'delivering'
  }
  return status === 'courier_accepted'
    || status === 'picked'
    || status === 'delivering'
}

function isTrackableOrderStatus(status: OrderStatus): boolean {
  return status === 'courier_accepted'
    || status === 'picked'
    || status === 'delivering'
    || status === 'rider_delivered'
}

function isActiveFulfillmentStatus(status?: string): boolean {
  return status === 'preparing' || status === 'ready' || status === 'completed'
}

function normalizeOrderStatus(status?: string): OrderStatus | undefined {
  const statuses: OrderStatus[] = [
    'pending',
    'paid',
    'preparing',
    'ready',
    'courier_accepted',
    'picked',
    'delivering',
    'rider_delivered',
    'user_delivered',
    'completed',
    'cancelled'
  ]
  return statuses.includes(status as OrderStatus) ? status as OrderStatus : undefined
}
