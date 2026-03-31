import {
  operatorRiderManagementService,
  formatOnlineStatus,
  formatRiderStatus,
  formatVehicleType,
  OperatorRiderDetailResponse,
  RiderStatus
} from '../../../../api/operator-rider-management'
import type { RiderStatsResponse } from '../../../../api/operator-rider-management'

type RiderDetailView = {
  id: number
  user_id: number
  real_name: string
  phone: string
  id_card_no?: string
  status: RiderStatus | string
  is_online: boolean
  online_status_label: string
  region_id: number
  region_name: string
  deposit_amount: number
  frozen_deposit: number
  total_orders: number
  total_earnings: number
  rating_display: string
  score_display: string
  vehicle_type_display: string
  emergency_contact: string
  emergency_phone: string
  current_latitude?: number
  current_longitude?: number
  location_updated_at?: string
  credit_score: number
  created_at: string
  updated_at: string
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

function adaptRiderDetail(detail: OperatorRiderDetailResponse & Record<string, unknown>): RiderDetailView {
  const status = String(detail.status || 'pending') as RiderStatus | string
  const onlineStatus = detail.online_status || ((detail.is_online as boolean) ? 'online' : 'offline')
  const totalOrders = Number(detail.stats?.total_deliveries || detail.total_orders || 0)
  const totalEarnings = Number(detail.stats?.total_earnings || detail.total_earnings || 0)
  const depositAmount = Number(detail.deposit_amount || 0)
  const frozenDeposit = Number(detail.frozen_deposit || 0)
  const currentLatitude = detail.last_location?.latitude ?? Number(detail.current_latitude || 0)
  const currentLongitude = detail.last_location?.longitude ?? Number(detail.current_longitude || 0)
  const locationUpdatedAt = detail.last_location?.updated_at || String(detail.location_updated_at || '')

  return {
    id: Number(detail.id || 0),
    user_id: Number(detail.user_id || 0),
    real_name: String(detail.name || detail.real_name || '未命名骑手'),
    phone: String(detail.phone || '-'),
    id_card_no: detail.id_card ? String(detail.id_card) : String(detail.id_card_no || ''),
    status,
    is_online: onlineStatus === 'online',
    online_status_label: formatOnlineStatus(onlineStatus as 'online' | 'offline' | 'busy' | 'break'),
    region_id: Number(detail.region_id || 0),
    region_name: String(detail.region_name || `区域 ${Number(detail.region_id || 0)}`),
    deposit_amount: depositAmount,
    frozen_deposit: frozenDeposit,
    total_orders: totalOrders,
    total_earnings: totalEarnings,
    rating_display: Number(detail.rating || detail.stats?.avg_rating || 0).toFixed(1),
    score_display: Number(detail.score || detail.credit_score || 0).toFixed(0),
    vehicle_type_display: detail.vehicle_type ? formatVehicleType(detail.vehicle_type) : '-',
    emergency_contact: String(detail.emergency_contact || '-'),
    emergency_phone: String(detail.emergency_phone || '-'),
    current_latitude: currentLatitude || undefined,
    current_longitude: currentLongitude || undefined,
    location_updated_at: locationUpdatedAt,
    credit_score: Number(detail.score || detail.credit_score || 0),
    created_at: String(detail.created_at || ''),
    updated_at: String(detail.updated_at || ''),
    deposit_amount_display: (depositAmount / 100).toFixed(2),
    frozen_deposit_display: (frozenDeposit / 100).toFixed(2),
    total_earnings_display: (totalEarnings / 100).toFixed(2),
    status_label: formatRiderStatus(status as RiderStatus)
  }
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
    suspendDialogVisible: false,
    resumeDialogVisible: false,
    actionReason: ''
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
      const detail = await operatorRiderManagementService.getRiderDetail(this.data.id)
      const detailView = adaptRiderDetail(detail as OperatorRiderDetailResponse & Record<string, unknown>)
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
        avg_delivery_min: (s.avg_delivery_seconds / 60).toFixed(1)
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

  onReasonChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
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
      await operatorRiderManagementService.suspendRider(this.data.id, {
        reason: this.data.actionReason
      })
      wx.showToast({ title: '暂停成功', icon: 'success' })
      this.setData({ suspendDialogVisible: false })
      this.loadAll()
    } catch (error) {
      console.error('暂停骑手失败:', error)
      wx.showToast({ title: '操作失败', icon: 'none' })
    } finally {
      wx.hideLoading()
    }
  },

  async onResumeConfirm() {
    const reason = this.data.actionReason.trim() || '运营恢复骑手接单'

    try {
      wx.showLoading({ title: '处理中...' })
      await operatorRiderManagementService.resumeRider(this.data.id, { reason })
      wx.showToast({ title: '恢复成功', icon: 'success' })
      this.setData({ resumeDialogVisible: false })
      this.loadAll()
    } catch (error) {
      console.error('恢复骑手失败:', error)
      wx.showToast({ title: '操作失败', icon: 'none' })
    } finally {
      wx.hideLoading()
    }
  }
})
