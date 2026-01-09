/**
 * 收藏系统接口
 * 包含收藏/取消收藏、获取收藏列表
 */

import { request } from '../utils/request'

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
     * 添加收藏
     * POST /v1/favorites
     */
    static async addFavorite(type: FavoriteType, targetId: number): Promise<void> {
        return await request({
            url: '/v1/favorites',
            method: 'POST',
            data: { type, target_id: targetId }
        })
    }

    /**
     * 取消收藏
     * POST /v1/favorites/remove
     */
    static async removeFavorite(type: FavoriteType, targetId: number): Promise<void> {
        return await request({
            url: '/v1/favorites/remove',
            method: 'POST',
            data: { type, target_id: targetId }
        })
    }

    /**
     * 获取收藏列表
     * GET /v1/favorites
     */
    static async getFavorites(params: FavoriteListParams): Promise<{ items: FavoriteItem[], total: number }> {
        return await request({
            url: '/v1/favorites',
            method: 'GET',
            data: params
        })
    }

    /**
     * 检查是否已收藏
     * GET /v1/favorites/check
     */
    static async checkFavorite(type: FavoriteType, targetId: number): Promise<{ is_favorite: boolean }> {
        return await request({
            url: '/v1/favorites/check',
            method: 'GET',
            data: { type, target_id: targetId }
        })
    }
}

export default FavoriteService
