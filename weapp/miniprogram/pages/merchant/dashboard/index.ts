import { getStableBarHeights } from '../../../utils/responsive'
import { MerchantOrderManagementService, KitchenDisplayService } from '../../../api/order-management'
import { logger } from '../../../utils/logger'
import dayjs from 'dayjs'
import * as echarts from '@/libs/echarts'
import { wsManager, WSMessageType } from '../../../utils/websocket'

type EChartInstance = ReturnType<typeof echarts.init>
type WsUnsubscribe = () => void

interface ChartCanvas {
  setChart: (chart: EChartInstance) => void
}

interface MerchantDailyStats {
  total_revenue: number
}

let chart: EChartInstance | null = null

function initChart(canvas: ChartCanvas, width: number, height: number, dpr: number) {
  chart = echarts.init(canvas, null, {
    width,
    height,
    devicePixelRatio: dpr
  })
  canvas.setChart(chart)

  const option = {
    grid: {
      left: '3%',
      right: '4%',
      bottom: '3%',
      top: '10%',
      containLabel: true
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: [],
      axisLine: { lineStyle: { color: '#999' } },
      axisLabel: { color: '#999', fontSize: 10 }
    },
    yAxis: {
      type: 'value',
      axisLine: { show: false },
      axisTick: { show: false },
      splitLine: { lineStyle: { type: 'dashed', color: '#eee' } },
      axisLabel: { color: '#999', fontSize: 10 }
    },
    series: [{
      type: 'line',
      smooth: true,
      data: [],
      symbol: 'circle',
      symbolSize: 6,
      itemStyle: { color: '#0052D9' },
      areaStyle: {
        color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: 'rgba(0, 82, 217, 0.2)' },
          { offset: 1, color: 'rgba(0, 82, 217, 0)' }
        ])
      }
    }]
  }

  chart.setOption(option)
  return chart
}

Page({
  data: {
    navBarHeight: 88,
    initialLoading: true,
    isOpen: true,
    merchantInfo: {
      name: '示例餐厅',
      merchant_id: 0
    },
    todayStats: {
      revenue: 0,
      orderCount: 0,
      avgOrderPrice: 0
    },
    pendingCounts: {
      takeout: 0,
      reservation: 0,
      exceptions: 0
    },
    hotDishes: [
      { id: 1, name: '招牌红烧肉', sales: 128, revenue: 627200 },
      { id: 2, name: '清蒸鲈鱼', sales: 95, revenue: 836000 },
      { id: 3, name: '手撕包菜', sales: 88, revenue: 193600 },
      { id: 4, name: '酸菜鱼', sales: 72, revenue: 561600 },
      { id: 5, name: '扬州炒饭', sales: 65, revenue: 130000 }
    ],
    loading: false,
    ec: {
      onInit: initChart
    },
    _wsListeners: [] as WsUnsubscribe[]
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    
    // 从全局或存储中获取商户信息
    const currentMerchant = wx.getStorageSync('current_merchant')
    if (currentMerchant) {
      this.setData({ merchantInfo: currentMerchant })
    }
    this.initWebSocket()
  },

  onShow() {
    this.refreshData()
  },

  onHide() {
    this.cleanupWebSocket()
  },

  onUnload() {
    this.cleanupWebSocket()
  },

  initWebSocket() {
    wsManager.connect()
    this.cleanupWebSocket()

    const sub = wsManager.on(WSMessageType.NOTIFICATION, (data) => {
      logger.info('Merchant received notification', data)
      const notification =
        typeof data === 'object' && data !== null
          ? (data as { type?: string })
          : {}
      // 检查是否是订单通知 (后端 params.Type = "order")
      if (notification.type === 'order') {
        wx.vibrateLong()
        wx.showModal({
          title: '新订单提醒',
          content: '您有新的订单需要处理',
          confirmText: '去处理',
          success: (res) => {
            if (res.confirm) {
              this.onPendingTakeout()
            }
          }
        })
        this.refreshData()
      }
    })

    this.data._wsListeners = [sub]
  },

  cleanupWebSocket() {
    if (this.data._wsListeners) {
      this.data._wsListeners.forEach((unsub) => unsub())
      this.data._wsListeners = []
    }
  },

  async refreshData() {
    if (this.data.loading) return
    this.setData({ loading: true })

    try {
      const today = dayjs().format('YYYY-MM-DD')
      
      // 1. 获取今日统计
      try {
        const stats = await MerchantOrderManagementService.getOrderStats({
          start_date: today,
          end_date: today
        })
        this.setData({
          todayStats: {
            revenue: stats.total_revenue,
            orderCount: stats.total_orders,
            avgOrderPrice: stats.avg_order_value
          }
        })
      } catch (err) {
        logger.error('Failed to fetch merchant stats', err)
      }

      // 2. 获取待处理计数 (使用真实的 KDS 统计)
      const kitchenOrders = await KitchenDisplayService.getKitchenOrders()
      const kStats = kitchenOrders.stats

      this.setData({
        'pendingCounts.takeout': kStats?.new_count || 0,
        'pendingCounts.reservation': kStats?.preparing_count || 0, // 借用制作中作为任务提醒
        'pendingCounts.exceptions': kStats?.orders_behind_schedule || 0 // 使用超时单作为异常提醒
      })

      // 3. 加载趋势图
      this.loadTrendData()

    } catch (err) {
      logger.error('Merchant dashboard refresh failed', err)
    } finally {
      this.setData({ loading: false, initialLoading: false })
      wx.stopPullDownRefresh()
    }
  },

  async loadTrendData() {
    try {
      // 模拟近7天日期
      const dates = []
      const promises: Array<Promise<MerchantDailyStats>> = []
      for (let i = 6; i >= 0; i--) {
        const d = dayjs().subtract(i, 'day').format('YYYY-MM-DD')
        dates.push(dayjs(d).format('MM-DD'))
        promises.push(MerchantOrderManagementService.getOrderStats({ start_date: d, end_date: d }))
      }

      const results = await Promise.all(promises)
      const revenues = results.map((r) => r.total_revenue / 100)

      if (chart) {
        chart.setOption({
          xAxis: { data: dates },
          series: [{ data: revenues }]
        })
      }
    } catch (err) {
      logger.error('Load trend data failed', err)
    }
  },

  onPullDownRefresh() {
    this.refreshData()
  },

  onToggleOpen() {
    this.setData({ isOpen: !this.data.isOpen })
    wx.showToast({
      title: this.data.isOpen ? '营业中' : '休息中',
      icon: 'success'
    })
  },

  onManageDishes() {
    wx.navigateTo({ url: '/pages/merchant/dishes/index' })
  },

  onManageInventory() {
    wx.navigateTo({ url: '/pages/merchant/tables/index' })
  },

  onManageReviews() {
    wx.showToast({ title: '跳转评价管理', icon: 'none' })
  },

  onPromotionConfig() {
    wx.showToast({ title: '跳转营销配置', icon: 'none' })
  },

  onFinanceInfo() {
    wx.showToast({ title: '跳转财务流水', icon: 'none' })
  },

  onPrinterSettings() {
    wx.showToast({ title: '跳转打印设置', icon: 'none' })
  },

  onManageChain() {
    wx.showToast({ title: '跳转连锁管理', icon: 'none' })
  },

  onPendingTakeout() {
    wx.navigateTo({ url: '/pages/merchant/orders/list/index?status=paid' })
  },

  onPendingReservations() {
    wx.navigateTo({ url: '/pages/merchant/orders/list/index?status=paid' }) // 暂时共用
  },

  onExceptionOrders() {
    wx.showToast({ title: '跳转异常单', icon: 'none' })
  }
})
