import { isLargeScreen } from '@/utils/responsive'
import {
  operatorRiderManagementService,
  RiderQueryParams,
  RiderStatus,
  OperatorRiderItem
} from '../../../api/operator-rider-management'
import { getErrorUserMessage } from '../../../utils/user-facing'

type RiderListPageOptions = {
  region_id?: string
  status?: string
}

type RiderListDataset = {
  id?: number
  name?: string
}

type RiderView = {
  id: number
  name: string
  phone: string
  status: string
  status_label: string
  is_online: boolean
  online_status_label: string
  region_id: number
  region_name: string
  delivery_count: number
  rating_display: string
  total_earnings_display: string
}

const RIDER_STATUS_LABEL: Record<string, string> = {
  active: '已审核',
  pending: '待审核',
  pending_approval: '待审核',
  suspended: '已暂停',
  rejected: '已驳回',
  offline: '已离线'
}

function parseRiderStatus(status?: string): RiderStatus | '' {
  if (
    status === 'pending' ||
    status === 'active' ||
    status === 'suspended' ||
    status === 'pending_approval' ||
    status === 'rejected' ||
    status === 'offline'
  ) {
    return status
  }

  return ''
}

function adaptRider(item: Partial<OperatorRiderItem> & Record<string, unknown>): RiderView {
  const name = String(item.name || item.real_name || '未命名骑手')
  const onlineStatus = String(item.online_status || ((item.is_online as boolean) ? 'online' : 'offline'))
  const isOnline = onlineStatus === 'online' || Boolean(item.is_online)
  const deliveryCount = Number(item.delivery_count || item.total_orders || 0)

  return {
    id: Number(item.id || 0),
    name,
    phone: String(item.phone || '-'),
    status: String(item.status || 'pending'),
    status_label: RIDER_STATUS_LABEL[String(item.status || 'pending')] || String(item.status || 'pending'),
    is_online: isOnline,
    online_status_label: isOnline ? '在线' : '离线',
    region_id: Number(item.region_id || 0),
    region_name: String(item.region_name || `区域 ${Number(item.region_id || 0)}`),
    delivery_count: deliveryCount,
    rating_display: Number(item.rating || 0).toFixed(1),
    total_earnings_display: `¥${(Number(item.total_earnings || 0) / 100).toFixed(2)}`
  }
}

Page({
  data: {
    navBarHeight: 88,
    isLargeScreen: false,
    loading: false,
    loadingMore: false,
    refreshing: false,
    initialLoading: true,
    error: '',
    page: 1,
    limit: 20,
    total: 0,
    hasMore: true,
    riders: [] as RiderView[],
    regionId: 0,
    statusFilter: '' as RiderStatus | '',
    searchKeyword: '',
    searchTimer: null as number | null,
    suspendDialogVisible: false,
    resumeDialogVisible: false,
    selectedRider: { id: 0, name: '' },
    actionReason: ''
  },

  onLoad(options: RiderListPageOptions) {
    const regionId = options.region_id ? parseInt(options.region_id) : 0
    const statusFilter = parseRiderStatus(options.status)
    this.setData({
      isLargeScreen: isLargeScreen(),
      regionId,
      statusFilter
    })
    this.loadRiders(true)
  },

  onShow() {
    if (!this.data.initialLoading) {
      this.loadRiders(true, true)
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  onPullDownRefresh() {
    this.setData({ refreshing: true })
    this.loadRiders(true).finally(() => {
      this.setData({ refreshing: false })
      wx.stopPullDownRefresh()
    })
  },

  async loadRiders(refresh: boolean, silent = false) {
    if (this.data.loading || (this.data.loadingMore && !refresh)) return

    try {
      if (refresh) {
        this.setData({ loading: true, error: '', page: 1, ...(silent ? {} : { initialLoading: this.data.initialLoading }) })
      } else {
        this.setData({ loadingMore: true })
      }

      const params: RiderQueryParams = {
        page: refresh ? 1 : this.data.page,
        limit: this.data.limit,
        keyword: this.data.searchKeyword || undefined,
        status: this.data.statusFilter || undefined,
        sort_by: 'created_at',
        sort_order: 'desc',
        ...(this.data.regionId ? { region_id: this.data.regionId } : {})
      }

      const res = await operatorRiderManagementService.getRiderList(params)
      const incoming = (res.riders || []).map((item) => adaptRider(item as Partial<OperatorRiderItem> & Record<string, unknown>))
      const riders = refresh ? incoming : [...this.data.riders, ...incoming]
      const total = Number(res.total || riders.length)

      this.setData({
        riders,
        page: refresh ? 2 : this.data.page + 1,
        total,
        hasMore: riders.length < total,
        loading: false,
        loadingMore: false,
        initialLoading: false
      })
    } catch (error: unknown) {
      const message = getErrorUserMessage(error, '加载骑手失败，请稍后重试')
      this.setData({ loading: false, loadingMore: false, initialLoading: false, error: message })
    }
  },

  onRetry() {
    this.loadRiders(true)
  },

  onLoadMore() {
    if (this.data.hasMore && !this.data.loading && !this.data.loadingMore) {
      this.loadRiders(false)
    }
  },

  onSearchChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const searchKeyword = e.detail.value || ''
    this.setData({ searchKeyword })

    if (this.data.searchTimer) {
      clearTimeout(this.data.searchTimer)
    }

    const timer = setTimeout(() => {
      this.loadRiders(true)
    }, 500)

    this.setData({ searchTimer: timer })
  },

  onSearchClear() {
    this.setData({ searchKeyword: '' })
    this.loadRiders(true)
  },

  onStatusFilterChange(e: WechatMiniprogram.CustomEvent<{ value: RiderStatus | '' }>) {
    this.setData({ statusFilter: e.detail.value })
    this.loadRiders(true)
  },

  onTapRider(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as RiderListDataset
    if (!id) return
    wx.navigateTo({ url: `/pages/operator/riders/detail/index?id=${id}` })
  },

  onSuspendTap(e: WechatMiniprogram.TouchEvent) {
    const { id, name } = e.currentTarget.dataset as RiderListDataset
    if (!id || !name) return
    this.setData({
      selectedRider: { id, name },
      suspendDialogVisible: true,
      actionReason: ''
    })
  },

  onResumeTap(e: WechatMiniprogram.TouchEvent) {
    const { id, name } = e.currentTarget.dataset as RiderListDataset
    if (!id || !name) return
    this.setData({
      selectedRider: { id, name },
      resumeDialogVisible: true,
      actionReason: ''
    })
  },

  onReasonChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({ actionReason: e.detail.value || '' })
  },

  async onSuspendConfirm() {
    if (!this.data.actionReason.trim()) {
      wx.showToast({ title: '请输入暂停原因', icon: 'none' })
      return
    }

    try {
      wx.showLoading({ title: '处理中...' })
      await operatorRiderManagementService.suspendRider(this.data.selectedRider.id, {
        reason: this.data.actionReason
      })
      this.setData({ suspendDialogVisible: false })
      this.loadRiders(true)
    } catch (error) {
      console.error('暂停骑手失败:', error)
      wx.showToast({ title: getErrorUserMessage(error, '暂停失败，请稍后重试'), icon: 'none' })
    } finally {
      wx.hideLoading()
    }
  },

  async onResumeConfirm() {
    const reason = this.data.actionReason.trim() || '运营恢复骑手接单'
    try {
      wx.showLoading({ title: '处理中...' })
      await operatorRiderManagementService.resumeRider(this.data.selectedRider.id, {
        reason
      })
      this.setData({ resumeDialogVisible: false })
      this.loadRiders(true)
    } catch (error) {
      console.error('恢复骑手失败:', error)
      wx.showToast({ title: getErrorUserMessage(error, '恢复失败，请稍后重试'), icon: 'none' })
    } finally {
      wx.hideLoading()
    }
  },

  onSuspendCancel() {
    this.setData({ suspendDialogVisible: false })
  },

  onResumeCancel() {
    this.setData({ resumeDialogVisible: false })
  },

  stopPropagation() {
    // 阻止事件冒泡
  }
})
