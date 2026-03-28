import {
  operatorMerchantManagementService,
  formatMerchantStatus,
  MerchantStatus,
  OperatorMerchantDetailResponse
} from '../../../../api/operator-merchant-management'
import type { MerchantStatsResponse } from '../../../../api/operator-merchant-management'

type MerchantDetailView = {
  id: number
  name: string
  description?: string
  logo_url?: string
  phone: string
  contact_person?: string
  contact_phone?: string
  address: string
  status: MerchantStatus | string
  category: string
  business_hours: string
  is_open: boolean
  region_id: number
  region_name: string
  latitude: number
  longitude: number
  commission_rate_display: string
  created_at: string
  updated_at: string
  last_active_at?: string
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

function fen2yuan(fen: number): string {
  return (fen / 100).toFixed(2)
}

function adaptMerchantDetail(detail: OperatorMerchantDetailResponse & Record<string, unknown>): MerchantDetailView {
  const status = String(detail.status || 'pending') as MerchantStatus | string

  return {
    id: Number(detail.id || 0),
    name: String(detail.name || '未命名商户'),
    description: detail.description ? String(detail.description) : '',
    logo_url: Array.isArray(detail.images) && detail.images.length > 0 ? String(detail.images[0]) : '',
    phone: String(detail.phone || detail.contact_phone || '-'),
    contact_person: detail.contact_person ? String(detail.contact_person) : '',
    contact_phone: detail.contact_phone ? String(detail.contact_phone) : '',
    address: String(detail.address || '-'),
    status,
    category: String(detail.category || '未分类'),
    business_hours: String(detail.business_hours || '-'),
    is_open: status === 'approved' || status === 'active',
    region_id: Number(detail.region_id || 0),
    region_name: String(detail.region_name || `区域 ${Number(detail.region_id || 0)}`),
    latitude: Number(detail.latitude || 0),
    longitude: Number(detail.longitude || 0),
    commission_rate_display: `${(Number(detail.commission_rate || 0) / 100).toFixed(2)}%`,
    created_at: String(detail.created_at || ''),
    updated_at: String(detail.updated_at || ''),
    last_active_at: detail.last_active_at ? String(detail.last_active_at) : '',
    status_label: formatMerchantStatus(status as MerchantStatus)
  }
}

Page({
  data: {
    id: 0,
    loading: true,
    statsLoading: false,
    error: '',
    navBarHeight: 88,
    detail: null as MerchantDetailView | null,
    stats: null as StatsView | null,
    suspendDialogVisible: false,
    resumeDialogVisible: false,
    actionReason: ''
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
      const raw = await operatorMerchantManagementService.getMerchantDetail(this.data.id)
      const detail = adaptMerchantDetail(raw as OperatorMerchantDetailResponse & Record<string, unknown>)
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
  },

  onActionReasonChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({ actionReason: e.detail.value || '' })
  },

  onOpenSuspendDialog() {
    this.setData({ suspendDialogVisible: true, actionReason: '' })
  },

  onOpenResumeDialog() {
    this.setData({ resumeDialogVisible: true, actionReason: '' })
  },

  onSuspendCancel() {
    this.setData({ suspendDialogVisible: false })
  },

  onResumeCancel() {
    this.setData({ resumeDialogVisible: false })
  },

  async onSuspendConfirm() {
    if (!this.data.actionReason.trim()) {
      wx.showToast({ title: '请输入暂停原因', icon: 'none' })
      return
    }

    try {
      wx.showLoading({ title: '处理中...' })
      await operatorMerchantManagementService.suspendMerchant(this.data.id, {
        reason: this.data.actionReason
      })
      wx.showToast({ title: '暂停成功', icon: 'success' })
      this.setData({ suspendDialogVisible: false })
      this.loadAll()
    } catch (error) {
      console.error('暂停商户失败:', error)
      wx.showToast({ title: '操作失败', icon: 'none' })
    } finally {
      wx.hideLoading()
    }
  },

  async onResumeConfirm() {
    const reason = this.data.actionReason.trim() || '运营恢复商户营业'

    try {
      wx.showLoading({ title: '处理中...' })
      await operatorMerchantManagementService.resumeMerchant(this.data.id, {
        reason
      })
      wx.showToast({ title: '恢复成功', icon: 'success' })
      this.setData({ resumeDialogVisible: false })
      this.loadAll()
    } catch (error) {
      console.error('恢复商户失败:', error)
      wx.showToast({ title: '操作失败', icon: 'none' })
    } finally {
      wx.hideLoading()
    }
  }
})
