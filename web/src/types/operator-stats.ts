export interface OperatorRegionResponse {
  id: number;
  name: string;
}

export interface OperatorRegionListResponse {
  regions: OperatorRegionResponse[];
  total: number;
  total_count: number;
  page: number;
  limit: number;
}

export interface OperatorRegionStatsResponse {
  region_id: number;
  region_name: string;
  merchant_count: number;
  total_orders: number;
  total_gmv: number;
  total_commission: number;
}

export interface OperatorDailyTrendRow {
  date: string;
  order_count: number;
  total_gmv: number;
  total_commission: number;
  operator_income: number;
  active_merchants: number;
  active_users: number;
}

export interface OperatorMerchantRankingRow {
  merchant_id: number;
  merchant_name: string;
  order_count: number;
  total_sales: number;
  total_commission: number;
  avg_order_amount: number;
}

export interface OperatorRiderRankingRow {
  rider_id: number;
  rider_name: string;
  delivery_count: number;
  completed_count: number;
  avg_delivery_time_seconds: number;
  total_earnings: number;
  completion_rate: number;
}

export interface OperatorFinanceOverviewResponse {
  current_month: {
    total_gmv: number;
    total_commission: number;
    operator_income: number;
    total_orders: number;
    settled_commission: number;
    pending_commission: number;
  };
  total: {
    total_gmv: number;
    total_commission: number;
    operator_income: number;
    settled_commission: number;
  };
  region_id: number;
  region_name: string;
  operator_share_ratio: number;
}
