import { claimManagementService, formatClaimStatus, formatClaimType } from '../../../api/appeals-customer-service'
import type { ClaimResponse, UserClaimType } from '../../../api/appeals-customer-service'
import { logger } from '../../../utils/logger'

const PAGE_SIZE = 20

/** 索赔状态 → TDesign Tag Theme 映射 */
const STATUS_THEME_MAP: Record<string, string> = {
  pending: 'warning',
  approved: 'success',
  rejected: 'danger',
  compensated: 'success'
}

/** 格式化金额（分 → 元） */
function formatAmount(amount: number): string {
  return (amount / 100).toFixed(2)
}

/** 格式化时间 */
function formatTime(dateStr: string): string {
  const d = new Date(dateStr)
  const month = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  const hour = String(d.getHours()).padStart(2, '0')
  const minute = String(d.getMinutes()).padStart(2, '0')
  return `${month}-${day} ${hour}:${minute}`
}

interface DisplayClaim {
  id: number
  statusText: string
  statusTheme: string
  claimTypeText: string
  claimAmountDisplay: string
  approvedAmountDisplay: string | null
  description: string
  createTimeDisplay: string
}

function adaptClaim(c: ClaimResponse): DisplayClaim {
  return {
    id: c.id,
    statusText: formatClaimStatus(c.status),
    statusTheme: STATUS_THEME_MAP[c.status] || 'default',
    claimTypeText: formatClaimType(c.claim_type),
    claimAmountDisplay: formatAmount(c.claim_amount),
    approvedAmountDisplay: c.approved_amount != null ? formatAmount(c.approved_amount) : null,
    description: c.description,
    createTimeDisplay: formatTime(c.created_at)
  }
}

Page({
  data: {
    navBarHeight: 88,
    loading: true,
    claims: [] as DisplayClaim[],
    hasMore: false,
    currentPage: 1
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ height: number }>) {
    this.setData({ navBarHeight: e.detail.height })
  },

  onLoad() {
    this.loadClaims()
  },

  onPullDownRefresh() {
    this.setData({ currentPage: 1 })
    this.loadClaims().then(() => wx.stopPullDownRefresh())
  },

  async loadClaims() {
    this.setData({ loading: true })
    try {
      const result = await claimManagementService.getUserClaims({
        page_id: this.data.currentPage,
        page_size: PAGE_SIZE
      })
      const displayClaims = (result.claims || []).map(adaptClaim)

      this.setData({
        claims: this.data.currentPage === 1
          ? displayClaims
          : [...this.data.claims, ...displayClaims],
        hasMore: displayClaims.length >= PAGE_SIZE,
        loading: false
      })
    } catch (err) {
      logger.error('[ServiceCenter] loadClaims failed', err)
      this.setData({ loading: false })
      wx.showToast({ title: '加载失败', icon: 'none' })
    }
  },

  loadMore() {
    if (!this.data.hasMore) return
    this.setData({ currentPage: this.data.currentPage + 1 })
    this.loadClaims()
  },

  /** 快捷入口点击 */
  onQuickEntry(e: WechatMiniprogram.BaseEvent) {
    const claimType = e.currentTarget.dataset.type as UserClaimType
    wx.navigateTo({
      url: `/pages/user_center/service_center/submit/index?claimType=${claimType}`
    })
  },

  /** 工单详情 */
  onClaimDetail(e: WechatMiniprogram.BaseEvent) {
    const claimId = e.currentTarget.dataset.id as number
    wx.navigateTo({
      url: `/pages/user_center/service_center/detail/index?id=${claimId}`
    })
  }
})
