import { isPaymentWorkflowPaid, waitForPaymentWorkflowTerminalResult } from '../../../services/payment-workflow'
import { checkoutPaidDineInSession } from '../../../services/dine-in-session'
import { buildPaymentResultView, normalizePaymentWorkflowStatus, PaymentResultAction, PaymentWorkflowStatus } from '../../../utils/payment-result-view'
import { getStableBarHeights } from '../../../utils/responsive'
import { getErrorUserMessage } from '../../../utils/user-facing'

function formatAmount(amountFen?: number): string {
  return typeof amountFen === 'number' ? (amountFen / 100).toFixed(2) : ''
}

function buildReservationListUrl(returnStatus?: string): string {
  const status = returnStatus || 'all'
  return `/pages/user_center/reservations/index?status=${encodeURIComponent(status)}`
}

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

let terminalWaitToken = 0
let dineInCheckoutClosing = false

Page({
  data: {
    navBarHeight: 88,
    navTitle: '支付结果',
    title: '',
    description: '',
    theme: 'warning',
    primaryButtonText: '刷新状态',
    primaryAction: 'refresh_status' as PaymentResultAction,
    secondaryButtonText: '查看详情',
    secondaryAction: 'detail_page' as PaymentResultAction,
    status: 'pending_confirmation' as PaymentWorkflowStatus,
    paymentOrderId: 0,
    businessId: '',
    businessType: 'order',
    returnStatus: '',
    orderNo: '',
    amount: '',
    statusNote: '',
    refreshing: false,
    waitingForTerminal: false,
    showSummary: false
  },

  onLoad(options: {
    status?: string
    paymentOrderId?: string
    businessId?: string
    businessType?: string
    orderNo?: string
    amount?: string
    returnStatus?: string
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
      returnStatus: options.returnStatus || '',
      orderNo: options.orderNo || '',
      amount: options.amount || '',
      showSummary: Boolean(options.amount || options.orderNo || paymentOrderId),
      ...view
    })

    if (status === 'pending_confirmation') {
      if (paymentOrderId) {
        this.startTerminalWait()
      } else {
        this.applyMissingPaymentOrderState()
      }
    } else if (isPaymentWorkflowPaid(status)) {
      void this.closeDineInCheckoutSessionIfNeeded()
    }
  },

  onUnload() {
    terminalWaitToken += 1
  },

  applyMissingPaymentOrderState() {
    const view = buildPaymentResultView('pay_params_missing')
    this.setData({
      status: 'pay_params_missing',
      statusNote: '缺少可查询的支付单，请返回详情页查看最新状态。',
      waitingForTerminal: false,
      refreshing: false,
      ...view
    })
  },

  startTerminalWait() {
    const token = terminalWaitToken + 1
    terminalWaitToken = token
    this.setData({
      navTitle: '支付确认中',
      status: 'pending_confirmation',
      statusNote: '',
      refreshing: true,
      waitingForTerminal: true
    })
    void this.waitForTerminalPaymentResult(token)
  },

  async waitForTerminalPaymentResult(token: number) {
    while (token === terminalWaitToken && this.data.paymentOrderId) {
      try {
        const result = await waitForPaymentWorkflowTerminalResult(this.data.paymentOrderId)
        if (token !== terminalWaitToken) {
          return
        }

        const view = buildPaymentResultView(result.status)
        this.setData({
          status: result.status,
          businessId: result.businessId ? String(result.businessId) : this.data.businessId,
          businessType: result.businessType ? String(result.businessType) : this.data.businessType,
          orderNo: result.outTradeNo || this.data.orderNo,
          amount: this.data.amount || formatAmount(result.amountFen),
          showSummary: Boolean(this.data.amount || result.amountFen || result.outTradeNo || this.data.orderNo || this.data.paymentOrderId),
          statusNote: '',
          refreshing: false,
          waitingForTerminal: false,
          ...view
        })
        if (isPaymentWorkflowPaid(result.status)) {
          void this.closeDineInCheckoutSessionIfNeeded()
        }
        return
      } catch (error) {
        if (token !== terminalWaitToken) {
          return
        }

        this.setData({
          statusNote: getErrorUserMessage(error, '支付结果还没有回写完成，系统正在继续确认。')
        })
        await delay(1500)
      }
    }
  },

  async refreshPaymentStatus(silent: boolean = false) {
    if (!this.data.paymentOrderId || this.data.refreshing) {
      if (!this.data.paymentOrderId) {
        this.setData({ statusNote: '暂无可刷新支付单，请返回详情页查看最新状态。' })
      }
      return
    }

    if (this.data.status === 'pending_confirmation') {
      this.startTerminalWait()
      return
    }

    this.setData({ refreshing: true, statusNote: silent ? this.data.statusNote : '' })

    try {
      const result = await waitForPaymentWorkflowTerminalResult(this.data.paymentOrderId, { maxAttempts: 1, interval: 0 })
      const view = buildPaymentResultView(result.status)
      this.setData({
        status: result.status,
        businessId: result.businessId ? String(result.businessId) : this.data.businessId,
        businessType: result.businessType ? String(result.businessType) : this.data.businessType,
        orderNo: result.outTradeNo || this.data.orderNo,
        amount: this.data.amount || formatAmount(result.amountFen),
        showSummary: Boolean(this.data.amount || result.amountFen || result.outTradeNo || this.data.orderNo || this.data.paymentOrderId),
        statusNote: '',
        refreshing: false,
        ...view
      })
      if (isPaymentWorkflowPaid(result.status)) {
        void this.closeDineInCheckoutSessionIfNeeded()
      }
    } catch (error) {
      this.setData({
        statusNote: getErrorUserMessage(error, '支付结果暂未同步，请稍后刷新。'),
        refreshing: false
      })
    }
  },

  async closeDineInCheckoutSessionIfNeeded() {
    if (dineInCheckoutClosing) return
    dineInCheckoutClosing = true
    try {
      await checkoutPaidDineInSession({
        orderId: Number(this.data.businessId) || undefined,
        paymentOrderId: this.data.paymentOrderId || undefined
      })
    } catch (error) {
      this.setData({
        statusNote: getErrorUserMessage(error, '支付已完成，桌台状态正在同步，请稍后刷新。')
      })
    } finally {
      dineInCheckoutClosing = false
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
      if (this.data.businessType === 'reservation') {
        wx.redirectTo({ url: buildReservationListUrl(this.data.returnStatus) })
        return
      }
      wx.redirectTo({ url: '/pages/orders/list/index' })
      return
    }

    if (action === 'home') {
      wx.switchTab({ url: '/pages/takeout/index' })
      return
    }

    if (this.data.businessId) {
      if (this.data.businessType === 'reservation') {
        wx.redirectTo({ url: `/pages/reservation/detail/index?id=${this.data.businessId}` })
        return
      }
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