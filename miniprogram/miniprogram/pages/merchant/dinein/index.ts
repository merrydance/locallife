/**
 * 商户堂食管理页面
 * 使用真实后端API
 */

import { responsiveBehavior } from '../../../utils/responsive'
import { tableManagementService, TableResponse } from '../../../api/table-device-management'

Page({
  behaviors: [responsiveBehavior],
  data: {
    tables: [] as any[],
    sessions: [] as any[],
    tableStats: { total: 0, available: 0, occupied: 0 },
    loading: false
  },

  onLoad() {
    // 移除 manual isLargeScreen 设置，由 responsiveBehavior 注入
    this.loadData()
  },

  onShow() {
    // 返回时刷新
    if (this.data.tables.length > 0) {
      this.loadData()
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadData() {
    this.setData({ loading: true })

    try {
      // 获取桌台列表
      const result = await tableManagementService.listTables('table')

      const tables = (result.tables || []).map((table: TableResponse) => ({
        id: table.id,
        name: table.table_no,
        status: table.status?.toUpperCase() || 'AVAILABLE',
        capacity: table.capacity,
        description: table.description,
        minimum_spend: table.minimum_spend,
        current_reservation_id: table.current_reservation_id
      }))

      // 筛选出占用中的桌台作为活跃会话
      const sessions = tables
        .filter((t: any) => t.status === 'OCCUPIED')
        .map((t: any) => ({
          id: `session_${t.id}`,
          table_id: t.id,
          table_name: t.name,
          status: 'ACTIVE'
        }))

      const tableStats = {
        total: tables.length,
        available: tables.filter((t: any) => t.status === 'AVAILABLE').length,
        occupied: tables.filter((t: any) => t.status === 'OCCUPIED').length
      }

      this.setData({
        tables,
        sessions,
        tableStats,
        loading: false
      })
    } catch (error) {
      console.error('加载桌台数据失败:', error)
      wx.showToast({ title: '加载失败', icon: 'error' })
      this.setData({ loading: false })
    }
  },

  onOpenTable(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    wx.showModal({
      title: '开台确认',
      content: '确认开台?',
      success: async (res) => {
        if (res.confirm) {
          try {
            await tableManagementService.updateTableStatus(id, { status: 'occupied' })
            wx.showToast({ title: '开台成功', icon: 'success' })
            this.loadData()
          } catch (error) {
            console.error('开台失败:', error)
            wx.showToast({ title: '开台失败', icon: 'error' })
          }
        }
      }
    })
  },

  onCloseTable(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    wx.showModal({
      title: '结台确认',
      content: '确认结台?',
      success: async (res) => {
        if (res.confirm) {
          try {
            await tableManagementService.updateTableStatus(id, { status: 'available' })
            wx.showToast({ title: '结台成功', icon: 'success' })
            this.loadData()
          } catch (error) {
            console.error('结台失败:', error)
            wx.showToast({ title: '结台失败', icon: 'error' })
          }
        }
      }
    })
  },

  onCheckout(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    wx.navigateTo({ url: `/pages/merchant/dinein/checkout/index?session_id=${id}` })
  }
})
