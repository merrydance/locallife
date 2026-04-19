import {
  operatorBasicManagementService,
  getFoodSafetyCaseStatusDisplay,
  type OperatorFoodSafetyCaseItem,
  type OperatorFoodSafetyCaseStatus,
  type OperatorFoodSafetyCaseStatusTheme
} from '../../../../api/operator-basic-management'
import { getErrorUserMessage } from '../../../../utils/user-facing'

type SafetyStatusFilter = '' | OperatorFoodSafetyCaseStatus

interface TabChangeDetail {
  value: SafetyStatusFilter
}

type FoodSafetyCaseView = {
  id: number
  merchant_id: number
  primary_product_key: string
  primary_product_label: string
  trigger_reason: string
  status: OperatorFoodSafetyCaseStatus
  status_label: string
  status_theme: OperatorFoodSafetyCaseStatusTheme
  suspended_at: string
  created_at: string
}

function formatProductLabel(item: OperatorFoodSafetyCaseItem): string {
  if (item.primary_product_label?.trim()) {
    return item.primary_product_label.trim()
  }
  if (item.primary_product_key?.trim()) {
    return item.primary_product_key.trim()
  }
  return '未识别问题商品'
}

Page({
  data: {
    cases: [] as FoodSafetyCaseView[],
    loading: false,
    loadingMore: false,
    initialLoading: true,
    error: '',
    navBarHeight: 88,
    status: '' as SafetyStatusFilter,
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

  adaptCase(item: OperatorFoodSafetyCaseItem): FoodSafetyCaseView {
    const statusDisplay = getFoodSafetyCaseStatusDisplay(item.status)
    return {
      id: item.id,
      merchant_id: item.merchant_id,
      primary_product_key: item.primary_product_key,
      primary_product_label: formatProductLabel(item),
      trigger_reason: item.trigger_reason,
      status: item.status,
      status_label: statusDisplay.label,
      status_theme: statusDisplay.theme,
      suspended_at: item.suspended_at,
      created_at: item.created_at
    }
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
      const res = await operatorBasicManagementService.getFoodSafetyCases({
        page: nextPage,
        limit: this.data.limit,
        status: this.data.status || undefined
      })
      const current = reset ? [] : this.data.cases
      this.setData({
        cases: [...current, ...(res.items || []).map((item) => this.adaptCase(item))],
        page: nextPage + 1,
        hasMore: Boolean(res.has_more),
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
