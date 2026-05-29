import type { OrderResponse } from '../_api/order-management'

export function canMerchantMarkOrderReady(order: Pick<OrderResponse, 'status' | 'order_type'> & Partial<Pick<OrderResponse, 'fulfillment_status' | 'can_mark_ready'>>): boolean {
  if (typeof order.can_mark_ready === 'boolean') {
    return order.can_mark_ready
  }

  if (order.order_type === 'takeout') {
    return order.status === 'preparing' ||
      (order.status === 'courier_accepted' && order.fulfillment_status === 'preparing')
  }

  return order.status === 'preparing'
}
