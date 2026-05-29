import dayjs from '../../_main_shared/miniprogram_npm/dayjs/index'
import {
  MerchantCustomerOrderBy,
  MerchantCustomerStatRow,
  MerchantStatsService
} from '../../_api/merchant-stats'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../utils/user-facing'

type CustomerSortKey = MerchantCustomerOrderBy

interface CustomerSortOption {
  key: CustomerSortKey
  label: string
}

interface MerchantCustomerCardView extends MerchantCustomerStatRow {
  display_name: string
  phone_text: string
  total_amount_text: string
  avg_order_amount_text: string
  first_order_at_text: string
  last_order_at_text: string
  activity_hint: string
}

const SORT_OPTIONS: CustomerSortOption[] = [
  { key: 'last_order_at', label: '最近下单' },
  { key: 'total_amount', label: '消费金额' },
  { key: 'total_orders', label: '订单次数' }
]

function formatMoney(amount: number) {
  return `¥${(amount / 100).toFixed(2)}`
}

function formatTime(value?: string) {
  if (!value) return '--'
  const parsed = dayjs(value)
  return parsed.isValid() ? parsed.format('YYYY-MM-DD HH:mm') : value
}

function buildActivityHint(customer: MerchantCustomerStatRow) {
  if (customer.total_orders <= 1) {
    return '当前仅有 1 次消费记录，可继续观察复购情况。'
  }
  if (customer.total_orders >= 5) {
    return '已形成稳定复购，可结合会员或代金券做精细化触达。'
  }
  return '已产生多次消费，适合继续跟进偏好和复购机会。'
}

function buildCustomerCard(customer: MerchantCustomerStatRow): MerchantCustomerCardView {
  return {
    ...customer,
    display_name: customer.full_name || `顾客 #${customer.user_id}`,
    phone_text: customer.phone || '未留手机号',
    total_amount_text: formatMoney(customer.total_amount || 0),
    avg_order_amount_text: formatMoney(customer.avg_order_amount || 0),
    first_order_at_text: formatTime(customer.first_order_at),
    last_order_at_text: formatTime(customer.last_order_at),
    activity_hint: buildActivityHint(customer)
  }
}

const getErrorMessage = getErrorUserMessage

Page({
  data: {
    navBarHeight: 88,
    sortOptions: SORT_OPTIONS,
    currentSort: 'last_order_at' as CustomerSortKey,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    loading: false,
    loadingMore: false,
    customers: [] as MerchantCustomerCardView[],
    total: 0,
    page: 0,
    pageSize: 20,
    hasMore: false
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.loadCustomers(true)
  },

  onShow() {
    if (!this.data.initialLoading && !this.data.loading && !this.data.loadingMore) {
      this.loadCustomers(true, false)
    }
  },

  onPullDownRefresh() {
    this.loadCustomers(true, false)
  },

  onReachBottom() {
    if (this.data.hasMore && !this.data.loading && !this.data.loadingMore) {
      this.loadCustomers(false)
    }
  },

  async loadCustomers(reset: boolean, showLoading = true) {
    if (reset && this.data.loading) return
    if (!reset && (this.data.loadingMore || !this.data.hasMore)) return

    const nextPage = reset ? 1 : this.data.page + 1
    const hasExistingData = this.data.customers.length > 0
    const isSilentRefresh = reset && !showLoading && hasExistingData

    this.setData({
      ...(reset ? { loading: true } : { loadingMore: true }),
      ...(showLoading
        ? {
            initialError: false,
            initialErrorMessage: '',
            refreshErrorMessage: ''
          }
        : isSilentRefresh
          ? { refreshErrorMessage: '' }
          : {})
    })

    try {
      const response = await MerchantStatsService.listCustomers({
        order_by: this.data.currentSort,
        page: nextPage,
        limit: this.data.pageSize
      })
      const incoming = Array.isArray(response.data) ? response.data.map(buildCustomerCard) : []

      this.setData({
        customers: reset ? incoming : [...this.data.customers, ...incoming],
        total: response.total || 0,
        page: response.page || nextPage,
        hasMore: (response.page || nextPage) * (response.limit || this.data.pageSize) < (response.total || 0),
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: ''
      })
    } catch (err) {
      logger.error('Load merchant customers failed', err)
      const message = getErrorMessage(err, '客户分析加载失败，请稍后重试')

      if (this.data.initialLoading) {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message
        })
      } else if (isSilentRefresh) {
        this.setData({ refreshErrorMessage: `${message}，当前已保留上次同步结果` })
      } else {
        wx.showToast({ title: message, icon: 'none' })
      }
    } finally {
      this.setData({ loading: false, loadingMore: false })
      wx.stopPullDownRefresh()
    }
  },

  onRetry() {
    this.loadCustomers(true)
  },

  onRetryRefresh() {
    this.loadCustomers(true, false)
  },

  onChangeSort(e: WechatMiniprogram.TouchEvent) {
    const { key } = e.currentTarget.dataset as { key?: CustomerSortKey }
    if (!key || key === this.data.currentSort) return

    this.setData({
      currentSort: key,
      page: 0,
      hasMore: false,
      customers: []
    }, () => {
      this.loadCustomers(true)
    })
  },

  onViewCustomerDetail(e: WechatMiniprogram.TouchEvent) {
    const { userId } = e.currentTarget.dataset as { userId?: number }
    if (!userId) return
    wx.navigateTo({ url: `/pages/merchant/stats/customers/detail/index?userId=${userId}` })
  }
})