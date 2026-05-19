import {
  createBaofuWithdrawal,
  getBaofuWithdrawalBalance
} from '../../../../../api/baofu-withdrawal'
import {
  buildBaofuWithdrawalBalanceView,
  buildBaofuWithdrawalSubmitCheck,
  type BaofuWithdrawalBalanceView
} from '../../../../../services/baofu-withdrawal-workflow'
import { logger } from '../../../../../utils/logger'
import { getStableBarHeights } from '../../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../../utils/user-facing'

const DETAIL_PAGE_PATH = '/pages/operator/finance/withdrawals/detail/index'
const EMPTY_BALANCE_VIEW = buildBaofuWithdrawalBalanceView(null)

Page({
  data: {
    navBarHeight: 88,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    loadingBalance: false,
    submitting: false,
    amountInput: '',
    balanceView: EMPTY_BALANCE_VIEW as BaofuWithdrawalBalanceView
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    await this.loadBalance()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  async loadBalance() {
    if (this.data.loadingBalance) {
      return
    }

    this.setData({
      loadingBalance: true,
      initialLoading: true,
      initialError: false,
      initialErrorMessage: ''
    })

    try {
      const balance = await getBaofuWithdrawalBalance('operator')
      this.setData({
        balanceView: buildBaofuWithdrawalBalanceView(balance),
        loadingBalance: false,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: ''
      })
    } catch (error) {
      logger.warn('Operator baofu withdrawal balance load failed', error)
      this.setData({
        loadingBalance: false,
        initialLoading: false,
        initialError: true,
        initialErrorMessage: getErrorUserMessage(error, '可提现余额加载失败，请稍后重试')
      })
    }
  },

  onRetry() {
    void this.loadBalance()
  },

  onAmountChange(e: WechatMiniprogram.CustomEvent<{ value?: string }>) {
    this.setData({ amountInput: String(e.detail.value || '') })
  },

  async onSubmit() {
    if (this.data.submitting) {
      return
    }

    const check = buildBaofuWithdrawalSubmitCheck(this.data.amountInput, this.data.balanceView)
    if (!check.canSubmit) {
      wx.showToast({ title: check.errorMessage || '请输入有效提现金额', icon: 'none' })
      return
    }

    this.setData({ submitting: true })
    try {
      const result = await createBaofuWithdrawal('operator', { amount: check.amount })
      wx.redirectTo({
        url: `${DETAIL_PAGE_PATH}?id=${result.withdrawal.id}&created=1`
      })
    } catch (error) {
      logger.warn('Submit operator baofu withdrawal failed', error)
      wx.showToast({
        title: getErrorUserMessage(error, '提现申请失败，请稍后重试'),
        icon: 'none'
      })
      this.setData({ submitting: false })
    }
  }
})
