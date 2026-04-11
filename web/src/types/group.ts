/**
 * 集团管理类型定义
 * 完全对齐后端 api/group.go 中的响应结构
 */

export interface GroupApplicationResponse {
  id: number;
  applicant_user_id: number;
  group_name: string;
  contact_phone: string;
  license_number?: string;
  license_image_url?: string;
  address?: string;
  region_id?: number;
  status: "draft" | "submitted" | "approved" | "rejected";
  reject_reason?: string;
  reviewed_by?: number;
  reviewed_at?: string;
  created_at: string;
  updated_at: string;
}

export interface GroupResponse {
  id: number;
  name: string;
  owner_user_id: number;
  status: "active" | "disabled";
  contact_phone?: string;
  license_number?: string;
  license_image_url?: string;
  address?: string;
  region_id?: number;
  created_at: string;
  updated_at: string;
}

export interface GroupMerchantResponse {
  id: number;
  name: string;
  logo_url?: string;
  address: string;
  phone: string;
  status: string;
}

export interface BrandResponse {
  id: number;
  group_id: number;
  name: string;
  logo_url?: string;
  description?: string;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface GroupJoinRequestResponse {
  id: number;
  group_id: number;
  merchant_id: number;
  applicant_user_id: number;
  status: "pending" | "approved" | "rejected" | "cancelled";
  reason?: string;
  reviewed_by?: number;
  reviewed_at?: string;
  created_at: string;
}

export interface GroupPoliciesResponse {
  group_id: number;
  pricing_mode: "central" | "store";
  menu_mode: "central" | "store";
  inventory_mode: "central" | "store";
  promotion_mode: "central" | "store";
}

export interface GroupTemplateResponse {
  id: number;
  group_id: number;
  version: number;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface BrandTemplateResponse {
  id: number;
  brand_id: number;
  version: number;
  status: string;
  created_at: string;
  updated_at: string;
}
