import { operatorBasicManagementService } from '../../../../api/operator-basic-management'
import { getStableBarHeights } from '../../../../utils/responsive'

interface CommissionRowView {
  date: string
  order_count: number
  total_gmv_fen: number
  total_commission_fen: number
}

interface CommissionListResponseLike {
  commissions?: Array<{
    items?: Array<{
      date: string
      order_count: number
      total_gmv: number
      commission: number
    }>
  }>
  items?: Array<{
    date: string
    order_count: number
    total_gmv: number
    commission: number
  }>
}

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
    commissionRows: [] as CommissionRowView[]
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
      const [overview, commissionList] = await Promise.all([
        operatorBasicManagementService.getFinanceOverview().catch(() => null),
        operatorBasicManagementService.getCommissionList({ page: 1, limit: 10 }).catch(() => null)
      ])

      const loadError = overview ? '' : '收入概览加载失败，请稍后重试'
      const commissionRows = this.adaptCommissionRows(commissionList)
      const commissionError = commissionList ? '' : '佣金明细加载失败，请稍后重试'

      this.setData({
        totalIncomeFen: overview?.total?.operator_income ?? 0,
        currentMonthIncomeFen: overview?.current_month?.operator_income ?? 0,
        currentMonthGmvFen: overview?.current_month?.total_gmv ?? 0,
        currentMonthOrders: overview?.current_month?.total_orders ?? 0,
        currentMonthCommissionFen: overview?.current_month?.total_commission ?? 0,
        operatorShareRatio: overview?.operator_share_ratio ?? 0,
        loadError,
        commissionRows,
        commissionError,
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

  adaptCommissionRows(response: unknown): CommissionRowView[] {
    if (!response || typeof response !== 'object') {
      return []
    }

    const payload = response as CommissionListResponseLike
    const rawItems = Array.isArray(payload.items)
      ? payload.items
      : Array.isArray(payload.commissions)
        ? payload.commissions.reduce<Array<{ date: string, order_count: number, total_gmv: number, commission: number }>>((accumulator, item) => {
          if (Array.isArray(item.items)) {
            accumulator.push(...item.items)
          }
          return accumulator
        }, [])
        : []

    return rawItems.slice(0, 10).map((item) => ({
      date: item.date,
      order_count: Number(item.order_count || 0),
      total_gmv_fen: Number(item.total_gmv || 0),
      total_commission_fen: Number(item.commission || 0)
    }))
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
