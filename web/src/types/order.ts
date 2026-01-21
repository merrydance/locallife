export type OrderItemCustomization = {
  name: string;
  value: string;
  extra_price?: number;
};

export type OrderItemResponse = {
  combo_id?: number;
  customizations?: OrderItemCustomization[];
  dish_id?: number;
  id: number;
  image_url: string;
  name: string;
  quantity: number;
  subtotal: number;
  unit_price: number;
};

export type OrderResponse = {
  id: number;
  order_no: string;
  order_type: "takeout" | "dine_in" | "takeaway" | "reservation";
  status:
    | "pending"
    | "paid"
    | "preparing"
    | "ready"
    | "courier_accepted"
    | "picked"
    | "delivering"
    | "rider_delivered"
    | "user_delivered"
    | "completed"
    | "cancelled";
  user_id: number;
  merchant_id: number;
  merchant_name: string;
  table_id?: number;
  address_id?: number;
  reservation_id?: number;
  items: OrderItemResponse[];
  subtotal: number;
  delivery_fee: number;
  delivery_fee_discount: number;
  discount_amount: number;
  total_amount: number;
  payment_method: "wechat" | "balance";
  notes?: string;
  delivery_distance?: number;
  created_at: string;
  paid_at?: string;
  completed_at?: string;
  cancelled_at?: string;
  cancel_reason?: string;
  updated_at: string;
};

export type OrderStatsResponse = {
  pending_count: number;
  paid_count: number;
  preparing_count: number;
  ready_count: number;
  delivering_count: number;
  completed_count: number;
  cancelled_count: number;
};
