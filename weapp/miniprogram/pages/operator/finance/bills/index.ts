import {
  buildOperatorCommissionBillMonthRange,
  buildOperatorCommissionBillRange,
  loadOperatorCommissionBillPage,
  OPERATOR_COMMISSION_BILL_PAGE_SIZE,
  type OperatorCommissionBillRange,
  type OperatorCommissionBillPageView,
  type OperatorCommissionBillRowView,
  type OperatorCommissionBillSummaryView
} from '../../_services/operator-finance'
import {
  formatFinanceDateParam,
  getFinanceDateTime,
  getFinanceRangeCalendarValue,
  validateFinanceDateRange
} from '../../_main_shared/utils/finance-date-range'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../utils/user-facing'

const OPERATOR_COMMISSION_BILL_MAX_RANGE_DAYS = 365
const EMPTY_SUMMARY: OperatorCommissionBillSummaryView = {
  rangeLabel: '',
  totalCommissionText: '¥0.00',
  totalGmvText: '¥0.00',
  totalOrdersText: '0 单'
}
const DEFAULT_RANGE = buildOperatorCommissionBillRange()

interface OperatorCommissionQuickRange {
  id: string
  label: string
  active: boolean
}

function buildQuickRanges(activeRange: OperatorCommissionBillRange): OperatorCommissionQuickRange[] {
  const ranges = [
    { id: '7', label: '近7天', range: buildOperatorCommissionBillRange(7) },
    { id: '30', label: '近30天', range: buildOperatorCommissionBillRange(30) },
    { id: 'month', label: '本月', range: buildOperatorCommissionBillMonthRange() }
  ]
  return ranges.map((item) => ({
    id: item.id,
    label: item.label,
    active: item.range.start_date === activeRange.start_date && item.range.end_date === activeRange.end_date
  }))
}

Page({
  data: {
    navBarHeight: 88,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    refreshing: false,
    hasLoadedOnce: false,
    loadingBills: false,
    loadingMore: false,
    range: DEFAULT_RANGE as OperatorCommissionBillRange,
    quickRanges: buildQuickRanges(DEFAULT_RANGE) as OperatorCommissionQuickRange[],
    rangePickerVisible: false,
    rangePickerValue: getFinanceRangeCalendarValue(DEFAULT_RANGE),
    rangePickerMinDate: getFinanceDateTime(new Date(new Date().getFullYear() - 1, 0, 1)),
    rangePickerMaxDate: getFinanceDateTime(new Date()),
    rows: [] as OperatorCommissionBillRowView[],
    summary: EMPTY_SUMMARY,
    page: 1,
    totalPages: 0,
    hasMore: false,
    totalCount: 0,
    scrollTop: 0
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    await this.loadBills()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  onPullDownRefresh() {
    this.setData({ refreshing: true })
    void this.loadBills({ silent: true, page: 1 }).finally(() => {
      this.setData({ refreshing: false })
      wx.stopPullDownRefresh()
    })
  },

  async onLoadMore() {
    if (!this.data.hasMore || this.data.loadingBills || this.data.loadingMore) {
      return
    }
    await this.loadBills({ silent: true, append: true, page: this.data.page + 1, range: this.data.range })
  },

  async loadBills(options: { silent?: boolean, append?: boolean, page?: number, range?: OperatorCommissionBillRange } = {}) {
    if (this.data.loadingBills) {
      this.setData({ refreshing: false })
      wx.stopPullDownRefresh()
      return
    }

    const { silent = false, append = false, page = 1 } = options
    const range = options.range || this.data.range
    const hasTrustedData = this.data.hasLoadedOnce

    this.setData({
      loadingBills: true,
      loadingMore: append,
      ...(silent || hasTrustedData
        ? { refreshErrorMessage: '' }
        : {
            initialLoading: true,
            initialError: false,
            initialErrorMessage: '',
            refreshErrorMessage: ''
          })
    })

    try {
      const view = await loadOperatorCommissionBillPage({
        page,
        limit: OPERATOR_COMMISSION_BILL_PAGE_SIZE,
        range
      })
      this.applyBillView(view, append, range)
    } catch (error) {
      logger.warn('Operator commission bills load failed', error, 'operator-finance-bills')
      const message = getErrorUserMessage(error, '佣金账单加载失败，请稍后重试')
      this.setData({
        initialLoading: false,
        initialError: !hasTrustedData,
        initialErrorMessage: hasTrustedData ? '' : message,
        refreshErrorMessage: hasTrustedData ? message : '',
        loadingBills: false,
        loadingMore: false
      })
    }
  },

  applyBillView(view: OperatorCommissionBillPageView, append: boolean, range: OperatorCommissionBillRange) {
    const rows = append ? this.data.rows.concat(view.rows) : view.rows

    this.setData({
      initialLoading: false,
      initialError: false,
      initialErrorMessage: '',
      refreshErrorMessage: '',
      hasLoadedOnce: true,
      loadingBills: false,
      loadingMore: false,
      range,
      quickRanges: buildQuickRanges(range),
      rangePickerValue: getFinanceRangeCalendarValue(range),
      rows,
      summary: view.summary,
      page: view.page,
      totalPages: view.totalPages,
      hasMore: view.hasMore,
      totalCount: view.totalCount
    })
  },

  onRetry() {
    void this.loadBills({ page: 1, range: this.data.range })
  },

  resetBillScrollTop() {
    this.setData({ scrollTop: 1 })
    wx.nextTick(() => {
      this.setData({ scrollTop: 0 })
    })
  },

  onOpenRangePicker() {
    this.setData({
      rangePickerVisible: true,
      rangePickerValue: getFinanceRangeCalendarValue(this.data.range)
    })
  },

  onCancelRangePicker() {
    this.setData({ rangePickerVisible: false })
  },

  onConfirmRangePicker(e: WechatMiniprogram.CustomEvent<{ value?: number[] }>) {
    const value = Array.isArray(e.detail.value) ? e.detail.value : []
    const start = value[0] ? new Date(value[0]) : null
    const end = value[1] ? new Date(value[1]) : null
    if (!start || !end) {
      wx.showToast({ title: '请选择完整日期区间', icon: 'none' })
      return
    }
    const range = {
      start_date: formatFinanceDateParam(start),
      end_date: formatFinanceDateParam(end)
    }
    const validation = validateFinanceDateRange(
      range,
      OPERATOR_COMMISSION_BILL_MAX_RANGE_DAYS,
      '佣金账单'
    )
    if (!validation.valid) {
      wx.showToast({ title: validation.message || '佣金账单最多选择365天', icon: 'none' })
      return
    }
    this.applyRange(range)
  },

  onUseQuickRange(e: WechatMiniprogram.BaseEvent) {
    const rangeID = String(e.currentTarget.dataset.range || '')
    if (rangeID === '7') {
      this.applyRange(buildOperatorCommissionBillRange(7))
      return
    }
    if (rangeID === '30') {
      this.applyRange(buildOperatorCommissionBillRange(30))
      return
    }
    if (rangeID === 'month') {
      this.applyRange(buildOperatorCommissionBillMonthRange())
    }
  },

  applyRange(range: OperatorCommissionBillRange) {
    this.setData({ rangePickerVisible: false })
    this.resetBillScrollTop()
    void this.loadBills({ page: 1, range })
  }
})
