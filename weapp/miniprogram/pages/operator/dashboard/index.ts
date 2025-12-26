/**
 * 运营仪表盘页面
 * 使用真实后端API
 */

import { responsiveBehavior } from '@/utils/responsive'
import { platformDashboardService, RealtimeDashboardData } from '../../../api/platform-dashboard'
import { operatorMerchantManagementService } from '../../../api/operator-merchant-management'

Page({
  behaviors: [responsiveBehavior],
  data: {
    stats: {
      total_gmv: 0,
      total_orders: 0,
      active_merchants: 0,
      active_riders: 0
    },
    pending_approvals: [] as any[],
    loading: false
  },

  onLoad() {
    // Layout data is automatically injected by responsiveBehavior
    this.loadDashboardData()
  },

  onShow() {
    // 返回时刷新
    this.loadDashboardData()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadDashboardData() {
    this.setData({ loading: true })
    try {
      // 获取实时大盘数据
      const realtimeData: RealtimeDashboardData = await platformDashboardService.getRealtimeDashboard()

      this.setData({
        stats: {
          total_gmv: realtimeData.gmv_24h || 0,
          total_orders: realtimeData.orders_24h || 0,
          active_merchants: realtimeData.active_merchants_24h || 0,
          active_riders: 0 // 后端暂无骑手数据
        },
        loading: false
      })

      // 加载待审批列表
      await this.loadPendingApprovals()
    } catch (error) {
      console.error('加载仪表盘数据失败:', error)
      wx.showToast({ title: '加载失败', icon: 'error' })
      this.setData({ loading: false })
    }
  },

  async loadPendingApprovals() {
    try {
      // 获取待审批商户列表
      const merchantList = await operatorMerchantManagementService.getMerchantList({
        page_id: 1,
        page_size: 10
      })

      // 筛选待审批的商户
      const pendingMerchants = (merchantList.merchants || []).filter(m => m.status === 'pending_approval')

      const approvals = pendingMerchants.map(m => ({
        id: m.id,
        type: 'MERCHANT_JOIN',
        name: `商户入驻申请 - ${m.name || '未知商户'}`,
        created_at: m.created_at
      }))

      this.setData({ pending_approvals: approvals })
    } catch (error) {
      console.warn('加载待审批列表失败:', error)
    }
  },

  async onApprove(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset

    try {
      await operatorMerchantManagementService.resumeMerchant(id, { reason: '审批通过' })
      wx.showToast({ title: '已通过', icon: 'success' })

      const newList = this.data.pending_approvals.filter((i) => i.id !== id)
      this.setData({ pending_approvals: newList })
    } catch (error) {
      console.error('审批失败:', error)
      wx.showToast({ title: '操作失败', icon: 'error' })
    }
  },

  async onReject(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset

    try {
      await operatorMerchantManagementService.suspendMerchant(id, { reason: '审批拒绝' })
      wx.showToast({ title: '已拒绝', icon: 'none' })

      const newList = this.data.pending_approvals.filter((i) => i.id !== id)
      this.setData({ pending_approvals: newList })
    } catch (error) {
      console.error('拒绝失败:', error)
      wx.showToast({ title: '操作失败', icon: 'error' })
    }
  }
})
