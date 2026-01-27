import RiderService, { RiderInfo, RiderStatus } from '../../../api/rider'
import DeliveryService, { RecommendedOrder, Delivery } from '../../../api/delivery'
import { logger } from '../../../utils/logger'
import { getStableBarHeights } from '../../../utils/responsive'
import { wsManager, WSMessageType } from '../../../utils/websocket'
import { globalStore } from '../../../utils/global-store'
import { request } from '../../../utils/request'

const app = getApp<IAppOption>()

Page({
  data: {
    // UI 状态
    navBarHeight: 88,
    activeTab: 'hall', // hall: 抢单大厅, my: 我的配送
    loading: false,
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
    _wsListeners: [] as any[]
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.initData().catch(err => logger.error('Load init error', err))
  },

  onShow() {
    if (this.data.isOnline) {
      this.refreshData().catch(err => logger.error('Show refresh error', err))
      this.initWebSocket()
    }
  },

  onHide() {
    this.cleanupWebSocket()
  },

  onUnload() {
    this.cleanupWebSocket()
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
        stats: {
          todayCount: info.total_orders || 0,
          todayEarnings: info.total_earnings || 0,
          creditScore: info.credit_score || 0
        }
      })

      if (status.is_online) {
        this.refreshData()
      }
    } catch (err) {
      logger.error('Failed to init rider data', err)
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
      
      const hallOrders = (hallOrdersRes || []).map(o => ({
        ...o,
        expires_at_format: this.formatExpiry(o.expires_at),
        distance_km: (o.distance / 1000).toFixed(1),
        pickup_distance_km: (o.distance_to_pickup / 1000).toFixed(1)
      }));

      const myDeliveries = (myDeliveriesRes || []).map(d => {
        const isOverdue = d.estimated_delivery_at ? new Date(d.estimated_delivery_at).getTime() < Date.now() : false;
        const deadline = d.status === 'assigned' || d.status === 'picking' ? d.estimated_pickup_at : d.estimated_delivery_at;
        
        return {
          ...d,
          status_desc: this.getStatusDesc(d.status),
          deadline_desc: this.formatDeadline(deadline),
          is_overdue: isOverdue,
          is_very_urgent: !isOverdue && deadline ? (new Date(deadline).getTime() - Date.now() < 15 * 60 * 1000) : false
        };
      });

      this.setData({
        recommendOrders: hallOrders,
        activeDeliveries: myDeliveries,
        isRefresherTriggered: false
      })
    } catch (err) {
      logger.error('Refresh data error', err)
      this.setData({ 
        recommendOrders: [],
        activeDeliveries: [],
        isRefresherTriggered: false 
      })
    }
  },

  getStatusDesc(status: string) {
    const map: any = {
      'assigned': '前往商家',
      'picking': '取餐中',
      'picked': '准备配送',
      'delivering': '配送中',
      'completed': '已送达',
      'exception': '订单异常'
    };
    return map[status] || status;
  },

  formatDeadline(timeStr?: string) {
    if (!timeStr) return '';
    const date = new Date(timeStr);
    const now = new Date();
    const diff = date.getTime() - now.getTime();
    
    if (diff < 0) return '已超时';
    
    const h = date.getHours().toString().padStart(2, '0');
    const m = date.getMinutes().toString().padStart(2, '0');
    
    if (diff < 60 * 60 * 1000) {
      return `剩 ${Math.floor(diff / 60000)} 分钟 (${h}:${m})`;
    }
    return `${h}:${m} 前`;
  },

  formatExpiry(expiresAt: string) {
    const diff = new Date(expiresAt).getTime() - Date.now();
    if (diff <= 0) return '即将消失';
    return `剩 ${Math.ceil(diff / 60000)} 分钟`;
  },

  onCall(e: any) {
    const { phone } = e.currentTarget.dataset;
    if (!phone) return;
    wx.makePhoneCall({ phoneNumber: phone });
  },

  /**
   * 状态流转操作
   */
  async onUpdateStatus(e: any) {
    const { id, action } = e.currentTarget.dataset;
    if (!id || !action) return;

    const actionMap: any = {
      'startPickup': { method: DeliveryService.startPickup, loading: '正在操作...' },
      'confirmPickup': { method: DeliveryService.confirmPickup, loading: '确认取餐中...' },
      'startDelivery': { method: DeliveryService.startDelivery, loading: '开始配送...' },
      'confirmDelivery': { method: DeliveryService.confirmDelivery, loading: '确认送达中...' }
    };

    const config = actionMap[action];
    if (!config) return;

    wx.showLoading({ title: config.loading });
    try {
      await config.method(id);
      wx.showToast({ title: '操作成功', icon: 'success' });
      this.refreshData();
    } catch (err: any) {
      wx.showToast({ title: err.userMessage || '操作失败', icon: 'none' });
    } finally {
      wx.hideLoading();
    }
  },

  onChat(e: any) {
    const { orderId } = e.currentTarget.dataset;
    wx.navigateTo({ url: `/pages/chat/index?orderId=${orderId}` });
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

  /**
   * 切换上下线
   */
  async onToggleOnline(e: any) {
    const targetOnline = e.detail.value
    wx.showLoading({ title: targetOnline ? '正在上线...' : '正在下线...' })
    
    try {
      let info: RiderInfo
      if (targetOnline) {
        info = await RiderService.goOnline()
        wx.showToast({ title: '已上线，可以接单', icon: 'success' })
      } else {
        info = await RiderService.goOffline()
        wx.showToast({ title: '已下线', icon: 'none' })
      }
      
      this.setData({ 
        isOnline: targetOnline,
        riderInfo: info
      })
      
      if (targetOnline) {
        this.refreshData().catch(err => logger.error('Toggle online refresh error', err))
        this.initWebSocket()
      } else {
        this.setData({ recommendOrders: [] })
        this.cleanupWebSocket()
      }
    } catch (err: any) {
      this.setData({ isOnline: !targetOnline })
      wx.showToast({ 
        title: err.userMessage || '操作失败', 
        icon: 'none' 
      })
    } finally {
      wx.hideLoading()
    }
  },

  onTabChange(e: any) {
    this.setData({ activeTab: e.detail.value })
  },

  async onPullDownRefresh() {
    this.setData({ isRefresherTriggered: true });
    await this.refreshData();
    this.setData({ isRefresherTriggered: false });
    wx.stopPullDownRefresh();
  },

  /**
   * 抢单操作
   */
  async onGrabOrder(e: any) {
    const { orderId } = e.currentTarget.dataset
    if (!orderId) return

    wx.showLoading({ title: '抢单中...' })
    try {
      await DeliveryService.grabOrder(orderId)
      wx.showToast({ title: '抢单成功！', icon: 'success' })
      
      // 切换到“我的”并刷新
      this.setData({ activeTab: 'my' })
      this.refreshData()
    } catch (err: any) {
      wx.showToast({ 
        title: err.userMessage || '抢单失败', 
        icon: 'none' 
      })
    } finally {
      wx.hideLoading()
    }
  },

  /**
   * 前往任务详情
   */
  onGoToDetail(e: any) {
    const { orderId } = e.currentTarget.dataset
    wx.navigateTo({
      url: `/pages/rider/task-detail/index?id=${orderId}`
    })
  },

  /**
   * 提现/钱包
   */
  onGoToWallet() {
    wx.navigateTo({ url: '/pages/rider/deposit/index' })
  },

  /**
   * 初始化 WebSocket 监听
   */
  initWebSocket() {
    if (!this.data.isOnline) return;
    
    wsManager.connect();
    
    // 清除旧监听
    this.cleanupWebSocket();

    // 1. 监听订单消失事件 (已被别人抢走)
    const goneSub = wsManager.on(WSMessageType.DELIVERY_POOL_GONE, (data) => {
      const { order_id } = data;
      const { recommendOrders } = this.data;
      
      // 检查该订单是否在当前列表中
      const index = recommendOrders.findIndex(o => o.order_id === order_id);
      if (index > -1) {
        logger.info(`订单 ${order_id} 已被他人抢走，从本地移除`, undefined, 'RiderDashboard');
        
        // 瞬间移除，由于是静态化原则，我们只删除对应的项，不引起滚动重置
        const newList = [...recommendOrders];
        newList.splice(index, 1);
        this.setData({ recommendOrders: newList });
      }
    });

    // 2. 监听新订单入场事件
    const newSub = wsManager.on(WSMessageType.DELIVERY_POOL_NEW, (data) => {
       // 不要在屏幕上跳动新卡片，而是提示“有新单”
       this.setData({
         newOrdersCount: this.data.newOrdersCount + 1
       });
       
       // 震动提醒骑手
       wx.vibrateShort({ type: 'medium' });
    });

    this.data._wsListeners = [goneSub, newSub];
  },

  cleanupWebSocket() {
    if (this.data._wsListeners) {
      this.data._wsListeners.forEach(unsub => {
        if (typeof unsub === 'function') unsub();
      });
      this.data._wsListeners = [];
    }
  },

  /**
   * 手动刷新大厅
   */
  onRefreshHall() {
    this.setData({ newOrdersCount: 0 });
    this.refreshData().catch(err => logger.error('Manual refresh error', err));
  }
})

