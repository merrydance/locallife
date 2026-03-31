import { riderExceptionHandlingService } from '../../../api/rider-exception-handling'
import {
  appealManagementService,
  claimManagementService,
  ClaimResponse,
  CreateAppealRequest,
  MerchantClaimDecisionResponse
} from '../../../api/appeals-customer-service'
import { invokeWechatPay } from '../../../api/payment'
import { getStableBarHeights } from '../../../utils/responsive'

interface RiderClaimsOptions {
  taskId?: string
}

interface ClaimDisplay {
  id: number
  orderId?: number
  orderNo: string
  typeLabel: string
  description: string
  status: string
  statusLabel: string
  createdAt: string
  appealId?: number
  appealStatus?: string
  appealStatusLabel?: string
  recoveryStatus?: string
  recoveryStatusLabel?: string
}

interface AppealDisplay {
  id: number
  claimId: number
  status: string
  statusLabel: string
  reason: string
  createdAt: string
  reviewNotes?: string
}

interface UserMessageError {
  userMessage?: string
}

function formatResponsibleParty(party?: string) {
  const map: Record<string, string> = {
    rider: '骑手承担',
    merchant: '商户承担',
    platform: '平台承担',
    user: '用户承担',
    shared: '多方分摊'
  }
  return party ? (map[party] || party) : '暂无'
}

function formatCompensationSource(source?: string) {
  const map: Record<string, string> = {
    rider: '骑手追偿',
    merchant: '商户追偿',
    platform: '平台承担',
    shared: '分摊承担'
  }
  return source ? (map[source] || source) : '暂无'
}

function formatDecisionStatus(status?: string) {
  const map: Record<string, string> = {
    decided: '已判定',
    pending: '待判定',
    waived: '已豁免'
  }
  return status ? (map[status] || status) : '暂无'
}

Page({
  data: {
    taskId: '',
    activeTab: 'claims',
    claims: [] as ClaimDisplay[],
    appeals: [] as AppealDisplay[],
    form: {
      claimId: 0,
      claimLabel: '',
      description: ''
    },
    navBarHeight: 88,
    loading: false,
    submitting: false,
    errorMessage: '',
    recoveryPaying: {} as Record<number, boolean>,
    decisionLoading: {} as Record<number, boolean>
  },

  onLoad(options: RiderClaimsOptions) {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    if (options.taskId) {
      this.setData({ taskId: options.taskId })
    }
    this.loadClaims()
  },

  onShow() {
    this.loadClaims()
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  switchTab(e: WechatMiniprogram.CustomEvent<{ value: 'claims' | 'appeals' }>) {
    this.setData({ activeTab: e.detail.value })
  },

  getClaimTypeLabel(type: string) {
    const map: Record<string, string> = {
      refund: '退款索赔',
      compensation: '补偿索赔',
      quality_issue: '商品问题',
      delivery_issue: '配送问题'
    }
    return map[type] || type
  },

  getClaimStatusLabel(status: string) {
    const map: Record<string, string> = {
      pending: '待处理',
      approved: '已通过',
      rejected: '已驳回',
      compensated: '已赔付'
    }
    return map[status] || status
  },

  getAppealStatusLabel(status: string) {
    const map: Record<string, string> = {
      pending: '申诉处理中',
      approved: '申诉通过',
      rejected: '申诉驳回',
      compensated: '已补偿'
    }
    return map[status] || status
  },

  getRecoveryStatusLabel(status: string) {
    const map: Record<string, string> = {
      pending: '待支付',
      paid: '已支付',
      overdue: '已逾期',
      waived: '已豁免',
      appealed: '申诉中'
    }
    return map[status] || status
  },

  async loadClaims() {
    this.setData({ loading: true })
    try {
      const [claimsResponse, appealsResponse] = await Promise.all([
        riderExceptionHandlingService.getRiderClaims({
          page_id: 1,
          page_size: 20
        }),
        appealManagementService.getRiderAppeals({
          page_id: 1,
          page_size: 20
        })
      ])

      const claims: ClaimDisplay[] = (claimsResponse.claims || []).map((claim: ClaimResponse) => ({
        id: claim.id,
        orderId: claim.order_id,
        orderNo: claim.order_no || `#${claim.order_id}`,
        typeLabel: this.getClaimTypeLabel(claim.claim_type),
        description: claim.description,
        status: claim.status,
        statusLabel: this.getClaimStatusLabel(claim.status),
        createdAt: claim.created_at,
        appealId: claim.appeal_id,
        appealStatus: claim.appeal_status,
        appealStatusLabel: claim.appeal_status ? this.getAppealStatusLabel(claim.appeal_status) : '',
        recoveryStatus: claim.recovery_status,
        recoveryStatusLabel: claim.recovery_status ? this.getRecoveryStatusLabel(claim.recovery_status) : ''
      }))

      const appeals: AppealDisplay[] = (appealsResponse.appeals || []).map((appeal) => ({
        id: appeal.id,
        claimId: appeal.claim_id,
        status: appeal.status,
        statusLabel: this.getAppealStatusLabel(appeal.status),
        reason: appeal.reason,
        createdAt: appeal.created_at,
        reviewNotes: appeal.review_notes
      }))

      const preselectedClaim = this.data.taskId
        ? claims.find((claim) => String(claim.orderId || '') === this.data.taskId)
        : undefined

      this.setData({
        claims,
        appeals,
        errorMessage: '',
        'form.claimId': this.data.form.claimId || preselectedClaim?.id || 0,
        'form.claimLabel': this.data.form.claimLabel || (preselectedClaim ? `${preselectedClaim.typeLabel} · ${preselectedClaim.orderNo}` : '')
      })
    } catch (error: unknown) {
      console.error('加载索赔与申诉失败:', error)
      const userMessage = (error as UserMessageError).userMessage
      const message = typeof userMessage === 'string' && userMessage ? userMessage : '索赔与申诉加载失败，请稍后重试'
      this.setData({
        claims: [],
        appeals: [],
        errorMessage: message
      })
    } finally {
      this.setData({ loading: false })
    }
  },

  onPickClaim() {
    if (this.data.claims.length === 0) {
      wx.showToast({ title: '当前没有可关联的索赔单', icon: 'none' })
      return
    }

    wx.showActionSheet({
      itemList: this.data.claims.map((claim) => `${claim.typeLabel} · ${claim.orderNo}`),
      success: ({ tapIndex }) => {
        const selected = this.data.claims[tapIndex]
        if (!selected) return
        this.setData({
          'form.claimId': selected.id,
          'form.claimLabel': `${selected.typeLabel} · ${selected.orderNo}`
        })
      }
    })
  },

  onPrepareAppeal(e: WechatMiniprogram.TouchEvent) {
    const { claimId } = e.currentTarget.dataset as { claimId?: number }
    const selected = this.data.claims.find((claim) => claim.id === claimId)
    if (!selected) return

    this.setData({
      activeTab: 'claims',
      'form.claimId': selected.id,
      'form.claimLabel': `${selected.typeLabel} · ${selected.orderNo}`
    })
    wx.pageScrollTo({ scrollTop: 0, duration: 200 })
  },

  onDescChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({ 'form.description': e.detail.value })
  },

  async onViewDecision(e: WechatMiniprogram.TouchEvent) {
    const claimId = Number(e.currentTarget.dataset.claimId)
    if (!claimId || this.data.decisionLoading[claimId]) {
      return
    }

    this.setData({ decisionLoading: { ...this.data.decisionLoading, [claimId]: true } })
    wx.showLoading({ title: '加载中...' })
    try {
      const result = await claimManagementService.getRiderClaimDecision(claimId)
      const decision: MerchantClaimDecisionResponse['decision'] = result.decision

      if (!decision) {
        wx.showModal({
          title: '判定依据',
          content: '当前索赔单还没有可展示的责任判定信息。',
          showCancel: false
        })
        return
      }

      const content = [
        `判定状态：${formatDecisionStatus(decision.decision_status)}`,
        `责任归属：${formatResponsibleParty(decision.responsible_party)}`,
        `赔付来源：${formatCompensationSource(decision.compensation_source)}`,
        `原因码：${decision.reason_codes?.length ? decision.reason_codes.join('、') : '无'}`,
        decision.trace_summary ? `判定摘要：${decision.trace_summary}` : ''
      ].filter(Boolean).join('\n')

      wx.showModal({
        title: '判定依据',
        content,
        showCancel: false
      })
    } catch (error: unknown) {
      const userMessage = (error as UserMessageError).userMessage
      const message = typeof userMessage === 'string' && userMessage ? userMessage : '判定依据加载失败'
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ decisionLoading: { ...this.data.decisionLoading, [claimId]: false } })
    }
  },

  async onPayRecovery(e: WechatMiniprogram.CustomEvent) {
    const claimId = Number(e.currentTarget.dataset.claimId)
    if (!claimId) {
      return
    }

    const payingMap = { ...this.data.recoveryPaying, [claimId]: true }
    this.setData({ recoveryPaying: payingMap })
    try {
      const payment = await claimManagementService.payRiderClaimRecovery(claimId)
      if (payment.pay_params) {
        await invokeWechatPay(payment.pay_params)
      }
      wx.showToast({ title: '追偿支付已提交', icon: 'success' })
      this.loadClaims()
    } catch (error: unknown) {
      console.error('支付追偿失败:', error)
      const errMsg = error && typeof error === 'object' && 'errMsg' in error ? (error as { errMsg?: string }).errMsg : ''
      if (typeof errMsg === 'string' && errMsg.includes('cancel')) {
        wx.showToast({ title: '已取消支付', icon: 'none' })
      } else {
        wx.showToast({ title: '支付失败', icon: 'none' })
      }
    } finally {
      const nextMap = { ...this.data.recoveryPaying, [claimId]: false }
      this.setData({ recoveryPaying: nextMap })
    }
  },

  async onSubmit() {
    const { form } = this.data
    if (!form.claimId || !form.description.trim()) {
      wx.showToast({ title: '请填写完整信息', icon: 'none' })
      return
    }

    this.setData({ submitting: true })
    try {
      const appealData: CreateAppealRequest = {
        claim_id: form.claimId,
        reason: form.description.trim()
      }

      await appealManagementService.createRiderAppeal(appealData)

      wx.showToast({ title: '申诉已提交', icon: 'success' })
      this.setData({
        activeTab: 'appeals',
        form: { claimId: 0, claimLabel: '', description: '' }
      })
      this.loadClaims()
    } catch (error: unknown) {
      console.error('提交申诉失败:', error)
      const userMessage = (error as UserMessageError).userMessage
      const message = typeof userMessage === 'string' && userMessage ? userMessage : '提交失败'
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      this.setData({ submitting: false })
    }
  },

  onRetry() {
    this.loadClaims()
  }
})
