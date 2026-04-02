import { getStableBarHeights } from '../../../utils/responsive'
import {
  createMerchantWithdraw,
  getMerchantDailyFinance,
  getMerchantFinanceOverview,
  getMerchantAccountBalance,
  getMerchantApplymentStatus,
  getMerchantWithdrawal,
  getMerchantPromotionExpenses,
  getMerchantServiceFees,
  listMerchantFinanceOrders,
  listMerchantSettlementTimeline,
  listMerchantSettlements,
  ApplymentStatusResponse,
  MerchantDailyFinanceItem,
  MerchantFinanceOrderItem,
  MerchantFinanceOverviewResponse,
  MerchantPromotionExpenseItem,
  MerchantPromotionExpensesResponse,
  MerchantSettlementItem,
  MerchantSettlementTimelineItem,
  MerchantSettlementsResponse,
  MerchantServiceFeeItem,
  MerchantServiceFeeSummaryResponse,
  MerchantAccountBalanceResponse,
  MerchantWithdrawItem,
  listMerchantWithdrawals
} from '../../../api/merchant-finance'
import { logger } from '../../../utils/logger'
import { settleAll } from '../../../utils/promise'
import dayjs from 'dayjs'
import { getErrorDebugMessage, getErrorUserMessage } from '../../../utils/user-facing'

type InputChangeDetail = {
  value: string
}

type FinanceRangeKey = '7d' | '30d'
type SettlementStatusFilter = 'all' | 'pending' | 'processing' | 'finished' | 'failed'

type RangeOption = {
  key: FinanceRangeKey
  label: string
  days: number
}

type SettlementStatusOption = {
  key: SettlementStatusFilter
  label: string
}

const emptyApplyment: ApplymentStatusResponse = {
  status: '',
  status_desc: ''
}

const FINANCE_RANGE_OPTIONS: RangeOption[] = [
  { key: '7d', label: '近7天', days: 7 },
  { key: '30d', label: '近30天', days: 30 }
]

const SETTLEMENT_STATUS_OPTIONS: SettlementStatusOption[] = [
  { key: 'all', label: '全部' },
  { key: 'pending', label: '待结算' },
  { key: 'processing', label: '处理中' },
  { key: 'finished', label: '已完成' },
  { key: 'failed', label: '失败' }
]

const EMPTY_FINANCE_OVERVIEW: MerchantFinanceOverviewResponse = {
  completed_orders: 0,
  pending_orders: 0,
  total_gmv: 0,
  total_income: 0,
  total_platform_fee: 0,
  total_operator_fee: 0,
  total_service_fee: 0,
  pending_income: 0,
  promotion_orders: 0,
  total_promotion_exp: 0,
  net_income: 0
}

const EMPTY_SERVICE_FEES: MerchantServiceFeeSummaryResponse = {
  details: [],
  total_platform_fee: 0,
  total_operator_fee: 0,
  total_service_fee: 0
}

const EMPTY_PROMOTIONS: MerchantPromotionExpensesResponse = {
  orders: [],
  total: 0,
  page: 1,
  limit: 5,
  total_pages: 0,
  total_promo_orders: 0,
  total_promo_amount: 0
}

const EMPTY_SETTLEMENTS: MerchantSettlementsResponse = {
  settlements: [],
  total: 0,
  page: 1,
  limit: 5,
  total_pages: 0,
  total_amount: 0,
  total_merchant_amount: 0,
  total_platform_fee: 0,
  total_operator_fee: 0
}

function buildFinanceRange(rangeKey: FinanceRangeKey) {
  const option = FINANCE_RANGE_OPTIONS.find((item) => item.key === rangeKey) || FINANCE_RANGE_OPTIONS[0]
  const end = dayjs()
  const start = end.subtract(option.days - 1, 'day')
  return {
    label: `${start.format('MM.DD')} - ${end.format('MM.DD')}`,
    params: {
      start_date: start.format('YYYY-MM-DD'),
      end_date: end.format('YYYY-MM-DD')
    }
  }
}

const getErrorMessage = getErrorUserMessage

Page({
  data: {
    navBarHeight: 88,
    loading: true,
    submitting: false,
    notConfigured: false,
    balanceLoaded: false,
    balanceError: false,
    balanceErrorMessage: '',
    balanceStatusDesc: '',
    rangeOptions: FINANCE_RANGE_OPTIONS,
    currentRange: '7d' as FinanceRangeKey,
    settlementStatusOptions: SETTLEMENT_STATUS_OPTIONS,
    currentSettlementStatus: 'all' as SettlementStatusFilter,
    financeLoaded: false,
    financeLoading: true,
    financeError: false,
    financeErrorMessage: '',
    financeUpdatedAtLabel: '--',
    financeDateRangeLabel: '--',
    financeOverview: EMPTY_FINANCE_OVERVIEW as MerchantFinanceOverviewResponse,
    financeOrders: [] as MerchantFinanceOrderItem[],
    serviceFeeSummary: EMPTY_SERVICE_FEES as MerchantServiceFeeSummaryResponse,
    serviceFeeDetails: [] as MerchantServiceFeeItem[],
    promotionSummary: EMPTY_PROMOTIONS as MerchantPromotionExpensesResponse,
    promotionOrders: [] as MerchantPromotionExpenseItem[],
    dailyFinanceRows: [] as MerchantDailyFinanceItem[],
    settlementsSummary: EMPTY_SETTLEMENTS as MerchantSettlementsResponse,
    settlements: [] as MerchantSettlementItem[],
    settlementTimeline: [] as MerchantSettlementTimelineItem[],

    /* Balance */
    balance: {
      sub_mch_id: '',
      available_amount: 0,
      pending_amount: 0,
      withdrawable_amount: 0,
      account_status: '',
      status_desc: ''
    } as MerchantAccountBalanceResponse,
    withdrawAmountYuan: '',
    withdrawRemark: '',
    withdrawSyncingId: 0,
    withdrawals: [] as MerchantWithdrawItem[],

    /* Applyment */
    loadingApplyment: true,
    applymentLoaded: false,
    applymentError: false,
    applymentErrorMessage: '',
    applymentStatus: emptyApplyment as ApplymentStatusResponse | null,
    hasApplyment: false
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.loadData()
  },

  onPullDownRefresh() {
    this.loadData()
  },

  async loadData() {
    this.setData({ loading: true })
    await Promise.all([this.loadBalance(), this.loadApplymentStatus(), this.loadFinanceInsights()])
    this.setData({ loading: false })
    wx.stopPullDownRefresh()
  },

  async loadFinanceInsights() {
    const { params, label } = buildFinanceRange(this.data.currentRange)

    this.setData({
      financeLoading: true,
      financeError: false,
      financeErrorMessage: '',
      financeDateRangeLabel: label
    })

    try {
      const [overviewResult, ordersResult, serviceFeeResult, promotionsResult, dailyResult, settlementsResult, timelineResult] = await settleAll([
        getMerchantFinanceOverview(params),
        listMerchantFinanceOrders({ ...params, page: 1, limit: 5 }),
        getMerchantServiceFees(params),
        getMerchantPromotionExpenses({ ...params, page: 1, limit: 5 }),
        getMerchantDailyFinance(params),
        listMerchantSettlements({
          ...params,
          page: 1,
          limit: 5,
          ...(this.data.currentSettlementStatus !== 'all'
            ? { status: this.data.currentSettlementStatus }
            : {})
        }),
        listMerchantSettlementTimeline({ ...params, page: 1, limit: 5 })
      ] as const)

      if (
        overviewResult.status === 'rejected'
        && ordersResult.status === 'rejected'
        && serviceFeeResult.status === 'rejected'
        && promotionsResult.status === 'rejected'
        && dailyResult.status === 'rejected'
        && settlementsResult.status === 'rejected'
        && timelineResult.status === 'rejected'
      ) {
        throw new Error('all_finance_requests_failed')
      }

      this.setData({
        financeOverview: overviewResult.status === 'fulfilled' ? overviewResult.value : EMPTY_FINANCE_OVERVIEW,
        financeOrders: ordersResult.status === 'fulfilled' ? ordersResult.value.orders || [] : [],
        serviceFeeSummary: serviceFeeResult.status === 'fulfilled' ? serviceFeeResult.value : EMPTY_SERVICE_FEES,
        serviceFeeDetails: serviceFeeResult.status === 'fulfilled' ? serviceFeeResult.value.details || [] : [],
        promotionSummary: promotionsResult.status === 'fulfilled' ? promotionsResult.value : EMPTY_PROMOTIONS,
        promotionOrders: promotionsResult.status === 'fulfilled' ? promotionsResult.value.orders || [] : [],
        dailyFinanceRows: dailyResult.status === 'fulfilled' ? (dailyResult.value.daily_stats || []).slice(0, 7) : [],
        settlementsSummary: settlementsResult.status === 'fulfilled' ? settlementsResult.value : EMPTY_SETTLEMENTS,
        settlements: settlementsResult.status === 'fulfilled' ? settlementsResult.value.settlements || [] : [],
        settlementTimeline: timelineResult.status === 'fulfilled' ? timelineResult.value.timeline || [] : [],
        financeLoaded: true,
        financeLoading: false,
        financeError: false,
        financeErrorMessage: '',
        financeUpdatedAtLabel: dayjs().format('HH:mm')
      })
    } catch (error) {
      logger.error('Load merchant finance insights failed', error, 'merchant-finance')
      this.setData({
        financeLoaded: true,
        financeLoading: false,
        financeError: true,
        financeErrorMessage: '财务概览加载失败，请稍后重试'
      })
    }
  },

  async loadBalance() {
    this.setData({ balanceError: false, balanceErrorMessage: '' })
    try {
      const [balance, records] = await Promise.all([
        getMerchantAccountBalance(),
        listMerchantWithdrawals(1, 20)
      ])

      const accountStatus = balance.account_status || records.account_status || ''
      const statusDesc = balance.status_desc || records.status_desc || ''
      const isActive = accountStatus === 'active'

      this.setData({
        balance,
        notConfigured: !isActive,
        withdrawals: isActive ? (records.withdrawals || []) : [],
        balanceLoaded: true,
        balanceError: false,
        balanceErrorMessage: '',
        balanceStatusDesc: statusDesc
      })
    } catch (error: unknown) {
      const msg = getErrorDebugMessage(error)
      if (msg.includes('404')) {
        this.setData({
          notConfigured: true,
          balanceLoaded: true,
          balanceError: false,
          balanceErrorMessage: '',
          balanceStatusDesc: '暂未查询到收付通账户，请先完成商户进件和签约。'
        })
      } else {
        logger.error('Load merchant finance data failed', error, 'merchant-finance')
        this.setData({
          balanceLoaded: true,
          balanceError: true,
          balanceErrorMessage: '加载资金数据失败，请重试',
          balanceStatusDesc: ''
        })
      }
    }
  },

  async loadApplymentStatus() {
    this.setData({ loadingApplyment: true, applymentError: false, applymentErrorMessage: '' })
    try {
      const data = await getMerchantApplymentStatus()
      this.setData({
        applymentStatus: data,
        hasApplyment: true,
        applymentLoaded: true,
        applymentError: false,
        applymentErrorMessage: ''
      })
    } catch (error: unknown) {
      const msg = getErrorDebugMessage(error)
      if (msg.includes('404')) {
        this.setData({
          applymentStatus: null,
          hasApplyment: false,
          applymentLoaded: true,
          applymentError: false,
          applymentErrorMessage: ''
        })
      } else {
        logger.error('Load applyment status failed', error, 'merchant-finance')
        this.setData({
          applymentStatus: null,
          hasApplyment: false,
          applymentLoaded: true,
          applymentError: true,
          applymentErrorMessage: '查询进件状态失败，请重试'
        })
      }
    } finally {
      this.setData({ loadingApplyment: false })
    }
  },

  onWithdrawAmountChange(e: WechatMiniprogram.CustomEvent<InputChangeDetail>) {
    this.setData({ withdrawAmountYuan: e.detail.value })
  },

  onWithdrawRemarkChange(e: WechatMiniprogram.CustomEvent<InputChangeDetail>) {
    this.setData({ withdrawRemark: e.detail.value })
  },

  async onSubmitWithdraw() {
    if (this.data.submitting) return

    const amountYuan = Number(this.data.withdrawAmountYuan)
    if (!Number.isFinite(amountYuan) || amountYuan < 1) {
      wx.showToast({ title: '提现金额至少1元', icon: 'none' })
      return
    }

    if (!this.data.withdrawRemark.trim()) {
      wx.showToast({ title: '请输入提现备注', icon: 'none' })
      return
    }

    const amount = Math.round(amountYuan * 100)
    if (amount > this.data.balance.withdrawable_amount) {
      wx.showToast({ title: '超过可提现余额', icon: 'none' })
      return
    }

    this.setData({ submitting: true })
    wx.showLoading({ title: '提交中...' })

    try {
      const result = await createMerchantWithdraw({
        amount,
        remark: this.data.withdrawRemark.trim()
      })

      this.upsertWithdrawal(result.withdrawal)
      this.setData({ withdrawAmountYuan: '', withdrawRemark: '' })
      await this.loadBalance()
      wx.showModal({
        title: '提现申请已提交',
        content: this.getWithdrawCreatedMessage(result.withdrawal),
        showCancel: false,
        confirmText: '知道了'
      })
    } catch (error) {
      logger.error('Submit merchant withdraw failed', error, 'merchant-finance')
      wx.showToast({
        title: getErrorMessage(error, '提现申请失败，请稍后重试'),
        icon: 'none'
      })
    } finally {
      wx.hideLoading()
      this.setData({ submitting: false })
    }
  },

  onRetrySection() {
    this.loadData()
  },

  async onRefreshWithdrawal(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id || this.data.withdrawSyncingId === id) return

    this.setData({ withdrawSyncingId: id })
    try {
      const record = await getMerchantWithdrawal(id)
      this.upsertWithdrawal(record)
      const latestStatusLabel = this.getStatusText(record.status)
      const latestReason = record.reason
        ? `，原因：${record.reason.slice(0, 10)}${record.reason.length > 10 ? '...' : ''}`
        : ''
      wx.showToast({ title: `状态已同步为${latestStatusLabel}${latestReason}`, icon: 'none' })
    } catch (error) {
      logger.error('Refresh merchant withdrawal failed', error, 'merchant-finance')
      wx.showToast({
        title: getErrorMessage(error, '同步提现状态失败，请稍后重试'),
        icon: 'none'
      })
    } finally {
      this.setData({ withdrawSyncingId: 0 })
    }
  },

  onSelectRange(e: WechatMiniprogram.TouchEvent) {
    const { key } = e.currentTarget.dataset as { key?: FinanceRangeKey }
    if (!key || key === this.data.currentRange) return
    this.setData({ currentRange: key })
    this.loadFinanceInsights()
  },

  onSelectSettlementStatus(e: WechatMiniprogram.TouchEvent) {
    const { key } = e.currentTarget.dataset as { key?: SettlementStatusFilter }
    if (!key || key === this.data.currentSettlementStatus) return
    this.setData({ currentSettlementStatus: key })
    this.loadFinanceInsights()
  },

  onGoApplymentSettings() {
    wx.navigateTo({ url: '/pages/merchant/settings/applyment/index' })
  },

  /* ── Utils ── */

  formatAmount(fen: number): string {
    return (fen / 100).toFixed(2)
  },

  formatAmountText(fen: number): string {
    return `¥${this.formatAmount(fen)}`
  },

  formatDateTime(value?: string): string {
    if (!value) return '暂无'
    return value.replace('T', ' ').slice(0, 16)
  },

  upsertWithdrawal(withdrawal: MerchantWithdrawItem) {
    const next = [withdrawal, ...this.data.withdrawals.filter((item) => item.id !== withdrawal.id)]
    this.setData({
      withdrawals: next.slice(0, 20)
    })
  },

  getWithdrawCreatedMessage(withdrawal: MerchantWithdrawItem): string {
    const parts = [
      `状态：${this.getStatusText(withdrawal.status)}`
    ]

    if (withdrawal.out_request_no) {
      parts.push(`请求单号：${withdrawal.out_request_no}`)
    }
    if (withdrawal.withdraw_id) {
      parts.push(`微信提现单号：${withdrawal.withdraw_id}`)
    }
    if (withdrawal.reason) {
      parts.push(`原因：${withdrawal.reason}`)
    }

    parts.push('可在提现记录里单独同步状态。')
    return parts.join('\n')
  },

  getOrderSourceText(source?: string): string {
    switch (source) {
      case 'takeout':
        return '外卖订单'
      case 'dine_in':
        return '堂食订单'
      case 'reservation':
        return '预订订单'
      default:
        return source || '订单'
    }
  },

  getFinanceOrderStatusText(status?: string): string {
    switch (status) {
      case 'finished':
        return '已完成'
      case 'pending':
        return '待结算'
      case 'cancelled':
        return '已取消'
      case 'failed':
        return '失败'
      default:
        return status || '处理中'
    }
  },

  getFinanceOrderStatusTheme(status?: string): string {
    switch (status) {
      case 'finished':
        return 'success'
      case 'processing':
        return 'primary'
      case 'pending':
        return 'warning'
      case 'cancelled':
      case 'failed':
        return 'danger'
      default:
        return 'default'
    }
  },

  getTimelineRecordTypeText(recordType?: string): string {
    switch (recordType) {
      case 'profit_sharing':
        return '分账结算'
      case 'adjustment':
        return '结算调整'
      default:
        return recordType || '流水'
    }
  },

  getAdjustmentTypeText(adjustmentType?: string): string {
    switch (adjustmentType) {
      case 'claim_recovery_charge':
        return '索赔追偿扣款'
      case 'claim_recovery_reversal':
        return '索赔追偿回补'
      default:
        return adjustmentType || '结算调整'
    }
  },

  getTimelineAmountText(item: MerchantSettlementTimelineItem): string {
    if (item.record_type === 'adjustment') {
      return this.formatAmountText(item.merchant_amount || item.total_amount || 0)
    }
    return this.formatAmountText(item.merchant_amount)
  },

  getApplymentStatusText(status: string): string {
    const map: Record<string, string> = {
      submitted: '已提交',
      bindbank_submitted: '进件审核中',
      auditing: '审核中',
      to_be_signed: '待签约',
      signing: '签约中',
      finish: '已开通',
      active: '已开通',
      rejected: '已拒绝'
    }
    return map[status] || status
  },

  getApplymentStatusTheme(status: string): string {
    switch (status) {
      case 'finish':
      case 'active':
        return 'success'
      case 'rejected':
        return 'danger'
      case 'to_be_signed':
      case 'signing':
        return 'primary'
      default:
        return 'warning'
    }
  },

  getStatusText(status: string): string {
    switch (status) {
      case 'pending': return '处理中'
      case 'success': return '成功'
      case 'failed': return '失败'
      default: return status
    }
  },

  getStatusTheme(status: string): string {
    switch (status) {
      case 'pending': return 'warning'
      case 'success': return 'success'
      case 'failed': return 'danger'
      default: return 'default'
    }
  }
})
