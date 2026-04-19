import { KitchenOrderResponse } from '../api/order-management'

export type KitchenStatusTheme = 'primary' | 'warning' | 'success'

interface KitchenStatusView {
  label: string
  theme: KitchenStatusTheme
  progressCurrent: number
  canStartPreparing: boolean
  canMarkReady: boolean
}

export function getKitchenStatusView(status?: KitchenOrderResponse['status'] | string): KitchenStatusView {
  const normalizedStatus = String(status || '').trim().toLowerCase()

  if (normalizedStatus === 'ready') {
    return {
      label: '待取餐',
      theme: 'success',
      progressCurrent: 2,
      canStartPreparing: false,
      canMarkReady: false
    }
  }

  if (normalizedStatus === 'preparing') {
    return {
      label: '制作中',
      theme: 'warning',
      progressCurrent: 1,
      canStartPreparing: false,
      canMarkReady: true
    }
  }

  return {
    label: normalizedStatus === 'paid' ? '待制作' : '状态同步中',
    theme: 'primary',
    progressCurrent: 0,
    canStartPreparing: normalizedStatus === 'paid',
    canMarkReady: false
  }
}