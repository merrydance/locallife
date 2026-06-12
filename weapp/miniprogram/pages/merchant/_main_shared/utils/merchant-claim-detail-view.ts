import dayjs from '../miniprogram_npm/dayjs/index'
import {
  BehaviorSummaryStat,
  MerchantClaimBehaviorSummaryResponse,
  MerchantUserRiskResponse
} from '../api/appeals-customer-service'
import { getErrorDebugMessage } from '../../../../utils/user-facing'

export interface MerchantClaimDetailView {
  appealId?: number
  orderId: number
  userId: number
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
  recoveryReleaseMessage?: string
  dueAtLabel?: string
  appealStatusLabel: string
  reviewNotes?: string
  hasAppeal: boolean
  canSubmitAppeal: boolean
  canPayRecovery: boolean
  progressCurrent: number
  progressClaimText: string
  progressRecoveryText: string
  progressAppealText: string
}

export interface BehaviorSummaryCardView {
  key: 'user' | 'merchant' | 'rider'
  title: string
  entityId: number
  totalOrders: number
  abnormalClaims: number
  abnormalRateText: string
  abnormalRateLevel: 'high' | 'medium' | 'low'
  hint: string
}

export interface MerchantUserRiskView {
  hasBlock: boolean
  reasonCode?: string
  blockUntilLabel?: string
  reminderText: string
}

export type ClaimTagTheme = 'primary' | 'warning' | 'success' | 'danger' | 'default'

interface MerchantClaimListActionState {
  actionHint: string
  isPendingAction: boolean
  isAppealedFlow: boolean
  isClosedFlow: boolean
}

const WARNING_RECOVERY_STATUSES = new Set(['pending', 'overdue'])
const SUCCESS_RECOVERY_STATUSES = new Set(['paid', 'waived'])
const SUCCESS_APPEAL_STATUSES = new Set(['approved'])

export function formatMoney(cents?: number): string {
  const value = typeof cents === 'number' ? cents : 0
  return `¥${(value / 100).toFixed(2)}`
}

export function formatTime(value?: string): string {
  if (!value) return '暂无'
  const parsed = dayjs(value)
  return parsed.isValid() ? parsed.format('YYYY-MM-DD HH:mm') : value
}

export function formatClaimType(claimType?: string): string {
  const map: Record<string, string> = {
    refund: '退款',
    compensation: '补偿',
    quality_issue: '质量问题',
    delivery_issue: '代取问题',
    'foreign-object': '异物',
    damage: '餐损',
    timeout: '超时',
    'food-safety': '食安'
  }
  if (!claimType) return '-'
  return map[claimType] || claimType
}

export function formatClaimStatus(status?: string): string {
  const map: Record<string, string> = {
    pending: '待审核',
    approved: '已通过',
    rejected: '已驳回',
    compensated: '已赔付'
  }
  if (!status) return '-'
  return map[status] || status
}

export function getClaimStatusTheme(status?: string): ClaimTagTheme {
  const normalizedStatus = String(status || '').trim().toLowerCase()

  if (normalizedStatus === 'approved') return 'warning'
  if (normalizedStatus === 'compensated') return 'success'
  if (normalizedStatus === 'rejected') return 'default'
  return 'primary'
}

export function formatAppealStatus(status?: string): string {
  const map: Record<string, string> = {
    submitted: '异议待审核',
    approved: '异议已通过',
    rejected: '异议已驳回'
  }
  if (!status) return '未提交异议'
  return map[status] || status
}

export function getAppealStatusTheme(status?: string): ClaimTagTheme {
  const normalizedStatus = String(status || '').trim().toLowerCase()

  if (!normalizedStatus) return 'default'
  if (normalizedStatus === 'submitted') return 'warning'
  if (SUCCESS_APPEAL_STATUSES.has(normalizedStatus)) return 'success'
  if (normalizedStatus === 'rejected') return 'danger'
  return 'default'
}

export function formatRecoveryStatus(status?: string): string {
  const map: Record<string, string> = {
    pending: '待回款',
    overdue: '已逾期',
    paid: '已支付',
    waived: '已核销',
    disputed: '异议中'
  }
  if (!status) return '无追偿单'
  return map[status] || status
}

export function getRecoveryStatusTheme(status?: string): ClaimTagTheme {
  const normalizedStatus = String(status || '').trim().toLowerCase()

  if (!normalizedStatus) return 'default'
  if (WARNING_RECOVERY_STATUSES.has(normalizedStatus)) return 'warning'
  if (SUCCESS_RECOVERY_STATUSES.has(normalizedStatus)) return 'success'
  if (normalizedStatus === 'disputed') return 'danger'
  return 'default'
}

export function formatResponsibleParty(party?: string): string {
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

export function formatCompensationSource(source?: string): string {
  const map: Record<string, string> = {
    merchant: '商户承担',
    rider: '骑手承担',
    platform: '平台先赔'
  }
  if (!source) return '未知来源'
  return map[source] || source
}

export function isClaimRecoveryNotFoundError(err: unknown) {
  const normalized = getErrorDebugMessage(err).toLowerCase()
  return normalized.includes('claim recovery not found') || normalized.includes('追偿单不存在')
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
    merchant: '本店',
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

export function buildBehaviorCards(summary: MerchantClaimBehaviorSummaryResponse): BehaviorSummaryCardView[] {
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
      title: '本店',
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

export function toUserRiskView(risk: MerchantUserRiskResponse): MerchantUserRiskView {
  return {
    hasBlock: risk.has_block,
    reasonCode: risk.reason_code,
    blockUntilLabel: risk.block_until ? formatTime(risk.block_until) : undefined,
    reminderText: risk.reminder_text || (risk.has_block ? '该顾客存在历史风险，请谨慎处理索赔和履约。' : '当前未发现顾客风险拦截记录。')
  }
}

export function getMerchantClaimProgressCurrent(claimStatus?: string, recoveryStatus?: string, hasAppeal?: boolean) {
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

export function getMerchantClaimListActionState(input: {
  status?: string
  appealStatus?: string
  recoveryStatus?: string
}): MerchantClaimListActionState {
  const appealStatus = String(input.appealStatus || '').trim().toLowerCase()
  const recoveryStatus = String(input.recoveryStatus || '').trim().toLowerCase()
  const claimStatus = String(input.status || '').trim().toLowerCase()
  const isPendingAction = WARNING_RECOVERY_STATUSES.has(recoveryStatus)
    || (appealStatus === 'rejected' && !SUCCESS_RECOVERY_STATUSES.has(recoveryStatus))
  const isAppealedFlow = recoveryStatus === 'disputed' || appealStatus === 'submitted'
  const isClosedFlow = SUCCESS_RECOVERY_STATUSES.has(recoveryStatus)
    || SUCCESS_APPEAL_STATUSES.has(appealStatus)

  if (isAppealedFlow) {
    return {
      actionHint: '异议已提交，等待平台复核结果。',
      isPendingAction,
      isAppealedFlow,
      isClosedFlow
    }
  }

  if (isPendingAction) {
    return {
      actionHint: '平台已生成追偿单，建议尽快支付追偿款或先提交异议。',
      isPendingAction,
      isAppealedFlow,
      isClosedFlow
    }
  }

  if (isClosedFlow) {
    return {
      actionHint: '当前索赔已进入结案态，可进入详情核对最终结果。',
      isPendingAction,
      isAppealedFlow,
      isClosedFlow
    }
  }

  if (claimStatus === 'approved' || claimStatus === 'auto-approved') {
    return {
      actionHint: '责任已判定，可进入详情查看依据并决定是否提交异议。',
      isPendingAction,
      isAppealedFlow,
      isClosedFlow
    }
  }

  return {
    actionHint: '点击查看索赔详情与处理进度。',
    isPendingAction,
    isAppealedFlow,
    isClosedFlow
  }
}

export function canSubmitMerchantClaimAppeal(claimStatus?: string, appealId?: number | null) {
  return claimStatus === 'approved' && !appealId
}

export function canPayClaimRecovery(recoveryStatus?: string) {
  return recoveryStatus === 'pending' || recoveryStatus === 'overdue'
}
