import { isLargeScreen } from '@/utils/responsive'
import {
  getMerchantStatusDisplay,
  operatorMerchantManagementService,
  parseMerchantStatusFilter,
  OperatorMerchantItem,
  MerchantQueryParams,
  MerchantStatus
} from '../../../api/operator-merchant-management'
import { getErrorUserMessage } from '../../../utils/user-facing'

interface MerchantListPageDataset {
  id?: number
  name?: string
}

interface MerchantListPageOptions {
  region_id?: string
  status?: string
}

interface MerchantView extends OperatorMerchantItem {
  status_label: string
  status_theme: 'success' | 'warning' | 'default'
  rating_display: string
  order_count_display: number
  total_gmv_display: string
  commission_amount_display: string
  region_name_display: string
  category_display: string
}

function adaptMerchantItem(item: OperatorMerchantItem): MerchantView {
  const statusDisplay = getMerchantStatusDisplay(item.status)
  return {
    ...item,
    status: statusDisplay.normalizedStatus,
    status_label: statusDisplay.label,
    status_theme: statusDisplay.theme,
    rating_display: Number(item.rating || 0).toFixed(1),
    order_count_display: Number(item.order_count || 0),
    total_gmv_display: `¥${(Number(item.total_gmv || 0) / 100).toFixed(2)}`,
    commission_amount_display: `¥${(Number(item.commission_amount || 0) / 100).toFixed(2)}`,
    region_name_display: item.region_name || `区域 ${item.region_id}`,
    category_display: item.category || '未分类'
  }
}

Page({
  data: {
    loading: false,
    loadingMore: false,
    refreshing: false,
    initialLoading: true,
    error: null as string | null,
    navBarHeight: 88,
    isLargeScreen: false,

    merchants: [] as MerchantView[],

    page: 1,
    limit: 20,
    total: 0,
    hasMore: true,

    regionId: 0,
    searchKeyword: '',
    statusFilter: '' as MerchantStatus | '',

    searchTimer: null as number | null
  },

  onLoad(options: MerchantListPageOptions) {
    const regionId = options.region_id ? parseInt(options.region_id) : 0
    const statusFilter = parseMerchantStatusFilter(options.status)
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
    if (this.data.loading || (this.data.loadingMore && !refresh)) return

    try {
      if (refresh) {
        if (!silent) {
          this.setData({ loading: true, error: null, page: 1 })
        } else {
          this.setData({ loading: true, page: 1 })
        }
      } else {
        this.setData({ loadingMore: true })
      }

      const params: MerchantQueryParams = {
        page: this.data.page,
        limit: this.data.limit,
        keyword: this.data.searchKeyword || undefined,
        status: this.data.statusFilter || undefined,
        sort_by: 'created_at',
        sort_order: 'desc',
        ...(this.data.regionId ? { region_id: this.data.regionId } : {})
      }

      const result = await operatorMerchantManagementService.getMerchantList(params)
      const list = (result.merchants || []).map(adaptMerchantItem)
      const merchants = refresh ? list : [...this.data.merchants, ...list]
      const total = Number(result.total || merchants.length)
      const hasMore = merchants.length < total

      this.setData({
        merchants,
        total,
        hasMore,
        page: refresh ? 2 : this.data.page + 1,
        loading: false,
        loadingMore: false,
        initialLoading: false,
        error: null
      })
    } catch (error) {
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

  onStatusFilterChange(e: WechatMiniprogram.CustomEvent<{ value: MerchantStatus | '' }>) {
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
