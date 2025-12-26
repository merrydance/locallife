import { isLargeScreen } from '@/utils/responsive'
import * as echarts from '../../libs/echarts'

function initChart(canvas: any, width: number, height: number, dpr: number) {
  const chart = echarts.init(canvas, null, {
    width,
    height,
    devicePixelRatio: dpr
  })
  canvas.setChart(chart)

  const option = {
    tooltip: {
      trigger: 'axis'
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '3%',
      containLabel: true
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: ['11-14', '11-15', '11-16', '11-17', '11-18', '11-19', '11-20']
    },
    yAxis: {
      type: 'value'
    },
    series: [
      {
        name: '销售额',
        type: 'line',
        stack: 'Total',

        emphasis: {
          focus: 'series'
        },
        data: [1200, 1320, 1010, 1340, 900, 2300, 2100],
        itemStyle: { color: '#0052D9' },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(0, 82, 217, 0.5)' },
            { offset: 1, color: 'rgba(0, 82, 217, 0)' }
          ])
        }
      },
      {
        name: '订单量',
        type: 'line',
        stack: 'Total',

        emphasis: {
          focus: 'series'
        },
        data: [220, 182, 191, 234, 290, 330, 310],
        itemStyle: { color: '#00A870' },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(0, 168, 112, 0.5)' },
            { offset: 1, color: 'rgba(0, 168, 112, 0)' }
          ])
        }
      }
    ]
  }

  chart.setOption(option)
  return chart
}

Page({
  data: {
    ec: {
      onInit: initChart
    },
    isLargeScreen: false,
    navBarHeight: 88,
    salesData: [
      { date: '2024-11-20', gmv: 2100, orders: 310, avg: 6.7 },
      { date: '2024-11-19', gmv: 2300, orders: 330, avg: 6.9 },
      { date: '2024-11-18', gmv: 900, orders: 290, avg: 3.1 },
      { date: '2024-11-17', gmv: 1340, orders: 234, avg: 5.7 },
      { date: '2024-11-16', gmv: 1010, orders: 191, avg: 5.2 },
      { date: '2024-11-15', gmv: 1320, orders: 182, avg: 7.2 },
      { date: '2024-11-14', gmv: 1200, orders: 220, avg: 5.4 }
    ]
  },

  onLoad() {
    this.setData({ isLargeScreen: isLargeScreen() })
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  }
})
