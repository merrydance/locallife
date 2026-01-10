/**
 * 商户工作台 v4.0 - 全屏沉浸式设计
 * 简化版：专注当日经营，三栏布局，WebSocket实时更新
 */

import { MerchantManagementService } from '../../../api/merchant'
import { getTables, Table, updateTableStatus } from '../../../api/merchant-table-device-management'
import { MerchantStatsService } from '../../../api/merchant-analytics'
import { MerchantOrderManagementService } from '../../../api/order-management'
import { WebSocketUtils, RealtimeUtils, WebSocketMessage } from '../../../api/websocket-realtime'
import { logger } from '../../../utils/logger'

const app = getApp<IAppOption>()

Page({
  data: {
    // 商户信息
    merchantName: '',
    isOpen: false,
    currentDate: '',

    // WebSocket 状态
    wsConnected: false,

    // 统计数据
    stats: {
      todayRevenue: 0,
      todayOrders: 0
    },
    revenueDisplay: '0.00',

    // 订单标签
    orderTab: 'all' as 'all' | 'paid' | 'preparing' | 'ready',

    // 状态计数
    statusCounts: {
      paid: 0,
      preparing: 0,
      ready: 0
    },

    // 订单数据
    pendingOrders: [] as any[],
    filteredOrders: [] as any[],

    // 桌台数据
    tableGroups: [] as any[],
    tableStats: {
      total: 0,
      available: 0,
      occupied: 0
    },

    // 桌台弹窗
    showTablePopup: false,
    activeTable: null as any
  },

  onLoad() {
    this.updateDate()
    this.loadData()
  },

  onShow() {
    if (this.data.merchantName) {
      this.loadOrders()
      this.loadTables()
    }
  },

  onHide() {
    // 页面隐藏时保持连接
  },

  onUnload() {
    // 页面卸载时断开 WebSocket
    WebSocketUtils.closeAll()
  },

  updateDate() {
    const now = new Date()
    const weekDays = ['星期日', '星期一', '星期二', '星期三', '星期四', '星期五', '星期六']
    const dateStr = `${now.getFullYear()}年${now.getMonth() + 1}月${now.getDate()}日 ${weekDays[now.getDay()]}`
    this.setData({ currentDate: dateStr })
  },

  async loadData() {
    try {
      await this.loadMerchantInfo()
      await Promise.all([
        this.loadStats(),
        this.loadOrders(),
        this.loadTables()
      ])
      // 营业中时连接 WebSocket
      if (this.data.isOpen) {
        this.connectWebSocket()
      }
    } catch (error) {
      logger.error('加载数据失败', error, 'Dashboard')
      wx.showToast({ title: '加载失败', icon: 'none' })
    }
  },

  async loadMerchantInfo() {
    try {
      const info = await MerchantManagementService.getMerchantInfo();
      if (info) {
        this.setData({
          merchantName: info.name,
          isOpen: info.is_open
        });
        app.globalData.merchantId = String(info.id);
        app.globalData.userRole = 'merchant';
      }
    } catch (error) {
      wx.showToast({ title: '加载商户失败', icon: 'error' });
      logger.error('加载商户信息失败', error, 'Dashboard');
    }
  },

  async loadStats() {
    try {
      const today = new Date().toISOString().split('T')[0]
      const stats = await MerchantStatsService.getStatsOverview({
        start_date: today,
        end_date: today
      })
      if (stats) {
        const revenue = stats.total_revenue || 0
        this.setData({
          stats: {
            todayRevenue: revenue,
            todayOrders: stats.total_orders || 0
          },
          revenueDisplay: (revenue / 100).toFixed(2)
        })
      }
    } catch (error) {
      logger.error('加载统计失败', error, 'Dashboard')
    }
  },

  async loadOrders() {
    try {
      const orders = await MerchantOrderManagementService.getOrderList({
        page_id: 1,
        page_size: 50
      })

      // 订单类型和状态映射
      const typeMap: Record<string, string> = {
        'takeout': '外卖',
        'dine_in': '堂食',
        'takeaway': '自取',
        'reservation': '预订'
      }
      const statusMap: Record<string, string> = {
        'paid': '待接单',
        'preparing': '制作中',
        'ready': '待取餐'
      }

      // 过滤和格式化
      const pendingOrders = (orders || [])
        .filter((o: any) => ['paid', 'preparing', 'ready'].includes(o.status))
        .map((o: any) => {
          let createdTime = ''
          if (o.created_at) {
            const d = new Date(o.created_at)
            const h = d.getHours()
            const m = d.getMinutes()
            createdTime = (h < 10 ? '0' : '') + h + ':' + (m < 10 ? '0' : '') + m
          }
          return {
            id: o.id,
            order_no: o.order_no,
            status: o.status,
            status_text: statusMap[o.status] || o.status,
            order_type: o.order_type,
            order_type_text: typeMap[o.order_type] || o.order_type,
            total_amount: o.total_amount,
            amount_display: (o.total_amount / 100).toFixed(2),
            items_summary: o.items?.slice(0, 2).map((i: any) => i.name).join('、') || '订单商品',
            table_no: o.table_no,
            created_at: o.created_at,
            created_time: createdTime
          }
        })

      const statusCounts = {
        paid: pendingOrders.filter((o: any) => o.status === 'paid').length,
        preparing: pendingOrders.filter((o: any) => o.status === 'preparing').length,
        ready: pendingOrders.filter((o: any) => o.status === 'ready').length
      }

      this.setData({ pendingOrders, statusCounts })
      this.filterOrders()
    } catch (error) {
      logger.error('加载订单失败', error, 'Dashboard')
    }
  },

  // 切换订单标签
  switchOrderTab(e: any) {
    const tab = e.currentTarget.dataset.tab
    this.setData({ orderTab: tab })
    this.filterOrders()
  },

  // 筛选订单
  filterOrders() {
    const { pendingOrders, orderTab } = this.data
    let filtered = pendingOrders
    if (orderTab !== 'all') {
      filtered = pendingOrders.filter((o: any) => o.status === orderTab)
    }
    this.setData({ filteredOrders: filtered })
  },

  async loadTables() {
    try {
      const response = await getTables({})
      const tables = response.tables || []

      const statusClassMap: Record<string, string> = {
        'available': 'status-available',
        'occupied': 'status-occupied',
        'reserved': 'status-reserved',
        'disabled': 'status-disabled'
      }
      const statusTextMap: Record<string, string> = {
        'available': '空闲',
        'occupied': '就餐中',
        'reserved': '已预订',
        'disabled': '停用'
      }

      const formattedTables = tables.map((t: Table) => ({
        ...t,
        status_class: statusClassMap[t.status] || '',
        status_text: statusTextMap[t.status] || t.status
      }))

      // 分组：散台和包间
      const tablesByType = new Map<string, any[]>()
      formattedTables.forEach((table: any) => {
        const type = table.table_type || 'table'
        if (!tablesByType.has(type)) {
          tablesByType.set(type, [])
        }
        tablesByType.get(type)!.push(table)
      })

      const tableGroups: any[] = []
      if (tablesByType.has('table')) {
        tableGroups.push({ name: '散台', type: 'table', tables: tablesByType.get('table')! })
      }
      if (tablesByType.has('room')) {
        tableGroups.push({ name: '包间', type: 'room', tables: tablesByType.get('room')! })
      }

      const tableStats = {
        total: tables.length,
        available: tables.filter((t: Table) => t.status === 'available').length,
        occupied: tables.filter((t: Table) => t.status === 'occupied').length
      }

      this.setData({ tableGroups, tableStats })
    } catch (error) {
      logger.error('加载桌台失败', error, 'Dashboard')
    }
  },

  // WebSocket 连接
  async connectWebSocket() {
    const merchantId = app.globalData.merchantId
    const userId = app.globalData.userId

    if (!merchantId) {
      logger.warn('商户ID不存在，跳过WebSocket连接', {}, 'Dashboard')
      return
    }

    try {
      await RealtimeUtils.initializeForMerchant(
        userId || '0',
        merchantId || '0',
        {
          onOpen: () => {
            logger.info('WebSocket 连接成功', { merchantId }, 'Dashboard')
            this.setData({ wsConnected: true })
          },
          onMessage: (msg: WebSocketMessage) => {
            this.handleWebSocketMessage(msg)
          },
          onNotification: (notif: any) => {
            if (notif.title?.includes('订单') || notif.content?.includes('订单')) {
              this.loadOrders()
              wx.vibrateShort({ type: 'medium' })
            }
          },
          onOrderUpdate: (orderData: any) => {
            logger.info('收到订单更新', orderData, 'Dashboard')
            this.loadOrders()
            this.loadStats()
            wx.vibrateShort({ type: 'medium' })
          }
        }
      )
    } catch (error) {
      logger.error('WebSocket 连接失败', error, 'Dashboard')
      this.setData({ wsConnected: false })
    }
  },

  handleWebSocketMessage(msg: WebSocketMessage) {
    if (msg.type === 'new_order' || msg.type === 'order_update') {
      wx.vibrateShort({ type: 'medium' })
      this.loadOrders()
      this.loadStats()
    } else if (msg.type === 'table_status_change') {
      this.loadTables()
    }
  },

  // 切换营业状态
  async onToggleStatus() {
    const newStatus = !this.data.isOpen
    try {
      await MerchantManagementService.updateMerchantStatus({ is_open: newStatus })
      this.setData({ isOpen: newStatus })
      wx.showToast({ title: newStatus ? '已开始营业' : '已暂停营业', icon: 'none' })

      // 营业状态变化时管理 WebSocket
      if (newStatus) {
        this.connectWebSocket()
      } else {
        WebSocketUtils.closeAll()
        this.setData({ wsConnected: false })

        // 打烊时释放所有桌台
        this.releaseAllTables()
      }
    } catch (error) {
      wx.showToast({ title: '操作失败', icon: 'none' })
    }
  },

  // 释放所有桌台（打烊时调用）
  async releaseAllTables() {
    try {
      const response = await getTables()
      const allTables = response.tables || []
      const occupiedTables = allTables.filter((t: Table) => t.status === 'occupied')

      if (occupiedTables.length > 0) {
        // 批量更新所有占用的桌台为可用
        for (const table of occupiedTables) {
          await updateTableStatus(table.id, 'available')
        }
        logger.info(`打烊释放了 ${occupiedTables.length} 个桌台`, null, 'Dashboard')
        this.loadTables()
      }
    } catch (error) {
      logger.error('释放桌台失败', error, 'Dashboard')
    }
  },

  // 订单操作
  onOrderTap(e: any) {
    const id = e.currentTarget.dataset.id
    wx.navigateTo({ url: `/pages/merchant/orders/index?highlight=${id}` })
  },

  async onAcceptOrder(e: any) {
    const id = e.currentTarget.dataset.id
    try {
      await MerchantOrderManagementService.acceptOrder(id)
      wx.showToast({ title: '已接单', icon: 'success' })
      this.loadOrders()
    } catch (error) {
      wx.showToast({ title: '接单失败', icon: 'none' })
    }
  },

  async onRejectOrder(e: any) {
    const id = e.currentTarget.dataset.id
    wx.showModal({
      title: '拒单原因',
      editable: true,
      placeholderText: '请输入拒单原因',
      success: async (res) => {
        if (res.confirm && res.content) {
          try {
            await MerchantOrderManagementService.rejectOrder(id, { reason: res.content })
            wx.showToast({ title: '已拒单', icon: 'success' })
            this.loadOrders()
          } catch (error) {
            wx.showToast({ title: '拒单失败', icon: 'none' })
          }
        }
      }
    })
  },

  async onReadyOrder(e: any) {
    const id = e.currentTarget.dataset.id
    try {
      await MerchantOrderManagementService.markOrderReady(id)
      wx.showToast({ title: '已出餐', icon: 'success' })
      this.loadOrders()
    } catch (error) {
      wx.showToast({ title: '操作失败', icon: 'none' })
    }
  },

  // 桌台操作 - 点击显示弹窗
  onTableCardTap(e: any) {
    const table = e.currentTarget.dataset.table
    this.setData({
      showTablePopup: true,
      activeTable: table
    })
  },

  closeTablePopup() {
    this.setData({
      showTablePopup: false,
      activeTable: null
    })
  },

  async setTableStatus(e: any) {
    const newStatus = e.currentTarget.dataset.status
    const { activeTable } = this.data
    if (!activeTable?.id) return

    try {
      await updateTableStatus(activeTable.id, newStatus)
      wx.showToast({ title: '状态已更新', icon: 'success' })
      this.closeTablePopup()
      this.loadTables()
    } catch (error) {
      logger.error('更新桌台状态失败', error, 'Dashboard')
      wx.showToast({ title: '更新失败', icon: 'none' })
    }
  },

  // 跳转到预订管理页面添加预订
  goToAddReservation() {
    const { activeTable } = this.data
    this.closeTablePopup()
    // 跳转到预订页面，带上桌台ID参数
    wx.navigateTo({
      url: `/pages/merchant/reservations/index?tableId=${activeTable?.id}&openAdd=true`
    })
  },

  // 旧的导航方法（保留用于快捷入口）
  onTableTap(e: any) {
    const id = e.currentTarget.dataset.id
    wx.navigateTo({ url: `/pages/merchant/tables/index?tableId=${id}` })
  },

  // 快捷导航
  goToInventory() {
    wx.navigateTo({ url: '/pages/merchant/inventory/index' })
  },

  goToMembers() {
    wx.navigateTo({ url: '/pages/merchant/members/index' })
  },

  goToReservations() {
    wx.navigateTo({ url: '/pages/merchant/reservations/index' })
  },

  goToKitchen() {
    wx.navigateTo({ url: '/pages/merchant/kds/index' })
  },

  goToStats() {
    wx.navigateTo({ url: '/pages/merchant/analytics/index' })
  },

  goToFinance() {
    wx.navigateTo({ url: '/pages/merchant/finance/index' })
  },

  goToSettings() {
    wx.navigateTo({ url: '/pages/merchant/settings/index' })
  },

  goToDishes() {
    wx.navigateTo({ url: '/pages/merchant/dishes/index' })
  },

  goToCombos() {
    wx.navigateTo({ url: '/pages/merchant/combos/index' })
  },

  goToTables() {
    wx.navigateTo({ url: '/pages/merchant/tables/index' })
  },

  goToMarketing() {
    wx.navigateTo({ url: '/pages/merchant/vouchers/index' })
  },

  goToNavigation() {
    wx.navigateTo({ url: '/pages/merchant/navigation/index' })
  }
})
