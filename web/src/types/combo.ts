export interface TagInfo {
  id: number;
  name: string;
}

export interface ComboDishInfo {
  dish_id: number;
  dish_name: string;
  dish_description?: string;
  dish_image_url?: string;
  dish_price: number;
  quantity: number;
}

export interface ComboSetResponse {
  id: number;
  merchant_id: number;
  name: string;
  description: string;
  image_url: string;
  combo_price: number;
  is_available: boolean;
  is_online: boolean;
  sort_order: number;
  created_at: string;
  updated_at: string;
  
  // 详情接口可能会返回这些
  dishes?: ComboDishInfo[];
  tags?: TagInfo[];
}

export interface ListCombosResponse {
  combo_sets: ComboSetResponse[];
  total: number;
}

// 创建/更新套餐时的请求体
export interface CreateComboRequest {
  name: string;
  description?: string;
  image_url?: string;
  combo_price: number;
  is_online?: boolean;  // 默认为 true
  is_available?: boolean; // 默认为 true
  sort_order?: number;
  
  // 关联数据
  dishes?: { dish_id: number; quantity?: number }[];
  tag_ids?: number[];
}

export interface UpdateComboRequest {
  name?: string;
  description?: string;
  image_url?: string;
  combo_price?: number;
  is_online?: boolean;
  is_available?: boolean;
  sort_order?: number;
  
  dishes?: { dish_id: number; quantity?: number }[];
  tag_ids?: number[];
}
