import {
  appealManagementService,
  claimManagementService,
  AppealResponse,
  ClaimResponse
} from '../../../api/appeals-customer-service'
import { logger } from '../../../utils/logger'
import { getStableBarHeights } from '../../../utils/responsive'
import { getErrorUserMessage } from '../../../utils/user-facing'

type ClaimTagTheme = 'primary' | 'warning' | 'success' | 'danger' | 'default'
type ClaimBucketTab = 'all' | 'pending_action' | 'appealed' | 'closed'
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
    compensated: '已赔付',
    'auto-approved': '已通过'
  }
  return map[status] || status
}

function getClaimStatusTheme(status: string): ClaimTagTheme {
  if (status === 'approved' || status === 'auto-approved') return 'warning'
  if (status === 'compensated') return 'success'
  if (status === 'rejected') return 'default'
  return 'primary'
}

function formatAppealStatus(status?: string): string {
  const map: Record<string, string> = {
    pending: '申诉处理中',
    approved: '申诉通过',
    rejected: '申诉驳回',
    compensated: '申诉已赔付'
  }
  if (!status) return '未提交申诉'
  return map[status] || status
}

function getAppealStatusTheme(status?: string): ClaimTagTheme {
  if (!status) return 'default'
  if (status === 'pending') return 'warning'
  if (status === 'approved' || status === 'compensated') return 'success'
  if (status === 'rejected') return 'danger'
  return 'default'
}

function formatRecoveryStatus(status?: string): string {
  const map: Record<string, string> = {
    pending: '待支付追偿',
    overdue: '追偿已逾期',
    paid: '追偿已支付',
    waived: '追偿已豁免',
    appealed: '追偿申诉中'
  }
  if (!status) return '暂无追偿'
  return map[status] || status
}

function getRecoveryStatusTheme(status?: string): ClaimTagTheme {
  if (!status) return 'default'
  if (status === 'pending' || status === 'overdue') return 'warning'
  if (status === 'paid' || status === 'waived') return 'success'
  if (status === 'appealed') return 'danger'
  return 'default'
}

function formatTime(value?: string): string {
  if (!value) return '暂无'
  return value.replace('T', ' ').slice(0, 16)
}

function getActionHint(claim: ClaimResponse): string {
  const appealStatus = claim.appeal_status
  const recoveryStatus = claim.recovery_status

  if (recoveryStatus === 'appealed' || appealStatus === 'pending') {
    return '平台正在复核申诉，进入详情可查看最新处理进度。'
  }

  if (recoveryStatus === 'pending' || recoveryStatus === 'overdue') {
    return '当前有待处理追偿，进入详情可支付追偿款或提交申诉。'
  }

  if (appealStatus === 'rejected') {
    return '申诉已驳回，请进入详情核对复核说明和追偿状态。'
  }

  if (appealStatus === 'approved' || appealStatus === 'compensated' || recoveryStatus === 'paid' || recoveryStatus === 'waived') {
    return '这笔索赔已进入结案阶段，进入详情可查看最终结果。'
  }

  return '进入详情查看责任判定，并决定是否申诉或支付追偿。'
}

const getErrorMessage = getErrorUserMessage

function buildClaimView(claim: ClaimResponse): RiderClaimView {
  return {
    id: claim.id,
    orderNo: claim.order_no || `#${claim.order_id}`,
    claimTypeLabel: formatClaimType(claim.claim_type),
    claimTypeTheme: getClaimTypeTheme(claim.claim_type),
    claimAmountText: formatMoney(claim.claim_amount),
    approvedAmountText: formatMoney(claim.approved_amount || claim.claim_amount),
    statusLabel: formatClaimStatus(String(claim.status)),
    statusTheme: getClaimStatusTheme(String(claim.status)),
    description: claim.description,
    appealStatusLabel: formatAppealStatus(claim.appeal_status),
    appealStatusTheme: getAppealStatusTheme(claim.appeal_status),
    recoveryStatusLabel: formatRecoveryStatus(claim.recovery_status),
    recoveryStatusTheme: getRecoveryStatusTheme(claim.recovery_status),
    createdAtLabel: formatTime(claim.created_at),
    actionHint: getActionHint(claim)
  }
}

function buildAppealView(appeal: AppealResponse): RiderAppealView {
  return {
    id: appeal.id,
    claimId: appeal.claim_id,
    orderNo: appeal.order_no || `#${appeal.claim_id}`,
    claimTypeLabel: formatClaimType(appeal.claim_type || 'compensation'),
    statusLabel: formatAppealStatus(appeal.status),
    statusTheme: getAppealStatusTheme(appeal.status),
    reason: appeal.reason,
    createdAtLabel: formatTime(appeal.created_at),
    reviewNotes: appeal.review_notes
  }
}

function toBucket(tab: ClaimBucketTab): 'pending_action' | 'appealed' | 'closed' | undefined {
  return tab === 'all' ? undefined : tab
}

async function fetchClaimSummary(): Promise<ClaimSummary> {
  const summary = await claimManagementService.getRiderClaimsSummary()

  return {
    total: summary.total || 0,
    pendingAction: summary.pending_action || 0,
    appealed: summary.appealed || 0,
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
      appealed: 0,
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
            appealed: 0,
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