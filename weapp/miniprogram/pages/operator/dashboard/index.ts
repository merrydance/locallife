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
      // 1. 并行获取基础数据
      const [finance, merchantList, regions] = await Promise.all([
        operatorBasicManagementService.getFinanceOverview(),
        operatorMerchantManagementService.getMerchantList({ page: 1, limit: 100 }),
        operatorBasicManagementService.getOperatorRegions({ limit: 100 })
      ])

      // 2. 获取区域统计数据 (用于获取活跃骑手等实时指标)
      // 聚合所有管理区域的数据
      let activeRiders = 0
      if (regions.regions && regions.regions.length > 0) {
        const regionStatsPromises = regions.regions.map(region => 
          operatorBasicManagementService.getRegionStats(region.id)
        )
        const regionStatsList = await Promise.all(regionStatsPromises)
        
        activeRiders = regionStatsList.reduce((sum, stats) => sum + (stats.active_rider_count || 0), 0)
      }

      // 3. 计算商户状态
      const pendingMerchants = (merchantList.merchants || []).filter(m => m.status === 'pending_approval')
      const activeMerchants = (merchantList.merchants || []).filter(m => m.status === 'active').length
      
      this.setData({
        finance,
        stats: {
          total_gmv_display: formatPriceNoSymbol(finance.current_month.total_gmv || 0),
          total_orders: finance.current_month.total_orders || 0,
          active_merchants: activeMerchants,
          active_riders: activeRiders
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
      wx.showLoading({ title: '处理中' })
      await operatorMerchantManagementService.resumeMerchant(id, { reason: '审批通过' })
      wx.showToast({ title: '已通过', icon: 'success' })
      this.loadDashboardData()
    } catch (error) {
      console.error('审批失败:', error)
      wx.showToast({ title: '操作失败', icon: 'error' })
    } finally {
      wx.hideLoading()
    }
  },

  async onReject(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    try {
      wx.showLoading({ title: '处理中' })
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
    } finally {
      wx.hideLoading()
    }
  }
})

