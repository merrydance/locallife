import { isLargeScreen } from '@/utils/responsive'
import * as echarts from '../libs/echarts'

function initChart(canvas: any, width: number, height: number, dpr: number) {
  const chart = echarts.init(canvas, null, {
    width,
    height,
    devicePixelRatio: dpr
  })
  canvas.setChart(chart)

  const option = {
    tooltip: {
      trigger: 'axis',
      axisPointer: {
        type: 'shadow'
      }
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '3%',
      containLabel: true
    },
    xAxis: [
      {
        type: 'category',
        data: ['海淀', '朝阳', '西城', '东城', '丰台', '石景山', '通州'],
        axisTick: {
          alignWithLabel: true
        }
      }
    ],
    yAxis: [
      {
        type: 'value'
      }
    ],
    series: [
      {
        name: '订单分布',
        type: 'bar',
        barWidth: '60%',
        data: [120, 200, 150, 80, 70, 110, 130],
        itemStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: '#0052D9' },
            { offset: 1, color: '#00A870' }
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
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  }
})
