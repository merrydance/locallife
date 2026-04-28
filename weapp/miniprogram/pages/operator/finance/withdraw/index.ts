import {
  loadOperatorFinancePageData,
  type OperatorCommissionRowView
} from '../../../../services/operator-finance'
import { getStableBarHeights } from '../../../../utils/responsive'

Page({
  data: {
    navBarHeight: 88,
    loadingOverview: true,
    loadError: '',
    totalIncomeFen: 0,
    currentMonthIncomeFen: 0,
    currentMonthGmvFen: 0,
    currentMonthOrders: 0,
    currentMonthCommissionFen: 0,
    operatorShareRatio: 0,
    commissionLoading: true,
    commissionError: '',
    commissionRows: [] as OperatorCommissionRowView[]
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.loadOverview()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
    this.setData({ navBarHeight: e.detail?.navBarHeight || this.data.navBarHeight })
  },

  onPullDownRefresh() {
    this.loadOverview()
  },

  async loadOverview() {
    this.setData({ loadingOverview: true, commissionLoading: true, loadError: '', commissionError: '' })
    try {
      const nextView = await loadOperatorFinancePageData()

      this.setData({
        ...nextView,
        commissionLoading: false
      })
    } catch (error) {
      console.error('加载运营商财务概览失败:', error)
      this.setData({ loadError: '收入概览加载失败，请稍后重试', commissionError: '佣金明细加载失败，请稍后重试', commissionLoading: false })
    } finally {
      this.setData({ loadingOverview: false })
      wx.stopPullDownRefresh()
    }
  },

  onRetryLoad() {
    this.loadOverview()
  },

  formatFen(fen: number): string {
    return (fen / 100).toFixed(2)
  },

  formatShareRatio(ratio: number): string {
    if (!Number.isFinite(ratio) || ratio <= 0) return '--'
    return `${(ratio * 100).toFixed(0)}%`
  }
})
