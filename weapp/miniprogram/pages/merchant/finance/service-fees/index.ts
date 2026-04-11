import {
  getMerchantServiceFees,
  type MerchantServiceFeeItem,
  type MerchantServiceFeeSummaryResponse
} from '../../../../api/merchant-finance-bills'
import {
  buildFinanceRange,
  FINANCE_RANGE_OPTIONS,
  formatAmountText,
  formatDateTime,
  getOrderSourceText
} from '../analysis-shared'
import { ensureMerchantConsoleAccess } from '../../../../utils/console-access'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../utils/user-facing'

type FinanceRangeKey = '7d' | '30d'

const EMPTY_SUMMARY: MerchantServiceFeeSummaryResponse = {
  details: [],
  total_platform_fee: 0,
  total_operator_fee: 0,
  total_service_fee: 0
}

function resolveRangeKey(range?: string): FinanceRangeKey {
  return range === '30d' ? '30d' : '7d'
}

Page({
  data: {
    navBarHeight: 0,
    accessReady: false,
    accessDenied: false,
    accessErrorMessage: '',
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    loading: false,
    currentRange: '7d' as FinanceRangeKey,
    rangeOptions: FINANCE_RANGE_OPTIONS,
    dateRangeLabel: '--',
    updatedAtLabel: '--',
    summary: EMPTY_SUMMARY as MerchantServiceFeeSummaryResponse,
    details: [] as MerchantServiceFeeItem[]
  },

  async onLoad(options?: { range?: string }) {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight, currentRange: resolveRangeKey(options?.range) })
    await this.bootstrapPage()
  },

  async onPullDownRefresh() {
    if (this.data.accessReady && !this.data.accessDenied) {
      await this.loadData({ force: true })
    }
    wx.stopPullDownRefresh()
  },

  async bootstrapPage() {
    const access = await ensureMerchantConsoleAccess()

    if (access.status !== 'granted') {
      this.setData({
        accessReady: true,
        accessDenied: true,
        accessErrorMessage: access.message || '当前账号暂时不能查看服务费明细',
        initialLoading: false
      })
      return
    }

    this.setData({ accessReady: true, accessDenied: false, accessErrorMessage: '' })
    await this.loadData({ force: true })
  },

  async loadData(options?: { force?: boolean }) {
    const { force = false } = options || {}
    if (!force && this.data.loading) {
      return
    }

    const { params, label } = buildFinanceRange(this.data.currentRange)
    const hasData = this.data.details.length > 0

    this.setData({
      loading: true,
      initialLoading: !hasData,
      initialError: false,
      initialErrorMessage: '',
      refreshErrorMessage: '',
      dateRangeLabel: label
    })

    try {
      const response = await getMerchantServiceFees(params)
      this.setData({
        summary: response,
        details: response.details || [],
        loading: false,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        updatedAtLabel: formatDateTime(new Date().toISOString()).slice(11, 16)
      })
    } catch (error) {
      logger.error('Load merchant finance service fees failed', error, 'merchant-finance-service-fees')
      const message = getErrorUserMessage(error, '服务费明细加载失败，请稍后重试')
      this.setData({
        loading: false,
        initialLoading: false,
        initialError: !hasData,
        initialErrorMessage: !hasData ? message : '',
        refreshErrorMessage: hasData ? `${message}，当前已保留上次同步结果` : ''
      })
    }
  },

  onSelectRange(e: WechatMiniprogram.TouchEvent) {
    const { key } = e.currentTarget.dataset as { key?: FinanceRangeKey }
    if (!key || key === this.data.currentRange) {
      return
    }

    this.setData({ currentRange: key }, () => {
      void this.loadData({ force: true })
    })
  },

  onRetry() {
    void this.loadData({ force: true })
  },

  onBack() {
    wx.navigateBack({ delta: 1 })
  },

  formatAmountText,
  formatDateTime,
  getOrderSourceText
})