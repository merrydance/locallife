import {
  loadOperatorRiderDetailView,
  loadOperatorRiderStatsView,
  type OperatorRiderDetailView,
  type OperatorRiderStatsView
} from '../../_services/operator-rider-management'
import { getErrorUserMessage } from '../../../../utils/user-facing'

Page({
  data: {
    id: 0,
    loading: true,
    statsLoading: false,
    error: '',
    navBarHeight: 88,
    detail: null as OperatorRiderDetailView | null,
    stats: null as OperatorRiderStatsView | null
  },

  onLoad(options: Record<string, string>) {
    const id = Number(options.id || 0)
    if (!id) {
      this.setData({ loading: false, error: '骑手ID无效' })
      return
    }
    this.setData({ id })
    this.loadAll()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  async loadAll() {
    if (!this.data.id) return
    this.setData({ loading: true, error: '', stats: null })
    try {
      this.setData({ detail: await loadOperatorRiderDetailView(this.data.id), loading: false })
    } catch (error: unknown) {
      const message = getErrorUserMessage(error, '加载骑手详情失败，请稍后重试')
      this.setData({ loading: false, error: message })
      return
    }
    // 加载代取统计
    this.setData({ statsLoading: true })
    try {
      this.setData({ stats: await loadOperatorRiderStatsView(this.data.id, 30) })
    } catch {
      // 统计加载失败不阻断主流程
    } finally {
      this.setData({ statsLoading: false })
    }
  },

  onRetry() {
    this.loadAll()
  }
})
