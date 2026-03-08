export interface OperatorRealtimeStatsResponse {
  active_merchant_count: number;
  active_rider_count: number;
  pending_merchant_count: number;
  pending_rider_count: number;
}

export interface OperatorMerchantListItem {
  id: number;
  name: string;
  phone: string;
  address: string;
  status: string;
  is_open: boolean;
  owner_user_id: number;
  region_id: number;
  latitude: number;
  longitude: number;
  created_at: string;
}

export interface OperatorMerchantListResponse {
  merchants: OperatorMerchantListItem[];
  total: number;
  total_count: number;
  page: number;
  limit: number;
}

export interface OperatorMerchantDetail {
  id: number;
  name: string;
  description: string;
  logo_url: string;
  phone: string;
  address: string;
  status: string;
  is_open: boolean;
  owner_user_id: number;
  region_id: number;
  latitude: number;
  longitude: number;
  version: number;
  created_at: string;
  updated_at: string;
}

export interface OperatorMerchantStatsDish {
  dish_name: string;
  total_sold: number;
  total_revenue: number;
}

export interface OperatorMerchantStats {
  days: number;
  total_orders: number;
  total_sales: number;
  total_commission: number;
  avg_daily_sales: number;
  total_customers: number;
  repeat_customers: number;
  repurchase_rate_basis_points: number;
  avg_orders_per_user_cents: number;
  top_dishes: OperatorMerchantStatsDish[];
}

export interface OperatorRiderListItem {
  id: number;
  user_id: number;
  real_name: string;
  phone: string;
  status: string;
  is_online: boolean;
  region_id: number;
  deposit_amount: number;
  total_orders: number;
  total_earnings: number;
  created_at: string;
}

export interface OperatorRiderListResponse {
  riders: OperatorRiderListItem[];
  total: number;
  total_count: number;
  page: number;
  limit: number;
}

export interface OperatorRiderDetail {
  id: number;
  user_id: number;
  real_name: string;
  phone: string;
  id_card_no: string;
  status: string;
  is_online: boolean;
  region_id: number;
  deposit_amount: number;
  frozen_deposit: number;
  total_orders: number;
  total_earnings: number;
  current_latitude: number;
  current_longitude: number;
  location_updated_at?: string;
  credit_score: number;
  high_value_qualified: boolean;
  created_at: string;
  updated_at: string;
}

export interface OperatorAppealListItem {
  id: number;
  claim_id: number;
  claim_type: string;
  claim_amount: number;
  claim_description: string;
  order_no: string;
  merchant_id: number;
  merchant_name: string;
  appellant_type: string;
  appellant_id: number;
  appellant_name: string;
  reason: string;
  status: string;
  created_at: string;
  reviewed_at?: string;
}

export interface OperatorAppealListResponse {
  appeals: OperatorAppealListItem[];
  total: number;
  total_count: number;
  page: number;
  limit: number;
}

export interface OperatorAppealDetail {
  id: number;
  claim_id: number;
  claim_type: string;
  claim_amount: number;
  claim_description: string;
  claim_status: string;
  claim_created_at: string;
  order_no: string;
  order_amount: number;
  order_status: string;
  order_created_at: string;
  merchant_id: number;
  merchant_name: string;
  merchant_phone: string;
  user_phone?: string;
  user_name: string;
  appellant_type: string;
  appellant_id: number;
  reason: string;
  status: string;
  region_id: number;
  created_at: string;
  reviewed_at?: string;
  reviewer_id?: number;
  review_notes?: string;
  compensation_amount?: number;
  claim_approved_amount?: number;
  lookback_result?: string;
}

export interface ClaimRecoveryResponse {
  id: number;
  claim_id: number;
  order_id: number;
  responsible_party: string;
  recovery_target?: string;
  recovery_amount: number;
  status: string;
  due_at: string;
  updated_at: string;
}

export interface SafetyReportItem {
  id: number;
  reporter_id: number;
  region_id: number;
  title: string;
  description: string;
  level: string;
  status: string;
  merchant_ids: number[];
  images: string[];
  resolution_notes?: string;
  created_at: string;
  updated_at: string;
}

export interface SafetyReportListResponse {
  items: SafetyReportItem[];
  page: number;
  limit: number;
  has_more: boolean;
  total: number;
}

export interface PeakHourConfigResponse {
  id: number;
  region_id: number;
  start_time: string;
  end_time: string;
  coefficient: number;
  days_of_week: number[];
  is_active: boolean;
  created_at: string;
  updated_at?: string;
}

export interface OperatorCommissionItem {
  date: string;
  order_count: number;
  total_gmv: number;
  commission_rate: string;
  commission: number;
}

export interface OperatorCommissionResponse {
  items: OperatorCommissionItem[];
  total: number;
  total_count: number;
  page: number;
  limit: number;
  summary: {
    total_gmv: number;
    total_commission: number;
    total_orders: number;
  };
}

export interface OperatorAccountBalanceResponse {
  sub_mch_id: string;
  available_amount: number;
  pending_amount: number;
  withdrawable_amount: number;
}

export interface OperatorWithdrawalItem {
  id: number;
  amount: number;
  status: string;
  channel: string;
  out_request_no?: string;
  withdraw_id?: string;
  sub_mch_id?: string;
  reason?: string;
  created_at: string;
  updated_at: string;
}

export interface OperatorWithdrawalsResponse {
  withdrawals: OperatorWithdrawalItem[];
  total: number;
  total_count: number;
  page: number;
  limit: number;
  total_pages: number;
}

export interface OperatorApplymentStatusResponse {
  status: string;
  status_desc: string;
  applyment_id?: number;
  sub_mch_id?: string;
  sign_url?: string;
  reject_reason?: string;
  created_at: string;
  updated_at: string;
}

export interface OperatorApplicationStatusResponse {
  status: string;
  reject_reason?: string;
  created_at: string;
  updated_at: string;
}

export interface OperatorBindBankRequest {
  account_type: "ACCOUNT_TYPE_BUSINESS" | "ACCOUNT_TYPE_PRIVATE";
  account_bank: string;
  bank_address_code: string;
  bank_name?: string;
  account_number: string;
  account_name: string;
  contact_phone: string;
  contact_email?: string;
}

export interface OperatorBindBankResponse {
  applyment_id: number;
  status: string;
  message: string;
}

export interface OperatorRuleItem {
  id: string;
  name: string;
  key: string;
  value: string;
  unit: string;
  desc: string;
  category: string;
  editable: boolean;
  action?: string;
}

export interface OperatorRulesResponse {
  rules: OperatorRuleItem[];
}

export interface RuleEngineItem {
  id: number;
  name: string;
  category: string;
  status: string;
  current_version_id?: number | null;
  created_at: string;
  updated_at: string;
}

export interface RuleEngineListResponse {
  rules: RuleEngineItem[];
  count: number;
}

export interface RuleEngineVersion {
  id: number;
  rule_id: number;
  version: number;
  status: string;
  priority: number;
  scope: string;
  condition: string;
  action: string;
  gray_config: string;
  created_at: string;
  updated_at: string;
}

export interface RuleEngineDetailResponse {
  rule: RuleEngineItem;
  versions: RuleEngineVersion[];
}

export interface RuleHitItem {
  id: number;
  rule_id: number;
  rule_version_id?: number | null;
  domain: string;
  decision: string;
  created_at: string;
}

export interface RuleHitsResponse {
  hits: RuleHitItem[];
  count: number;
}

export interface ProfitSharingConfigItem {
  id: number;
  status: string;
  order_source: string;
  region_id?: number;
  merchant_id?: number;
  platform_rate: number;
  operator_rate: number;
  rider_enabled: boolean;
  priority: number;
  effective_at?: string;
  expires_at?: string;
  created_by?: number;
  created_at: string;
  updated_at: string;
}

export interface OperatorProfitSharingConfigListResponse {
  items: ProfitSharingConfigItem[];
  page: number;
  limit: number;
}