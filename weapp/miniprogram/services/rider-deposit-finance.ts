import type { RiderDepositPendingRechargeContext } from './rider-deposit-payment'

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
      withdrawHint: `当前有 ${activeDeliveries} 单进行中的配送，完成后才可申请提现`
    }
  }

  if (withdrawalProcessingAmount > 0 && deliveryFrozenDeposit > 0) {
    return {
      canWithdraw: false,
      withdrawHint: `当前有 ${formatFenToYuan(deliveryFrozenDeposit)} 配送冻结，另有 ${formatFenToYuan(withdrawalProcessingAmount)} 提现处理中，暂不可再次提现`
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
      withdrawHint: `当前有 ${formatFenToYuan(deliveryFrozenDeposit)} 配送冻结，待订单完成或取消后可提现`
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

export function getRiderDepositWithdrawStatusView(status?: string): RiderDepositWithdrawStatusView {
  if (status === 'success') {
    return {
      isSuccess: true,
      feedbackMessage: '提现已完成，账单记录已经同步更新。',
      feedbackTheme: 'success',
      shouldScheduleRefresh: false
    }
  }

  return {
    isSuccess: false,
    feedbackMessage: '提现申请已提交，到账进度会同步到账单列表。',
    feedbackTheme: 'warning',
    shouldScheduleRefresh: true
  }
}