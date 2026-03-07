import { getStableBarHeights } from '../../../utils/responsive'
import {
  createMerchantWithdraw,
  getMerchantAccountBalance,
  getMerchantApplymentStatus,
  merchantBindBank,
  listMerchantWithdrawals,
  ApplymentStatusResponse,
  MerchantAccountBalanceResponse,
  MerchantWithdrawItem
} from '../../../api/merchant-finance'
import { logger } from '../../../utils/logger'

type InputChangeDetail = {
  value: string
}

const emptyApplyment: ApplymentStatusResponse = {
  status: '',
  status_desc: ''
}

Page({
  data: {
    navBarHeight: 88,
    loading: true,
    submitting: false,
    notConfigured: false,

    /* Balance */
    balance: {
      sub_mch_id: '',
      available_amount: 0,
      pending_amount: 0,
      withdrawable_amount: 0
    } as MerchantAccountBalanceResponse,
    withdrawAmountYuan: '',
    withdrawRemark: '',
    withdrawals: [] as MerchantWithdrawItem[],

    /* Applyment */
    loadingApplyment: true,
    applymentLoaded: false,
    applymentStatus: emptyApplyment as ApplymentStatusResponse | null,
    hasApplyment: false,

    /* Bind bank form */
    showBindForm: false,
    submittingBind: false,
    bindAccountType: 'ACCOUNT_TYPE_BUSINESS',
    bindAccountBank: '',
    bindBankAddressCode: '',
    bindBankName: '',
    bindAccountNumber: '',
    bindAccountName: '',
    bindContactPhone: '',
    bindContactEmail: ''
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.loadData()
  },

  onPullDownRefresh() {
    this.loadData()
  },

  async loadData() {
    this.setData({ loading: true })
    await Promise.all([this.loadBalance(), this.loadApplymentStatus()])
    this.setData({ loading: false })
    wx.stopPullDownRefresh()
  },

  async loadBalance() {
    try {
      const [balance, records] = await Promise.all([
        getMerchantAccountBalance(),
        listMerchantWithdrawals(1, 20)
      ])
      this.setData({
        balance,
        notConfigured: false,
        withdrawals: records.withdrawals || []
      })
    } catch (error: unknown) {
      const msg = (error instanceof Error ? error.message : '') || ''
      if (msg.includes('404')) {
        this.setData({ notConfigured: true })
      } else {
        logger.error('Load merchant finance data failed', error, 'merchant-finance')
        wx.showToast({ title: '加载资金数据失败', icon: 'none' })
      }
    }
  },

  async loadApplymentStatus() {
    this.setData({ loadingApplyment: true })
    try {
      const data = await getMerchantApplymentStatus()
      this.setData({
        applymentStatus: data,
        hasApplyment: true,
        applymentLoaded: true,
        showBindForm: false
      })
    } catch (error: unknown) {
      const msg = (error instanceof Error ? error.message : '') || ''
      if (msg.includes('404')) {
        this.setData({ applymentStatus: null, hasApplyment: false, applymentLoaded: true })
      } else {
        logger.error('Load applyment status failed', error, 'merchant-finance')
        wx.showToast({ title: '查询进件状态失败', icon: 'none' })
        this.setData({ applymentStatus: null, hasApplyment: false, applymentLoaded: true })
      }
    } finally {
      this.setData({ loadingApplyment: false })
    }
  },

  onWithdrawAmountChange(e: WechatMiniprogram.CustomEvent<InputChangeDetail>) {
    this.setData({ withdrawAmountYuan: e.detail.value })
  },

  onWithdrawRemarkChange(e: WechatMiniprogram.CustomEvent<InputChangeDetail>) {
    this.setData({ withdrawRemark: e.detail.value })
  },

  async onSubmitWithdraw() {
    if (this.data.submitting) return

    const amountYuan = Number(this.data.withdrawAmountYuan)
    if (!Number.isFinite(amountYuan) || amountYuan < 1) {
      wx.showToast({ title: '提现金额至少1元', icon: 'none' })
      return
    }

    if (!this.data.withdrawRemark.trim()) {
      wx.showToast({ title: '请输入提现备注', icon: 'none' })
      return
    }

    const amount = Math.round(amountYuan * 100)
    if (amount > this.data.balance.withdrawable_amount) {
      wx.showToast({ title: '超过可提现余额', icon: 'none' })
      return
    }

    this.setData({ submitting: true })
    wx.showLoading({ title: '提交中...' })

    try {
      await createMerchantWithdraw({
        amount,
        remark: this.data.withdrawRemark.trim()
      })

      wx.showToast({ title: '提现申请已提交', icon: 'success' })
      this.setData({ withdrawAmountYuan: '', withdrawRemark: '' })
      await this.loadData()
    } catch (error) {
      logger.error('Submit merchant withdraw failed', error, 'merchant-finance')
      wx.showToast({ title: '提现申请失败', icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ submitting: false })
    }
  },

  /* ── Bind bank ── */

  onShowBindForm() {
    this.setData({ showBindForm: true })
  },

  onHideBindForm() {
    this.setData({ showBindForm: false })
  },

  onBindAccountTypeChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({ bindAccountType: e.detail.value })
  },

  onBindFieldChange(
    e: WechatMiniprogram.CustomEvent<InputChangeDetail> & {
      currentTarget: { dataset: { field: string } }
    }
  ) {
    const field = e.currentTarget.dataset.field as
      | 'bindAccountBank'
      | 'bindBankAddressCode'
      | 'bindBankName'
      | 'bindAccountNumber'
      | 'bindAccountName'
      | 'bindContactPhone'
      | 'bindContactEmail'
    this.setData({ [field]: e.detail.value })
  },

  async onSubmitBindBank() {
    if (this.data.submittingBind) return

    const {
      bindAccountType,
      bindAccountBank,
      bindBankAddressCode,
      bindAccountNumber,
      bindAccountName,
      bindContactPhone,
      bindBankName,
      bindContactEmail
    } = this.data

    if (!bindAccountBank.trim() || !bindBankAddressCode.trim() || !bindAccountNumber.trim() || !bindAccountName.trim() || !bindContactPhone.trim()) {
      wx.showToast({ title: '请填写必填项', icon: 'none' })
      return
    }

    this.setData({ submittingBind: true })
    wx.showLoading({ title: '提交中...' })

    try {
      await merchantBindBank({
        account_type: bindAccountType as 'ACCOUNT_TYPE_BUSINESS' | 'ACCOUNT_TYPE_PRIVATE',
        account_bank: bindAccountBank.trim(),
        bank_address_code: bindBankAddressCode.trim(),
        bank_name: bindBankName.trim() || undefined,
        account_number: bindAccountNumber.trim(),
        account_name: bindAccountName.trim(),
        contact_phone: bindContactPhone.trim(),
        contact_email: bindContactEmail.trim() || undefined
      })

      wx.showToast({ title: '银行账户已提交，等待审核', icon: 'success' })
      this.setData({
        bindAccountBank: '',
        bindBankAddressCode: '',
        bindBankName: '',
        bindAccountNumber: '',
        bindAccountName: '',
        bindContactPhone: '',
        bindContactEmail: ''
      })
      await this.loadApplymentStatus()
    } catch (error) {
      logger.error('Submit bind bank failed', error, 'merchant-finance')
      wx.showToast({ title: '提交银行账户失败', icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ submittingBind: false })
    }
  },

  /* ── Utils ── */

  formatAmount(fen: number): string {
    return (fen / 100).toFixed(2)
  },

  getApplymentStatusText(status: string): string {
    const map: Record<string, string> = {
      submitted: '已提交',
      bindbank_submitted: '进件审核中',
      auditing: '审核中',
      to_be_signed: '待签约',
      signing: '签约中',
      finish: '已开通',
      active: '已开通',
      rejected: '已拒绝'
    }
    return map[status] || status
  },

  getApplymentStatusTheme(status: string): string {
    switch (status) {
      case 'finish':
      case 'active':
        return 'success'
      case 'rejected':
        return 'danger'
      case 'to_be_signed':
      case 'signing':
        return 'primary'
      default:
        return 'warning'
    }
  },

  getStatusText(status: string): string {
    switch (status) {
      case 'pending': return '处理中'
      case 'success': return '成功'
      case 'failed': return '失败'
      default: return status
    }
  },

  getStatusTheme(status: string): string {
    switch (status) {
      case 'pending': return 'warning'
      case 'success': return 'success'
      case 'failed': return 'danger'
      default: return 'default'
    }
  }
})
