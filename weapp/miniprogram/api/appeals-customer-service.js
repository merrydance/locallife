"use strict";
/**
 * 申诉和客服接口重构 (Task 2.8)
 * 基于swagger.json完全重构，移除所有没有后端支持的旧功能
 * 包含：申诉管理、索赔处理、评价回复
 */
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
Object.defineProperty(exports, "__esModule", { value: true });
exports.operatorAppealReviewService = exports.reviewReplyService = exports.claimManagementService = exports.appealManagementService = exports.AppealsCustomerServiceAdapter = exports.OperatorAppealReviewService = exports.ReviewReplyService = exports.ClaimManagementService = exports.AppealManagementService = void 0;
exports.getMerchantCustomerServiceDashboard = getMerchantCustomerServiceDashboard;
exports.batchReplyReviews = batchReplyReviews;
exports.getAppealStatistics = getAppealStatistics;
exports.validateAppealReason = validateAppealReason;
exports.validateReviewReply = validateReviewReply;
exports.formatAppealStatus = formatAppealStatus;
exports.formatClaimStatus = formatClaimStatus;
exports.formatClaimType = formatClaimType;
exports.calculateAppealProcessingTime = calculateAppealProcessingTime;
const request_1 = require("../utils/request");
// ==================== 申诉管理服务类 ====================
/**
 * 申诉管理服务
 * 提供申诉的查询、创建、处理等功能
 */
class AppealManagementService {
    /**
     * 获取商户申诉列表
     * @param params 查询参数
     */
    getMerchantAppeals(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/merchant/appeals',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取申诉详情
     * @param appealId 申诉ID
     */
    getAppealDetail(appealId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/merchant/appeals/${appealId}`,
                method: 'GET'
            });
        });
    }
    /**
     * 创建申诉
     * @param appealData 申诉数据
     */
    createAppeal(appealData) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/merchant/appeals',
                method: 'POST',
                data: appealData
            });
        });
    }
    /**
     * 获取骑手申诉列表
     * @param params 查询参数
     */
    getRiderAppeals(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/rider/appeals',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取骑手申诉详情
     * @param appealId 申诉ID
     */
    getRiderAppealDetail(appealId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/rider/appeals/${appealId}`,
                method: 'GET'
            });
        });
    }
    /**
     * 骑手创建申诉
     * @param appealData 申诉数据
     */
    createRiderAppeal(appealData) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/rider/appeals',
                method: 'POST',
                data: appealData
            });
        });
    }
}
exports.AppealManagementService = AppealManagementService;
// ==================== 索赔管理服务类 ====================
/**
 * 索赔管理服务
 * 提供索赔的查询、处理等功能
 */
class ClaimManagementService {
    /**
     * 获取用户索赔列表
     * @param params 查询参数
     */
    getUserClaims(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/claims',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取索赔详情
     * @param claimId 索赔ID
     */
    getClaimDetail(claimId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/claims/${claimId}`,
                method: 'GET'
            });
        });
    }
    /**
     * 获取商户收到的索赔列表
     * @param params 查询参数
     */
    getMerchantClaims(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/merchant/claims',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取商户索赔详情
     * @param claimId 索赔ID
     */
    getMerchantClaimDetail(claimId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/merchant/claims/${claimId}`,
                method: 'GET'
            });
        });
    }
    /**
     * 获取骑手索赔列表
     * @param params 查询参数
     */
    getRiderClaims(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/rider/claims',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取骑手索赔详情
     * @param claimId 索赔ID
     */
    getRiderClaimDetail(claimId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/rider/claims/${claimId}`,
                method: 'GET'
            });
        });
    }
}
exports.ClaimManagementService = ClaimManagementService;
// ==================== 评价回复服务类 ====================
/**
 * 评价回复服务
 * 提供评价查询和回复功能
 */
class ReviewReplyService {
    /**
     * 获取商户评价列表
     * @param merchantId 商户ID
     * @param params 查询参数
     */
    getMerchantReviews(merchantId, params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/reviews/merchants/${merchantId}`,
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取评价详情
     * @param reviewId 评价ID
     */
    getReviewDetail(reviewId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/reviews/${reviewId}`,
                method: 'GET'
            });
        });
    }
    /**
     * 回复评价
     * @param reviewId 评价ID
     * @param replyData 回复内容
     */
    replyToReview(reviewId, replyData) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/reviews/${reviewId}/reply`,
                method: 'POST',
                data: replyData
            });
        });
    }
}
exports.ReviewReplyService = ReviewReplyService;
// ==================== 运营商申诉审核服务类 ====================
/**
 * 运营商申诉审核服务
 * 提供申诉审核功能（仅运营商使用）
 */
class OperatorAppealReviewService {
    /**
     * 获取待审核申诉列表
     * @param params 查询参数
     */
    getPendingAppeals(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/operator/appeals',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取申诉详情
     * @param appealId 申诉ID
     */
    getAppealDetailForReview(appealId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/operator/appeals/${appealId}`,
                method: 'GET'
            });
        });
    }
    /**
     * 审核申诉
     * @param appealId 申诉ID
     * @param reviewData 审核数据
     */
    reviewAppeal(appealId, reviewData) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/operator/appeals/${appealId}/review`,
                method: 'POST',
                data: reviewData
            });
        });
    }
}
exports.OperatorAppealReviewService = OperatorAppealReviewService;
// ==================== 数据适配器 ====================
/**
 * 申诉和客服数据适配器
 * 处理前端数据格式与后端API数据格式的转换
 */
class AppealsCustomerServiceAdapter {
    /**
     * 适配申诉响应数据
     */
    static adaptAppealResponse(data) {
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
        };
    }
    /**
     * 适配索赔响应数据
     */
    static adaptClaimResponse(data) {
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
        };
    }
    /**
     * 适配评价响应数据
     */
    static adaptReviewResponse(data) {
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
        };
    }
    /**
     * 适配创建申诉请求数据
     */
    static adaptCreateAppealRequest(data) {
        return {
            claim_id: data.claimId,
            reason: data.reason,
            evidence_urls: data.evidenceUrls
        };
    }
}
exports.AppealsCustomerServiceAdapter = AppealsCustomerServiceAdapter;
// ==================== 导出服务实例 ====================
exports.appealManagementService = new AppealManagementService();
exports.claimManagementService = new ClaimManagementService();
exports.reviewReplyService = new ReviewReplyService();
exports.operatorAppealReviewService = new OperatorAppealReviewService();
// ==================== 便捷函数 ====================
/**
 * 获取商户客服工作台数据
 * @param merchantId 商户ID
 */
function getMerchantCustomerServiceDashboard(merchantId) {
    return __awaiter(this, void 0, void 0, function* () {
        const [appealsResult, claimsResult, reviewsResult] = yield Promise.all([
            exports.appealManagementService.getMerchantAppeals({ page_id: 1, page_size: 10, status: 'pending' }),
            exports.claimManagementService.getMerchantClaims({ page_id: 1, page_size: 10, status: 'pending' }),
            exports.reviewReplyService.getMerchantReviews(merchantId, { page_id: 1, page_size: 10, has_reply: false })
        ]);
        const pendingCount = appealsResult.appeals.length + claimsResult.claims.length + reviewsResult.reviews.length;
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
        };
    });
}
/**
 * 批量回复评价
 * @param replies 回复列表
 */
function batchReplyReviews(replies) {
    return __awaiter(this, void 0, void 0, function* () {
        const promises = replies.map((_a) => __awaiter(this, [_a], void 0, function* ({ reviewId, reply }) {
            try {
                yield exports.reviewReplyService.replyToReview(reviewId, { reply });
                return { reviewId, success: true, message: '回复成功' };
            }
            catch (error) {
                return {
                    reviewId,
                    success: false,
                    message: (error === null || error === void 0 ? void 0 : error.message) || '回复失败'
                };
            }
        }));
        return Promise.all(promises);
    });
}
/**
 * 获取申诉统计信息
 * @param startDate 开始日期
 * @param endDate 结束日期
 */
function getAppealStatistics(startDate, endDate) {
    return __awaiter(this, void 0, void 0, function* () {
        // 这里需要根据实际的统计接口来实现
        // 目前swagger中没有专门的统计接口，可能需要通过查询所有数据来计算
        const appealsResult = yield exports.appealManagementService.getMerchantAppeals({
            page_id: 1,
            page_size: 1000 // 获取大量数据用于统计
        });
        const appeals = appealsResult.appeals;
        const totalAppeals = appeals.length;
        const approvedAppeals = appeals.filter(a => a.status === 'approved').length;
        const rejectedAppeals = appeals.filter(a => a.status === 'rejected').length;
        const pendingAppeals = appeals.filter(a => a.status === 'pending').length;
        const approvalRate = totalAppeals > 0 ? (approvedAppeals / totalAppeals) * 100 : 0;
        // 计算平均处理时间（天）
        const processedAppeals = appeals.filter(a => a.reviewed_at);
        const avgProcessingTime = processedAppeals.length > 0
            ? processedAppeals.reduce((sum, appeal) => {
                const created = new Date(appeal.created_at);
                const reviewed = new Date(appeal.reviewed_at);
                const diffDays = (reviewed.getTime() - created.getTime()) / (1000 * 60 * 60 * 24);
                return sum + diffDays;
            }, 0) / processedAppeals.length
            : 0;
        return {
            totalAppeals,
            approvedAppeals,
            rejectedAppeals,
            pendingAppeals,
            approvalRate,
            avgProcessingTime
        };
    });
}
/**
 * 验证申诉理由
 * @param reason 申诉理由
 */
function validateAppealReason(reason) {
    if (!reason || reason.trim().length === 0) {
        return { valid: false, message: '申诉理由不能为空' };
    }
    if (reason.length < 10) {
        return { valid: false, message: '申诉理由至少需要10个字符' };
    }
    if (reason.length > 1000) {
        return { valid: false, message: '申诉理由不能超过1000个字符' };
    }
    return { valid: true };
}
/**
 * 验证评价回复
 * @param reply 回复内容
 */
function validateReviewReply(reply) {
    if (!reply || reply.trim().length === 0) {
        return { valid: false, message: '回复内容不能为空' };
    }
    if (reply.length > 500) {
        return { valid: false, message: '回复内容不能超过500个字符' };
    }
    return { valid: true };
}
/**
 * 格式化申诉状态显示
 * @param status 申诉状态
 */
function formatAppealStatus(status) {
    const statusMap = {
        pending: '待审核',
        approved: '已通过',
        rejected: '已拒绝',
        compensated: '已赔付'
    };
    return statusMap[status] || status;
}
/**
 * 格式化索赔状态显示
 * @param status 索赔状态
 */
function formatClaimStatus(status) {
    const statusMap = {
        pending: '待审核',
        approved: '已通过',
        rejected: '已拒绝',
        compensated: '已赔付'
    };
    return statusMap[status] || status;
}
/**
 * 格式化索赔类型显示
 * @param claimType 索赔类型
 */
function formatClaimType(claimType) {
    const typeMap = {
        refund: '退款',
        compensation: '赔偿',
        quality_issue: '质量问题',
        delivery_issue: '配送问题'
    };
    return typeMap[claimType] || claimType;
}
/**
 * 计算申诉处理时效
 * @param createdAt 创建时间
 * @param reviewedAt 审核时间
 */
function calculateAppealProcessingTime(createdAt, reviewedAt) {
    const created = new Date(createdAt);
    const reviewed = reviewedAt ? new Date(reviewedAt) : new Date();
    const diffMs = reviewed.getTime() - created.getTime();
    const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));
    const diffHours = Math.floor((diffMs % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60));
    // 假设申诉处理时效为3个工作日
    const isOverdue = diffDays > 3;
    return {
        days: diffDays,
        hours: diffHours,
        isOverdue
    };
}
