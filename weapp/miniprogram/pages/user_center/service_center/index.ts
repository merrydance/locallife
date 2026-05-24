import { claimManagementService, getUserClaimPresentation } from '../../../api/appeals-customer-service'
import type { UserClaimResponse, UserClaimType } from '../../../api/appeals-customer-service'
import { logger } from '../../../utils/logger'
import Navigation from '../../../utils/navigation'

const PAGE_SIZE = 20

/** 索赔类型 → 中文显示（涵盖后端所有 claim_type 值） */
const CLAIM_TYPE_DISPLAY: Record<string, string> = {
  'foreign-object': '异物问题',
  'damage': '餐品损坏',
  'timeout': '代取超时',
  'food-safety': '食品安全',
  'refund': '退款',
  'compensation': '赔偿',
  'quality_issue': '质量问题',
  'delivery_issue': '代取问题'
}

function displayClaimType(type: string): string {
  return CLAIM_TYPE_DISPLAY[type] || type
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
  statusSummary: string
  claimTypeText: string
  claimAmountDisplay: string
  approvedAmountDisplay: string | null
  description: string
  createTimeDisplay: string
  payoutEta: string | null
}

function adaptClaim(c: UserClaimResponse): DisplayClaim {
  const presentation = getUserClaimPresentation(c)
  return {
    id: c.id,
    statusText: presentation.statusText,
    statusTheme: presentation.statusTheme,
    statusSummary: presentation.summary,
    claimTypeText: displayClaimType(c.claim_type),
    claimAmountDisplay: formatAmount(c.claim_amount),
    approvedAmountDisplay:
      c.approved_amount !== null && c.approved_amount !== undefined
        ? formatAmount(c.approved_amount)
        : null,
    description: c.description,
    createTimeDisplay: formatTime(c.created_at),
    payoutEta: c.payout_eta || null
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

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight: number }>) {
    if (e.detail.navBarHeight !== null && e.detail.navBarHeight !== undefined) {
      this.setData({ navBarHeight: e.detail.navBarHeight })
    }
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
        page: this.data.currentPage,
        page_size: PAGE_SIZE
      })
      const claimsArr = Array.isArray(result.claims) ? result.claims : []
      const displayClaims = claimsArr.map(adaptClaim)
      const existingClaims = Array.isArray(this.data.claims) ? this.data.claims : []

      this.setData({
        claims: this.data.currentPage === 1
          ? displayClaims
          : existingClaims.concat(displayClaims),
        hasMore: result.page * result.page_size < result.total,
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
    const claimType = e.currentTarget.dataset.type as string
    if (claimType === 'food-safety-report') {
      wx.navigateTo({
        url: '/pages/orders/list/index?status=completed&selectMode=1',
        events: {
          onOrderSelected: (order: { id: number }) => {
            setTimeout(() => {
              Navigation.toFoodSafetyReport({ orderId: order.id })
            }, 100)
          }
        }
      })
      return
    }

    wx.navigateTo({
      url: `/pages/user_center/service_center/submit/index?claimType=${claimType as UserClaimType}`
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
