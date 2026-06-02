export type ApplymentAccountType = 'ACCOUNT_TYPE_BUSINESS' | 'ACCOUNT_TYPE_PRIVATE'
export type ApplymentContactType = 'LEGAL' | 'SUPER'
export type ApplymentContactDocType = 'IDENTIFICATION_TYPE_MAINLAND_IDCARD'

export interface ApplymentBindBankPayload {
  account_type: ApplymentAccountType
  account_bank: string
  account_bank_code?: number
  bank_alias?: string
  bank_alias_code?: string
  need_bank_branch?: boolean
  bank_address_code?: string
  manual_bank_city?: string
  deposit_bank_province?: string
  deposit_bank_city?: string
  bank_branch_id?: string
  bank_name?: string
  account_number: string
  account_name?: string
  contact_email?: string
  contact_type?: ApplymentContactType
  contact_name?: string
  contact_id_doc_type?: ApplymentContactDocType
  contact_id_card_number?: string
  contact_id_doc_copy_asset_id?: number
  contact_id_doc_copy_back_asset_id?: number
  contact_id_doc_period_begin?: string
  contact_id_doc_period_end?: string
}

export interface ApplymentBindBankDraftPayload extends ApplymentBindBankPayload {
  contact_id_doc_copy_url?: string
  contact_id_doc_copy_raw_url?: string
  contact_id_doc_copy_back_url?: string
  contact_id_doc_copy_back_raw_url?: string
}
