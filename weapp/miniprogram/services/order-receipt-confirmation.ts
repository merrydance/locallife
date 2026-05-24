import { confirmOrder, OrderResponse } from '../api/order'
import { logger } from '../utils/logger'
import { getErrorUserMessage } from '../utils/user-facing'

type ConfirmReceiptStatus = 'confirmed' | 'cancelled' | 'failed'

export interface ConfirmReceiptOptions {
  orderId: number
  modalContent?: string
  loadingTitle?: string
  successToastTitle?: string
  confirmFailureToastTitle?: string
  source?: string
}

export interface ConfirmReceiptResult {
  status: ConfirmReceiptStatus
  order?: OrderResponse
  error?: unknown
}

function normalizeOrderId(orderId: number): number {
  return Number.isFinite(orderId) ? orderId : 0
}

function showConfirmModal(content: string): Promise<boolean> {
  return new Promise((resolve) => {
    wx.showModal({
      title: '确认收货',
      content,
      confirmText: '确认',
      cancelText: '再等等',
      success: (res) => resolve(!!res.confirm),
      fail: () => resolve(false)
    })
  })
}

export async function confirmReceiptWithRecovery(options: ConfirmReceiptOptions): Promise<ConfirmReceiptResult> {
  const orderId = normalizeOrderId(options.orderId)
  if (!orderId) {
    wx.showToast({ title: '订单信息缺失，请刷新后重试', icon: 'none' })
    return { status: 'failed' }
  }

  const confirmed = await showConfirmModal(options.modalContent || '确认已收到订单？')
  if (!confirmed) {
    return { status: 'cancelled' }
  }

  wx.showLoading({ title: options.loadingTitle || '处理中...' })
  try {
    const order = await confirmOrder(orderId)
    wx.hideLoading()
    wx.showToast({ title: options.successToastTitle || '已确认收货', icon: 'none' })
    return { status: 'confirmed', order }
  } catch (error: unknown) {
    wx.hideLoading()
    logger.error('确认收货失败', error, options.source || 'order-receipt-confirmation')
    wx.showToast({
      title: getErrorUserMessage(error, options.confirmFailureToastTitle || '确认失败，请稍后重试'),
      icon: 'none'
    })
    return { status: 'failed', error }
  }
}
