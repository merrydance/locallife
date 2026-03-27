import { getStableBarHeights } from '../../../utils/responsive'
import { MerchantOrderManagementService, OrderManagementAdapter, OrderResponse } from '../../../api/order-management'
import { MerchantStatsService } from '../../../api/merchant-stats'
import { getUserInfo } from '../../../api/auth'
import { getMyMerchantOpenStatus, getMyMerchantProfile, updateMyMerchantOpenStatus } from '../../../api/merchant'
import { logger } from '../../../utils/logger'
import dayjs from 'dayjs'
import { wsManager, WSMessageType } from '../../../utils/websocket'
import { playNewOrderAlert, destroyAudioAlert } from '../../../utils/audio-alert'

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
    businessStatusSubmitting: false,
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
    // 页面重新可见时重新注册 WS 监听（onHide 已清除旧监听，底层连接仍在）
    this.initWebSocket()
  },

  onHide() {
    this.cleanupWebSocket()
  },

  onUnload() {
    this.cleanupWebSocket()
    destroyAudioAlert()
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
    // 先清除旧监听，再发起连接（保证不重复注册）
    this.cleanupWebSocket()
    wsManager.connect()

    const sub = wsManager.on(WSMessageType.NOTIFICATION, (data) => {
      logger.info('Merchant received notification', data)
      const notification =
        typeof data === 'object' && data !== null
          ? (data as { type?: string })
          : {}
      // 检查是否是订单通知 (后端 params.Type = "order")
      if (notification.type === 'order') {
        wx.vibrateLong()
        playNewOrderAlert()
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

      try {
        const [merchantProfile, merchantOpenStatus] = await Promise.all([
          getMyMerchantProfile(),
          getMyMerchantOpenStatus()
        ])

        this.setData({
          merchantInfo: {
            name: merchantProfile.name,
            merchant_id: merchantProfile.id
          },
          isOpen: merchantOpenStatus.is_open
        })

        try {
          const currentMerchant = wx.getStorageSync('current_merchant') || {}
          wx.setStorageSync('current_merchant', {
            ...currentMerchant,
            id: merchantProfile.id,
            merchant_id: merchantProfile.id,
            name: merchantProfile.name,
            is_open: merchantOpenStatus.is_open
          })
        } catch (storageErr) {
          logger.warn('Sync current merchant cache failed', storageErr)
        }
      } catch (merchantErr) {
        logger.error('Failed to fetch merchant runtime status', merchantErr)
      }

      try {
        const overview = await MerchantStatsService.getOverview({
          start_date: today,
          end_date: today
        })
        const orderCount = overview.total_orders || 0
        const revenue = overview.total_sales || 0
        this.setData({
          todayStats: {
            revenue,
            orderCount,
            avgOrderPrice: orderCount > 0 ? Math.round(revenue / orderCount) : 0
          }
        })
      } catch (overviewErr) {
        logger.error('Failed to fetch merchant overview', overviewErr)
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
      const result = await MerchantOrderManagementService.getOrderList({
        page_id: 1,
        page_size: 10,
        status
      })
      const orderList = result.orders || []
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

  onGoKitchen() {
    wx.navigateTo({ url: '/pages/merchant/kitchen/index' })
  },

  onGoStats() {
    wx.navigateTo({ url: '/pages/merchant/stats/index' })
  },

  onOrderTap(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    wx.navigateTo({ url: `/pages/merchant/orders/detail/index?id=${id}` })
  },

  async onToggleBusiness() {
    if (this.data.businessStatusSubmitting) return

    const targetOpen = !this.data.isOpen
    this.setData({ businessStatusSubmitting: true })

    try {
      const response = await updateMyMerchantOpenStatus(targetOpen)
      this.setData({ isOpen: response.is_open })
      wx.showToast({ title: response.message || (response.is_open ? '店铺营业中' : '店铺已打烊'), icon: 'success' })
      this.refreshData().catch((err) => logger.error('Refresh dashboard after toggle failed', err))
    } catch (err: unknown) {
      logger.error('Update merchant open status failed', err)
      let message = '营业状态更新失败'
      if (typeof err === 'object' && err !== null && 'userMessage' in err) {
        const userMessage = (err as { userMessage?: unknown }).userMessage
        if (typeof userMessage === 'string' && userMessage.trim()) {
          message = userMessage
        }
      }
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      this.setData({ businessStatusSubmitting: false })
    }
  },

  onGoToSettings() {
    wx.navigateTo({ url: '/pages/merchant/config/index' })
  }
})
