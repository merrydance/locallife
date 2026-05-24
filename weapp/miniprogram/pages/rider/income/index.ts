import {
  buildEmptyRiderIncomeSummaryView,
  buildEmptyRiderIncomeWithdrawalBalanceView,
  buildDefaultRiderIncomeDateRange,
  buildRiderIncomeDateRangeLabel,
  loadRiderIncomeLedgerPage,
  loadRiderIncomePageData,
  RIDER_INCOME_PAGE_SIZE,
  RiderIncomeDateRange,
  RiderIncomeDailyItemView,
  RiderIncomeLedgerItemView,
  RiderIncomeStatusFilter
} from '../../../services/rider-income'
import type { BaofuWithdrawalBalanceView } from '../../../services/baofu-withdrawal-workflow'
import { logger } from '../../../utils/logger'
import { getStableBarHeights } from '../../../utils/responsive'
import { getErrorUserMessage } from '../../../utils/user-facing'

const DAILY_PREVIEW_LIMIT = 7
const EMPTY_WITHDRAWAL_BALANCE_VIEW = buildEmptyRiderIncomeWithdrawalBalanceView()

Page({
  data: {
    navBarHeight: 88,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    listErrorMessage: '',
    loadMoreErrorMessage: '',
    settlementNotice: '',
    withdrawalBalanceReady: false,
    withdrawalEntryNote: '',
    withdrawalBalanceView: EMPTY_WITHDRAWAL_BALANCE_VIEW as BaofuWithdrawalBalanceView,
    loading: false,
    loadingMore: false,
    statusTab: 'all' as RiderIncomeStatusFilter,
    dateRange: buildDefaultRiderIncomeDateRange() as RiderIncomeDateRange,
    dateRangeLabel: '',
    summary: buildEmptyRiderIncomeSummaryView(),
    dailyItems: [] as RiderIncomeDailyItemView[],
    ledgerItems: [] as RiderIncomeLedgerItemView[],
    pageId: 1,
    pageSize: RIDER_INCOME_PAGE_SIZE,
    total: 0,
    hasMore: false,
    emptyDescription: '当前范围暂无代取费结算记录'
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    const dateRange = buildDefaultRiderIncomeDateRange()
    this.setData({
      navBarHeight,
      dateRange,
      dateRangeLabel: buildRiderIncomeDateRangeLabel(dateRange)
    })
    this.reloadPageData(false)
  },

  onShow() {
    if (this.data.initialLoading || this.data.loading || this.data.loadingMore) {
      return
    }
    this.reloadPageData(true)
  },

  onPullDownRefresh() {
    this.reloadPageData(false)
  },

  onReachBottom() {
    this.loadMoreLedger()
  },

  onRetryPage() {
    this.reloadPageData(false)
  },

  onRetryRefresh() {
    this.reloadPageData(true)
  },

  onRetryLedger() {
    this.loadLedger(1, true)
  },

  onRetryLoadMore() {
    this.loadLedger(this.data.pageId + 1, false)
  },

  onGoToSettlementAccount() {
    wx.navigateTo({ url: '/pages/rider/settlement-account/index' })
  },

  onGoToWithdrawals() {
    wx.navigateTo({ url: '/pages/rider/income/withdrawals/index' })
  },

  onStatusTabChange(e: WechatMiniprogram.CustomEvent<{ value: RiderIncomeStatusFilter }>) {
    const statusTab = e.detail.value
    if (statusTab === this.data.statusTab) {
      return
    }

    this.setData({
      statusTab,
      ledgerItems: [],
      pageId: 1,
      total: 0,
      hasMore: false,
      listErrorMessage: '',
      loadMoreErrorMessage: '',
      emptyDescription: statusTab === 'all' ? '当前范围暂无代取费结算记录' : '当前状态下暂无结算记录'
    })
    this.loadLedger(1, true)
  },

  async reloadPageData(silent: boolean) {
    const shouldShowInitialLoading = !silent && (this.data.initialLoading || this.data.initialError)
    this.setData(shouldShowInitialLoading
      ? {
        initialLoading: true,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        listErrorMessage: '',
        loadMoreErrorMessage: ''
      }
      : { loading: true, refreshErrorMessage: '', listErrorMessage: '', loadMoreErrorMessage: '' })

    try {
      const pageView = await loadRiderIncomePageData({
        dateRange: this.data.dateRange,
        statusTab: this.data.statusTab as RiderIncomeStatusFilter,
        dailyPreviewLimit: DAILY_PREVIEW_LIMIT,
        pageSize: RIDER_INCOME_PAGE_SIZE
      })
      this.setData({
        settlementNotice: pageView.settlementNotice,
        withdrawalBalanceReady: pageView.withdrawalBalanceReady,
        withdrawalBalanceView: pageView.withdrawalBalanceView,
        withdrawalEntryNote: pageView.withdrawalEntryNote,
        summary: pageView.summary,
        dailyItems: pageView.dailyItems,
        ledgerItems: pageView.ledgerPage.items,
        pageId: pageView.ledgerPage.pageId,
        pageSize: pageView.ledgerPage.pageSize,
        total: pageView.ledgerPage.total,
        hasMore: pageView.ledgerPage.hasMore,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        listErrorMessage: '',
        loadMoreErrorMessage: ''
      })
    } catch (error) {
      logger.error('Load rider income page failed', error)
      const message = getErrorUserMessage(error, '代取费结算加载失败，请稍后重试')
      if (shouldShowInitialLoading) {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message,
          summary: buildEmptyRiderIncomeSummaryView(),
          dailyItems: [],
          ledgerItems: [],
          pageId: 1,
          total: 0,
          hasMore: false,
          listErrorMessage: '',
          loadMoreErrorMessage: ''
        })
      } else {
        this.setData({ refreshErrorMessage: `${message}，当前已保留上次同步结果` })
      }
    } finally {
      this.setData({ loading: false, loadingMore: false })
      wx.stopPullDownRefresh()
    }
  },

  async loadLedger(pageId: number, reset: boolean) {
    if (reset) {
      this.setData({ loading: true, listErrorMessage: '', loadMoreErrorMessage: '' })
    } else {
      this.setData({ loadingMore: true, loadMoreErrorMessage: '' })
    }

    try {
      const ledgerPage = await loadRiderIncomeLedgerPage({
        dateRange: this.data.dateRange,
        statusTab: this.data.statusTab as RiderIncomeStatusFilter,
        pageId,
        pageSize: RIDER_INCOME_PAGE_SIZE
      })
      this.setData({
        ledgerItems: reset ? ledgerPage.items : this.data.ledgerItems.concat(ledgerPage.items),
        pageId: ledgerPage.pageId,
        pageSize: ledgerPage.pageSize,
        total: ledgerPage.total,
        hasMore: ledgerPage.hasMore,
        listErrorMessage: '',
        loadMoreErrorMessage: ''
      })
    } catch (error) {
      logger.error('Load rider income ledger failed', error)
      const message = getErrorUserMessage(error, '结算明细加载失败，请稍后重试')
      if (reset) {
        this.setData({ listErrorMessage: message, ledgerItems: [], hasMore: false })
      } else {
        this.setData({ loadMoreErrorMessage: message })
      }
    } finally {
      this.setData({ loading: false, loadingMore: false })
    }
  },

  loadMoreLedger() {
    if (this.data.loading || this.data.loadingMore || !this.data.hasMore || this.data.loadMoreErrorMessage) {
      return
    }
    this.loadLedger(this.data.pageId + 1, false)
  }
})
