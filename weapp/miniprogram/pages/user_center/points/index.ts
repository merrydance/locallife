import { PointsService, PointsHistoryItem } from '../../../api/points'
import { logger } from '../../../utils/logger'

Page({
  data: {
    points: 0,
    history: [] as PointsHistoryItem[],
    navBarHeight: 88,
    loading: false
  },

  onLoad() {
    this.loadData()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadData() {
    this.setData({ loading: true })
    try {
      const [summary, historyRes] = await Promise.all([
        PointsService.getSummary(),
        PointsService.getHistory(1, 20)
      ])

      this.setData({
        points: summary.balance,
        history: historyRes.list,
        loading: false
      })
    } catch (error) {
      logger.error('Load points failed', error)
      // Fallback or error state
      this.setData({ loading: false })
    }
  },

  onExchange() {
    wx.showToast({ title: '积分商城开发中', icon: 'none' })
  }
})
