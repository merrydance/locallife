export interface VoucherResponse {
  id: number;
  merchant_id: number;
  code: string;
  name: string;
  description?: string;
  amount: number; // 分
  min_order_amount: number; // 分
  total_quantity: number;
  claimed_quantity: number;
  used_quantity: number;
  valid_from: string;
  valid_until: string;
  is_active: boolean;
  allowed_order_types: string[];
  created_at: string;
}

export interface CreateVoucherRequest {
  name: string;
  code?: string;
  description?: string;
  amount: number; // 分
  min_order_amount: number; // 分
  total_quantity: number;
  valid_from: string;
  valid_until: string;
  allowed_order_types: string[];
}

export interface UpdateVoucherRequest {
  name?: string;
  description?: string;
  amount?: number;
  min_order_amount?: number;
  total_quantity?: number;
  valid_from?: string;
  valid_until?: string;
  is_active?: boolean;
  allowed_order_types?: string[];
}

export interface ListVouchersResponse {
  vouchers: VoucherResponse[];
  total: number;
  page_id: number;
  page_size: number;
}
