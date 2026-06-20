import { isLargeScreen } from '@/utils/responsive'
import {
  loadOperatorMerchantListPageData,
  parseOperatorMerchantStatusFilter,
  type OperatorMerchantFilterStatus,
  type OperatorMerchantListView
} from '../_services/operator-merchant-management'
import { getErrorUserMessage } from '../../../utils/user-facing'

interface MerchantListPageDataset {
  id?: number
  name?: string
}

interface MerchantListPageOptions {
  region_id?: string
  status?: string
}

let merchantListRequestSeq = 0

Page({
  data: {
    loading: false,
    loadingMore: false,
    refreshing: false,
    initialLoading: true,
    error: null as string | null,
    navBarHeight: 88,
    isLargeScreen: false,

    merchants: [] as OperatorMerchantListView[],

    page: 1,
    limit: 20,
    total: 0,
    hasMore: true,

    regionId: 0,
    searchKeyword: '',
    statusFilter: '' as OperatorMerchantFilterStatus,

    searchTimer: null as number | null
  },

  onLoad(options: MerchantListPageOptions) {
    const regionId = options.region_id ? parseInt(options.region_id) : 0
    const statusFilter = parseOperatorMerchantStatusFilter(options.status)
    this.setData({
      isLargeScreen: isLargeScreen(),
      regionId,
      statusFilter
    })
    this.loadMerchants(true)
  },

  onShow() {
    if (!this.data.initialLoading) {
      this.loadMerchants(true, true)
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  onRetry() {
    this.loadMerchants(true)
  },

  onPullDownRefresh() {
    this.setData({ refreshing: true, page: 1 })
    this.loadMerchants(true).finally(() => {
      this.setData({ refreshing: false })
      wx.stopPullDownRefresh()
    })
  },

  async loadMerchants(refresh = false, silent = false) {
    if (!refresh && (this.data.loading || this.data.loadingMore)) return
    const requestSeq = ++merchantListRequestSeq

    try {
      if (refresh) {
        if (!silent) {
          this.setData({ loading: true, loadingMore: false, error: null, page: 1 })
        } else {
          this.setData({ loading: true, loadingMore: false, page: 1 })
        }
      } else {
        this.setData({ loadingMore: true })
      }

      const result = await loadOperatorMerchantListPageData({
        pageId: refresh ? 1 : this.data.page,
        pageSize: this.data.limit,
        regionId: this.data.regionId,
        statusFilter: this.data.statusFilter,
        searchKeyword: this.data.searchKeyword
      })
      if (requestSeq !== merchantListRequestSeq) {
        return
      }
      const merchants = refresh ? result.merchants : [...this.data.merchants, ...result.merchants]
      const total = refresh ? result.total : Number(result.total || merchants.length)
      const hasMore = merchants.length < total

      this.setData({
        merchants,
        total,
        hasMore,
        page: refresh ? result.nextPage : this.data.page + 1,
        loading: false,
        loadingMore: false,
        initialLoading: false,
        error: null
      })
    } catch (error) {
      if (requestSeq !== merchantListRequestSeq) {
        return
      }
      console.error('加载商户列表失败:', error)
      if (refresh) {
        this.setData({
          error: getErrorUserMessage(error, '加载商户列表失败，请稍后重试'),
          initialLoading: false,
          loading: false,
          loadingMore: false
        })
      } else {
        this.setData({ loading: false, loadingMore: false })
        wx.showToast({ title: getErrorUserMessage(error, '加载更多失败，请稍后重试'), icon: 'none' })
      }
    }
  },

  onLoadMore() {
    if (this.data.hasMore && !this.data.loading && !this.data.loadingMore) {
      this.loadMerchants(false)
    }
  },

  onSearchChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const keyword = e.detail.value || ''
    this.setData({ searchKeyword: keyword })

    if (this.data.searchTimer) {
      clearTimeout(this.data.searchTimer)
    }

    const timer = setTimeout(() => {
      this.setData({ page: 1 })
      this.loadMerchants(true)
    }, 500)

    this.setData({ searchTimer: timer })
  },

  onSearchClear() {
    this.setData({ searchKeyword: '', page: 1 })
    this.loadMerchants(true)
  },

  onStatusFilterChange(e: WechatMiniprogram.CustomEvent<{ value: OperatorMerchantFilterStatus }>) {
    this.setData({
      statusFilter: e.detail.value,
      page: 1
    })
    this.loadMerchants(true)
  },

  onMerchantTap(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as MerchantListPageDataset
    if (!id) return
    wx.navigateTo({ url: `/pages/operator/merchants/detail/index?id=${id}` })
  }
})
