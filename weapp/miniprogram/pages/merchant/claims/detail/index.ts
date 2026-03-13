import {
  AppealResponse,
  appealManagementService,
  claimManagementService,
  ClaimRecoveryResponse,
  MerchantClaimDecisionResponse,
  validateAppealReason
} from '../../../../api/appeals-customer-service'
import { getStableBarHeights } from '../../../../utils/responsive'

interface ClaimDetailOptions {
  id?: string
}

interface MerchantClaimDetailView {
  orderNo: string
  claimTypeLabel: string
  statusLabel: string
  claimAmountText: string
  approvedAmountText: string
  createdAt: string
  description: string
  responsiblePartyLabel: string
  compensationSourceLabel: string
  reasonCodesText: string
  traceSummary?: string
  recoveryStatusLabel: string
  recoveryAmountText?: string
  dueAt?: string
  appealStatusLabel: string
  reviewNotes?: string
  hasAppeal: boolean
  canPayRecovery: boolean
  progressCurrent: number
  progressClaimText: string
  progressRecoveryText: string
  progressAppealText: string
}

function formatMoney(cents?: number): string {
  const value = typeof cents === 'number' ? cents : 0
  return `¥${(value / 100).toFixed(2)}`
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

function formatClaimStatus(status?: string): string {
  const map: Record<string, string> = {
    pending: '待审核',
    approved: '已通过',
    rejected: '已驳回',
    compensated: '已赔付'
  }
  if (!status) return '-'
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

function formatResponsibleParty(party?: string): string {
  const map: Record<string, string> = {
    merchant: '商户责任',
    rider: '骑手责任',
    user: '用户责任',
    platform_fallback: '平台兜底',
    unknown: '待判定'
  }
  if (!party) return '待判定'
  return map[party] || party
}

function formatCompensationSource(source?: string): string {
  const map: Record<string, string> = {
    merchant: '商户承担',
    rider: '骑手承担',
    platform: '平台先赔'
  }
  if (!source) return '未知来源'
  return map[source] || source
}

Page({
  data: {
    navBarHeight: 88,
    claimId: 0,
    loading: true,
    submitting: false,
    recoveryPaying: false,
    detail: null as MerchantClaimDetailView | null,
    appealReason: ''
  },

  onLoad(options: ClaimDetailOptions) {
    const { navBarHeight } = getStableBarHeights()
    const claimId = Number(options.id || 0)
    this.setData({ navBarHeight, claimId })
    if (!claimId) {
      this.setData({ loading: false })
      wx.showToast({ title: '缺少索赔ID', icon: 'none' })
      return
    }
    this.loadDetail()
  },

  onShow() {
    if (this.data.claimId) {
      this.loadDetail(true)
    }
  },

  onPullDownRefresh() {
    this.loadDetail()
  },

  async loadDetail(silent = false) {
    if (!silent) {
      this.setData({ loading: true })
    }

    try {
      const claim = await claimManagementService.getMerchantClaimDetail(this.data.claimId)
      let decision: MerchantClaimDecisionResponse['decision'] = null
      try {
        const decisionResult = await claimManagementService.getMerchantClaimDecision(this.data.claimId)
        decision = decisionResult.decision
      } catch (_error) {
        decision = null
      }

      let recovery: ClaimRecoveryResponse | null = null
      try {
        recovery = await claimManagementService.getMerchantClaimRecovery(this.data.claimId)
      } catch (_error) {
        recovery = null
      }

      let appeal: AppealResponse | null = null
      if (claim.appeal_id) {
        try {
          appeal = await appealManagementService.getMerchantAppealDetail(claim.appeal_id)
        } catch (_error) {
          appeal = null
        }
      }

      const detail: MerchantClaimDetailView = {
        orderNo: claim.order_no || String(claim.order_id),
        claimTypeLabel: formatClaimType(claim.claim_type),
        statusLabel: formatClaimStatus(claim.status),
        claimAmountText: formatMoney(claim.claim_amount),
        approvedAmountText: formatMoney(claim.approved_amount || claim.claim_amount),
        createdAt: claim.created_at,
        description: claim.description,
        responsiblePartyLabel: formatResponsibleParty(decision?.responsible_party),
        compensationSourceLabel: formatCompensationSource(decision?.compensation_source),
        reasonCodesText: decision?.reason_codes?.join('、') || '无',
        traceSummary: decision?.trace_summary,
        recoveryStatusLabel: formatRecoveryStatus(recovery?.status),
        recoveryAmountText: recovery ? formatMoney(recovery.recovery_amount) : undefined,
        dueAt: recovery?.due_at,
        appealStatusLabel: formatAppealStatus(claim.appeal_status),
        reviewNotes: appeal?.review_notes,
        hasAppeal: Boolean(claim.appeal_id),
        canPayRecovery: Boolean(recovery && (recovery.status === 'pending' || recovery.status === 'overdue')),
        progressCurrent: this.getProgressCurrent(claim.status, recovery?.status, Boolean(claim.appeal_id)),
        progressClaimText: claim.created_at,
        progressRecoveryText: recovery?.due_at || '等待平台生成或无需追偿',
        progressAppealText: appeal?.reviewed_at || appeal?.created_at || '未提交异议'
      }

      this.setData({
        detail,
        loading: false,
        appealReason: claim.appeal_reason || ''
      })
    } catch (error) {
      console.error('加载索赔详情失败:', error)
      wx.showToast({ title: '加载失败', icon: 'error' })
      this.setData({ loading: false })
    } finally {
      wx.stopPullDownRefresh()
    }
  },

  onAppealInput(e: WechatMiniprogram.CustomEvent) {
    this.setData({ appealReason: String(e.detail.value || '') })
  },

  async onSubmitAppeal() {
    const reason = this.data.appealReason.trim()
    const validation = validateAppealReason(reason)
    if (!validation.valid) {
      wx.showToast({ title: validation.message || '请输入有效异议理由', icon: 'none' })
      return
    }

    try {
      this.setData({ submitting: true })
      await appealManagementService.createAppeal({
        claim_id: this.data.claimId,
        reason
      })
      wx.showToast({ title: '异议已提交', icon: 'success' })
      await this.loadDetail(true)
    } catch (error) {
      console.error('提交异议失败:', error)
      wx.showToast({ title: '提交失败', icon: 'error' })
    } finally {
      this.setData({ submitting: false })
    }
  },

  async onPayRecovery() {
    if (!this.data.claimId) return

    try {
      this.setData({ recoveryPaying: true })
      await claimManagementService.payMerchantClaimRecovery(this.data.claimId)
      wx.showToast({ title: '回款已确认', icon: 'success' })
      await this.loadDetail(true)
    } catch (error) {
      console.error('确认回款失败:', error)
      wx.showToast({ title: '操作失败', icon: 'error' })
    } finally {
      this.setData({ recoveryPaying: false })
    }
  },

  onViewAppeals() {
    wx.navigateTo({ url: '/pages/merchant/appeals/index' })
  },

  getProgressCurrent(claimStatus?: string, recoveryStatus?: string, hasAppeal?: boolean) {
    if (hasAppeal) {
      return recoveryStatus === 'waived' ? 3 : 2
    }
    if (recoveryStatus === 'paid' || recoveryStatus === 'waived' || claimStatus === 'rejected') {
      return 3
    }
    if (recoveryStatus === 'pending' || recoveryStatus === 'overdue' || claimStatus === 'approved') {
      return 1
    }
    return 0
  }
})
