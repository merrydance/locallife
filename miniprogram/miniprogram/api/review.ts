/**
 * 评价系统接口
 * 包含创建评价、查询评价、商家回复等功能
 */

import { request, API_BASE } from '../utils/request'
import { getToken } from '../utils/auth'

// ==================== 数据类型定义 ====================

/**
 * 评价类型枚举
 */
export type ReviewRating = 1 | 2 | 3 | 4 | 5

/**
 * 创建评价请求
 */
export interface CreateReviewRequest {
    order_id: number
    rating: ReviewRating
    content: string
    images?: string[] // 图片URL列表
    is_anonymous?: boolean
}

/**
 * 商家回复请求
 */
export interface ReplyReviewRequest {
    review_id: number
    content: string
}

/**
 * 评价详情响应
 */
export interface ReviewResponse {
    id: number
    order_id: number
    user_id: number
    user_name: string // 匿名时显示"匿名用户"
    user_avatar: string // 匿名时显示默认头像
    merchant_id: number
    rating: ReviewRating
    content: string
    images?: string[]
    reply?: string
    reply_at?: string
    created_at: string
    is_anonymous: boolean
}

/**
 * 评价列表查询参数
 */
export interface ReviewListParams {
    merchant_id?: number
    user_id?: number
    page_id: number
    page_size: number
    has_image?: boolean
    has_reply?: boolean
}

/**
 * 评价列表响应
 */
export interface ReviewListResponse {
    reviews: ReviewResponse[]
    total: number
    rating_avg: number // 平均分
    rating_counts: {
        all: number
        good: number    // 4-5分
        medium: number  // 3分
        bad: number     // 1-2分
        has_image: number
    }
}

// ==================== 评价服务 ====================

export class ReviewService {

    /**
     * 创建评价
     * POST /v1/reviews
     */
    static async createReview(data: CreateReviewRequest): Promise<ReviewResponse> {
        return await request({
            url: '/v1/reviews',
            method: 'POST',
            data
        })
    }

    /**
     * 获取评价列表
     * GET /v1/reviews
     */
    static async getReviews(params: ReviewListParams): Promise<ReviewListResponse> {
        return await request({
            url: '/v1/reviews',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取评价详情
     * GET /v1/reviews/:id
     */
    static async getReviewDetail(id: number): Promise<ReviewResponse> {
        return await request({
            url: `/v1/reviews/${id}`,
            method: 'GET'
        })
    }

    // ==================== 商家端接口 ====================

    /**
     * 商家回复评价
     * POST /v1/merchant/reviews/:id/reply
     */
    static async replyReview(id: number, content: string): Promise<ReviewResponse> {
        return await request({
            url: `/v1/merchant/reviews/${id}/reply`,
            method: 'POST',
            data: { content }
        })
    }

    /**
     * 上传评价图片
     * POST /v1/reviews/images/upload
     */
    static async uploadReviewImage(filePath: string): Promise<string> {
        return new Promise((resolve, reject) => {
            const token = getToken()
            wx.uploadFile({
                url: `${API_BASE}/v1/reviews/images/upload`,
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

export default ReviewService
