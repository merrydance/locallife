import { getStableBarHeights } from '../../../../utils/responsive'
import {
  MerchantOrderManagementService,
  MerchantOrderPrintJobResponse,
  MerchantOrderPrintJobStatusResponse,
  OrderResponse,
  OrderManagementAdapter,
  MERCHANT_REJECT_REASON_OPTIONS
} from '../../../../api/order-management'
import { createRefund, getPaymentRefunds, getPayments, getRefundReturns, PaymentOrder, ProfitSharingReturn, RefundOrder } from '../../../../api/payment'
import { logger } from '../../../../utils/logger'
import { settleAll } from '../../../../utils/promise'
import dayjs from 'dayjs'
import { getErrorUserMessage } from '../../../../utils/user-facing'

interface MerchantOrderDetailOptions {
  id?: string
}

interface MerchantOrderDetailView extends OrderResponse {
  status_label: string
  status_color: string
  status_icon: string
  status_desc: string
  order_type_label: string
  payment_method_label: string
  created_at_fmt: string
  paid_at_fmt: string
  completed_at_fmt: string
  status_hint_label: string
  step_current: number
  timeline_steps: Array<{ title: string, content: string }>
  location_label: string
  location_primary: string
  location_secondary: string
  contact_name: string
  contact_phone: string
  can_accept: boolean
  can_reject: boolean
  can_mark_ready: boolean
  can_complete: boolean
  can_manual_print: boolean
}

type PrintJobStatusTheme = 'success' | 'warning' | 'danger' | 'default'

interface MerchantOrderPrintJobView extends MerchantOrderPrintJobResponse {
  status_label: string
  status_theme: PrintJobStatusTheme
  summary: string
  created_at_fmt: string
  printed_at_fmt: string
  can_retry: boolean
  error_message_label: string
  vendor_order_id_label: string
  cloud_status_label: string
  cloud_status_checked_at: string
  can_query_cloud_status: boolean
}

type RefundStatusTheme = 'success' | 'warning' | 'danger' | 'default'
type PaymentStatusTheme = 'success' | 'warning' | 'default'

interface MerchantOrderPaymentView extends PaymentOrder {
  status_label: string
  status_theme: PaymentStatusTheme
  amount_text: string
  created_at_fmt: string
  remaining_refund_amount: number
  remaining_refund_text: string
  refund_count: number
  can_create_refund: boolean
  allow_full_refund: boolean
}

interface MerchantOrderRefundView extends RefundOrder {
  status_label: string
  status_theme: RefundStatusTheme
  refund_type_label: string
  refund_amount_text: string
  created_at_fmt: string
  refunded_at_fmt: string
  refund_reason_label: string
  return_count: number
  returns: MerchantRefundReturnView[]
}

interface MerchantRefundReturnView extends ProfitSharingReturn {
  amount_text: string
  status_label: string
  status_theme: RefundStatusTheme
  created_at_fmt: string
  finished_at_fmt: string
  fail_reason_label: string
}

interface RefundFormData {
  refund_type: 'full' | 'partial'
  refund_amount: string
  refund_reason: string
}

function createDefaultRefundForm(): RefundFormData {
  return {
    refund_type: 'full',
    refund_amount: '',
    refund_reason: ''
  }
}

function formatMoney(amount: number) {
  return `¥${(amount / 100).toFixed(2)}`
}

function getPaymentStatusLabel(status: string) {
  const map: Record<string, string> = {
    pending: '待支付',
    paid: '已支付',
    refunded: '已退款',
    closed: '已关闭',
    failed: '支付失败'
  }
  return map[status] || '状态同步中'
}

function getPaymentStatusTheme(status: string): PaymentStatusTheme {
  if (status === 'paid') return 'success'
  if (status === 'refunded') return 'warning'
  return 'default'
}

function getRefundStatusLabel(status: string) {
  const map: Record<string, string> = {
    pending: '退款申请中',
    processing: '退款处理中',
    success: '退款成功',
    failed: '退款失败',
    closed: '退款关闭'
  }
  return map[status] || '状态同步中'
}

function getRefundStatusTheme(status: string): RefundStatusTheme {
  if (status === 'success') return 'success'
  if (status === 'pending' || status === 'processing') return 'warning'
  if (status === 'failed') return 'danger'
  return 'default'
}

function getReturnStatusLabel(status: string) {
  const map: Record<string, string> = {
    pending: '待回退',
    processing: '回退处理中',
    success: '回退成功',
    failed: '回退失败',
    closed: '回退关闭'
  }
  return map[status] || '状态同步中'
}

function getReturnStatusTheme(status: string): RefundStatusTheme {
  if (status === 'success') return 'success'
  if (status === 'pending' || status === 'processing') return 'warning'
  if (status === 'failed') return 'danger'
  return 'default'
}

function getRefundReservedAmount(refunds: RefundOrder[]) {
  return refunds
    .filter((refund) => !['failed', 'closed'].includes(refund.status))
    .reduce((sum, refund) => sum + refund.refund_amount, 0)
}

function buildRefundReturnView(item: ProfitSharingReturn): MerchantRefundReturnView {
  return {
    ...item,
    amount_text: formatMoney(item.amount),
    status_label: getReturnStatusLabel(item.status),
    status_theme: getReturnStatusTheme(item.status),
    created_at_fmt: dayjs(item.created_at).format('YYYY-MM-DD HH:mm'),
    finished_at_fmt: item.finished_at ? dayjs(item.finished_at).format('YYYY-MM-DD HH:mm') : '--',
    fail_reason_label: item.fail_reason || ''
  }
}

function buildRefundView(refund: RefundOrder, returns: ProfitSharingReturn[] = []): MerchantOrderRefundView {
  const normalizedReturns = Array.isArray(returns) ? returns.map(buildRefundReturnView) : []
  return {
    ...refund,
    status_label: getRefundStatusLabel(refund.status),
    status_theme: getRefundStatusTheme(refund.status),
    refund_type_label: refund.refund_type === 'full' ? '全额退款' : '部分退款',
    refund_amount_text: formatMoney(refund.refund_amount),
    created_at_fmt: dayjs(refund.created_at).format('YYYY-MM-DD HH:mm'),
    refunded_at_fmt: refund.refunded_at ? dayjs(refund.refunded_at).format('YYYY-MM-DD HH:mm') : '--',
    refund_reason_label: refund.refund_reason || '',
    return_count: normalizedReturns.length,
    returns: normalizedReturns
  }
}

function buildPaymentView(payment: PaymentOrder, refunds: RefundOrder[]): MerchantOrderPaymentView {
  const reservedAmount = getRefundReservedAmount(refunds)
  const remainingRefundAmount = payment.status === 'refunded'
    ? 0
    : Math.max(payment.amount - reservedAmount, 0)

  return {
    ...payment,
    status_label: getPaymentStatusLabel(payment.status),
    status_theme: getPaymentStatusTheme(payment.status),
    amount_text: formatMoney(payment.amount),
    created_at_fmt: dayjs(payment.created_at).format('YYYY-MM-DD HH:mm'),
    remaining_refund_amount: remainingRefundAmount,
    remaining_refund_text: formatMoney(remainingRefundAmount),
    refund_count: refunds.length,
    can_create_refund: ['paid', 'refunded'].includes(payment.status) && remainingRefundAmount > 0,
    allow_full_refund: remainingRefundAmount === payment.amount
  }
}

function selectActivePayment(payments: PaymentOrder[]): PaymentOrder | null {
  if (!Array.isArray(payments) || !payments.length) return null

  const sorted = [...payments].sort((left, right) => {
    const leftTime = dayjs(left.created_at).valueOf()
    const rightTime = dayjs(right.created_at).valueOf()
    return rightTime - leftTime
  })

  return sorted.find((payment) => ['paid', 'refunded'].includes(payment.status))
    || sorted.find((payment) => payment.status === 'pending')
    || sorted[0]
    || null
}

function getCloudStatusLabel(result?: MerchantOrderPrintJobStatusResponse) {
  if (!result) return ''
  if (!result.cloud_query_available) {
    return '当前打印机暂不支持云端状态查询'
  }
  if (typeof result.cloud_printed === 'boolean') {
    return result.cloud_printed ? '云端回执显示已打印' : '云端回执显示未打印'
  }
  return '云端状态暂未返回，请稍后再试'
}

function formatPrintJobStatus(status: string): string {
  const statusMap: Record<string, string> = {
    success: '已打印',
    pending: '待回执',
    processing: '处理中',
    failed: '打印失败'
  }
  return statusMap[status] || '状态同步中'
}

function getPrintJobStatusTheme(status: string): PrintJobStatusTheme {
  if (status === 'success') return 'success'
  if (status === 'failed') return 'danger'
  if (status === 'pending' || status === 'processing') return 'warning'
  return 'default'
}

function getPrintJobSummary(job: MerchantOrderPrintJobResponse): string {
  if (job.status === 'success') {
    return '打印任务已完成，可联系门店确认纸单是否正常出单。'
  }
  if (job.status === 'failed') {
    if (job.error_message) {
      return '打印任务未成功下发，请先根据失败原因排查后再重试。'
    }
    return '打印任务未成功下发，建议重试以恢复门店打印。'
  }
  if (job.status === 'pending' || job.status === 'processing') {
    return '打印任务已提交，正在等待云打印平台返回结果。'
  }
  return '打印任务状态正在同步，请稍后刷新查看。'
}

function buildPrintJobView(job: MerchantOrderPrintJobResponse): MerchantOrderPrintJobView {
  return {
    ...job,
    status_label: formatPrintJobStatus(job.status),
    status_theme: getPrintJobStatusTheme(job.status),
    summary: getPrintJobSummary(job),
    created_at_fmt: dayjs(job.created_at).format('YYYY-MM-DD HH:mm'),
    printed_at_fmt: job.printed_at ? dayjs(job.printed_at).format('YYYY-MM-DD HH:mm') : '--',
    can_retry: job.status === 'failed',
    error_message_label: job.error_message || '',
    vendor_order_id_label: job.vendor_order_id || '',
    cloud_status_label: '',
    cloud_status_checked_at: '',
    can_query_cloud_status: !!job.vendor_order_id && job.status !== 'success'
  }
}

const getErrorMessage = getErrorUserMessage

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
    refundNoticeMessage: '',
    refunds: [] as MerchantOrderRefundView[],
    refundPopupVisible: false,
    refundSubmitting: false,
    refundAvailableTypes: ['full', 'partial'] as Array<'full' | 'partial'>,
    refundForm: createDefaultRefundForm(),
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
        initialErrorMessage: '缺少订单编号，无法查看详情'
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

      if (orderResult.status === 'rejected') {
        throw orderResult.reason
      }

      const nextState: Record<string, unknown> = {
        order: this.formatDetail(orderResult.value),
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: ''
      }

      if (printJobsResult.status === 'fulfilled') {
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

      if (paymentsResult.status === 'fulfilled') {
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
              ? await settleAll(refundOrders.map((refund) => getRefundReturns(refund.id)))
              : []
            const refundViews = refundOrders.map((refund, index) => {
              const returnsResult = refundReturnsResults[index]
              const returns = returnsResult && returnsResult.status === 'fulfilled' && Array.isArray(returnsResult.value)
                ? returnsResult.value
                : []
              return buildRefundView(refund, returns)
            })
            nextState.refunds = refundViews
            nextState.payment = buildPaymentView(activePayment, refundOrders)
            nextState.refundsLoaded = true
            nextState.refundsError = false
            nextState.refundsErrorMessage = ''
          } catch (refundErr) {
            const message = getErrorMessage(refundErr, '退款记录加载失败，请稍后重试')
            if (canPreserveRefunds && this.data.payment?.id === activePayment.id) {
              nextState.payment = this.data.payment
              nextState.refunds = this.data.refunds
              nextState.refundsLoaded = true
              nextState.refundsError = true
              nextState.refundsErrorMessage = `${message}，当前已保留上次结果`
            } else {
              nextState.payment = buildPaymentView(activePayment, [])
              nextState.refunds = []
              nextState.refundsLoaded = true
              nextState.refundsError = true
              nextState.refundsErrorMessage = message
            }
          }
        } else {
          nextState.payment = null
          nextState.refunds = []
          nextState.refundsLoaded = true
          nextState.refundsError = false
          nextState.refundsErrorMessage = ''
        }
      } else if (canPreserveRefunds) {
        nextState.payment = this.data.payment
        nextState.refunds = this.data.refunds
        nextState.refundsLoaded = true
        nextState.refundsError = true
        nextState.refundsErrorMessage = `${getErrorMessage(paymentsResult.reason, '退款信息同步失败')}，当前已保留上次结果`
      } else {
        nextState.payment = null
        nextState.refunds = []
        nextState.refundsLoaded = true
        nextState.refundsError = true
        nextState.refundsErrorMessage = getErrorMessage(paymentsResult.reason, '退款信息加载失败，请稍后重试')
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

    return {
      ...order,
      status_label: OrderManagementAdapter.formatOrderStatus(order.status),
      status_color: OrderManagementAdapter.getStatusColor(order.status),
      status_icon: this.getStatusIcon(order.status),
      status_desc: this.getStatusDesc(order),
      order_type_label: OrderManagementAdapter.formatOrderType(order.order_type),
      payment_method_label: OrderManagementAdapter.formatPaymentMethod(order.payment_method || 'wechat'),
      created_at_fmt: dayjs(order.created_at).format('YYYY-MM-DD HH:mm'),
      paid_at_fmt: this.formatTime(order.paid_at),
      completed_at_fmt: this.formatTime(order.completed_at),
      status_hint_label: order.status_hint || this.getFallbackStatusHint(order),
      step_current: timeline.current,
      timeline_steps: timeline.steps,
      location_label: scene.label,
      location_primary: scene.primary,
      location_secondary: scene.secondary,
      contact_name: order.delivery_contact_name || '',
      contact_phone: order.delivery_contact_phone || '',
      can_accept: OrderManagementAdapter.canAcceptOrder(order),
      can_reject: OrderManagementAdapter.canRejectOrder(order),
      can_mark_ready: OrderManagementAdapter.canMarkReady(order),
      can_complete: OrderManagementAdapter.canCompleteOrder(order),
      can_manual_print: !['pending', 'cancelled'].includes(order.status)
    }
  },

  buildSceneInfo(order: OrderResponse) {
    if (order.order_type === 'takeout') {
      return {
        label: '配送地址',
        primary: order.delivery_address || '待同步配送地址',
        secondary: [order.delivery_contact_name, order.delivery_contact_phone].filter(Boolean).join(' ')
      }
    }

    if (order.order_type === 'dine_in') {
      return {
        label: '就餐位置',
        primary: order.table_id ? `${order.table_id} 号桌` : '堂食就餐',
        secondary: order.reservation_id ? `预订 #${order.reservation_id}` : '到店就餐'
      }
    }

    if (order.order_type === 'takeaway') {
      return {
        label: '取餐方式',
        primary: order.pickup_code_masked ? `取餐码 ${order.pickup_code_masked}` : '到店自取',
        secondary: order.pickup_code ? `原始取餐码 ${order.pickup_code}` : '顾客到店后核销'
      }
    }

    return {
      label: '预订信息',
      primary: order.reservation_id ? `预订 #${order.reservation_id}` : '预订点菜',
      secondary: order.table_id ? `${order.table_id} 号桌` : '到店后履约'
    }
  },

  buildTimeline(order: OrderResponse) {
    if (order.order_type === 'takeout') {
      const steps = [
        { title: '订单提交', content: this.formatTime(order.created_at, 'YYYY-MM-DD HH:mm') },
        { title: '已支付', content: this.formatTime(order.paid_at) },
        { title: '商户处理', content: this.formatTimelineValue(order.prep_start_at, order.status === 'paid' ? '待商户接单' : '--') },
        { title: '出餐完成', content: this.formatTimelineValue(order.ready_at, order.status === 'preparing' ? '制作中' : '--') },
        { title: '骑手接单', content: this.formatTimelineValue(order.courier_accept_at, order.status === 'ready' ? '待分配骑手' : '--') },
        { title: '骑手取餐', content: this.formatTimelineValue(order.picked_at, order.status === 'courier_accepted' ? '骑手前往取餐' : '--') },
        { title: '送达完成', content: this.formatTimelineValue(order.user_delivered_at || order.auto_user_delivered_at || order.rider_delivered_at || order.completed_at, order.status === 'delivering' ? '配送途中' : '--') }
      ]

      const currentMap: Record<string, number> = {
        pending: 0,
        paid: 1,
        preparing: 2,
        ready: 3,
        courier_accepted: 4,
        picked: 5,
        delivering: 5,
        rider_delivered: 6,
        user_delivered: 6,
        completed: 6,
        cancelled: 0
      }

      return { steps, current: currentMap[order.status] ?? 0 }
    }

    const steps = [
      { title: '订单提交', content: this.formatTime(order.created_at, 'YYYY-MM-DD HH:mm') },
      { title: '已支付', content: this.formatTime(order.paid_at) },
      { title: '商户处理', content: this.formatTimelineValue(order.prep_start_at, order.status === 'paid' ? '待商户接单' : '--') },
      { title: order.order_type === 'dine_in' ? '备餐完成' : '待取餐', content: this.formatTimelineValue(order.ready_at, order.status === 'preparing' ? '制作中' : '--') },
      { title: '履约完成', content: this.formatTimelineValue(order.completed_at, order.status === 'ready' ? '待商户确认' : '--') }
    ]

    const currentMap: Record<string, number> = {
      pending: 0,
      paid: 1,
      preparing: 2,
      ready: 3,
      completed: 4,
      cancelled: 0
    }

    return { steps, current: currentMap[order.status] ?? 0 }
  },

  formatTime(value?: string, pattern = 'HH:mm') {
    return value ? dayjs(value).format(pattern) : '--'
  },

  formatTimelineValue(value?: string, fallback = '--') {
    return value ? this.formatTime(value) : fallback
  },

  getStatusIcon(status: string) {
    const icons: Record<string, string> = {
      paid: 'notification',
      preparing: 'loading',
      ready: 'check-circle',
      courier_accepted: 'assignment-user',
      picked: 'shop',
      delivering: 'undertake-deliver',
      rider_delivered: 'check-circle-filled',
      user_delivered: 'check-circle-filled',
      completed: 'check-circle',
      cancelled: 'close-circle'
    }
    return icons[status] || 'info-circle'
  },

  getStatusDesc(order: OrderResponse) {
    const descs: Record<string, string> = {
      paid: '顾客已付款，请尽快接单或拒单处理',
      preparing: '订单已进入制作阶段',
      ready: order.order_type === 'takeout' ? '餐品已备妥，等待骑手接力' : '餐品已备妥，等待顾客取餐或到店核销',
      courier_accepted: '骑手已接单，正在前往门店',
      picked: '骑手已取餐，订单即将配送',
      delivering: '骑手配送中，请留意异常与超时',
      rider_delivered: '骑手已送达，等待顾客确认',
      user_delivered: '顾客已确认收货，订单即将完结',
      completed: '订单已成功履约完成',
      cancelled: order.cancel_reason || '订单已被系统或商户取消'
    }
    return descs[order.status] || ''
  },

  getFallbackStatusHint(order: OrderResponse) {
    if (order.cancel_reason) {
      return `取消原因：${order.cancel_reason}`
    }
    if (order.overtime) {
      return '当前订单已超时，请优先关注'
    }
    return ''
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

      await this.performAction(
        () => MerchantOrderManagementService.rejectOrder(this.data.orderId, { reason }),
        '已拒单并发起退款'
      )
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
      refundNoticeMessage: ''
    })
  },

  onCloseRefundPopup() {
    if (this.data.refundSubmitting) return
    this.setData({
      refundPopupVisible: false,
      refundForm: createDefaultRefundForm()
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
      'refundForm.refund_type': value
    })
  },

  onRefundFieldChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const { field } = e.currentTarget.dataset as { field?: keyof RefundFormData }
    if (!field) return
    this.setData({
      [`refundForm.${field}`]: e.detail.value || ''
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

    this.setData({ refundSubmitting: true })
    wx.showLoading({ title: '提交中...' })
    try {
      await createRefund({
        payment_order_id: payment.id,
        refund_type: refundType,
        refund_amount: refundAmount,
        refund_reason: refundReason || undefined
      })

      this.setData({
        refundPopupVisible: false,
        refundForm: createDefaultRefundForm(),
        refundNoticeMessage: '退款申请已提交，后续会按微信侧处理进度自动更新。'
      })
      await this.loadDetail(false)
    } catch (err) {
      logger.error('Create merchant refund failed', err)
      wx.showToast({ title: getErrorMessage(err, '发起退款失败，请稍后重试'), icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ refundSubmitting: false })
    }
  },

  async performAction(request: () => Promise<unknown>, _successText: string) {
    this.setData({ submitting: true })
    try {
      const updatedOrder = await request() as OrderResponse
      this.setData({
        order: this.formatDetail(updatedOrder),
        refreshErrorMessage: ''
      })
      await this.loadDetail(false)

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
