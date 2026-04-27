import { queryPaymentWorkflowResult } from '../../../services/payment-workflow'
import { buildPaymentResultView, normalizePaymentWorkflowStatus, PaymentResultAction, PaymentWorkflowStatus } from '../../../utils/payment-result-view'
import { getStableBarHeights } from '../../../utils/responsive'
import { getErrorUserMessage } from '../../../utils/user-facing'

function formatAmount(amountFen?: number): string {
  return typeof amountFen === 'number' ? (amountFen / 100).toFixed(2) : ''
}

Page({
  data: {
    navBarHeight: 88,
    navTitle: '支付结果',
    title: '支付结果确认中',
    description: '支付已提交，系统正在同步微信支付结果。',
    theme: 'warning',
    primaryButtonText: '刷新状态',
    primaryAction: 'refresh_status' as PaymentResultAction,
    secondaryButtonText: '查看详情',
    secondaryAction: 'detail_page' as PaymentResultAction,
    status: 'pending_confirmation' as PaymentWorkflowStatus,
    paymentOrderId: 0,
    businessId: '',
    businessType: 'order',
    orderNo: '',
    amount: '',
    statusNote: '',
    refreshing: false,
    showSummary: false
  },

  onLoad(options: {
    status?: string
    paymentOrderId?: string
    businessId?: string
    businessType?: string
    orderNo?: string
    amount?: string
  }) {
    const { navBarHeight } = getStableBarHeights()
    const status = normalizePaymentWorkflowStatus(options.status)
    const view = buildPaymentResultView(status)
    const paymentOrderId = Number(options.paymentOrderId || '0') || 0

    this.setData({
      navBarHeight,
      status,
      paymentOrderId,
      businessId: options.businessId || '',
      businessType: options.businessType || 'order',
      orderNo: options.orderNo || '',
      amount: options.amount || '',
      showSummary: Boolean(options.amount || options.orderNo || paymentOrderId),
      ...view
    })

    if (status === 'pending_confirmation' && paymentOrderId) {
      this.refreshPaymentStatus(true)
    }
  },

  async refreshPaymentStatus(silent: boolean = false) {
    if (!this.data.paymentOrderId || this.data.refreshing) {
      if (!this.data.paymentOrderId) {
        this.setData({ statusNote: '暂无可刷新支付单，请返回详情页查看最新状态。' })
      }
      return
    }

    this.setData({ refreshing: true, statusNote: silent ? this.data.statusNote : '' })

    try {
      const result = await queryPaymentWorkflowResult(this.data.paymentOrderId)
      const view = buildPaymentResultView(result.status)
      this.setData({
        status: result.status,
        businessId: result.businessId ? String(result.businessId) : this.data.businessId,
        businessType: result.businessType ? String(result.businessType) : this.data.businessType,
        orderNo: result.outTradeNo || this.data.orderNo,
        amount: this.data.amount || formatAmount(result.amountFen),
        showSummary: Boolean(this.data.amount || result.amountFen || result.outTradeNo || this.data.orderNo || this.data.paymentOrderId),
        statusNote: result.status === 'pending_confirmation' ? '支付结果仍在同步中，请稍后刷新。' : '',
        refreshing: false,
        ...view
      })
    } catch (error) {
      this.setData({
        statusNote: getErrorUserMessage(error, '支付结果暂未同步，请稍后刷新。'),
        refreshing: false
      })
    }
  },

  onPrimaryAction() {
    this.applyAction(this.data.primaryAction)
  },

  onSecondaryAction() {
    this.applyAction(this.data.secondaryAction)
  },

  applyAction(action: PaymentResultAction) {
    if (action === 'refresh_status') {
      this.refreshPaymentStatus(false)
      return
    }

    if (action === 'list_page') {
      wx.redirectTo({ url: '/pages/orders/list/index' })
      return
    }

    if (action === 'home') {
      wx.switchTab({ url: '/pages/takeout/index' })
      return
    }

    if (this.data.businessId) {
      wx.redirectTo({ url: `/pages/orders/detail/index?id=${this.data.businessId}` })
      return
    }

    if (this.data.paymentOrderId) {
      wx.redirectTo({ url: `/pages/user_center/payment-detail/index?id=${this.data.paymentOrderId}` })
      return
    }

    wx.redirectTo({ url: '/pages/orders/list/index' })
  }
})