import { operatorMerchantManagementService } from '../../../../api/operator-merchant-management'
import type { MerchantStatsResponse } from '../../../../api/operator-merchant-management'

type MerchantDetail = {
  id: number
  name: string
  description?: string
  logo_url?: string
  phone: string
  address: string
  status: string
  is_open: boolean
  owner_user_id: number
  region_id: number
  latitude: number
  longitude: number
  created_at: string
  updated_at: string
}

type MerchantDetailView = MerchantDetail & {
  status_label: string
}

type StatsView = MerchantStatsResponse & {
  total_sales_display: string
  total_commission_display: string
  avg_daily_sales_display: string
  repurchase_rate_display: string
  avg_orders_per_user_display: string
  top_dishes_with_revenue: Array<{ dish_name: string, total_sold: number, total_revenue_display: string }>
}

function statusLabel(s: string): string {
  const map: Record<string, string> = {
    active: '正常营业',
    approved: '已审核',
    pending: '待审核',
    rejected: '已拒绝',
    suspended: '已暂停',
    closed: '已关闭'
  }
  return map[s] ?? s
}

function fen2yuan(fen: number): string {
  return (fen / 100).toFixed(2)
}

Page({
  data: {
    id: 0,
    loading: true,
    statsLoading: false,
    error: '',
    navBarHeight: 88,
    detail: null as MerchantDetailView | null,
    stats: null as StatsView | null
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
      const raw = await operatorMerchantManagementService.getMerchantDetail(this.data.id) as unknown as MerchantDetail
      const detail: MerchantDetailView = { ...raw, status_label: statusLabel(raw.status) }
      this.setData({ detail, loading: false })
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : '加载商户详情失败'
      this.setData({ loading: false, error: message })
      return
    }
    // 加载经营统计
    this.setData({ statsLoading: true })
    try {
      const s = await operatorMerchantManagementService.getMerchantStats(this.data.id, 30)
      const statsView: StatsView = {
        ...s,
        total_sales_display: fen2yuan(s.total_sales),
        total_commission_display: fen2yuan(s.total_commission),
        avg_daily_sales_display: fen2yuan(s.avg_daily_sales),
        repurchase_rate_display: (s.repurchase_rate_basis_points / 100).toFixed(1),
        avg_orders_per_user_display: (s.avg_orders_per_user_cents / 100).toFixed(2),
        top_dishes_with_revenue: (s.top_dishes ?? []).map((d) => ({
          dish_name: d.dish_name,
          total_sold: d.total_sold,
          total_revenue_display: fen2yuan(d.total_revenue)
        }))
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
