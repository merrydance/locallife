import {
  appealManagementService,
  claimManagementService,
  AppealResponse,
  ClaimResponse
} from '../_main_shared/api/appeals-customer-service'
import { logger } from '../../../utils/logger'
import {
  buildRiderAppealViewStatus,
  ClaimTagTheme,
  formatClaimType,
  formatMoney,
  formatTime,
  getRiderAppealStatusView,
  getRiderClaimActionHint,
  getRiderClaimStatusView,
  getRiderRecoveryStatusView
} from '../_utils/rider-claims-view'
import { getStableBarHeights } from '../../../utils/responsive'
import { getErrorUserMessage } from '../../../utils/user-facing'
type ClaimBucketTab = 'all' | 'pending_action' | 'disputed' | 'closed'
type ClaimsSectionTab = 'claims' | 'appeals'

const PAGE_SIZE = 20

interface RiderClaimView {
  id: number
  orderNo: string
  claimTypeLabel: string
  claimTypeTheme: ClaimTagTheme
  claimAmountText: string
  approvedAmountText: string
  statusLabel: string
  statusTheme: ClaimTagTheme
  description: string
  appealStatusLabel: string
  appealStatusTheme: ClaimTagTheme
  recoveryStatusLabel: string
  recoveryStatusTheme: ClaimTagTheme
  createdAtLabel: string
  actionHint: string
}

interface RiderAppealView {
  id: number
  claimId: number
  orderNo: string
  claimTypeLabel: string
  statusLabel: string
  statusTheme: ClaimTagTheme
  reason: string
  createdAtLabel: string
  reviewNotes?: string
}

interface ClaimSummary {
  total: number
  pendingAction: number
  disputed: number
  closed: number
}

function getClaimTypeTheme(claimType: string): ClaimTagTheme {
  if (claimType === 'food-safety' || claimType === 'foreign-object') return 'danger'
  if (claimType === 'damage' || claimType === 'quality_issue') return 'warning'
  return 'primary'
}

const getErrorMessage = getErrorUserMessage

function buildClaimView(claim: ClaimResponse): RiderClaimView {
  const claimStatusView = getRiderClaimStatusView(String(claim.status))
  const appealStatusView = getRiderAppealStatusView(claim.recovery_dispute_status || claim.appeal_status)
  const recoveryStatusView = getRiderRecoveryStatusView(claim.recovery_status)

  return {
    id: claim.id,
    orderNo: claim.order_no || `#${claim.order_id}`,
    claimTypeLabel: formatClaimType(claim.claim_type),
    claimTypeTheme: getClaimTypeTheme(claim.claim_type),
    claimAmountText: formatMoney(claim.claim_amount),
    approvedAmountText: formatMoney(claim.approved_amount || claim.claim_amount),
    statusLabel: claimStatusView.label,
    statusTheme: claimStatusView.theme,
    description: claim.description,
    appealStatusLabel: appealStatusView.label,
    appealStatusTheme: appealStatusView.theme,
    recoveryStatusLabel: recoveryStatusView.label,
    recoveryStatusTheme: recoveryStatusView.theme,
    createdAtLabel: formatTime(claim.created_at),
    actionHint: getRiderClaimActionHint(claim)
  }
}

function buildAppealView(appeal: AppealResponse): RiderAppealView {
  const appealStatusView = buildRiderAppealViewStatus(appeal.status)

  return {
    id: appeal.id,
    claimId: appeal.claim_id,
    orderNo: appeal.order_no || `#${appeal.claim_id}`,
    claimTypeLabel: formatClaimType(appeal.claim_type || 'compensation'),
    statusLabel: appealStatusView.label,
    statusTheme: appealStatusView.theme,
    reason: appeal.reason,
    createdAtLabel: formatTime(appeal.created_at),
    reviewNotes: appeal.review_notes
  }
}

function toBucket(tab: ClaimBucketTab): 'pending_action' | 'disputed' | 'closed' | undefined {
  return tab === 'all' ? undefined : tab
}

async function fetchClaimSummary(): Promise<ClaimSummary> {
  const summary = await claimManagementService.getRiderClaimsSummary()

  return {
    total: summary.total || 0,
    pendingAction: summary.pending_action || 0,
    disputed: summary.disputed || summary.appealed || 0,
    closed: summary.closed || 0
  }
}

async function fetchClaimPage(bucket: ClaimBucketTab, pageId: number) {
  const result = await claimManagementService.getRiderClaims({
    page_id: pageId,
    page_size: PAGE_SIZE,
    bucket: toBucket(bucket)
  })

  return {
    claims: (result.claims || []).map(buildClaimView),
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
    loading: false,
    loadingMore: false,
    sectionTab: 'claims' as ClaimsSectionTab,
    bucketTab: 'all' as ClaimBucketTab,
    claims: [] as RiderClaimView[],
    appeals: [] as RiderAppealView[],
    pageId: 1,
    pageSize: PAGE_SIZE,
    hasMore: false,
    summary: {
      total: 0,
      pendingAction: 0,
      disputed: 0,
      closed: 0
    }
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.reloadPageData(false)
  },

  onShow() {
    if (!this.data.initialLoading && !this.data.loading && !this.data.loadingMore) {
      this.reloadPageData(true)
    }
  },

  onPullDownRefresh() {
    this.reloadPageData(false)
  },

  onRetryRefresh() {
    this.reloadPageData(false)
  },

  onReachBottom() {
    if (this.data.sectionTab === 'claims') {
      this.loadMoreClaims()
    }
  },

  onSectionTabChange(e: WechatMiniprogram.CustomEvent<{ value: ClaimsSectionTab }>) {
    this.setData({ sectionTab: e.detail.value })
  },

  onBucketTabChange(e: WechatMiniprogram.CustomEvent<{ value: ClaimBucketTab }>) {
    const bucketTab = e.detail.value
    if (bucketTab === this.data.bucketTab) return

    this.setData({
      bucketTab,
      claims: [],
      pageId: 1,
      hasMore: false,
      loading: true,
      initialError: false,
      initialErrorMessage: '',
      refreshErrorMessage: ''
    })
    this.loadClaimList(bucketTab)
  },

  async reloadPageData(silent = false) {
    this.setData(
      silent
        ? { refreshErrorMessage: '' }
        : { loading: true, initialError: false, initialErrorMessage: '', refreshErrorMessage: '' }
    )

    try {
      const [summary, claimsPage, appealsResult] = await Promise.all([
        fetchClaimSummary(),
        fetchClaimPage(this.data.bucketTab as ClaimBucketTab, 1),
        appealManagementService.getRiderAppeals({ page_id: 1, page_size: 20 })
      ])

      this.setData({
        summary,
        claims: claimsPage.claims,
        appeals: (appealsResult.appeals || []).map(buildAppealView),
        pageId: claimsPage.pageId,
        pageSize: claimsPage.pageSize,
        hasMore: claimsPage.hasMore,
        loading: false,
        loadingMore: false,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: ''
      })
    } catch (error) {
      logger.error('Load rider claims failed', error)
      const message = getErrorMessage(error, '索赔与申诉加载失败，请稍后重试')
      if (this.data.initialLoading || !silent) {
        this.setData({
          loading: false,
          loadingMore: false,
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message,
          refreshErrorMessage: '',
          claims: [],
          appeals: [],
          hasMore: false,
          summary: {
            total: 0,
            pendingAction: 0,
            disputed: 0,
            closed: 0
          }
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

  async loadClaimList(bucketTab: ClaimBucketTab) {
    try {
      const page = await fetchClaimPage(bucketTab, 1)
      this.setData({
        claims: page.claims,
        pageId: page.pageId,
        pageSize: page.pageSize,
        hasMore: page.hasMore,
        loading: false,
        loadingMore: false,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: ''
      })
    } catch (error) {
      logger.error('Switch rider claims bucket failed', error)
      const message = getErrorMessage(error, '索赔列表加载失败，请稍后重试')
      this.setData({
        loading: false,
        loadingMore: false,
        initialLoading: false,
        initialError: true,
        initialErrorMessage: message,
        claims: [],
        hasMore: false
      })
    }
  },

  async loadMoreClaims() {
    if (this.data.loading || this.data.loadingMore || !this.data.hasMore) {
      return
    }

    this.setData({ loadingMore: true })
    try {
      const nextPage = this.data.pageId + 1
      const page = await fetchClaimPage(this.data.bucketTab as ClaimBucketTab, nextPage)
      this.setData({
        claims: this.data.claims.concat(page.claims),
        pageId: page.pageId,
        pageSize: page.pageSize,
        hasMore: page.hasMore,
        loadingMore: false
      })
    } catch (error) {
      logger.error('Load more rider claims failed', error)
      wx.showToast({ title: getErrorMessage(error, '加载更多索赔失败，请稍后重试'), icon: 'none' })
      this.setData({ loadingMore: false })
    }
  },

  onViewClaimDetail(e: WechatMiniprogram.TouchEvent) {
    const { claimId } = e.currentTarget.dataset as { claimId?: number }
    if (!claimId) return
    wx.navigateTo({ url: `/pages/rider/claims/detail/index?id=${claimId}` })
  },

  onViewAppealClaim(e: WechatMiniprogram.TouchEvent) {
    const { claimId } = e.currentTarget.dataset as { claimId?: number }
    if (!claimId) return
    wx.navigateTo({ url: `/pages/rider/claims/detail/index?id=${claimId}` })
  },

  onRetry() {
    this.reloadPageData(false)
  }
})
