import {
  loadOperatorFoodSafetyCaseListPageData,
  type OperatorFoodSafetyCaseView,
  type OperatorSafetyStatusFilter
} from '../../../../services/operator-safety'
import { getErrorUserMessage } from '../../../../utils/user-facing'

interface TabChangeDetail {
  value: OperatorSafetyStatusFilter
}

Page({
  data: {
    cases: [] as OperatorFoodSafetyCaseView[],
    loading: false,
    loadingMore: false,
    initialLoading: true,
    error: '',
    navBarHeight: 88,
    status: '' as OperatorSafetyStatusFilter,
    page: 1,
    limit: 20,
    hasMore: false
  },

  onLoad() {
    this.loadCases(true)
  },

  onShow() {
    if (!this.data.initialLoading) {
      this.loadCases(true)
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  onPullDownRefresh() {
    this.loadCases(true).finally(() => {
      wx.stopPullDownRefresh()
    })
  },

  async loadCases(reset = false) {
    if (this.data.loading || (this.data.loadingMore && !reset)) return
    const nextPage = reset ? 1 : this.data.page
    if (reset) {
      this.setData({ loading: true, error: '' })
    } else {
      this.setData({ loadingMore: true })
    }

    try {
      const result = await loadOperatorFoodSafetyCaseListPageData({
        pageId: nextPage,
        pageSize: this.data.limit,
        status: this.data.status
      })
      const current = reset ? [] : this.data.cases
      this.setData({
        cases: [...current, ...result.cases],
        page: result.nextPage,
        hasMore: result.hasMore,
        loading: false,
        loadingMore: false,
        initialLoading: false
      })
    } catch (error: unknown) {
      const message = getErrorUserMessage(error, '加载食安案件失败，请稍后重试')
      this.setData({ loading: false, loadingMore: false, initialLoading: false, error: message })
    }
  },

  onTabChange(e: WechatMiniprogram.CustomEvent<TabChangeDetail>) {
    this.setData({ status: e.detail.value })
    this.loadCases(true)
  },

  onLoadMore() {
    if (!this.data.hasMore || this.data.loading) return
    this.loadCases(false)
  },

  onDetail(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    wx.navigateTo({ url: `/pages/operator/safety/detail/index?id=${id}` })
  }
})
