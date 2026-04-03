import {
  claimManagementService,
  ClaimResponse
} from '../../../api/appeals-customer-service'
import { logger } from '../../../utils/logger'
import { getStableBarHeights } from '../../../utils/responsive'
import { getErrorUserMessage } from '../../../utils/user-facing'

type ClaimTagTheme = 'primary' | 'warning' | 'success' | 'danger' | 'default'

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

function formatMoney(cents?: number): string {
  const value = typeof cents === 'number' ? cents : 0
  return `¥${(value / 100).toFixed(2)}`
}

function formatClaimType(claimType: string): string {
  const map: Record<string, string> = {
    refund: '退款',
    compensation: '补偿',
    quality_issue: '质量问题',
    delivery_issue: '配送问题',
    'foreign-object': '异物',
    damage: '餐损',
    timeout: '超时',
    'food-safety': '食安'
  }
  return map[claimType] || claimType
}

function getClaimTypeTheme(claimType: string): ClaimTagTheme {
  if (claimType === 'food-safety' || claimType === 'foreign-object') return 'danger'
  if (claimType === 'damage' || claimType === 'quality_issue') return 'warning'
  return 'primary'
}

function formatClaimStatus(status: string): string {
  const map: Record<string, string> = {
    pending: '待审核',
    approved: '已通过',
    rejected: '已驳回',
    compensated: '已赔付'
  }
  return map[status] || status
}

function getClaimStatusTheme(status: string): ClaimTagTheme {
  if (status === 'approved') return 'warning'
  if (status === 'compensated') return 'success'
  if (status === 'rejected') return 'default'
  return 'primary'
}

function formatAppealStatus(status?: string): string {
  const map: Record<string, string> = {
    pending: '异议待审核',
    approved: '异议已通过',
    rejected: '异议已驳回',
    compensated: '异议已赔付'
  }
  if (!status) return '未提交异议'
  return map[status] || status
}

function getAppealStatusTheme(status?: string): ClaimTagTheme {
  if (!status) return 'default'
  if (status === 'pending') return 'warning'
  if (status === 'approved' || status === 'compensated') return 'success'
  if (status === 'rejected') return 'danger'
  return 'default'
}

function formatTime(value?: string): string {
  if (!value) return '暂无'
  return value.replace('T', ' ').slice(0, 16)
}

function formatResponsibleParty(party?: string): string {
  const map: Record<string, string> = {
    merchant: '商户责任',
    rider: '骑手责任',
    user: '用户责任',
    platform_fallback: '平台兜底',
    unknown: '待判定'
  }
  if (!party) return '责任待拉取'
  return map[party] || party
}

function formatCompensationSource(source?: string): string {
  const map: Record<string, string> = {
    merchant: '商户承担',
    rider: '骑手承担',
    platform: '平台先赔'
  }
  if (!source) return '赔付来源待拉取'
  return map[source] || source
}

function formatRecoveryStatus(status?: string): string {
  const map: Record<string, string> = {
    pending: '待回款',
    overdue: '已逾期',
    paid: '已支付',
    waived: '已核销',
    appealed: '异议中'
  }
  if (!status) return '无追偿单'
  return map[status] || status
}

function getRecoveryStatusTheme(status?: string): ClaimTagTheme {
  if (!status) return 'default'
  if (status === 'pending' || status === 'overdue') return 'warning'
  if (status === 'paid' || status === 'waived') return 'success'
  if (status === 'appealed') return 'danger'
  return 'default'
}

function getActionHint(options: {
  status: string
  appealStatus?: string
  recoveryStatus?: string
}) {
  if (options.recoveryStatus === 'appealed' || options.appealStatus === 'pending') {
    return '异议已提交，等待平台复核结果。'
  }

  if (options.recoveryStatus === 'pending' || options.recoveryStatus === 'overdue' || options.appealStatus === 'rejected') {
    return '平台已生成追偿单，建议尽快支付追偿款或先提交异议。'
  }

  if (
    options.recoveryStatus === 'paid' ||
    options.recoveryStatus === 'waived' ||
    options.appealStatus === 'approved' ||
    options.appealStatus === 'compensated'
  ) {
    return '当前索赔已进入结案态，可进入详情核对最终结果。'
  }

  if (options.status === 'approved' || options.status === 'auto-approved') {
    return '责任已判定，可进入详情查看依据并决定是否提交异议。'
  }

  return '点击查看索赔详情与处理进度。'
}

const getErrorMessage = getErrorUserMessage

function buildBaseClaimView(claim: ClaimResponse): MerchantClaimView {
  const appealStatus = claim.appeal_status
  const recoveryStatus = claim.recovery_status
  const hasAppeal = Boolean(claim.appeal_id)
  const isPendingAction = recoveryStatus === 'pending' || recoveryStatus === 'overdue' || (appealStatus === 'rejected' && recoveryStatus !== 'paid' && recoveryStatus !== 'waived')
  const isAppealedFlow = recoveryStatus === 'appealed' || appealStatus === 'pending'
  const isClosedFlow = recoveryStatus === 'paid' || recoveryStatus === 'waived' || appealStatus === 'approved' || appealStatus === 'compensated'

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
    actionHint: getActionHint({
      status: claim.status,
      appealStatus,
      recoveryStatus
    }),
    hasAppeal,
    isPendingAction,
    isAppealedFlow,
    isClosedFlow
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
