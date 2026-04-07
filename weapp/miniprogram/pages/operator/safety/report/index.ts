import {
  operatorBasicManagementService,
  SubmitSafetyReportRequest,
  getSafetyReportStatusDisplay,
  type SafetyReportStatus,
  type SafetyReportStatusTheme
} from '../../../../api/operator-basic-management'
import { getErrorUserMessage } from '../../../../utils/user-facing'

type SafetyStatusFilter = '' | SafetyReportStatus

type SafetyLevel = 'low' | 'medium' | 'high' | 'critical'

interface TabChangeDetail {
  value: SafetyStatusFilter
}

interface LevelChangeDetail {
  value: SafetyLevel
}

type SafetyReportView = {
  id: number
  title: string
  level: SafetyLevel
  level_label: string
  status: SafetyReportStatus
  status_label: string
  status_theme: SafetyReportStatusTheme
  created_at: string
}

function formatSafetyLevel(level: SafetyLevel): string {
  const labels: Record<SafetyLevel, string> = {
    low: '低',
    medium: '中',
    high: '高',
    critical: '严重'
  }
  return labels[level]
}

Page({
  data: {
    reports: [] as SafetyReportView[],
    loading: false,
    loadingMore: false,
    initialLoading: true,
    error: '',
    navBarHeight: 88,
    status: '' as SafetyStatusFilter,
    page: 1,
    limit: 20,
    hasMore: false,
    submitVisible: false,
    submitLoading: false,
    submitTitle: '',
    submitDescription: '',
    submitLevel: 'medium' as SafetyLevel,
    submitMerchantIdsRaw: ''
  },

  onLoad() {
    this.loadReports(true)
  },

  onShow() {
    if (!this.data.initialLoading) {
      this.loadReports(true)
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  onPullDownRefresh() {
    this.loadReports(true).finally(() => {
      wx.stopPullDownRefresh()
    })
  },

  adaptReport(item: { id: number, title: string, level: SafetyLevel, status: SafetyReportStatus, created_at: string }): SafetyReportView {
    const statusDisplay = getSafetyReportStatusDisplay(item.status)
    return {
      ...item,
      level_label: formatSafetyLevel(item.level),
      status_label: statusDisplay.label,
      status_theme: statusDisplay.theme
    }
  },

  async loadReports(reset = false) {
    if (this.data.loading || (this.data.loadingMore && !reset)) return
    const nextPage = reset ? 1 : this.data.page
    if (reset) {
      this.setData({ loading: true, error: '' })
    } else {
      this.setData({ loadingMore: true })
    }

    try {
      const res = await operatorBasicManagementService.getSafetyReports({
        page: nextPage,
        limit: this.data.limit,
        status: this.data.status || undefined
      })
      const current = reset ? [] : this.data.reports
      this.setData({
        reports: [...current, ...(res.items || []).map((item) => this.adaptReport(item))],
        page: nextPage + 1,
        hasMore: Boolean(res.has_more),
        loading: false,
        loadingMore: false,
        initialLoading: false
      })
    } catch (error: unknown) {
      const message = getErrorUserMessage(error, '加载食安事件失败，请稍后重试')
      this.setData({ loading: false, loadingMore: false, initialLoading: false, error: message })
    }
  },

  onTabChange(e: WechatMiniprogram.CustomEvent<TabChangeDetail>) {
    this.setData({ status: e.detail.value })
    this.loadReports(true)
  },

  onLoadMore() {
    if (!this.data.hasMore || this.data.loading) return
    this.loadReports(false)
  },

  onToggleSubmit() {
    this.setData({ submitVisible: !this.data.submitVisible })
  },

  onSubmitTitleChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({ submitTitle: e.detail.value || '' })
  },

  onSubmitDescriptionChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({ submitDescription: e.detail.value || '' })
  },

  onSubmitLevelChange(e: WechatMiniprogram.CustomEvent<LevelChangeDetail>) {
    this.setData({ submitLevel: e.detail.value })
  },

  onSubmitMerchantIdsChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({ submitMerchantIdsRaw: e.detail.value || '' })
  },

  parseMerchantIds(raw: string): number[] {
    if (!raw.trim()) return []
    return raw.split(',').map((token) => Number(token.trim())).filter((id) => Number.isInteger(id) && id > 0)
  },

  async onSubmitReport() {
    if (!this.data.submitTitle.trim()) {
      wx.showToast({ title: '请输入事件标题', icon: 'none' })
      return
    }
    if (this.data.submitDescription.trim().length < 5) {
      wx.showToast({ title: '事件描述至少 5 个字符', icon: 'none' })
      return
    }

    const payload: SubmitSafetyReportRequest = {
      title: this.data.submitTitle.trim(),
      description: this.data.submitDescription.trim(),
      level: this.data.submitLevel,
      merchant_ids: this.parseMerchantIds(this.data.submitMerchantIdsRaw)
    }

    try {
      this.setData({ submitLoading: true })
      wx.showLoading({ title: '提交中', mask: true })
      await operatorBasicManagementService.submitSafetyReport(payload)
      this.setData({
        submitVisible: false,
        submitTitle: '',
        submitDescription: '',
        submitLevel: 'medium',
        submitMerchantIdsRaw: ''
      })
      this.loadReports(true)
    } catch (error: unknown) {
      const message = getErrorUserMessage(error, '提交失败，请稍后重试')
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      this.setData({ submitLoading: false })
      wx.hideLoading()
    }
  },

  onDetail(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    wx.navigateTo({ url: `/pages/operator/safety/detail/index?id=${id}` })
  }
})
