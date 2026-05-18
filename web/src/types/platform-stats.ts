export interface PlatformOverviewResponse {
  total_orders: number;
  total_gmv: number;
  total_commission: number;
  active_merchants: number;
  active_users: number;
}

export interface PlatformRealtimeDashboardResponse {
  orders_24h: number;
  gmv_24h: number;
  active_merchants_24h: number;
  active_users_24h: number;
  pending_orders: number;
  preparing_orders: number;
  ready_orders: number;
  delivering_orders: number;
}

export interface PlatformDailyStatRow {
  date: string;
  order_count: number;
  total_gmv: number;
  total_commission: number;
  active_merchants: number;
  active_users: number;
  takeout_orders: number;
  dine_in_orders: number;
}

export interface HourlyDistributionRow {
  hour: number;
  order_count: number;
  total_gmv: number;
}

export interface PlatformProfitSharingReconciliationRow {
  status: string;
  total_orders: number;
  total_amount: number;
  total_platform_commission: number;
  total_operator_commission: number;
}

export interface PlatformBaofuDailyReconciliationRow {
  date: string;
  provider: string;
  channel: string;
  paid_amount: number;
  payment_fee: number;
  merchant_amount: number;
  rider_amount: number;
  platform_commission: number;
  operator_commission: number;
  withdraw_succeeded_amount: number;
  withdraw_processing_amount: number;
  unapplied_fact_count: number;
  unknown_command_count: number;
  fee_ledger_mismatch_count: number;
}

export interface PlatformBaofuSettlementReadiness {
  state: string;
  label: string;
  payment_ready: boolean;
}

export interface PlatformBaofuSettlementStatusResponse {
  settlement_account: PlatformBaofuSettlementReadiness;
  masked_contract_no?: string;
  masked_sharing_mer_id?: string;
}

export interface RegionComparisonRow {
  region_id: number;
  region_name: string;
  merchant_count: number;
  order_count: number;
  total_gmv: number;
  total_commission: number;
  avg_order_amount: number;
  active_users: number;
}

export interface RuleSummary {
  id: number;
  name: string;
  category: string;
  status: string;
  current_version_id: number | null;
  created_at: string;
  updated_at: string;
}

export interface RuleHitRow {
  id: number;
  rule_id: number;
  rule_version_id: number | null;
  domain: string;
  decision: string;
  reason?: string | null;
  actor_role?: string | null;
  region_id?: number | null;
  merchant_id?: number | null;
  created_at: string;
}
