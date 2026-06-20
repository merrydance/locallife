import { isLargeScreen } from '@/utils/responsive'
import {
  loadOperatorRiderListPageData,
  parseOperatorRiderStatusFilter,
  type OperatorRiderFilterStatus,
  type OperatorRiderListView
} from '../_services/operator-rider-management'
import { getErrorUserMessage } from '../../../utils/user-facing'

type RiderListPageOptions = {
  region_id?: string
  status?: string
}

type RiderListDataset = {
  id?: number
  name?: string
}

let riderListRequestSeq = 0

Page({
  data: {
    navBarHeight: 88,
    isLargeScreen: false,
    loading: false,
    loadingMore: false,
    refreshing: false,
    initialLoading: true,
    error: '',
    page: 1,
    limit: 20,
    total: 0,
    hasMore: true,
    riders: [] as OperatorRiderListView[],
    regionId: 0,
    statusFilter: '' as OperatorRiderFilterStatus,
    searchKeyword: '',
    searchTimer: null as number | null
  },

  onLoad(options: RiderListPageOptions) {
    const regionId = options.region_id ? parseInt(options.region_id) : 0
    const statusFilter = parseOperatorRiderStatusFilter(options.status)
    this.setData({
      isLargeScreen: isLargeScreen(),
      regionId,
      statusFilter
    })
    this.loadRiders(true)
  },

  onShow() {
    if (!this.data.initialLoading) {
      this.loadRiders(true, true)
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  onPullDownRefresh() {
    this.setData({ refreshing: true })
    this.loadRiders(true).finally(() => {
      this.setData({ refreshing: false })
      wx.stopPullDownRefresh()
    })
  },

  async loadRiders(refresh: boolean, silent = false) {
    if (!refresh && (this.data.loading || this.data.loadingMore)) return
    const requestSeq = ++riderListRequestSeq

    try {
      if (refresh) {
        this.setData({ loading: true, error: '', page: 1, ...(silent ? {} : { initialLoading: this.data.initialLoading }) })
      } else {
        this.setData({ loadingMore: true })
      }

      const result = await loadOperatorRiderListPageData({
        pageId: refresh ? 1 : this.data.page,
        pageSize: this.data.limit,
        regionId: this.data.regionId,
        statusFilter: this.data.statusFilter,
        searchKeyword: this.data.searchKeyword
      })

      if (requestSeq !== riderListRequestSeq) {
        return
      }

      const riders = refresh ? result.riders : [...this.data.riders, ...result.riders]
      const total = refresh ? result.total : Number(result.total || riders.length)

      this.setData({
        riders,
        page: refresh ? result.nextPage : this.data.page + 1,
        total,
        hasMore: riders.length < total,
        loading: false,
        loadingMore: false,
        initialLoading: false
      })
    } catch (error: unknown) {
      if (requestSeq !== riderListRequestSeq) {
        return
      }
      const message = getErrorUserMessage(error, '加载骑手失败，请稍后重试')
      this.setData({ loading: false, loadingMore: false, initialLoading: false, error: message })
    }
  },

  onRetry() {
    this.loadRiders(true)
  },

  onLoadMore() {
    if (this.data.hasMore && !this.data.loading && !this.data.loadingMore) {
      this.loadRiders(false)
    }
  },

  onSearchChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const searchKeyword = e.detail.value || ''
    this.setData({ searchKeyword })

    if (this.data.searchTimer) {
      clearTimeout(this.data.searchTimer)
    }

    const timer = setTimeout(() => {
      this.loadRiders(true)
    }, 500)

    this.setData({ searchTimer: timer })
  },

  onSearchClear() {
    this.setData({ searchKeyword: '', page: 1 })
    this.loadRiders(true)
  },

  onStatusFilterChange(e: WechatMiniprogram.CustomEvent<{ value: OperatorRiderFilterStatus }>) {
    this.setData({ statusFilter: e.detail.value, page: 1 })
    this.loadRiders(true)
  },

  onTapRider(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as RiderListDataset
    if (!id) return
    wx.navigateTo({ url: `/pages/operator/riders/detail/index?id=${id}` })
  }
})
