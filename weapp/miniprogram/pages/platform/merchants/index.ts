import { responsiveBehavior } from '@/utils/responsive'
import {
  platformManagementService,
  type PlatformMerchantCard
} from '@/api/platform-management'
import { getErrorUserMessage } from '@/utils/user-facing'
import { type StatusTagTheme } from '@/utils/status-tag'
import { buildPlatformMerchantStatusView } from '@/utils/platform-status-view'

type NavHeightEvent = WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>
type TapEvent = WechatMiniprogram.CustomEvent & {
  currentTarget: {
    dataset: {
      id?: number | string
    }
  }
}

interface PlatformMerchantCardView extends PlatformMerchantCard {
  statusLabel: string
  statusTheme: StatusTagTheme
  regionText: string
  openLabel: string
  monthOrdersText: string
  complaintText: string
}

function buildMerchantCardView(item: PlatformMerchantCard): PlatformMerchantCardView {
  const statusView = buildPlatformMerchantStatusView(item.status)
  return {
    ...item,
    statusLabel: statusView.label,
    statusTheme: statusView.theme,
    regionText: item.region_name || '--',
    openLabel: item.is_open ? '营业中' : '休息中',
    monthOrdersText: `${item.month_orders || 0} 单`,
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
    merchants: [] as PlatformMerchantCardView[]
  },

  onLoad() {
    this.loadMerchants(true)
  },

  onShow() {
    if (!this.data.requesting && this.data.merchants.length > 0) {
      this.loadMerchants(true)
    }
  },

  onNavHeight(e: NavHeightEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 0 })
  },

  async onRefresh() {
    this.setData({ refreshing: true })
    try {
      await this.loadMerchants(true)
    } finally {
      this.setData({ refreshing: false })
    }
  },

  async onLoadMore() {
    if (!this.data.hasMore || this.data.loading) return
    await this.loadMerchants(false)
  },

  async loadMerchants(reset: boolean) {
    if (this.data.requesting) return

    const page = reset ? 1 : this.data.page + 1
    this.setData({ loading: true, requesting: true, error: null })
    try {
      const response = await platformManagementService.listPlatformMerchants({
        page,
        limit: this.data.limit
      })
      const incoming = (response.merchants || []).map(buildMerchantCardView)

      this.setData({
        merchants: reset ? incoming : this.data.merchants.concat(incoming),
        total: response.total || 0,
        page: response.page || page,
        hasMore: Boolean(response.has_more)
      })
    } catch (error: unknown) {
      this.setData({ error: getErrorUserMessage(error, '加载商户列表失败，请稍后重试') })
    } finally {
      this.setData({ loading: false, requesting: false })
    }
  },

  onRetry() {
    this.loadMerchants(true)
  },

  onDetailTap(e: TapEvent) {
    const id = Number(e.currentTarget.dataset.id || 0)
    if (!id) return
    wx.navigateTo({ url: `/pages/platform/merchants/detail?id=${id}` })
  }
})
