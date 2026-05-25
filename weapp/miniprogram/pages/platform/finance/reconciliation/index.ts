import {
  buildPlatformReconciliationRange,
  loadPlatformFinanceReconciliationDetailsPage,
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
    merchantFlowText: '¥0.00',
    riderFlowText: '¥0.00',
    platformCommissionText: '¥0.00',
    operatorCommissionText: '¥0.00',
    merchantShareText: '¥0.00',
    riderShareText: '¥0.00'
  },
  summaryCards: [],
  detailRows: [],
  detailsTotal: 0,
  detailsTotalText: '共 0 条',
  detailsPageId: 1,
  detailsPageSize: 20,
  detailsHasMore: false
}
const DEFAULT_RANGE = buildPlatformReconciliationRange()
let detailsRequestSeq = 0

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

Page({
  data: {
    navBarHeight: 88,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    hasLoadedOnce: false,
    loadingReconciliation: false,
    loadingDetails: false,
    detailsErrorMessage: '',
    loadMoreDetailsErrorMessage: '',
    range: DEFAULT_RANGE as PlatformFinanceReconciliationRange,
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

    detailsRequestSeq += 1
    this.setData({
      loadingReconciliation: true,
      loadingDetails: false,
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
        loadingDetails: false,
        detailsErrorMessage: '',
        loadMoreDetailsErrorMessage: '',
        range,
        rangePickerValue: getRangeCalendarValue(range),
        view
      })
      void this.loadDetailsPage(1, true)
    } catch (error) {
      logger.warn('Platform finance reconciliation load failed', error, 'platform-finance-reconciliation')
      const message = getErrorUserMessage(error, '对账账单加载失败，请稍后重试')
      this.setData({
        initialLoading: false,
        initialError: !hasTrustedData,
        initialErrorMessage: hasTrustedData ? '' : message,
        refreshErrorMessage: hasTrustedData ? message : '',
        loadingReconciliation: false,
        loadingDetails: false
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

  applyRange(range: PlatformFinanceReconciliationRange) {
    this.setData({
      rangePickerVisible: false,
      detailsErrorMessage: '',
      loadMoreDetailsErrorMessage: ''
    })
    void this.loadReconciliation({ range })
  },

  async loadDetailsPage(pageId: number, reset: boolean) {
    if (!reset && this.data.loadingDetails) {
      return
    }
    if (!reset && !this.data.view.detailsHasMore) {
      return
    }

    const requestSeq = ++detailsRequestSeq
    this.setData(reset
      ? { loadingDetails: true, detailsErrorMessage: '', loadMoreDetailsErrorMessage: '' }
      : { loadingDetails: true, loadMoreDetailsErrorMessage: '' })

    try {
      const detailPage = await loadPlatformFinanceReconciliationDetailsPage({
        range: this.data.range,
        pageId,
        pageSize: this.data.view.detailsPageSize
      })
      if (requestSeq !== detailsRequestSeq) {
        return
      }
      this.setData({
        view: {
          ...this.data.view,
          detailRows: reset
            ? detailPage.detailRows
            : this.data.view.detailRows.concat(detailPage.detailRows),
          detailsTotal: detailPage.detailsTotal,
          detailsTotalText: detailPage.detailsTotalText,
          detailsPageId: detailPage.detailsPageId,
          detailsPageSize: detailPage.detailsPageSize,
          detailsHasMore: detailPage.detailsHasMore
        },
        detailsErrorMessage: '',
        loadMoreDetailsErrorMessage: ''
      })
    } catch (error) {
      if (requestSeq !== detailsRequestSeq) {
        return
      }
      logger.warn('Platform finance reconciliation details load failed', error, 'platform-finance-reconciliation')
      const message = getErrorUserMessage(error, '分账明细加载失败，请稍后重试')
      if (reset) {
        this.setData({
          detailsErrorMessage: message,
          view: {
            ...this.data.view,
            detailRows: [],
            detailsTotal: 0,
            detailsTotalText: '共 0 条',
            detailsPageId: 1,
            detailsHasMore: false
          }
        })
      } else {
        this.setData({ loadMoreDetailsErrorMessage: message })
      }
    } finally {
      if (requestSeq === detailsRequestSeq) {
        this.setData({ loadingDetails: false })
      }
    }
  },

  onLoadMoreDetails() {
    void this.loadDetailsPage(this.data.view.detailsPageId + 1, false)
  },

  onRetryDetails() {
    void this.loadDetailsPage(1, true)
  }
})
