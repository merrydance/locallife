import dayjs from 'dayjs'
import {
  AppealResponse,
  appealManagementService,
  BehaviorSummaryStat,
  claimManagementService,
  ClaimRecoveryResponse,
  ClaimRecoveryPaymentResponse,
  MerchantClaimBehaviorSummaryResponse,
  MerchantClaimDecisionResponse,
  validateAppealReason
} from '../../../../api/appeals-customer-service'
import { invokeWechatPay, pollPaymentStatus } from '../../../../api/payment'
import { logger } from '../../../../utils/logger'
import { settleAll } from '../../../../utils/promise'
import { getStableBarHeights } from '../../../../utils/responsive'

interface ClaimDetailOptions {
  id?: string
}

interface RiderClaimDetailView {
  appealId?: number
  orderId: number
  orderNo: string
  claimTypeLabel: string
  statusLabel: string
  claimAmountText: string
  approvedAmountText: string
  createdAtLabel: string
  description: string
  responsiblePartyLabel: string
  compensationSourceLabel: string
  reasonCodesText: string
  traceSummary?: string
  recoveryStatusLabel: string
  recoveryAmountText?: string
  dueAtLabel?: string
  appealStatusLabel: string
  appealReasonText?: string
  reviewNotes?: string
  reviewedAtLabel?: string
  hasAppeal: boolean
  canSubmitAppeal: boolean
  canPayRecovery: boolean
  progressCurrent: number
  progressClaimText: string
  progressRecoveryText: string
  progressAppealText: string
}

interface BehaviorSummaryCardView {
  key: 'user' | 'merchant' | 'rider'
  title: string
  entityId: number
  totalOrders: number
  abnormalClaims: number
  abnormalRateText: string
  abnormalRateLevel: 'high' | 'medium' | 'low'
  hint: string
}

function formatMoney(cents?: number): string {
  const value = typeof cents === 'number' ? cents : 0
  return `¥${(value / 100).toFixed(2)}`
}

function formatTime(value?: string): string {
  if (!value) return '暂无'
  const parsed = dayjs(value)
  return parsed.isValid() ? parsed.format('YYYY-MM-DD HH:mm') : value
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
    compensated: '已赔付',
    'auto-approved': '已通过'
  }
  if (!status) return '-'
  return map[status] || status
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

function formatRecoveryStatus(status?: string): string {
  const map: Record<string, string> = {
    pending: '待支付追偿',
    overdue: '追偿已逾期',
    paid: '追偿已支付',
    waived: '追偿已豁免',
    appealed: '追偿申诉中'
  }
  if (!status) return '无追偿单'
  return map[status] || status
}

function formatResponsibleParty(party?: string): string {
  const map: Record<string, string> = {
    merchant: '商户责任',
    rider: '骑手责任',
    user: '用户责任',
    shared: '多方分摊',
    platform: '平台承担',
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
    platform: '平台承担',
    shared: '多方分摊'
  }
  if (!source) return '未知来源'
  return map[source] || source
}

function getErrorMessage(err: unknown, fallback: string) {
  if (typeof err === 'object' && err !== null && 'userMessage' in err) {
    const userMessage = (err as { userMessage?: unknown }).userMessage
    if (typeof userMessage === 'string' && userMessage.trim()) {
      return userMessage
    }
  }
  return fallback
}

function isClaimRecoveryNotFoundError(err: unknown) {
  const candidates: string[] = []

  if (typeof err === 'object' && err !== null) {
    const knownError = err as { userMessage?: unknown, message?: unknown, originalError?: { message?: unknown } }
    if (typeof knownError.userMessage === 'string') candidates.push(knownError.userMessage)
    if (typeof knownError.message === 'string') candidates.push(knownError.message)
    if (typeof knownError.originalError?.message === 'string') candidates.push(knownError.originalError.message)
  } else if (typeof err === 'string') {
    candidates.push(err)
  }

  return candidates.some((text) => {
    const normalized = text.toLowerCase()
    return normalized.includes('claim recovery not found') || normalized.includes('追偿单不存在')
  })
}

function formatRate(value?: number) {
  const rate = typeof value === 'number' ? value : 0
  return `${(rate * 100).toFixed(1)}%`
}

function getBehaviorRateLevel(rate?: number): 'high' | 'medium' | 'low' {
  const value = typeof rate === 'number' ? rate : 0
  if (value >= 0.1) return 'high'
  if (value >= 0.03) return 'medium'
  return 'low'
}

function getBehaviorHint(entityType: 'user' | 'merchant' | 'rider', summary: BehaviorSummaryStat) {
  const roleLabelMap: Record<'user' | 'merchant' | 'rider', string> = {
    user: '用户',
    merchant: '商户',
    rider: '骑手'
  }

  if (!summary.total_orders) {
    return `${roleLabelMap[entityType]}在统计窗口内暂无有效履约订单。`
  }

  if (!summary.abnormal_claims) {
    return `${roleLabelMap[entityType]}近窗内未出现异常索赔。`
  }

  return `${roleLabelMap[entityType]}近窗内有 ${summary.abnormal_claims} 笔异常索赔，可结合责任判定交叉核对。`
}

function buildBehaviorCards(summary: MerchantClaimBehaviorSummaryResponse): BehaviorSummaryCardView[] {
  const cards: BehaviorSummaryCardView[] = [
    {
      key: 'user',
      title: '用户',
      entityId: summary.user.entity_id,
      totalOrders: summary.user.total_orders,
      abnormalClaims: summary.user.abnormal_claims,
      abnormalRateText: formatRate(summary.user.abnormal_rate),
      abnormalRateLevel: getBehaviorRateLevel(summary.user.abnormal_rate),
      hint: getBehaviorHint('user', summary.user)
    },
    {
      key: 'merchant',
      title: '商户',
      entityId: summary.merchant.entity_id,
      totalOrders: summary.merchant.total_orders,
      abnormalClaims: summary.merchant.abnormal_claims,
      abnormalRateText: formatRate(summary.merchant.abnormal_rate),
      abnormalRateLevel: getBehaviorRateLevel(summary.merchant.abnormal_rate),
      hint: getBehaviorHint('merchant', summary.merchant)
    }
  ]

  if (summary.rider) {
    cards.push({
      key: 'rider',
      title: '骑手',
      entityId: summary.rider.entity_id,
      totalOrders: summary.rider.total_orders,
      abnormalClaims: summary.rider.abnormal_claims,
      abnormalRateText: formatRate(summary.rider.abnormal_rate),
      abnormalRateLevel: getBehaviorRateLevel(summary.rider.abnormal_rate),
      hint: getBehaviorHint('rider', summary.rider)
    })
  }

  return cards
}

Page({
  data: {
    navBarHeight: 88,
    claimId: 0,
    loading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    actionNoticeMessage: '',
    submitting: false,
    recoveryPaying: false,
    decisionLoading: false,
    decisionError: false,
    decisionErrorMessage: '',
    recoveryLoading: false,
    recoveryError: false,
    recoveryErrorMessage: '',
    behaviorLoading: true,
    behaviorError: false,
    behaviorErrorMessage: '',
    behaviorWindowLabel: '近30日',
    behaviorCards: [] as BehaviorSummaryCardView[],
    hasRiderBehavior: false,
    detail: null as RiderClaimDetailView | null,
    appealReason: ''
  },

  onLoad(options: ClaimDetailOptions) {
    const { navBarHeight } = getStableBarHeights()
    const claimId = Number(options.id || 0)
    this.setData({ navBarHeight, claimId })

    if (!claimId) {
      this.setData({
        loading: false,
        initialError: true,
        initialErrorMessage: '缺少索赔 ID，无法查看详情'
      })
      return
    }

    this.loadDetail()
  },

  onShow() {
    if (this.data.claimId && !this.data.loading && !this.data.submitting && !this.data.recoveryPaying) {
      this.loadDetail(true)
    }
  },

  onPullDownRefresh() {
    this.loadDetail(Boolean(this.data.detail))
  },

  async loadDetail(silent = false) {
    if (!silent) {
      this.setData({
        loading: true,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        actionNoticeMessage: '',
        decisionLoading: false,
        decisionError: false,
        decisionErrorMessage: '',
        recoveryLoading: false,
        recoveryError: false,
        recoveryErrorMessage: '',
        behaviorLoading: true,
        behaviorError: false,
        behaviorErrorMessage: ''
      })
    } else {
      this.setData({ refreshErrorMessage: '' })
    }

    try {
      const claim = await claimManagementService.getRiderClaimDetail(this.data.claimId)
      const [decisionResult, recoveryResult, appealResult, behaviorResult] = await settleAll([
        claimManagementService.getRiderClaimDecision(this.data.claimId),
        claimManagementService.getRiderClaimRecovery(this.data.claimId),
        claim.appeal_id ? appealManagementService.getRiderAppealDetail(claim.appeal_id) : Promise.resolve(null as AppealResponse | null),
        claimManagementService.getRiderClaimBehaviorSummary(claim.order_id)
      ] as const)

      const decision: MerchantClaimDecisionResponse['decision'] = decisionResult.status === 'fulfilled'
        ? decisionResult.value.decision
        : null

      const decisionError = decisionResult.status === 'rejected'

      const recoveryNotFound = recoveryResult.status === 'rejected' && isClaimRecoveryNotFoundError(recoveryResult.reason)
      const recovery: ClaimRecoveryResponse | null = recoveryResult.status === 'fulfilled'
        ? recoveryResult.value
        : null

      const recoveryError = recoveryResult.status === 'rejected' && !recoveryNotFound

      const appeal: AppealResponse | null = appealResult.status === 'fulfilled'
        ? appealResult.value
        : null

      const behaviorSummary = behaviorResult.status === 'fulfilled'
        ? behaviorResult.value
        : null

      const claimStatus = String(claim.status)
      const detail: RiderClaimDetailView = {
        appealId: claim.appeal_id,
        orderId: claim.order_id,
        orderNo: claim.order_no || String(claim.order_id),
        claimTypeLabel: formatClaimType(claim.claim_type),
        statusLabel: formatClaimStatus(claimStatus),
        claimAmountText: formatMoney(claim.claim_amount),
        approvedAmountText: formatMoney(claim.approved_amount || claim.claim_amount),
        createdAtLabel: formatTime(claim.created_at),
        description: claim.description,
        responsiblePartyLabel: decisionError ? '责任判定加载失败' : formatResponsibleParty(decision?.responsible_party),
        compensationSourceLabel: decisionError ? '请重试后查看' : formatCompensationSource(decision?.compensation_source),
        reasonCodesText: decisionError ? '加载失败' : (decision?.reason_codes?.join('、') || '无'),
        traceSummary: decisionError ? undefined : decision?.trace_summary,
        recoveryStatusLabel: recoveryError ? '追偿信息加载失败' : formatRecoveryStatus(recovery?.status),
        recoveryAmountText: recovery ? formatMoney(recovery.recovery_amount) : undefined,
        dueAtLabel: recovery?.due_at ? formatTime(recovery.due_at) : undefined,
        appealStatusLabel: formatAppealStatus(claim.appeal_status),
        appealReasonText: appeal?.reason || claim.appeal_reason,
        reviewNotes: appeal?.review_notes || claim.appeal_review_notes,
        reviewedAtLabel: appeal?.reviewed_at ? formatTime(appeal.reviewed_at) : undefined,
        hasAppeal: Boolean(claim.appeal_id),
        canSubmitAppeal: (claimStatus === 'approved' || claimStatus === 'auto-approved') && !claim.appeal_id,
        canPayRecovery: Boolean(recovery && (recovery.status === 'pending' || recovery.status === 'overdue')),
        progressCurrent: this.getProgressCurrent(claimStatus, recovery?.status, Boolean(claim.appeal_id)),
        progressClaimText: formatTime(claim.created_at),
        progressRecoveryText: recovery?.due_at ? `待处理至 ${formatTime(recovery.due_at)}` : '等待平台生成或无需追偿',
        progressAppealText: appeal?.reviewed_at
          ? `已复核 ${formatTime(appeal.reviewed_at)}`
          : appeal?.created_at
            ? `已提交 ${formatTime(appeal.created_at)}`
            : '未提交申诉'
      }

      this.setData({
        detail,
        loading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        actionNoticeMessage: '',
        appealReason: claim.appeal_reason || '',
        decisionLoading: false,
        decisionError,
        decisionErrorMessage: decisionError ? getErrorMessage(decisionResult.reason, '责任判定加载失败，可单独重试') : '',
        recoveryLoading: false,
        recoveryError,
        recoveryErrorMessage: recoveryError ? getErrorMessage(recoveryResult.reason, '追偿信息加载失败，可单独重试') : '',
        behaviorLoading: false,
        behaviorError: behaviorResult.status === 'rejected',
        behaviorErrorMessage: behaviorResult.status === 'rejected' ? '行为摘要加载失败，可单独重试' : '',
        behaviorWindowLabel: behaviorSummary ? `${behaviorSummary.window.start_date} 至 ${behaviorSummary.window.end_date}` : '近30日',
        behaviorCards: behaviorSummary ? buildBehaviorCards(behaviorSummary) : [],
        hasRiderBehavior: Boolean(behaviorSummary?.rider)
      })
    } catch (error) {
      logger.error('Load rider claim detail failed', error)
      const message = getErrorMessage(error, '索赔详情加载失败，请稍后重试')
      if (!this.data.detail || !silent) {
        this.setData({
          loading: false,
          initialError: true,
          initialErrorMessage: message,
          refreshErrorMessage: '',
          actionNoticeMessage: '',
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
  },

  applyAppealSubmitted(appeal: AppealResponse, reason: string) {
    if (!this.data.detail) return

    this.setData({
      'detail.appealId': appeal.id,
      'detail.appealStatusLabel': formatAppealStatus(appeal.status),
      'detail.appealReasonText': appeal.reason || reason,
      'detail.reviewNotes': appeal.review_notes,
      'detail.reviewedAtLabel': appeal.reviewed_at ? formatTime(appeal.reviewed_at) : undefined,
      'detail.hasAppeal': true,
      'detail.canSubmitAppeal': false,
      'detail.progressCurrent': Math.max(this.data.detail.progressCurrent, 2),
      'detail.progressAppealText': formatTime(appeal.created_at),
      appealReason: appeal.reason || reason,
      actionNoticeMessage: '申诉已提交，平台复核状态稍后同步',
      refreshErrorMessage: ''
    })
  },

  applyRecoveryPaymentState(recovery: ClaimRecoveryResponse, status: string) {
    if (!this.data.detail) return

    const nextRecoveryStatus = status === 'paid' ? 'paid' : recovery.status
    this.setData({
      'detail.recoveryStatusLabel': formatRecoveryStatus(nextRecoveryStatus),
      'detail.recoveryAmountText': formatMoney(recovery.recovery_amount),
      'detail.dueAtLabel': recovery.due_at ? formatTime(recovery.due_at) : undefined,
      'detail.canPayRecovery': false,
      'detail.progressCurrent': this.getProgressCurrent(undefined, nextRecoveryStatus, this.data.detail.hasAppeal),
      actionNoticeMessage: status === 'paid'
        ? '追偿支付已完成，页面状态稍后同步'
        : '追偿支付已提交，页面状态稍后同步',
      refreshErrorMessage: ''
    })
  },

  async onRetryDecision() {
    if (!this.data.claimId || this.data.decisionLoading) return

    this.setData({
      decisionLoading: true,
      decisionError: false,
      decisionErrorMessage: ''
    })

    try {
      const result = await claimManagementService.getRiderClaimDecision(this.data.claimId)
      const decision = result.decision
      this.setData({
        decisionLoading: false,
        decisionError: false,
        decisionErrorMessage: '',
        'detail.responsiblePartyLabel': formatResponsibleParty(decision?.responsible_party),
        'detail.compensationSourceLabel': formatCompensationSource(decision?.compensation_source),
        'detail.reasonCodesText': decision?.reason_codes?.join('、') || '无',
        'detail.traceSummary': decision?.trace_summary
      })
    } catch (error) {
      logger.error('Reload rider claim decision failed', error)
      this.setData({
        decisionLoading: false,
        decisionError: true,
        decisionErrorMessage: getErrorMessage(error, '责任判定加载失败，可稍后重试'),
        'detail.responsiblePartyLabel': '责任判定加载失败',
        'detail.compensationSourceLabel': '请重试后查看',
        'detail.reasonCodesText': '加载失败',
        'detail.traceSummary': undefined
      })
    }
  },

  async onRetryRecovery() {
    if (!this.data.claimId || this.data.recoveryLoading) return

    this.setData({
      recoveryLoading: true,
      recoveryError: false,
      recoveryErrorMessage: ''
    })

    try {
      const recovery = await claimManagementService.getRiderClaimRecovery(this.data.claimId)
      this.setData({
        recoveryLoading: false,
        recoveryError: false,
        recoveryErrorMessage: '',
        'detail.recoveryStatusLabel': formatRecoveryStatus(recovery.status),
        'detail.recoveryAmountText': formatMoney(recovery.recovery_amount),
        'detail.dueAtLabel': recovery.due_at ? formatTime(recovery.due_at) : undefined,
        'detail.canPayRecovery': recovery.status === 'pending' || recovery.status === 'overdue'
      })
    } catch (error) {
      logger.error('Reload rider claim recovery failed', error)

      if (isClaimRecoveryNotFoundError(error)) {
        this.setData({
          recoveryLoading: false,
          recoveryError: false,
          recoveryErrorMessage: '',
          'detail.recoveryStatusLabel': '无追偿单',
          'detail.recoveryAmountText': undefined,
          'detail.dueAtLabel': undefined,
          'detail.canPayRecovery': false
        })
        return
      }

      this.setData({
        recoveryLoading: false,
        recoveryError: true,
        recoveryErrorMessage: getErrorMessage(error, '追偿信息加载失败，可稍后重试'),
        'detail.recoveryStatusLabel': '追偿信息加载失败',
        'detail.recoveryAmountText': undefined,
        'detail.dueAtLabel': undefined,
        'detail.canPayRecovery': false
      })
    }
  },

  async onRetryBehavior() {
    const orderId = this.data.detail?.orderId
    if (!orderId || this.data.behaviorLoading) return

    this.setData({
      behaviorLoading: true,
      behaviorError: false,
      behaviorErrorMessage: ''
    })

    try {
      const summary = await claimManagementService.getRiderClaimBehaviorSummary(orderId)
      this.setData({
        behaviorLoading: false,
        behaviorError: false,
        behaviorErrorMessage: '',
        behaviorWindowLabel: `${summary.window.start_date} 至 ${summary.window.end_date}`,
        behaviorCards: buildBehaviorCards(summary),
        hasRiderBehavior: Boolean(summary.rider)
      })
    } catch (error) {
      logger.error('Load rider claim behavior summary failed', error)
      this.setData({
        behaviorLoading: false,
        behaviorError: true,
        behaviorErrorMessage: getErrorMessage(error, '行为摘要加载失败，可稍后重试')
      })
    }
  },

  onAppealInput(e: WechatMiniprogram.CustomEvent) {
    this.setData({ appealReason: String(e.detail.value || '') })
  },

  async onSubmitAppeal() {
    const reason = this.data.appealReason.trim()
    const validation = validateAppealReason(reason)
    if (!validation.valid) {
      wx.showToast({ title: validation.message || '请输入有效申诉理由', icon: 'none' })
      return
    }

    try {
      this.setData({ submitting: true })
      wx.showLoading({ title: '提交中...' })
      const appeal = await appealManagementService.createRiderAppeal({
        claim_id: this.data.claimId,
        reason
      })
      this.applyAppealSubmitted(appeal, reason)
      wx.showToast({ title: '申诉已提交', icon: 'success' })
      await this.loadDetail(true)
    } catch (error) {
      logger.error('Submit rider appeal failed', error)
      wx.showToast({ title: getErrorMessage(error, '提交申诉失败，请稍后重试'), icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ submitting: false })
    }
  },

  async onPayRecovery() {
    if (!this.data.claimId) return

    const confirmed = await new Promise<boolean>((resolve) => {
      wx.showModal({
        title: '支付追偿款',
        content: '将为该追偿单创建微信支付订单，支付完成后系统再更新追偿状态。',
        confirmText: '去支付',
        success: (res) => resolve(Boolean(res.confirm)),
        fail: () => resolve(false)
      })
    })

    if (!confirmed) return

    try {
      this.setData({ recoveryPaying: true })
      wx.showLoading({ title: '拉起支付中...' })
      const paymentResult = await claimManagementService.payRiderClaimRecovery(this.data.claimId)
      const shouldSync = await this.handleRecoveryPayment(paymentResult)
      if (!shouldSync) {
        return
      }

      this.applyRecoveryPaymentState(paymentResult.recovery, paymentResult.status)
      await this.loadDetail(true)
    } catch (error) {
      logger.error('Confirm rider claim recovery failed', error)
      wx.showToast({ title: getErrorMessage(error, '支付追偿款失败，请稍后重试'), icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ recoveryPaying: false })
    }
  },

  async handleRecoveryPayment(paymentResult: ClaimRecoveryPaymentResponse) {
    if (paymentResult.pay_params) {
      try {
        await invokeWechatPay(paymentResult.pay_params)
      } catch (error: unknown) {
        const wxError = error as { errMsg?: string }
        if (wxError?.errMsg?.includes('cancel')) {
          wx.showToast({ title: '已取消支付', icon: 'none' })
          return false
        }
        throw error
      }

      try {
        await pollPaymentStatus(paymentResult.payment_order_id, 5, 1500)
      } catch (error) {
        logger.error('Poll rider claim recovery payment status timeout', error)
      }

      wx.showToast({ title: '追偿支付已提交', icon: 'success' })
      return true
    }

    if (paymentResult.status === 'paid') {
      wx.showToast({ title: '追偿款已支付', icon: 'success' })
      return true
    }

    wx.showToast({ title: '支付单已创建，请稍后查看状态', icon: 'none' })
    return true
  },

  onRetryRefresh() {
    this.loadDetail(true)
  },

  onRetry() {
    this.loadDetail(false)
  },

  getProgressCurrent(claimStatus?: string, recoveryStatus?: string, hasAppeal?: boolean) {
    if (hasAppeal) {
      return recoveryStatus === 'waived' ? 3 : 2
    }
    if (recoveryStatus === 'paid' || recoveryStatus === 'waived' || claimStatus === 'rejected') {
      return 3
    }
    if (recoveryStatus === 'pending' || recoveryStatus === 'overdue' || claimStatus === 'approved' || claimStatus === 'auto-approved') {
      return 1
    }
    return 0
  }
})