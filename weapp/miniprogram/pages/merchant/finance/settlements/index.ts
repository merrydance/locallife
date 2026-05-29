import {
  buildDefaultFinanceRange,
  buildMerchantFinanceMonthRange,
  loadMerchantFinanceSettlementPage,
  MERCHANT_FINANCE_SETTLEMENT_MAX_RANGE_DAYS,
  MERCHANT_FINANCE_PAGE_SIZE,
  type MerchantFinanceRange,
  type MerchantFinanceSettlementPageView,
  type MerchantFinanceSettlementRowView,
  type MerchantFinanceSettlementSummaryView
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

const EMPTY_SUMMARY: MerchantFinanceSettlementSummaryView = {
  rangeLabel: '',
  settlementAmountText: '¥0.00',
  totalOrderAmountText: '¥0.00',
  deductionAmountText: '¥0.00',
  totalCountText: '0 笔'
}
const DEFAULT_RANGE = buildDefaultFinanceRange()

interface MerchantSettlementQuickRange {
  id: string
  label: string
  active: boolean
}

function buildQuickRanges(activeRange: MerchantFinanceRange): MerchantSettlementQuickRange[] {
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
    loadingSettlements: false,
    range: DEFAULT_RANGE as MerchantFinanceRange,
    quickRanges: buildQuickRanges(DEFAULT_RANGE) as MerchantSettlementQuickRange[],
    rangePickerVisible: false,
    rangePickerValue: getFinanceRangeCalendarValue(DEFAULT_RANGE),
    rangePickerMinDate: getFinanceDateTime(new Date(new Date().getFullYear() - 1, 0, 1)),
    rangePickerMaxDate: getFinanceDateTime(new Date()),
    rows: [] as MerchantFinanceSettlementRowView[],
    summary: EMPTY_SUMMARY,
    page: 1,
    totalPages: 0,
    hasMore: false,
    totalCount: 0
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    await this.loadSettlements()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  onPullDownRefresh() {
    void this.loadSettlements({ silent: true, page: 1 })
  },

  onReachBottom() {
    void this.onLoadMore()
  },

  async onLoadMore() {
    if (!this.data.hasMore || this.data.loadingSettlements) {
      return
    }
    await this.loadSettlements({
      silent: true,
      page: this.data.page + 1,
      append: true,
      range: this.data.range
    })
  },

  async loadSettlements(options: { silent?: boolean, page?: number, append?: boolean, range?: MerchantFinanceRange } = {}) {
    if (this.data.loadingSettlements) {
      wx.stopPullDownRefresh()
      return
    }

    const { silent = false, page = 1, append = false } = options
    const range = options.range || this.data.range
    const hasTrustedData = this.data.hasLoadedOnce
    this.setData({
      loadingSettlements: true,
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
      const view = await loadMerchantFinanceSettlementPage({
        page,
        limit: MERCHANT_FINANCE_PAGE_SIZE,
        range
      })
      this.applySettlementView(view, append, range)
    } catch (error) {
      logger.warn('Merchant settlements load failed', error, 'merchant-finance-settlements')
      const message = getErrorUserMessage(error, '结算记录加载失败，请稍后重试')
      this.setData({
        initialLoading: false,
        initialError: !hasTrustedData,
        initialErrorMessage: hasTrustedData ? '' : message,
        refreshErrorMessage: hasTrustedData ? message : '',
        loadingSettlements: false
      })
    } finally {
      wx.stopPullDownRefresh()
    }
  },

  applySettlementView(
    view: MerchantFinanceSettlementPageView,
    append: boolean,
    range: MerchantFinanceRange
  ) {
    const rows = append ? this.data.rows.concat(view.rows) : view.rows
    this.setData({
      rows,
      summary: view.summary,
      page: view.page,
      totalPages: view.totalPages,
      hasMore: view.hasMore,
      totalCount: view.totalCount,
      range,
      quickRanges: buildQuickRanges(range),
      rangePickerValue: getFinanceRangeCalendarValue(range),
      hasLoadedOnce: true,
      initialLoading: false,
      initialError: false,
      initialErrorMessage: '',
      refreshErrorMessage: '',
      loadingSettlements: false
    })
  },

  onRetry() {
    void this.loadSettlements({ page: 1, range: this.data.range })
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
      MERCHANT_FINANCE_SETTLEMENT_MAX_RANGE_DAYS,
      '结算记录'
    )
    if (!validation.valid) {
      wx.showToast({ title: validation.message || '结算记录最多选择365天', icon: 'none' })
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
    void this.loadSettlements({ page: 1, range })
  }
})
