import { request } from '../utils/request'

export type ApplymentAccountType = 'ACCOUNT_TYPE_BUSINESS' | 'ACCOUNT_TYPE_PRIVATE'
export type ApplymentContactType = 'LEGAL' | 'SUPER'
export type ApplymentContactDocType = 'IDENTIFICATION_TYPE_MAINLAND_IDCARD'

export interface ApplymentBankOption {
  bank_alias: string
  bank_alias_code: string
  account_bank: string
  account_bank_code: number
  need_bank_branch: boolean
}

export interface ApplymentProvinceOption {
  province_name: string
  province_code: number
}

export interface ApplymentCityOption {
  city_name: string
  city_code: number
}

export interface ApplymentBranchOption {
  bank_branch_name: string
  bank_branch_id: string
}

export interface ApplymentBankListResponse {
  banks: ApplymentBankOption[]
  total: number
  refreshed_at: string
}

export interface ApplymentBankSearchResponse {
  matches: ApplymentBankOption[]
  total: number
  refreshed_at: string
}

export interface ApplymentProvinceListResponse {
  provinces: ApplymentProvinceOption[]
  total: number
  refreshed_at: string
}

export interface ApplymentCityListResponse {
  cities: ApplymentCityOption[]
  total: number
  refreshed_at: string
}

export interface ApplymentBranchListResponse {
  branches: ApplymentBranchOption[]
  total: number
  account_bank: string
  account_bank_code: number
  bank_alias: string
  bank_alias_code: string
  refreshed_at: string
}

export interface ApplymentBindBankPayload {
  account_type: ApplymentAccountType
  account_bank: string
  account_bank_code?: number
  bank_alias?: string
  bank_alias_code?: string
  need_bank_branch?: boolean
  bank_address_code?: string
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

export function listApplymentBanks(apiBasePath: string, accountType: ApplymentAccountType) {
  return request<ApplymentBankListResponse>({
    url: `${apiBasePath}/banks`,
    method: 'GET',
    data: {
      account_type: accountType
    }
  })
}

export function searchApplymentBanksByAccount(apiBasePath: string, accountType: ApplymentAccountType, accountNumber: string) {
  return request<ApplymentBankSearchResponse>({
    url: `${apiBasePath}/banks/search-by-bank-account`,
    method: 'GET',
    data: {
      account_type: accountType,
      account_number: accountNumber
    }
  })
}

export function listApplymentProvinces(apiBasePath: string) {
  return request<ApplymentProvinceListResponse>({
    url: `${apiBasePath}/areas/provinces`,
    method: 'GET'
  })
}

export function listApplymentCities(apiBasePath: string, provinceCode: number) {
  return request<ApplymentCityListResponse>({
    url: `${apiBasePath}/areas/provinces/${provinceCode}/cities`,
    method: 'GET'
  })
}

export function listApplymentBankBranches(apiBasePath: string, bankAliasCode: string, cityCode: number) {
  return request<ApplymentBranchListResponse>({
    url: `${apiBasePath}/banks/${bankAliasCode}/branches`,
    method: 'GET',
    data: {
      city_code: cityCode
    }
  })
}
