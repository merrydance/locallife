import { getOperatorApplymentStatus, operatorBindBank, type OperatorApplymentStatusResponse } from '../../../api/operator-applyment'
import type { ApplymentBindBankPayload } from '../../../api/applyment-bank'
import { getErrorUserMessage } from '../../../utils/user-facing'

interface StatusViewModel {
  statusCode: string
  statusDesc: string
  applymentId: string
  subMchId: string
  rejectReason: string
  signURL: string
  isOpened: boolean
  canSubmitOpenInfo: boolean
  isInReview: boolean
  needsSign: boolean
  guideText: string
}

const DEFAULT_STATUS_VIEW: StatusViewModel = {
  statusCode: 'pending',
  statusDesc: '未提交',
  applymentId: '-',
  subMchId: '-',
  rejectReason: '-',
  signURL: '',
  isOpened: false,
  canSubmitOpenInfo: true,
  isInReview: false,
  needsSign: false,
  guideText: '当前尚未开通微信支付商户，请提交必要信息完成开户。'
}

function buildStatusView(status: OperatorApplymentStatusResponse | null): StatusViewModel {
  if (!status) {
    return { ...DEFAULT_STATUS_VIEW }
  }

  const statusCode = status.status || 'pending'
  const statusDescMap: Record<string, string> = {
    pending: '未提交开户信息',
    active: '可提交开户信息',
    bindbank_submitted: '开户信息已提交',
    submitted: '微信审核中',
    auditing: '微信审核中',
    to_be_signed: '待签约确认',
    signing: '签约处理中',
    finish: '开户完成',
    rejected: '开户被拒绝',
    rejected_sign: '签约失败'
  }
  const statusDesc = status.status_desc || statusDescMap[statusCode] || statusCode
  const isOpened = statusCode === 'finish' && Boolean(status.sub_mch_id)
  const needsSign = statusCode === 'to_be_signed' || statusCode === 'signing'
  const isInReview = statusCode === 'bindbank_submitted' || statusCode === 'submitted' || statusCode === 'auditing' || needsSign
  const canSubmitOpenInfo = statusCode === 'pending' || statusCode === 'active' || statusCode === 'rejected' || statusCode === 'rejected_sign'

  let guideText = '当前尚未开通微信支付商户，请提交必要信息完成开户。'
  if (statusCode === 'rejected' || statusCode === 'rejected_sign') {
    guideText = '开户被拒，请根据拒绝原因修改信息后重新提交。'
  } else if (statusCode === 'bindbank_submitted' || statusCode === 'submitted' || statusCode === 'auditing') {
    guideText = '微信支付正在审核开户信息，审核期间无需重复提交。'
  } else if (needsSign) {
    guideText = '微信支付已进入签约阶段，请尽快完成签约确认。'
  } else if (isOpened) {
    guideText = '微信支付商户已开通，可正常经营与提现。'
  }

  return {
    statusCode,
    statusDesc,
    applymentId: status.applyment_id ? String(status.applyment_id) : '-',
    subMchId: status.sub_mch_id || '-',
    rejectReason: status.reject_reason || '-',
    signURL: status.sign_url || '',
    isOpened,
    canSubmitOpenInfo,
    isInReview,
    needsSign,
    guideText
  }
}

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
    error: '',
    status: null as OperatorApplymentStatusResponse | null,
    statusView: { ...DEFAULT_STATUS_VIEW }
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
      this.setData({
        status,
        statusView: buildStatusView(status)
      })
    } catch (error: unknown) {
      const message = getErrorUserMessage(error, '获取开户状态失败，请稍后重试')
      this.setData({
        error: message,
        status: null,
        statusView: { ...DEFAULT_STATUS_VIEW }
      })
    } finally {
      this.setData({ loading: false })
    }
  },

  onCancelForm() {
    this.setData({ error: '' })
  },

  async onSubmit(e: WechatMiniprogram.CustomEvent<ApplymentBindBankPayload>) {
    if (this.data.submitting) return

    this.setData({ submitting: true, error: '' })
    try {
      const resp = await operatorBindBank(e.detail)
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
