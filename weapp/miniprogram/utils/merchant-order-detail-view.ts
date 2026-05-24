import dayjs from 'dayjs'
import {
  type MerchantOrderPrintJobResponse,
  type MerchantOrderPrintJobStatusResponse,
  type OrderResponse
} from '../api/order-management'
import { type PaymentOrder, type ProfitSharingReturn, type RefundOrder } from '../api/payment'

export interface MerchantOrderDetailOptions {
  id?: string
}

export interface MerchantOrderDetailView extends OrderResponse {
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
  pickup_code_display: string
  contact_name: string
  contact_phone: string
  fee_breakdown_view: MerchantOrderFeeBreakdownView
  can_accept: boolean
  can_reject: boolean
  can_mark_ready: boolean
  can_complete: boolean
  can_manual_print: boolean
}

type PrintJobStatusTheme = 'success' | 'warning' | 'danger' | 'default'
type RefundStatusTheme = 'success' | 'warning' | 'danger' | 'default'
type PaymentStatusTheme = 'success' | 'warning' | 'default'

export interface MerchantOrderPrintJobView extends MerchantOrderPrintJobResponse {
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

export interface MerchantOrderPaymentView extends PaymentOrder {
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

export interface MerchantRefundReturnView extends ProfitSharingReturn {
  amount_text: string
  status_label: string
  status_theme: RefundStatusTheme
  created_at_fmt: string
  finished_at_fmt: string
  fail_reason_label: string
}

export interface MerchantOrderRefundView extends RefundOrder {
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

export interface RefundFormData {
  refund_type: 'full' | 'partial'
  refund_amount: string
  refund_reason: string
}

export interface MerchantOrderFeeBreakdownRow {
  key: string
  label: string
  value: number
  value_text: string
  tone: 'default' | 'discount' | 'total' | 'income' | 'fee'
  visible: boolean
}

export interface MerchantOrderFeeSettlementGroup {
  key: string
  title: string
  total_text: string
  tone: 'merchant' | 'rider'
  visible: boolean
  rows: MerchantOrderFeeBreakdownRow[]
}

export interface MerchantOrderFeeBreakdownView {
  available: boolean
  unavailable_text: string
  customer_payable_text: string
  merchant_receivable_text: string
  summary_rows: MerchantOrderFeeBreakdownRow[]
  settlement_groups: MerchantOrderFeeSettlementGroup[]
}

export function createDefaultRefundForm(): RefundFormData {
  return { refund_type: 'full', refund_amount: '', refund_reason: '' }
}

function formatMoney(amount: number) {
  return `¥${(amount / 100).toFixed(2)}`
}

function formatSignedMoney(amount: number) {
  if (amount < 0) {
    return `-¥${(Math.abs(amount) / 100).toFixed(2)}`
  }
  return formatMoney(amount)
}

function createFeeBreakdownRow(
  key: string,
  label: string,
  value: number,
  tone: MerchantOrderFeeBreakdownRow['tone'],
  alwaysVisible = false
): MerchantOrderFeeBreakdownRow {
  return {
    key,
    label,
    value,
    value_text: formatSignedMoney(value),
    tone,
    visible: alwaysVisible || value !== 0
  }
}

export function buildMerchantOrderFeeBreakdownView(order: OrderResponse): MerchantOrderFeeBreakdownView {
  const breakdown = order.fee_breakdown
  if (!breakdown) {
    const customerPayable = formatMoney(order.total_amount || 0)
    return {
      available: false,
      unavailable_text: '订单费用明细暂不可用，请稍后重试',
      customer_payable_text: customerPayable,
      merchant_receivable_text: '金额同步中',
      summary_rows: [],
      settlement_groups: []
    }
  }

  const merchantRows = [
    createFeeBreakdownRow('merchant_food_payable_amount', '餐费应分', breakdown.food_payable_amount, 'default', true),
    createFeeBreakdownRow('merchant_platform_service_fee_amount', '平台服务费', -breakdown.platform_service_fee_amount, 'fee', true),
    createFeeBreakdownRow('merchant_payment_channel_fee_amount', '支付通道费', -breakdown.payment_channel_fee_amount, 'fee', true),
    createFeeBreakdownRow('merchant_receivable_amount', '商户实收', breakdown.merchant_receivable_amount, 'income', true)
  ]
  const riderGrossAmount = breakdown.rider_gross_amount || 0
  const riderPaymentFeeAmount = breakdown.rider_payment_fee_amount || 0
  const riderNetEarningsAmount = breakdown.rider_net_earnings_amount || 0
  const riderRows = [
    createFeeBreakdownRow('rider_gross_amount', '骑手应分', riderGrossAmount, 'default', true),
    createFeeBreakdownRow('rider_payment_fee_amount', '骑手通道费', -riderPaymentFeeAmount, 'fee', true),
    createFeeBreakdownRow('rider_net_earnings_amount', '骑手收入', riderNetEarningsAmount, 'income', true)
  ]

  return {
    available: true,
    unavailable_text: '',
    customer_payable_text: formatMoney(breakdown.customer_payable_amount),
    merchant_receivable_text: formatMoney(breakdown.merchant_receivable_amount),
    summary_rows: [
      createFeeBreakdownRow('food_amount', '餐费原价', breakdown.food_amount, 'default'),
      createFeeBreakdownRow('merchant_discount_amount', '商户优惠', -breakdown.merchant_discount_amount, 'discount'),
      createFeeBreakdownRow('voucher_discount_amount', '平台/券优惠', -breakdown.voucher_discount_amount, 'discount'),
      createFeeBreakdownRow('food_payable_amount', '餐费应付', breakdown.food_payable_amount, 'default'),
      createFeeBreakdownRow('delivery_fee_amount', '代取费', breakdown.delivery_fee_amount, 'default'),
      createFeeBreakdownRow('delivery_fee_discount_amount', '代取优惠', -breakdown.delivery_fee_discount_amount, 'discount'),
      createFeeBreakdownRow('delivery_payable_amount', '代取应付', breakdown.delivery_payable_amount, 'default'),
      createFeeBreakdownRow('customer_payable_amount', '用户实付', breakdown.customer_payable_amount, 'total', true)
    ],
    settlement_groups: [
      {
        key: 'merchant',
        title: '商户部分',
        total_text: formatMoney(breakdown.merchant_receivable_amount),
        tone: 'merchant',
        visible: true,
        rows: merchantRows
      },
      {
        key: 'rider',
        title: '骑手部分',
        total_text: formatMoney(riderNetEarningsAmount),
        tone: 'rider',
        visible: riderGrossAmount !== 0 || riderPaymentFeeAmount !== 0 || riderNetEarningsAmount !== 0,
        rows: riderRows
      }
    ]
  }
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
  return refunds.filter((refund) => !['failed', 'closed'].includes(refund.status)).reduce((sum, refund) => sum + refund.refund_amount, 0)
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

export function buildRefundView(refund: RefundOrder, returns: ProfitSharingReturn[] = []): MerchantOrderRefundView {
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

export function buildPaymentView(payment: PaymentOrder, refunds: RefundOrder[]): MerchantOrderPaymentView {
  const reservedAmount = getRefundReservedAmount(refunds)
  const remainingRefundAmount = payment.status === 'refunded' ? 0 : Math.max(payment.amount - reservedAmount, 0)

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

export function selectActivePayment(payments: PaymentOrder[]): PaymentOrder | null {
  if (!Array.isArray(payments) || !payments.length) return null
  const sorted = [...payments].sort((left, right) => dayjs(right.created_at).valueOf() - dayjs(left.created_at).valueOf())
  return sorted.find((payment) => ['paid', 'refunded'].includes(payment.status)) || sorted.find((payment) => payment.status === 'pending') || sorted[0] || null
}

export function getCloudStatusLabel(result?: MerchantOrderPrintJobStatusResponse) {
  if (!result) return ''
  if (!result.cloud_query_available) return '当前打印机暂不支持云端状态查询'
  if (typeof result.cloud_printed === 'boolean') return result.cloud_printed ? '云端回执显示已打印' : '云端回执显示未打印'
  return '云端状态暂未返回，请稍后再试'
}

function formatOrderTimelineTime(value?: string, pattern = 'HH:mm') {
  return value ? dayjs(value).format(pattern) : '--'
}

function formatOrderTimelineValue(value?: string, fallback = '--') {
  return value ? formatOrderTimelineTime(value) : fallback
}

export function getMerchantOrderStatusIcon(status?: string) {
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
  return icons[status || ''] || 'info-circle'
}

export function getMerchantOrderStatusDesc(order: OrderResponse) {
  const descs: Record<string, string> = {
    paid: '顾客已付款，请尽快接单或拒单处理',
    preparing: '订单已进入制作阶段',
    ready: order.order_type === 'takeout' ? '餐品已备妥，等待骑手接力' : '餐品已备妥，等待顾客取餐或到店核销',
    courier_accepted: '骑手已接单，正在前往门店',
    picked: '骑手已取餐，订单即将代取',
    delivering: '骑手代取中，请留意异常与超时',
    rider_delivered: '骑手已送达，等待顾客确认',
    user_delivered: '顾客已确认收货，订单即将完结',
    completed: '订单已成功履约完成',
    cancelled: order.cancel_reason || '订单已被系统或商户取消'
  }
  return descs[order.status] || ''
}

export function getMerchantOrderFallbackStatusHint(order: Pick<OrderResponse, 'cancel_reason' | 'overtime'>) {
  if (order.cancel_reason) {
    return `取消原因：${order.cancel_reason}`
  }
  if (order.overtime) {
    return '当前订单已超时，请优先关注'
  }
  return ''
}

export function canMerchantOrderManualPrint(order: Pick<OrderResponse, 'status'>) {
  return !['pending', 'cancelled'].includes(order.status)
}

export function buildMerchantOrderTimeline(order: OrderResponse) {
  if (order.order_type === 'takeout') {
    const steps = [
      { title: '订单提交', content: formatOrderTimelineTime(order.created_at, 'YYYY-MM-DD HH:mm') },
      { title: '已支付', content: formatOrderTimelineTime(order.paid_at) },
      { title: '商户处理', content: formatOrderTimelineValue(order.prep_start_at, order.status === 'paid' ? '待商户接单' : '--') },
      { title: '出餐完成', content: formatOrderTimelineValue(order.ready_at, order.status === 'preparing' ? '制作中' : '--') },
      { title: '骑手接单', content: formatOrderTimelineValue(order.courier_accept_at, order.status === 'ready' ? '待分配骑手' : '--') },
      { title: '骑手取餐', content: formatOrderTimelineValue(order.picked_at, order.status === 'courier_accepted' ? '骑手前往取餐' : '--') },
      { title: '送达完成', content: formatOrderTimelineValue(order.user_delivered_at || order.auto_user_delivered_at || order.rider_delivered_at || order.completed_at, order.status === 'delivering' ? '代取途中' : '--') }
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
    { title: '订单提交', content: formatOrderTimelineTime(order.created_at, 'YYYY-MM-DD HH:mm') },
    { title: '已支付', content: formatOrderTimelineTime(order.paid_at) },
    { title: '商户处理', content: formatOrderTimelineValue(order.prep_start_at, order.status === 'paid' ? '待商户接单' : '--') },
    { title: order.order_type === 'dine_in' ? '备餐完成' : '待取餐', content: formatOrderTimelineValue(order.ready_at, order.status === 'preparing' ? '制作中' : '--') },
    { title: '履约完成', content: formatOrderTimelineValue(order.completed_at, order.status === 'ready' ? '待商户确认' : '--') }
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
  if (job.status === 'success') return '打印任务已完成，可联系门店确认纸单是否正常出单。'
  if (job.status === 'failed') return job.error_message ? '打印任务未成功下发，请先根据失败原因排查后再重试。' : '打印任务未成功下发，建议重试以恢复门店打印。'
  if (job.status === 'pending' || job.status === 'processing') return '打印任务已提交，正在等待云打印平台返回结果。'
  return '打印任务状态正在同步，请稍后刷新查看。'
}

export function buildPrintJobView(job: MerchantOrderPrintJobResponse): MerchantOrderPrintJobView {
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
