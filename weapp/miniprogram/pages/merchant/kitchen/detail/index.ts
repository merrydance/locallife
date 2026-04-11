import dayjs from 'dayjs'
import { KitchenDisplayService, KitchenOrderItem, KitchenOrderResponse, OrderManagementAdapter } from '../../../../api/order-management'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../utils/user-facing'

interface KitchenDetailOptions {
  id?: string
}

type KitchenDetailAction = '' | 'preparing' | 'ready'
type KitchenStatusTheme = 'primary' | 'warning' | 'success'

interface KitchenDetailItemView extends KitchenOrderItem {
  categoryLabel: string
  prepareTimeLabel: string
  customizationSummary: string
  hasImage: boolean
}

interface KitchenDetailView extends KitchenOrderResponse {
  orderNoShort: string
  orderTypeLabel: string
  statusLabel: string
  statusTheme: KitchenStatusTheme
  waitingLabel: string
  remainingLabel: string
  seatOrPickupLabel: string
  estimatedReadyLabel: string
  createdAtLabel: string
  paidAtLabel: string
  preparingStartedAtLabel: string
  readyAtLabel: string
  noteLabel: string
  urgencyLabel: string
  itemCount: number
  totalQuantity: number
  progressCurrent: number
  canStartPreparing: boolean
  canMarkReady: boolean
  items: KitchenDetailItemView[]
}

function formatTime(value?: string) {
  if (!value) return '暂无'
  const parsed = dayjs(value)
  return parsed.isValid() ? parsed.format('YYYY-MM-DD HH:mm') : value
}

function getKitchenStatusLabel(status: string) {
  const map: Record<string, string> = {
    paid: '待制作',
    preparing: '制作中',
    ready: '待取餐'
  }
  return map[status] || '状态同步中'
}

function getKitchenStatusTheme(status: string): KitchenStatusTheme {
  if (status === 'ready') return 'success'
  if (status === 'preparing') return 'warning'
  return 'primary'
}

function formatKitchenItem(item: KitchenOrderItem): KitchenDetailItemView {
  const customizationSummary = Array.isArray(item.customizations) && item.customizations.length
    ? item.customizations.map((option) => option.option_name).join('、')
    : ''

  return {
    ...item,
    categoryLabel: item.category_name || '未分类商品',
    prepareTimeLabel: `${item.prepare_time || 0} 分钟`,
    customizationSummary,
    hasImage: Boolean(item.image_url)
  }
}

function buildKitchenDetailView(order: KitchenOrderResponse): KitchenDetailView {
  const remainingMinutes = Math.round(OrderManagementAdapter.getRemainingTime(order))
  const seatOrPickupLabel = order.table_number || order.table_no
    ? `${order.table_number || order.table_no}号桌`
    : order.pickup_number
      ? `取餐号 ${order.pickup_number}`
      : order.customer_name || '现场订单'
  const items = (order.items || []).map(formatKitchenItem)
  const totalQuantity = items.reduce((sum, item) => sum + (item.quantity || 0), 0)

  return {
    ...order,
    items,
    orderNoShort: order.order_no.slice(-6).toUpperCase(),
    orderTypeLabel: OrderManagementAdapter.formatOrderType(order.order_type),
    statusLabel: getKitchenStatusLabel(order.status),
    statusTheme: getKitchenStatusTheme(order.status),
    waitingLabel: `${order.waiting_minutes || 0} 分钟`,
    remainingLabel: remainingMinutes > 0 ? `预计还需 ${remainingMinutes} 分钟` : '请优先处理',
    seatOrPickupLabel,
    estimatedReadyLabel: order.estimated_ready_at ? formatTime(order.estimated_ready_at) : '暂无',
    createdAtLabel: formatTime(order.created_at),
    paidAtLabel: formatTime(order.paid_at),
    preparingStartedAtLabel: formatTime(order.preparing_started_at),
    readyAtLabel: formatTime(order.ready_at),
    noteLabel: order.notes || '无备注',
    urgencyLabel: order.is_urged ? '顾客已催单，请优先处理' : '当前暂无催单提醒',
    itemCount: items.length,
    totalQuantity,
    progressCurrent: order.status === 'ready' ? 2 : order.status === 'preparing' ? 1 : 0,
    canStartPreparing: order.status === 'paid',
    canMarkReady: order.status === 'preparing'
  }
}

const getErrorMessage = getErrorUserMessage

Page({
  data: {
    navBarHeight: 88,
    orderId: 0,
    loading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    actionNoticeMessage: '',
    actionLoading: false,
    actionType: '' as KitchenDetailAction,
    detail: null as KitchenDetailView | null
  },

  onLoad(options: KitchenDetailOptions) {
    const { navBarHeight } = getStableBarHeights()
    const orderId = Number(options.id || 0)
    this.setData({ navBarHeight, orderId })

    if (!orderId) {
      this.setData({
        loading: false,
        initialError: true,
        initialErrorMessage: '缺少订单 ID，无法查看后厨详情'
      })
      return
    }

    this.loadDetail()
  },

  onShow() {
    if (this.data.orderId && this.data.detail && !this.data.loading && !this.data.actionLoading) {
      this.loadDetail(true, true)
    }
  },

  onPullDownRefresh() {
    this.loadDetail(Boolean(this.data.detail), true)
  },

  onRetry() {
    this.loadDetail(false)
  },

  onRetryRefresh() {
    this.loadDetail(true, true)
  },

  onViewBoard() {
    wx.navigateBack()
  },

  async loadDetail(silent = false, preserveActionNotice = false) {
    if (!silent) {
      this.setData({
        loading: true,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        actionNoticeMessage: ''
      })
    } else {
      this.setData({ refreshErrorMessage: '' })
    }

    try {
      const detail = await KitchenDisplayService.getKitchenOrderDetail(this.data.orderId)
      this.setData({
        detail: buildKitchenDetailView(detail),
        loading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        actionNoticeMessage: preserveActionNotice ? this.data.actionNoticeMessage : ''
      })
    } catch (err) {
      logger.error('Load kitchen order detail failed', err)
      const message = getErrorMessage(err, '后厨订单详情加载失败，请稍后重试')
      if (!this.data.detail || !silent) {
        this.setData({
          loading: false,
          initialError: true,
          initialErrorMessage: message,
          refreshErrorMessage: '',
          actionNoticeMessage: '',
          detail: null
        })
      } else {
        this.setData({
          loading: false,
          refreshErrorMessage: `${message}，当前已保留上次同步结果`
        })
      }
    } finally {
      wx.stopPullDownRefresh()
    }
  },

  async onStartPreparing() {
    if (!this.data.orderId || !this.data.detail?.canStartPreparing) return
    await this.runKitchenAction('preparing', KitchenDisplayService.startPreparing(this.data.orderId))
  },

  async onMarkReady() {
    if (!this.data.orderId || !this.data.detail?.canMarkReady) return
    await this.runKitchenAction('ready', KitchenDisplayService.markKitchenOrderReady(this.data.orderId))
  },

  async runKitchenAction(actionType: KitchenDetailAction, requestPromise: Promise<KitchenOrderResponse>) {
    if (this.data.actionLoading) return

    this.setData({ actionLoading: true, actionType })
    try {
      const detail = await requestPromise
      this.setData({
        detail: buildKitchenDetailView(detail),
        refreshErrorMessage: '',
        actionNoticeMessage: actionType === 'preparing' ? '订单已进入制作中，后厨状态已同步。' : '订单已标记出餐，可继续关注取餐状态。'
      })
      await this.loadDetail(true, true)
    } catch (err) {
      logger.error('Kitchen detail action failed', err)
      wx.showToast({ title: getErrorMessage(err, '操作失败，请稍后重试'), icon: 'none' })
    } finally {
      this.setData({ actionLoading: false, actionType: '' })
    }
  }
})