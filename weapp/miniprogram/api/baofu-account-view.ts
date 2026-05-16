/**
 * Baofu settlement account view model.
 *
 * Builds a structured BaofuSettlementAccountView from a raw API response,
 * combining status text, themes, action capabilities, and payment extraction.
 * Separated so that pages import view-model logic without pulling in network
 * or raw status helpers directly.
 */

import type { BaofuSettlementAccountResponse, BaofuSettlementAccountPayment } from './baofu-account'
import type { StatusTagTheme } from '../utils/status-tag'
import type { BaofuSettlementAccountResultTheme } from './baofu-account-status'
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
  isFailed: boolean
  isProcessing: boolean
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

export function buildBaofuSettlementAccountView(
  response?: BaofuSettlementAccountResponse | null
): BaofuSettlementAccountView {
  const normalizedStatus = normalizeBaofuSettlementAccountStatus(response?.status || response?.state)
  const verifyFeeAmount = Number(response?.verify_fee_amount || response?.payment?.amount || response?.amount || 200)
  const nextActionText = getBaofuAccountNextActionText(normalizedStatus, verifyFeeAmount)
  const statusDesc = String(response?.status_desc || nextActionText).trim()

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
    isFailed: normalizedStatus === 'failed',
    isProcessing: isBaofuSettlementOpeningProcessingStatus(normalizedStatus)
  }
}
