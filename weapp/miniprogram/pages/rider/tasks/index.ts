import { Delivery } from '../../../api/delivery'
import { logger } from '../../../utils/logger'
import { getStableBarHeights } from '../../../utils/responsive'

Page({
  data: {
    navBarHeight: 88,
    loading: false,
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
    this.fetchHistory()
  },

  async fetchHistory() {
    if (this.data.loading) return
    this.setData({ loading: true })
    
    try {
        const resp = await (require('../../../utils/request').request({
            url: '/v1/delivery/history',
            method: 'GET',
            data: {
                page_id: this.data.pageID,
                page_size: 20
            }
        }))
        
        const list = resp.deliveries || []
        this.setData({
            deliveries: this.data.pageID === 1 ? list : [...this.data.deliveries, ...list],
            hasMore: list.length === 20,
            totalEarnings: resp.total_earnings || 0,
            totalCount: resp.total || 0
        })
    } catch (err) {
        logger.error('Fetch delivery history failed', err)
    } finally {
        this.setData({ loading: false })
    }
  },

  onReachBottom() {
    if (this.data.hasMore && !this.data.loading) {
        this.setData({ pageID: this.data.pageID + 1 }, () => this.fetchHistory())
    }
  },

  onGoToDetail(e: WechatMiniprogram.TouchEvent) {
    const { orderId } = e.currentTarget.dataset as { orderId?: number }
    if (!orderId) return
    wx.navigateTo({
        url: `/pages/rider/task-detail/index?id=${orderId}`
    })
  }
})
