/**
 * 申诉和客服接口重构 (Task 2.8)
 * 基于swagger.json完全重构，移除所有没有后端支持的旧功能
 * 包含：申诉管理、索赔处理、评价回复
 */

import type { MiniProgramPayParams } from './payment'
import { request } from '../../../../utils/request'

// ==================== 数据类型定义 ====================

/** 申诉状态枚举 */
export type AppealStatus = 'submitted' | 'approved' | 'rejected'

/** 申诉人类型枚举 */
export type AppellantType = 'merchant' | 'rider' | 'user'

/** 索赔状态枚举 */
export type ClaimStatus = 'pending' | 'approved' | 'rejected' | 'compensated'

/** 用户侧索赔生命周期状态 */
export type UserClaimStatus = 'accepted' | 'rejected' | 'warned_waiting_customer_confirmation' | 'withdrawn'

/** 用户侧索赔裁定状态 */
export type UserClaimDecisionStatus = 'auto-adjudicated' | 'rejected'

/** 用户侧索赔赔付状态 */
export type UserClaimPayoutStatus = 'processing' | 'paid'
export type UserClaimCompensationStatus = 'awaiting_compensation' | 'compensating' | 'compensated'

/** 索赔类型枚举 */
export type ClaimType = 'refund' | 'compensation' | 'quality_issue' | 'delivery_issue'

/** 审批类型枚举 */
export type ApprovalType = 'full' | 'partial' | 'rejected'

/** 追偿单状态枚举 */
export type ClaimRecoveryStatus = 'pending' | 'paid' | 'overdue' | 'waived' | 'disputed'
export type ClaimRecoveryReleaseStatus = 'pending' | 'released' | 'retrying' | 'syncing'
export type AppealStatusTheme = 'warning' | 'success' | 'danger'
export type ClaimRecoveryStatusTheme = 'warning' | 'success' | 'danger'

// ==================== 申诉管理相关类型 ====================

/** 申诉响应 - 基于swagger api.appealResponse */
export interface AppealResponse {
    id: number
    appellant_id: number
    appellant_type: AppellantType
    claim_id: number
    reason: string
    status: AppealStatus
    region_id: number
    compensation_amount?: number
    review_notes?: string
    reviewer_id?: number
    created_at: string
    reviewed_at?: string
    compensated_at?: string

    // 商户端异议记录扩展字段
    claim_type?: ClaimType
    claim_amount?: number
    claim_description?: string
    claim_approved_amount?: number
    order_no?: string
    order_amount?: number
    user_phone?: string
}

/** 创建申诉请求 */
export interface CreateAppealRequest extends Record<string, unknown> {
    claim_id: number
    reason: string
}

/** 申诉列表查询参数 */
export interface AppealsQueryParams extends Record<string, unknown> {
    page?: number
    limit?: number
    page_id?: number
    page_size?: number
    status?: AppealStatus
    region_id?: number
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
    status: ClaimStatus
    is_malicious: boolean
    review_notes?: string
    reviewer_id?: number
    created_at: string
    reviewed_at?: string

    // 商户/骑手索赔列表的扩展字段
    order_no?: string
    order_amount?: number
    user_phone?: string
    user_name?: string
    recovery_id?: number
    recovery_dispute_id?: number
    recovery_dispute_status?: AppealStatus
    recovery_dispute_reason?: string
    recovery_dispute_review_notes?: string
    appeal_id?: number
    appeal_status?: AppealStatus
    recovery_status?: ClaimRecoveryStatus
    appeal_reason?: string
    appeal_review_notes?: string
}

/** 商户索赔判定依据响应 */
export interface MerchantClaimDecisionResponse {
    decision: {
        decision_id: number
        responsible_party: string
        compensation_source: string
        decision_status: string
        reason_codes: string[]
        trace_summary?: string
        created_at: string
        updated_at: string
    } | null
}

/** 追偿单响应 */
export interface ClaimRecoveryResponse {
    id: number
    claim_id: number
    order_id: number
    responsible_party: string
    recovery_target?: string
    recovery_amount: number
    status: ClaimRecoveryStatus
    release_status?: ClaimRecoveryReleaseStatus
    release_message?: string
    due_at: string
    updated_at: string
}

export interface ClaimRecoveryPaymentResponse {
    recovery: ClaimRecoveryResponse
    payment_order_id: number
    out_trade_no: string
    amount: number
    status: string
    expires_at?: string
    pay_params?: MiniProgramPayParams
}

export interface MerchantUserRiskResponse {
    user_id: number
    has_block: boolean
    reason_code?: string
    block_until?: string
    reminder_text?: string
}

export interface BehaviorSummaryStat {
    entity_type: string
    entity_id: number
    total_orders: number
    abnormal_claims: number
    abnormal_rate: number
}

export interface MerchantClaimBehaviorSummaryResponse {
    order_id: number
    window: {
        start_date: string
        end_date: string
    }
    user: BehaviorSummaryStat
    merchant: BehaviorSummaryStat
    rider?: BehaviorSummaryStat
}

/** 用户索赔列表查询参数 */
export interface UserClaimsQueryParams extends Record<string, unknown> {
    page?: number
    page_size?: number
}

/** 索赔列表查询参数 */
export interface ClaimsQueryParams extends Record<string, unknown> {
    page_id: number
    page_size: number
    status?: ClaimStatus
    bucket?: 'pending_action' | 'disputed' | 'closed'
    claim_type?: ClaimType
}

// ==================== 用户索赔提交相关类型 ====================

/** 用户提交索赔类型枚举 - 对齐后端 SubmitClaimRequest.claim_type */
export type UserClaimType = 'foreign-object' | 'damage' | 'timeout'

/** 用户提交索赔请求 - 对齐后端 SubmitClaimRequest */
export interface SubmitClaimRequest extends Record<string, unknown> {
    order_id: number
    claim_type: UserClaimType
    claim_amount: number            // 单位：分，最高 100000000 (1万元)
    claim_reason: string            // 5-1000 字符
    device_fingerprint?: string     // 可选
}

/** 用户提交索赔响应 - 对齐后端 SubmitClaimResponse */
export interface SubmitClaimResponse {
    claim_id: number
    status: 'accepted'
    decision_status?: 'auto-adjudicated'
    payout_status?: 'processing' | 'paid'
    approved_amount?: number
    compensation_source?: string    // merchant | rider | platform
    reason: string
    payout_eta?: string             // 预计赔付时间
    warning?: string                // 警告信息
}

/** 用户索赔响应 - 对齐用户态 /v1/claims DTO */
export interface UserClaimResponse {
    id: number
    order_id: number
    claim_type: string
    description: string
    claim_amount: number
    approved_amount?: number
    status: UserClaimStatus
    decision_status?: UserClaimDecisionStatus
    compensation_status?: UserClaimCompensationStatus
    payout_status?: UserClaimPayoutStatus
    customer_action_required?: boolean
    customer_action?: string
    reason?: string
    payout_eta?: string
    created_at: string
    processed_at?: string
}

export interface UserClaimsListResponse {
    claims: UserClaimResponse[]
    total: number
    page: number
    page_size: number
}

export interface UserClaimPresentation {
    statusText: string
    statusTheme: string
    statusIcon: string
    statusColor: string
    summary: string
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

export interface ClaimSummaryDTO {
    total: number
    pending_action: number
    disputed: number
    appealed?: number
    closed: number
}

export interface AppealSummaryDTO {
    total: number
    submitted: number
    approved: number
    pending?: number
    compensated?: number
    rejected: number
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
            url: '/v1/merchant/recovery-disputes',
            method: 'GET',
            data: params
        })
    }

    async getMerchantAppealsSummary(): Promise<AppealSummaryDTO> {
        return request({
            url: '/v1/merchant/recovery-disputes/summary',
            method: 'GET'
        })
    }

    /**
     * 获取申诉详情
     * @param appealId 申诉ID
     */
    async getAppealDetail(appealId: number): Promise<AppealResponse> {
        return request({
            url: `/v1/merchant/recovery-disputes/${appealId}`,
            method: 'GET'
        })
    }

    /**
     * 获取商户异议详情
     * @param appealId 异议ID
     */
    async getMerchantAppealDetail(appealId: number): Promise<AppealResponse> {
        return request({
            url: `/v1/merchant/recovery-disputes/${appealId}`,
            method: 'GET'
        })
    }

    /**
     * 创建申诉
     * @param appealData 申诉数据
     */
    async createAppeal(appealData: CreateAppealRequest): Promise<AppealResponse> {
        return request({
            url: '/v1/merchant/recovery-disputes',
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
        const { page, limit, ...rest } = params
        const query = {
            ...rest,
            page_id: params.page_id || page || 1,
            page_size: params.page_size || limit || 20
        }
        const response = await request<{
            disputes: AppealResponse[]
            total: number
            page_id: number
            page_size: number
            has_more: boolean
        }>({
            url: '/v1/rider/recovery-disputes',
            method: 'GET',
            data: query
        })
        return {
            appeals: response.disputes || [],
            total: response.total,
            page_id: response.page_id,
            page_size: response.page_size,
            has_more: response.has_more
        }
    }

    /**
     * 获取骑手申诉详情
     * @param appealId 申诉ID
     */
    async getRiderAppealDetail(appealId: number): Promise<AppealResponse> {
        return request({
            url: `/v1/rider/recovery-disputes/${appealId}`,
            method: 'GET'
        })
    }

    /**
     * 骑手创建申诉
     * @param appealData 申诉数据
     */
    async createRiderAppeal(appealData: CreateAppealRequest): Promise<AppealResponse> {
        return request({
            url: '/v1/rider/recovery-disputes',
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
     * 用户提交索赔
     */
    async submitClaim(data: SubmitClaimRequest): Promise<SubmitClaimResponse> {
        return request({
            url: '/v1/claims',
            method: 'POST',
            data
        })
    }

    /**
     * 获取用户索赔列表
     * @param params 查询参数
     */
    async getUserClaims(params: UserClaimsQueryParams): Promise<UserClaimsListResponse> {
        return request({
            url: '/v1/claims',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取商户追偿单详情
     */
    async getMerchantClaimRecovery(recoveryId: number): Promise<ClaimRecoveryResponse> {
        return request({
            url: `/v1/merchant/recoveries/${recoveryId}`,
            method: 'GET'
        })
    }

    /**
     * 获取商户顾客风险提示
     */
    async getMerchantUserRisk(userId: number): Promise<MerchantUserRiskResponse> {
        return request({
            url: `/v1/merchant/risk/users/${userId}`,
            method: 'GET'
        })
    }

    /**
     * 商户支付追偿单
     */
    async payMerchantClaimRecovery(recoveryId: number): Promise<ClaimRecoveryPaymentResponse> {
        return request({
            url: `/v1/merchant/recoveries/${recoveryId}/pay`,
            method: 'POST'
        })
    }

    /**
     * 获取骑手追偿单详情
     */
    async getRiderClaimRecovery(recoveryId: number): Promise<ClaimRecoveryResponse> {
        return request({
            url: `/v1/rider/recoveries/${recoveryId}`,
            method: 'GET'
        })
    }

    /**
     * 骑手支付追偿单
     */
    async payRiderClaimRecovery(recoveryId: number): Promise<ClaimRecoveryPaymentResponse> {
        return request({
            url: `/v1/rider/recoveries/${recoveryId}/pay`,
            method: 'POST'
        })
    }

    /**
     * 获取索赔详情
     * @param claimId 索赔ID
     */
    async getClaimDetail(claimId: number): Promise<UserClaimResponse> {
        return request({
            url: `/v1/claims/${claimId}`,
            method: 'GET'
        })
    }

    async confirmContinueClaim(claimId: number): Promise<UserClaimResponse> {
        return request({
            url: `/v1/claims/${claimId}/confirm-continue`,
            method: 'POST'
        })
    }

    async withdrawClaim(claimId: number): Promise<UserClaimResponse> {
        return request({
            url: `/v1/claims/${claimId}/withdraw`,
            method: 'POST'
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

    async getMerchantClaimsSummary(): Promise<ClaimSummaryDTO> {
        return request({
            url: '/v1/merchant/claims/summary',
            method: 'GET'
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
     * 获取商户索赔责任判定依据
     * @param claimId 索赔ID
     */
    async getMerchantClaimDecision(claimId: number): Promise<MerchantClaimDecisionResponse> {
        return request({
            url: `/v1/merchant/claims/${claimId}/decision`,
            method: 'GET'
        })
    }

    /**
     * 获取骑手索赔责任判定依据
     * @param claimId 索赔ID
     */
    async getRiderClaimDecision(claimId: number): Promise<MerchantClaimDecisionResponse> {
        return request({
            url: `/v1/rider/claims/${claimId}/decision`,
            method: 'GET'
        })
    }

    /**
     * 获取商户索赔行为回溯摘要
     * @param orderId 订单ID
     */
    async getMerchantClaimBehaviorSummary(orderId: number): Promise<MerchantClaimBehaviorSummaryResponse> {
        return request({
            url: '/v1/merchant/claims/behavior-summary',
            method: 'GET',
            data: {
                order_id: orderId
            }
        })
    }

    /**
     * 获取骑手索赔行为回溯摘要
     * @param orderId 订单ID
     */
    async getRiderClaimBehaviorSummary(orderId: number): Promise<MerchantClaimBehaviorSummaryResponse> {
        return request({
            url: '/v1/rider/claims/behavior-summary',
            method: 'GET',
            data: {
                order_id: orderId
            }
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

    async getRiderClaimsSummary(): Promise<ClaimSummaryDTO> {
        return request({
            url: '/v1/rider/claims/summary',
            method: 'GET'
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
    }): CreateAppealRequest {
        return {
            claim_id: data.claimId,
            reason: data.reason
        }
    }
}

// ==================== 导出服务实例 ====================

export const appealManagementService = new AppealManagementService()
export const claimManagementService = new ClaimManagementService()
export const reviewReplyService = new ReviewReplyService()

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
    const [appealsSummary, claimsSummary, appealsResult, claimsResult, reviewsResult] = await Promise.all([
        appealManagementService.getMerchantAppealsSummary(),
        claimManagementService.getMerchantClaimsSummary(),
        appealManagementService.getMerchantAppeals({ page_id: 1, page_size: 10, status: 'submitted' }),
        claimManagementService.getMerchantClaims({ page_id: 1, page_size: 10, bucket: 'pending_action' }),
        reviewReplyService.getMerchantReviews(merchantId, { page_id: 1, page_size: 10, has_reply: false })
    ])

    const pendingCount = appealsResult.appeals.length + claimsResult.claims.length + reviewsResult.reviews.length

    return {
        pendingAppeals: appealsResult.appeals,
        pendingClaims: claimsResult.claims,
        unrepliedReviews: reviewsResult.reviews,
        stats: {
            totalAppeals: appealsSummary.total,
            totalClaims: claimsSummary.total,
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
}>): Promise<{ reviewId: number, success: boolean, message: string }[]> {
    const promises = replies.map(async ({ reviewId, reply }) => {
        try {
            await reviewReplyService.replyToReview(reviewId, { reply })
            return { reviewId, success: true, message: '回复成功' }
        } catch (error: unknown) {
            return {
                reviewId,
                success: false,
                message: error instanceof Error ? error.message : '回复失败'
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
export async function getAppealStatistics(_startDate: string, _endDate: string): Promise<{
    totalAppeals: number
    approvedAppeals: number
    rejectedAppeals: number
    pendingAppeals: number
    approvalRate: number
    avgProcessingTime: number
}> {
    const summary = await appealManagementService.getMerchantAppealsSummary()
    const totalAppeals = summary.total
    const approvedAppeals = summary.approved
    const rejectedAppeals = summary.rejected
    const pendingAppeals = summary.submitted || summary.pending || 0
    const approvalRate = totalAppeals > 0 ? (approvedAppeals / totalAppeals) * 100 : 0

    return {
        totalAppeals,
        approvedAppeals,
        rejectedAppeals,
        pendingAppeals,
        approvalRate,
        avgProcessingTime: 0
    }
}

/**
 * 验证申诉理由
 * @param reason 申诉理由
 */
export function validateAppealReason(reason: string): { valid: boolean, message?: string } {
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
export function validateReviewReply(reply: string): { valid: boolean, message?: string } {
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
        submitted: '待审核',
        approved: '已通过',
        rejected: '已拒绝'
    }
    return statusMap[status] || status
}

export function getAppealStatusDisplay(status: AppealStatus | string) {
    const normalizedStatus = String(status || 'submitted') as AppealStatus | string

    if (normalizedStatus === 'submitted') {
        return { label: '待审核', theme: 'warning' as AppealStatusTheme, isPending: true, isClosed: false, isApproved: false, isRejected: false, isCompensated: false }
    }

    if (normalizedStatus === 'approved') {
        return { label: '已通过', theme: 'success' as AppealStatusTheme, isPending: false, isClosed: true, isApproved: true, isRejected: false, isCompensated: false }
    }

    return { label: normalizedStatus === 'rejected' ? '已拒绝' : formatAppealStatus(normalizedStatus as AppealStatus), theme: 'danger' as AppealStatusTheme, isPending: false, isClosed: true, isApproved: false, isRejected: normalizedStatus === 'rejected', isCompensated: false }
}

export function getClaimRecoveryStatusDisplay(status: ClaimRecoveryStatus | string) {
    const normalizedStatus = String(status || 'pending') as ClaimRecoveryStatus | string

    if (normalizedStatus === 'pending' || normalizedStatus === 'overdue') {
        return {
            label: normalizedStatus === 'overdue' ? '已逾期' : '待追偿',
            theme: 'warning' as ClaimRecoveryStatusTheme,
            canWaive: true
        }
    }

    if (normalizedStatus === 'paid' || normalizedStatus === 'waived') {
        return {
            label: normalizedStatus === 'paid' ? '已支付' : '已核销',
            theme: 'success' as ClaimRecoveryStatusTheme,
            canWaive: false
        }
    }

    return {
        label: normalizedStatus === 'disputed' ? '异议中' : normalizedStatus,
        theme: 'danger' as ClaimRecoveryStatusTheme,
        canWaive: false
    }
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

export function getUserClaimPresentation(claim: Pick<UserClaimResponse, 'status' | 'decision_status' | 'payout_status' | 'customer_action_required' | 'customer_action'>): UserClaimPresentation {
    if (claim.status === 'withdrawn') {
        return {
            statusText: '已撤回',
            statusTheme: 'warning',
            statusIcon: 'rollback',
            statusColor: '#ff9800',
            summary: '您已撤回本次反馈，系统不会继续赔付处理。'
        }
    }

    if (claim.customer_action_required && claim.customer_action === 'confirm_continue') {
        return {
            statusText: '待确认赔付',
            statusTheme: 'warning',
            statusIcon: 'time-filled',
            statusColor: '#ff9800',
            summary: '平台已完成核验，请确认是否继续进入赔付处理。'
        }
    }

    if (claim.status === 'rejected' || claim.decision_status === 'rejected') {
        return {
            statusText: '未支持赔付',
            statusTheme: 'danger',
            statusIcon: 'close-circle-filled',
            statusColor: '#d32f2f',
            summary: '平台已完成核验，当前未支持本次赔付。'
        }
    }

    if (claim.payout_status === 'paid') {
        return {
            statusText: '赔付已到账',
            statusTheme: 'success',
            statusIcon: 'check-circle-filled',
            statusColor: '#2e7d32',
            summary: '平台已完成自动裁定，赔付已到账。'
        }
    }

    if (claim.decision_status === 'auto-adjudicated') {
        return {
            statusText: '已自动裁定',
            statusTheme: 'primary',
            statusIcon: 'check-circle-filled',
            statusColor: '#1976d2',
            summary: '平台已完成自动裁定，赔付正在处理中。'
        }
    }

    return {
        statusText: '平台已受理',
        statusTheme: 'warning',
        statusIcon: 'time-filled',
        statusColor: '#ff9800',
        summary: '平台已受理您的反馈，正在处理中。'
    }
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
        delivery_issue: '代取问题'
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
