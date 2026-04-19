import dayjs from 'dayjs'
import { AppealResponse, appealManagementService } from '../../../../api/appeals-customer-service'
import { logger } from '../../../../utils/logger'
import {
  getMerchantAppealProgressCurrent,
  getMerchantAppealResultSummary,
  getMerchantAppealStatusView,
  MerchantAppealTagTheme
} from '../../../../utils/merchant-appeal-view'
import { getStableBarHeights } from '../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../utils/user-facing'

interface AppealDetailOptions {
  id?: string
}

type AppealTagTheme = MerchantAppealTagTheme

interface AppealDetailView {
  id: number
  claimId: number
  orderNo: string
  orderAmountText: string
  claimTypeLabel: string
  claimAmountText: string
  approvedAmountText?: string
  hasApprovedAmount: boolean
  compensationAmountText: string
  statusLabel: string
  statusTheme: AppealTagTheme
  appellantTypeLabel: string
  userPhone?: string
  hasUserPhone: boolean
  reason: string
  claimDescription: string
  reviewNotes: string
  hasReviewNotes: boolean
  hasCompensation: boolean
  createdAtLabel: string
  reviewedAtLabel: string
  compensatedAtLabel: string
  hasCompensatedAt: boolean
  resultTitle: string
  resultDescription: string
  progressCurrent: number
}

function formatMoney(cents?: number) {
  const value = typeof cents === 'number' ? cents : 0
  return `¥${(value / 100).toFixed(2)}`
}

function formatTime(value?: string) {
  if (!value) return '暂无'
  const parsed = dayjs(value)
  return parsed.isValid() ? parsed.format('YYYY-MM-DD HH:mm') : value
}

function formatClaimType(claimType?: string) {
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

function formatAppellantType(appellantType?: string) {
  const map: Record<string, string> = {
    merchant: '商户发起',
    rider: '骑手发起',
    user: '用户发起'
  }
  if (!appellantType) return '申诉发起方'
  return map[appellantType] || appellantType
}

const getErrorMessage = getErrorUserMessage

function toAppealDetailView(appeal: AppealResponse): AppealDetailView {
  const statusView = getMerchantAppealStatusView(appeal.status, '-')
  const result = getMerchantAppealResultSummary(appeal.status)

  return {
    id: appeal.id,
    claimId: appeal.claim_id,
    orderNo: appeal.order_no || `#${appeal.claim_id}`,
    orderAmountText: formatMoney(appeal.order_amount),
    claimTypeLabel: formatClaimType(appeal.claim_type),
    claimAmountText: formatMoney(appeal.claim_amount),
    approvedAmountText: typeof appeal.claim_approved_amount === 'number' ? formatMoney(appeal.claim_approved_amount) : undefined,
    hasApprovedAmount: typeof appeal.claim_approved_amount === 'number',
    compensationAmountText: formatMoney(appeal.compensation_amount),
    statusLabel: statusView.label,
    statusTheme: statusView.theme,
    appellantTypeLabel: formatAppellantType(appeal.appellant_type),
    userPhone: appeal.user_phone || undefined,
    hasUserPhone: Boolean(appeal.user_phone),
    reason: appeal.reason || '暂无异议理由',
    claimDescription: appeal.claim_description || '暂无索赔说明',
    reviewNotes: appeal.review_notes || '',
    hasReviewNotes: Boolean(appeal.review_notes),
    hasCompensation: typeof appeal.compensation_amount === 'number',
    createdAtLabel: formatTime(appeal.created_at),
    reviewedAtLabel: formatTime(appeal.reviewed_at),
    compensatedAtLabel: formatTime(appeal.compensated_at),
    hasCompensatedAt: Boolean(appeal.compensated_at),
    resultTitle: result.title,
    resultDescription: result.description,
    progressCurrent: getMerchantAppealProgressCurrent(appeal.status)
  }
}

Page({
  data: {
    navBarHeight: 88,
    appealId: 0,
    loading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    detail: null as AppealDetailView | null
  },

  onLoad(options: AppealDetailOptions) {
    const { navBarHeight } = getStableBarHeights()
    const appealId = Number(options.id || 0)
    this.setData({ navBarHeight, appealId })

    if (!appealId) {
      this.setData({
        loading: false,
        initialError: true,
        initialErrorMessage: '缺少异议 ID，无法查看详情'
      })
      return
    }

    this.loadDetail()
  },

  onShow() {
    if (this.data.appealId && this.data.detail && !this.data.loading) {
      this.loadDetail(true)
    }
  },

  onPullDownRefresh() {
    this.loadDetail(Boolean(this.data.detail))
  },

  onRetry() {
    this.loadDetail(false)
  },

  onRetryRefresh() {
    this.loadDetail(true)
  },

  onViewClaimDetail() {
    const claimId = this.data.detail?.claimId
    if (!claimId) return
    wx.navigateTo({ url: `/pages/merchant/claims/detail/index?id=${claimId}` })
  },

  async loadDetail(silent = false) {
    if (!silent) {
      this.setData({
        loading: true,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: ''
      })
    } else {
      this.setData({ refreshErrorMessage: '' })
    }

    try {
      const appeal = await appealManagementService.getMerchantAppealDetail(this.data.appealId)
      this.setData({
        detail: toAppealDetailView(appeal),
        loading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: ''
      })
    } catch (err) {
      logger.error('Load merchant appeal detail failed', err)
      const message = getErrorMessage(err, '异议详情加载失败，请稍后重试')
      if (!this.data.detail || !silent) {
        this.setData({
          loading: false,
          initialError: true,
          initialErrorMessage: message,
          refreshErrorMessage: '',
          detail: null
        })
      } else {
        this.setData({
          loading: false,
          refreshErrorMessage: `${message}，当前已保留上次同步结果`
        })
      }
    } finally {
      wx.stopPullDownRefresh()
    }
  }
})