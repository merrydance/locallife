import {
  buildMerchantApplymentStatusView,
  getMerchantApplymentStatus,
  type ApplymentStatusResponse,
  type MerchantApplymentStatusView
} from '../api/merchant-applyment'
import {
  getMerchantSettlementAccount,
  getSettlementAccountStatusView,
  type MerchantSettlementAccountResponse
} from '../api/merchant-settlement-account'

export interface MerchantPaymentReadinessView {
  applymentStatus: ApplymentStatusResponse
  applymentView: MerchantApplymentStatusView
  settlementAccount: MerchantSettlementAccountResponse | null
  settlementActive: boolean
  summaryTitle: string
  summaryDescription: string
  actionText: string
  actionPath: string
  canOpenWithdraw: boolean
  canViewSettlementAccount: boolean
  canEditSettlementAccount: boolean
}

async function tryGetMerchantSettlementAccount() {
  return await getMerchantSettlementAccount()
}

export function buildMerchantPaymentReadinessView(params: {
  applymentStatus: ApplymentStatusResponse
  settlementAccount: MerchantSettlementAccountResponse | null
}): MerchantPaymentReadinessView {
  const applymentView = buildMerchantApplymentStatusView(params.applymentStatus)
  const settlementAccount = params.settlementAccount
  const settlementStatusView = getSettlementAccountStatusView(settlementAccount)
  const settlementActive = settlementStatusView.isActiveAccount
  const canOpenWithdraw = applymentView.isOpened && settlementStatusView.canOpenWithdraw

  if (canOpenWithdraw) {
    return {
      applymentStatus: params.applymentStatus,
      applymentView,
      settlementAccount,
      settlementActive,
      summaryTitle: '收付款能力已就绪',
      summaryDescription: '收付通和微信提现卡都已开通，可正常收款、结算和提现。',
      actionText: '查看微信提现卡',
      actionPath: '/pages/merchant/finance/settlement-account/index',
      canOpenWithdraw,
      canViewSettlementAccount: true,
      canEditSettlementAccount: true
    }
  }

  if (applymentView.isOpened && settlementStatusView.isVerificationFailed) {
    return {
      applymentStatus: params.applymentStatus,
      applymentView,
      settlementAccount,
      settlementActive,
      summaryTitle: '微信提现卡校验失败',
      summaryDescription: settlementStatusView.statusDesc || '微信提现卡校验失败，请尽快更换银行卡。',
      actionText: '查看微信提现卡',
      actionPath: '/pages/merchant/finance/settlement-account/index',
      canOpenWithdraw: false,
      canViewSettlementAccount: settlementStatusView.canViewSettlementAccount,
      canEditSettlementAccount: settlementStatusView.canEditSettlementAccount
    }
  }

  if (applymentView.isOpened && (settlementStatusView.isVerificationPending || settlementStatusView.hasUnknownVerificationState)) {
    return {
      applymentStatus: params.applymentStatus,
      applymentView,
      settlementAccount,
      settlementActive,
      summaryTitle: '微信提现卡校验中',
      summaryDescription: settlementStatusView.statusDesc || '微信提现卡状态同步中，请稍后查看结果。',
      actionText: '查看微信提现卡',
      actionPath: '/pages/merchant/finance/settlement-account/index',
      canOpenWithdraw: false,
      canViewSettlementAccount: settlementStatusView.canViewSettlementAccount,
      canEditSettlementAccount: settlementStatusView.canEditSettlementAccount
    }
  }

  if (applymentView.isOpened && !settlementActive) {
    return {
      applymentStatus: params.applymentStatus,
      applymentView,
      settlementAccount,
      settlementActive,
      summaryTitle: '收付通已开通，等待资金账户同步',
      summaryDescription: settlementStatusView.statusDesc || '收付通已开通，微信提现卡信息正在同步，请稍后再试。',
      actionText: '查看开户进度',
      actionPath: '/pages/merchant/settings/applyment/index',
      canOpenWithdraw: false,
      canViewSettlementAccount: false,
      canEditSettlementAccount: false
    }
  }

  if (applymentView.canSubmitOpenInfo) {
    return {
      applymentStatus: params.applymentStatus,
      applymentView,
      settlementAccount,
      settlementActive,
      summaryTitle: '先完成收付通开户',
      summaryDescription: applymentView.summaryText,
      actionText: applymentView.submitActionLabel,
      actionPath: '/pages/merchant/settings/applyment/index',
      canOpenWithdraw: false,
      canViewSettlementAccount: false,
      canEditSettlementAccount: false
    }
  }

  return {
    applymentStatus: params.applymentStatus,
    applymentView,
    settlementAccount,
    settlementActive,
    summaryTitle: applymentView.headline,
    summaryDescription: applymentView.summaryText,
    actionText: applymentView.primaryActionText,
    actionPath: '/pages/merchant/settings/applyment/index',
    canOpenWithdraw: false,
    canViewSettlementAccount: settlementStatusView.canViewSettlementAccount,
    canEditSettlementAccount: settlementStatusView.canEditSettlementAccount
  }
}

export async function fetchMerchantPaymentReadiness() {
  const [applymentStatus, settlementAccount] = await Promise.all([
    getMerchantApplymentStatus(),
    tryGetMerchantSettlementAccount()
  ])

  return buildMerchantPaymentReadinessView({
    applymentStatus,
    settlementAccount
  })
}