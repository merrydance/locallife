import { operatorBasicManagementService } from '../../../../api/operator-basic-management'
import { getOperatorAccountBalance, withdrawOperator } from '../../../../api/operator-finance'
import { getStableBarHeights } from '../../../../utils/responsive'

interface AmountChangeDetail {
  value: string
}

interface CommissionRowView {
  date: string
  order_count: number
  total_gmv_fen: number
  total_commission_fen: number
}

interface CommissionListResponseLike {
  commissions?: Array<{
    items?: Array<{
      date: string
      order_count: number
      total_gmv: number
      commission: number
    }>
  }>
  items?: Array<{
    date: string
    order_count: number
    total_gmv: number
    commission: number
  }>
}

Page({
  data: {
    navBarHeight: 88,
    loadingOverview: true,
    loadError: '',
    submitting: false,
    amountInput: '',
    amountError: '',
    submitDisabled: true,
    minAmountFen: 100,
    availableAmountFen: 0,
    totalIncomeFen: 0,
    currentMonthIncomeFen: 0,
    operatorShareRatio: 0,
    commissionLoading: true,
    commissionError: '',
    commissionRows: [] as CommissionRowView[]
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.loadOverview()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
    this.setData({ navBarHeight: e.detail?.navBarHeight || this.data.navBarHeight })
  },

  onPullDownRefresh() {
    this.loadOverview()
  },

  async loadOverview() {
    this.setData({ loadingOverview: true, commissionLoading: true, loadError: '', commissionError: '' })
    try {
      const [overview, balance, commissionList] = await Promise.all([
        operatorBasicManagementService.getFinanceOverview().catch(() => null),
        getOperatorAccountBalance().catch(() => null),
        operatorBasicManagementService.getCommissionList({ page: 1, limit: 10 }).catch(() => null)
      ])

      const withdrawableAmountFen = balance?.withdrawable_amount ?? 0
      const loadError = balance ? '' : '可提现余额加载失败，请稍后重试'
      const commissionRows = this.adaptCommissionRows(commissionList)
      const commissionError = commissionList ? '' : '佣金明细加载失败，请稍后重试'

      this.setData(
        {
          availableAmountFen: withdrawableAmountFen,
          totalIncomeFen: overview?.total?.operator_income ?? 0,
          currentMonthIncomeFen: overview?.current_month?.operator_income ?? 0,
          operatorShareRatio: overview?.operator_share_ratio ?? 0,
          loadError,
          commissionRows,
          commissionError,
          commissionLoading: false
        },
        () => this.updateSubmitState()
      )
    } catch (error) {
      console.error('加载运营商财务概览失败:', error)
      this.setData({ loadError: '资金信息加载失败，请稍后重试', commissionError: '佣金明细加载失败，请稍后重试', commissionLoading: false }, () => this.updateSubmitState())
    } finally {
      this.setData({ loadingOverview: false })
      wx.stopPullDownRefresh()
    }
  },

  adaptCommissionRows(response: unknown): CommissionRowView[] {
    if (!response || typeof response !== 'object') {
      return []
    }

    const payload = response as CommissionListResponseLike
    const rawItems = Array.isArray(payload.items)
      ? payload.items
      : Array.isArray(payload.commissions)
        ? payload.commissions.reduce<Array<{ date: string, order_count: number, total_gmv: number, commission: number }>>((accumulator, item) => {
          if (Array.isArray(item.items)) {
            accumulator.push(...item.items)
          }
          return accumulator
        }, [])
        : []

    return rawItems.slice(0, 10).map((item) => ({
      date: item.date,
      order_count: Number(item.order_count || 0),
      total_gmv_fen: Number(item.total_gmv || 0),
      total_commission_fen: Number(item.commission || 0)
    }))
  },

  onAmountChange(e: WechatMiniprogram.CustomEvent<AmountChangeDetail>) {
    const nextValue = this.normalizeAmountInput(e.detail.value || '')
    this.setData({ amountInput: nextValue }, () => this.updateSubmitState())
  },

  async onSubmit() {
    if (this.data.submitDisabled) {
      if (this.data.amountError) {
        wx.showToast({ title: this.data.amountError, icon: 'none' })
      }
      return
    }

    const amountFen = this.getInputAmountFen()
    if (amountFen < this.data.minAmountFen) {
      wx.showToast({ title: '提现金额至少1元', icon: 'none' })
      return
    }

    this.setData({ submitting: true, submitDisabled: true })

    try {
      await withdrawOperator({ amount: amountFen })
      wx.showToast({ title: '提现申请已提交', icon: 'success' })
      this.setData({ amountInput: '' })
      await this.loadOverview()
    } catch (error: unknown) {
      wx.showToast({ title: this.getWithdrawErrorMessage(error), icon: 'none' })
    } finally {
      this.setData({ submitting: false }, () => this.updateSubmitState())
    }
  },

  onRetryLoad() {
    this.loadOverview()
  },

  onBack() {
    wx.navigateBack()
  },

  formatFen(fen: number): string {
    return (fen / 100).toFixed(2)
  },

  formatShareRatio(ratio: number): string {
    if (!Number.isFinite(ratio) || ratio <= 0) return '--'
    return `${(ratio * 100).toFixed(0)}%`
  },

  normalizeAmountInput(rawValue: string): string {
    const cleaned = rawValue.replace(/[^\d.]/g, '')
    const firstDotIndex = cleaned.indexOf('.')
    if (firstDotIndex === -1) return cleaned

    const integerPart = cleaned.slice(0, firstDotIndex)
    const decimalPart = cleaned
      .slice(firstDotIndex + 1)
      .replace(/\./g, '')
      .slice(0, 2)

    return `${integerPart}.${decimalPart}`
  },

  getInputAmountFen(): number {
    const amount = Number(this.data.amountInput)
    if (!Number.isFinite(amount) || amount <= 0) return 0
    return Math.round(amount * 100)
  },

  updateSubmitState() {
    const amountFen = this.getInputAmountFen()
    let amountError = ''

    if (this.data.amountInput) {
      if (amountFen < this.data.minAmountFen) {
        amountError = '最低提现金额为1元'
      } else if (amountFen > this.data.availableAmountFen) {
        amountError = '金额超过可提现余额'
      }
    }

    const submitDisabled =
      this.data.loadingOverview ||
      !!this.data.loadError ||
      this.data.submitting ||
      this.data.availableAmountFen < this.data.minAmountFen ||
      !this.data.amountInput ||
      !!amountError

    this.setData({
      amountError,
      submitDisabled
    })
  },

  getWithdrawErrorMessage(error: unknown): string {
    const rawMessage = error instanceof Error ? error.message : '提现申请失败'
    const lowerMessage = rawMessage.toLowerCase()

    if (lowerMessage.includes('wallet account not bound')) {
      return '请先绑定提现账户后再申请提现'
    }

    if (lowerMessage.includes('insufficient balance')) {
      return '可提现余额不足'
    }

    if (lowerMessage.includes('operator is not active')) {
      return '账号未激活，暂不可提现'
    }

    return rawMessage || '提现申请失败'
  }
})
