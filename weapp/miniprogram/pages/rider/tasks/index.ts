import { Delivery } from '../../../api/delivery'
import { logger } from '../../../utils/logger'
import { locationService } from '../../../utils/location'
import { getStableBarHeights } from '../../../utils/responsive'

const PAGE_SIZE = 20

interface DeliveryHistoryResponse {
  deliveries?: Delivery[]
  total_earnings?: number
  total?: number
}

interface UserMessageError {
  userMessage?: string
}

Page({
  data: {
    navBarHeight: 88,
    loading: false,
    loadingMore: false,
    errorMessage: '',
    loadMoreError: '',
    deliveries: [] as Delivery[],
    pageID: 1,
    hasMore: true,
    
    // 统计
    totalEarnings: 0,
    totalCount: 0
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.fetchHistory(1, true)
  },

  async fetchHistory(page: number = 1, reset: boolean = false) {
    if ((reset && this.data.loading) || (!reset && this.data.loadingMore)) return
    this.setData(reset ? { loading: true } : { loadingMore: true })
    
    try {
        const resp = await (require('../../../utils/request').request({
            url: '/v1/delivery/history',
            method: 'GET',
            data: {
                page,
                limit: PAGE_SIZE
            }
        })) as DeliveryHistoryResponse
        
        const list = resp.deliveries || []
        this.setData({
            deliveries: reset ? list : [...this.data.deliveries, ...list],
            hasMore: list.length === PAGE_SIZE,
            totalEarnings: resp.total_earnings || 0,
            totalCount: resp.total || 0,
            pageID: page,
            errorMessage: '',
            loadMoreError: ''
        })
    } catch (err: unknown) {
        logger.error('Fetch delivery history failed', err)
        const userMessage = (err as UserMessageError).userMessage
        const message = typeof userMessage === 'string' && userMessage ? userMessage : '历史任务加载失败，请稍后重试'
        if (reset) {
          this.setData({ errorMessage: message, loadMoreError: '', deliveries: [], hasMore: true })
        } else {
          this.setData({ loadMoreError: message })
        }
    } finally {
        this.setData({ loading: false, loadingMore: false })
    }
  },

  onReachBottom() {
    if (this.data.hasMore && !this.data.loading && !this.data.loadingMore) {
        this.fetchHistory(this.data.pageID + 1)
    }
  },

  onRetry() {
    this.fetchHistory(1, true)
  },

  onRetryLoadMore() {
    this.fetchHistory(this.data.pageID + 1, false)
  },

  onGoToDetail(e: WechatMiniprogram.TouchEvent) {
    const { orderId } = e.currentTarget.dataset as { orderId?: number }
    if (!orderId) return
    wx.navigateTo({
        url: `/pages/rider/task-detail/index?id=${orderId}`
    })
  },

  async onOpenLocation(e: WechatMiniprogram.TouchEvent) {
    const {
      latitude,
      longitude,
      name,
      address,
      label
    } = e.currentTarget.dataset as {
      latitude?: number
      longitude?: number
      name?: string
      address?: string
      label?: string
    }

    await locationService.openLocation({
      latitude,
      longitude,
      name,
      address,
      failMessage: `打开${label || '导航'}失败，请稍后重试`
    })
  }
})
