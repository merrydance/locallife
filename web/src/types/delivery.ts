export interface DeliveryPromotionResponse {
  id: number;
  merchant_id: number;
  name: string;
  min_order_amount: number; // 分
  discount_amount: number; // 分
  valid_from: string;
  valid_until: string;
  is_active: boolean;
  created_at: string;
  updated_at?: string;
}

export interface CreateDeliveryPromotionRequest {
  name: string;
  min_order_amount: number; // 分
  discount_amount: number; // 分
  valid_from: string;
  valid_until: string;
}

// Note: Backend currently doesn't have an update endpoint, but we define it for parity
export interface UpdateDeliveryPromotionRequest {
  name?: string;
  min_order_amount?: number;
  discount_amount?: number;
  valid_from?: string;
  valid_until?: string;
  is_active?: boolean;
}
