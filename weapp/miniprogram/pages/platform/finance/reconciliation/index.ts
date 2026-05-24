import {
  buildPlatformReconciliationMonthRange,
  buildPlatformReconciliationRange,
  loadPlatformFinanceReconciliationPage,
  type PlatformFinanceReconciliationRange,
  type PlatformFinanceReconciliationPageView
} from '../../../../services/platform-finance-reconciliation'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../utils/user-facing'

const EMPTY_VIEW: PlatformFinanceReconciliationPageView = {
  rangeLabel: '',
  summary: {
    totalOrdersText: '0 单',
    totalProfitSharingAmountText: '¥0.00',
    platformCommissionText: '¥0.00',
    operatorCommissionText: '¥0.00',
    paidAmountText: '¥0.00',
    merchantAmountText: '¥0.00',
    riderAmountText: '¥0.00',
    withdrawSucceededText: '¥0.00',
    withdrawProcessingText: '¥0.00',
    exceptionCountText: '0 项',
    currentAvailableAmountText: '--',
    currentPendingAmountText: '--',
    currentLedgerAmountText: '--',
    currentFrozenAmountText: '--',
    balanceStatusText: '当前余额暂不可确认',
    balanceUnavailable: true
  },
  metrics: [],
  statusRows: [],
  dailyRows: []
}
const DEFAULT_RANGE = buildPlatformReconciliationRange()

interface PlatformReconciliationQuickRange {
  id: string
  label: string
  active: boolean
}

interface LoadReconciliationOptions {
  silent?: boolean
  range?: PlatformFinanceReconciliationRange
}

function parseDateValue(value?: string): Date | null {
  if (!value) {
    return null
  }
  const normalized = value.replace(/-/g, '/')
  const date = new Date(normalized)
  if (Number.isNaN(date.getTime())) {
    return null
  }
  return date
}

function formatDate(date: Date): string {
  const year = date.getFullYear()
  const month = `${date.getMonth() + 1}`.padStart(2, '0')
  const day = `${date.getDate()}`.padStart(2, '0')
  return `${year}-${month}-${day}`
}

function getDateTime(date: Date): number {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime()
}

function getRangeCalendarValue(range: PlatformFinanceReconciliationRange): number[] {
  const start = parseDateValue(range.start_date)
  const end = parseDateValue(range.end_date)
  if (!start || !end) {
    return []
  }
  return [getDateTime(start), getDateTime(end)]
}

function buildQuickRanges(activeRange: PlatformFinanceReconciliationRange): PlatformReconciliationQuickRange[] {
  const rangeOptions = [
    { id: '7', label: '近7天', range: buildPlatformReconciliationRange(7) },
    { id: '30', label: '近30天', range: buildPlatformReconciliationRange(30) },
    { id: 'month', label: '本月', range: buildPlatformReconciliationMonthRange() }
  ]
  return rangeOptions.map((option) => ({
    id: option.id,
    label: option.label,
    active: option.range.start_date === activeRange.start_date && option.range.end_date === activeRange.end_date
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
    loadingReconciliation: false,
    range: DEFAULT_RANGE as PlatformFinanceReconciliationRange,
    quickRanges: buildQuickRanges(DEFAULT_RANGE) as PlatformReconciliationQuickRange[],
    rangePickerVisible: false,
    rangePickerValue: getRangeCalendarValue(DEFAULT_RANGE),
    rangePickerMinDate: getDateTime(new Date(new Date().getFullYear() - 1, 0, 1)),
    rangePickerMaxDate: getDateTime(new Date()),
    view: EMPTY_VIEW
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    await this.loadReconciliation()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  onPullDownRefresh() {
    void this.loadReconciliation({ silent: true })
  },

  async loadReconciliation(options: LoadReconciliationOptions = {}) {
    if (this.data.loadingReconciliation) {
      wx.stopPullDownRefresh()
      return
    }

    const { silent = false } = options
    const range = options.range || this.data.range
    const hasTrustedData = this.data.hasLoadedOnce

    this.setData({
      loadingReconciliation: true,
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
      const view = await loadPlatformFinanceReconciliationPage(range)
      this.setData({
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        hasLoadedOnce: true,
        loadingReconciliation: false,
        range,
        quickRanges: buildQuickRanges(range),
        rangePickerValue: getRangeCalendarValue(range),
        view
      })
    } catch (error) {
      logger.warn('Platform finance reconciliation load failed', error, 'platform-finance-reconciliation')
      const message = getErrorUserMessage(error, '对账账单加载失败，请稍后重试')
      this.setData({
        initialLoading: false,
        initialError: !hasTrustedData,
        initialErrorMessage: hasTrustedData ? '' : message,
        refreshErrorMessage: hasTrustedData ? message : '',
        loadingReconciliation: false
      })
    } finally {
      wx.stopPullDownRefresh()
    }
  },

  onRetry() {
    void this.loadReconciliation()
  },

  onOpenRangePicker() {
    this.setData({
      rangePickerVisible: true,
      rangePickerValue: getRangeCalendarValue(this.data.range)
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

    this.applyRange({
      start_date: formatDate(start),
      end_date: formatDate(end)
    })
  },

  onUseQuickRange(e: WechatMiniprogram.BaseEvent) {
    const rangeID = String(e.currentTarget.dataset.range || '')
    if (rangeID === '7') {
      this.applyRange(buildPlatformReconciliationRange(7))
      return
    }
    if (rangeID === '30') {
      this.applyRange(buildPlatformReconciliationRange(30))
      return
    }
    if (rangeID === 'month') {
      this.applyRange(buildPlatformReconciliationMonthRange())
    }
  },

  applyRange(range: PlatformFinanceReconciliationRange) {
    this.setData({
      rangePickerVisible: false
    })
    void this.loadReconciliation({ range })
  }
})
