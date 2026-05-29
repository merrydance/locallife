import { responsiveBehavior } from '@/utils/responsive'
import {
  platformManagementService,
  type PlatformOperatorCard
} from '../_api/platform-management'
import { getErrorUserMessage } from '@/utils/user-facing'
import { type StatusTagTheme } from '../_main_shared/utils/status-tag'
import { buildPlatformOperatorStatusView } from '../_utils/platform-status-view'

type NavHeightEvent = WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>
type TapEvent = WechatMiniprogram.CustomEvent & {
  currentTarget: {
    dataset: {
      id?: number | string
    }
  }
}

interface PlatformOperatorCardView extends PlatformOperatorCard {
  statusLabel: string
  statusTheme: StatusTagTheme
  regionText: string
  merchantText: string
  complaintText: string
}

function buildOperatorCardView(item: PlatformOperatorCard): PlatformOperatorCardView {
  const statusView = buildPlatformOperatorStatusView(item.status)
  return {
    ...item,
    statusLabel: statusView.label,
    statusTheme: statusView.theme,
    regionText: `${item.region_count || 0} 个`,
    merchantText: `${item.merchant_count || 0} 家`,
    complaintText: `${item.complaint_count || 0} 次`
  }
}

Page({
  behaviors: [responsiveBehavior],
  data: {
    navBarHeight: 0,
    loading: false,
    requesting: false,
    refreshing: false,
    error: null as string | null,
    page: 1,
    limit: 20,
    total: 0,
    hasMore: false,
    operators: [] as PlatformOperatorCardView[]
  },

  onLoad() {
    this.loadOperators(true)
  },

  onShow() {
    if (!this.data.requesting && this.data.operators.length > 0) {
      this.loadOperators(true)
    }
  },

  onNavHeight(e: NavHeightEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 0 })
  },

  async onRefresh() {
    this.setData({ refreshing: true })
    try {
      await this.loadOperators(true)
    } finally {
      this.setData({ refreshing: false })
    }
  },

  async onLoadMore() {
    if (!this.data.hasMore || this.data.loading) return
    await this.loadOperators(false)
  },

  async loadOperators(reset: boolean) {
    if (this.data.requesting) return

    const page = reset ? 1 : this.data.page + 1
    this.setData({ loading: true, requesting: true, error: null })
    try {
      const response = await platformManagementService.listPlatformOperators({
        page,
        limit: this.data.limit
      })
      const incoming = (response.operators || []).map(buildOperatorCardView)

      this.setData({
        operators: reset ? incoming : this.data.operators.concat(incoming),
        total: response.total || 0,
        page: response.page || page,
        hasMore: Boolean(response.has_more)
      })
    } catch (error: unknown) {
      this.setData({ error: getErrorUserMessage(error, '加载运营商列表失败，请稍后重试') })
    } finally {
      this.setData({ loading: false, requesting: false })
    }
  },

  onRetry() {
    this.loadOperators(true)
  },

  onDetailTap(e: TapEvent) {
    const id = Number(e.currentTarget.dataset.id || 0)
    if (!id) return
    wx.navigateTo({ url: `/pages/platform/operators/detail?id=${id}` })
  }
})
