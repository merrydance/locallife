import type {
  BaofuAccountOwnerRole,
  BaofuAccountProfile,
  BaofuSettlementAccountProfileDefaults
} from '../api/baofu-account'
import type {
  ApplymentBindBankPayload
} from '../api/applyment-bank'

export type BaofuEnterpriseProfileField =
  | 'legal_name'
  | 'business_license_number'
  | 'legal_person_name'
  | 'legal_person_id_number'
  | 'corporate_mobile'
  | 'email'

export interface BaofuEnterpriseProfileForm {
  legal_name: string
  business_license_number: string
  legal_person_name: string
  legal_person_id_number: string
  corporate_mobile: string
  email: string
}

export interface BaofuEnterpriseStoredFlags {
  hasStoredLegalPersonID: boolean
  hasStoredCorporateMobile: boolean
  hasStoredEmail: boolean
  hasStoredBankAccount: boolean
}

export type BaofuPersonalProfileField =
  | 'name'
  | 'certificate_no'
  | 'bank_account_no'
  | 'bank_mobile'

export interface BaofuPersonalProfileForm {
  name: string
  certificate_no: string
  bank_account_no: string
  bank_mobile: string
}

function normalizeText(value?: string | null): string {
  return typeof value === 'string' ? value.trim() : ''
}

export function emptyBaofuEnterpriseProfileForm(): BaofuEnterpriseProfileForm {
  return {
    legal_name: '',
    business_license_number: '',
    legal_person_name: '',
    legal_person_id_number: '',
    corporate_mobile: '',
    email: ''
  }
}

export function emptyBaofuPersonalProfileForm(): BaofuPersonalProfileForm {
  return {
    name: '',
    certificate_no: '',
    bank_account_no: '',
    bank_mobile: ''
  }
}

export function getBaofuEnterpriseStoredFlags(
  defaults?: BaofuSettlementAccountProfileDefaults | null
): BaofuEnterpriseStoredFlags {
  return {
    hasStoredLegalPersonID: Boolean(defaults?.has_legal_person_id_number),
    hasStoredCorporateMobile: Boolean(defaults?.has_corporate_mobile),
    hasStoredEmail: Boolean(defaults?.has_email),
    hasStoredBankAccount: Boolean(defaults?.has_bank_account_no)
  }
}

export function buildBaofuEnterpriseFormFromDefaults(
  defaults?: BaofuSettlementAccountProfileDefaults | null
): BaofuEnterpriseProfileForm {
  return {
    legal_name: normalizeText(defaults?.legal_name),
    business_license_number: normalizeText(defaults?.business_license_number),
    legal_person_name: normalizeText(defaults?.legal_person_name),
    legal_person_id_number: '',
    corporate_mobile: '',
    email: ''
  }
}

export function buildBaofuEnterpriseBankDraftFromDefaults(
  defaults?: BaofuSettlementAccountProfileDefaults | null
): Partial<ApplymentBindBankPayload> {
  const fallbackAccountType = 'ACCOUNT_TYPE_BUSINESS' as const
  if (!defaults) {
    return { account_type: fallbackAccountType }
  }

  const selfEmployed = Boolean(defaults.self_employed)
  const hasBranch = Boolean(defaults.bank_branch_id)
  const depositBankProvince = normalizeText(defaults.deposit_bank_province)
  const depositBankCity = normalizeText(defaults.deposit_bank_city)
  const depositBankName = normalizeText(defaults.deposit_bank_name)
  return {
    account_type: selfEmployed ? 'ACCOUNT_TYPE_PRIVATE' : 'ACCOUNT_TYPE_BUSINESS',
    account_bank: normalizeText(defaults.account_bank || defaults.bank_name),
    account_bank_code: defaults.account_bank_code || 0,
    bank_alias: normalizeText(defaults.bank_alias || defaults.bank_name),
    bank_alias_code: normalizeText(defaults.bank_alias_code),
    need_bank_branch: hasBranch || Boolean(depositBankProvince || depositBankCity || depositBankName),
    bank_address_code: hasBranch ? normalizeText(defaults.bank_address_code) : depositBankProvince,
    manual_bank_city: depositBankCity,
    bank_branch_id: hasBranch ? normalizeText(defaults.bank_branch_id) : depositBankName,
    bank_name: depositBankName,
    account_number: '',
    account_name: normalizeText(selfEmployed ? (defaults.card_user_name || defaults.legal_person_name) : defaults.legal_name),
    contact_email: ''
  }
}

export function buildBaofuEnterpriseProfilePayload(
  form: BaofuEnterpriseProfileForm,
  bank: ApplymentBindBankPayload,
  defaults?: BaofuSettlementAccountProfileDefaults | null
): BaofuAccountProfile {
  const flags = getBaofuEnterpriseStoredFlags(defaults)
  const payload: BaofuAccountProfile = {
    legal_name: form.legal_name.trim(),
    business_license_number: form.business_license_number.trim(),
    legal_person_name: form.legal_person_name.trim(),
    legal_person_id_number: form.legal_person_id_number.trim(),
    corporate_mobile: form.corporate_mobile.trim(),
    email: form.email.trim(),
    bank_account_no: normalizeText(bank.account_number),
    bank_name: normalizeText(bank.account_bank || bank.bank_alias || defaults?.bank_name),
    deposit_bank_province: normalizeText(bank.deposit_bank_province || defaults?.deposit_bank_province),
    deposit_bank_city: normalizeText(bank.deposit_bank_city || defaults?.deposit_bank_city),
    deposit_bank_name: normalizeText(bank.bank_name || bank.account_bank || bank.bank_alias || defaults?.deposit_bank_name)
  }

  if (bank.account_type === 'ACCOUNT_TYPE_PRIVATE') {
    payload.self_employed = true
    payload.card_user_name = normalizeText(bank.account_name || form.legal_person_name || defaults?.legal_person_name)
  } else {
    payload.self_employed = false
  }

  if (!payload.legal_person_id_number && flags.hasStoredLegalPersonID) {
    delete payload.legal_person_id_number
  }
  if (!payload.corporate_mobile && flags.hasStoredCorporateMobile) {
    delete payload.corporate_mobile
  }
  if (!payload.email && flags.hasStoredEmail) {
    delete payload.email
  }
  if (!payload.bank_account_no && flags.hasStoredBankAccount) {
    delete payload.bank_account_no
  }
  return payload
}

export function validateBaofuEnterpriseProfileForm(
  role: Extract<BaofuAccountOwnerRole, 'merchant' | 'platform'>,
  form: BaofuEnterpriseProfileForm,
  bankDraft: Partial<ApplymentBindBankPayload> | null,
  defaults?: BaofuSettlementAccountProfileDefaults | null
): string {
  const flags = getBaofuEnterpriseStoredFlags(defaults)
  if (!form.legal_name.trim()) {
    return role === 'platform' ? '请输入平台主体名称' : '请输入商户主体名称'
  }
  if (!form.business_license_number.trim()) {
    return '请输入营业执照号'
  }
  if (!form.legal_person_name.trim()) {
    return '请输入法人姓名'
  }
  if (!flags.hasStoredLegalPersonID && !/(^\d{15}$)|(^\d{17}[\dXx]$)/.test(form.legal_person_id_number.trim())) {
    return '请输入正确法人身份证号'
  }
  if (bankDraft?.account_type === 'ACCOUNT_TYPE_PRIVATE' && !flags.hasStoredCorporateMobile && !/^1\d{10}$/.test(form.corporate_mobile.trim())) {
    return '请输入正确法人手机号'
  }
  if (!flags.hasStoredEmail && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(form.email.trim())) {
    return '请输入正确联系邮箱'
  }
  return ''
}

export function buildBaofuPersonalFormFromDefaults(
  current: BaofuPersonalProfileForm,
  defaults?: BaofuSettlementAccountProfileDefaults | null
): BaofuPersonalProfileForm {
  return {
    ...current,
    name: normalizeText(defaults?.legal_name || defaults?.legal_person_name || current.name)
  }
}

export function buildBaofuPersonalProfilePayload(
  role: Extract<BaofuAccountOwnerRole, 'operator' | 'rider'>,
  form: BaofuPersonalProfileForm
): BaofuAccountProfile {
  if (role === 'rider') {
    const payload: BaofuAccountProfile = {
      real_name: form.name.trim(),
      mobile: form.bank_mobile.trim(),
      id_card_number: form.certificate_no.trim(),
      bank_account_number: form.bank_account_no.trim()
    }
    if (!payload.id_card_number) {
      delete payload.id_card_number
    }
    return payload
  }

  const payload: BaofuAccountProfile = {
    legal_name: form.name.trim(),
    certificate_no: form.certificate_no.trim(),
    bank_account_no: form.bank_account_no.trim(),
    bank_mobile: form.bank_mobile.trim()
  }
  if (!payload.certificate_no) {
    delete payload.certificate_no
  }
  return payload
}

export function validateBaofuPersonalProfileForm(
  form: BaofuPersonalProfileForm,
  defaults?: BaofuSettlementAccountProfileDefaults | null
): string {
  if (!form.name.trim()) {
    return '请输入姓名'
  }
  if (!defaults?.has_certificate_no && !/(^\d{15}$)|(^\d{17}[\dXx]$)/.test(form.certificate_no.trim())) {
    return '请输入正确身份证号'
  }
  if (!/^\d{8,30}$/.test(form.bank_account_no.trim())) {
    return '请输入正确银行卡号'
  }
  if (!/^1\d{10}$/.test(form.bank_mobile.trim())) {
    return '请输入银行预留手机号'
  }
  return ''
}
