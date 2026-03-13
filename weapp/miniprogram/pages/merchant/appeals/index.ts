import { AppealResponse, appealManagementService } from '../../../api/appeals-customer-service'
import { getStableBarHeights } from '../../../utils/responsive'

interface AppealRecordView {
  id: number
  claimId: number
  orderNo: string
  statusLabel: string
  reason: string
  claimTypeLabel: string
  claimAmountText: string
  compensationAmountText?: string
  reviewNotes?: string
  reviewedAt?: string
  createdAt: string
  rawStatus: string
}

type AppealTab = 'all' | 'pending' | 'approved' | 'rejected'

function formatMoney(cents?: number): string {
  if (typeof cents !== 'number') return ''
  return `¥${(cents / 100).toFixed(2)}`
}

function formatAppealStatus(status?: string): string {
  const map: Record<string, string> = {
    pending: '待审核',
    approved: '已通过',
    rejected: '已驳回',
    compensated: '已赔付'
  }
  if (!status) return '-'
  return map[status] || status
}

function formatClaimType(claimType?: string): string {
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
  if (!claimType) return '-'
  return map[claimType] || claimType
}

Page({
  data: {
    navBarHeight: 88,
    loading: true,
    currentTab: 'all' as AppealTab,
    appeals: [] as AppealRecordView[],
    filteredAppeals: [] as AppealRecordView[],
    summary: {
      total: 0,
      pending: 0,
      approved: 0,
      rejected: 0
    }
  },

  onTabChange(e: WechatMiniprogram.CustomEvent<{ value: AppealTab }>) {
    const currentTab = e.detail.value
    this.setData({ currentTab })
    this.applyFilters(this.data.appeals, currentTab)
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.loadAppeals()
  },

  onShow() {
    this.loadAppeals(true)
  },

  onPullDownRefresh() {
    this.loadAppeals()
  },

  async loadAppeals(silent = false) {
    if (!silent) {
      this.setData({ loading: true })
    }

    try {
      const result = await appealManagementService.getMerchantAppeals({
        page_id: 1,
        page_size: 50
      })

      const appeals = (result.appeals || []).map((appeal: AppealResponse): AppealRecordView => ({
        id: appeal.id,
        claimId: appeal.claim_id,
        orderNo: appeal.order_no || `#${appeal.claim_id}`,
        statusLabel: formatAppealStatus(appeal.status),
        reason: appeal.reason,
        claimTypeLabel: formatClaimType(appeal.claim_type),
        claimAmountText: formatMoney(appeal.claim_amount),
        compensationAmountText: typeof appeal.compensation_amount === 'number' ? formatMoney(appeal.compensation_amount) : undefined,
        reviewNotes: appeal.review_notes,
        reviewedAt: appeal.reviewed_at,
        createdAt: appeal.created_at,
        rawStatus: appeal.status
      }))

      this.setData({ appeals, loading: false })
      this.applyFilters(appeals, this.data.currentTab)
    } catch (error) {
      console.error('加载异议记录失败:', error)
      wx.showToast({ title: '加载失败', icon: 'error' })
      this.setData({ loading: false, appeals: [] })
    } finally {
      wx.stopPullDownRefresh()
    }
  },

  onViewClaimDetail(e: WechatMiniprogram.TouchEvent) {
    const { claimId } = e.currentTarget.dataset as { claimId?: number }
    if (!claimId) return
    wx.navigateTo({ url: `/pages/merchant/claims/detail/index?id=${claimId}` })
  },

  applyFilters(appeals: AppealRecordView[], currentTab: AppealTab) {
    const summary = {
      total: appeals.length,
      pending: appeals.filter((item) => item.rawStatus === 'pending').length,
      approved: appeals.filter((item) => item.rawStatus === 'approved').length,
      rejected: appeals.filter((item) => item.rawStatus === 'rejected').length
    }

    const filteredAppeals = appeals.filter((item) => {
      if (currentTab === 'all') return true
      return item.rawStatus === currentTab
    })

    this.setData({ summary, filteredAppeals })
  }
})
