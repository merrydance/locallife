import {
  buildDefaultFinanceRange,
  buildMerchantFinanceMonthRange,
  loadMerchantFinanceBillPage,
  MERCHANT_FINANCE_BILL_MAX_RANGE_DAYS,
  MERCHANT_FINANCE_PAGE_SIZE,
  type MerchantFinanceBillPageView,
  type MerchantFinanceBillRowView,
  type MerchantFinanceBillSummaryView,
  type MerchantFinanceRange
} from '../../_services/merchant-finance-workflow'
import {
  formatFinanceDateParam,
  getFinanceDateTime,
  getFinanceRangeCalendarValue,
  validateFinanceDateRange
} from '../../_main_shared/utils/finance-date-range'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../utils/user-facing'

const EMPTY_SUMMARY: MerchantFinanceBillSummaryView = {
  rangeLabel: '',
  totalIncomeText: '¥0.00',
  totalGmvText: '¥0.00',
  totalOrdersText: '0 单',
  pendingIncomeText: '¥0.00'
}
const DEFAULT_RANGE = buildDefaultFinanceRange()

interface MerchantFinanceQuickRange {
  id: string
  label: string
  active: boolean
}

function buildQuickRanges(activeRange: MerchantFinanceRange): MerchantFinanceQuickRange[] {
  const ranges = [
    { id: '7', label: '近7天', range: buildDefaultFinanceRange(7) },
    { id: '30', label: '近30天', range: buildDefaultFinanceRange(30) },
    { id: 'month', label: '本月', range: buildMerchantFinanceMonthRange() }
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
    hasLoadedOnce: false,
    loadingBills: false,
    range: DEFAULT_RANGE as MerchantFinanceRange,
    quickRanges: buildQuickRanges(DEFAULT_RANGE) as MerchantFinanceQuickRange[],
    rangePickerVisible: false,
    rangePickerValue: getFinanceRangeCalendarValue(DEFAULT_RANGE),
    rangePickerMinDate: getFinanceDateTime(new Date(new Date().getFullYear() - 1, 0, 1)),
    rangePickerMaxDate: getFinanceDateTime(new Date()),
    rows: [] as MerchantFinanceBillRowView[],
    summary: EMPTY_SUMMARY,
    page: 1,
    totalPages: 0,
    hasMore: false,
    totalCount: 0
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
    void this.loadBills({ silent: true, page: 1 })
  },

  onReachBottom() {
    void this.onLoadMore()
  },

  async onLoadMore() {
    if (!this.data.hasMore || this.data.loadingBills) {
      return
    }
    await this.loadBills({ silent: true, append: true, page: this.data.page + 1, range: this.data.range })
  },

  async loadBills(options: { silent?: boolean, append?: boolean, page?: number, range?: MerchantFinanceRange } = {}) {
    if (this.data.loadingBills) {
      wx.stopPullDownRefresh()
      return
    }

    const { silent = false, append = false, page = 1 } = options
    const range = options.range || this.data.range
    const hasTrustedData = this.data.hasLoadedOnce

    this.setData({
      loadingBills: true,
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
      const view = await loadMerchantFinanceBillPage({
        page,
        limit: MERCHANT_FINANCE_PAGE_SIZE,
        range
      })
      this.applyBillView(view, append, range)
    } catch (error) {
      logger.warn('Merchant finance bills load failed', error, 'merchant-finance-bills')
      const message = getErrorUserMessage(error, '订单流水加载失败，请稍后重试')
      this.setData({
        initialLoading: false,
        initialError: !hasTrustedData,
        initialErrorMessage: hasTrustedData ? '' : message,
        refreshErrorMessage: hasTrustedData ? message : '',
        loadingBills: false
      })
    } finally {
      wx.stopPullDownRefresh()
    }
  },

  applyBillView(view: MerchantFinanceBillPageView, append: boolean, range: MerchantFinanceRange) {
    const rows = append ? this.data.rows.concat(view.rows) : view.rows

    this.setData({
      initialLoading: false,
      initialError: false,
      initialErrorMessage: '',
      hasLoadedOnce: true,
      loadingBills: false,
      range,
      quickRanges: buildQuickRanges(range),
      rangePickerValue: getFinanceRangeCalendarValue(range),
      rows,
      summary: append && view.summaryErrorMessage ? this.data.summary : view.summary,
      refreshErrorMessage: view.summaryErrorMessage,
      page: view.page,
      totalPages: view.totalPages,
      hasMore: view.hasMore,
      totalCount: view.totalCount
    })
  },

  onRetry() {
    void this.loadBills({ page: 1, range: this.data.range })
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
      MERCHANT_FINANCE_BILL_MAX_RANGE_DAYS,
      '订单流水'
    )
    if (!validation.valid) {
      wx.showToast({ title: validation.message || '订单流水最多选择90天', icon: 'none' })
      return
    }
    this.applyRange(range)
  },

  onUseQuickRange(e: WechatMiniprogram.BaseEvent) {
    const rangeID = String(e.currentTarget.dataset.range || '')
    if (rangeID === '7') {
      this.applyRange(buildDefaultFinanceRange(7))
      return
    }
    if (rangeID === '30') {
      this.applyRange(buildDefaultFinanceRange(30))
      return
    }
    if (rangeID === 'month') {
      this.applyRange(buildMerchantFinanceMonthRange())
    }
  },

  applyRange(range: MerchantFinanceRange) {
    this.setData({ rangePickerVisible: false })
    void this.loadBills({ page: 1, range })
  }
})
