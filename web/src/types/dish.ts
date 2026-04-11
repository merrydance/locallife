export interface TagInfo {
  id: number;
  name: string;
}

export interface CustomizationOption {
  id: number;
  tag_id: number;
  tag_name: string;
  extra_price: number;
  sort_order: number;
}

export interface CustomizationGroup {
  id: number;
  name: string;
  is_required: boolean;
  sort_order: number;
  options: CustomizationOption[];
}

export interface Ingredient {
  id: number;
  name: string;
  category: string;
  is_allergen: boolean;
}

export interface DishResponse {
  id: number;
  merchant_id: number;
  category_id?: number;
  category_name?: string;
  name: string;
  description: string;
  image_url: string;
  image_asset_id?: number;
  price: number;
  member_price?: number;
  is_available: boolean;
  is_online: boolean;
  sort_order: number;
  prepare_time: number;
  ingredients?: Ingredient[];
  tags?: TagInfo[];
  customization_groups?: CustomizationGroup[];
}

export interface ListDishesResponse {
  dishes: DishResponse[];
  total: number;
}

export interface DishCategory {
  id: number;
  name: string;
  sort_order: number;
}

export interface ListDishCategoriesResponse {
  categories: DishCategory[];
}

export interface CreateDishRequest {
  name: string;
  description?: string;
  image_url?: string;
  image_asset_id?: number;
  price: number;
  member_price?: number;
  category_id?: number;
  is_online?: boolean;
  is_available?: boolean;
  sort_order?: number;
  prepare_time?: number;
  tag_ids?: number[];
  customization_groups?: {
    name: string;
    is_required: boolean;
    sort_order: number;
    options: {
      tag_id: number;
      extra_price: number;
      sort_order: number;
    }[];
  }[];
}

export type UpdateDishRequest = Partial<CreateDishRequest>;
