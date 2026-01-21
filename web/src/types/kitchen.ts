export type KitchenOrderItem = {
  customizations?: {
    group_name: string;
    option_name: string;
    price_adjustment: number;
  }[];
  id: number;
  image_url: string;
  name: string;
  prepare_time: number;
  quantity: number;
};

export type KitchenOrderResponse = {
  created_at: string;
  estimated_ready_at?: string;
  id: number;
  is_urged: boolean;
  items: KitchenOrderItem[];
  notes?: string;
  order_no: string;
  order_type: string;
  paid_at?: string;
  pickup_number?: string;
  status: string;
  table_no?: string;
  table_number?: string;
  waiting_minutes: number;
  preparing_started_at?: string;
  ready_at?: string;
  customer_name?: string;
  estimated_time?: number;
};

export type KitchenStats = {
  avg_prepare_time?: number;
  completed_today_count?: number;
  new_count?: number;
  preparing_count?: number;
  ready_count?: number;
  total_pending?: number;
  avg_preparation_time?: number;
  orders_behind_schedule?: number;
};

export type KitchenOrdersResponse = {
  new_orders: KitchenOrderResponse[];
  preparing_orders: KitchenOrderResponse[];
  ready_orders: KitchenOrderResponse[];
  stats?: KitchenStats;
};
