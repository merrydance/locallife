import type { RiderDepositPendingRechargeContext } from '../_main_shared/services/rider-deposit-payment'

export interface RiderDepositFinanceView {
  canWithdraw: boolean
  withdrawHint: string
  hasPendingRecharge: boolean
  pendingRechargeTitle: string
  pendingRechargeDescription: string
  pendingRechargeAmountDisplay: string
}

export interface RiderDepositWithdrawStatusView {
  isSuccess: boolean
  feedbackMessage: string
  feedbackTheme: 'success' | 'warning'
  shouldScheduleRefresh: boolean
}

export interface RiderDepositWorkbenchSummaryView {
  value: string
  note: string
  highlight: boolean
  highlightClass: string
  actionText: string
}

function formatFenToYuan(amount: number): string {
  return `¥${(Math.max(amount, 0) / 100).toFixed(2)}`
}

export function buildWithdrawHint(
  availableDeposit: number,
  deliveryFrozenDeposit: number,
  withdrawalProcessingAmount: number,
  activeDeliveries: number
): {
  canWithdraw: boolean
  withdrawHint: string
} {
  if (activeDeliveries > 0) {
    return {
      canWithdraw: false,
      withdrawHint: `当前有 ${activeDeliveries} 单进行中的代取，完成后才可申请提现`
    }
  }

  if (withdrawalProcessingAmount > 0 && deliveryFrozenDeposit > 0) {
    return {
      canWithdraw: false,
      withdrawHint: `当前有 ${formatFenToYuan(deliveryFrozenDeposit)} 代取冻结，另有 ${formatFenToYuan(withdrawalProcessingAmount)} 提现处理中，暂不可再次提现`
    }
  }

  if (withdrawalProcessingAmount > 0) {
    return {
      canWithdraw: false,
      withdrawHint: `当前有 ${formatFenToYuan(withdrawalProcessingAmount)} 正在提现处理中，到账前暂不可再次提现`
    }
  }

  if (deliveryFrozenDeposit > 0) {
    return {
      canWithdraw: false,
      withdrawHint: `当前有 ${formatFenToYuan(deliveryFrozenDeposit)} 代取冻结，待订单完成或取消后可提现`
    }
  }

  if (availableDeposit >= 100) {
    return {
      canWithdraw: true,
      withdrawHint: `当前可提现 ${formatFenToYuan(availableDeposit)}，提现将退回至微信零钱`
    }
  }

  return {
    canWithdraw: false,
    withdrawHint: `当前可提现 ${formatFenToYuan(availableDeposit)}，至少需满 ¥1.00 才能提现`
  }
}

export function buildRiderDepositFinanceView(input: {
  availableDeposit: number
  deliveryFrozenDeposit: number
  withdrawalProcessingAmount: number
  activeDeliveries: number
  pendingRecharge: RiderDepositPendingRechargeContext | null
}): RiderDepositFinanceView {
  const withdrawState = buildWithdrawHint(
    input.availableDeposit,
    input.deliveryFrozenDeposit,
    input.withdrawalProcessingAmount,
    input.activeDeliveries
  )

  if (!input.pendingRecharge) {
    return {
      ...withdrawState,
      hasPendingRecharge: false,
      pendingRechargeTitle: '',
      pendingRechargeDescription: '',
      pendingRechargeAmountDisplay: ''
    }
  }

  return {
    ...withdrawState,
    hasPendingRecharge: true,
    pendingRechargeTitle: '有一笔押金充值待确认',
    pendingRechargeDescription: '支付已发起但结果还未最终确认，可继续支付或查看支付进度。',
    pendingRechargeAmountDisplay: formatFenToYuan(input.pendingRecharge.amount)
  }
}

export function buildRiderDepositWorkbenchSummaryView(input: {
  availableDeposit: number
  deliveryFrozenDeposit: number
  withdrawalProcessingAmount: number
  thresholdAmount: number
  activeDeliveries: number
  canGoOnline: boolean
  onlineBlockReason: string
}): RiderDepositWorkbenchSummaryView {
  const hasProcessingWithdrawal = input.withdrawalProcessingAmount > 0
  const hasDeliveryFrozen = input.deliveryFrozenDeposit > 0
  const financeView = buildRiderDepositFinanceView({
    availableDeposit: input.availableDeposit,
    deliveryFrozenDeposit: input.deliveryFrozenDeposit,
    withdrawalProcessingAmount: input.withdrawalProcessingAmount,
    activeDeliveries: hasProcessingWithdrawal || hasDeliveryFrozen ? 0 : input.activeDeliveries,
    pendingRecharge: null
  })
  const belowThreshold = input.availableDeposit < input.thresholdAmount
  const blockedByDeposit = !input.canGoOnline && (/押金/.test(input.onlineBlockReason || '') || belowThreshold)

  let note = `门槛 ${formatFenToYuan(input.thresholdAmount)}`
  if (blockedByDeposit) {
    note = input.onlineBlockReason || '可用押金不足，补足后可上线接单'
  } else if (hasProcessingWithdrawal || hasDeliveryFrozen) {
    note = financeView.withdrawHint
  }

  const highlight = blockedByDeposit || belowThreshold

  return {
    value: formatFenToYuan(input.availableDeposit),
    note,
    highlight,
    highlightClass: highlight ? 'is-highlight' : '',
    actionText: blockedByDeposit ? '去充值' : '查看押金'
  }
}

export function getRiderDepositWithdrawStatusView(status?: string): RiderDepositWithdrawStatusView {
  if (status === 'success') {
    return {
      isSuccess: true,
      feedbackMessage: '提现已对账完成，账单记录已经同步更新。',
      feedbackTheme: 'success',
      shouldScheduleRefresh: false
    }
  }

  if (status === 'failed') {
    return {
      isSuccess: false,
      feedbackMessage: '提现未完成，资金会回到可用押金。请刷新账户后再重新申请。',
      feedbackTheme: 'warning',
      shouldScheduleRefresh: false
    }
  }

  if (status === 'partial_failed') {
    return {
      isSuccess: false,
      feedbackMessage: '部分提现未完成，请以账单明细和可用押金为准。',
      feedbackTheme: 'warning',
      shouldScheduleRefresh: true
    }
  }

  return {
    isSuccess: false,
    feedbackMessage: '提现请求已受理，到账结果会在微信退款结果确认后同步到账单列表。',
    feedbackTheme: 'warning',
    shouldScheduleRefresh: true
  }
}