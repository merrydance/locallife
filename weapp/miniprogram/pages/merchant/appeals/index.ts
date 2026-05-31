import dayjs from '../_main_shared/miniprogram_npm/dayjs/index'
import { AppealResponse, AppealStatus, appealManagementService } from '../_main_shared/api/appeals-customer-service'
import { logger } from '../../../utils/logger'
import {
  getMerchantAppealResultHint,
  getMerchantAppealStatusView,
  MerchantAppealTagTheme
} from '../_utils/merchant-appeal-view'
import { getStableBarHeights } from '../../../utils/responsive'
import { getErrorUserMessage } from '../../../utils/user-facing'

type AppealTagTheme = MerchantAppealTagTheme

interface AppealRecordView {
  id: number
  claimId: number
  orderNo: string
  claimDescription: string
  statusLabel: string
  statusTheme: AppealTagTheme
  reason: string
  claimTypeLabel: string
  claimAmountText: string
  compensationAmountText?: string
  reviewNotes?: string
  reviewedAtLabel: string
  createdAtLabel: string
  resultHint: string
  rawStatus: string
}

type AppealTab = 'all' | 'submitted' | 'approved' | 'rejected'

const PAGE_SIZE = 20

interface AppealSummary {
  total: number
  submitted: number
  approved: number
  rejected: number
}

function formatMoney(cents?: number): string {
  if (typeof cents !== 'number') return ''
  return `¥${(cents / 100).toFixed(2)}`
}

function formatClaimType(claimType?: string): string {
  const map: Record<string, string> = {
    refund: '退款',
    compensation: '补偿',
    quality_issue: '质量问题',
    delivery_issue: '代取问题',
    'foreign-object': '异物',
    damage: '餐损',
    timeout: '超时',
    'food-safety': '食安'
  }
  if (!claimType) return '-'
  return map[claimType] || claimType
}

function formatTime(value?: string) {
  if (!value) return '暂无'
  const parsed = dayjs(value)
  return parsed.isValid() ? parsed.format('YYYY-MM-DD HH:mm') : value
}

const getErrorMessage = getErrorUserMessage

function toAppealStatus(tab: AppealTab): AppealStatus | undefined {
  return tab === 'all' ? undefined : tab
}

function mapAppealRecord(appeal: AppealResponse): AppealRecordView {
  const statusView = getMerchantAppealStatusView(appeal.status, '-')

  return {
    id: appeal.id,
    claimId: appeal.claim_id,
    orderNo: appeal.order_no || `#${appeal.claim_id}`,
    claimDescription: appeal.claim_description || '暂无索赔说明',
    statusLabel: statusView.label,
    statusTheme: statusView.theme,
    reason: appeal.reason,
    claimTypeLabel: formatClaimType(appeal.claim_type),
    claimAmountText: formatMoney(appeal.claim_amount),
    compensationAmountText: typeof appeal.compensation_amount === 'number' ? formatMoney(appeal.compensation_amount) : undefined,
    reviewNotes: appeal.review_notes,
    reviewedAtLabel: formatTime(appeal.reviewed_at),
    createdAtLabel: formatTime(appeal.created_at),
    resultHint: getMerchantAppealResultHint(appeal.status),
    rawStatus: appeal.status
  }
}

async function fetchAppealSummary(): Promise<AppealSummary> {
  const summary = await appealManagementService.getMerchantAppealsSummary()

  return {
    total: summary.total || 0,
    submitted: summary.submitted || summary.pending || 0,
    approved: summary.approved || 0,
    rejected: summary.rejected || 0
  }
}

async function fetchAppealPage(tab: AppealTab, pageId: number) {
  const result = await appealManagementService.getMerchantAppeals({
    page_id: pageId,
    page_size: PAGE_SIZE,
    status: toAppealStatus(tab)
  })

  return {
    appeals: (result.appeals || []).map(mapAppealRecord),
    pageId: result.page_id || pageId,
    pageSize: result.page_size || PAGE_SIZE,
    total: result.total || 0,
    hasMore: typeof result.has_more === 'boolean'
      ? result.has_more
      : (result.page_id || pageId) * (result.page_size || PAGE_SIZE) < (result.total || 0)
  }
}

Page({
  data: {
    navBarHeight: 88,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    loading: true,
    loadingMore: false,
    currentTab: 'all' as AppealTab,
    appeals: [] as AppealRecordView[],
    filteredAppeals: [] as AppealRecordView[],
    pageId: 1,
    pageSize: PAGE_SIZE,
    hasMore: false,
    summary: {
      total: 0,
      submitted: 0,
      approved: 0,
      rejected: 0
    }
  },

  onTabChange(e: WechatMiniprogram.CustomEvent<{ value: AppealTab }>) {
    const currentTab = e.detail.value
    if (currentTab === this.data.currentTab) return
    this.setData({
      currentTab,
      appeals: [],
      filteredAppeals: [],
      pageId: 1,
      hasMore: false,
      loading: true,
      initialError: false,
      initialErrorMessage: '',
      refreshErrorMessage: ''
    })
    this.loadAppealList(currentTab)
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.reloadPageData(false)
  },

  onShow() {
    if (!this.data.initialLoading && !this.data.loading) {
      this.reloadPageData(true)
    }
  },

  onPullDownRefresh() {
    this.reloadPageData(false)
  },

  onReachBottom() {
    this.loadMoreAppeals()
  },

  async reloadPageData(silent = false) {
    if (!silent) {
      this.setData({ loading: true, initialError: false, initialErrorMessage: '', refreshErrorMessage: '' })
    }

    try {
      const currentTab = this.data.currentTab as AppealTab
      const [summary, page] = await Promise.all([
        fetchAppealSummary(),
        fetchAppealPage(currentTab, 1)
      ])

      this.setData({
        summary,
        appeals: page.appeals,
        filteredAppeals: page.appeals,
        pageId: page.pageId,
        pageSize: page.pageSize,
        hasMore: page.hasMore,
        loading: false,
        loadingMore: false,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: ''
      })
    } catch (error) {
      logger.error('Load merchant appeals failed', error)
      const message = getErrorMessage(error, '异议记录加载失败，请稍后重试')
      if (this.data.initialLoading || !silent) {
        this.setData({
          loading: false,
          loadingMore: false,
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message,
          refreshErrorMessage: '',
          appeals: [],
          filteredAppeals: [],
          hasMore: false
        })
      } else {
        this.setData({
          loading: false,
          loadingMore: false,
          refreshErrorMessage: `${message}，当前已保留上次同步结果`
        })
      }
    } finally {
      wx.stopPullDownRefresh()
    }
  },

  onRetryRefresh() {
    this.reloadPageData(true)
  },

  async loadAppealList(tab: AppealTab) {
    try {
      const page = await fetchAppealPage(tab, 1)
      this.setData({
        appeals: page.appeals,
        filteredAppeals: page.appeals,
        pageId: page.pageId,
        pageSize: page.pageSize,
        hasMore: page.hasMore,
        loading: false,
        loadingMore: false,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: ''
      })
    } catch (error) {
      logger.error('Switch merchant appeals tab failed', error)
      const message = getErrorMessage(error, '异议记录加载失败，请稍后重试')
      this.setData({
        loading: false,
        loadingMore: false,
        initialLoading: false,
        initialError: true,
        initialErrorMessage: message,
        refreshErrorMessage: '',
        appeals: [],
        filteredAppeals: [],
        hasMore: false
      })
    }
  },

  async loadMoreAppeals() {
    if (this.data.loading || this.data.loadingMore || !this.data.hasMore) {
      return
    }

    this.setData({ loadingMore: true })
    try {
      const nextPage = this.data.pageId + 1
      const page = await fetchAppealPage(this.data.currentTab as AppealTab, nextPage)
      const filteredAppeals = this.data.filteredAppeals.concat(page.appeals)
      this.setData({
        appeals: filteredAppeals,
        filteredAppeals,
        pageId: page.pageId,
        pageSize: page.pageSize,
        hasMore: page.hasMore,
        loadingMore: false
      })
    } catch (error) {
      logger.error('Load more merchant appeals failed', error)
      wx.showToast({
        title: getErrorMessage(error, '加载更多异议失败，请稍后重试'),
        icon: 'none'
      })
      this.setData({ loadingMore: false })
    }
  },

  onViewAppealDetail(e: WechatMiniprogram.TouchEvent) {
    const { appealId } = e.currentTarget.dataset as { appealId?: number }
    if (!appealId) return
    wx.navigateTo({ url: `/pages/merchant/appeals/detail/index?id=${appealId}` })
  },

  onRetry() {
    this.reloadPageData(false)
  }
})
