import { responsiveBehavior } from '@/utils/responsive'
import {
  platformManagementService,
  type AdminOperatorApplicationItem,
  type AdminRegionExpansionApplicationItem
} from '@/api/platform-management'
import { getErrorUserMessage } from '@/utils/user-facing'

type OperatorApplicationStatus = 'all' | 'submitted' | 'approved' | 'rejected'
type SortBy =
  | 'latest'
  | 'earliest'
  | 'name_asc'
  | 'name_desc'
  | 'approved_first'
  | 'rejected_first'
  | 'submitted_first'
type OperatorStatusTheme = 'warning' | 'success' | 'danger' | 'default'
type OperatorApplicationDisplayItem = AdminOperatorApplicationItem & {
  statusLabel: string
  statusTheme: OperatorStatusTheme
}

type NavHeightEvent = WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>
type TapEvent = WechatMiniprogram.CustomEvent & {
  currentTarget: {
    dataset: {
      id?: number | string
    }
  }
}

Page({
  behaviors: [responsiveBehavior],
  data: {
    navBarHeight: 0,
    loading: false,
    requesting: false,
    refreshing: false,
    error: null as string | null,
    page: 1,
    limit: 20,
    total: 0,
    hasMore: false,
    rawApplications: [] as AdminOperatorApplicationItem[],
    applications: [] as OperatorApplicationDisplayItem[],
    statusFilter: 'all' as OperatorApplicationStatus,
    sortBy: 'latest' as SortBy,
    sortOptionLabels: ['最新提交', '最早提交', '名称 A-Z', '名称 Z-A', '通过在前', '拒绝在前', '待审在前'],
    sortOptions: ['latest', 'earliest', 'name_asc', 'name_desc', 'approved_first', 'rejected_first', 'submitted_first'] as SortBy[],
    sortIndex: 0,
    filterStats: {
      all: 0,
      submitted: 0,
      approved: 0,
      rejected: 0
    },

    // ── 区域扩展申请 tab ──────────────────────────────────────
    activeTab: 'onboarding',
    regionApps:        [] as AdminRegionExpansionApplicationItem[],
    regionDisplayApps: [] as AdminRegionExpansionApplicationItem[],
    regionStatusFilter: 'all' as 'all' | 'pending' | 'approved' | 'rejected',
    regionFilterStats: { all: 0, pending: 0, approved: 0, rejected: 0 },
    regionPage: 1,
    regionTotal: 0,
    regionHasMore: false,
    regionLoading: false,
    regionError: null as string | null,

    // 驳回弹窗
    rejectDialogVisible: false,
    rejectReason: '',
    rejectTargetId: 0,
    rejectTargetDesc: '',
    submittingReject: false
  },

  onLoad() {
    this.loadApplications(true)
    this.loadRegionApplications(true)
  },

  onShow() {
    if (this.data.requesting) return
    if (this.data.rawApplications.length === 0 && this.data.regionApps.length === 0) return
    if (this.data.activeTab === 'onboarding') {
      this.loadApplications(true)
    } else {
      this.loadRegionApplications(true)
    }
  },

  onNavHeight(e: NavHeightEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 0 })
  },

  async onRefresh() {
    this.setData({ refreshing: true })
    try {
      if (this.data.activeTab === 'onboarding') {
        await this.loadApplications(true)
      } else {
        await this.loadRegionApplications(true)
      }
    } finally {
      this.setData({ refreshing: false })
    }
  },

  async onLoadMore() {
    if (this.data.activeTab === 'onboarding') {
      if (!this.data.hasMore || this.data.loading) return
      await this.loadApplications(false)
    } else {
      if (!this.data.regionHasMore || this.data.regionLoading) return
      await this.loadRegionApplications(false)
    }
  },

  async loadApplications(reset: boolean) {
    if (this.data.requesting) {
      return
    }

    const page = reset ? 1 : this.data.page + 1
    this.setData({ loading: true, requesting: true, error: null })
    try {
      const response = await platformManagementService.getAdminOperatorApplications({
        page,
        limit: this.data.limit
      })

      const merged = reset
        ? (response.applications || [])
        : this.mergeApplications(this.data.rawApplications, response.applications || [])

      const filterStats = this.buildFilterStats(merged)
      const displayed = this.applyFilterAndSort(merged, this.data.statusFilter, this.data.sortBy)

      this.setData({
        rawApplications: merged,
        applications: displayed,
        total: response.total,
        page: response.page,
        hasMore: response.has_more,
        filterStats
      })
    } catch (error: unknown) {
      const message = getErrorUserMessage(error, '加载申请失败，请稍后重试')
      this.setData({ error: message })
    } finally {
      this.setData({ loading: false, requesting: false })
    }
  },

  onRetry() {
    this.loadApplications(true)
  },

  onRegionRetry() {
    this.loadRegionApplications(true)
  },

  onTabChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({ activeTab: e.detail.value })
  },

  // ── 区域扩展申请加载 ──────────────────────────────────────────────────────

  async loadRegionApplications(reset: boolean) {
    if (this.data.regionLoading) return
    const page = reset ? 1 : this.data.regionPage + 1
    this.setData({ regionLoading: true, regionError: null })
    try {
      const res = await platformManagementService.getAdminRegionExpansionApplications({ page, limit: 20 })
      const incoming = res.applications || []
      const merged = reset ? incoming : this._mergeRegion(this.data.regionApps, incoming)
      const regionFilterStats = {
        all:      merged.length,
        pending:  merged.filter((i) => i.status === 'pending').length,
        approved: merged.filter((i) => i.status === 'approved').length,
        rejected: merged.filter((i) => i.status === 'rejected').length
      }
      const regionDisplayApps = this._filterRegion(merged, this.data.regionStatusFilter)
      this.setData({
        regionApps: merged,
        regionDisplayApps,
        regionTotal: res.total ?? 0,
        regionPage: page,
        regionHasMore: merged.length < (res.total ?? 0),
        regionFilterStats
      })
    } catch (e: unknown) {
      this.setData({ regionError: getErrorUserMessage(e, '区域申请加载失败，请稍后重试') })
    } finally {
      this.setData({ regionLoading: false })
    }
  },

  onRegionFilterChange(e: WechatMiniprogram.CustomEvent & { currentTarget: { dataset: { name: string } } }) {
    const f = (e.currentTarget.dataset.name || 'all') as 'all' | 'pending' | 'approved' | 'rejected'
    this.setData({
      regionStatusFilter: f,
      regionDisplayApps: this._filterRegion(this.data.regionApps, f)
    })
  },

  _mergeRegion(
    existing: AdminRegionExpansionApplicationItem[],
    incoming: AdminRegionExpansionApplicationItem[]
  ): AdminRegionExpansionApplicationItem[] {
    const map = new Map<number, AdminRegionExpansionApplicationItem>()
    existing.forEach((i) => map.set(i.id, i))
    incoming.forEach((i) => map.set(i.id, i))
    return Array.from(map.values())
  },

  _filterRegion(list: AdminRegionExpansionApplicationItem[], status: string) {
    return status === 'all' ? list : list.filter((i) => i.status === status)
  },

  // ── 区域扩展审批 ──────────────────────────────────────────────────────────

  onApproveRegion(e: WechatMiniprogram.TouchEvent) {
    const { id, name, region } = e.currentTarget.dataset as { id: number, name: string, region: string }
    wx.showModal({
      title: '确认通过',
      content: `通过「${name}」申请管理「${region}」？通过后将自动关联区域。`,
      confirmText: '通过',
      success: (res) => { if (res.confirm) this._doApprove(id) }
    })
  },

  async _doApprove(id: number) {
    try {
      await platformManagementService.approveRegionExpansionApplication(id)
      await this.loadRegionApplications(true)
    } catch (e: unknown) {
      wx.showToast({ title: getErrorUserMessage(e, '审核失败，请稍后重试'), icon: 'none' })
    }
  },

  onRejectRegion(e: WechatMiniprogram.TouchEvent) {
    const { id, name, region } = e.currentTarget.dataset as { id: number, name: string, region: string }
    this.setData({
      rejectDialogVisible: true,
      rejectTargetId: id,
      rejectTargetDesc: `驳回「${name}」申请管理「${region}」`,
      rejectReason: ''
    })
  },

  onRejectReasonInput(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({ rejectReason: e.detail.value || '' })
  },

  onRejectCancel() {
    this.setData({ rejectDialogVisible: false })
  },

  async onRejectConfirm() {
    const { rejectTargetId, rejectReason } = this.data
    if (!rejectReason.trim() || rejectReason.trim().length < 2) {
      wx.showToast({ title: '请填写驳回原因（至少2字）', icon: 'none' })
      return
    }
    this.setData({ submittingReject: true })
    try {
      await platformManagementService.rejectRegionExpansionApplication(rejectTargetId, { reject_reason: rejectReason.trim() })
      this.setData({ rejectDialogVisible: false })
      await this.loadRegionApplications(true)
    } catch (e: unknown) {
      wx.showToast({ title: getErrorUserMessage(e, '驳回失败，请稍后重试'), icon: 'none' })
    } finally {
      this.setData({ submittingReject: false })
    }
  },

  mergeApplications(
    existing: AdminOperatorApplicationItem[],
    incoming: AdminOperatorApplicationItem[]
  ): AdminOperatorApplicationItem[] {
    if (!incoming.length) return existing
    const map = new Map<number, AdminOperatorApplicationItem>()
    existing.forEach((item) => map.set(item.id, item))
    incoming.forEach((item) => map.set(item.id, item))
    return Array.from(map.values())
  },

  buildFilterStats(list: AdminOperatorApplicationItem[]) {
    return {
      all: list.length,
      submitted: list.filter((item) => item.status === 'submitted').length,
      approved: list.filter((item) => item.status === 'approved').length,
      rejected: list.filter((item) => item.status === 'rejected').length
    }
  },

  getStatusLabel(status: string): string {
    if (status === 'submitted') return '待审核'
    if (status === 'approved') return '已通过'
    if (status === 'rejected') return '已驳回'
    return status || '未知状态'
  },

  getStatusTheme(status: string): 'warning' | 'success' | 'danger' | 'default' {
    if (status === 'submitted') return 'warning'
    if (status === 'approved') return 'success'
    if (status === 'rejected') return 'danger'
    return 'default'
  },

  getSortTime(item: AdminOperatorApplicationItem): number {
    const source = item.submitted_at || item.created_at
    const t = source ? new Date(source).getTime() : 0
    return Number.isFinite(t) ? t : 0
  },

  getStatusPriority(status: string, sortBy: SortBy): number {
    if (sortBy === 'approved_first') {
      if (status === 'approved') return 0
      if (status === 'submitted') return 1
      if (status === 'rejected') return 2
      return 3
    }
    if (sortBy === 'rejected_first') {
      if (status === 'rejected') return 0
      if (status === 'submitted') return 1
      if (status === 'approved') return 2
      return 3
    }
    if (sortBy === 'submitted_first') {
      if (status === 'submitted') return 0
      if (status === 'approved') return 1
      if (status === 'rejected') return 2
      return 3
    }
    return 0
  },

  applyFilterAndSort(
    source: AdminOperatorApplicationItem[],
    statusFilter: OperatorApplicationStatus,
    sortBy: SortBy
  ): OperatorApplicationDisplayItem[] {
    let list = source
    if (statusFilter !== 'all') {
      list = list.filter((item) => item.status === statusFilter)
    }

    const sorted = [...list]
    sorted.sort((a, b) => {
      if (sortBy === 'latest') return this.getSortTime(b) - this.getSortTime(a)
      if (sortBy === 'earliest') return this.getSortTime(a) - this.getSortTime(b)

      if (sortBy === 'approved_first' || sortBy === 'rejected_first' || sortBy === 'submitted_first') {
        const rank = this.getStatusPriority(a.status, sortBy) - this.getStatusPriority(b.status, sortBy)
        if (rank !== 0) return rank
        return this.getSortTime(b) - this.getSortTime(a)
      }

      const nameA = (a.name || a.legal_person_name || '').toLowerCase()
      const nameB = (b.name || b.legal_person_name || '').toLowerCase()
      if (sortBy === 'name_asc') return nameA.localeCompare(nameB, 'zh-CN')
      return nameB.localeCompare(nameA, 'zh-CN')
    })

    return sorted.map((item) => ({
      ...item,
      statusLabel: this.getStatusLabel(item.status),
      statusTheme: this.getStatusTheme(item.status)
    }))
  },

  onFilterChange(e: TapEvent) {
    const status = (e.currentTarget.dataset.name || 'all') as OperatorApplicationStatus
    if (status === this.data.statusFilter) return

    const applications = this.applyFilterAndSort(this.data.rawApplications, status, this.data.sortBy)
    this.setData({
      statusFilter: status,
      applications
    })
  },

  onSortPickerChange(e: WechatMiniprogram.CustomEvent<{ value: number | string }>) {
    const index = Number(e.detail.value || 0)
    const sortBy = this.data.sortOptions[index] || 'latest'
    const applications = this.applyFilterAndSort(this.data.rawApplications, this.data.statusFilter, sortBy)
    this.setData({
      sortIndex: index,
      sortBy,
      applications
    })
  },

  onDetailTap(e: TapEvent) {
    const id = Number(e.currentTarget.dataset.id || 0)
    if (!id) return
    wx.navigateTo({
      url: `/pages/platform/operators/detail?id=${id}`
    })
  }
})
