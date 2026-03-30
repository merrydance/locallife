import RiderService, { RiderInfo, RiderStatus } from '../../../api/rider'
import DeliveryService, { RecommendedOrder, Delivery } from '../../../api/delivery'
import { logger } from '../../../utils/logger'
import { locationService } from '../../../utils/location'
import { normalizeLocationError, syncRiderDeliveryLocation } from '../../../utils/rider-location'
import { getStableBarHeights } from '../../../utils/responsive'
import { wsManager, WSMessageType } from '../../../utils/websocket'
import { networkMonitor } from '../../../utils/network-monitor'
import { request } from '../../../utils/request'

const MAX_GRAB_DISTANCE = 5000 // 最大抢单距离 5km

let runtimeNetworkUnsubscribe: null | (() => void) = null

type WsUnsubscribe = () => void
type DeliveryActionType = 'startPickup' | 'confirmPickup' | 'startDelivery' | 'confirmDelivery'
type DeliveryActionMethod = (deliveryId: number) => Promise<Delivery>

interface UserMessageError {
  userMessage?: string
}

interface DeliveryActionConfig {
  method: DeliveryActionMethod
  loading: string
  source: string
}

/**
 * 计算两个经纬度之间的距离（单位：米）
 */
function getDistance(lat1: number, lng1: number, lat2: number, lng2: number): number {
  const R = 6371e3 // 地球半径
  const φ1 = lat1 * Math.PI / 180
  const φ2 = lat2 * Math.PI / 180
  const Δφ = (lat2 - lat1) * Math.PI / 180
  const Δλ = (lng2 - lng1) * Math.PI / 180

  const a = Math.sin(Δφ / 2) * Math.sin(Δφ / 2) +
    Math.cos(φ1) * Math.cos(φ2) *
    Math.sin(Δλ / 2) * Math.sin(Δλ / 2)
  const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a))

  return R * c
}

Page({
  data: {
    // UI 状态
    navBarHeight: 88,
    activeTab: 'hall', // hall: 抢单大厅, my: 我的配送
    loading: false,
    onlineSwitchLoading: false,
    initError: '',
    loadError: '',
    isRefresherTriggered: false,

    // 骑手基础信息
    riderInfo: null as RiderInfo | null,
    riderStatus: null as RiderStatus | null,
    isOnline: false,

    // 数据列表
    recommendOrders: [] as RecommendedOrder[], // 待抢订单
    activeDeliveries: [] as Delivery[],       // 当前配送中
    stats: {
      todayCount: 0,
      todayEarnings: 0,
      creditScore: 0
    },
    
    // 实时数据补充
    newOrdersCount: 0, 
    _wsListeners: [] as WsUnsubscribe[]
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.bindNetworkMonitor()
    this.initData().catch((err) => logger.error('Load init error', err))
  },

  onShow() {
    this.bindNetworkMonitor()
    this.enterOnlineRuntime().catch((err) => logger.error('Show refresh error', err))
  },

  onHide() {
    this.cleanupWebSocket()
    this.unbindNetworkMonitor()
  },

  onUnload() {
    this.cleanupWebSocket()
    this.unbindNetworkMonitor()
  },

  bindNetworkMonitor() {
    if (runtimeNetworkUnsubscribe) return

    runtimeNetworkUnsubscribe = networkMonitor.subscribe((state) => {
      if (!state.isConnected) {
        if (this.data.isOnline && this.data.recommendOrders.length === 0 && this.data.activeDeliveries.length === 0) {
          this.setData({ loadError: '网络已断开，请恢复后重试' })
        }
        return
      }

      if (this.data.loading) return

      if (this.data.initError || !this.data.riderInfo || !this.data.riderStatus) {
        this.initData().catch((err) => logger.error('Network restore init error', err))
        return
      }

      if (this.data.isOnline) {
        this.enterOnlineRuntime().catch((err) => logger.error('Network restore refresh error', err))
      }
    })
  },

  unbindNetworkMonitor() {
    if (runtimeNetworkUnsubscribe) {
      runtimeNetworkUnsubscribe()
      runtimeNetworkUnsubscribe = null
    }
  },

  async initData() {
    this.setData({ loading: true })
    try {
      const [info, status] = await Promise.all([
        RiderService.getMe(),
        RiderService.getStatus()
      ])
      
      this.setData({
        riderInfo: info,
        riderStatus: status,
        isOnline: status.is_online,
        initError: '',
        stats: {
          todayCount: info.total_orders || 0,
          todayEarnings: info.total_earnings || 0,
          creditScore: info.credit_score || 0
        }
      })

      if (status.is_online) {
        await this.enterOnlineRuntime()
      }
    } catch (err: unknown) {
      logger.error('Failed to init rider data', err)
      const userMessage = (err as UserMessageError).userMessage
      const message = typeof userMessage === 'string' && userMessage ? userMessage : '骑手工作台加载失败，请稍后重试'
      this.setData({
        initError: message,
        loadError: '',
        riderInfo: null,
        riderStatus: null,
        recommendOrders: [],
        activeDeliveries: []
      })
    } finally {
      this.setData({ loading: false })
    }
  },

  /**
   * 刷新业务数据（订单列表）
   */
  async refreshData() {
    if (!this.data.isOnline) return

    try {
      // 获取位置
      const location = await this.getLocation()
      
      const [hallOrdersRes, myDeliveriesRes] = await Promise.all([
        DeliveryService.getRecommendedOrders(location.longitude, location.latitude),
        request({ url: '/v1/delivery/active', method: 'GET' }) as Promise<Delivery[]>
      ])
      
      const hallOrders = (hallOrdersRes || []).map((o) => ({
        ...o,
        expires_at_format: this.formatExpiry(o.expires_at),
        distance_km: (o.distance / 1000).toFixed(1),
        pickup_distance_km: (o.distance_to_pickup / 1000).toFixed(1),
        route_distance_km: (((o.real_distance || o.distance_to_pickup || o.distance) || 0) / 1000).toFixed(1)
      }))

      const myDeliveries = (myDeliveriesRes || []).map((d) => {
        const isOverdue = d.estimated_delivery_at ? new Date(d.estimated_delivery_at).getTime() < Date.now() : false
        const deadline = d.status === 'assigned' || d.status === 'picking' ? d.estimated_pickup_at : d.estimated_delivery_at
        
        return {
          ...d,
          status_desc: this.getStatusDesc(d.status),
          deadline_desc: this.formatDeadline(deadline),
          is_overdue: isOverdue,
          is_very_urgent: !isOverdue && deadline ? (new Date(deadline).getTime() - Date.now() < 15 * 60 * 1000) : false
        }
      })

      this.setData({
        recommendOrders: hallOrders,
        activeDeliveries: myDeliveries,
        isRefresherTriggered: false,
        initError: '',
        loadError: ''
      })
    } catch (err: unknown) {
      logger.error('Refresh data error', err)
      const userMessage = (err as UserMessageError).userMessage
      const message = typeof userMessage === 'string' && userMessage ? userMessage : '任务数据加载失败，请重试'
      this.setData({ 
        isRefresherTriggered: false,
        loadError: message
      })
    }
  },

  getStatusDesc(status: string) {
    const map: Record<string, string> = {
      'assigned': '前往商家',
      'picking': '取餐中',
      'picked': '准备配送',
      'delivering': '配送中',
      'completed': '已送达',
      'exception': '订单异常'
    }
    return map[status] || status
  },

  formatDeadline(timeStr?: string) {
    if (!timeStr) return ''
    const date = new Date(timeStr)
    const now = new Date()
    const diff = date.getTime() - now.getTime()
    
    if (diff < 0) return '已超时'
    
    const h = date.getHours().toString().padStart(2, '0')
    const m = date.getMinutes().toString().padStart(2, '0')
    
    if (diff < 60 * 60 * 1000) {
      return `剩 ${Math.floor(diff / 60000)} 分钟 (${h}:${m})`
    }
    return `${h}:${m} 前`
  },

  formatExpiry(expiresAt: string) {
    const diff = new Date(expiresAt).getTime() - Date.now()
    if (diff <= 0) return '即将消失'
    return `剩 ${Math.ceil(diff / 60000)} 分钟`
  },

  onCall(e: WechatMiniprogram.TouchEvent) {
    const { phone } = e.currentTarget.dataset as { phone?: string }
    if (!phone) return
    wx.makePhoneCall({ phoneNumber: phone })
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
  },

  /**
   * 状态流转操作
   */
  async onUpdateStatus(e: WechatMiniprogram.TouchEvent) {
    const { id, action } = e.currentTarget.dataset as { id?: number, action?: DeliveryActionType }
    if (!id || !action) return

    const actionMap: Record<DeliveryActionType, DeliveryActionConfig> = {
      'startPickup': { method: DeliveryService.startPickup, loading: '正在操作...', source: 'rider_dashboard_start_pickup' },
      'confirmPickup': { method: DeliveryService.confirmPickup, loading: '确认取餐中...', source: 'rider_dashboard_confirm_pickup' },
      'startDelivery': { method: DeliveryService.startDelivery, loading: '开始配送...', source: 'rider_dashboard_start_delivery' },
      'confirmDelivery': { method: DeliveryService.confirmDelivery, loading: '确认送达中...', source: 'rider_dashboard_confirm_delivery' }
    }

    const config = actionMap[action]
    if (!config) return

    wx.showLoading({ title: config.loading })
    try {
      await this.syncDeliveryLocation(id, config.source)
      await config.method(id)
      wx.showToast({ title: '操作成功', icon: 'success' })
      this.refreshData()
    } catch (err: unknown) {
      const userMessage = (err as UserMessageError).userMessage
      const message = typeof userMessage === 'string' && userMessage ? userMessage : '操作失败'
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      wx.hideLoading()
    }
  },

  onGoToHistory() {
    wx.navigateTo({ url: '/pages/rider/tasks/index' })
  },

  onGoToClaims() {
    wx.navigateTo({ url: '/pages/rider/claims/index' })
  },

  async getLocation(): Promise<WechatMiniprogram.GetLocationSuccessCallbackResult> {
    return new Promise((resolve, reject) => {
      wx.getLocation({
        type: 'gcj02',
        success: resolve,
        fail: (err) => reject(err || new Error('getLocation failed'))
      })
    })
  },

  async syncDeliveryLocation(deliveryId: number, source: string) {
    try {
      await syncRiderDeliveryLocation(deliveryId, source)
    } catch (err: unknown) {
      throw normalizeLocationError(err)
    }
  },

  /**
   * 切换上下线
   */
  async onToggleOnline(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
    await this.toggleOnlineStatus(e.detail.value)
  },

  async toggleOnlineStatus(targetOnline: boolean) {
    if (this.data.onlineSwitchLoading) return

    this.setData({ onlineSwitchLoading: true })
    wx.showLoading({ title: targetOnline ? '正在上线...' : '正在下线...' })
    
    try {
      const latestStatus = await RiderService.getStatus()
      const canToggle = targetOnline ? latestStatus.can_go_online : latestStatus.can_go_offline

      if (!canToggle) {
        const message = targetOnline
          ? (latestStatus.online_block_reason || '当前无法上线')
          : (latestStatus.active_deliveries > 0 ? '有配送中的订单，无法下线' : '当前无法下线')

        this.setData({
          riderStatus: latestStatus,
          isOnline: latestStatus.is_online
        })
        wx.showToast({ title: message, icon: 'none' })
        return
      }

      let info: RiderInfo
      if (targetOnline) {
        info = await RiderService.goOnline()
        wx.showToast({ title: '已上线，可以接单', icon: 'success' })
      } else {
        info = await RiderService.goOffline()
        wx.showToast({ title: '已下线', icon: 'none' })
      }

      const fallbackStatus: RiderStatus = {
        ...latestStatus,
        is_online: targetOnline,
        online_status: targetOnline
          ? (latestStatus.active_deliveries > 0 ? 'delivering' : 'online')
          : 'offline',
        can_go_online: !targetOnline,
        can_go_offline: targetOnline,
        online_block_reason: targetOnline ? undefined : latestStatus.online_block_reason
      }

      const nextStatus = await RiderService.getStatus().catch(() => fallbackStatus)
      
      this.setData({ 
        isOnline: nextStatus.is_online,
        riderInfo: info,
        riderStatus: nextStatus,
        initError: ''
      })
      
      if (targetOnline) {
        this.enterOnlineRuntime().catch((err) => logger.error('Toggle online refresh error', err))
      } else {
        this.setData({ recommendOrders: [], activeDeliveries: [], newOrdersCount: 0, loadError: '' })
        this.cleanupWebSocket()
      }
    } catch (err: unknown) {
      const fallbackStatus = this.data.riderStatus
      this.setData({ isOnline: fallbackStatus ? fallbackStatus.is_online : !targetOnline })
      const userMessage = (err as UserMessageError).userMessage
      const message = typeof userMessage === 'string' && userMessage ? userMessage : '操作失败'
      wx.showToast({ 
        title: message,
        icon: 'none' 
      })
    } finally {
      wx.hideLoading()
      this.setData({ onlineSwitchLoading: false })
    }
  },

  onTabChange(e: WechatMiniprogram.CustomEvent<{ value: 'hall' | 'my' }>) {
    this.setData({ activeTab: e.detail.value })
  },

  async onPullDownRefresh() {
    this.setData({ isRefresherTriggered: true })
    await this.refreshData()
    this.setData({ isRefresherTriggered: false })
    wx.stopPullDownRefresh()
  },

  /**
   * 抢单操作
   */
  async onGrabOrder(e: WechatMiniprogram.TouchEvent) {
    const { orderId } = e.currentTarget.dataset as { orderId?: number }
    if (!orderId) return

    const order = this.data.recommendOrders.find((o) => o.order_id === orderId)
    if (!order) return

    wx.showLoading({ title: '校验中...' })
    try {
      // 1. 获取当前位置进行 LBS 校验
      const location = await this.getLocation().catch(() => null)
      if (!location) {
        wx.showToast({ title: '无法获取当前位置，抢单失败', icon: 'none' })
        return
      }

      // 2. 物理距离校验
      const distance = getDistance(
        location.latitude,
        location.longitude,
        order.pickup_latitude,
        order.pickup_longitude
      )

      if (distance > MAX_GRAB_DISTANCE) {
        wx.showToast({ 
          title: `距离过远 (约${(distance / 1000).toFixed(1)}km)，仅限${MAX_GRAB_DISTANCE / 1000}km内抢单`, 
          icon: 'none',
          duration: 3000
        })
        return
      }

      wx.showLoading({ title: '抢单中...' })
      await DeliveryService.grabOrder(orderId)
      wx.showToast({ title: '抢单成功！', icon: 'success' })
      
      // 切换到“我的”并刷新
      this.setData({ activeTab: 'my' })
      this.refreshData()
    } catch (err: unknown) {
      const userMessage = (err as UserMessageError).userMessage
      const message = typeof userMessage === 'string' && userMessage ? userMessage : '抢单失败'
      wx.showToast({ 
        title: message,
        icon: 'none' 
      })
    } finally {
      wx.hideLoading()
    }
  },

  /**
   * 前往任务详情
   */
  onGoToDetail(e: WechatMiniprogram.TouchEvent) {
    const { orderId } = e.currentTarget.dataset as { orderId?: number }
    if (!orderId) return
    wx.navigateTo({
      url: `/pages/rider/task-detail/index?id=${orderId}`
    })
  },

  /**
   * 钱包
   */
  onGoToWallet() {
    wx.navigateTo({ url: '/pages/rider/deposit/index' })
  },

  async enterOnlineRuntime() {
    if (!this.data.isOnline) return

    await this.refreshData()
    this.initWebSocket()
  },

  /**
   * 初始化 WebSocket 监听
   */
  initWebSocket() {
    if (!this.data.isOnline) return

    // 先清除旧监听，再发起连接（保证不重复注册）
    this.cleanupWebSocket()
    wsManager.connect()

    // 1. 监听订单消失事件 (已被别人抢走)
    const goneSub = wsManager.on(WSMessageType.DELIVERY_POOL_GONE, (data) => {
      const payload =
        typeof data === 'object' && data !== null
          ? (data as { order_id?: number })
          : {}
      const { order_id } = payload
      if (!order_id) return
      const { recommendOrders } = this.data
      
      // 检查该订单是否在当前列表中
      const index = recommendOrders.findIndex((o) => o.order_id === order_id)
      if (index > -1) {
        logger.info(`订单 ${order_id} 已被他人抢走，从本地移除`, undefined, 'RiderDashboard')
        
        // 瞬间移除，由于是静态化原则，我们只删除对应的项，不引起滚动重置
        const newList = [...recommendOrders]
        newList.splice(index, 1)
        this.setData({ recommendOrders: newList })
      }
    })

    // 2. 监听新订单入场事件
    const newSub = wsManager.on(WSMessageType.DELIVERY_POOL_NEW, (_data: unknown) => {
       // 不要在屏幕上跳动新卡片，而是提示“有新单”
       this.setData({
         newOrdersCount: this.data.newOrdersCount + 1
       })
       
       // 震动提醒骑手
       wx.vibrateShort({ type: 'medium' })
     })

    this.data._wsListeners = [goneSub, newSub]
  },

  cleanupWebSocket() {
    if (this.data._wsListeners) {
      this.data._wsListeners.forEach((unsub) => {
        if (typeof unsub === 'function') unsub()
      })
      this.data._wsListeners = []
    }
  },

  /**
   * 手动刷新大厅
   */
  onRefreshHall() {
    this.setData({ newOrdersCount: 0 })
    this.refreshData().catch((err) => logger.error('Manual refresh error', err))
  },

  onRetryLoad() {
    if (this.data.initError || !this.data.riderInfo || !this.data.riderStatus) {
      this.initData().catch((err) => logger.error('Retry init error', err))
      return
    }
    this.refreshData().catch((err) => logger.error('Retry refresh error', err))
  }
})
