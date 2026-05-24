import dayjs from 'dayjs'
import {
  BehaviorSummaryStat,
  MerchantClaimBehaviorSummaryResponse
} from '../api/appeals-customer-service'
import { getErrorDebugMessage } from './user-facing'

export interface RiderClaimDetailView {
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
    compensated: '已赔付',
    'auto-approved': '已通过'
  }
  if (!status) return '-'
  return map[status] || status
}

export function formatAppealStatus(status?: string): string {
  const map: Record<string, string> = {
    pending: '申诉处理中',
    approved: '申诉通过',
    rejected: '申诉驳回',
    compensated: '申诉已赔付'
  }
  if (!status) return '未提交申诉'
  return map[status] || status
}

export function formatRecoveryStatus(status?: string): string {
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

export function formatResponsibleParty(party?: string): string {
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

export function formatCompensationSource(source?: string): string {
  const map: Record<string, string> = {
    merchant: '商户承担',
    rider: '骑手承担',
    platform: '平台承担',
    shared: '多方分摊'
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

export function getRiderClaimProgressCurrent(claimStatus?: string, recoveryStatus?: string, hasAppeal?: boolean) {
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

export function canSubmitRiderClaimAppeal(claimStatus?: string, appealId?: number | null) {
  return (claimStatus === 'approved' || claimStatus === 'auto-approved') && !appealId
}

export function canPayClaimRecovery(recoveryStatus?: string) {
  return recoveryStatus === 'pending' || recoveryStatus === 'overdue'
}
