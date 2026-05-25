import { Delivery, getDeliveryStatusDisplay } from '../../../api/delivery'
import { deliveryTaskManagementService, type DeliveryHistoryParams } from '../../../api/delivery-task-management'
import { buildRiderDeliveryIncomeView, RiderDeliveryIncomeView } from '../../../utils/rider-delivery-income-view'
import { logger } from '../../../utils/logger'
import { locationService } from '../../../utils/location'
import { getStableBarHeights } from '../../../utils/responsive'

const PAGE_SIZE = 20
const DEFAULT_HISTORY_DAYS = 30
const DATE_PICKER_MIN_YEAR = 2020

let historyRequestSeq = 0

type DeliveryHistoryView = Delivery & {
  display_time: string
  status_text: string
  status_theme: 'success' | 'warning' | 'danger' | 'primary' | 'default'
  income_view: RiderDeliveryIncomeView
}

interface DeliveryHistoryDateRange {
  start_date: string
  end_date: string
}

interface DeliveryHistoryResponse {
  deliveries?: Delivery[]
  total_earnings?: number
  completed_total?: number
  total?: number
  page_id?: number
  page_size?: number
}

interface UserMessageError {
  userMessage?: string
}

function parseDateValue(value?: string): Date | null {
  if (!value) {
    return null
  }

  const date = new Date(value.replace(/-/g, '/'))
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

function buildDefaultDateRange(days = DEFAULT_HISTORY_DAYS): DeliveryHistoryDateRange {
  const end = new Date()
  const start = new Date(end)
  start.setDate(end.getDate() - Math.max(1, days - 1))
  return {
    start_date: formatDate(start),
    end_date: formatDate(end)
  }
}

function hasCompleteDateRange(range: DeliveryHistoryDateRange): boolean {
  return Boolean(range.start_date && range.end_date)
}

function getRangeCalendarValue(range: DeliveryHistoryDateRange): number[] {
  const start = parseDateValue(range.start_date)
  const end = parseDateValue(range.end_date)
  if (!start || !end) {
    return []
  }
  return [getDateTime(start), getDateTime(end)]
}

function decorateHistoryDelivery(delivery: Delivery): DeliveryHistoryView {
  const statusMeta = getDeliveryStatusDisplay(delivery.status)
  return {
    ...delivery,
    display_time: delivery.completed_at || delivery.delivered_at || delivery.created_at || '',
    status_text: statusMeta.text,
    status_theme: statusMeta.theme,
    income_view: buildRiderDeliveryIncomeView(delivery)
  }
}

const DEFAULT_DATE_RANGE = buildDefaultDateRange()

Page({
  data: {
    navBarHeight: 88,
    loading: false,
    loadingMore: false,
    errorMessage: '',
    refreshErrorMessage: '',
    loadMoreError: '',
    deliveries: [] as DeliveryHistoryView[],
    pageID: 1,
    hasMore: true,
    totalCount: 0,
    dateRange: DEFAULT_DATE_RANGE as DeliveryHistoryDateRange,
    rangePickerVisible: false,
    rangePickerValue: getRangeCalendarValue(DEFAULT_DATE_RANGE),
    rangePickerMinDate: getDateTime(new Date(DATE_PICKER_MIN_YEAR, 0, 1)),
    rangePickerMaxDate: getDateTime(new Date())
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.fetchHistory(1, true)
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  buildHistoryParams(page: number): DeliveryHistoryParams {
    const params: DeliveryHistoryParams = {
      page_id: page,
      page_size: PAGE_SIZE
    }
    const range = this.data.dateRange as DeliveryHistoryDateRange
    if (hasCompleteDateRange(range)) {
      params.start_date = range.start_date
      params.end_date = range.end_date
    }
    return params
  },

  async fetchHistory(page: number = 1, reset: boolean = false) {
    if (!reset && (this.data.loading || this.data.loadingMore)) return

    const requestSeq = ++historyRequestSeq
    this.setData(reset
      ? { loading: true, refreshErrorMessage: '', loadMoreError: '', errorMessage: '' }
      : { loadingMore: true, loadMoreError: '' })

    try {
      const resp = await deliveryTaskManagementService.getDeliveryHistory(this.buildHistoryParams(page)) as DeliveryHistoryResponse
      if (requestSeq !== historyRequestSeq) return

      const list = (resp.deliveries || []).map(decorateHistoryDelivery)
      const total = resp.total || 0
      const pageID = resp.page_id || page
      const pageSize = resp.page_size || PAGE_SIZE
      this.setData({
        deliveries: reset ? list : [...this.data.deliveries, ...list],
        hasMore: pageID * pageSize < total,
        totalCount: total,
        pageID,
        errorMessage: '',
        refreshErrorMessage: '',
        loadMoreError: ''
      })
    } catch (err: unknown) {
      if (requestSeq !== historyRequestSeq) return
      logger.error('Fetch delivery history failed', err)
      const userMessage = (err as UserMessageError).userMessage
      const message = typeof userMessage === 'string' && userMessage ? userMessage : '历史任务加载失败，请稍后重试'
      if (reset) {
        if (this.data.deliveries.length > 0) {
          this.setData({ refreshErrorMessage: `${message}，当前已保留上次任务记录`, errorMessage: '', loadMoreError: '' })
        } else {
          this.setData({ errorMessage: message, refreshErrorMessage: '', loadMoreError: '', deliveries: [], hasMore: true, totalCount: 0 })
        }
      } else {
        this.setData({ loadMoreError: message })
      }
    } finally {
      if (requestSeq === historyRequestSeq) {
        this.setData({ loading: false, loadingMore: false })
      }
    }
  },

  onReachBottom() {
    if (this.data.hasMore && !this.data.loading && !this.data.loadingMore) {
      this.fetchHistory(this.data.pageID + 1)
    }
  },

  onRetry() {
    this.fetchHistory(1, true)
  },

  onRetryLoadMore() {
    this.fetchHistory(this.data.pageID + 1, false)
  },

  onOpenRangePicker() {
    this.setData({
      rangePickerVisible: true,
      rangePickerValue: getRangeCalendarValue(this.data.dateRange as DeliveryHistoryDateRange)
    })
  },

  onCloseRangePicker() {
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

    this.applyDateRange({
      start_date: formatDate(start),
      end_date: formatDate(end)
    })
  },

  applyDateRange(range: DeliveryHistoryDateRange) {
    this.setData({
      dateRange: range,
      rangePickerVisible: false,
      rangePickerValue: getRangeCalendarValue(range),
      deliveries: [],
      pageID: 1,
      hasMore: true,
      totalCount: 0,
      errorMessage: '',
      refreshErrorMessage: '',
      loadMoreError: ''
    })
    this.fetchHistory(1, true)
  },

  onGoToDetail(e: WechatMiniprogram.TouchEvent) {
    const { orderId } = e.currentTarget.dataset as { orderId?: number }
    if (!orderId) return
    wx.navigateTo({
      url: `/pages/rider/task-detail/index?id=${orderId}`
    })
  },

  async onOpenLocation(e: WechatMiniprogram.TouchEvent) {
    const {
      latitude,
      longitude,
      name,
      address,
      label
    } = e.currentTarget.dataset as {
      latitude?: number
      longitude?: number
      name?: string
      address?: string
      label?: string
    }

    await locationService.openLocation({
      latitude,
      longitude,
      name,
      address,
      failMessage: `打开${label || '导航'}失败，请稍后重试`
    })
  }
})
