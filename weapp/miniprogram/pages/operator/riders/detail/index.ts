import { request } from '../../../../utils/request'
import { operatorRiderManagementService } from '../../../../api/operator-rider-management'
import type { RiderStatsResponse } from '../../../../api/operator-rider-management'

type RiderDetail = {
  id: number
  user_id: number
  real_name: string
  phone: string
  id_card_no?: string
  status: string
  is_online: boolean
  region_id: number
  deposit_amount: number
  frozen_deposit: number
  total_orders: number
  total_earnings: number
  current_latitude?: number
  current_longitude?: number
  location_updated_at?: string
  credit_score: number
  high_value_qualified: boolean
  created_at: string
  updated_at: string
}

type RiderDetailView = RiderDetail & {
  deposit_amount_display: string
  frozen_deposit_display: string
  total_earnings_display: string
  status_label: string
}

type RiderStatsView = RiderStatsResponse & {
  period_earnings_display: string
  completion_rate_display: string
  avg_delivery_min: string
}

function statusLabel(s: string): string {
  const map: Record<string, string> = {
    active: '已审核',
    pending: '待审核',
    suspended: '已暫停',
    deactivated: '已注销',
  }
  return map[s] ?? s
}

Page({
  data: {
    id: 0,
    loading: true,
    statsLoading: false,
    error: '',
    navBarHeight: 88,
    detail: null as RiderDetailView | null,
    stats: null as RiderStatsView | null,
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
      const detail = await request<RiderDetail>({
        url: `/v1/operator/riders/${this.data.id}`,
        method: 'GET'
      })
      const detailView: RiderDetailView = {
        ...detail,
        deposit_amount_display: (Number(detail.deposit_amount || 0) / 100).toFixed(2),
        frozen_deposit_display: (Number(detail.frozen_deposit || 0) / 100).toFixed(2),
        total_earnings_display: (Number(detail.total_earnings || 0) / 100).toFixed(2),
        status_label: statusLabel(detail.status),
      }
      this.setData({ detail: detailView, loading: false })
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : '加载骑手详情失败'
      this.setData({ loading: false, error: message })
      return
    }
    // 加载配送统计
    this.setData({ statsLoading: true })
    try {
      const s = await operatorRiderManagementService.getRiderStats(this.data.id, 30)
      const statsView: RiderStatsView = {
        ...s,
        period_earnings_display: (s.period_earnings / 100).toFixed(2),
        completion_rate_display: (s.completion_rate_basis_points / 100).toFixed(1),
        avg_delivery_min: (s.avg_delivery_seconds / 60).toFixed(1),
      }
      this.setData({ stats: statsView })
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
