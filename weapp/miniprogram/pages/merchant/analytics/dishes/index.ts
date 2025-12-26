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
      trigger: 'item'
    },
    legend: {
      top: '5%',
      left: 'center'
    },
    series: [
      {
        name: '菜品分类',
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
          { value: 1048, name: '热菜' },
          { value: 735, name: '凉菜' },
          { value: 580, name: '主食' },
          { value: 484, name: '饮料' },
          { value: 300, name: '甜点' }
        ]
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
    topDishes: [
      { rank: 1, name: '招牌红烧肉', sales: 520, revenue: 19760, trend: 'up' },
      { rank: 2, name: '糖醋排骨', sales: 430, revenue: 13760, trend: 'up' },
      { rank: 3, name: '宫保鸡丁', sales: 310, revenue: 8680, trend: 'down' },
      { rank: 4, name: '麻婆豆腐', sales: 280, revenue: 3360, trend: 'up' },
      { rank: 5, name: '米饭', sales: 1200, revenue: 2400, trend: 'flat' }
    ]
  },

  onLoad() {
    this.setData({ isLargeScreen: isLargeScreen() })
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  }
})
