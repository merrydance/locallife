import { getOperatorApplymentStatus, operatorBindBank, type OperatorApplymentStatusResponse } from '../../../api/operator-applyment'

type AccountType = 'ACCOUNT_TYPE_PRIVATE' | 'ACCOUNT_TYPE_BUSINESS'

Page({
  data: {
    navBarHeight: 88,
    loading: true,
    submitting: false,
    error: '',
    success: '',
    status: null as OperatorApplymentStatusResponse | null,
    statusView: {
      statusCode: 'pending',
      statusDesc: '未提交',
      applymentId: '-',
      subMchId: '-',
      rejectReason: '-',
      signURL: '',
      isOpened: false,
      canSubmitOpenInfo: true,
      isInReview: false,
      guideText: '当前尚未开通微信支付商户，请提交必要信息完成开户。'
    },
    form: {
      account_type: 'ACCOUNT_TYPE_PRIVATE' as AccountType,
      account_bank: '',
      bank_address_code: '',
      bank_name: '',
      account_number: '',
      account_name: '',
      contact_phone: '',
      contact_email: ''
    }
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
        statusView: {
          statusCode: status.status || 'pending',
          statusDesc: status.status_desc || status.status || '未提交',
          applymentId: status.applyment_id ? String(status.applyment_id) : '-',
          subMchId: status.sub_mch_id || '-',
          rejectReason: status.reject_reason || '-',
          signURL: status.sign_url || '',
          isOpened: this.isOpened(status.status, status.sub_mch_id),
          canSubmitOpenInfo: this.canSubmitOpenInfo(status.status),
          isInReview: this.isInReview(status.status),
          guideText: this.getGuideText(status.status)
        }
      })
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : '获取开户状态失败'
      this.setData({
        error: message,
        status: null,
        statusView: {
          statusCode: 'pending',
          statusDesc: '未提交',
          applymentId: '-',
          subMchId: '-',
          rejectReason: '-',
          signURL: '',
          isOpened: false,
          canSubmitOpenInfo: true,
          isInReview: false,
          guideText: '当前尚未开通微信支付商户，请提交必要信息完成开户。'
        }
      })
    } finally {
      this.setData({ loading: false })
    }
  },

  onInputChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const field = (e.currentTarget.dataset.field || '') as keyof typeof this.data.form
    if (!field) return
    this.setData({ [`form.${field}`]: e.detail.value })
  },

  onAccountTypeChange(e: WechatMiniprogram.CustomEvent<{ value: AccountType }>) {
    this.setData({ 'form.account_type': e.detail.value })
  },

  async onSubmit() {
    if (this.data.submitting) return

    const form = this.data.form
    if (!form.account_bank || !form.bank_address_code || !form.account_number || !form.account_name || !form.contact_phone) {
      wx.showToast({ title: '请填写完整必填字段', icon: 'none' })
      return
    }

    this.setData({ submitting: true, success: '', error: '' })
    try {
      const resp = await operatorBindBank({
        ...form,
        bank_name: form.bank_name || undefined,
        contact_email: form.contact_email || undefined
      })
      this.setData({ success: resp.message || '开户申请已提交' })
      wx.showToast({ title: '提交成功', icon: 'success' })
      await this.loadStatus()
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : '提交开户申请失败'
      this.setData({ error: message })
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      this.setData({ submitting: false })
    }
  },

  onOpenSignUrl() {
    const signURL = this.data.statusView.signURL
    if (!signURL) return
    wx.setClipboardData({ data: signURL })
  },

  isOpened(status: string, subMchID?: string) {
    return status === 'finish' && Boolean(subMchID)
  },

  canSubmitOpenInfo(status: string) {
    return status === 'pending' || status === 'rejected' || status === 'rejected_sign'
  },

  isInReview(status: string) {
    return status === 'submitted' || status === 'auditing' || status === 'to_be_signed' || status === 'signing'
  },

  getGuideText(status: string) {
    if (status === 'rejected' || status === 'rejected_sign') {
      return '开户被拒，请根据拒绝原因修改信息后重新提交。'
    }
    return '当前尚未开通微信支付商户，请提交必要信息完成开户。'
  }
})
