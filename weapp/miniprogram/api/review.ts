import { request, uploadFile } from '../utils/request'

export interface Review {
    id: number
    order_id: number
    user_id: number
    merchant_id: number
    content: string
    images?: string[]
    is_visible: boolean
    merchant_reply?: string
    replied_at?: string
    merchant_name?: string
    merchant_logo?: string
    created_at: string
}

export interface ListReviewsResponse {
    reviews: Review[]
    total_count: number
    page_id: number
    page_size: number
}

// Params
export interface CreateReviewParams {
    order_id: number
    content: string
    images?: string[]
}

export class ReviewService {
    /**
     * 获取我的评价列表
     * GET /v1/reviews/me
     */
    static async listMyReviews(pageId: number = 1, pageSize: number = 10): Promise<ListReviewsResponse> {
        return await request({
            url: '/v1/reviews/me',
            method: 'GET',
            data: { page_id: pageId, page_size: pageSize }
        })
    }

    /**
     * 创建评价
     * POST /v1/reviews
     */
    static async createReview(data: CreateReviewParams): Promise<Review> {
        return await request({
            url: '/v1/reviews',
            method: 'POST',
            data
        })
    }

    /**
     * 获取指定订单的评价
     * GET /v1/reviews/orders/:order_id
     */
    static async getReviewByOrderId(orderId: number): Promise<Review> {
        return await request({
            url: `/v1/reviews/orders/${orderId}`,
            method: 'GET'
        })
    }

    /**
     * 获取评价详情
     * GET /v1/reviews/:id
     */
    static async getReview(id: number): Promise<Review> {
        return await request({
            url: `/v1/reviews/${id}`,
            method: 'GET'
        })
    }

    /**
     * 上传评价图片
     * POST /v1/reviews/images/upload
     */
    static async uploadReviewImage(filePath: string): Promise<string> {
        return await uploadFile<{ image_url: string }>(filePath, '/v1/reviews/images/upload', 'image')
            .then(res => res.image_url)
    }
}

export default ReviewService
