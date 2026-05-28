import { KitchenOrderResponse } from '../api/order-management'

export type KitchenStatusTheme = 'primary' | 'warning' | 'success'

interface KitchenStatusView {
  label: string
  theme: KitchenStatusTheme
  progressCurrent: number
  canStartPreparing: boolean
  canMarkReady: boolean
  statusHint: string
}

export function getKitchenStatusView(orderOrStatus?: KitchenOrderResponse | KitchenOrderResponse['status'] | string): KitchenStatusView {
  const order = typeof orderOrStatus === 'object' && orderOrStatus !== null ? orderOrStatus : null
  const normalizedStatus = String(order?.kitchen_status || order?.status || orderOrStatus || '').trim().toLowerCase()
  const statusHint = String(order?.status_hint || '').trim()
  const canMarkReady = typeof order?.can_mark_ready === 'boolean'
    ? order.can_mark_ready
    : normalizedStatus === 'preparing'

  if (normalizedStatus === 'ready') {
    return {
      label: '待取餐',
      theme: 'success',
      progressCurrent: 2,
      canStartPreparing: false,
      canMarkReady: false,
      statusHint
    }
  }

  if (normalizedStatus === 'preparing') {
    return {
      label: '制作中',
      theme: 'warning',
      progressCurrent: 1,
      canStartPreparing: false,
      canMarkReady,
      statusHint
    }
  }

  return {
    label: normalizedStatus === 'paid' ? '待制作' : '状态同步中',
    theme: 'primary',
    progressCurrent: 0,
    canStartPreparing: normalizedStatus === 'paid',
    canMarkReady: false,
    statusHint
  }
}
