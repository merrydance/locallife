import {
  claimManagementService,
  ClaimResponse
} from '../../../api/appeals-customer-service'
import { getStableBarHeights } from '../../../utils/responsive'

interface MerchantClaimView {
  id: number
  orderNo: string
  claimTypeLabel: string
  claimAmountText: string
  approvedAmountText: string
  statusLabel: string
  description: string
  appealStatusLabel: string
  hasAppeal: boolean
  isPendingAction: boolean
  isAppealedFlow: boolean
  isClosedFlow: boolean
}

type ClaimFilterTab = 'all' | 'pending_action' | 'appealed' | 'closed'

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

function formatClaimStatus(status: string): string {
  const map: Record<string, string> = {
    pending: '待审核',
    approved: '已通过',
    rejected: '已驳回',
    compensated: '已赔付'
  }
  return map[status] || status
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


Page({
  data: {
    navBarHeight: 88,
    loading: false,
    currentTab: 'all' as ClaimFilterTab,
    claims: [] as MerchantClaimView[],
    filteredClaims: [] as MerchantClaimView[],
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
    this.setData({ currentTab })
    this.applyFilters(this.data.claims, currentTab)
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.loadClaims()
  },

  onShow() {
    this.loadClaims()
  },

  onPullDownRefresh() {
    this.loadClaims()
  },

  async loadClaims() {
    this.setData({ loading: true })
    try {
      const result = await claimManagementService.getMerchantClaims({
        page_id: 1,
        page_size: 30
      })

      const claims = result.claims.map((claim: ClaimResponse): MerchantClaimView => {
        const hasAppeal = Boolean(claim.appeal_id)
        const isPendingAction = claim.status === 'approved' && !hasAppeal
        const isAppealedFlow = hasAppeal
        const isClosedFlow = claim.status === 'rejected' || claim.status === 'compensated'

        return {
          id: claim.id,
          orderNo: claim.order_no || `#${claim.order_id}`,
          claimTypeLabel: formatClaimType(claim.claim_type),
          claimAmountText: formatMoney(claim.claim_amount),
          approvedAmountText: formatMoney(claim.approved_amount || claim.claim_amount),
          statusLabel: formatClaimStatus(claim.status),
          description: claim.description,
          appealStatusLabel: formatAppealStatus(claim.appeal_status),
          hasAppeal,
          isPendingAction,
          isAppealedFlow,
          isClosedFlow
        }
      })

      this.setData({ claims, loading: false })
      this.applyFilters(claims, this.data.currentTab)
    } catch (error) {
      console.error('加载商户索赔列表失败:', error)
      wx.showToast({ title: '加载失败', icon: 'error' })
      this.setData({ loading: false, claims: [] })
    } finally {
      wx.stopPullDownRefresh()
    }
  },

  applyFilters(claims: MerchantClaimView[], currentTab: ClaimFilterTab) {
    const summary = {
      total: claims.length,
      pendingAction: claims.filter((item) => item.isPendingAction).length,
      appealed: claims.filter((item) => item.isAppealedFlow).length,
      closed: claims.filter((item) => item.isClosedFlow).length
    }

    const filteredClaims = claims.filter((item) => {
      if (currentTab === 'pending_action') return item.isPendingAction
      if (currentTab === 'appealed') return item.isAppealedFlow
      if (currentTab === 'closed') return item.isClosedFlow
      return true
    })

    this.setData({ summary, filteredClaims })
  }
})
