export type AdminOperatorApplicationItem = {
  id: number;
  user_id: number;
  region_id: number;
  region_name: string;
  region_code: string;
  name: string;
  contact_name: string;
  contact_phone: string;
  business_license_url: string;
  business_license_number: string;
  legal_person_name: string;
  legal_person_id_number: string;
  requested_contract_years: number;
  status: string;
  submitted_at?: string;
  created_at: string;
};

export type OperatorApplicationDetail = {
  id: number;
  user_id: number;
  /** 申请已通过且运营商实体存在时由后端返回 */
  operator_id?: number;
  region_id: number;
  region_name?: string;
  name?: string;
  contact_name?: string;
  contact_phone?: string;
  business_license_url?: string;
  business_license_number?: string;
  legal_person_name?: string;
  legal_person_id_number?: string;
  id_card_front_url?: string;
  id_card_back_url?: string;
  requested_contract_years: number;
  status: string;
  reject_reason?: string;
  submitted_at?: string;
  reviewed_at?: string;
  created_at: string;
  updated_at: string;
};

export type AdminOperatorRegionItem = {
  id: number;
  region_id: number;
  region_name: string;
  region_code: string;
  status: string;
};

export type AdminOperatorRegionsResponse = {
  regions: AdminOperatorRegionItem[];
  total: number;
};

export type AdminOperatorApplicationsResponse = {
  applications: AdminOperatorApplicationItem[];
  total: number;
  page: number;
  limit: number;
  has_more: boolean;
};

export type AdminGroupApplication = {
  id: number;
  applicant_user_id: number;
  group_name: string;
  contact_phone: string;
  license_number?: string;
  license_image_url?: string;
  address?: string;
  region_id?: number;
  status: "draft" | "submitted" | "approved" | "rejected" | string;
  reject_reason?: string;
  reviewed_by?: number;
  reviewed_at?: string;
  created_at: string;
  updated_at: string;
};

export type AdminGroupApplicationsResponse = {
  applications: AdminGroupApplication[];
  total: number;
  page: number;
  limit: number;
  has_more: boolean;
};

export type AdminRegionExpansionApplication = {
  id: number;
  operator_id: number;
  operator_name: string;
  contact_name: string;
  contact_phone: string;
  region_id: number;
  region_name: string;
  region_code: string;
  status: "pending" | "approved" | "rejected" | string;
  reject_reason?: string;
  created_at: string;
};

export type AdminRegionExpansionApplicationsResponse = {
  applications: AdminRegionExpansionApplication[];
  total: number;
  page: number;
  limit: number;
};
