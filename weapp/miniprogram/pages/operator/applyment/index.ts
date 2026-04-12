import {
  buildOperatorApplymentStatusView,
  DEFAULT_OPERATOR_APPLYMENT_STATUS_VIEW,
  getOperatorApplymentStatus,
  operatorBindBank,
  type OperatorApplymentStatusResponse,
  type OperatorApplymentStatusView
} from '../../../api/operator-applyment'
import type { ApplymentBindBankDraftPayload, ApplymentBindBankPayload } from '../../../api/applyment-bank'
import { getErrorUserMessage } from '../../../utils/user-facing'

function copyText(data: string, successTitle: string) {
  const trimmed = String(data || '').trim()
  if (!trimmed) {
    return
  }

  wx.setClipboardData({
    data: trimmed,
    success: () => {
      wx.showToast({ title: successTitle, icon: 'success' })
    }
  })
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
    bindBankDraft: null as ApplymentBindBankDraftPayload | null,
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

  onBindDraftChange(e: WechatMiniprogram.CustomEvent<ApplymentBindBankDraftPayload>) {
    this.setData({ bindBankDraft: e.detail })
  },

  async onSubmit(e: WechatMiniprogram.CustomEvent<ApplymentBindBankPayload>) {
    if (this.data.submitting) return

    this.setData({ submitting: true, error: '' })
    try {
      await operatorBindBank(e.detail)
      this.setData({ bindBankDraft: null })
      wx.redirectTo({
        url: '/pages/operator/applyment/completed/index'
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
    copyText(this.data.statusView.signURL, '签约链接已复制')
  },

  onCopyLegalValidationUrl() {
    copyText(this.data.statusView.legalValidationURL, '验证链接已复制')
  },

  onCopyValidationAccountNumber() {
    copyText(this.data.statusView.accountValidation?.destinationAccountNumber || '', '收款卡号已复制')
  },

  onCopyValidationRemark() {
    copyText(this.data.statusView.accountValidation?.remark || '', '汇款备注已复制')
  }
})
