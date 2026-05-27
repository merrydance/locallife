import { responsiveBehavior } from '@/utils/responsive'
import {
  platformManagementService,
  type PlatformRiderCard
} from '@/api/platform-management'
import { getErrorUserMessage } from '@/utils/user-facing'
import { resolveStatusTagTheme, type StatusTagTheme } from '@/utils/status-tag'

type NavHeightEvent = WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>
type TapEvent = WechatMiniprogram.CustomEvent & {
  currentTarget: {
    dataset: {
      id?: number | string
    }
  }
}

interface PlatformRiderCardView extends PlatformRiderCard {
  regionText: string
  activeLabel: string
  activeTheme: StatusTagTheme
  complaintText: string
  statusLabel: string
}

function riderStatusLabel(status: string): string {
  switch (status) {
    case 'active':
      return '可接单'
    case 'approved':
      return '已通过'
    case 'suspended':
      return '暂停接单'
    default:
      return status || '--'
  }
}

function buildRiderCardView(item: PlatformRiderCard): PlatformRiderCardView {
  return {
    ...item,
    regionText: item.region_name || '--',
    activeLabel: item.active ? '近3天活跃' : '近3天未接单',
    activeTheme: item.active ? resolveStatusTagTheme('success') : resolveStatusTagTheme('warning'),
    complaintText: `${item.complaint_count || 0} 次`,
    statusLabel: riderStatusLabel(item.status)
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
    riders: [] as PlatformRiderCardView[]
  },

  onLoad() {
    this.loadRiders(true)
  },

  onShow() {
    if (!this.data.requesting && this.data.riders.length > 0) {
      this.loadRiders(true)
    }
  },

  onNavHeight(e: NavHeightEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 0 })
  },

  async onRefresh() {
    this.setData({ refreshing: true })
    try {
      await this.loadRiders(true)
    } finally {
      this.setData({ refreshing: false })
    }
  },

  async onLoadMore() {
    if (!this.data.hasMore || this.data.loading) {
      return
    }
    await this.loadRiders(false)
  },

  async loadRiders(reset: boolean) {
    if (this.data.requesting) {
      return
    }

    const page = reset ? 1 : this.data.page + 1
    this.setData({ loading: true, requesting: true, error: null })
    try {
      const response = await platformManagementService.listPlatformRiders({
        page,
        limit: this.data.limit
      })
      const incoming = (response.riders || []).map(buildRiderCardView)

      this.setData({
        riders: reset ? incoming : this.data.riders.concat(incoming),
        total: response.total || 0,
        page: response.page || page,
        hasMore: Boolean(response.has_more)
      })
    } catch (error: unknown) {
      this.setData({ error: getErrorUserMessage(error, '加载骑手列表失败，请稍后重试') })
    } finally {
      this.setData({ loading: false, requesting: false })
    }
  },

  onRetry() {
    this.loadRiders(true)
  },

  onDetailTap(e: TapEvent) {
    const id = Number(e.currentTarget.dataset.id || 0)
    if (!id) return
    wx.navigateTo({ url: `/pages/platform/riders/detail?id=${id}` })
  }
})
