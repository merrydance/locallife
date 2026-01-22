export interface InventoryItem {
  id: number;
  merchant_id: number;
  dish_id: number;
  dish_name: string;
  dish_price: number;
  date: string;
  total_quantity: number; // -1 表示无限库存
  sold_quantity: number;
  reserved_quantity: number;
  available: number;
}

export interface InventoryStats {
  total_dishes: number;
  unlimited_dishes: number;
  sold_out_dishes: number;
  available_dishes: number;
}

export interface ListInventoryResponse {
  inventories: InventoryItem[];
}

export interface UpdateInventoryRequest {
  dish_id: number;
  date: string;
  total_quantity?: number;
  sold_quantity?: number;
}
