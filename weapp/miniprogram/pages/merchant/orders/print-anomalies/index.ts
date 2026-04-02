import {
  MerchantOrderManagementService,
  MerchantPrintAnomalyItem,
  OrderManagementAdapter
} from '../../../../api/order-management'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../utils/user-facing'
import dayjs from 'dayjs'

type PrintAnomalyFilter = 'all' | 'failed' | 'pending'
type PrintAnomalyTheme = 'warning' | 'danger' | 'default'

interface PrintAnomalyView extends MerchantPrintAnomalyItem {
  status_label: string
  status_theme: PrintAnomalyTheme
  order_type_label: string
  last_attempt_label: string
  summary: string
  retry_hint_label: string
  error_message_label: string
  vendor_order_id_label: string
}

function formatRetryHint(retryHint?: string) {
  if (!retryHint) return ''
  if (retryHint === 'printer is inactive') {
    return '该打印机当前已停用，请先启用打印机后再重试。'
  }
  if (retryHint === 'printer type is not supported for retry') {
    return '当前打印机类型暂不支持小程序端重试，请到设备配置中核对品牌类型。'
  }
  return retryHint
}

const FILTER_OPTIONS: Array<{ key: PrintAnomalyFilter, label: string }> = [
  { key: 'all', label: '全部异常' },
  { key: 'failed', label: '打印失败' },
  { key: 'pending', label: '待回执' }
]

function formatPrintAnomalyStatus(status: string) {
  if (status === 'failed') return '打印失败'
  if (status === 'pending') return '待回执'
  return '状态同步中'
}

function getPrintAnomalyTheme(status: string): PrintAnomalyTheme {
  if (status === 'failed') return 'danger'
  if (status === 'pending') return 'warning'
  return 'default'
}

function getPrintAnomalySummary(item: MerchantPrintAnomalyItem) {
  if (item.retry_hint) {
    return formatRetryHint(item.retry_hint)
  }
  if (item.error_message && item.local_status === 'failed') {
    return '最近一次打印已明确失败，请先查看失败原因，再决定是否重试补打。'
  }
  if (item.local_status === 'pending') {
    return '打印任务仍未收到回执，请确认门店设备和云打印平台状态。'
  }
  return '打印任务状态异常，请尽快处理。'
}

function buildPrintAnomalyView(item: MerchantPrintAnomalyItem): PrintAnomalyView {
  return {
    ...item,
    status_label: formatPrintAnomalyStatus(item.local_status),
    status_theme: getPrintAnomalyTheme(item.local_status),
    order_type_label: OrderManagementAdapter.formatOrderType(item.order_type),
    last_attempt_label: dayjs(item.last_attempt_at).format('MM-DD HH:mm'),
    summary: getPrintAnomalySummary(item),
    retry_hint_label: formatRetryHint(item.retry_hint),
    error_message_label: item.error_message || '',
    vendor_order_id_label: item.vendor_order_id || ''
  }
}

const getErrorMessage = getErrorUserMessage

Page({
  data: {
    navBarHeight: 88,
    filterOptions: FILTER_OPTIONS,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    loading: false,
    retryingPrintLogId: 0,
    currentFilter: 'all' as PrintAnomalyFilter,
    anomalies: [] as PrintAnomalyView[],
    pageId: 1,
    pageSize: 20,
    hasMore: true
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.loadAnomalies(true)
  },

  onShow() {
    if (!this.data.initialLoading && !this.data.loading) {
      this.loadAnomalies(true, false)
    }
  },

  onPullDownRefresh() {
    this.loadAnomalies(true, false)
  },

  onReachBottom() {
    this.loadAnomalies(false)
  },

  async loadAnomalies(reset = false, showLoading = true) {
    if (this.data.loading) return
    if (!reset && !this.data.hasMore) return

    const hasExistingData = this.data.anomalies.length > 0
    const isSilentRefresh = reset && !showLoading && hasExistingData

    this.setData({
      loading: true,
      ...(showLoading
        ? { initialError: false, initialErrorMessage: '', refreshErrorMessage: '' }
        : isSilentRefresh
          ? { refreshErrorMessage: '' }
          : {})
    })

    try {
      const pageId = reset ? 1 : this.data.pageId
      const response = await MerchantOrderManagementService.listPrintAnomalies({
        page_id: pageId,
        page_size: this.data.pageSize,
        status: this.data.currentFilter === 'all' ? undefined : this.data.currentFilter
      })

      const items = Array.isArray(response.items) ? response.items.map(buildPrintAnomalyView) : []
      this.setData({
        anomalies: reset ? items : [...this.data.anomalies, ...items],
        pageId: pageId + 1,
        hasMore: pageId * this.data.pageSize < (response.total || 0),
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: ''
      })
    } catch (err: unknown) {
      logger.error('Load merchant print anomalies failed', err)
      const message = getErrorMessage(err, '打印异常加载失败，请稍后重试')

      if (this.data.initialLoading) {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message
        })
      } else if (isSilentRefresh) {
        this.setData({ refreshErrorMessage: `${message}，当前已保留上次同步结果` })
      } else {
        wx.showToast({ title: message, icon: 'none' })
      }
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  onRetry() {
    this.loadAnomalies(true)
  },

  onRetryRefresh() {
    this.loadAnomalies(true, false)
  },

  onChangeFilter(e: WechatMiniprogram.TouchEvent) {
    const { key } = e.currentTarget.dataset as { key?: PrintAnomalyFilter }
    if (!key || key === this.data.currentFilter) return

    this.setData({
      currentFilter: key,
      anomalies: [],
      pageId: 1,
      hasMore: true,
      refreshErrorMessage: ''
    }, () => {
      this.loadAnomalies(true)
    })
  },

  onViewOrder(e: WechatMiniprogram.TouchEvent) {
    const { orderId } = e.currentTarget.dataset as { orderId?: number }
    if (!orderId) return
    wx.navigateTo({ url: `/pages/merchant/orders/detail/index?id=${orderId}` })
  },

  async onRetryPrint(e: WechatMiniprogram.TouchEvent) {
    const { orderId, printLogId, printerName } = e.currentTarget.dataset as {
      orderId?: number
      printLogId?: number
      printerName?: string
    }
    if (!orderId || !printLogId || this.data.retryingPrintLogId) return

    wx.showModal({
      title: '重试打印',
      content: `重新向打印机「${printerName || '未命名设备'}」下发该异常打印任务？`,
      confirmText: '立即重试',
      cancelText: '取消',
      success: async (res) => {
        if (!res.confirm || this.data.retryingPrintLogId) return

        this.setData({ retryingPrintLogId: printLogId })
        try {
          await MerchantOrderManagementService.retryOrderPrintJob(orderId, printLogId)
          await this.loadAnomalies(true, false)
        } catch (err: unknown) {
          logger.error('Retry merchant print anomaly failed', err)
          wx.showToast({ title: getErrorMessage(err, '重试失败，请稍后重试'), icon: 'none' })
        } finally {
          this.setData({ retryingPrintLogId: 0 })
        }
      }
    })
  }
})