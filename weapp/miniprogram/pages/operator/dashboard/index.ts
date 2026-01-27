import { responsiveBehavior } from '@/utils/responsive'
import { formatPriceNoSymbol } from '@/utils/util'
import { operatorBasicManagementService } from '../../../api/operator-basic-management'
import { operatorMerchantManagementService } from '../../../api/operator-merchant-management'

Page({
  behaviors: [responsiveBehavior],
  data: {
    stats: {
      total_gmv_display: '0.00',
      total_orders: 0,
      active_merchants: 0,
      active_riders: 0
    },
    finance: null as any,
    pending_approvals: [] as any[],
    loading: false,
    initialLoading: true,
    error: null as string | null,
    navBarHeight: 88,
  },

  onLoad() {
    this.loadDashboardData()
  },

  onShow() {
    if (!this.data.initialLoading) {
      this.loadDashboardData()
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadDashboardData() {
    if (this.data.loading && !this.data.initialLoading) return
    this.setData({ loading: true, error: null })
    
    try {
      // 1. 获取财务概览 (运营商视角)
      const finance = await operatorBasicManagementService.getFinanceOverview()
      
      // 2. 获取商户列表 (用于统计状态分布)
      const merchantList = await operatorMerchantManagementService.getMerchantList({
        page: 1,
        limit: 100
      })

      // 3. 计算实时指标
      const pendingMerchants = (merchantList.merchants || []).filter(m => m.status === 'pending_approval')
      
      this.setData({
        finance,
        stats: {
          total_gmv_display: formatPriceNoSymbol(finance.current_month.total_gmv || 0),
          total_orders: finance.current_month.total_orders || 0,
          active_merchants: (merchantList.merchants || []).filter(m => m.status === 'active').length,
          active_riders: 0 // 后端当前 region_stats 尚未直接返回实时活跃骑手，暂设为0或通过列表统计
        },
        pending_approvals: pendingMerchants.map(m => ({
          id: m.id,
          type: 'MERCHANT_JOIN',
          name: `商户入驻申请 - ${m.name || '未知商户'}`,
          created_at: m.created_at
        })),
        loading: false,
        initialLoading: false 
      })
    } catch (error: any) {
      console.error('加载运营仪表盘失败:', error)
      this.setData({ 
        loading: false,
        initialLoading: false,
        error: error.message || '加载仪表盘数据失败'
      })
    }
  },

  onRetry() {
    this.loadDashboardData()
  },

  async onApprove(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    try {
      await operatorMerchantManagementService.resumeMerchant(id, { reason: '审批通过' })
      wx.showToast({ title: '已通过', icon: 'success' })
      this.loadDashboardData()
    } catch (error) {
      console.error('审批失败:', error)
      wx.showToast({ title: '操作失败', icon: 'error' })
    }
  },

  async onReject(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    try {
      // 拒绝入驻暂用挂起逻辑，或调用专门的审核拒绝API（若有）
      await operatorMerchantManagementService.suspendMerchant(id, { 
        reason: '审批拒绝',
        duration_hours: 720
      })
      wx.showToast({ title: '已拒绝', icon: 'none' })
      this.loadDashboardData()
    } catch (error) {
      console.error('拒绝失败:', error)
      wx.showToast({ title: '操作失败', icon: 'error' })
    }
  }
})

