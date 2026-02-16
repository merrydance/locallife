import { isLargeScreen } from '@/utils/responsive'

Page({
  data: {
    isLargeScreen: false,
    navBarHeight: 88,
    loading: false,
    initialLoading: true,
    error: null as string | null,
    metrics: [
      { label: '总GMV', value: '¥1,254,300', change: '+15%', trend: 'up' },
      { label: '活跃商户', value: '45', change: '+2', trend: 'up' },
      { label: '活跃骑手', value: '28', change: '-1', trend: 'down' },
      { label: '总订单', value: '12,580', change: '+8%', trend: 'up' }
    ],
    topMerchants: [
      { rank: 1, name: '老上海本帮菜', gmv: 85000, orders: 1250 },
      { rank: 2, name: '川味小馆', gmv: 62000, orders: 980 },
      { rank: 3, name: '北京烤鸭', gmv: 58000, orders: 850 },
      { rank: 4, name: '西北面馆', gmv: 45000, orders: 1100 },
      { rank: 5, name: '日式料理', gmv: 42000, orders: 600 }
    ]
  },

  onLoad() {
    this.setData({ isLargeScreen: isLargeScreen() })
    this.loadData()
  },

  async loadData() {
    this.setData({ loading: true, error: null })
    try {
      // 模拟 API 加载延迟
      await new Promise((resolve) => setTimeout(resolve, 800))
      
      this.setData({
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
