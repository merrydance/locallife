/**
 * 收藏系统接口
 * 包含收藏/取消收藏、获取收藏列表
 */

import { request } from '../../../../utils/request'

// ==================== 数据类型定义 ====================

export type FavoriteType = 'merchant' | 'dish'

/**
 * 收藏项
 */
export interface FavoriteItem {
    id: number
    target_id: number // 商户ID 或 菜品ID
    type: FavoriteType
    title: string // 商户名 或 菜品名
    image: string
    sub_title?: string // 描述 或 价格
    rating?: number // 商户评分
    tags?: string[]
    created_at: string
}

export interface FavoriteMerchantListItem {
    id: number
    merchant_id: number
    merchant_name: string
    merchant_logo_url?: string
    merchant_logo?: string
    address?: string
    status?: string
    is_ordering_suspended?: boolean
    created_at?: string
}

/**
 * 收藏列表查询参数
 */
export interface FavoriteListParams {
    type: FavoriteType
    page_id: number
    page_size: number
}

// ==================== 收藏服务 ====================

export class FavoriteService {

    /**
     * 添加收藏商户
     * POST /v1/favorites/merchants
     */
    static async addFavoriteMerchant(merchantId: number): Promise<void> {
        return await request({
            url: '/v1/favorites/merchants',
            method: 'POST',
            data: { merchant_id: merchantId }
        })
    }

    /**
     * 添加收藏菜品
     * POST /v1/favorites/dishes
     */
    static async addFavoriteDish(dishId: number): Promise<void> {
        return await request({
            url: '/v1/favorites/dishes',
            method: 'POST',
            data: { dish_id: dishId }
        })
    }

    /**
     * 取消收藏商户
     * DELETE /v1/favorites/merchants/:id
     */
    static async removeFavoriteMerchant(merchantId: number): Promise<void> {
        return await request({
            url: `/v1/favorites/merchants/${merchantId}`,
            method: 'DELETE'
        })
    }

    /**
     * 取消收藏菜品
     * DELETE /v1/favorites/dishes/:id
     */
    static async removeFavoriteDish(dishId: number): Promise<void> {
        return await request({
            url: `/v1/favorites/dishes/${dishId}`,
            method: 'DELETE'
        })
    }

    /**
     * 获取收藏商户列表
     * GET /v1/favorites/merchants
     */
    static async getFavoriteMerchants(page: number = 1, pageSize: number = 20): Promise<{ merchants: FavoriteMerchantListItem[], total: number }> {
        return await request({
            url: '/v1/favorites/merchants',
            method: 'GET',
            data: { page, page_size: pageSize }
        })
    }

    /**
     * 获取收藏菜品列表
     * GET /v1/favorites/dishes
     */
    static async getFavoriteDishes(page: number = 1, pageSize: number = 20): Promise<{ dishes: Array<Record<string, unknown>>, total: number }> {
        return await request({
            url: '/v1/favorites/dishes',
            method: 'GET',
            data: { page, page_size: pageSize }
        })
    }
}

export default FavoriteService
