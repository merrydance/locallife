/**
 * Baofu settlement account status helpers.
 *
 * Status text, tag themes, result themes, action text, and boolean status
 * judgment functions. Separated from the main API file so that UI layers
 * can import status logic without pulling in network concerns.
 */

import type { StatusTagTheme } from '../utils/status-tag'

export type BaofuSettlementAccountResultTheme = 'success' | 'warning' | 'error'

export function normalizeBaofuSettlementAccountStatus(status?: string): string {
  const normalized = String(status || '').trim().toLowerCase()
  return normalized || 'unknown'
}

export function getBaofuAccountStatusText(status?: string): string {
  switch (normalizeBaofuSettlementAccountStatus(status)) {
    case 'ready':
      return '已开通'
    case 'profile_pending':
      return '待补充资料'
    case 'verify_fee_pending':
      return '待支付核验费'
    case 'verify_fee_processing':
      return '支付确认中'
    case 'opening_processing':
      return '开户处理中'
    case 'merchant_report_processing':
      return '商户报备中'
    case 'applet_auth_pending':
      return '授权目录绑定中'
    case 'failed':
      return '开通失败'
    case 'voided':
      return '已作废'
    default:
      return '同步中'
  }
}

export function getBaofuAccountStatusTheme(status?: string): StatusTagTheme {
  switch (normalizeBaofuSettlementAccountStatus(status)) {
    case 'ready':
      return 'success'
    case 'failed':
    case 'voided':
      return 'danger'
    case 'profile_pending':
    case 'verify_fee_pending':
    case 'verify_fee_processing':
      return 'warning'
    case 'opening_processing':
    case 'merchant_report_processing':
    case 'applet_auth_pending':
      return 'primary'
    default:
      return 'default'
  }
}

export function getBaofuAccountResultTheme(status?: string): BaofuSettlementAccountResultTheme {
  switch (normalizeBaofuSettlementAccountStatus(status)) {
    case 'ready':
      return 'success'
    case 'failed':
    case 'voided':
      return 'error'
    default:
      return 'warning'
  }
}

export function getBaofuAccountNextActionText(
  status?: string,
  verifyFeeAmount = 200
): string {
  const feeDisplay = (verifyFeeAmount / 100).toFixed(2)
  switch (normalizeBaofuSettlementAccountStatus(status)) {
    case 'ready':
      return '结算账户已可用'
    case 'profile_pending':
      return '请补全开户资料后提交'
    case 'verify_fee_pending':
      return `支付 ${feeDisplay} 元核验费后继续开户`
    case 'verify_fee_processing':
      return '核验费支付结果确认中，正在自动同步'
    case 'opening_processing':
      return '宝付开户处理中，正在自动同步'
    case 'merchant_report_processing':
      return '正在配置微信支付商户展示名称，正在自动同步'
    case 'applet_auth_pending':
      return '正在绑定微信支付授权目录，正在自动同步'
    case 'failed':
      return '开户未通过，请核对资料后重试；如持续失败请联系平台处理'
    case 'voided':
      return '开户流程已作废，请联系平台处理'
    default:
      return '开户状态同步中，正在自动同步'
  }
}

export function isBaofuSettlementTerminalStatus(status?: string): boolean {
  const normalized = normalizeBaofuSettlementAccountStatus(status)
  return normalized === 'ready' ||
    normalized === 'failed' ||
    normalized === 'profile_pending' ||
    normalized === 'voided'
}

export function isBaofuSettlementAfterPaymentTerminalStatus(status?: string): boolean {
  return isBaofuSettlementTerminalStatus(status)
}

export function isBaofuSettlementPaymentRequiredStatus(status?: string): boolean {
  const normalized = normalizeBaofuSettlementAccountStatus(status)
  return normalized === 'verify_fee_pending'
}

export function isBaofuSettlementOpeningProcessingStatus(status?: string): boolean {
  const normalized = normalizeBaofuSettlementAccountStatus(status)
  return normalized === 'opening_processing' ||
    normalized === 'verify_fee_processing' ||
    normalized === 'merchant_report_processing' ||
    normalized === 'applet_auth_pending'
}
