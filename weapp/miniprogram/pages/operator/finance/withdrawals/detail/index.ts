import { getBaofuWithdrawal } from '../../../_main_shared/api/baofu-withdrawal'
import {
  buildBaofuWithdrawalItemView,
  type BaofuWithdrawalItemView
} from '../../../_main_shared/services/baofu-withdrawal-workflow'
import { logger } from '../../../../../utils/logger'
import { getStableBarHeights } from '../../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../../utils/user-facing'

const WITHDRAWAL_LIST_PAGE_PATH = '/pages/operator/finance/withdrawals/index'
const WITHDRAWAL_DETAIL_POLL_INTERVAL_MS = 15000

Page({
  data: {
    navBarHeight: 88,
    id: 0,
    createdNotice: false,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    loadingDetail: false,
    item: null as BaofuWithdrawalItemView | null
  },

  _pollTimer: null as number | null,
  _pageVisible: false,

  async onLoad(options: { id?: string, created?: string } = {}) {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    const id = Number(options.id || 0)
    if (!id) {
      this.setData({
        id: 0,
        initialLoading: false,
        initialError: true,
        initialErrorMessage: '提现记录不存在'
      })
      return
    }
    this.setData({ id, createdNotice: options.created === '1' })
    await this.loadDetail()
    this.updatePolling()
  },

  onShow() {
    this._pageVisible = true
    if (this.data.id && this.data.item) {
      void this.loadDetail({ silent: true }).then(() => this.updatePolling())
    }
  },

  onHide() {
    this._pageVisible = false
    this.stopPolling()
  },

  onUnload() {
    this._pageVisible = false
    this.stopPolling()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
  },

  async loadDetail(options: { silent?: boolean } = {}) {
    if (!this.data.id || this.data.loadingDetail) {
      return
    }
    const { silent = false } = options
    const hasTrustedData = Boolean(this.data.item)

    this.setData({
      loadingDetail: true,
      ...(silent || hasTrustedData
        ? { refreshErrorMessage: '' }
        : {
            initialLoading: true,
            initialError: false,
            initialErrorMessage: '',
            refreshErrorMessage: ''
          })
    })

    try {
      const response = await getBaofuWithdrawal('operator', this.data.id)
      this.setData({
        item: buildBaofuWithdrawalItemView(response.withdrawal),
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        loadingDetail: false
      })
      this.updatePolling()
    } catch (error) {
      logger.warn('Operator baofu withdrawal detail load failed', error)
      const message = getErrorUserMessage(error, '提现详情加载失败，请稍后重试')
      this.setData({
        initialLoading: false,
        initialError: !hasTrustedData,
        initialErrorMessage: hasTrustedData ? '' : message,
        refreshErrorMessage: hasTrustedData ? message : '',
        loadingDetail: false
      })
    }
  },

  onRetry() {
    void this.loadDetail()
  },

  onRefresh() {
    void this.loadDetail({ silent: true })
  },

  updatePolling() {
    const shouldPoll = Boolean(this.data.item && !this.data.item.statusView.isTerminal)
    if (!this._pageVisible || !shouldPoll) {
      this.stopPolling()
      return
    }
    if (this._pollTimer !== null) {
      return
    }
    this._pollTimer = setInterval(() => {
      void this.loadDetail({ silent: true })
    }, WITHDRAWAL_DETAIL_POLL_INTERVAL_MS) as unknown as number
  },

  stopPolling() {
    if (this._pollTimer === null) {
      return
    }
    clearInterval(this._pollTimer)
    this._pollTimer = null
  },

  onBackToList() {
    wx.redirectTo({ url: WITHDRAWAL_LIST_PAGE_PATH })
  }
})
