export interface PlatformProfitSharingConfigItem {
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

export interface PlatformProfitSharingConfigListResponse {
  items: PlatformProfitSharingConfigItem[];
  page: number;
  limit: number;
}
