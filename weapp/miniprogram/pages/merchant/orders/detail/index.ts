import { getStableBarHeights } from '../../../../utils/responsive'
import {
  MerchantOrderManagementService,
  OrderResponse,
  OrderManagementAdapter,
  MERCHANT_REJECT_REASON_OPTIONS
} from '../../_api/order-management'
import { createRefund, getMerchantRefundReturns, getPaymentRefunds, getPayments } from '../../_main_shared/api/payment'
import { logger } from '../../../../utils/logger'
import { isSettledFulfilled, isSettledRejected, settleAll } from '../../../../utils/promise'
import dayjs from '../../_main_shared/miniprogram_npm/dayjs/index'
import { getErrorUserMessage } from '../../../../utils/user-facing'
import { waitForRefundTerminalResult } from '../../_main_shared/services/refund-workflow'
import {
  buildPaymentView,
  buildMerchantOrderFeeBreakdownView,
  buildMerchantOrderTimeline,
  buildPrintJobView,
  buildRefundView,
  canMerchantOrderManualPrint,
  createDefaultRefundForm,
  getCloudStatusLabel,
  getMerchantOrderFallbackStatusHint,
  getMerchantOrderStatusDesc,
  getMerchantOrderStatusIcon,
  type MerchantOrderDetailOptions,
  type MerchantOrderDetailView,
  type MerchantOrderPaymentView,
  type MerchantOrderPrintJobView,
  type MerchantOrderRefundView,
  type RefundFormData,
  selectActivePayment
} from '../../_utils/merchant-order-detail-view'

const getErrorMessage = getErrorUserMessage

function buildRefundIdempotencyKey(orderId: number, paymentId: number) {
  return `merchant-manual-refund:${orderId}:${paymentId}:${Date.now()}:${Math.random().toString(36).slice(2, 10)}`
}

Page({
  data: {
    navBarHeight: 88,
    orderId: 0,
    order: null as MerchantOrderDetailView | null,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    printJobsLoaded: false,
    printJobsError: false,
    printJobsErrorMessage: '',
    printJobs: [] as MerchantOrderPrintJobView[],
    payment: null as MerchantOrderPaymentView | null,
    refundsLoaded: false,
    refundsError: false,
    refundsErrorMessage: '',
    refundReturnsErrorMessage: '',
    refundNoticeMessage: '',
    refunds: [] as MerchantOrderRefundView[],
    refundPopupVisible: false,
    refundSubmitting: false,
    refundAvailableTypes: ['full', 'partial'] as Array<'full' | 'partial'>,
    refundForm: createDefaultRefundForm(),
    refundIdempotencyKey: '',
    retryingPrintJobId: 0,
    queryingPrintJobStatusId: 0,
    manualPrinting: false,
    loading: true,
    submitting: false,
    isIPhoneX: false
  },

  onLoad(options: MerchantOrderDetailOptions) {
    const { navBarHeight } = getStableBarHeights()
    const { model } = wx.getSystemInfoSync()
    const isIPhoneX = model.includes('iPhone X') || model.includes('iPhone 11') || model.includes('iPhone 12') || model.includes('iPhone 13')

    this.setData({
      navBarHeight,
      isIPhoneX,
      orderId: parseInt(options.id || '0')
    })

    if (!this.data.orderId) {
      this.setData({
        loading: false,
        initialError: true,
        initialErrorMessage: '缺少订单标识，无法查看详情'
      })
      return
    }

    this.loadDetail()
  },

  onPullDownRefresh() {
    this.loadDetail(false)
  },

  onRetry() {
    this.loadDetail(true)
  },

  onRetryRefresh() {
    this.loadDetail(false)
  },

  async loadDetail(showLoading = true) {
    const canPreserveDetail = !showLoading && Boolean(this.data.order)
    const canPreservePrintJobs = !showLoading && this.data.printJobsLoaded
    const canPreserveRefunds = !showLoading && this.data.refundsLoaded
    this.setData({
      loading: true,
      ...(showLoading
        ? { initialError: false, initialErrorMessage: '', refreshErrorMessage: '' }
        : canPreserveDetail
          ? { refreshErrorMessage: '' }
          : {})
    })
    try {
      const [orderResult, printJobsResult, paymentsResult] = await settleAll([
        MerchantOrderManagementService.getOrderDetail(this.data.orderId),
        MerchantOrderManagementService.listOrderPrintJobs(this.data.orderId),
        getPayments({ order_id: this.data.orderId, page_id: 1, page_size: 10 })
      ] as const)

      if (isSettledRejected(orderResult)) {
        throw orderResult.reason
      }

      const nextState: Record<string, unknown> = {
        order: this.formatDetail(orderResult.value),
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: ''
      }

      if (isSettledFulfilled(printJobsResult)) {
        nextState.printJobs = Array.isArray(printJobsResult.value.items)
          ? printJobsResult.value.items.map(buildPrintJobView)
          : []
        nextState.printJobsLoaded = true
        nextState.printJobsError = false
        nextState.printJobsErrorMessage = ''
      } else if (canPreservePrintJobs) {
        nextState.printJobsError = true
        nextState.printJobsErrorMessage = `${getErrorMessage(printJobsResult.reason, '打印状态同步失败')}，当前已保留上次结果`
      } else {
        nextState.printJobs = []
        nextState.printJobsLoaded = true
        nextState.printJobsError = true
        nextState.printJobsErrorMessage = getErrorMessage(printJobsResult.reason, '打印状态加载失败，请稍后重试')
      }

      if (isSettledFulfilled(paymentsResult)) {
        const paymentOrders = Array.isArray(paymentsResult.value.payment_orders)
          ? paymentsResult.value.payment_orders
          : []
        const activePayment = selectActivePayment(paymentOrders)

        if (activePayment) {
          try {
            const refundsResponse = await getPaymentRefunds(activePayment.id)
            const refundOrders = Array.isArray(refundsResponse.refund_orders)
              ? refundsResponse.refund_orders
              : []
            const refundReturnsResults = refundOrders.length
              ? await settleAll(refundOrders.map((refund) => getMerchantRefundReturns(refund.id)))
              : []
            const returnLoadFailed = refundReturnsResults.some(isSettledRejected)
            const refundViews = refundOrders.map((refund, index) => {
              const returnsResult = refundReturnsResults[index]
              const returns = returnsResult && isSettledFulfilled(returnsResult) && Array.isArray(returnsResult.value)
                ? returnsResult.value
                : []
              return buildRefundView(refund, returns)
            })
            nextState.refunds = refundViews
            nextState.payment = buildPaymentView(activePayment, refundOrders)
            nextState.refundsLoaded = true
            nextState.refundsError = false
            nextState.refundsErrorMessage = ''
            nextState.refundReturnsErrorMessage = returnLoadFailed
              ? '分账回退同步失败，退款记录已保留；系统会稍后继续同步'
              : ''
          } catch (refundErr) {
            const message = getErrorMessage(refundErr, '退款记录加载失败，请稍后重试')
            if (canPreserveRefunds && this.data.payment?.id === activePayment.id) {
              nextState.payment = this.data.payment
              nextState.refunds = this.data.refunds
              nextState.refundsLoaded = true
              nextState.refundsError = true
              nextState.refundsErrorMessage = `${message}，当前已保留上次结果`
              nextState.refundReturnsErrorMessage = ''
            } else {
              nextState.payment = buildPaymentView(activePayment, [])
              nextState.refunds = []
              nextState.refundsLoaded = true
              nextState.refundsError = true
              nextState.refundsErrorMessage = message
              nextState.refundReturnsErrorMessage = ''
            }
          }
        } else {
          nextState.payment = null
          nextState.refunds = []
          nextState.refundsLoaded = true
          nextState.refundsError = false
          nextState.refundsErrorMessage = ''
          nextState.refundReturnsErrorMessage = ''
        }
      } else if (canPreserveRefunds) {
        nextState.payment = this.data.payment
        nextState.refunds = this.data.refunds
        nextState.refundsLoaded = true
        nextState.refundsError = true
        nextState.refundsErrorMessage = `${getErrorMessage(paymentsResult.reason, '退款信息同步失败')}，当前已保留上次结果`
        nextState.refundReturnsErrorMessage = ''
      } else {
        nextState.payment = null
        nextState.refunds = []
        nextState.refundsLoaded = true
        nextState.refundsError = true
        nextState.refundsErrorMessage = getErrorMessage(paymentsResult.reason, '退款信息加载失败，请稍后重试')
        nextState.refundReturnsErrorMessage = ''
      }

      this.setData(nextState)
    } catch (err) {
      logger.error('Merchant load order detail failed', err)
      const message = getErrorMessage(err, '订单详情加载失败，请稍后重试')
      if (canPreserveDetail) {
        this.setData({ refreshErrorMessage: `${message}，当前已保留上次同步结果` })
      } else {
        this.setData({
          initialError: true,
          initialErrorMessage: message
        })
      }
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  formatDetail(order: OrderResponse): MerchantOrderDetailView {
    const timeline = this.buildTimeline(order)
    const scene = this.buildSceneInfo(order)
    const pickupCodeDisplay = this.formatPickupCodeDisplay(order)

    return {
      ...order,
      status_label: OrderManagementAdapter.formatOrderStatus(order.status),
      status_color: OrderManagementAdapter.getStatusColor(order.status),
      status_icon: getMerchantOrderStatusIcon(order.status),
      status_desc: getMerchantOrderStatusDesc(order),
      order_type_label: OrderManagementAdapter.formatOrderType(order.order_type),
      payment_method_label: OrderManagementAdapter.formatPaymentMethod(order.payment_method || 'wechat'),
      created_at_fmt: dayjs(order.created_at).format('YYYY-MM-DD HH:mm'),
      paid_at_fmt: this.formatTime(order.paid_at),
      completed_at_fmt: this.formatTime(order.completed_at),
      status_hint_label: order.status_hint || getMerchantOrderFallbackStatusHint(order),
      step_current: timeline.current,
      timeline_steps: timeline.steps,
      location_label: scene.label,
      location_primary: scene.primary,
      location_secondary: scene.secondary,
      pickup_code_display: pickupCodeDisplay,
      contact_name: order.delivery_contact_name || '',
      contact_phone: order.delivery_contact_phone || '',
      fee_breakdown_view: buildMerchantOrderFeeBreakdownView(order),
      can_accept: OrderManagementAdapter.canAcceptOrder(order),
      can_reject: OrderManagementAdapter.canRejectOrder(order),
      can_mark_ready: OrderManagementAdapter.canMarkReady(order),
      can_complete: OrderManagementAdapter.canCompleteOrder(order),
      can_manual_print: canMerchantOrderManualPrint(order)
    }
  },

  buildSceneInfo(order: OrderResponse) {
    const pickupCodeDisplay = this.formatPickupCodeDisplay(order)

    if (order.order_type === 'takeout') {
      return {
        label: '代取地址',
        primary: order.delivery_address || '待同步代取地址',
        secondary: [order.delivery_contact_name, order.delivery_contact_phone].filter(Boolean).join(' ')
      }
    }

    if (order.order_type === 'dine_in') {
      return {
        label: '就餐位置',
        primary: order.table_id ? `${order.table_id} 号桌` : '堂食就餐',
        secondary: `取餐码 ${pickupCodeDisplay}`
      }
    }

    if (order.order_type === 'takeaway') {
      return {
        label: '取餐方式',
        primary: `取餐码 ${pickupCodeDisplay}`,
        secondary: '顾客到店后核销'
      }
    }

    return {
      label: '预订信息',
      primary: order.reservation_id ? `预订 #${order.reservation_id}` : '预订点菜',
      secondary: order.table_id ? `${order.table_id} 号桌` : '到店后履约'
    }
  },

  formatPickupCodeDisplay(order: Pick<OrderResponse, 'pickup_code' | 'pickup_code_masked'>) {
    const pickupCode = String(order.pickup_code || '').trim()
    if (/^\d{4}$/.test(pickupCode)) {
      return pickupCode
    }
    return String(order.pickup_code_masked || '').trim() || '----'
  },

  buildTimeline(order: OrderResponse) {
    return buildMerchantOrderTimeline(order)
  },

  formatTime(value?: string, pattern = 'HH:mm') {
    return value ? dayjs(value).format(pattern) : '--'
  },

  formatTimelineValue(value?: string, fallback = '--') {
    return value ? this.formatTime(value) : fallback
  },

  async onAccept() {
    await this.performAction(() => MerchantOrderManagementService.acceptOrder(this.data.orderId), '接单成功')
  },

  async onReject() {
    try {
      const result = await wx.showActionSheet({
        itemList: [...MERCHANT_REJECT_REASON_OPTIONS],
        alertText: '请选择拒单原因，系统将按后端契约发起退款'
      })
      const reason = MERCHANT_REJECT_REASON_OPTIONS[result.tapIndex]
      if (!reason) return

      await this.performAction(async () => {
        const response = await MerchantOrderManagementService.rejectOrder(this.data.orderId, { reason })
        return {
          order: response.order,
          message: response.refund_submission?.message || '已拒单，退款状态请在订单退款记录中查看'
        }
      }, '已拒单，退款状态请在订单退款记录中查看')
    } catch (error) {
      const err = error as { errMsg?: string }
      if (err?.errMsg?.includes('cancel')) return
      logger.error('Select reject reason failed', error)
      wx.showToast({ title: '选择拒单原因失败', icon: 'none' })
    }
  },

  async onMarkReady() {
    await this.performAction(() => MerchantOrderManagementService.markOrderReady(this.data.orderId), '制作完成')
  },

  async onComplete() {
    await this.performAction(() => MerchantOrderManagementService.completeOrder(this.data.orderId), '订单已核销')
  },

  async onRetryPrintJob(e: WechatMiniprogram.TouchEvent) {
    const { id, printerName } = e.currentTarget.dataset as { id?: number, printerName?: string }
    if (!id || this.data.retryingPrintJobId) return

    wx.showModal({
      title: '重试打印',
      content: `重新向打印机「${printerName || '未命名设备'}」下发该订单的打印任务？`,
      confirmText: '立即重试',
      cancelText: '取消',
      success: async (res) => {
        if (!res.confirm || this.data.retryingPrintJobId) return

        this.setData({ retryingPrintJobId: id })
        try {
          await MerchantOrderManagementService.retryOrderPrintJob(this.data.orderId, id)
          await this.loadDetail(false)
        } catch (err) {
          logger.error('Retry merchant print job failed', err)
          wx.showToast({ title: getErrorMessage(err, '重试打印失败，请稍后重试'), icon: 'none' })
        } finally {
          this.setData({ retryingPrintJobId: 0 })
        }
      }
    })
  },

  async onQueryPrintJobStatus(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id || this.data.queryingPrintJobStatusId) return

    const index = this.data.printJobs.findIndex((item) => item.id === id)
    if (index < 0) return

    this.setData({ queryingPrintJobStatusId: id })
    try {
      const status = await MerchantOrderManagementService.getOrderPrintJobStatus(this.data.orderId, id)
      this.setData({
        [`printJobs[${index}].cloud_status_label`]: getCloudStatusLabel(status),
        [`printJobs[${index}].cloud_status_checked_at`]: dayjs(status.checked_at).format('MM-DD HH:mm'),
        [`printJobs[${index}].can_query_cloud_status`]: status.cloud_query_available
      })
    } catch (err) {
      logger.error('Query merchant print job status failed', err)
      wx.showToast({ title: getErrorMessage(err, '查询云端状态失败，请稍后重试'), icon: 'none' })
    } finally {
      this.setData({ queryingPrintJobStatusId: 0 })
    }
  },

  onManualPrint() {
    if (this.data.manualPrinting || !this.data.order?.can_manual_print) return

    wx.showModal({
      title: '手动补打',
      content: '将为当前订单创建一次新的打印任务。仅当门店已开启手动打印模式时，此操作才会生效。',
      confirmText: '立即补打',
      cancelText: '取消',
      success: async (res) => {
        if (!res.confirm || this.data.manualPrinting) return

        this.setData({ manualPrinting: true })
        try {
          await MerchantOrderManagementService.printOrder(this.data.orderId)
          wx.showToast({ title: '已创建补打任务', icon: 'success' })
          await this.loadDetail(false)
        } catch (err) {
          logger.error('Manual print order failed', err)
          wx.showToast({ title: getErrorMessage(err, '创建补打任务失败，请稍后重试'), icon: 'none' })
        } finally {
          this.setData({ manualPrinting: false })
        }
      }
    })
  },

  onOpenRefundPopup() {
    const payment = this.data.payment
    if (!payment || !payment.can_create_refund || this.data.refundSubmitting) return

    const refundType = payment.allow_full_refund ? 'full' : 'partial'
    this.setData({
      refundPopupVisible: true,
      refundAvailableTypes: payment.allow_full_refund ? ['full', 'partial'] : ['partial'],
      refundForm: {
        refund_type: refundType,
        refund_amount: (payment.remaining_refund_amount / 100).toFixed(2),
        refund_reason: ''
      },
      refundIdempotencyKey: buildRefundIdempotencyKey(this.data.orderId, payment.id),
      refundNoticeMessage: ''
    })
  },

  onCloseRefundPopup() {
    if (this.data.refundSubmitting) return
    this.setData({
      refundPopupVisible: false,
      refundForm: createDefaultRefundForm(),
      refundIdempotencyKey: ''
    })
  },

  onRefundPopupVisibleChange(e: WechatMiniprogram.CustomEvent<{ visible: boolean }>) {
    if (!e.detail.visible) {
      this.onCloseRefundPopup()
    }
  },

  onRefundTypeChange(e: WechatMiniprogram.CustomEvent<{ value?: 'full' | 'partial' }>) {
    const value = e.detail?.value as 'full' | 'partial' | undefined
    if (!value) return

    this.setData({
      'refundForm.refund_type': value,
      refundIdempotencyKey: buildRefundIdempotencyKey(this.data.orderId, this.data.payment?.id || 0)
    })
  },

  onRefundFieldChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const { field } = e.currentTarget.dataset as { field?: keyof RefundFormData }
    if (!field) return
    this.setData({
      [`refundForm.${field}`]: e.detail.value || '',
      refundIdempotencyKey: buildRefundIdempotencyKey(this.data.orderId, this.data.payment?.id || 0)
    })
  },

  async onSubmitRefund() {
    const payment = this.data.payment
    if (!payment || this.data.refundSubmitting || !payment.can_create_refund) return

    const refundType = this.data.refundForm.refund_type
    const refundReason = this.data.refundForm.refund_reason.trim()
    let refundAmount = payment.remaining_refund_amount

    if (refundType === 'partial') {
      const amountText = this.data.refundForm.refund_amount.trim()
      const amountValue = Number(amountText)
      if (!amountText || !Number.isFinite(amountValue) || amountValue <= 0) {
        wx.showToast({ title: '请输入正确的退款金额', icon: 'none' })
        return
      }
      refundAmount = Math.round(amountValue * 100)
      if (refundAmount <= 0) {
        wx.showToast({ title: '退款金额需大于 0', icon: 'none' })
        return
      }
      if (refundAmount > payment.remaining_refund_amount) {
        wx.showToast({ title: '退款金额不能超过剩余可退金额', icon: 'none' })
        return
      }
    }

    const idempotencyKey = this.data.refundIdempotencyKey || buildRefundIdempotencyKey(this.data.orderId, payment.id)
    this.setData({ refundSubmitting: true })
    wx.showLoading({ title: '提交中...' })
    let createdRefundId = 0
    try {
      const refund = await createRefund({
        payment_order_id: payment.id,
        refund_type: refundType,
        refund_amount: refundAmount,
        refund_reason: refundReason || undefined
      }, {
        idempotencyKey
      })
      createdRefundId = refund.id

      this.setData({
        refundPopupVisible: false,
        refundForm: createDefaultRefundForm(),
        refundIdempotencyKey: '',
        refundNoticeMessage: '退款申请已提交，正在确认退款结果。'
      })
      wx.showLoading({ title: '确认退款结果...' })
      const waitResult = await waitForRefundTerminalResult(createdRefundId)
      await this.loadDetail(false)
      const resultNotice = waitResult.terminal
        ? `退款结果已更新：${buildRefundView(waitResult.refund, []).status_label}`
        : '退款结果还在同步中，请稍后查看退款详情。'
      this.setData({ refundNoticeMessage: resultNotice })
    } catch (err) {
      if (createdRefundId) {
        logger.warn('Wait merchant refund terminal failed after creation', err, 'merchant-order-detail.onSubmitRefund')
        await this.loadDetail(false)
        this.setData({ refundNoticeMessage: '退款申请已提交，结果还在同步中，系统会稍后继续同步退款记录。' })
      } else {
        logger.error('Create merchant refund failed', err)
        wx.showToast({ title: getErrorMessage(err, '发起退款失败，请稍后重试'), icon: 'none' })
      }
    } finally {
      wx.hideLoading()
      this.setData({ refundSubmitting: false })
    }
  },

  async performAction(request: () => Promise<unknown>, successText: string) {
    this.setData({ submitting: true })
    try {
      const response = await request() as OrderResponse | { order: OrderResponse, message?: string }
      const updatedOrder = 'order' in response ? response.order : response
      this.setData({
        order: this.formatDetail(updatedOrder),
        refreshErrorMessage: ''
      })
      await this.loadDetail(false)
      const message = 'order' in response ? response.message : successText
      if (message) {
        wx.showToast({ title: message, icon: 'none', duration: 3000 })
      }

      const pages = getCurrentPages()
      const listPage = pages[pages.length - 2] as { loadOrders?: (reset?: boolean, showLoading?: boolean) => void } | undefined
      if (listPage?.loadOrders) {
        listPage.loadOrders(true, false)
      }
    } catch (err) {
      logger.error('Action failed', err)
      wx.showToast({ title: getErrorMessage(err, '操作失败，请稍后重试'), icon: 'none' })
    } finally {
      this.setData({ submitting: false })
    }
  },

  onCallCustomer() {
    if (this.data.order?.contact_phone) {
      wx.makePhoneCall({ phoneNumber: this.data.order.contact_phone })
    }
  }
})
