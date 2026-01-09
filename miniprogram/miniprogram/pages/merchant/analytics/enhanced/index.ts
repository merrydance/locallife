import { isLargeScreen } from '@/utils/responsive'
import * as echarts from '../../libs/echarts'
import { getMerchantDashboard } from '../../../api/merchant'
import { logger } from '../../../utils/logger'
import { ErrorHandler } from '../../../utils/error-handler'

const app = getApp<IAppOption>()
let chartLine: any = null
let chartPie: any = null

function initChart(canvas: any, width: number, height: number, dpr: number) {
  const chart = echarts.init(canvas, null, {
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
      containLabel: true
    },
    tooltip: {
      trigger: 'axis'
    },
    xAxis: {
      type: 'category',
      data: [] // Filled by API
    },
    yAxis: {
      type: 'value'
    },
    series: [{
      data: [], // Filled by API
      type: 'line',
      smooth: true,
      itemStyle: {
        color: '#0052D9'
      },
      areaStyle: {
        color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: 'rgba(0, 82, 217, 0.5)' },
          { offset: 1, color: 'rgba(0, 82, 217, 0)' }
        ])
      }
    }]
  }

  chart.setOption(option)
  chartLine = chart
  return chart
}

function initPieChart(canvas: any, width: number, height: number, dpr: number) {
  const chart = echarts.init(canvas, null, {
    width,
    height,
    devicePixelRatio: dpr
  })
  canvas.setChart(chart)

  // Mock data for Pie Chart (API doesn't provide time distribution yet)
  const option = {
    tooltip: {
      trigger: 'item'
    },
    legend: {
      bottom: '0%',
      left: 'center'
    },
    series: [
      {
        name: '时段分布',
        type: 'pie',
        radius: ['40%', '70%'],
        avoidLabelOverlap: false,
        itemStyle: {
          borderRadius: 10,
          borderColor: '#fff',
          borderWidth: 2
        },
        label: {
          show: false,
          position: 'center'
        },
        emphasis: {
          label: {
            show: true,
            fontSize: '20',
            fontWeight: 'bold'
          }
        },
        labelLine: {
          show: false
        },
        data: [
          { value: 1048, name: '午餐' },
          { value: 735, name: '晚餐' },
          { value: 580, name: '夜宵' },
          { value: 484, name: '下午茶' }
        ]
      }
    ]
  }

  chart.setOption(option)
  chartPie = chart
  return chart
}

Page({
  data: {
    ec: {
      onInit: initChart
    },
    ecPie: {
      onInit: initPieChart
    },
    isLargeScreen: false,
    navBarHeight: 88,
    metrics: [
      { label: '今日GMV', value: '-', change: '-', trend: 'up' },
      { label: '今日订单', value: '-', change: '-', trend: 'up' },
      { label: '待处理', value: '-', change: '-', trend: 'up' },
      { label: '近7日', value: '趋势图', change: '-', trend: 'up' }
    ] as any[]
  },

  onLoad() {
    this.setData({ isLargeScreen: isLargeScreen() })

    if (app.globalData.merchantId) {
      this.loadData()
    } else {
      app.userInfoReadyCallback = () => {
        if (app.globalData.merchantId) {
          this.loadData()
        }
      }
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadData() {
    try {
      const merchantId = app.globalData.merchantId!
      const res = await getMerchantDashboard(merchantId)

      // Update Metrics
      const metrics = [
        { label: '今日GMV', value: `¥${(res.today_sales / 100).toFixed(2)}`, change: '-', trend: 'up' },
        { label: '今日订单', value: String(res.today_orders), change: '-', trend: 'up' },
        { label: '待处理', value: String(res.pending_orders), change: '-', trend: 'down' }, // Assuming lower is better or just trend
        { label: '近7日', value: '趋势图', change: '-', trend: 'up' }
      ]
      this.setData({ metrics })

      // Update Line Chart
      if (chartLine && res.seven_days_sales) {
        const dates = res.seven_days_sales.map((d) => d.date.slice(5)) // MM-DD
        const sales = res.seven_days_sales.map((d) => d.amount / 100) // Yuan

        chartLine.setOption({
          xAxis: {
            data: dates
          },
          series: [{
            data: sales
          }]
        })
      }

    } catch (error) {
      ErrorHandler.handle(error, 'Analytics.loadData')
    }
  }
})
