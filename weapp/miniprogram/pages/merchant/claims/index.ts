import {
  claimManagementService,
  ClaimResponse
} from '../../../api/appeals-customer-service'
import { logger } from '../../../utils/logger'
import {
  ClaimTagTheme,
  formatAppealStatus,
  formatClaimStatus,
  formatClaimType,
  formatCompensationSource,
  formatMoney,
  formatRecoveryStatus,
  formatResponsibleParty,
  formatTime,
  getAppealStatusTheme,
  getClaimStatusTheme,
  getMerchantClaimListActionState,
  getRecoveryStatusTheme
} from '../../../utils/merchant-claim-detail-view'
import { getStableBarHeights } from '../../../utils/responsive'
import { getErrorUserMessage } from '../../../utils/user-facing'

interface MerchantClaimView {
  id: number
  orderNo: string
  orderAmountText: string
  customerLabel: string
  rawStatus: string
  rawAppealStatus?: string
  rawRecoveryStatus?: string
  claimTypeLabel: string
  claimTypeTheme: ClaimTagTheme
  claimAmountText: string
  approvedAmountText: string
  statusLabel: string
  statusTheme: ClaimTagTheme
  description: string
  appealStatusLabel: string
  appealStatusTheme: ClaimTagTheme
  createdAtLabel: string
  responsibilityLabel: string
  compensationSourceLabel: string
  recoveryStatusLabel: string
  recoveryStatusTheme: ClaimTagTheme
  actionHint: string
  hasAppeal: boolean
  isPendingAction: boolean
  isAppealedFlow: boolean
  isClosedFlow: boolean
}

type ClaimFilterTab = 'all' | 'pending_action' | 'appealed' | 'closed'

const PAGE_SIZE = 20

interface ClaimSummary {
  total: number
  pendingAction: number
  appealed: number
  closed: number
}

function getClaimTypeTheme(claimType: string): ClaimTagTheme {
  if (claimType === 'food-safety' || claimType === 'foreign-object') return 'danger'
  if (claimType === 'damage' || claimType === 'quality_issue') return 'warning'
  return 'primary'
}

const getErrorMessage = getErrorUserMessage

function buildBaseClaimView(claim: ClaimResponse): MerchantClaimView {
  const appealStatus = claim.appeal_status
  const recoveryStatus = claim.recovery_status
  const hasAppeal = Boolean(claim.appeal_id)
  const actionState = getMerchantClaimListActionState({
    status: claim.status,
    appealStatus,
    recoveryStatus
  })

  return {
    id: claim.id,
    orderNo: claim.order_no || `#${claim.order_id}`,
    orderAmountText: formatMoney(claim.order_amount),
    customerLabel: claim.user_name ? `${claim.user_name}${claim.user_phone ? ` · ${claim.user_phone}` : ''}` : (claim.user_phone || '顾客信息未返回'),
    rawStatus: claim.status,
    rawAppealStatus: appealStatus,
    rawRecoveryStatus: recoveryStatus,
    claimTypeLabel: formatClaimType(claim.claim_type),
    claimTypeTheme: getClaimTypeTheme(claim.claim_type),
    claimAmountText: formatMoney(claim.claim_amount),
    approvedAmountText: formatMoney(claim.approved_amount || claim.claim_amount),
    statusLabel: formatClaimStatus(claim.status),
    statusTheme: getClaimStatusTheme(claim.status),
    description: claim.description,
    appealStatusLabel: formatAppealStatus(appealStatus),
    appealStatusTheme: getAppealStatusTheme(appealStatus),
    createdAtLabel: formatTime(claim.created_at),
    responsibilityLabel: '责任待拉取',
    compensationSourceLabel: '赔付来源待拉取',
    recoveryStatusLabel: formatRecoveryStatus(recoveryStatus),
    recoveryStatusTheme: getRecoveryStatusTheme(recoveryStatus),
    actionHint: actionState.actionHint,
    hasAppeal,
    isPendingAction: actionState.isPendingAction,
    isAppealedFlow: actionState.isAppealedFlow,
    isClosedFlow: actionState.isClosedFlow
  }
}

function toBucket(tab: ClaimFilterTab): 'pending_action' | 'appealed' | 'closed' | undefined {
  return tab === 'all' ? undefined : tab
}

async function fetchClaimSummary(): Promise<ClaimSummary> {
  const summary = await claimManagementService.getMerchantClaimsSummary()

  return {
    total: summary.total || 0,
    pendingAction: summary.pending_action || 0,
    appealed: summary.appealed || 0,
    closed: summary.closed || 0
  }
}

async function fetchClaimPage(tab: ClaimFilterTab, pageId: number) {
  const result = await claimManagementService.getMerchantClaims({
    page_id: pageId,
    page_size: PAGE_SIZE,
    bucket: toBucket(tab)
  })

  return {
    claims: (result.claims || []).map(buildBaseClaimView),
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
    currentTab: 'all' as ClaimFilterTab,
    claims: [] as MerchantClaimView[],
    filteredClaims: [] as MerchantClaimView[],
    pageId: 1,
    pageSize: PAGE_SIZE,
    hasMore: false,
    summary: {
      total: 0,
      pendingAction: 0,
      appealed: 0,
      closed: 0
    }
  },

  onViewAppeals() {
    wx.navigateTo({ url: '/pages/merchant/appeals/index' })
  },

  onViewDetail(e: WechatMiniprogram.TouchEvent) {
    const { claimId } = e.currentTarget.dataset as { claimId?: number }
    if (!claimId) return
    wx.navigateTo({ url: `/pages/merchant/claims/detail/index?id=${claimId}` })
  },

  onTabChange(e: WechatMiniprogram.CustomEvent<{ value: ClaimFilterTab }>) {
    const currentTab = e.detail.value
    if (currentTab === this.data.currentTab) return
    this.setData({
      currentTab,
      claims: [],
      filteredClaims: [],
      pageId: 1,
      hasMore: false,
      loading: true,
      initialError: false,
      initialErrorMessage: '',
      refreshErrorMessage: ''
    })
    this.loadClaimList(currentTab)
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
    this.loadMoreClaims()
  },

  async reloadPageData(silent = false) {
    if (!silent) {
      this.setData({ loading: true, initialError: false, initialErrorMessage: '', refreshErrorMessage: '' })
    }

    try {
      const currentTab = this.data.currentTab as ClaimFilterTab
      const [summary, page] = await Promise.all([
        fetchClaimSummary(),
        fetchClaimPage(currentTab, 1)
      ])

      this.setData({
        summary,
        claims: page.claims,
        filteredClaims: page.claims,
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
      this.hydrateClaimSummaries(page.claims)
    } catch (error) {
      logger.error('Load merchant claims failed', error)
      const message = getErrorMessage(error, '商户索赔列表加载失败，请稍后重试')
      if (this.data.initialLoading || !silent) {
        this.setData({
          loading: false,
          loadingMore: false,
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message,
          refreshErrorMessage: '',
          claims: [],
          filteredClaims: [],
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

  async loadClaimList(currentTab: ClaimFilterTab) {
    try {
      const page = await fetchClaimPage(currentTab, 1)
      this.setData({
        claims: page.claims,
        filteredClaims: page.claims,
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
      this.hydrateClaimSummaries(page.claims)
    } catch (error) {
      logger.error('Switch merchant claims tab failed', error)
      const message = getErrorMessage(error, '商户索赔列表加载失败，请稍后重试')
      this.setData({
        loading: false,
        loadingMore: false,
        initialLoading: false,
        initialError: true,
        initialErrorMessage: message,
        refreshErrorMessage: '',
        claims: [],
        filteredClaims: [],
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
      const page = await fetchClaimPage(this.data.currentTab as ClaimFilterTab, nextPage)
      const claims = this.data.filteredClaims.concat(page.claims)
      this.setData({
        claims,
        filteredClaims: claims,
        pageId: page.pageId,
        pageSize: page.pageSize,
        hasMore: page.hasMore,
        loadingMore: false
      })
      this.hydrateClaimSummaries(page.claims)
    } catch (error) {
      logger.error('Load more merchant claims failed', error)
      wx.showToast({
        title: getErrorMessage(error, '加载更多索赔失败，请稍后重试'),
        icon: 'none'
      })
      this.setData({ loadingMore: false })
    }
  },

  async hydrateClaimSummaries(claims: MerchantClaimView[]) {
    const targetIds = claims.slice(0, 8).map((item) => item.id)
    if (!targetIds.length) return

    const merged = await Promise.all(targetIds.map(async (claimId) => {
      try {
        const decision = (await claimManagementService.getMerchantClaimDecision(claimId)).decision
        return {
          claimId,
          responsibilityLabel: formatResponsibleParty(decision?.responsible_party),
          compensationSourceLabel: formatCompensationSource(decision?.compensation_source)
        }
      } catch (error) {
        logger.warn('Hydrate merchant claim decision failed', { claimId, error })
        return null
      }
    }))

    const mergeMap = new Map(merged.filter((item): item is NonNullable<typeof item> => Boolean(item)).map((item) => [item.claimId, item]))
    const nextClaims = this.data.claims.map((item) => {
      const summary = mergeMap.get(item.id)
      if (!summary) return item
      return {
        ...item,
        responsibilityLabel: summary.responsibilityLabel,
        compensationSourceLabel: summary.compensationSourceLabel
      }
    })

    this.setData({ claims: nextClaims, filteredClaims: nextClaims })
  },

  onRetry() {
    this.reloadPageData(false)
  }
})
