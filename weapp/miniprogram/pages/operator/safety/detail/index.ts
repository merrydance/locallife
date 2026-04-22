import {
  loadOperatorFoodSafetyDetailPageData,
  saveOperatorFoodSafetyInvestigation,
  saveOperatorFoodSafetyResolution,
  type OperatorFoodSafetyCaseDetailView,
  type OperatorFoodSafetyIncidentView
} from '../../../../services/operator-safety'
import { getErrorUserMessage } from '../../../../utils/user-facing'

interface InputDetail {
  value: string
}

Page({
  data: {
    id: 0,
    navBarHeight: 88,
    loading: true,
    initialLoading: true,
    investigateSubmitting: false,
    resolveSubmitting: false,
    error: '',
    caseDetail: null as OperatorFoodSafetyCaseDetailView | null,
    incidents: [] as OperatorFoodSafetyIncidentView[],
    investigationReport: '',
    merchantRectificationReport: '',
    resolution: ''
  },

  onLoad(options: Record<string, string>) {
    const id = Number(options.id || 0)
    if (!id) {
      wx.showToast({ title: '事件ID无效', icon: 'none' })
      return
    }
    this.setData({ id })
    this.loadDetail()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  async loadDetail() {
    this.setData({ loading: true, error: '' })
    try {
      const nextView = await loadOperatorFoodSafetyDetailPageData(this.data.id)
      this.setData({
        ...nextView,
        loading: false,
        initialLoading: false
      })
    } catch (error: unknown) {
      const message = getErrorUserMessage(error, '加载详情失败，请稍后重试')
      this.setData({ loading: false, initialLoading: false, error: message })
    }
  },

  onRetry() {
    this.loadDetail()
  },

  onInvestigationReportChange(e: WechatMiniprogram.CustomEvent<InputDetail>) {
    this.setData({ investigationReport: e.detail.value })
  },

  onMerchantRectificationReportChange(e: WechatMiniprogram.CustomEvent<InputDetail>) {
    this.setData({ merchantRectificationReport: e.detail.value })
  },

  onResolutionChange(e: WechatMiniprogram.CustomEvent<InputDetail>) {
    this.setData({ resolution: e.detail.value })
  },

  async onSubmitInvestigation() {
    if (!this.data.caseDetail?.is_active) {
      wx.showToast({ title: '案件已结案，当前为只读状态', icon: 'none' })
      return
    }

    const investigationReport = this.data.investigationReport.trim()
    if (investigationReport.length < 10) {
      wx.showToast({ title: '调查报告至少 10 个字符', icon: 'none' })
      return
    }

    try {
      this.setData({ investigateSubmitting: true })
      wx.showLoading({ title: '保存中' })
      await saveOperatorFoodSafetyInvestigation(this.data.id, investigationReport)
      await this.loadDetail()
      wx.showToast({ title: '调查报告已保存', icon: 'success' })
    } catch (error: unknown) {
      const message = getErrorUserMessage(error, '保存调查报告失败，请稍后重试')
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      this.setData({ investigateSubmitting: false })
      wx.hideLoading()
    }
  },

  async onSubmitResolution() {
    if (!this.data.caseDetail?.is_active) {
      wx.showToast({ title: '案件已结案，当前为只读状态', icon: 'none' })
      return
    }

    const investigationReport = this.data.investigationReport.trim() || this.data.caseDetail.investigation_report || undefined
    const merchantRectificationReport = this.data.merchantRectificationReport.trim()
    const resolution = this.data.resolution.trim()

    if (merchantRectificationReport.length < 10) {
      wx.showToast({ title: '整改报告至少 10 个字符', icon: 'none' })
      return
    }
    if (resolution.length < 5) {
      wx.showToast({ title: '结案结论至少 5 个字符', icon: 'none' })
      return
    }

    try {
      this.setData({ resolveSubmitting: true })
      wx.showLoading({ title: '提交中' })
      await saveOperatorFoodSafetyResolution({
        id: this.data.id,
        investigationReport,
        merchantRectificationReport,
        resolution
      })
      await this.loadDetail()
      wx.showToast({ title: '案件已结案', icon: 'success' })
    } catch (error: unknown) {
      const message = getErrorUserMessage(error, '处置失败，请稍后重试')
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      this.setData({ resolveSubmitting: false })
      wx.hideLoading()
    }
  }
})
