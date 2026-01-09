/**
 * 申诉和客服接口重构 (Task 2.8)
 * 基于swagger.json完全重构，移除所有没有后端支持的旧功能
 * 包含：申诉管理、索赔处理、评价回复
 */

import { request } from '../utils/request'

// ==================== 数据类型定义 ====================

/** 申诉状态枚举 */
export type AppealStatus = 'pending' | 'approved' | 'rejected' | 'compensated'

/** 申诉人类型枚举 */
export type AppellantType = 'merchant' | 'rider' | 'user'

/** 索赔状态枚举 */
export type ClaimStatus = 'pending' | 'approved' | 'rejected' | 'compensated'

/** 索赔类型枚举 */
export type ClaimType = 'refund' | 'compensation' | 'quality_issue' | 'delivery_issue'

/** 审批类型枚举 */
export type ApprovalType = 'full' | 'partial' | 'rejected'

// ==================== 申诉管理相关类型 ====================

/** 申诉响应 - 基于swagger api.appealResponse */
export interface AppealResponse {
    id: number
    appellant_id: number
    appellant_type: AppellantType
    claim_id: number
    reason: string
    evidence_urls: string[]
    status: AppealStatus
    region_id: number
    compensation_amount?: number
    review_notes?: string
    reviewer_id?: number
    created_at: string
    reviewed_at?: string
    compensated_at?: string
}

/** 创建申诉请求 */
export interface CreateAppealRequest extends Record<string, unknown> {
    claim_id: number
    reason: string
    evidence_urls?: string[]
}

/** 申诉列表查询参数 */
export interface AppealsQueryParams extends Record<string, unknown> {
    page_id: number
    page_size: number
    status?: AppealStatus
}

// ==================== 索赔管理相关类型 ====================

/** 索赔响应 - 基于swagger api.claimResponse */
export interface ClaimResponse {
    id: number
    user_id: number
    order_id: number
    claim_type: ClaimType
    description: string
    claim_amount: number
    approved_amount?: number
    approval_type?: ApprovalType
    evidence_urls: string[]
    status: ClaimStatus
    is_malicious: boolean
    review_notes?: string
    reviewer_id?: number
    created_at: string
    reviewed_at?: string
}

/** 索赔列表查询参数 */
export interface ClaimsQueryParams extends Record<string, unknown> {
    page_id: number
    page_size: number
    status?: ClaimStatus
    claim_type?: ClaimType
}

// ==================== 评价回复相关类型 ====================

/** 评价响应 - 基于swagger api.reviewResponse */
export interface ReviewResponse {
    id: number
    user_id: number
    merchant_id: number
    order_id: number
    content: string
    images: string[]
    merchant_reply?: string
    is_visible: boolean
    created_at: string
    replied_at?: string
}

/** 回复评价请求 - 基于swagger api.replyReviewRequest */
export interface ReplyReviewRequest extends Record<string, unknown> {
    reply: string
}

/** 商户评价查询参数 */
export interface MerchantReviewsQueryParams extends Record<string, unknown> {
    page_id: number
    page_size: number
    has_reply?: boolean
    start_date?: string
    end_date?: string
}

// ==================== 申诉管理服务类 ====================

/**
 * 申诉管理服务
 * 提供申诉的查询、创建、处理等功能
 */
export class AppealManagementService {
    /**
     * 获取商户申诉列表
     * @param params 查询参数
     */
    async getMerchantAppeals(params: AppealsQueryParams): Promise<{
        appeals: AppealResponse[]
        total: number
        page_id: number
        page_size: number
        has_more: boolean
    }> {
        return request({
            url: '/v1/merchant/appeals',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取申诉详情
     * @param appealId 申诉ID
     */
    async getAppealDetail(appealId: number): Promise<AppealResponse> {
        return request({
            url: `/v1/merchant/appeals/${appealId}`,
            method: 'GET'
        })
    }

    /**
     * 创建申诉
     * @param appealData 申诉数据
     */
    async createAppeal(appealData: CreateAppealRequest): Promise<AppealResponse> {
        return request({
            url: '/v1/merchant/appeals',
            method: 'POST',
            data: appealData
        })
    }

    /**
     * 获取骑手申诉列表
     * @param params 查询参数
     */
    async getRiderAppeals(params: AppealsQueryParams): Promise<{
        appeals: AppealResponse[]
        total: number
        page_id: number
        page_size: number
        has_more: boolean
    }> {
        return request({
            url: '/v1/rider/appeals',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取骑手申诉详情
     * @param appealId 申诉ID
     */
    async getRiderAppealDetail(appealId: number): Promise<AppealResponse> {
        return request({
            url: `/v1/rider/appeals/${appealId}`,
            method: 'GET'
        })
    }

    /**
     * 骑手创建申诉
     * @param appealData 申诉数据
     */
    async createRiderAppeal(appealData: CreateAppealRequest): Promise<AppealResponse> {
        return request({
            url: '/v1/rider/appeals',
            method: 'POST',
            data: appealData
        })
    }
}

// ==================== 索赔管理服务类 ====================

/**
 * 索赔管理服务
 * 提供索赔的查询、处理等功能
 */
export class ClaimManagementService {
    /**
     * 获取用户索赔列表
     * @param params 查询参数
     */
    async getUserClaims(params: ClaimsQueryParams): Promise<{
        claims: ClaimResponse[]
        total: number
        page_id: number
        page_size: number
        has_more: boolean
    }> {
        return request({
            url: '/v1/claims',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取索赔详情
     * @param claimId 索赔ID
     */
    async getClaimDetail(claimId: number): Promise<ClaimResponse> {
        return request({
            url: `/v1/claims/${claimId}`,
            method: 'GET'
        })
    }

    /**
     * 获取商户收到的索赔列表
     * @param params 查询参数
     */
    async getMerchantClaims(params: ClaimsQueryParams): Promise<{
        claims: ClaimResponse[]
        total: number
        page_id: number
        page_size: number
        has_more: boolean
    }> {
        return request({
            url: '/v1/merchant/claims',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取商户索赔详情
     * @param claimId 索赔ID
     */
    async getMerchantClaimDetail(claimId: number): Promise<ClaimResponse> {
        return request({
            url: `/v1/merchant/claims/${claimId}`,
            method: 'GET'
        })
    }

    /**
     * 获取骑手索赔列表
     * @param params 查询参数
     */
    async getRiderClaims(params: ClaimsQueryParams): Promise<{
        claims: ClaimResponse[]
        total: number
        page_id: number
        page_size: number
        has_more: boolean
    }> {
        return request({
            url: '/v1/rider/claims',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取骑手索赔详情
     * @param claimId 索赔ID
     */
    async getRiderClaimDetail(claimId: number): Promise<ClaimResponse> {
        return request({
            url: `/v1/rider/claims/${claimId}`,
            method: 'GET'
        })
    }
}

// ==================== 评价回复服务类 ====================

/**
 * 评价回复服务
 * 提供评价查询和回复功能
 */
export class ReviewReplyService {
    /**
     * 获取商户评价列表
     * @param merchantId 商户ID
     * @param params 查询参数
     */
    async getMerchantReviews(merchantId: number, params: MerchantReviewsQueryParams): Promise<{
        reviews: ReviewResponse[]
        total: number
        page_id: number
        page_size: number
        has_more: boolean
    }> {
        return request({
            url: `/v1/reviews/merchants/${merchantId}`,
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取评价详情
     * @param reviewId 评价ID
     */
    async getReviewDetail(reviewId: number): Promise<ReviewResponse> {
        return request({
            url: `/v1/reviews/${reviewId}`,
            method: 'GET'
        })
    }

    /**
     * 回复评价
     * @param reviewId 评价ID
     * @param replyData 回复内容
     */
    async replyToReview(reviewId: number, replyData: ReplyReviewRequest): Promise<ReviewResponse> {
        return request({
            url: `/v1/reviews/${reviewId}/reply`,
            method: 'POST',
            data: replyData
        })
    }
}

// ==================== 运营商申诉审核服务类 ====================

/**
 * 运营商申诉审核服务
 * 提供申诉审核功能（仅运营商使用）
 */
export class OperatorAppealReviewService {
    /**
     * 获取待审核申诉列表
     * @param params 查询参数
     */
    async getPendingAppeals(params: AppealsQueryParams): Promise<{
        appeals: AppealResponse[]
        total: number
        page_id: number
        page_size: number
        has_more: boolean
    }> {
        return request({
            url: '/v1/operator/appeals',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取申诉详情
     * @param appealId 申诉ID
     */
    async getAppealDetailForReview(appealId: number): Promise<AppealResponse> {
        return request({
            url: `/v1/operator/appeals/${appealId}`,
            method: 'GET'
        })
    }

    /**
     * 审核申诉
     * @param appealId 申诉ID
     * @param reviewData 审核数据
     */
    async reviewAppeal(appealId: number, reviewData: {
        status: 'approved' | 'rejected'
        review_notes?: string
        compensation_amount?: number
    }): Promise<AppealResponse> {
        return request({
            url: `/v1/operator/appeals/${appealId}/review`,
            method: 'POST',
            data: reviewData
        })
    }
}

// ==================== 数据适配器 ====================

/**
 * 申诉和客服数据适配器
 * 处理前端数据格式与后端API数据格式的转换
 */
export class AppealsCustomerServiceAdapter {
    /**
     * 适配申诉响应数据
     */
    static adaptAppealResponse(data: AppealResponse): {
        id: number
        appellantId: number
        appellantType: AppellantType
        claimId: number
        reason: string
        evidenceUrls: string[]
        status: AppealStatus
        regionId: number
        compensationAmount?: number
        reviewNotes?: string
        reviewerId?: number
        createdAt: string
        reviewedAt?: string
        compensatedAt?: string
    } {
        return {
            id: data.id,
            appellantId: data.appellant_id,
            appellantType: data.appellant_type,
            claimId: data.claim_id,
            reason: data.reason,
            evidenceUrls: data.evidence_urls,
            status: data.status,
            regionId: data.region_id,
            compensationAmount: data.compensation_amount,
            reviewNotes: data.review_notes,
            reviewerId: data.reviewer_id,
            createdAt: data.created_at,
            reviewedAt: data.reviewed_at,
            compensatedAt: data.compensated_at
        }
    }

    /**
     * 适配索赔响应数据
     */
    static adaptClaimResponse(data: ClaimResponse): {
        id: number
        userId: number
        orderId: number
        claimType: ClaimType
        description: string
        claimAmount: number
        approvedAmount?: number
        approvalType?: ApprovalType
        evidenceUrls: string[]
        status: ClaimStatus
        isMalicious: boolean
        reviewNotes?: string
        reviewerId?: number
        createdAt: string
        reviewedAt?: string
    } {
        return {
            id: data.id,
            userId: data.user_id,
            orderId: data.order_id,
            claimType: data.claim_type,
            description: data.description,
            claimAmount: data.claim_amount,
            approvedAmount: data.approved_amount,
            approvalType: data.approval_type,
            evidenceUrls: data.evidence_urls,
            status: data.status,
            isMalicious: data.is_malicious,
            reviewNotes: data.review_notes,
            reviewerId: data.reviewer_id,
            createdAt: data.created_at,
            reviewedAt: data.reviewed_at
        }
    }

    /**
     * 适配评价响应数据
     */
    static adaptReviewResponse(data: ReviewResponse): {
        id: number
        userId: number
        merchantId: number
        orderId: number
        content: string
        images: string[]
        merchantReply?: string
        isVisible: boolean
        createdAt: string
        repliedAt?: string
    } {
        return {
            id: data.id,
            userId: data.user_id,
            merchantId: data.merchant_id,
            orderId: data.order_id,
            content: data.content,
            images: data.images,
            merchantReply: data.merchant_reply,
            isVisible: data.is_visible,
            createdAt: data.created_at,
            repliedAt: data.replied_at
        }
    }

    /**
     * 适配创建申诉请求数据
     */
    static adaptCreateAppealRequest(data: {
        claimId: number
        reason: string
        evidenceUrls?: string[]
    }): CreateAppealRequest {
        return {
            claim_id: data.claimId,
            reason: data.reason,
            evidence_urls: data.evidenceUrls
        }
    }
}

// ==================== 导出服务实例 ====================

export const appealManagementService = new AppealManagementService()
export const claimManagementService = new ClaimManagementService()
export const reviewReplyService = new ReviewReplyService()
export const operatorAppealReviewService = new OperatorAppealReviewService()

// ==================== 便捷函数 ====================

/**
 * 获取商户客服工作台数据
 * @param merchantId 商户ID
 */
export async function getMerchantCustomerServiceDashboard(merchantId: number): Promise<{
    pendingAppeals: AppealResponse[]
    pendingClaims: ClaimResponse[]
    unrepliedReviews: ReviewResponse[]
    stats: {
        totalAppeals: number
        totalClaims: number
        totalReviews: number
        pendingCount: number
    }
}> {
    const [appealsResult, claimsResult, reviewsResult] = await Promise.all([
        appealManagementService.getMerchantAppeals({ page_id: 1, page_size: 10, status: 'pending' }),
        claimManagementService.getMerchantClaims({ page_id: 1, page_size: 10, status: 'pending' }),
        reviewReplyService.getMerchantReviews(merchantId, { page_id: 1, page_size: 10, has_reply: false })
    ])

    const pendingCount = appealsResult.appeals.length + claimsResult.claims.length + reviewsResult.reviews.length

    return {
        pendingAppeals: appealsResult.appeals,
        pendingClaims: claimsResult.claims,
        unrepliedReviews: reviewsResult.reviews,
        stats: {
            totalAppeals: appealsResult.total,
            totalClaims: claimsResult.total,
            totalReviews: reviewsResult.total,
            pendingCount
        }
    }
}

/**
 * 批量回复评价
 * @param replies 回复列表
 */
export async function batchReplyReviews(replies: Array<{
    reviewId: number
    reply: string
}>): Promise<{ reviewId: number; success: boolean; message: string }[]> {
    const promises = replies.map(async ({ reviewId, reply }) => {
        try {
            await reviewReplyService.replyToReview(reviewId, { reply })
            return { reviewId, success: true, message: '回复成功' }
        } catch (error: any) {
            return {
                reviewId,
                success: false,
                message: error?.message || '回复失败'
            }
        }
    })

    return Promise.all(promises)
}

/**
 * 获取申诉统计信息
 * @param startDate 开始日期
 * @param endDate 结束日期
 */
export async function getAppealStatistics(startDate: string, endDate: string): Promise<{
    totalAppeals: number
    approvedAppeals: number
    rejectedAppeals: number
    pendingAppeals: number
    approvalRate: number
    avgProcessingTime: number
}> {
    // 这里需要根据实际的统计接口来实现
    // 目前swagger中没有专门的统计接口，可能需要通过查询所有数据来计算
    const appealsResult = await appealManagementService.getMerchantAppeals({
        page_id: 1,
        page_size: 1000 // 获取大量数据用于统计
    })

    const appeals = appealsResult.appeals
    const totalAppeals = appeals.length
    const approvedAppeals = appeals.filter(a => a.status === 'approved').length
    const rejectedAppeals = appeals.filter(a => a.status === 'rejected').length
    const pendingAppeals = appeals.filter(a => a.status === 'pending').length
    const approvalRate = totalAppeals > 0 ? (approvedAppeals / totalAppeals) * 100 : 0

    // 计算平均处理时间（天）
    const processedAppeals = appeals.filter(a => a.reviewed_at)
    const avgProcessingTime = processedAppeals.length > 0
        ? processedAppeals.reduce((sum, appeal) => {
            const created = new Date(appeal.created_at)
            const reviewed = new Date(appeal.reviewed_at!)
            const diffDays = (reviewed.getTime() - created.getTime()) / (1000 * 60 * 60 * 24)
            return sum + diffDays
        }, 0) / processedAppeals.length
        : 0

    return {
        totalAppeals,
        approvedAppeals,
        rejectedAppeals,
        pendingAppeals,
        approvalRate,
        avgProcessingTime
    }
}

/**
 * 验证申诉理由
 * @param reason 申诉理由
 */
export function validateAppealReason(reason: string): { valid: boolean; message?: string } {
    if (!reason || reason.trim().length === 0) {
        return { valid: false, message: '申诉理由不能为空' }
    }

    if (reason.length < 10) {
        return { valid: false, message: '申诉理由至少需要10个字符' }
    }

    if (reason.length > 1000) {
        return { valid: false, message: '申诉理由不能超过1000个字符' }
    }

    return { valid: true }
}

/**
 * 验证评价回复
 * @param reply 回复内容
 */
export function validateReviewReply(reply: string): { valid: boolean; message?: string } {
    if (!reply || reply.trim().length === 0) {
        return { valid: false, message: '回复内容不能为空' }
    }

    if (reply.length > 500) {
        return { valid: false, message: '回复内容不能超过500个字符' }
    }

    return { valid: true }
}

/**
 * 格式化申诉状态显示
 * @param status 申诉状态
 */
export function formatAppealStatus(status: AppealStatus): string {
    const statusMap: Record<AppealStatus, string> = {
        pending: '待审核',
        approved: '已通过',
        rejected: '已拒绝',
        compensated: '已赔付'
    }
    return statusMap[status] || status
}

/**
 * 格式化索赔状态显示
 * @param status 索赔状态
 */
export function formatClaimStatus(status: ClaimStatus): string {
    const statusMap: Record<ClaimStatus, string> = {
        pending: '待审核',
        approved: '已通过',
        rejected: '已拒绝',
        compensated: '已赔付'
    }
    return statusMap[status] || status
}

/**
 * 格式化索赔类型显示
 * @param claimType 索赔类型
 */
export function formatClaimType(claimType: ClaimType): string {
    const typeMap: Record<ClaimType, string> = {
        refund: '退款',
        compensation: '赔偿',
        quality_issue: '质量问题',
        delivery_issue: '配送问题'
    }
    return typeMap[claimType] || claimType
}

/**
 * 计算申诉处理时效
 * @param createdAt 创建时间
 * @param reviewedAt 审核时间
 */
export function calculateAppealProcessingTime(createdAt: string, reviewedAt?: string): {
    days: number
    hours: number
    isOverdue: boolean
} {
    const created = new Date(createdAt)
    const reviewed = reviewedAt ? new Date(reviewedAt) : new Date()

    const diffMs = reviewed.getTime() - created.getTime()
    const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24))
    const diffHours = Math.floor((diffMs % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60))

    // 假设申诉处理时效为3个工作日
    const isOverdue = diffDays > 3

    return {
        days: diffDays,
        hours: diffHours,
        isOverdue
    }
}