import { confirmOrder, OrderResponse } from '../api/order'
import { logger } from '../utils/logger'
import { getErrorUserMessage } from '../utils/user-facing'

type ConfirmReceiptStatus = 'confirmed' | 'cancelled' | 'wechat_opened' | 'failed'

export interface ConfirmReceiptOptions {
  orderId: number
  transactionId?: string
  modalContent?: string
  loadingTitle?: string
  successToastTitle?: string
  confirmFailureToastTitle?: string
  wechatFailureToastTitle?: string
  source?: string
}

export interface ConfirmReceiptResult {
  status: ConfirmReceiptStatus
  order?: OrderResponse
  error?: unknown
}

type OpenBusinessViewOptions = {
  businessType: string
  extraData: Record<string, string>
  success?: (res: unknown) => void
  fail?: (error: unknown) => void
}

type WxWithBusinessView = typeof wx & {
  openBusinessView?: (options: OpenBusinessViewOptions) => void
}

function normalizeOrderId(orderId: number): number {
  return Number.isFinite(orderId) ? orderId : 0
}

function hasWechatOrderConfirmComponent(): boolean {
  return typeof (wx as WxWithBusinessView).openBusinessView === 'function'
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

async function confirmLocally(options: ConfirmReceiptOptions): Promise<ConfirmReceiptResult> {
  const confirmed = await showConfirmModal(options.modalContent || '确认已收到订单？')
  if (!confirmed) {
    return { status: 'cancelled' }
  }

  wx.showLoading({ title: options.loadingTitle || '处理中...' })
  try {
    const order = await confirmOrder(options.orderId)
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

function openWechatOrderConfirm(orderId: number, transactionId: string, source?: string): Promise<ConfirmReceiptResult> {
  return new Promise((resolve) => {
    const app = getApp<IAppOption>()
    app.globalData.pendingConfirmOrderId = orderId

    ;(wx as WxWithBusinessView).openBusinessView?.({
      businessType: 'weappOrderConfirm',
      extraData: { transaction_id: transactionId },
      success() {
        resolve({ status: 'wechat_opened' })
      },
      fail(error: unknown) {
        app.globalData.pendingConfirmOrderId = undefined
        logger.warn('打开微信确认收货组件失败，回退本地确认', error, source || 'order-receipt-confirmation')
        resolve({ status: 'failed', error })
      }
    })
  })
}

export async function confirmReceiptWithRecovery(options: ConfirmReceiptOptions): Promise<ConfirmReceiptResult> {
  const orderId = normalizeOrderId(options.orderId)
  if (!orderId) {
    wx.showToast({ title: '订单信息缺失，请刷新后重试', icon: 'none' })
    return { status: 'failed' }
  }

  const transactionId = (options.transactionId || '').trim()
  if (!transactionId || !hasWechatOrderConfirmComponent()) {
    return confirmLocally({ ...options, orderId })
  }

  const wechatResult = await openWechatOrderConfirm(orderId, transactionId, options.source)
  if (wechatResult.status === 'wechat_opened') {
    return wechatResult
  }

  wx.showToast({
    title: options.wechatFailureToastTitle || '微信确认暂不可用，已改为本页确认',
    icon: 'none'
  })
  return confirmLocally({ ...options, orderId })
}
