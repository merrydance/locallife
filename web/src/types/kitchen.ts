export type KitchenOrderItem = {
  id: number;
  name: string;
  category_name?: string;
  quantity: number;
  customizations?: {
    name: string;
    value: string;
    extra_price?: number;
  }[];
  image_url?: string;
  prepare_time: number; // 预估制作时间（分钟）
};

export type KitchenOrderResponse = {
  id: number;
  order_no: string;
  order_type: string;
  status: string;
  table_no?: string;
  pickup_number?: string;
  notes?: string;
  items: KitchenOrderItem[];
  estimated_ready_at?: string;
  waiting_minutes: number;
  is_urged: boolean;
  created_at: string;
  paid_at: string;
};

export type KitchenStats = {
  new_count: number;
  preparing_count: number;
  ready_count: number;
  completed_today_count: number;
  avg_prepare_time: number;
};

export type KitchenOrdersResponse = {
  new_orders: KitchenOrderResponse[];
  preparing_orders: KitchenOrderResponse[];
  ready_orders: KitchenOrderResponse[];
  stats: KitchenStats;
};
