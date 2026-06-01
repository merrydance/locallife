/**
 * Baofu settlement account view model.
 *
 * Builds a structured BaofuSettlementAccountView from a raw API response,
 * combining status text, themes, action capabilities, and payment extraction.
 * Separated so that pages import view-model logic without pulling in network
 * or raw status helpers directly.
 */

import type { BaofuSettlementAccountResponse, BaofuSettlementAccountPayment } from './baofu-account'
import type { BaofuSettlementAccountResultTheme, StatusTagTheme } from './baofu-account-status'
import {
  normalizeBaofuSettlementAccountStatus,
  getBaofuAccountStatusText,
  getBaofuAccountStatusTheme,
  getBaofuAccountResultTheme,
  getBaofuAccountNextActionText,
  isBaofuSettlementTerminalStatus,
  isBaofuSettlementPaymentRequiredStatus,
  isBaofuSettlementOpeningProcessingStatus
} from './baofu-account-status'

export interface BaofuSettlementAccountView {
  normalizedStatus: string
  statusText: string
  statusDesc: string
  nextActionText: string
  tagTheme: StatusTagTheme
  resultTheme: BaofuSettlementAccountResultTheme
  verifyFeeAmount: number
  verifyFeeDisplay: string
  canSubmitProfile: boolean
  canStartPayment: boolean
  canRefresh: boolean
  canContinuePayment: boolean
  canRefreshStatus: boolean
  isTerminal: boolean
  isWaiting: boolean
  isReady: boolean
  paymentReady: boolean
  isFailed: boolean
  isProcessing: boolean
  statusFeedbackTitle: string
  statusReasonTitle: string
  statusNextStepTitle: string
  statusIcon: string
  showVerifyFeePrompt: boolean
}

export function getBaofuAccountPayment(response?: BaofuSettlementAccountResponse | null): BaofuSettlementAccountPayment | null {
  if (!response) {
    return null
  }

  const nested = response.payment
  const paymentOrderId = nested?.payment_order_id || response.payment_order_id || 0
  const payParams = nested?.pay_params || response.pay_params

  if (!paymentOrderId && !payParams) {
    return null
  }

  return {
    payment_order_id: paymentOrderId,
    amount: nested?.amount || response.amount || response.verify_fee_amount || 200,
    business_type: nested?.business_type || response.business_type || 'baofu_account_verify_fee',
    out_trade_no: nested?.out_trade_no || response.out_trade_no,
    pay_params: payParams,
    expires_at: nested?.expires_at || response.expires_at
  }
}

function buildBaofuAccountStatusIcon(status: string): string {
  switch (status) {
    case 'ready':
      return 'check-circle'
    case 'failed':
    case 'voided':
      return 'error-circle'
    case 'verify_fee_pending':
      return 'wallet'
    case 'profile_pending':
      return 'assignment'
    case 'verify_fee_processing':
    case 'opening_processing':
    case 'merchant_report_processing':
    case 'applet_auth_pending':
      return 'time'
    default:
      return 'info-circle'
  }
}

export function buildBaofuSettlementAccountView(
  response?: BaofuSettlementAccountResponse | null
): BaofuSettlementAccountView {
  const normalizedStatus = normalizeBaofuSettlementAccountStatus(response?.status || response?.state)
  const verifyFeeAmount = Number(response?.verify_fee_amount || response?.payment?.amount || response?.amount || 200)
  const nextActionText = getBaofuAccountNextActionText(normalizedStatus, verifyFeeAmount)
  const statusDesc = String(response?.status_desc || nextActionText).trim()
  const paymentReady = normalizedStatus === 'ready' || response?.payment_ready === true
  const isFailedStatus = normalizedStatus === 'failed'
  const isVoidedStatus = normalizedStatus === 'voided'
  const isTerminalIssue = isFailedStatus || isVoidedStatus

  return {
    normalizedStatus,
    statusText: response?.label || getBaofuAccountStatusText(normalizedStatus),
    statusDesc,
    nextActionText,
    tagTheme: getBaofuAccountStatusTheme(normalizedStatus),
    resultTheme: getBaofuAccountResultTheme(normalizedStatus),
    verifyFeeAmount,
    verifyFeeDisplay: (verifyFeeAmount / 100).toFixed(2),
    canSubmitProfile: normalizedStatus === 'profile_pending' || normalizedStatus === 'failed',
    canStartPayment: isBaofuSettlementPaymentRequiredStatus(normalizedStatus),
    canRefresh: normalizedStatus !== 'ready' && normalizedStatus !== 'voided',
    canContinuePayment: isBaofuSettlementPaymentRequiredStatus(normalizedStatus),
    canRefreshStatus: normalizedStatus !== 'ready' && normalizedStatus !== 'voided',
    isTerminal: isBaofuSettlementTerminalStatus(normalizedStatus),
    isWaiting: normalizedStatus === 'unknown' || isBaofuSettlementOpeningProcessingStatus(normalizedStatus),
    isReady: normalizedStatus === 'ready',
    paymentReady,
    isFailed: normalizedStatus === 'failed',
    isProcessing: isBaofuSettlementOpeningProcessingStatus(normalizedStatus),
    statusFeedbackTitle: isFailedStatus ? '错误信息反馈' : '开户状态',
    statusReasonTitle: isFailedStatus ? '失败原因' : isVoidedStatus ? '作废原因' : '状态说明',
    statusNextStepTitle: isTerminalIssue ? '处理建议' : '下一步',
    statusIcon: buildBaofuAccountStatusIcon(normalizedStatus),
    showVerifyFeePrompt: normalizedStatus === 'verify_fee_pending'
  }
}
