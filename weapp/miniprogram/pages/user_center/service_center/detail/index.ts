import { claimManagementService, formatClaimStatus } from '../../../../api/appeals-customer-service'
import type { ClaimResponse } from '../../../../api/appeals-customer-service'
import { logger } from '../../../../utils/logger'

/** 索赔类型 → 中文显示（涵盖所有 claim_type 值） */
const CLAIM_TYPE_DISPLAY: Record<string, string> = {
  'foreign-object': '异物问题',
  'damage': '餐品损坏',
  'timeout': '配送超时',
  'food-safety': '食品安全',
  'refund': '退款',
  'compensation': '赔偿',
  'quality_issue': '质量问题',
  'delivery_issue': '配送问题'
}

function displayClaimType(type: string): string {
  return CLAIM_TYPE_DISPLAY[type] || type
}

/** 索赔状态 → 图标映射 */
const STATUS_ICON_MAP: Record<string, string> = {
  pending: 'time-filled',
  approved: 'check-circle-filled',
  rejected: 'close-circle-filled',
  compensated: 'check-circle-filled'
}

/** 索赔状态 → 颜色映射 */
const STATUS_COLOR_MAP: Record<string, string> = {
  pending: '#ff9800',
  approved: '#2e7d32',
  rejected: '#d32f2f',
  compensated: '#2e7d32'
}

/** 格式化金额（分 → 元） */
function formatAmount(amount: number): string {
  return (amount / 100).toFixed(2)
}

/** 格式化时间 */
function formatDateTime(dateStr: string): string {
  const d = new Date(dateStr)
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  const h = String(d.getHours()).padStart(2, '0')
  const min = String(d.getMinutes()).padStart(2, '0')
  return `${y}-${m}-${day} ${h}:${min}`
}

interface DisplayClaimDetail {
  id: number
  statusText: string
  claimTypeText: string
  claimAmountDisplay: string
  approvedAmountDisplay: string | null
  description: string
  orderId: number
  createTimeDisplay: string
  reviewedAtDisplay: string | null
  reviewNotes: string | null
}

function adaptClaimDetail(c: ClaimResponse): DisplayClaimDetail {
  return {
    id: c.id,
    statusText: formatClaimStatus(c.status),
    claimTypeText: displayClaimType(c.claim_type),

    claimAmountDisplay: formatAmount(c.claim_amount),
    approvedAmountDisplay:
      c.approved_amount !== null && c.approved_amount !== undefined
        ? formatAmount(c.approved_amount)
        : null,
    description: c.description,
    orderId: c.order_id,
    createTimeDisplay: formatDateTime(c.created_at),
    reviewedAtDisplay: c.reviewed_at ? formatDateTime(c.reviewed_at) : null,
    reviewNotes: c.review_notes || null
  }
}

Page({
  data: {
    navBarHeight: 88,
    loading: true,
    isError: false,
    errorMsg: '',
    claim: null as DisplayClaimDetail | null,
    statusIcon: '',
    statusColor: '',
    claimId: 0
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent<{ navBarHeight: number }>) {
    if (e.detail.navBarHeight !== null && e.detail.navBarHeight !== undefined) {
      this.setData({ navBarHeight: e.detail.navBarHeight })
    }
  },

  onLoad(options: { id?: string }) {
    if (!options.id) {
      this.setData({ isError: true, errorMsg: '缺少工单ID', loading: false })
      return
    }
    this.setData({ claimId: parseInt(options.id) })
    this.loadDetail()
  },

  async loadDetail() {
    this.setData({ loading: true, isError: false })
    try {
      const claim = await claimManagementService.getClaimDetail(this.data.claimId)
      const displayClaim = adaptClaimDetail(claim)

      this.setData({
        claim: displayClaim,
        statusIcon: STATUS_ICON_MAP[claim.status] || 'info-circle-filled',
        statusColor: STATUS_COLOR_MAP[claim.status] || '#999',
        loading: false
      })
    } catch (err) {
      logger.error('[ClaimDetail] loadDetail failed', err)
      this.setData({
        loading: false,
        isError: true,
        errorMsg: '加载失败，请稍后重试'
      })
    }
  }
})
