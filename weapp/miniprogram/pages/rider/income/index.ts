import {
  RiderIncomeLedgerResponse,
  riderIncomeApi,
  RiderIncomeStatus
} from '../../../api/rider-income'
import RiderService from '../../../api/rider'
import {
  buildDefaultRiderIncomeDateRange,
  buildRiderIncomeDailyItemView,
  buildRiderIncomeDateRangeLabel,
  buildRiderIncomeLedgerItemView,
  buildRiderIncomeSummaryView,
  RIDER_INCOME_PAGE_SIZE,
  RiderIncomeDateRange,
  RiderIncomeDailyItemView,
  RiderIncomeLedgerItemView,
  RiderIncomeStatusFilter,
  RiderIncomeSummaryView
} from '../../../services/rider-income'
import { logger } from '../../../utils/logger'
import { getStableBarHeights } from '../../../utils/responsive'
import { getErrorUserMessage } from '../../../utils/user-facing'

const DAILY_PREVIEW_LIMIT = 7

function emptySummaryView(): RiderIncomeSummaryView {
  return buildRiderIncomeSummaryView({
    total_deliveries: 0,
    total_rider_income: 0,
    total_delivery_fee: 0,
    status_summary: []
  })
}

function normalizeLedgerPage(response: RiderIncomeLedgerResponse, fallbackPage: number) {
  return {
    items: (response.items || []).map(buildRiderIncomeLedgerItemView),
    pageId: response.page_id || fallbackPage,
    pageSize: response.page_size || RIDER_INCOME_PAGE_SIZE,
    total: typeof response.total === 'number' ? response.total : 0,
    hasMore: typeof response.has_more === 'boolean'
      ? response.has_more
      : (response.page_id || fallbackPage) * (response.page_size || RIDER_INCOME_PAGE_SIZE) < (response.total || 0)
  }
}

function statusFilterToParam(status: RiderIncomeStatusFilter): RiderIncomeStatus | undefined {
  return status === 'all' ? undefined : status
}

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
    loading: false,
    loadingMore: false,
    statusTab: 'all' as RiderIncomeStatusFilter,
    dateRange: buildDefaultRiderIncomeDateRange() as RiderIncomeDateRange,
    dateRangeLabel: '',
    summary: emptySummaryView(),
    dailyItems: [] as RiderIncomeDailyItemView[],
    ledgerItems: [] as RiderIncomeLedgerItemView[],
    pageId: 1,
    pageSize: RIDER_INCOME_PAGE_SIZE,
    total: 0,
    hasMore: false,
    emptyDescription: '当前范围暂无配送费结算记录'
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
      emptyDescription: statusTab === 'all' ? '当前范围暂无配送费结算记录' : '当前状态下暂无结算记录'
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
      const range = this.data.dateRange
      const [summary, daily, ledger, riderStatus] = await Promise.all([
        riderIncomeApi.getSummary(range),
        riderIncomeApi.getDaily(range),
        riderIncomeApi.listLedger({
          ...range,
          status: statusFilterToParam(this.data.statusTab as RiderIncomeStatusFilter),
          page_id: 1,
          page_size: RIDER_INCOME_PAGE_SIZE
        }),
        RiderService.getStatus()
      ])
      const settlementNotice = riderStatus.settlement_account?.payment_ready === false
        ? '结算账户未开通，暂不能接收配送费分账订单'
        : ''
      const ledgerPage = normalizeLedgerPage(ledger, 1)
      this.setData({
        settlementNotice,
        summary: buildRiderIncomeSummaryView(summary),
        dailyItems: (daily.items || []).slice(0, DAILY_PREVIEW_LIMIT).map(buildRiderIncomeDailyItemView),
        ledgerItems: ledgerPage.items,
        pageId: ledgerPage.pageId,
        pageSize: ledgerPage.pageSize,
        total: ledgerPage.total,
        hasMore: ledgerPage.hasMore,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        listErrorMessage: '',
        loadMoreErrorMessage: ''
      })
    } catch (error) {
      logger.error('Load rider income page failed', error)
      const message = getErrorUserMessage(error, '配送费结算加载失败，请稍后重试')
      if (shouldShowInitialLoading) {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message,
          summary: emptySummaryView(),
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
      const response = await riderIncomeApi.listLedger({
        ...this.data.dateRange,
        status: statusFilterToParam(this.data.statusTab as RiderIncomeStatusFilter),
        page_id: pageId,
        page_size: RIDER_INCOME_PAGE_SIZE
      })
      const ledgerPage = normalizeLedgerPage(response, pageId)
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
