/**
 * 菜品和套餐管理接口
 * 基于swagger.json完全重构，仅保留后端支持的接口
 */

import { request, API_BASE } from '../utils/request'
import { getToken } from '../utils/auth'

// ==================== 菜品数据类型定义 ====================

/**
 * 菜品完整信息响应 - 完全对齐 api.dishResponse
 */
export interface DishResponse {
    id: number                                   // 菜品ID
    merchant_id: number                          // 商户ID
    name: string                                 // 菜品名称
    description: string                          // 菜品描述
    price: number                                // 价格（分）
    member_price?: number                        // 会员价（分）
    image_url: string                            // 菜品图片URL
    category_id: number                          // 分类ID
    category_name: string                        // 分类名称
    is_online: boolean                           // 是否上架
    is_available: boolean                        // 是否可用
    prepare_time?: number                        // 预估制作时间（分钟）
    sort_order: number                           // 排序
    customization_groups?: CustomizationGroup[]  // 定制化分组
    ingredients?: Ingredient[]                   // 配料信息
    tags?: TagInfo[]                             // 标签信息
}

/**
 * 菜品摘要信息 - 对齐 api.dishSummary（用于搜索和推荐）
 */
export interface DishSummary {
    id: number                                   // 菜品ID
    merchant_id: number                          // 商户ID
    name: string                                 // 菜品名称
    price: number                                // 价格（分）
    member_price?: number                        // 会员价（分）
    image_url: string                            // 菜品图片URL
    is_available: boolean                        // 是否可用
    tags?: string[]                              // 标签
    merchant_name: string                        // 商户名称
    merchant_logo: string                        // 商户Logo
    merchant_latitude: number                    // 商户纬度
    merchant_longitude: number                   // 商户经度
    merchant_region_id: number                   // 商户区域ID
    distance?: number                            // 距离（米）
    estimated_delivery_time?: number             // 预估配送时间（秒）
    estimated_delivery_fee?: number              // 预估配送费（分）
    monthly_sales?: number                       // 近30天销量
}

/**
 * 定制化分组 - 对齐 api.customizationGroup
 */
export interface CustomizationGroup {
    id: number                                   // 分组ID
    name: string                                 // 分组名称
    is_required: boolean                         // 是否必选
    sort_order: number                           // 排序
    options: CustomizationOption[]               // 选项列表
}

/**
 * 定制化选项 - 对齐 api.customizationOption
 */
export interface CustomizationOption {
    id: number                                   // 选项ID
    tag_id: number                               // 标签ID
    tag_name: string                             // 标签名称
    extra_price: number                          // 加价（分）
    sort_order: number                           // 排序
}

/**
 * 配料信息 - 对齐 api.ingredient
 */
export interface Ingredient {
    id: number                                   // 配料ID
    name: string                                 // 配料名称
    category?: string                            // 分类
    is_allergen?: boolean                        // 是否过敏原
}

/**
 * 标签信息 - 对齐 api.tagInfo
 */
export interface TagInfo {
    id: number                                   // 标签ID
    name: string                                 // 标签名称
}

/**
 * 菜品分类 - 对齐 api.dishCategory
 */
export interface DishCategory {
    id: number                                   // 分类ID
    name: string                                 // 分类名称
    sort_order: number                           // 排序
    dish_count?: number                          // 菜品数量
}

/**
 * 创建/更新分类请求
 */
export interface CreateDishCategoryRequest {
    name: string
    sort_order?: number
}

/**
 * 菜品列表响应 - 对齐 api.listDishesResponse
 */
export interface ListDishesResponse {
    dishes: DishResponse[]                       // 菜品列表
    total_count: number                          // 总数
}

/**
 * 创建菜品请求 - 对齐 api.createDishRequest
 */
export interface CreateDishRequest extends Record<string, unknown> {
    category_id?: number                         // 分类ID
    description?: string                         // 菜品描述
    image_url?: string                           // 菜品图片URL
    ingredient_ids?: number[]                    // 食材ID列表（最多20个）
    is_available?: boolean                       // 是否可用
    is_online?: boolean                          // 是否上架
    member_price?: number                        // 会员价（分）
    name: string                                 // 菜品名称（必填）
    prepare_time?: number                        // 预估制作时间（分钟）
    price: number                                // 价格（分，必填）
    sort_order?: number                          // 排序
    tag_ids?: number[]                           // 标签ID列表（最多10个）
    customization_groups?: CustomizationGroupInput[]  // 定制选项分组
}

/**
 * 更新菜品请求 - 对齐 api.updateDishRequest
 */
export interface UpdateDishRequest extends Record<string, unknown> {
    name?: string                                // 菜品名称
    description?: string                         // 菜品描述
    price?: number                               // 价格（分）
    member_price?: number                        // 会员价（分）
    image_url?: string                           // 菜品图片URL
    category_id?: number                         // 分类ID
    prepare_time?: number                        // 预估制作时间（分钟）
    sort_order?: number                          // 排序
    is_online?: boolean                          // 是否上架
    is_available?: boolean                       // 是否可用
    tag_ids?: number[]                           // 标签ID列表（最多10个）
}

/**
 * 批量更新菜品状态请求 - 对齐 api.batchUpdateDishStatusRequest
 */
export interface BatchUpdateDishStatusRequest extends Record<string, unknown> {
    dish_ids: number[]                           // 菜品ID列表（必填，最多100个）
    is_online: boolean                           // true=上架, false=下架（必填）
}

/**
 * 批量菜品状态响应 - 对齐 api.batchDishStatusResponse
 */
export interface BatchDishStatusResponse {
    updated: number[]                            // 更新成功的ID
    failed: number[]                             // 更新失败的ID
    message?: string                             // 消息
}

// ==================== 套餐数据类型定义 ====================

/**
 * 套餐响应 - 对齐 api.comboSetResponse
 */
export interface ComboSetResponse {
    id: number                                   // 套餐ID
    name: string                                 // 套餐名称
    description?: string                         // 套餐描述
    combo_price: number                          // 套餐价格（分）
    is_online: boolean                           // 是否上架
}

/**
 * 套餐中的菜品 - 对齐 api.dishInComboResponse
 */
export interface DishInComboResponse {
    dish_id: number                              // 菜品ID
    dish_name: string                            // 菜品名称
    dish_price?: number                          // 菜品价格（分）
    dish_image_url?: string                      // 菜品图片
    quantity?: number                            // 数量
}

/**
 * 套餐详情响应 - 对齐 api.comboSetWithDetailsResponse
 */
export interface ComboSetWithDetailsResponse {
    id: number                                   // 套餐ID
    name: string                                 // 套餐名称
    description?: string                         // 套餐描述
    combo_price: number                          // 套餐价格（分）
    is_online: boolean                           // 是否上架
    dishes: DishInComboResponse[]                // 套餐包含的菜品
    tags?: TagInfo[]                             // 标签信息
}

/**
 * 套餐菜品输入 - 对齐 api.comboDishInput
 */
export interface ComboDishInput {
    dish_id: number                              // 菜品ID（必填）
    quantity: number                             // 数量，1-99
}

export interface UpdateComboSetRequest extends Record<string, unknown> {
    combo_price?: number                         // 套餐价格（分）
    description?: string                         // 描述，最大500字符
    is_online?: boolean                          // 是否上架
    name?: string                                // 套餐名称，最大100字符
    dishes?: ComboDishInput[]                    // 可选：更新套餐菜品列表（带数量）
    tag_ids?: number[]                           // 可选：更新属性标签ID列表（最多10个）
}

/**
 * 创建套餐请求 - 对齐 api.createComboSetRequest
 */
export interface CreateComboSetRequest extends Record<string, unknown> {
    combo_price: number                          // 套餐价格（分，必填）
    description?: string                         // 描述，最大500字符
    is_online?: boolean                          // 是否上架
    name: string                                 // 套餐名称，最大100字符（必填）
    dish_ids?: number[]                          // 套餐包含的菜品ID列表
    tag_ids?: number[]                           // 属性标签ID列表（最多10个）
}

// ==================== 库存数据类型定义 ====================

/**
 * 每日库存响应 - 对齐 api.dailyInventoryResponse
 */
export interface DailyInventoryResponse {
    available: number                            // 可用数量（计算字段: total - sold）
    date: string                                 // 日期（YYYY-MM-DD）
    dish_id: number                              // 菜品ID
    id: number                                   // 库存记录ID
    merchant_id: number                          // 商户ID
    sold_quantity: number                        // 已售数量
    total_quantity: number                       // 总库存数量
}

/**
 * 库存列表响应 - 对齐 api.listDailyInventoryResponse
 */
export interface ListDailyInventoryResponse {
    inventories: DailyInventoryWithDishResponse[]
}

/**
 * 带菜品信息的库存响应 - 对齐 api.dailyInventoryWithDishResponse
 */
export interface DailyInventoryWithDishResponse {
    available: number
    date: string
    dish_id: number
    dish_name: string
    dish_price: number
    id: number
    merchant_id: number
    sold_quantity: number
    total_quantity: number
}

/**
 * 更新库存请求 - 对齐 api.updateDailyInventoryRequest
 */
export interface UpdateDailyInventoryRequest extends Record<string, unknown> {
    date: string                                 // 日期（YYYY-MM-DD，必填）
    dish_id: number                              // 菜品ID（必填）
    sold_quantity?: number                       // 已售数量
    total_quantity?: number                      // 总库存数量
}

/**
 * 创建每日库存请求 - 对齐 api.createDailyInventoryRequest
 */
export interface CreateDailyInventoryRequest extends Record<string, unknown> {
    date: string                                 // 日期（YYYY-MM-DD，必填）
    dish_id: number                              // 菜品ID（必填）
    total_quantity: number                       // 总库存数量（-1表示无限库存，必填）
}

/**
 * 更新单个库存请求 - 对齐 api.updateSingleInventoryRequest
 */
export interface UpdateSingleInventoryRequest extends Record<string, unknown> {
    date: string                                 // 日期（YYYY-MM-DD，必填）
    sold_quantity?: number                       // 已售数量
    total_quantity?: number                      // 总库存数量（-1表示无限库存）
}

/**
 * 检查库存请求 - 对齐 api.checkInventoryRequest
 */
export interface CheckInventoryRequest extends Record<string, unknown> {
    dish_id: number                              // 菜品ID（必填）
    quantity: number                             // 数量（必填）
    date: string                                 // 日期（YYYY-MM-DD，必填）
}

/**
 * 检查库存响应 - 对齐 api.checkInventoryResponse
 */
export interface CheckInventoryResponse {
    available: boolean                           // 是否有库存
    current_stock: number                        // 当前库存
    message?: string                             // 消息
}

/**
 * 库存统计响应 - 对齐 api.inventoryStatsResponse
 */
export interface InventoryStatsResponse {
    total_dishes: number                         // 总菜品数
    available_dishes: number                     // 有库存菜品数
    sold_out_dishes: number                      // 已售罄菜品数
    unlimited_dishes: number                     // 无限库存菜品数
}

/**
 * 菜品分类响应 - 对齐 api.dishCategoryResponse
 */
export interface DishCategoryResponse {
    id: number                                   // 分类ID
    name: string                                 // 分类名称
    sort_order: number                           // 排序
}

/**
 * 创建菜品分类请求 - 对齐 api.createDishCategoryRequest
 */
export interface CreateDishCategoryRequest extends Record<string, unknown> {
    name: string                                 // 分类名称（1-30字符，必填）
    sort_order?: number                          // 排序（0-999）
}

/**
 * 更新菜品分类请求 - 对齐 api.updateDishCategoryRequest
 */
export interface UpdateDishCategoryRequest extends Record<string, unknown> {
    name?: string                                // 分类名称（1-30字符）
    sort_order?: number                          // 排序（0-999）
}

/**
 * 菜品分类列表响应 - 对齐 api.listDishCategoriesResponse
 */
export interface ListDishCategoriesResponse {
    categories: DishCategoryResponse[]           // 分类列表
}

/**
 * 菜品状态响应 - 对齐 api.dishStatusResponse
 */
export interface DishStatusResponse {
    id: number                                   // 菜品ID
    name: string                                 // 菜品名称
    is_online: boolean                           // 是否上架
    message?: string                             // 消息
}

/**
 * 更新菜品状态请求 - 对齐 api.updateDishStatusRequest
 */
export interface UpdateDishStatusRequest extends Record<string, unknown> {
    is_online: boolean                           // true=上架, false=下架（必填）
}

/**
 * 菜品定制化响应 - 对齐 api.dishCustomizationsResponse
 */
export interface DishCustomizationsResponse {
    dish_id: number                              // 菜品ID
    groups: CustomizationGroupResponse[]         // 定制分组列表
}

/**
 * 定制化分组响应 - 对齐 api.customizationGroupResponse
 */
export interface CustomizationGroupResponse {
    id: number                                   // 分组ID
    name: string                                 // 分组名称
    is_required: boolean                         // 是否必选
    sort_order: number                           // 排序
    options: CustomizationOptionResponse[]       // 选项列表
}

/**
 * 定制化选项响应 - 对齐 api.customizationOptionResponse
 */
export interface CustomizationOptionResponse {
    id: number                                   // 选项ID
    tag_id: number                               // 标签ID
    tag_name: string                             // 标签名称
    extra_price: number                          // 加价（分）
    sort_order: number                           // 排序
}

/**
 * 定制化分组输入 - 对齐 api.customizationGroupInput
 */
export interface CustomizationGroupInput {
    name: string                                 // 分组名称（1-50字符，必填）
    is_required?: boolean                        // 是否必选
    sort_order?: number                          // 排序
    options: CustomizationOptionInput[]          // 选项列表（必填）
}

/**
 * 定制化选项输入 - 对齐 api.customizationOptionInput
 */
export interface CustomizationOptionInput {
    tag_id: number                               // 标签ID（必填）
    extra_price?: number                         // 加价（分，0-1000000）
    sort_order?: number                          // 排序
}

/**
 * 设置菜品定制化请求 - 对齐 api.setDishCustomizationsRequest
 */
export interface SetDishCustomizationsRequest extends Record<string, unknown> {
    groups?: CustomizationGroupInput[]           // 定制分组列表（最多20个）
}



/**
 * 创建套餐请求 - 对齐 api.createComboSetRequest
 */
export interface CreateComboSetRequest extends Record<string, unknown> {
    name: string                                 // 套餐名称（1-100字符，必填）
    combo_price: number                          // 套餐优惠价（分，最大100万元，必填）
    description?: string                         // 描述（最大500字符）
    original_price?: number                      // 原价（分）
    is_online?: boolean                          // 是否上线
    dish_ids?: number[]                          // 向后兼容：关联菜品ID列表（最多50个）
    dishes?: ComboDishInput[]                    // 推荐：带数量的菜品列表
}

/**
 * 添加套餐菜品请求 - 对齐 api.addComboDishBodyRequest
 */
export interface AddComboDishBodyRequest extends Record<string, unknown> {
    dish_id: number                              // 菜品ID（必填）
}

/**
 * 切换套餐上架状态请求 - 对齐 api.toggleComboOnlineBodyRequest
 */
export interface ToggleComboOnlineBodyRequest extends Record<string, unknown> {
    is_online: boolean                           // true=上架, false=下架（必填）
}

/**
 * 套餐列表响应 - 对齐 api.listComboSetsResponse
 */
export interface ListComboSetsResponse {
    combo_sets: ComboSetResponse[]               // 套餐列表
}

/**
 * 推荐菜品响应 - 对齐 api.recommendDishesResponse
 */
export interface RecommendDishesResponse {
    dishes: DishSummary[]                        // 推荐菜品列表
    algorithm: string                            // 推荐算法
    expired_at: string                           // 过期时间
}

/**
 * 推荐套餐响应 - 对齐 api.recommendCombosResponse
 */
export interface RecommendCombosResponse {
    combos: ComboSummary[]                       // 推荐套餐列表
    algorithm: string                            // 推荐算法
    expired_at: string                           // 过期时间
}

/**
 * 套餐摘要 - 对齐 api.comboSummary
 */
export interface ComboSummary {
    id: number                                   // 套餐ID
    name: string                                 // 套餐名称
    image_url: string                            // 套餐图片
    combo_price: number                          // 套餐价（分）
    original_price: number                       // 原价（分）
    savings_percent: number                      // 优惠百分比
    monthly_sales: number                        // 近30天销量
    tags: string[]                               // 套餐标签
    merchant_id: number                          // 商户ID
    merchant_name: string                        // 商户名称
    merchant_logo: string                        // 商户Logo
    merchant_latitude: number                    // 商户纬度
    merchant_longitude: number                   // 商户经度
    merchant_region_id: number                   // 商户区域ID
    distance?: number                            // 距离（米）
    estimated_delivery_fee?: number              // 预估配送费（分）
}

// ==================== 标签管理服务 ====================

/**
 * 标签服务
 * 提供标签查询功能
 */
export class TagService {

    /**
     * 获取指定类型的标签列表
     * GET /v1/tags?type=xxx
     * @param type 标签类型: dish, merchant, combo, table, customization
     */
    static async listTags(type: 'dish' | 'merchant' | 'combo' | 'table' | 'customization'): Promise<TagInfo[]> {
        const response = await request<{ tags: TagInfo[] }>({
            url: '/v1/tags',
            method: 'GET',
            data: { type }
        })
        return response.tags || []
    }

    /**
     * 获取菜品属性标签列表
     * 便捷方法，等同于 listTags('dish')
     */
    static async listDishTags(): Promise<TagInfo[]> {
        return this.listTags('dish')
    }

    /**
     * 获取定制选项标签列表
     * 便捷方法，等同于 listTags('customization')
     */
    static async listCustomizationTags(): Promise<TagInfo[]> {
        return this.listTags('customization')
    }

    /**
     * 创建标签
     * POST /v1/tags
     */
    static async createTag(data: { name: string; type: string }): Promise<TagInfo> {
        return await request<TagInfo>({
            url: '/v1/tags',
            method: 'POST',
            data
        })
    }

    /**
     * 删除标签
     * DELETE /v1/tags/:id
     */
    static async deleteTag(id: number): Promise<void> {
        await request({
            url: `/v1/tags/${id}`,
            method: 'DELETE'
        })
    }
}

// ==================== 菜品管理服务 ====================

/**
 * 菜品管理服务
 * 基于swagger.json完全重构，仅包含后端支持的接口
 */
export class DishManagementService {

    /**
     * 获取商户菜品列表
     * GET /v1/dishes
     */
    static async listDishes(params: {
        category_id?: number
        is_online?: boolean
        is_available?: boolean
        page_id: number
        page_size: number
    }): Promise<ListDishesResponse> {
        return await request({
            url: '/v1/dishes',
            method: 'GET',
            data: params
        })
    }

    /**
     * 创建菜品
     * POST /v1/dishes
     */
    static async createDish(data: CreateDishRequest): Promise<DishResponse> {
        return await request({
            url: '/v1/dishes',
            method: 'POST',
            data
        })
    }

    /**
     * 获取菜品详情（消费者端）
     * GET /v1/public/dishes/{id}
     * 注意：使用公开接口，无需商户权限
     */
    static async getDishDetail(dishId: number): Promise<DishResponse> {
        return await request({
            url: `/v1/public/dishes/${dishId}`,
            method: 'GET'
        })
    }

    /**
     * 更新菜品信息
     * PUT /v1/dishes/{id}
     */
    static async updateDish(dishId: number, data: UpdateDishRequest): Promise<DishResponse> {
        return await request({
            url: `/v1/dishes/${dishId}`,
            method: 'PUT',
            data
        })
    }

    /**
     * 删除菜品
     * DELETE /v1/dishes/{id}
     */
    static async deleteDish(dishId: number): Promise<void> {
        return await request({
            url: `/v1/dishes/${dishId}`,
            method: 'DELETE'
        })
    }

    /**
     * 更新菜品状态
     * PATCH /v1/dishes/{id}/status (使用PUT方法)
     */
    static async updateDishStatus(dishId: number, data: {
        is_online?: boolean
        is_available?: boolean
    }): Promise<void> {
        return await request({
            url: `/v1/dishes/${dishId}/status`,
            method: 'PUT',
            data
        })
    }

    /**
     * 批量更新菜品状态
     * PATCH /v1/dishes/batch/status
     */
    static async batchUpdateDishStatus(data: BatchUpdateDishStatusRequest): Promise<BatchDishStatusResponse> {
        return await request({
            url: '/v1/dishes/batch/status',
            method: 'PATCH',
            data
        })
    }

    /**
     * 获取菜品定制化选项
     * GET /v1/dishes/{id}/customizations
     */
    static async getDishCustomizations(dishId: number): Promise<CustomizationGroup[]> {
        return await request({
            url: `/v1/dishes/${dishId}/customizations`,
            method: 'GET'
        })
    }

    /**
     * 设置菜品定制化选项
     * PUT /v1/dishes/{id}/customizations
     */
    static async setDishCustomizations(dishId: number, groups: SetDishCustomizationsRequest): Promise<void> {
        return await request({
            url: `/v1/dishes/${dishId}/customizations`,
            method: 'PUT',
            data: groups
        })
    }

    /**
     * 获取菜品分类列表
     * GET /v1/dishes/categories
     */
    static async getDishCategories(): Promise<DishCategory[]> {
        const response = await request<{ categories: DishCategory[] }>({
            url: '/v1/dishes/categories',
            method: 'GET'
        })
        // 后端返回 { categories: [...] }，需要提取数组
        return response.categories || []
    }

    /**
     * 创建菜品分类
     * POST /v1/dishes/categories
     */
    static async createDishCategory(data: CreateDishCategoryRequest): Promise<DishCategory> {
        return await request({
            url: '/v1/dishes/categories',
            method: 'POST',
            data
        })
    }

    /**
     * 更新菜品分类
     * PUT /v1/dishes/categories/{id}
     */
    static async updateDishCategory(id: number, data: CreateDishCategoryRequest): Promise<DishCategory> {
        return await request({
            url: `/v1/dishes/categories/${id}`,
            method: 'PUT',
            data
        })
    }

    /**
     * 删除菜品分类
     * DELETE /v1/dishes/categories/{id}
     */
    static async deleteDishCategory(id: number): Promise<void> {
        await request({
            url: `/v1/dishes/categories/${id}`,
            method: 'DELETE'
        })
    }

    /**
     * 上传菜品图片
     * POST /v1/dishes/images/upload
     */
    static async uploadDishImage(filePath: string): Promise<string> {
        return new Promise((resolve, reject) => {
            const token = getToken()
            wx.uploadFile({
                url: `${API_BASE}/v1/dishes/images/upload`,
                filePath,
                name: 'image',
                header: {
                    'Authorization': `Bearer ${token}`
                },
                success: (res) => {
                    if (res.statusCode === 200) {
                        try {
                            const data = JSON.parse(res.data)
                            if (data.code === 0 && data.data && data.data.image_url) {
                                resolve(data.data.image_url)
                            } else if (data.image_url) {
                                resolve(data.image_url)
                            } else {
                                resolve(data.data?.image_url || data.image_url)
                            }
                        } catch (e) {
                            reject(new Error('Parse response failed'))
                        }
                    } else {
                        reject(new Error(`HTTP ${res.statusCode}`))
                    }
                },
                fail: reject
            })
        })
    }
}

// ==================== 套餐管理服务 ====================

/**
 * 套餐管理服务
 * 基于swagger.json完全重构，仅包含后端支持的接口
 */
export class ComboManagementService {

    /**
     * 获取商户套餐列表
     * GET /v1/combos
     */
    static async listCombos(params: {
        page_id: number
        page_size: number
        is_online?: boolean
    }): Promise<{
        combo_sets: ComboSetResponse[]
        total: number
        page_id: number
        page_size: number
        has_more: boolean
    }> {
        return await request({
            url: '/v1/combos',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取套餐详情
     * GET /v1/combos/{id}
     */
    static async getComboDetail(comboId: number): Promise<ComboSetWithDetailsResponse> {
        return await request({
            url: `/v1/combos/${comboId}`,
            method: 'GET'
        })
    }

    /**
     * 更新套餐信息
     * PUT /v1/combos/{id}
     */
    static async updateCombo(comboId: number, data: UpdateComboSetRequest): Promise<ComboSetResponse> {
        return await request({
            url: `/v1/combos/${comboId}`,
            method: 'PUT',
            data
        })
    }

    /**
     * 创建套餐
     * POST /v1/combos
     */
    static async createCombo(data: CreateComboSetRequest): Promise<ComboSetResponse> {
        return await request({
            url: '/v1/combos',
            method: 'POST',
            data
        })
    }

    /**
     * 删除套餐
     * DELETE /v1/combos/{id}
     */
    static async deleteCombo(comboId: number): Promise<void> {
        return await request({
            url: `/v1/combos/${comboId}`,
            method: 'DELETE'
        })
    }

    /**
     * 添加菜品到套餐
     * POST /v1/combos/{id}/dishes
     */
    static async addDishToCombo(comboId: number, data: {
        dish_id: number
        quantity: number
    }): Promise<void> {
        return await request({
            url: `/v1/combos/${comboId}/dishes`,
            method: 'POST',
            data
        })
    }

    /**
     * 从套餐中移除菜品
     * DELETE /v1/combos/{id}/dishes/{dish_id}
     */
    static async removeDishFromCombo(comboId: number, dishId: number): Promise<void> {
        return await request({
            url: `/v1/combos/${comboId}/dishes/${dishId}`,
            method: 'DELETE'
        })
    }

    /**
     * 更新套餐上架状态
     * PUT /v1/combos/{id}/online
     */
    static async updateComboOnlineStatus(comboId: number, data: {
        is_online: boolean
    }): Promise<ComboSetResponse> {
        return await request({
            url: `/v1/combos/${comboId}/online`,
            method: 'PUT',
            data
        })
    }
}

// ==================== 库存管理服务 ====================

/**
 * 库存管理服务
 * 基于swagger.json完全重构，仅包含后端支持的接口
 */
export class InventoryManagementService {

    /**
     * 查询每日库存
     * GET /v1/inventory
     */
    static async getDailyInventory(date: string): Promise<ListDailyInventoryResponse> {
        return await request({
            url: '/v1/inventory',
            method: 'GET',
            data: { date }
        })
    }

    /**
     * 更新库存
     * PUT /v1/inventory
     */
    static async updateInventory(data: UpdateDailyInventoryRequest): Promise<DailyInventoryResponse> {
        return await request({
            url: '/v1/inventory',
            method: 'PUT',
            data
        })
    }

    /**
     * 检查库存
     * POST /v1/inventory/check
     */
    static async checkInventory(data: {
        items: Array<{
            dish_id: number
            quantity: number
        }>
        date?: string
    }): Promise<{
        available: boolean
        unavailable_items?: Array<{
            dish_id: number
            dish_name: string
            requested_quantity: number
            available_quantity: number
        }>
    }> {
        return await request({
            url: '/v1/inventory/check',
            method: 'POST',
            data
        })
    }

    /**
     * 获取库存统计
     * GET /v1/inventory/stats
     */
    static async getInventoryStats(params: {
        start_date: string
        end_date: string
    }): Promise<{
        total_dishes: number
        low_stock_dishes: number
        out_of_stock_dishes: number
        stats: Array<{
            dish_id: number
            dish_name: string
            avg_daily_sales: number
            total_sales: number
            current_stock: number
        }>
    }> {
        return await request({
            url: '/v1/inventory/stats',
            method: 'GET',
            data: params
        })
    }
}

// ==================== 顾客端菜品接口 ====================

/**
 * 搜索菜品项 - 对齐后端 searchDishResponse
 */
export interface SearchDishItem {
    id: number
    merchant_id: number
    category_id?: number
    name: string
    description: string
    image_url: string
    price: number      // 后端已转换为元
    member_price?: number
    is_available: boolean
    is_online: boolean
    sort_order: number
}

/**
 * 搜索菜品响应 - 对齐后端实际返回格式
 */
export interface SearchDishesResponse {
    dishes: SearchDishItem[]
    total?: number
    page_id?: number
    page_size?: number
    has_more?: boolean
}

/**
 * 搜索菜品 - 基于 /v1/search/dishes
 */
export async function searchDishes(params: {
    keyword: string
    merchant_id?: number
    page_id: number
    page_size: number
}): Promise<SearchDishItem[]> {
    const response = await request<SearchDishesResponse>({
        url: '/v1/search/dishes',
        method: 'GET',
        data: params,
        useCache: true,
        cacheTTL: 2 * 60 * 1000 // 2分钟缓存
    })
    // 后端返回 { dishes: [], total: ..., ... }
    // 提取 dishes 数组返回
    return response.dishes || []
}

/**
 * 推荐菜品响应 - 对齐后端实际返回格式
 */
export interface RecommendedDishesResponse {
    dishes: DishSummary[]
    algorithm: string
    expired_at: string
}

/**
 * 推荐菜品请求参数
 */
export interface RecommendDishesParams {
    merchant_id?: number
    limit?: number
    page?: number              // 页码，从1开始
    tag_id?: number            // 按标签ID过滤
    keyword?: string           // 搜索关键词
    user_latitude?: number
    user_longitude?: number
}

/**
 * 推荐菜品响应（包含分页信息）
 */
export interface RecommendDishesResult {
    dishes: DishSummary[]
    has_more: boolean
    page: number
    total_count: number
}

/**
 * 获取推荐菜品 - 基于 /v1/recommendations/dishes
 * 支持分页，返回包含 has_more 的完整响应
 */
export async function getRecommendedDishes(params?: RecommendDishesParams): Promise<RecommendDishesResult> {
    const response = await request<RecommendedDishesResponse & { has_more?: boolean; page?: number; total_count?: number }>({
        url: '/v1/recommendations/dishes',
        method: 'GET',
        data: params,
        useCache: params?.page === 1 || !params?.page,  // 只缓存第一页
        cacheTTL: 3 * 60 * 1000 // 3分钟缓存
    })
    // 返回完整响应，包含分页信息
    return {
        dishes: response.dishes || [],
        has_more: response.has_more ?? false,
        page: response.page ?? 1,
        total_count: response.total_count ?? 0
    }
}

/**
 * 推荐套餐响应 - 对齐后端实际返回格式
 */
export interface RecommendedCombosResponse {
    combos: ComboSummary[]  // 后端返回 comboSummary，包含完整信息
    algorithm: string
    expired_at: string
}

/**
 * 推荐套餐请求参数
 */
export interface RecommendCombosParams {
    merchant_id?: number
    limit?: number
    page?: number
    keyword?: string              // 搜索关键词
    user_latitude?: number        // 用户纬度（用于计算距离和配送费）
    user_longitude?: number       // 用户经度（用于计算距离和配送费）
}

/**
 * 推荐套餐结果（包含分页信息）
 */
export interface RecommendCombosResult {
    combos: ComboSummary[]  // 使用 ComboSummary（包含完整信息：图片、销量、距离等）
    has_more: boolean
    page: number
    total_count: number
}

/**
 * 获取推荐套餐 - 基于 /v1/recommendations/combos
 * 支持分页，返回包含 has_more 的完整响应
 */
export async function getRecommendedCombos(params?: RecommendCombosParams): Promise<RecommendCombosResult> {
    const response = await request<RecommendedCombosResponse & { has_more?: boolean; page?: number; total_count?: number }>({
        url: '/v1/recommendations/combos',
        method: 'GET',
        data: params,
        useCache: params?.page === 1 || !params?.page,
        cacheTTL: 3 * 60 * 1000 // 3分钟缓存
    })
    return {
        combos: response.combos || [],
        has_more: response.has_more ?? false,
        page: response.page ?? 1,
        total_count: response.total_count ?? 0
    }
}

// ==================== 标签 API ====================

/**
 * 标签响应
 */
export interface Tag {
    id: number
    name: string
    type: string
    sort_order: number
}

/**
 * 获取标签列表 - 基于 /v1/tags
 * @param type 标签类型: dish, combo, merchant, attribute, customization
 */
export async function getTags(type: string): Promise<Tag[]> {
    interface TagsResponse {
        tags: Tag[]
    }
    const response = await request<TagsResponse>({
        url: '/v1/tags',
        method: 'GET',
        data: { type },
        useCache: true,
        cacheTTL: 10 * 60 * 1000 // 10分钟缓存
    })
    return response.tags || []
}

// ==================== 导出默认服务 ====================

export default DishManagementService