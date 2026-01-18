import { isLargeScreen } from '../../../utils/responsive'

const app = getApp<IAppOption>()

interface MetricItem {
  label: string
  value: string
  status: 'GOOD' | 'WARNING' | 'BAD'
}

interface ViolationItem {
  id: number
  type: 'WARNING' | 'VIOLATION'
  title: string
  desc: string
  created_at: string
}

Page({
  data: {
    score: 0,
    level: '健康',
    levelDesc: '经营状况良好',
    metrics: [] as MetricItem[],
    violations: [] as ViolationItem[],
    isLargeScreen: false,
    navBarHeight: 88,
    loading: false
  },

  onLoad() {
    this.setData({ isLargeScreen: isLargeScreen() })
    this.loadHealthInfo()
  },

  onShow() {
    this.loadHealthInfo()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadHealthInfo() {
    this.setData({
      score: 0,
      level: '已下线',
      levelDesc: '信用分功能已下线',
      metrics: [],
      violations: [],
      loading: false
    })
    wx.showToast({ title: '信用分功能已下线', icon: 'none' })
  },

  calculateLevelInfo(score: number): { level: string; levelDesc: string } {
    if (score >= 90) {
      return { level: '优秀', levelDesc: '经营状况优秀' }
    } else if (score >= 80) {
      return { level: '健康', levelDesc: '经营状况良好' }
    } else if (score >= 60) {
      return { level: '一般', levelDesc: '需要改进' }
    } else {
      return { level: '警告', levelDesc: '存在风险' }
    }
  },

  onAppeal(e: WechatMiniprogram.CustomEvent) {
    const { id } = e.currentTarget.dataset
    wx.navigateTo({ url: `/pages/merchant/appeals/index?id=${id}` })
  }
})
