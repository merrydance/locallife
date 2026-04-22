import {
  loadOperatorMerchantDetailView,
  loadOperatorMerchantStatsView,
  type OperatorMerchantDetailView,
  type OperatorMerchantStatsView
} from '../../../../services/operator-merchant-management'
import { getErrorUserMessage } from '../../../../utils/user-facing'

Page({
  data: {
    id: 0,
    loading: true,
    statsLoading: false,
    error: '',
    navBarHeight: 88,
    detail: null as OperatorMerchantDetailView | null,
    stats: null as OperatorMerchantStatsView | null
  },

  onLoad(options: Record<string, string>) {
    const id = Number(options.id || 0)
    if (!id) {
      this.setData({ loading: false, error: '商户ID无效' })
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
      this.setData({ detail: await loadOperatorMerchantDetailView(this.data.id), loading: false })
    } catch (error: unknown) {
      const message = getErrorUserMessage(error, '加载商户详情失败，请稍后重试')
      this.setData({ loading: false, error: message })
      return
    }
    // 加载经营统计
    this.setData({ statsLoading: true })
    try {
      this.setData({ stats: await loadOperatorMerchantStatsView(this.data.id, 30) })
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
