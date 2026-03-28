import { operatorBasicManagementService, SafetyReportItem } from '../../../../api/operator-basic-management'

type ResolveStatus = 'resolved' | 'rejected'

interface InputDetail {
  value: string
}

interface RadioChangeDetail {
  value: ResolveStatus
}

Page({
  data: {
    id: 0,
    navBarHeight: 88,
    loading: true,
    initialLoading: true,
    submitting: false,
    resumeSubmitting: false,
    error: '',
    report: null as SafetyReportItem | null,
    resolutionStatus: 'resolved' as ResolveStatus,
    resolutionNotes: '',
    recoverMerchantIdsRaw: '',
    recoverReason: '',
    singleResumeMerchantId: '',
    singleResumeReason: '',
    recoveredMerchantIds: [] as number[]
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
      const report = await operatorBasicManagementService.getSafetyReportDetail(this.data.id)
      const normalizedReport: SafetyReportItem = {
        ...report,
        merchant_ids: Array.isArray((report as unknown as { merchant_ids?: unknown }).merchant_ids)
          ? report.merchant_ids
          : []
      }
      this.setData({
        report: normalizedReport,
        resolutionStatus: normalizedReport.status === 'rejected' ? 'rejected' : 'resolved',
        resolutionNotes: normalizedReport.resolution_notes || '',
        loading: false,
        initialLoading: false
      })
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : '加载详情失败'
      this.setData({ loading: false, initialLoading: false, error: message })
    }
  },

  onRetry() {
    this.loadDetail()
  },

  onResolutionStatusChange(e: WechatMiniprogram.CustomEvent<RadioChangeDetail>) {
    this.setData({ resolutionStatus: e.detail.value })
  },

  onResolutionNotesChange(e: WechatMiniprogram.CustomEvent<InputDetail>) {
    this.setData({ resolutionNotes: e.detail.value })
  },

  onRecoverMerchantIdsChange(e: WechatMiniprogram.CustomEvent<InputDetail>) {
    this.setData({ recoverMerchantIdsRaw: e.detail.value })
  },

  onRecoverReasonChange(e: WechatMiniprogram.CustomEvent<InputDetail>) {
    this.setData({ recoverReason: e.detail.value })
  },

  onSingleResumeMerchantIdChange(e: WechatMiniprogram.CustomEvent<InputDetail>) {
    this.setData({ singleResumeMerchantId: e.detail.value })
  },

  onSingleResumeReasonChange(e: WechatMiniprogram.CustomEvent<InputDetail>) {
    this.setData({ singleResumeReason: e.detail.value })
  },

  parseMerchantIds(raw: string): number[] {
    if (!raw.trim()) return []
    return raw
      .split(',')
      .map((token) => Number(token.trim()))
      .filter((id) => Number.isInteger(id) && id > 0)
  },

  async onSubmitResolution() {
    if (this.data.report?.status !== 'pending') {
      wx.showToast({ title: '事件已处理，当前为只读状态', icon: 'none' })
      return
    }

    if (!this.data.resolutionNotes.trim()) {
      wx.showToast({ title: '请填写处置报告', icon: 'none' })
      return
    }

    const recoverMerchantIds = this.parseMerchantIds(this.data.recoverMerchantIdsRaw)
    if (recoverMerchantIds.length > 0 && !this.data.recoverReason.trim()) {
      wx.showToast({ title: '请填写恢复原因', icon: 'none' })
      return
    }

    try {
      this.setData({ submitting: true })
      wx.showLoading({ title: '提交中' })
      const result = await operatorBasicManagementService.resolveSafetyReport(this.data.id, {
        status: this.data.resolutionStatus,
        resolution_notes: this.data.resolutionNotes,
        recover_merchant_ids: recoverMerchantIds,
        recover_reason: this.data.recoverReason || undefined
      })
      this.setData({ recoveredMerchantIds: result.recovered_merchant_ids || [] })
      wx.showToast({ title: '处置成功', icon: 'success' })
      await this.loadDetail()
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : '处置失败'
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      this.setData({ submitting: false })
      wx.hideLoading()
    }
  },

  async onResumeSingleMerchant() {
    if (this.data.report?.status !== 'pending') {
      wx.showToast({ title: '事件已处理，当前为只读状态', icon: 'none' })
      return
    }

    const merchantId = Number(this.data.singleResumeMerchantId)
    if (!merchantId) {
      wx.showToast({ title: '请输入有效商户ID', icon: 'none' })
      return
    }
    if (!this.data.singleResumeReason.trim()) {
      wx.showToast({ title: '请填写恢复原因', icon: 'none' })
      return
    }

    try {
      this.setData({ resumeSubmitting: true })
      wx.showLoading({ title: '恢复中' })
      await operatorBasicManagementService.resumeMerchant(merchantId, this.data.singleResumeReason)
      wx.showToast({ title: '恢复成功', icon: 'success' })
      this.setData({
        singleResumeMerchantId: '',
        singleResumeReason: '',
        recoveredMerchantIds: [...this.data.recoveredMerchantIds, merchantId]
      })
      await this.loadDetail()
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : '恢复失败'
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      this.setData({ resumeSubmitting: false })
      wx.hideLoading()
    }
  }
})
