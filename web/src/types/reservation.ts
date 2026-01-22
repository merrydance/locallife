export type ReservationStatus =
  | "pending"     // 待支付
  | "paid"        // 已支付
  | "confirmed"   // 已确认
  | "checked_in"  // 已签到
  | "completed"   // 已完成
  | "cancelled"   // 已取消
  | "expired"     // 已过期
  | "no_show";    // 未到店

export type PaymentMode = "deposit" | "full";

export type ReservationItem = {
  id: number;
  reservation_id: number;
  dish_id?: number;
  combo_id?: number;
  name: string;
  image_url?: string;
  quantity: number;
  unit_price: number;
  total_price: number;
  type: "dish" | "combo";
};

export type ReservationResponse = {
  id: number;
  table_id: number;
  table_no?: string;
  table_type?: string;
  user_id: number;
  merchant_id: number;
  merchant_name?: string;
  merchant_address?: string;
  merchant_phone?: string;
  reservation_date: string;
  reservation_time: string;
  guest_count: number;
  contact_name: string;
  contact_phone: string;
  payment_mode: PaymentMode;
  deposit_amount: number;
  prepaid_amount: number;
  refund_deadline: string;
  payment_deadline: string;
  status: ReservationStatus;
  notes?: string;
  paid_at?: string;
  confirmed_at?: string;
  completed_at?: string;
  cancelled_at?: string;
  cancel_reason?: string;
  items?: ReservationItem[];
  created_at: string;
  updated_at?: string;
};

export type ReservationStatsResponse = {
  pending_count: number;
  paid_count: number;
  confirmed_count: number;
  checked_in_count?: number;
  completed_count: number;
  cancelled_count: number;
  expired_count: number;
  no_show_count: number;
};
