export type ApplymentAccountType =
  | "ACCOUNT_TYPE_BUSINESS"
  | "ACCOUNT_TYPE_PRIVATE";

export interface ApplymentBankOption {
  bank_alias: string;
  bank_alias_code: string;
  account_bank: string;
  account_bank_code: number;
  need_bank_branch: boolean;
}

export interface ApplymentProvinceOption {
  province_name: string;
  province_code: number;
}

export interface ApplymentCityOption {
  city_name: string;
  city_code: number;
}

export interface ApplymentBranchOption {
  bank_branch_name: string;
  bank_branch_id: string;
}

export interface ApplymentBankListResponse {
  banks: ApplymentBankOption[];
  total: number;
  refreshed_at: string;
}

export interface ApplymentBankSearchResponse {
  matches: ApplymentBankOption[];
  total: number;
  refreshed_at: string;
}

export interface ApplymentProvinceListResponse {
  provinces: ApplymentProvinceOption[];
  total: number;
  refreshed_at: string;
}

export interface ApplymentCityListResponse {
  cities: ApplymentCityOption[];
  total: number;
  refreshed_at: string;
}

export interface ApplymentBranchListResponse {
  branches: ApplymentBranchOption[];
  total: number;
  account_bank: string;
  account_bank_code: number;
  bank_alias: string;
  bank_alias_code: string;
  refreshed_at: string;
}

export interface ApplymentBindBankPayload {
  account_type: ApplymentAccountType;
  account_bank: string;
  account_bank_code?: number;
  bank_alias?: string;
  bank_alias_code?: string;
  need_bank_branch?: boolean;
  bank_address_code?: string;
  bank_branch_id?: string;
  bank_name?: string;
  account_number: string;
  account_name: string;
  contact_phone: string;
  contact_email?: string;
}