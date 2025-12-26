// 基于swagger.json重构的菜品模型

/**
 * 菜品视图模型 - 用于UI展示
 */
export interface Dish {
    id: number  // 改为number类型，对齐后端
    name: string
    imageUrl: string
    price: number  // 单位：分
    priceDisplay: string
    shopName: string
    merchantId: number  // 改为number类型
    attributes: string[]
    spicyLevel: number
    salesBadge: string
    ratingDisplay: string
    distance: string
    deliveryTimeDisplay: string
    deliveryFeeDisplay: string
    discountRule: string
    tags: string[]
    isPremade: boolean
    customization_groups?: CustomizationGroup[]  // 改为对齐swagger
    distance_meters?: number
    member_price?: number  // 会员价
    is_available?: boolean  // 是否可用
    prepare_time?: number  // 制作时间
}

/**
 * 定制化分组 - 对齐swagger api.customizationGroup
 */
export interface CustomizationGroup {
    id: number
    name: string
    is_required: boolean
    max_selections: number
    options: CustomizationOption[]
}

/**
 * 定制化选项 - 对齐swagger api.customizationOption
 */
export interface CustomizationOption {
    id: number
    name: string
    price_adjustment: number  // 价格调整（分）
}

/**
 * 菜品响应DTO - 对齐swagger api.dishResponse
 */
export interface DishResponse {
    id: number
    name: string
    description: string
    price: number  // 单位：分
    member_price?: number  // 会员价（分）
    image_url: string
    category_id: number
    category_name: string
    merchant_id: number
    is_available: boolean
    is_online: boolean
    prepare_time: number  // 预估制作时间（分钟）
    sort_order: number
    customization_groups: CustomizationGroup[]
    ingredients: Ingredient[]
    tags: TagInfo[]
}

/**
 * 菜品摘要DTO - 对齐swagger api.dishSummary (用于Feed流)
 */
export interface DishSummary {
    id: number
    name: string
    price: number
    member_price?: number
    image_url: string
    merchant_id: number
    merchant_name: string
    merchant_logo: string
    merchant_latitude: number
    merchant_longitude: number
    merchant_region_id: number
    distance: number
    estimated_delivery_fee: number
    monthly_sales: number
    is_available: boolean
    tags: string[]
}

/**
 * 配料信息
 */
export interface Ingredient {
    id: number
    name: string
    allergen?: boolean
}

/**
 * 标签信息
 */
export interface TagInfo {
    id: number
    name: string
    color?: string
}

// 兼容性：保留旧的接口名称
export type DishDTO = DishResponse
export type DishSpecGroup = CustomizationGroup
export type DishSpec = CustomizationOption
