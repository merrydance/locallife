import { Delivery, getDeliveryStatusDisplay } from '../../../api/delivery'
import { deliveryTaskManagementService } from '../../../api/delivery-task-management'
import { logger } from '../../../utils/logger'
import { locationService } from '../../../utils/location'
import { getStableBarHeights } from '../../../utils/responsive'

const PAGE_SIZE = 20

type DeliveryHistoryView = Delivery & {
  display_time: string
  status_text: string
  status_theme: 'success' | 'warning' | 'danger' | 'primary' | 'default'
}

interface DeliveryHistoryResponse {
  deliveries?: Delivery[]
  total_earnings?: number
  completed_total?: number
  total?: number
  page_id?: number
  page_size?: number
}

interface UserMessageError {
  userMessage?: string
}

function decorateHistoryDelivery(delivery: Delivery): DeliveryHistoryView {
  const statusMeta = getDeliveryStatusDisplay(delivery.status)
  return {
    ...delivery,
    display_time: delivery.completed_at || delivery.delivered_at || delivery.created_at || '',
    status_text: statusMeta.text,
    status_theme: statusMeta.theme
  }
}

Page({
  data: {
    navBarHeight: 88,
    loading: false,
    loadingMore: false,
    errorMessage: '',
    loadMoreError: '',
    deliveries: [] as DeliveryHistoryView[],
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
      const resp = await deliveryTaskManagementService.getDeliveryHistory({
        page_id: page,
        page_size: PAGE_SIZE
      }) as DeliveryHistoryResponse
        
        const list = (resp.deliveries || []).map(decorateHistoryDelivery)
        const total = resp.total || 0
        this.setData({
            deliveries: reset ? list : [...this.data.deliveries, ...list],
          hasMore: page * PAGE_SIZE < total,
            totalEarnings: resp.total_earnings || 0,
          totalCount: resp.completed_total || 0,
          pageID: resp.page_id || page,
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
