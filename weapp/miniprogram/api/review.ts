import { request } from '../utils/request'
import { uploadMedia, MediaUploadResult } from '../utils/media'
import { normalizePaginatedResult, type PaginatedListResult, type PaginationEnvelope } from './types'

export interface Review {
    id: number
    order_id: number
    user_id: number
    merchant_id: number
    content: string
    rating?: number
    tags?: string[]
    image_urls?: string[]
    images?: string[]
    is_visible: boolean
    merchant_reply?: string
    replied_at?: string
    merchant_name?: string
    merchant_logo_url?: string
    merchant_logo?: string
    created_at: string
}

export interface ListReviewsResponse {
    reviews: Review[]
    total: number
    page_id: number
    page_size: number
}

export interface ReviewListResult extends PaginatedListResult<Review> {
    reviews: Review[]
}

type ReviewsResponse = PaginationEnvelope & {
    reviews?: Review[]
}

export interface CreateReviewParams {
    order_id: number
    content: string
    rating?: number
    tags?: string[]
    media_asset_ids?: number[]
}

export interface ReplyReviewParams {
    reply: string
}

function normalizeReview(review: Review): Review {
    const imageUrls = Array.isArray(review.image_urls)
        ? review.image_urls
        : Array.isArray(review.images)
            ? review.images
            : []

    return {
        ...review,
        image_urls: imageUrls,
        images: imageUrls,
        merchant_logo: review.merchant_logo || review.merchant_logo_url
    }
}

function normalizeReviewListResponse(reviews: Review[] | undefined, pageId: number, pageSize: number, envelope: ReviewsResponse): ReviewListResult {
    const normalizedReviews = Array.isArray(reviews) ? reviews.map(normalizeReview) : []
    const normalized = normalizePaginatedResult(normalizedReviews, envelope, { page: pageId, pageSize })

    return {
        ...normalized,
        reviews: normalizedReviews
    }
}

export class ReviewService {
    /**
     * 获取我的评价列表
     * GET /v1/reviews/me
     */
    static async listMyReviews(pageId: number = 1, pageSize: number = 10): Promise<ReviewListResult> {
        const res = await request<ReviewsResponse>({
            url: '/v1/reviews/me',
            method: 'GET',
            data: { page_id: pageId, page_size: pageSize }
        })

        return normalizeReviewListResponse(res?.reviews, pageId, pageSize, res || {})
    }

    static async listMerchantAllReviews(merchantId: number, pageId: number = 1, pageSize: number = 20): Promise<ReviewListResult> {
        const res = await request<ReviewsResponse>({
            url: `/v1/reviews/merchants/${merchantId}/all`,
            method: 'GET',
            data: { page_id: pageId, page_size: pageSize }
        })

        return normalizeReviewListResponse(res?.reviews, pageId, pageSize, res || {})
    }

    /**
     * 创建评价
     * POST /v1/reviews
     */
    static async createReview(data: CreateReviewParams): Promise<Review> {
        const review = await request<Review>({
            url: '/v1/reviews',
            method: 'POST',
            data
        })

        return normalizeReview(review)
    }

    /**
     * 获取指定订单的评价
     * GET /v1/reviews/orders/:order_id
     */
    static async getReviewByOrderId(orderId: number): Promise<Review> {
        const review = await request<Review>({
            url: `/v1/reviews/orders/${orderId}`,
            method: 'GET'
        })

        return normalizeReview(review)
    }

    /**
     * 获取评价详情
     * GET /v1/reviews/:id
     */
    static async getReview(id: number): Promise<Review> {
        const review = await request<Review>({
            url: `/v1/reviews/${id}`,
            method: 'GET'
        })

        return normalizeReview(review)
    }

    static async replyToReview(reviewId: number, data: ReplyReviewParams): Promise<Review> {
        const review = await request<Review>({
            url: `/v1/reviews/${reviewId}/reply`,
            method: 'POST',
            data
        })

        return normalizeReview(review)
    }

    /**
     * 上传评价图片（媒体服务三步流程）
     * @returns { mediaId, displayUrl, urls }
     */
    static async uploadReviewImage(filePath: string): Promise<MediaUploadResult> {
        return uploadMedia(filePath, {
            businessType: 'user',
            mediaCategory: 'review'
        })
    }
}

export default ReviewService
