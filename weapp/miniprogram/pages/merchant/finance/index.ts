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
  MerchantDailyFinanceSummaryResponse,
  MerchantFinanceOrderItem,
  MerchantFinanceOrdersResponse,
  MerchantFinanceOverviewResponse,
  MerchantPromotionExpenseItem,
  MerchantPromotionExpensesResponse,
  MerchantSettlementItem,
  MerchantSettlementTimelineItem,
  MerchantSettlementTimelineResponse,
  MerchantSettlementsResponse,
  MerchantServiceFeeItem,
  MerchantServiceFeeSummaryResponse,
  MerchantAccountBalanceResponse,
  MerchantWithdrawItem,
  listMerchantWithdrawals
} from '../../../api/merchant-finance'
import { logger } from '../../../utils/logger'
import { settleAll, type SettledResult } from '../../../utils/promise'
import dayjs from 'dayjs'
import { getErrorDebugMessage, getErrorUserMessage } from '../../../utils/user-facing'
import { ensureMerchantConsoleAccess } from '../../../utils/console-access'

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

type FinanceSectionKey = 'overview' | 'orders' | 'serviceFees' | 'promotions' | 'daily' | 'settlements' | 'timeline'

type FinanceSectionState = {
  loaded: boolean
  error: boolean
  errorMessage: string
  stale: boolean
}

type FinanceSectionStates = Record<FinanceSectionKey, FinanceSectionState>

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

const FINANCE_SECTION_KEYS: FinanceSectionKey[] = [
  'overview',
  'orders',
  'serviceFees',
  'promotions',
  'daily',
  'settlements',
  'timeline'
]

const FINANCE_SECTION_LABELS: Record<FinanceSectionKey, string> = {
  overview: '财务概览',
  orders: '订单收入明细',
  serviceFees: '服务费明细',
  promotions: '营销支出',
  daily: '财务日报',
  settlements: '结算记录',
  timeline: '结算流水'
}

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

function createFinanceSectionState(): FinanceSectionState {
  return {
    loaded: false,
    error: false,
    errorMessage: '',
    stale: false
  }
}

function createFinanceSectionStates(): FinanceSectionStates {
  return {
    overview: createFinanceSectionState(),
    orders: createFinanceSectionState(),
    serviceFees: createFinanceSectionState(),
    promotions: createFinanceSectionState(),
    daily: createFinanceSectionState(),
    settlements: createFinanceSectionState(),
    timeline: createFinanceSectionState()
  }
}

function cloneFinanceSectionStates(states: FinanceSectionStates): FinanceSectionStates {
  return {
    overview: { ...states.overview },
    orders: { ...states.orders },
    serviceFees: { ...states.serviceFees },
    promotions: { ...states.promotions },
    daily: { ...states.daily },
    settlements: { ...states.settlements },
    timeline: { ...states.timeline }
  }
}

function buildFinanceSnapshotKey(rangeKey: FinanceRangeKey, status: SettlementStatusFilter) {
  return `${rangeKey}:${status}`
}

function buildRefreshErrorMessage(messages: string[]) {
  const normalized = messages.filter((message) => typeof message === 'string' && message.trim())
  if (!normalized.length) return ''
  return Array.from(new Set(normalized)).join('；')
}

function hasExistingApplyment(status?: string) {
  return Boolean(status && status !== 'not_applied' && status !== 'pending')
}

const getErrorMessage = getErrorUserMessage

Page({
  data: {
    navBarHeight: 88,
    accessReady: false,
    accessDenied: false,
    accessErrorMessage: '',
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
    financeRefreshErrorMessage: '',
    financeUpdatedAtLabel: '--',
    financeDateRangeLabel: '--',
    financeSnapshotKey: '',
    financeRequestVersion: 0,
    financeSectionStates: createFinanceSectionStates() as FinanceSectionStates,
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

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })

    const accessResult = await ensureMerchantConsoleAccess()
    this.setData({
      accessReady: true,
      accessDenied: accessResult.status === 'denied',
      accessErrorMessage: accessResult.status === 'error' ? accessResult.message : ''
    })
    if (accessResult.status !== 'granted') {
      this.setData({ loading: false })
      return
    }

    this.loadData()
  },

  onPullDownRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    this.loadData()
  },

  onRetryAccess() {
    this.setData({
      accessReady: false,
      accessDenied: false,
      accessErrorMessage: '',
      loading: true
    })
    this.onLoad()
  },

  async loadData() {
    this.setData({ loading: true })
    await Promise.all([this.loadBalance(), this.loadApplymentStatus(), this.loadFinanceInsights()])
    this.setData({ loading: false })
    wx.stopPullDownRefresh()
  },

  async loadFinanceInsights() {
    const requestedRange = this.data.currentRange
    const requestedSettlementStatus = this.data.currentSettlementStatus
    const { params, label } = buildFinanceRange(requestedRange)
    const requestedSnapshotKey = buildFinanceSnapshotKey(requestedRange, requestedSettlementStatus)
    const requestVersion = this.data.financeRequestVersion + 1
    const sameSnapshot = this.data.financeSnapshotKey === requestedSnapshotKey
    const previousSectionStates = sameSnapshot
      ? cloneFinanceSectionStates(this.data.financeSectionStates as FinanceSectionStates)
      : createFinanceSectionStates()

    this.setData({
      financeRequestVersion: requestVersion,
      financeLoading: true,
      financeError: false,
      financeErrorMessage: '',
      financeRefreshErrorMessage: '',
      financeLoaded: sameSnapshot ? this.data.financeLoaded : false,
      financeSectionStates: sameSnapshot ? this.data.financeSectionStates : createFinanceSectionStates(),
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
          ...(requestedSettlementStatus !== 'all'
            ? { status: requestedSettlementStatus }
            : {})
        }),
        listMerchantSettlementTimeline({ ...params, page: 1, limit: 5 })
      ] as const)

      if (requestVersion !== this.data.financeRequestVersion) {
        return
      }

      const nextSectionStates = cloneFinanceSectionStates(previousSectionStates)
      const nextData: Record<string, unknown> = {}
      const staleMessages: string[] = []
      let hasAnySuccess = false

      const applySectionResult = <T>(
        key: FinanceSectionKey,
        result: SettledResult<T>,
        onSuccess: (value: T) => void
      ) => {
        if (result.status === 'fulfilled') {
          hasAnySuccess = true
          onSuccess(result.value)
          nextSectionStates[key] = {
            loaded: true,
            error: false,
            errorMessage: '',
            stale: false
          }
          return
        }

        const message = getErrorMessage(result.reason, `${FINANCE_SECTION_LABELS[key]}加载失败，请稍后重试`)
        if (previousSectionStates[key].loaded) {
          const staleMessage = `${message}，当前已保留上次同步结果`
          nextSectionStates[key] = {
            loaded: true,
            error: true,
            errorMessage: staleMessage,
            stale: true
          }
          staleMessages.push(staleMessage)
          return
        }

        nextSectionStates[key] = {
          loaded: false,
          error: true,
          errorMessage: message,
          stale: false
        }
      }

      applySectionResult<MerchantFinanceOverviewResponse>('overview', overviewResult, (value) => {
        nextData.financeOverview = value
      })
      applySectionResult<MerchantFinanceOrdersResponse>('orders', ordersResult, (value) => {
        nextData.financeOrders = value.orders || []
      })
      applySectionResult<MerchantServiceFeeSummaryResponse>('serviceFees', serviceFeeResult, (value) => {
        nextData.serviceFeeSummary = value
        nextData.serviceFeeDetails = value.details || []
      })
      applySectionResult<MerchantPromotionExpensesResponse>('promotions', promotionsResult, (value) => {
        nextData.promotionSummary = value
        nextData.promotionOrders = value.orders || []
      })
      applySectionResult<MerchantDailyFinanceSummaryResponse>('daily', dailyResult, (value) => {
        nextData.dailyFinanceRows = (value.daily_stats || []).slice(0, 7)
      })
      applySectionResult<MerchantSettlementsResponse>('settlements', settlementsResult, (value) => {
        nextData.settlementsSummary = value
        nextData.settlements = value.settlements || []
      })
      applySectionResult<MerchantSettlementTimelineResponse>('timeline', timelineResult, (value) => {
        nextData.settlementTimeline = value.timeline || []
      })

      const hasAnyLoaded = FINANCE_SECTION_KEYS.some((key) => nextSectionStates[key].loaded)

      this.setData({
        ...nextData,
        financeSnapshotKey: hasAnyLoaded ? requestedSnapshotKey : this.data.financeSnapshotKey,
        financeSectionStates: nextSectionStates,
        financeLoaded: hasAnyLoaded,
        financeLoading: false,
        financeError: !hasAnyLoaded,
        financeErrorMessage: !hasAnyLoaded ? '财务数据加载失败，请稍后重试' : '',
        financeRefreshErrorMessage: buildRefreshErrorMessage(staleMessages),
        ...(hasAnySuccess ? { financeUpdatedAtLabel: dayjs().format('HH:mm') } : {})
      })
    } catch (error) {
      if (requestVersion !== this.data.financeRequestVersion) {
        return
      }

      logger.error('Load merchant finance insights failed', error, 'merchant-finance')
      const hasExistingData = FINANCE_SECTION_KEYS.some((key) => previousSectionStates[key].loaded)
      this.setData({
        financeLoaded: hasExistingData,
        financeLoading: false,
        financeError: !hasExistingData,
        financeErrorMessage: hasExistingData ? '' : '财务数据加载失败，请稍后重试',
        financeRefreshErrorMessage: hasExistingData ? '财务数据刷新失败，当前已保留上次同步结果' : ''
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
        hasApplyment: hasExistingApplyment(data.status),
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
    if (this.data.accessErrorMessage) {
      this.onRetryAccess()
      return
    }

    if (!this.data.accessReady || this.data.accessDenied) return
    this.loadData()
  },

  onRetryFinanceInsights() {
    if (this.data.accessErrorMessage) {
      this.onRetryAccess()
      return
    }

    if (!this.data.accessReady || this.data.accessDenied) return
    this.loadFinanceInsights()
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
      not_applied: '尚未提交进件资料',
      pending: '尚未提交进件资料',
      submitted: '已提交',
      bindbank_submitted: '进件审核中',
      auditing: '审核中',
      to_be_signed: '待签约',
      signing: '签约中',
      finish: '已开通',
      active: '已开通',
      rejected: '已拒绝',
      rejected_sign: '签约被拒绝',
      frozen: '已冻结'
    }
    return map[status] || '状态更新中'
  },

  getApplymentStatusTheme(status: string): string {
    switch (status) {
      case 'finish':
      case 'active':
        return 'success'
      case 'rejected':
      case 'rejected_sign':
      case 'frozen':
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
