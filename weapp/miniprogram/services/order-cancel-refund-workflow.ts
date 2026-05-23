import { cancelOrder, isCancelledOrderStatus } from '../api/order'
import { getRefundStatusView } from '../api/payment'
import { logger } from '../utils/logger'
import { findLatestOrderRefund } from '../utils/order-refund-progress'
import Navigation from '../utils/navigation'

export const CANCEL_REASONS = [
  '不想要了',
  '信息填写错误',
  '商品价格较贵',
  '配送时间太长',
  '其他原因'
]

export const REFUND_TRACK_POLL_INTERVAL_MS = 2000
export const REFUND_TRACK_MAX_ATTEMPTS = 15

export interface CancelRefundOrderDTO {
  status?: string
  paid_at?: string
  cancel_reason?: string
}

export interface OrderCancelRefundViewState {
  showCancelButton: boolean
  showCancelDialog: boolean
  cancelSubmitting: boolean
  cancelReason: string
  cancelReasons: string[]
  cancelRefundExpected: boolean
  cancelRefundPending: boolean
  cancelRefundStatusText: string
  cancelRefundActionText: string
  cancelRefundHint: string
  cancelRefundId: number
  cancelRefundPaymentId: number
}

export interface OrderCancelRefundTimerOwner {
  _refundTimer?: ReturnType<typeof setInterval>
  _refundPolling?: boolean
}

export interface OrderCancelRefundPageLike extends OrderCancelRefundTimerOwner {
  data: OrderCancelRefundViewState & {
    orderId: string
    orderDTO: CancelRefundOrderDTO | null
  }
  setData(partial: Partial<OrderCancelRefundViewState> & { orderDTO?: CancelRefundOrderDTO | null } & Record<string, unknown>): void
  loadOrderDetail?(): Promise<void>
}

export const orderCancelRefundInitialState: OrderCancelRefundViewState = {
  showCancelButton: false,
  showCancelDialog: false,
  cancelSubmitting: false,
  cancelReason: CANCEL_REASONS[0],
  cancelReasons: CANCEL_REASONS,
  cancelRefundExpected: false,
  cancelRefundPending: false,
  cancelRefundStatusText: '',
  cancelRefundActionText: '',
  cancelRefundHint: '',
  cancelRefundId: 0,
  cancelRefundPaymentId: 0
}

function clearRefundTrackingTimer(page: OrderCancelRefundPageLike) {
  if (page._refundTimer) {
    clearInterval(page._refundTimer)
    page._refundTimer = undefined
  }
  page._refundPolling = false
}

export function disposeOrderCancelRefundWorkflow(page: OrderCancelRefundPageLike) {
  clearRefundTrackingTimer(page)
}

function resetRefundTrackingState(page: OrderCancelRefundPageLike) {
  clearRefundTrackingTimer(page)
  page.setData({
    cancelRefundPending: false,
    cancelRefundStatusText: '',
    cancelRefundActionText: '',
    cancelRefundHint: '',
    cancelRefundId: 0,
    cancelRefundPaymentId: 0
  })
}

function buildRefundTrackingHint(status: string, isFailed: boolean): string {
  if (status === 'success') {
    return '退款已完成，请查看详情。'
  }

  if (isFailed) {
    return '退款未能完成，请查看退款详情。'
  }

  return '退款进度已同步，可查看退款详情。'
}

function buildRefundActionText(status: string, isFailed: boolean): string {
  return status === 'success' || isFailed ? '查看退款详情' : '查看退款进度'
}

export async function startRefundTrackingAfterCancel(page: OrderCancelRefundPageLike) {
  if (!page.data.orderDTO || !isCancelledOrderStatus(page.data.orderDTO.status) || !page.data.orderDTO.paid_at) {
    if (!page.data.cancelRefundExpected) {
      resetRefundTrackingState(page)
    } else {
      clearRefundTrackingTimer(page)
    }
    return
  }

  clearRefundTrackingTimer(page)

  page.setData({
    cancelRefundExpected: true,
    cancelRefundPending: true,
    cancelRefundStatusText: '退款处理中',
    cancelRefundActionText: '',
    cancelRefundHint: '订单已取消，系统正在确认退款进度，退款资金将原路退回。',
    cancelRefundId: 0,
    cancelRefundPaymentId: 0
  })

  try {
    const latest = await findLatestOrderRefund(parseInt(page.data.orderId))
    if (latest.refund) {
      const statusView = getRefundStatusView(latest.refund.status)
      clearRefundTrackingTimer(page)
      page.setData({
        cancelRefundPending: false,
        cancelRefundStatusText: statusView.text,
        cancelRefundActionText: buildRefundActionText(statusView.normalizedStatus, statusView.isFailed),
        cancelRefundHint: buildRefundTrackingHint(statusView.normalizedStatus, statusView.isFailed),
        cancelRefundId: latest.refund.id,
        cancelRefundPaymentId: latest.payment?.id || 0
      })
      return
    }

    page.setData({
      cancelRefundPaymentId: latest.payment?.id || 0
    })
    pollRefundTracking(page)
  } catch (error) {
    logger.warn('查询取消退款进度失败，将继续轮询', {
      orderId: page.data.orderId,
      paymentId: page.data.cancelRefundPaymentId || 0
    }, 'Detail.startRefundTrackingAfterCancel')
    logger.warn('查询取消退款进度异常详情', error, 'Detail.startRefundTrackingAfterCancel')
    pollRefundTracking(page)
  }
}

export function pollRefundTracking(page: OrderCancelRefundPageLike) {
  clearRefundTrackingTimer(page)

  let attempts = 0
  page._refundTimer = setInterval(async () => {
    if (page._refundPolling) return
    page._refundPolling = true
    attempts += 1

    try {
      const latest = await findLatestOrderRefund(parseInt(page.data.orderId))
      if (latest.refund) {
        const statusView = getRefundStatusView(latest.refund.status)
        clearRefundTrackingTimer(page)
        page.setData({
          cancelRefundPending: false,
          cancelRefundStatusText: statusView.text,
          cancelRefundActionText: buildRefundActionText(statusView.normalizedStatus, statusView.isFailed),
          cancelRefundHint: buildRefundTrackingHint(statusView.normalizedStatus, statusView.isFailed),
          cancelRefundId: latest.refund.id,
          cancelRefundPaymentId: latest.payment?.id || page.data.cancelRefundPaymentId || 0
        })
        return
      }

      page.setData({
        cancelRefundPaymentId: latest.payment?.id || page.data.cancelRefundPaymentId || 0
      })
    } catch (error) {
      logger.warn('轮询取消退款进度失败，将继续重试', {
        orderId: page.data.orderId,
        paymentId: page.data.cancelRefundPaymentId || 0,
        attempts
      }, 'Detail.pollRefundTracking')
      logger.warn('轮询取消退款进度异常详情', error, 'Detail.pollRefundTracking')
    } finally {
      page._refundPolling = false
    }

    if (attempts >= REFUND_TRACK_MAX_ATTEMPTS) {
      clearRefundTrackingTimer(page)
      logger.warn('取消退款进度轮询已达上限', {
        orderId: page.data.orderId,
        paymentId: page.data.cancelRefundPaymentId || 0,
        attempts
      }, 'Detail.pollRefundTracking')
      page.setData({
        cancelRefundHint: '退款结果还在同步中，请稍后在退款详情页刷新查看。'
      })
    }
  }, REFUND_TRACK_POLL_INTERVAL_MS)
}

export const orderCancelRefundWorkflow = {
  onCancelOrder(this: OrderCancelRefundPageLike) {
    const refundExpected = Boolean(this.data.orderDTO?.paid_at)
    this.setData({
      showCancelDialog: true,
      cancelReason: CANCEL_REASONS[0],
      cancelRefundExpected: refundExpected
    })
  },

  onCloseCancelDialog(this: OrderCancelRefundPageLike) {
    if (this.data.cancelSubmitting) return
    this.setData({
      showCancelDialog: false
    })
  },

  onCancelReasonChange(this: OrderCancelRefundPageLike, e: WechatMiniprogram.CustomEvent) {
    const reason = typeof e.detail?.value === 'string' ? e.detail.value : String(e.detail?.value || '')
    this.setData({
      cancelReason: reason || CANCEL_REASONS[0]
    })
  },

  async confirmCancelOrder(this: OrderCancelRefundPageLike) {
    if (this.data.cancelSubmitting) return

    const reason = this.data.cancelReason || CANCEL_REASONS[0]
    const refundExpected = Boolean(this.data.cancelRefundExpected || this.data.orderDTO?.paid_at)
    this.setData({ cancelSubmitting: true })
    wx.showLoading({ title: '取消中...' })
    try {
      const cancelledOrderDTO = await cancelOrder(parseInt(this.data.orderId), { reason })
      wx.hideLoading()
      this.setData({
        showCancelDialog: false,
        showCancelButton: false
      })
      if (isCancelledOrderStatus(cancelledOrderDTO.status) && refundExpected) {
        const mergedCancelledOrderDTO = {
          ...cancelledOrderDTO,
          paid_at: cancelledOrderDTO.paid_at || this.data.orderDTO?.paid_at
        }
        this.setData({
          orderDTO: mergedCancelledOrderDTO,
          showCancelButton: false,
          cancelRefundExpected: true,
          cancelRefundPending: true,
          cancelRefundStatusText: '退款处理中',
          cancelRefundActionText: '',
          cancelRefundHint: '订单已取消，系统正在确认退款进度，退款资金将原路退回。',
          cancelRefundId: 0,
          cancelRefundPaymentId: 0
        })
        try {
          await this.loadOrderDetail?.()
        } catch (refreshError) {
          logger.warn('取消订单后刷新详情失败，将继续确认退款进度', {
            orderId: this.data.orderId
          }, 'Detail.confirmCancelOrder')
          logger.warn('取消订单后刷新详情异常详情', refreshError, 'Detail.confirmCancelOrder')
        }
        void startRefundTrackingAfterCancel(this)
      } else if (isCancelledOrderStatus(cancelledOrderDTO.status)) {
        try {
          await this.loadOrderDetail?.()
        } catch (refreshError) {
          logger.warn('取消订单后刷新详情失败', refreshError, 'Detail.confirmCancelOrder')
        }
      } else {
        logger.warn('取消订单返回非取消状态', {
          orderId: this.data.orderId,
          status: cancelledOrderDTO.status,
          refundExpected
        }, 'Detail.confirmCancelOrder')
      }
    } catch (error) {
      wx.hideLoading()
      logger.error('取消订单失败', error, 'Detail.confirmCancelOrder')
      wx.showToast({ title: '取消失败，请稍后重试', icon: 'error' })
    } finally {
      this.setData({ cancelSubmitting: false })
    }
  },

  onViewCancelRefundDetail(this: OrderCancelRefundPageLike) {
    if (!this.data.cancelRefundId) {
      wx.showToast({ title: '退款进度同步中，请稍后再试', icon: 'none' })
      return
    }
    Navigation.toRefundDetail(this.data.cancelRefundId)
  }
}
