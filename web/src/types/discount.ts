export interface DiscountResponse {
  id: number;
  merchant_id: number;
  name: string;
  description?: string;
  min_order_amount: number; // 分
  discount_amount: number; // 分
  can_stack_with_voucher: boolean;
  can_stack_with_membership: boolean;
  valid_from: string;
  valid_until: string;
  is_active: boolean;
  created_at: string;
}

export interface CreateDiscountRequest {
  name: string;
  description?: string;
  min_order_amount: number; // 分
  discount_amount: number; // 分
  can_stack_with_voucher: boolean;
  can_stack_with_membership: boolean;
  valid_from: string;
  valid_until: string;
}

export interface UpdateDiscountRequest {
  id: number;
  name?: string;
  description?: string;
  min_order_amount?: number;
  discount_amount?: number;
  can_stack_with_voucher?: boolean;
  can_stack_with_membership?: boolean;
  valid_from?: string;
  valid_until?: string;
  is_active?: boolean;
}

export interface ListDiscountsResponse {
  rules: DiscountResponse[];
  total_count: number;
  total: number;
  page_id: number;
  page_size: number;
}
