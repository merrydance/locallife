import {
  loadOperatorFinancePageData,
  type OperatorCommissionRowView
} from '../../_services/operator-finance'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'

Page({
  data: {
    navBarHeight: 88,
    loadingOverview: true,
    loadError: '',
    totalIncomeFen: 0,
    totalIncomeDisplay: '¥0.00',
    currentMonthIncomeFen: 0,
    currentMonthIncomeDisplay: '¥0.00',
    currentMonthGmvFen: 0,
    currentMonthGmvDisplay: '¥0.00',
    currentMonthOrders: 0,
    currentMonthCommissionFen: 0,
    currentMonthCommissionDisplay: '¥0.00',
    operatorShareRatio: 0,
    operatorShareRatioDisplay: '--',
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
      logger.error('Load operator finance overview failed action=load_overview role=operator', error, 'operator-finance-withdraw')
      this.setData({ loadError: '收入概览加载失败，请稍后重试', commissionError: '佣金明细加载失败，请稍后重试', commissionLoading: false })
    } finally {
      this.setData({ loadingOverview: false })
      wx.stopPullDownRefresh()
    }
  },

  onRetryLoad() {
    this.loadOverview()
  },

  onOpenSettlementAccount() {
    wx.navigateTo({ url: '/pages/operator/finance/settlement-account/index' })
  },

  onOpenBills() {
    wx.navigateTo({ url: '/pages/operator/finance/bills/index' })
  },

  onOpenWithdrawals() {
    wx.navigateTo({ url: '/pages/operator/finance/withdrawals/index' })
  }
})
