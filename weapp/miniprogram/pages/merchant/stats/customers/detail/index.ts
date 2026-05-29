import dayjs from '../../../_main_shared/miniprogram_npm/dayjs/index'
import { MerchantCustomerDetailResponse, MerchantStatsService } from '../../../_api/merchant-stats'
import { logger } from '../../../../../utils/logger'
import { getStableBarHeights } from '../../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../../utils/user-facing'

interface CustomerDetailOptions {
  userId?: string
}

interface FavoriteDishView {
  dish_id: number
  dish_name: string
  order_count: number
  total_quantity: number
  summary: string
}

interface CustomerDetailView extends MerchantCustomerDetailResponse {
  display_name: string
  phone_text: string
  total_amount_text: string
  avg_order_amount_text: string
  first_order_at_text: string
  last_order_at_text: string
  favorite_dishes_view: FavoriteDishView[]
}

function formatMoney(amount: number) {
  return `¥${(amount / 100).toFixed(2)}`
}

function formatTime(value?: string) {
  if (!value) return '--'
  const parsed = dayjs(value)
  return parsed.isValid() ? parsed.format('YYYY-MM-DD HH:mm') : value
}

function buildFavoriteDishSummary(orderCount: number, totalQuantity: number) {
  return `${orderCount} 次下单，累计 ${totalQuantity} 份`
}

function buildDetailView(detail: MerchantCustomerDetailResponse): CustomerDetailView {
  return {
    ...detail,
    display_name: detail.full_name || `顾客 #${detail.user_id}`,
    phone_text: detail.phone || '未留手机号',
    total_amount_text: formatMoney(detail.total_amount || 0),
    avg_order_amount_text: formatMoney(detail.avg_order_amount || 0),
    first_order_at_text: formatTime(detail.first_order_at),
    last_order_at_text: formatTime(detail.last_order_at),
    favorite_dishes_view: Array.isArray(detail.favorite_dishes)
      ? detail.favorite_dishes.map((item) => ({
          ...item,
          summary: buildFavoriteDishSummary(item.order_count || 0, item.total_quantity || 0)
        }))
      : []
  }
}

const getErrorMessage = getErrorUserMessage

Page({
  data: {
    navBarHeight: 88,
    userId: 0,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    loading: false,
    detail: null as CustomerDetailView | null
  },

  onLoad(options: CustomerDetailOptions) {
    const { navBarHeight } = getStableBarHeights()
    const userId = Number(options.userId || 0)
    this.setData({ navBarHeight, userId })

    if (!userId) {
      this.setData({
        initialLoading: false,
        initialError: true,
        initialErrorMessage: '缺少顾客 ID，无法查看详情'
      })
      return
    }

    this.loadDetail()
  },

  onPullDownRefresh() {
    this.loadDetail(false)
  },

  onRetry() {
    this.loadDetail()
  },

  onRetryRefresh() {
    this.loadDetail(false)
  },

  async loadDetail(showLoading = true) {
    if (this.data.loading) return
    const hasExistingData = Boolean(this.data.detail)
    const isSilentRefresh = !showLoading && hasExistingData

    this.setData({
      loading: true,
      ...(showLoading
        ? { initialError: false, initialErrorMessage: '', refreshErrorMessage: '' }
        : isSilentRefresh
          ? { refreshErrorMessage: '' }
          : {})
    })

    try {
      const detail = await MerchantStatsService.getCustomerDetail(this.data.userId)
      this.setData({
        detail: buildDetailView(detail),
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: ''
      })
    } catch (err) {
      logger.error('Load merchant customer detail failed', err)
      const message = getErrorMessage(err, '客户详情加载失败，请稍后重试')

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
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  onCallCustomer() {
    const phone = this.data.detail?.phone
    if (!phone) {
      wx.showToast({ title: '顾客未留手机号', icon: 'none' })
      return
    }
    wx.makePhoneCall({ phoneNumber: phone })
  }
})