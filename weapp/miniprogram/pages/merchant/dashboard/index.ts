import { getStableBarHeights } from '../../../utils/responsive'
import { MerchantOrderManagementService, OrderManagementAdapter, OrderResponse } from '../../../api/order-management'
import { MerchantStatsService } from '../../../api/merchant-stats'
import { getUserInfo } from '../../../api/auth'
import { logger } from '../../../utils/logger'
import dayjs from 'dayjs'
import { wsManager, WSMessageType } from '../../../utils/websocket'

type WsUnsubscribe = () => void
type OrderStatusTab = 'paid' | 'preparing' | 'ready' | 'completed'

interface DashboardOrderItem extends OrderResponse {
  order_no_short: string
  order_type_label: string
  status_label: string
  status_color: string
  time_label: string
}

Page({
  data: {
    navBarHeight: 88,
    initialLoading: true,
    isOpen: true,
    merchantInfo: {
      name: '示例餐厅',
      merchant_id: 0
    },
    todayStats: {
      revenue: 0,
      orderCount: 0,
      avgOrderPrice: 0
    },
    currentOrderTab: 'paid' as OrderStatusTab,
    orderFlowLoading: false,
    orderFlow: [] as DashboardOrderItem[],
    loading: false,
    accessDenied: false,
    _wsListeners: [] as WsUnsubscribe[]
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    
    // 从全局或存储中获取商户信息
    const currentMerchant = wx.getStorageSync('current_merchant')
    if (currentMerchant) {
      this.setData({ merchantInfo: currentMerchant })
    }

    const hasAccess = await this.ensureMerchantAccess()
    if (!hasAccess) {
      this.setData({ accessDenied: true, initialLoading: false })
      return
    }

    this.initWebSocket()
  },

  onShow() {
    if (this.data.accessDenied) return
    this.refreshData()
  },

  onHide() {
    this.cleanupWebSocket()
  },

  onUnload() {
    this.cleanupWebSocket()
  },

  async ensureMerchantAccess() {
    try {
      const user = await getUserInfo()
      const normalizedRoles = (user.roles || []).map((role) => String(role).toLowerCase())
      const isMerchant = normalizedRoles.some((role) =>
        ['merchant', 'merchant_owner', 'merchant_boss', 'merchant_staff'].includes(role)
      )

      if (!isMerchant) {
        wx.showToast({ title: '当前账号无商户权限', icon: 'none' })
      }

      return isMerchant
    } catch (err) {
      logger.error('Check merchant access failed', err)
      wx.showToast({ title: '无法校验商户权限', icon: 'none' })
      return false
    }
  },

  initWebSocket() {
    wsManager.connect()
    this.cleanupWebSocket()

    const sub = wsManager.on(WSMessageType.NOTIFICATION, (data) => {
      logger.info('Merchant received notification', data)
      const notification =
        typeof data === 'object' && data !== null
          ? (data as { type?: string })
          : {}
      // 检查是否是订单通知 (后端 params.Type = "order")
      if (notification.type === 'order') {
        wx.vibrateLong()
        wx.showModal({
          title: '新订单提醒',
          content: '您有新的订单需要处理',
          confirmText: '去处理',
          success: (res) => {
            if (res.confirm) {
              this.onGoOrderList()
            }
          }
        })
        this.refreshData()
      }
    })

    this.data._wsListeners = [sub]
  },

  cleanupWebSocket() {
    if (this.data._wsListeners) {
      this.data._wsListeners.forEach((unsub) => unsub())
      this.data._wsListeners = []
    }
  },

  async refreshData() {
    if (this.data.loading) return
    this.setData({ loading: true })

    try {
      const today = dayjs().format('YYYY-MM-DD')

      const [overviewRes] = await Promise.allSettled([
        MerchantStatsService.getOverview({
          start_date: today,
          end_date: today
        })
      ])

      if (overviewRes.status === 'fulfilled') {
        const overview = overviewRes.value
        const orderCount = overview.total_orders || 0
        const revenue = overview.total_sales || 0
        this.setData({
          todayStats: {
            revenue,
            orderCount,
            avgOrderPrice: orderCount > 0 ? Math.round(revenue / orderCount) : 0
          }
        })
      } else {
        logger.error('Failed to fetch merchant overview', overviewRes.reason)
      }
      await this.loadOrderFlow(this.data.currentOrderTab)

    } catch (err) {
      logger.error('Merchant dashboard refresh failed', err)
    } finally {
      this.setData({ loading: false, initialLoading: false })
      wx.stopPullDownRefresh()
    }
  },

  onPullDownRefresh() {
    this.refreshData()
  },

  async loadOrderFlow(status: OrderStatusTab) {
    this.setData({ orderFlowLoading: true })
    try {
      const orders = await MerchantOrderManagementService.getOrderList({
        page_id: 1,
        page_size: 10,
        status
      })
      const orderList = Array.isArray(orders) ? orders : []
      const orderFlow = orderList.map((order) => ({
        ...order,
        order_no_short: order.order_no.slice(-6).toUpperCase(),
        order_type_label: OrderManagementAdapter.formatOrderType(order.order_type),
        status_label: OrderManagementAdapter.formatOrderStatus(order.status),
        status_color: OrderManagementAdapter.getStatusColor(order.status),
        time_label: dayjs(order.created_at).format('HH:mm')
      }))
      this.setData({ orderFlow })
    } catch (err) {
      logger.error('Load dashboard order flow failed', err)
      this.setData({ orderFlow: [] })
    } finally {
      this.setData({ orderFlowLoading: false })
    }
  },

  onOrderTabChange(e: WechatMiniprogram.CustomEvent<{ value: OrderStatusTab }>) {
    const value = e.detail.value
    this.setData({ currentOrderTab: value })
    this.loadOrderFlow(value)
  },

  onGoOrderList() {
    wx.navigateTo({ url: `/pages/merchant/orders/list/index?status=${this.data.currentOrderTab}` })
  },

  onOrderTap(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    wx.navigateTo({ url: `/pages/merchant/orders/detail/index?id=${id}` })
  },

  onToggleBusiness() {
    this.setData({ isOpen: !this.data.isOpen })
    wx.showToast({
      title: this.data.isOpen ? '营业中' : '休息中',
      icon: 'success'
    })
  },

  onGoToSettings() {
    wx.navigateTo({ url: '/pages/merchant/config/index' })
  }
})
