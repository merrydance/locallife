import { isLargeScreen } from '@/utils/responsive'
import { operatorAnalyticsService } from '../../../api/operator-analytics'
import { operatorMerchantManagementService } from '../../../api/operator-merchant-management'

function formatCurrencyFen(amount: number): string {
  return `¥${(Number(amount || 0) / 100).toFixed(2)}`
}

Page({
  data: {
    isLargeScreen: false,
    navBarHeight: 88,
    loading: false,
    initialLoading: true,
    error: null as string | null,
    metrics: [] as Array<{ label: string; value: string; change: string; trend: 'up' | 'down' }>,
    topMerchants: [] as Array<{ rank: number; name: string; gmv: string; orders: number }>
  },

  onLoad() {
    this.setData({ isLargeScreen: isLargeScreen() })
    this.loadData()
  },

  async loadData() {
    this.setData({ loading: true, error: null })
    try {
      const end = new Date()
      const start = new Date()
      start.setDate(end.getDate() - 7)
      const startDate = start.toISOString().split('T')[0]
      const endDate = end.toISOString().split('T')[0]

      const [realtime, trends, ranking] = await Promise.all([
        operatorAnalyticsService.getRealtimeStats(),
        operatorAnalyticsService.getDailyTrend(undefined, startDate, endDate),
        operatorMerchantManagementService.getMerchantRanking({
          start_date: startDate,
          end_date: endDate,
          limit: 5
        })
      ])

      const trendList = Array.isArray(trends) ? trends : []
      const latest = trendList[trendList.length - 1] || { total_gmv: 0, order_count: 0 }
      const previous = trendList[trendList.length - 2] || { total_gmv: 0, order_count: 0 }
      const calcChange = (current: number, prev: number) => {
        if (!prev) return '+0%'
        const rate = ((current - prev) / prev) * 100
        const signed = rate >= 0 ? `+${rate.toFixed(1)}%` : `${rate.toFixed(1)}%`
        return signed
      }
      const gmvChange = calcChange(Number(latest.total_gmv || 0), Number(previous.total_gmv || 0))
      const ordersChange = calcChange(Number(latest.order_count || 0), Number(previous.order_count || 0))

      const metrics = [
        {
          label: '近7天GMV',
          value: formatCurrencyFen(Number(latest.total_gmv || 0)),
          change: gmvChange,
          trend: gmvChange.startsWith('-') ? 'down' : 'up'
        },
        {
          label: '活跃商户',
          value: String(realtime.active_merchant_count || 0),
          change: `待审 ${realtime.pending_merchant_count || 0}`,
          trend: 'up' as const
        },
        {
          label: '活跃骑手',
          value: String(realtime.active_rider_count || 0),
          change: `待审 ${realtime.pending_rider_count || 0}`,
          trend: 'up' as const
        },
        {
          label: '近7天订单',
          value: String(latest.order_count || 0),
          change: ordersChange,
          trend: ordersChange.startsWith('-') ? 'down' : 'up'
        }
      ]

      const rankingList = (Array.isArray((ranking as unknown as { rankings?: unknown[] }).rankings)
        ? (ranking as unknown as { rankings: Array<Record<string, unknown>> }).rankings
        : (Array.isArray(ranking) ? ranking : [])) as Array<Record<string, unknown>>

      const topMerchants = rankingList.slice(0, 5).map((item, index) => ({
        rank: index + 1,
        name: String(item.merchant_name || '-'),
        gmv: (Number(item.total_sales || item.total_gmv || 0) / 100).toFixed(2),
        orders: Number(item.order_count || 0)
      }))
      
      this.setData({
        metrics,
        topMerchants,
        initialLoading: false,
        loading: false
      })
    } catch (error) {
      console.error('加载分析数据失败:', error)
      this.setData({
        initialLoading: false,
        loading: false,
        error: '加载分析数据失败'
      })
    }
  },

  onRetry() {
    this.loadData()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  }
})
