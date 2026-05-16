/**
 * Baofu settlement account API layer.
 *
 * Types, interfaces, and network calls for the Baofu settlement account
 * domain. Status helpers live in ./baofu-account-status.ts; view model
 * logic lives in ./baofu-account-view.ts. Both are re-exported here so
 * that existing import paths continue to work.
 */

import { request } from '../utils/request'
import type { MiniProgramPayParams } from './payment'

export type BaofuAccountOwnerRole = 'rider' | 'merchant' | 'operator' | 'platform'

export type BaofuSettlementAccountStatus =
  | 'ready'
  | 'failed'
  | 'profile_pending'
  | 'verify_fee_pending'
  | 'verify_fee_processing'
  | 'opening_processing'
  | 'merchant_report_processing'
  | 'applet_auth_pending'
  | 'voided'

export type BaofuSettlementAccountProfileStatus =
  | 'complete'
  | 'incomplete'

export type BaofuSettlementAccountOpenState =
  | 'active'
  | 'processing'
  | 'failed'
  | 'abnormal'

export interface BaofuAccountProfile {
  legal_name?: string
  business_license_number?: string
  legal_person_name?: string
  legal_person_id_number?: string
  corporate_mobile?: string
  email?: string
  bank_account_no?: string
  account_name?: string
  real_name?: string
  certificate_no?: string
  id_card_number?: string
  bank_mobile?: string
  mobile?: string
  phone?: string
  bank_account_number?: string
  account_number?: string
  bank_name?: string
  deposit_bank_province?: string
  deposit_bank_city?: string
  deposit_bank_name?: string
  contact_name?: string
  contact_mobile?: string
  card_user_name?: string
  self_employed?: boolean
}

export interface BaofuSettlementAccountPayment {
  payment_order_id?: number
  amount?: number
  business_type?: 'baofu_account_verify_fee' | string
  out_trade_no?: string
  pay_params?: MiniProgramPayParams
  expires_at?: string
}

export interface BaofuSettlementAccountProfileDefaults {
  source?: 'wechat_applyment' | string
  legal_name?: string
  certificate_no_mask?: string
  business_license_number?: string
  legal_person_name?: string
  card_user_name?: string
  self_employed?: boolean
  legal_person_id_number_mask?: string
  corporate_mobile_mask?: string
  email_mask?: string
  bank_account_no_mask?: string
  bank_name?: string
  deposit_bank_province?: string
  deposit_bank_city?: string
  deposit_bank_name?: string
  bank_address_code?: string
  bank_branch_id?: string
  account_bank?: string
  account_bank_code?: number
  bank_alias?: string
  bank_alias_code?: string
  contact_name?: string
  contact_mobile_mask?: string
  has_legal_person_id_number?: boolean
  has_corporate_mobile?: boolean
  has_certificate_no?: boolean
  has_email?: boolean
  has_bank_account_no?: boolean
  has_contact_mobile?: boolean
  has_saved_sensitive_defaults?: boolean
}

export interface BaofuSettlementAccountResponse {
  owner_type: 'merchant' | 'platform' | 'rider' | 'operator'
  owner_id: number
  account_type: 'personal' | 'business'
  status: BaofuSettlementAccountStatus
  state: BaofuSettlementAccountStatus
  status_desc?: string
  label?: string
  payment_ready?: boolean
  open_state?: BaofuSettlementAccountOpenState
  profile_status?: BaofuSettlementAccountProfileStatus
  missing_fields?: string[]
  flow_id?: number
  flow_state?: BaofuSettlementAccountStatus
  verify_fee_amount?: number
  payment_order_id?: number
  amount?: number
  business_type?: 'baofu_account_verify_fee' | string
  out_trade_no?: string
  pay_params?: MiniProgramPayParams
  expires_at?: string
  payment?: BaofuSettlementAccountPayment
  bank_card_last4?: string
  bank_account_no_mask?: string
  bank_mobile_mask?: string
  contact_mobile_mask?: string
  email_mask?: string
  wechat_sub_mch_id_mask?: string
  profile_defaults?: BaofuSettlementAccountProfileDefaults
  submitted_at?: string
  updated_at?: string
}

export type BaofuSettlementAccountPageActionType =
  | 'submit_profile'
  | 'continue_payment'
  | 'refresh_status'
  | 'none'

export interface BaofuSettlementAccountPageAction {
  type: BaofuSettlementAccountPageActionType
  text: string
  theme: 'primary' | 'default'
  variant?: 'base' | 'outline' | 'text'
}

export interface SubmitBaofuSettlementAccountRequest {
  profile: BaofuAccountProfile
}

// --- Network calls ---

function baofuAccountEndpoint(role: BaofuAccountOwnerRole): string {
  switch (role) {
    case 'merchant':
      return '/v1/merchant/settlement-account'
    case 'operator':
      return '/v1/operators/me/settlement-account'
    case 'platform':
      return '/v1/platform/finance/settlement-account'
    default:
      return '/v1/rider/settlement-account'
  }
}

export function getBaofuSettlementAccount(role: BaofuAccountOwnerRole): Promise<BaofuSettlementAccountResponse> {
  return request({
    url: baofuAccountEndpoint(role),
    method: 'GET'
  })
}

export function submitBaofuSettlementAccountProfile(
  role: BaofuAccountOwnerRole,
  payload: SubmitBaofuSettlementAccountRequest
): Promise<BaofuSettlementAccountResponse> {
  return request({
    url: baofuAccountEndpoint(role),
    method: 'POST',
    data: payload
  })
}

export const getRiderBaofuSettlementAccount = () => getBaofuSettlementAccount('rider')

export const submitRiderBaofuSettlementAccountProfile = (profile: BaofuAccountProfile) =>
  submitBaofuSettlementAccountProfile('rider', { profile })

export const getMerchantBaofuSettlementAccount = () => getBaofuSettlementAccount('merchant')

export const submitMerchantBaofuSettlementAccountProfile = (profile: BaofuAccountProfile) =>
  submitBaofuSettlementAccountProfile('merchant', { profile })

export const getOperatorBaofuSettlementAccount = () => getBaofuSettlementAccount('operator')

export const submitOperatorBaofuSettlementAccountProfile = (profile: BaofuAccountProfile) =>
  submitBaofuSettlementAccountProfile('operator', { profile })

export const getPlatformBaofuSettlementAccount = () => getBaofuSettlementAccount('platform')

export const submitPlatformBaofuSettlementAccountProfile = (profile: BaofuAccountProfile) =>
  submitBaofuSettlementAccountProfile('platform', { profile })

// --- Re-exports for backward compatibility ---
// Consumers may continue importing from this file.

export * from './baofu-account-status'
export * from './baofu-account-view'
