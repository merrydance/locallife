import type { ListOrdersParams, OrderResponse, OrderStatus } from '../api/order'
import { isCompletedOrderStatus } from '../api/order'
import type { UserClaimResponse } from '../api/appeals-customer-service'

export type SubmitResultPresentation = {
  icon: string
  color: string
  theme: 'default' | 'success' | 'warning' | 'error'
  title: string
  summary: string
}

export type SelectedClaimOrder = {
  id: number
  orderNo: string
  merchantName: string
  amount: number
  amountDisplay: string
}

export type ClaimOrderOption = SelectedClaimOrder & {
  selected: boolean
}

const CLAIM_ORDER_PAGE_SIZE = 20
const CLAIM_ORDER_STATUS: OrderStatus = 'completed'

export function getClaimCandidateOrderListParams(): ListOrdersParams {
  return {
    page_id: 1,
    page_size: CLAIM_ORDER_PAGE_SIZE,
    status: CLAIM_ORDER_STATUS
  }
}

export function formatClaimAmount(fen: number): string {
  return (Math.max(fen, 0) / 100).toFixed(2)
}

export function isClaimCandidateOrder(order: Pick<OrderResponse, 'status' | 'total_amount'>): boolean {
  return isCompletedOrderStatus(order.status) && order.total_amount > 0
}

export function toSelectedClaimOrder(order: Pick<OrderResponse, 'id' | 'order_no' | 'merchant_name' | 'total_amount'>): SelectedClaimOrder {
  return {
    id: order.id,
    orderNo: order.order_no,
    merchantName: order.merchant_name || '订单',
    amount: order.total_amount,
    amountDisplay: formatClaimAmount(order.total_amount)
  }
}

export function buildClaimOrderOptions(
  orders: OrderResponse[],
  claims: UserClaimResponse[],
  selectedOrderId?: number
): ClaimOrderOption[] {
  const claimedOrderIDs = new Set(claims.map((claim) => claim.order_id))
  return orders
    .filter((order) => isClaimCandidateOrder(order) && !claimedOrderIDs.has(order.id))
    .map((order) => ({
      ...toSelectedClaimOrder(order),
      selected: selectedOrderId === order.id
    }))
}

export function getSubmitResultPresentation(result: {
  payout_status?: string
  decision_status?: string
}): SubmitResultPresentation {
  if (result.payout_status === 'paid') {
    return {
      icon: 'check-circle-filled',
      color: '#2e7d32',
      theme: 'success',
      title: '赔付已到账',
      summary: '平台已受理并完成自动裁定，赔付已到账。'
    }
  }

  if (result.decision_status === 'auto-adjudicated') {
    return {
      icon: 'check-circle-filled',
      color: '#1976d2',
      theme: 'default',
      title: '已自动裁定',
      summary: '平台已受理并完成自动裁定，赔付正在处理中。'
    }
  }

  return {
    icon: 'time-filled',
    color: '#1976d2',
    theme: 'default',
    title: '平台已受理',
    summary: '平台已受理您的反馈，正在为您处理。'
  }
}