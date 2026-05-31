import { AppealResponse, ClaimResponse } from '../_main_shared/api/appeals-customer-service'
import {
  ClaimTagTheme,
  formatClaimStatus,
  formatClaimType,
  formatMoney,
  formatTime,
  getClaimStatusTheme
} from '../_main_shared/utils/merchant-claim-detail-view'

export { ClaimTagTheme, formatClaimType, formatMoney, formatTime }

interface RiderStatusView {
  label: string
  theme: ClaimTagTheme
}

const SUCCESS_APPEAL_STATUSES = new Set(['approved'])
const WARNING_RECOVERY_STATUSES = new Set(['pending', 'overdue'])
const SUCCESS_RECOVERY_STATUSES = new Set(['paid', 'waived'])
const CLOSED_RIDER_CLAIM_RECOVERY_STATUSES = new Set(['paid', 'waived'])

export function getRiderClaimStatusView(status?: string): RiderStatusView {
  return {
    label: formatClaimStatus((status || 'pending') as ClaimResponse['status']),
    theme: getClaimStatusTheme(status)
  }
}

export function getRiderAppealStatusView(status?: string): RiderStatusView {
  const normalizedStatus = String(status || '').trim().toLowerCase()

  if (!normalizedStatus) {
    return { label: '未提交申诉', theme: 'default' }
  }

  if (normalizedStatus === 'submitted') {
    return { label: '申诉处理中', theme: 'warning' }
  }

  if (SUCCESS_APPEAL_STATUSES.has(normalizedStatus)) {
    return {
      label: '申诉通过',
      theme: 'success'
    }
  }

  return { label: '申诉驳回', theme: 'danger' }
}

export function getRiderRecoveryStatusView(status?: string): RiderStatusView {
  const normalizedStatus = String(status || '').trim().toLowerCase()

  if (!normalizedStatus) {
    return { label: '暂无追偿', theme: 'default' }
  }

  if (WARNING_RECOVERY_STATUSES.has(normalizedStatus)) {
    return {
      label: normalizedStatus === 'overdue' ? '追偿已逾期' : '待支付追偿',
      theme: 'warning'
    }
  }

  if (SUCCESS_RECOVERY_STATUSES.has(normalizedStatus)) {
    return {
      label: normalizedStatus === 'paid' ? '追偿已支付' : '追偿已豁免',
      theme: 'success'
    }
  }

  return { label: '追偿申诉中', theme: 'danger' }
}

export function getRiderClaimActionHint(claim: Pick<ClaimResponse, 'appeal_status' | 'recovery_dispute_status' | 'recovery_status'>): string {
  const appealStatus = String(claim.recovery_dispute_status || claim.appeal_status || '').trim().toLowerCase()
  const recoveryStatus = String(claim.recovery_status || '').trim().toLowerCase()

  if (recoveryStatus === 'disputed' || appealStatus === 'submitted') {
    return '平台正在复核申诉，进入详情可查看最新处理进度。'
  }

  if (WARNING_RECOVERY_STATUSES.has(recoveryStatus)) {
    return '当前有待处理追偿，进入详情可支付追偿款或提交申诉。'
  }

  if (appealStatus === 'rejected') {
    return '申诉已驳回，请进入详情核对复核说明和追偿状态。'
  }

  if (SUCCESS_APPEAL_STATUSES.has(appealStatus) || CLOSED_RIDER_CLAIM_RECOVERY_STATUSES.has(recoveryStatus)) {
    return '这笔索赔已进入结案阶段，进入详情可查看最终结果。'
  }

  return '进入详情查看责任判定，并决定是否申诉或支付追偿。'
}

export function buildRiderAppealViewStatus(status?: AppealResponse['status']): RiderStatusView {
  return getRiderAppealStatusView(status)
}
