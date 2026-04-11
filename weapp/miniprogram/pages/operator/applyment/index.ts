import {
  buildOperatorApplymentStatusView,
  DEFAULT_OPERATOR_APPLYMENT_STATUS_VIEW,
  getOperatorApplymentStatus,
  operatorBindBank,
  type OperatorApplymentStatusResponse,
  type OperatorApplymentStatusView
} from '../../../api/operator-applyment'
import type { ApplymentBindBankPayload } from '../../../api/applyment-bank'
import { getErrorUserMessage } from '../../../utils/user-facing'

function buildSubmitSuccessMessage(message: string, applymentId: string): string {
  const content = [message || '开户申请已提交']

  if (applymentId && applymentId !== '-') {
    content.push(`申请单号：${applymentId}`)
  }

  content.push('接下来：微信支付通常会在1-3个工作日内完成审核，审核中无需重复提交。')
  return content.join('\n')
}

function buildSubmitFailureMessage(message: string): string {
  return [
    message || '提交开户申请失败，请稍后重试',
    '如已确认资料无误，请稍后重新提交，或联系平台协助排查。'
  ].join('\n')
}

Page({
  data: {
    navBarHeight: 88,
    loading: true,
    submitting: false,
    refreshingStatus: false,
    error: '',
    bindBankDraft: null as ApplymentBindBankPayload | null,
    status: null as OperatorApplymentStatusResponse | null,
    statusView: { ...DEFAULT_OPERATOR_APPLYMENT_STATUS_VIEW } as OperatorApplymentStatusView
  },

  onLoad() {
    this.loadStatus()
  },

  async onPullDownRefresh() {
    await this.loadStatus()
    wx.stopPullDownRefresh()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  async loadStatus() {
    this.setData({ loading: true, error: '' })
    try {
      const status = await getOperatorApplymentStatus()
      const statusView = buildOperatorApplymentStatusView(status)
      if (statusView.isOpened) {
        wx.redirectTo({ url: '/pages/operator/applyment/completed/index' })
        return
      }

      this.setData({
        status,
        statusView
      })
    } catch (error: unknown) {
      const message = getErrorUserMessage(error, '获取开户状态失败，请稍后重试')
      this.setData({
        error: message,
        status: null,
        statusView: { ...DEFAULT_OPERATOR_APPLYMENT_STATUS_VIEW }
      })
    } finally {
      this.setData({ loading: false })
    }
  },

  async onRefreshStatus() {
    if (this.data.refreshingStatus || this.data.loading) {
      return
    }

    this.setData({ refreshingStatus: true })
    try {
      await this.loadStatus()
    } finally {
      this.setData({ refreshingStatus: false })
    }
  },

  onCancelForm() {
    this.setData({ error: '' })
  },

  onBindDraftChange(e: WechatMiniprogram.CustomEvent<ApplymentBindBankPayload>) {
    this.setData({ bindBankDraft: e.detail })
  },

  async onSubmit(e: WechatMiniprogram.CustomEvent<ApplymentBindBankPayload>) {
    if (this.data.submitting) return

    this.setData({ submitting: true, error: '' })
    try {
      const resp = await operatorBindBank(e.detail)
      this.setData({ bindBankDraft: null })
      await this.loadStatus()

      const applymentId = resp.applyment_id ? String(resp.applyment_id) : this.data.statusView.applymentId
      const message = encodeURIComponent(buildSubmitSuccessMessage(resp.message || '开户申请已提交', applymentId))
      const query = [`message=${message}`]
      if (applymentId && applymentId !== '-') {
        query.push(`applymentId=${encodeURIComponent(applymentId)}`)
      }
      wx.redirectTo({
        url: `/pages/operator/applyment/success/index?${query.join('&')}`
      })
    } catch (error: unknown) {
      const message = getErrorUserMessage(error, '提交开户申请失败，请稍后重试')
      wx.showModal({
        title: '提交未完成',
        content: buildSubmitFailureMessage(message),
        showCancel: false,
        confirmText: '我知道了'
      })
    } finally {
      this.setData({ submitting: false })
    }
  },

  onOpenSignUrl() {
    const signURL = this.data.statusView.signURL
    if (!signURL) return
    wx.setClipboardData({ data: signURL })
  }
})
