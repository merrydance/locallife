/**
 * 商户数据分析页面
 * 使用真实后端API
 */

import { isLargeScreen } from '../../../utils/responsive'
import { MerchantStatsService, StatsOverviewResponse, TopDishResponse } from '../../../api/merchant-analytics'

Page({
  data: {
    timeRange: 'TODAY' as 'TODAY' | 'WEEK' | 'MONTH',
    metrics: {
      gmv: { value: 0, change: 0 },
      orderCount: { value: 0, change: 0 },
      avgOrderValue: { value: 0, change: 0 },
      repeatRate: { value: 0, change: 0 }
    },
    topDishes: [] as any[],
    isLargeScreen: false,
    navBarHeight: 88,
    loading: false
  },

  onLoad() {
    this.setData({ isLargeScreen: isLargeScreen() })
    this.loadAnalytics()
  },

  onShow() {
    // 返回时刷新
    this.loadAnalytics()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  getDateRange(): { start_date: string; end_date: string } {
    const today = new Date()
    const endDate = this.formatDate(today)
    let startDate = endDate

    switch (this.data.timeRange) {
      case 'WEEK':
        const weekAgo = new Date(today)
        weekAgo.setDate(weekAgo.getDate() - 7)
        startDate = this.formatDate(weekAgo)
        break
      case 'MONTH':
        const monthAgo = new Date(today)
        monthAgo.setMonth(monthAgo.getMonth() - 1)
        startDate = this.formatDate(monthAgo)
        break
      default:
        // TODAY
        break
    }

    return { start_date: startDate, end_date: endDate }
  },

  formatDate(date: Date): string {
    const year = date.getFullYear()
    const month = ('0' + (date.getMonth() + 1)).slice(-2)
    const day = ('0' + date.getDate()).slice(-2)
    return `${year}-${month}-${day}`
  },

  async loadAnalytics() {
    this.setData({ loading: true })

    try {
      const dateRange = this.getDateRange()

      // 并行加载概览和热门菜品
      const [overview, topDishes] = await Promise.all([
        MerchantStatsService.getStatsOverview(dateRange),
        MerchantStatsService.getTopDishes({ ...dateRange, limit: 10 })
      ])

      // 更新指标
      this.setData({
        metrics: {
          gmv: {
            value: Math.round((overview.total_revenue || 0) / 100),
            change: Math.round((overview.growth_rate || 0) * 100)
          },
          orderCount: {
            value: overview.total_orders || 0,
            change: 0
          },
          avgOrderValue: {
            value: Math.round((overview.avg_order_value || 0) / 100),
            change: 0
          },
          repeatRate: {
            value: Math.round((overview.completion_rate || 0) * 100),
            change: 0
          }
        },
        topDishes: (topDishes || []).map((dish: TopDishResponse) => ({
          name: dish.dish_name,
          sales: dish.sales_count,
          revenue: dish.revenue
        })),
        loading: false
      })
    } catch (error) {
      console.error('加载分析数据失败:', error)
      wx.showToast({ title: '加载失败', icon: 'error' })
      this.setData({ loading: false })
    }
  },

  onTimeRangeChange(e: WechatMiniprogram.CustomEvent) {
    this.setData({ timeRange: e.detail.value })
    this.loadAnalytics()
  }
})
