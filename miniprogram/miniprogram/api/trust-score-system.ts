/**
 * 信任分和风控接口重构 (Task 5.3)
 * 基于swagger.json完全重构，移除所有没有后端支持的旧功能
 * 包含：信任分系统、风控检测、申诉审核、食品安全
 */

import { request } from '../utils/request'

// ==================== 数据类型定义 ====================

/** 用户角色枚举 */
export type UserRole = 'customer' | 'merchant' | 'rider' | 'operator'

/** 信任分等级枚举 */
export type TrustLevel = 'excellent' | 'good' | 'average' | 'poor' | 'critical'

/** 风控状态枚举 */
export type RiskStatus = 'safe' | 'warning' | 'high_risk' | 'blocked'

/** 申诉状态枚举 */
export type AppealStatus = 'pending' | 'processing' | 'resolved' | 'rejected'

/** 食品安全等级枚举 */
export type FoodSafetyLevel = 'A' | 'B' | 'C' | 'D'

// ==================== 信任分系统相关类型 ====================

/** 信任分档案响应 - 基于swagger api.trustScoreProfileResponse */
export interface TrustScoreProfileResponse {
    user_id: number
    role: UserRole
    current_score: number
    level: TrustLevel
    last_updated: string
    score_breakdown: {
        base_score: number
        behavior_score: number
        compliance_score: number
        service_score: number
        penalty_score: number
    }
    risk_factors: Array<{
        factor: string
        impact: number
        description: string
        created_at: string
    }>
    recent_changes: Array<{
        change_type: 'increase' | 'decrease'
        amount: number
        reason: string
        created_at: string
    }>
    restrictions: Array<{
        type: string
        description: string
        start_date: string
        end_date?: string
        is_active: boolean
    }>
}

/** 信任分历史响应 - 基于swagger api.trustScoreHistoryResponse */
export interface TrustScoreHistoryResponse {
    history: Array<{
        id: number
        user_id: number
        role: UserRole
        score_before: number
        score_after: number
        change_amount: number
        change_reason: string
        change_type: 'manual' | 'automatic' | 'system'
        operator_id?: number
        operator_name?: string
        created_at: string
        details?: Record<string, any>
    }>
    total: number
    page: number
    limit: number
    has_more: boolean
}

// ==================== 风控检测相关类型 ====================

/** 风控检测请求 - 基于swagger api.fraudDetectionRequest */
export interface FraudDetectionRequest extends Record<string, unknown> {
    user_id: number
    role: UserRole
    action_type: string
    context_data: Record<string, any>
    ip_address?: string
    device_info?: Record<string, any>
}

/** 风控检测响应 - 基于swagger api.fraudDetectionResponse */
export interface FraudDetectionResponse {
    risk_level: 'low' | 'medium' | 'high' | 'critical'
    risk_score: number
    is_blocked: boolean
    detected_patterns: Array<{
        pattern_type: string
        confidence: number
        description: string
    }>
    recommended_actions: Array<{
        action: string
        priority: 'low' | 'medium' | 'high'
        description: string
    }>
    additional_verification_required: boolean
    verification_methods: string[]
}

/** 商户暂停请求 - 对齐 api.SuspendMerchantRequest */
export interface SuspendMerchantRequest extends Record<string, unknown> {
    admin_id: number                             // 管理员ID
    duration_hours: number                       // 暂停时长（小时，最长30天=720小时）
    merchant_id: number                          // 商户ID
    reason: string                               // 暂停原因
}

/** 食品安全报告请求 - 对齐 api.ReportFoodSafetyRequest */
export interface ReportFoodSafetyRequest extends Record<string, unknown> {
    merchant_id: number                          // 商户ID（必填）
    order_id: number                             // 订单ID（必填）
    reporter_id: number                          // 报告人ID（必填）
    incident_type: 'foreign-object' | 'contamination' | 'expired'  // 事件类型（必填）
    severity_level: number                       // 严重程度（1-5，必填）
    description: string                          // 描述（10-1000字符，必填）
    evidence_photos: string                      // 证据图片（最大500字符，必填）
}

/** 食品安全报告响应 - 对齐 api.ReportFoodSafetyResponse */
export interface ReportFoodSafetyResponse {
    incident_id: number                          // 事件ID
    merchant_suspended: boolean                  // 商户是否被暂停
    message: string                              // 消息
    suspend_duration?: number                    // 暂停时长（小时）
}

/** 触发欺诈检测请求 - 对齐 api.TriggerFraudDetectionRequest */
export interface TriggerFraudDetectionRequest extends Record<string, unknown> {
    claim_id?: number                            // 申诉ID
    address_id?: number                          // 地址ID
    device_fingerprint?: string                  // 设备指纹
}

/** 提交申诉请求 - 对齐 api.SubmitAppealRequest */
export interface SubmitAppealRequest extends Record<string, unknown> {
    entity_type: 'customer' | 'merchant' | 'rider'  // 实体类型（必填）
    entity_id: number                            // 实体ID（必填）
    appeal_reason: string                        // 申诉原因（10-1000字符，必填）
    evidence?: string                            // 证据（最大500字符）
}

/** 提交索赔请求 - 对齐 api.SubmitClaimRequest */
export interface SubmitClaimRequest extends Record<string, unknown> {
    order_id: number                             // 订单ID（必填）
    claim_type: 'foreign-object' | 'damage' | 'timeout' | 'food-safety'  // 索赔类型（必填）
    claim_amount: number                         // 索赔金额（分，1-100000000，必填）
    claim_reason: string                         // 索赔原因（5-1000字符，必填）
    evidence_photos?: string[]                   // 证据图片（最多10张）
}

/** 提交索赔响应 - 对齐 api.SubmitClaimResponse */
export interface SubmitClaimResponse {
    claim_id: number                             // 索赔ID
    status: string                               // 状态：instant, auto, manual, evidence-required, platform-pay
    approved_amount?: number                     // 批准金额（分）
    compensation_source?: string                 // 赔偿来源：merchant, rider, platform
    needs_evidence?: boolean                     // 是否需要证据
    refund_eta?: string                          // 预计到账时间
    reason?: string                              // 原因
    warning?: string                             // 警告信息
}

/** 提交恢复请求 - 对齐 api.SubmitRecoveryRequest */
export interface SubmitRecoveryRequest extends Record<string, unknown> {
    entity_type: 'merchant' | 'rider'            // 实体类型（必填）
    entity_id: number                            // 实体ID（必填）
    commitment_message: string                   // 改善承诺（10-500字符，必填）
}

// ==================== 申诉审核相关类型 ====================

/** 信任分申诉请求 - 基于swagger api.trustScoreAppealRequest */
export interface TrustScoreAppealRequest extends Record<string, unknown> {
    user_id: number
    role: UserRole
    appeal_type: 'score_dispute' | 'penalty_dispute' | 'restriction_dispute'
    description: string
    evidence_files?: string[]
    requested_action: string
}

/** 信任分申诉响应 - 基于swagger api.trustScoreAppealResponse */
export interface TrustScoreAppealResponse {
    id: number
    user_id: number
    role: UserRole
    appeal_type: string
    description: string
    status: AppealStatus
    evidence_files: string[]
    requested_action: string
    created_at: string
    updated_at: string
    reviewed_at?: string
    reviewer_id?: number
    reviewer_name?: string
    review_result?: string
    compensation?: {
        score_adjustment: number
        restrictions_removed: string[]
        other_benefits: string[]
    }
}

/** 索赔审核请求 - 对齐 api.ReviewClaimRequest */
export interface ReviewClaimRequest extends Record<string, unknown> {
    approved: boolean                            // 是否通过
    approved_amount?: number                     // 审核通过金额（分）
    review_note: string                          // 审核备注
}

// ==================== 食品安全相关类型 ====================

/** 食品安全报告请求 - 基于swagger api.foodSafetyReportRequest */
export interface FoodSafetyReportRequest extends Record<string, unknown> {
    merchant_id: number
    reporter_id: number
    report_type: 'hygiene' | 'quality' | 'safety' | 'other'
    description: string
    severity: 'low' | 'medium' | 'high' | 'critical'
    evidence_files?: string[]
    location?: string
    incident_time?: string
}

/** 食品安全报告响应 - 基于swagger api.foodSafetyReportResponse */
export interface FoodSafetyReportResponse {
    id: number
    merchant_id: number
    merchant_name: string
    reporter_id: number
    reporter_name: string
    report_type: string
    description: string
    severity: string
    status: 'pending' | 'investigating' | 'resolved' | 'dismissed'
    evidence_files: string[]
    location?: string
    incident_time?: string
    created_at: string
    updated_at: string
    investigation_notes?: string
    resolution?: string
    safety_rating_impact?: number
}

// ==================== 信任分恢复相关类型 ====================

/** 信任分恢复请求 - 基于swagger api.trustScoreRecoveryRequest */
export interface TrustScoreRecoveryRequest extends Record<string, unknown> {
    user_id: number
    role: UserRole
    recovery_type: 'training_completion' | 'good_behavior' | 'manual_adjustment'
    evidence?: Record<string, any>
    notes?: string
}

// ==================== 信任分系统服务类 ====================

/**
 * 信任分系统服务
 * 提供信任分查询、历史记录等功能
 */
export class TrustScoreSystemService {
    /**
     * 获取用户信任分档案
     * @param role 用户角色
     * @param userId 用户ID
     */
    async getTrustScoreProfile(role: UserRole, userId: number): Promise<TrustScoreProfileResponse> {
        return request({
            url: `/v1/trust-score/profiles/${role}/${userId}`,
            method: 'GET'
        })
    }

    /**
     * 获取信任分历史记录
     * @param role 用户角色
     * @param userId 用户ID
     * @param page 页码
     * @param limit 每页数量
     */
    async getTrustScoreHistory(
        role: UserRole,
        userId: number,
        page: number = 1,
        limit: number = 20
    ): Promise<TrustScoreHistoryResponse> {
        return request({
            url: `/v1/trust-score/history/${role}/${userId}`,
            method: 'GET',
            data: { page, limit }
        })
    }

    /**
     * 提交信任分申诉
     * @param appealData 申诉数据
     */
    async submitTrustScoreAppeal(appealData: TrustScoreAppealRequest): Promise<TrustScoreAppealResponse> {
        return request({
            url: '/v1/trust-score/appeals',
            method: 'POST',
            data: appealData
        })
    }

    /**
     * 提交索赔申请
     * @param claimData 索赔数据
     */
    async submitClaim(claimData: any): Promise<any> {
        return request({
            url: '/v1/trust-score/claims',
            method: 'POST',
            data: claimData
        })
    }

    /**
     * 审核索赔申请
     * @param claimId 索赔ID
     * @param reviewData 审核数据
     */
    async reviewClaim(claimId: number, reviewData: ReviewClaimRequest): Promise<any> {
        return request({
            url: `/v1/trust-score/claims/${claimId}/review`,
            method: 'PATCH',
            data: reviewData
        })
    }

    /**
     * 申请信任分恢复
     * @param recoveryData 恢复数据
     */
    async requestTrustScoreRecovery(recoveryData: TrustScoreRecoveryRequest): Promise<any> {
        return request({
            url: '/v1/trust-score/recovery',
            method: 'POST',
            data: recoveryData
        })
    }
}

// ==================== 风控检测服务类 ====================

/**
 * 风控检测服务
 * 提供风险检测、商户暂停等功能
 */
export class FraudDetectionService {
    /**
     * 执行风控检测
     * @param detectionData 检测数据
     */
    async detectFraud(detectionData: FraudDetectionRequest): Promise<FraudDetectionResponse> {
        return request({
            url: '/v1/trust-score/fraud/detect',
            method: 'POST',
            data: detectionData
        })
    }

    /**
     * 暂停商户
     * @param merchantId 商户ID
     * @param suspendData 暂停数据
     */
    async suspendMerchant(merchantId: number, suspendData: SuspendMerchantRequest): Promise<any> {
        return request({
            url: `/v1/trust-score/merchants/${merchantId}/suspend`,
            method: 'PATCH',
            data: suspendData
        })
    }
}

// ==================== 食品安全服务类 ====================

/**
 * 食品安全服务
 * 提供食品安全报告等功能
 */
export class FoodSafetyService {
    /**
     * 提交食品安全报告
     * @param reportData 报告数据
     */
    async submitFoodSafetyReport(reportData: FoodSafetyReportRequest): Promise<FoodSafetyReportResponse> {
        return request({
            url: '/v1/trust-score/food-safety/report',
            method: 'POST',
            data: reportData
        })
    }
}

export const trustScoreSystemService = new TrustScoreSystemService()
export const fraudDetectionService = new FraudDetectionService()
export const foodSafetyService = new FoodSafetyService()
// ==================== 信任分分析服务类 ====================

/**
 * 信任分分析服务
 * 提供信任分分析、风险评估等功能
 */
export class TrustScoreAnalyticsService {
    /**
     * 分析用户信任分状况
     * @param profile 信任分档案
     */
    analyzeTrustScoreStatus(profile: TrustScoreProfileResponse): {
        healthStatus: 'healthy' | 'warning' | 'critical'
        riskLevel: 'low' | 'medium' | 'high'
        recommendations: string[]
        nextReviewDate: string
        improvementPlan: Array<{
            action: string
            expectedImpact: number
            timeframe: string
            priority: 'high' | 'medium' | 'low'
        }>
        restrictions: {
            active: number
            expiringSoon: number
            permanent: number
        }
    } {
        const currentScore = profile.current_score
        const riskFactors = profile.risk_factors
        const restrictions = profile.restrictions

        // 评估健康状况
        let healthStatus: 'healthy' | 'warning' | 'critical' = 'healthy'
        if (currentScore < 60) healthStatus = 'critical'
        else if (currentScore < 80) healthStatus = 'warning'

        // 评估风险等级
        let riskLevel: 'low' | 'medium' | 'high' = 'low'
        const highRiskFactors = riskFactors.filter(factor => Math.abs(factor.impact) > 10)
        if (highRiskFactors.length > 2) riskLevel = 'high'
        else if (highRiskFactors.length > 0) riskLevel = 'medium'

        // 生成建议
        const recommendations = this.generateTrustScoreRecommendations(profile)

        // 计算下次审核日期
        const nextReviewDate = this.calculateNextReviewDate(profile)

        // 生成改善计划
        const improvementPlan = this.generateImprovementPlan(profile)

        // 统计限制情况
        const activeRestrictions = restrictions.filter(r => r.is_active)
        const now = new Date()
        const expiringSoon = activeRestrictions.filter(r => {
            if (!r.end_date) return false
            const endDate = new Date(r.end_date)
            const daysUntilExpiry = (endDate.getTime() - now.getTime()) / (1000 * 60 * 60 * 24)
            return daysUntilExpiry <= 7 && daysUntilExpiry > 0
        })
        const permanent = activeRestrictions.filter(r => !r.end_date)

        return {
            healthStatus,
            riskLevel,
            recommendations,
            nextReviewDate,
            improvementPlan,
            restrictions: {
                active: activeRestrictions.length,
                expiringSoon: expiringSoon.length,
                permanent: permanent.length
            }
        }
    }

    /**
     * 分析风控检测结果
     * @param detection 风控检测结果
     */
    analyzeFraudDetection(detection: FraudDetectionResponse): {
        riskAssessment: {
            level: string
            score: number
            confidence: number
        }
        threatAnalysis: {
            primaryThreats: string[]
            riskPatterns: string[]
            urgencyLevel: 'low' | 'medium' | 'high' | 'critical'
        }
        actionPlan: {
            immediate: string[]
            shortTerm: string[]
            monitoring: string[]
        }
        verificationNeeded: boolean
        estimatedResolutionTime: string
    } {
        const riskScore = detection.risk_score
        const patterns = detection.detected_patterns
        const actions = detection.recommended_actions

        // 风险评估
        const riskAssessment = {
            level: detection.risk_level,
            score: riskScore,
            confidence: patterns.length > 0 ?
                patterns.reduce((sum, p) => sum + p.confidence, 0) / patterns.length : 0
        }

        // 威胁分析
        const primaryThreats = patterns
            .filter(p => p.confidence > 0.7)
            .map(p => p.pattern_type)

        const riskPatterns = patterns.map(p => p.description)

        let urgencyLevel: 'low' | 'medium' | 'high' | 'critical' = 'low'
        if (detection.is_blocked) urgencyLevel = 'critical'
        else if (riskScore > 80) urgencyLevel = 'high'
        else if (riskScore > 60) urgencyLevel = 'medium'

        // 行动计划
        const immediateActions = actions
            .filter(a => a.priority === 'high')
            .map(a => a.action)

        const shortTermActions = actions
            .filter(a => a.priority === 'medium')
            .map(a => a.action)

        const monitoringActions = actions
            .filter(a => a.priority === 'low')
            .map(a => a.action)

        // 预估解决时间
        let estimatedResolutionTime = '1-2小时'
        if (urgencyLevel === 'critical') estimatedResolutionTime = '立即处理'
        else if (urgencyLevel === 'high') estimatedResolutionTime = '30分钟内'
        else if (urgencyLevel === 'medium') estimatedResolutionTime = '2-4小时'
        else estimatedResolutionTime = '24小时内'

        return {
            riskAssessment,
            threatAnalysis: {
                primaryThreats,
                riskPatterns,
                urgencyLevel
            },
            actionPlan: {
                immediate: immediateActions,
                shortTerm: shortTermActions,
                monitoring: monitoringActions
            },
            verificationNeeded: detection.additional_verification_required,
            estimatedResolutionTime
        }
    }

    /**
     * 生成信任分建议
     */
    private generateTrustScoreRecommendations(profile: TrustScoreProfileResponse): string[] {
        const recommendations: string[] = []
        const score = profile.current_score
        const breakdown = profile.score_breakdown
        const riskFactors = profile.risk_factors

        if (score < 60) {
            recommendations.push('信任分过低，建议立即采取改善措施')
        }

        if (breakdown.behavior_score < 70) {
            recommendations.push('行为评分偏低，建议规范操作行为')
        }

        if (breakdown.compliance_score < 80) {
            recommendations.push('合规评分不足，建议加强规则学习')
        }

        if (breakdown.service_score < 75) {
            recommendations.push('服务评分有待提升，建议改善服务质量')
        }

        if (riskFactors.length > 3) {
            recommendations.push('风险因素较多，建议逐项处理')
        }

        if (profile.restrictions.some(r => r.is_active)) {
            recommendations.push('存在活跃限制，建议尽快解除')
        }

        return recommendations
    }

    /**
     * 计算下次审核日期
     */
    private calculateNextReviewDate(profile: TrustScoreProfileResponse): string {
        const lastUpdated = new Date(profile.last_updated)
        const score = profile.current_score

        // 根据信任分确定审核周期
        let reviewCycleDays = 30 // 默认30天
        if (score < 60) reviewCycleDays = 7 // 低分用户每周审核
        else if (score < 80) reviewCycleDays = 14 // 中等分数用户两周审核
        else if (score >= 90) reviewCycleDays = 60 // 高分用户两月审核

        const nextReview = new Date(lastUpdated.getTime() + reviewCycleDays * 24 * 60 * 60 * 1000)
        return nextReview.toISOString().split('T')[0]
    }

    /**
     * 生成改善计划
     */
    private generateImprovementPlan(profile: TrustScoreProfileResponse): Array<{
        action: string
        expectedImpact: number
        timeframe: string
        priority: 'high' | 'medium' | 'low'
    }> {
        const plan: Array<{
            action: string
            expectedImpact: number
            timeframe: string
            priority: 'high' | 'medium' | 'low'
        }> = []

        const breakdown = profile.score_breakdown
        const riskFactors = profile.risk_factors

        // 基于评分分解生成改善计划
        if (breakdown.behavior_score < 70) {
            plan.push({
                action: '完成行为规范培训',
                expectedImpact: 10,
                timeframe: '1周',
                priority: 'high'
            })
        }

        if (breakdown.compliance_score < 80) {
            plan.push({
                action: '学习平台规则并通过考试',
                expectedImpact: 8,
                timeframe: '3天',
                priority: 'high'
            })
        }

        if (breakdown.service_score < 75) {
            plan.push({
                action: '提升服务质量，获得更多好评',
                expectedImpact: 12,
                timeframe: '2周',
                priority: 'medium'
            })
        }

        // 基于风险因素生成改善计划
        riskFactors.forEach(factor => {
            if (Math.abs(factor.impact) > 5) {
                plan.push({
                    action: `处理风险因素：${factor.factor}`,
                    expectedImpact: Math.abs(factor.impact),
                    timeframe: '1周',
                    priority: Math.abs(factor.impact) > 10 ? 'high' : 'medium'
                })
            }
        })

        return plan.sort((a, b) => {
            const priorityOrder = { high: 3, medium: 2, low: 1 }
            return priorityOrder[b.priority] - priorityOrder[a.priority]
        })
    }
}

// ==================== 数据适配器 ====================

/**
 * 信任分系统数据适配器
 * 处理前端数据格式与后端API数据格式的转换
 */
export class TrustScoreSystemAdapter {
    /**
     * 适配信任分档案数据
     */
    static adaptTrustScoreProfile(data: TrustScoreProfileResponse): {
        userId: number
        role: UserRole
        currentScore: number
        level: TrustLevel
        lastUpdated: string
        scoreBreakdown: {
            baseScore: number
            behaviorScore: number
            complianceScore: number
            serviceScore: number
            penaltyScore: number
        }
        riskFactors: Array<{
            factor: string
            impact: number
            description: string
            createdAt: string
        }>
        recentChanges: Array<{
            changeType: 'increase' | 'decrease'
            amount: number
            reason: string
            createdAt: string
        }>
        restrictions: Array<{
            type: string
            description: string
            startDate: string
            endDate?: string
            isActive: boolean
        }>
    } {
        return {
            userId: data.user_id,
            role: data.role,
            currentScore: data.current_score,
            level: data.level,
            lastUpdated: data.last_updated,
            scoreBreakdown: {
                baseScore: data.score_breakdown.base_score,
                behaviorScore: data.score_breakdown.behavior_score,
                complianceScore: data.score_breakdown.compliance_score,
                serviceScore: data.score_breakdown.service_score,
                penaltyScore: data.score_breakdown.penalty_score
            },
            riskFactors: data.risk_factors.map(factor => ({
                factor: factor.factor,
                impact: factor.impact,
                description: factor.description,
                createdAt: factor.created_at
            })),
            recentChanges: data.recent_changes.map(change => ({
                changeType: change.change_type,
                amount: change.amount,
                reason: change.reason,
                createdAt: change.created_at
            })),
            restrictions: data.restrictions.map(restriction => ({
                type: restriction.type,
                description: restriction.description,
                startDate: restriction.start_date,
                endDate: restriction.end_date,
                isActive: restriction.is_active
            }))
        }
    }

    /**
     * 适配风控检测响应数据
     */
    static adaptFraudDetectionResponse(data: FraudDetectionResponse): {
        riskLevel: 'low' | 'medium' | 'high' | 'critical'
        riskScore: number
        isBlocked: boolean
        detectedPatterns: Array<{
            patternType: string
            confidence: number
            description: string
        }>
        recommendedActions: Array<{
            action: string
            priority: 'low' | 'medium' | 'high'
            description: string
        }>
        additionalVerificationRequired: boolean
        verificationMethods: string[]
    } {
        return {
            riskLevel: data.risk_level,
            riskScore: data.risk_score,
            isBlocked: data.is_blocked,
            detectedPatterns: data.detected_patterns.map(pattern => ({
                patternType: pattern.pattern_type,
                confidence: pattern.confidence,
                description: pattern.description
            })),
            recommendedActions: data.recommended_actions.map(action => ({
                action: action.action,
                priority: action.priority,
                description: action.description
            })),
            additionalVerificationRequired: data.additional_verification_required,
            verificationMethods: data.verification_methods
        }
    }

    /**
     * 适配食品安全报告数据
     */
    static adaptFoodSafetyReport(data: FoodSafetyReportResponse): {
        id: number
        merchantId: number
        merchantName: string
        reporterId: number
        reporterName: string
        reportType: string
        description: string
        severity: string
        status: 'pending' | 'investigating' | 'resolved' | 'dismissed'
        evidenceFiles: string[]
        location?: string
        incidentTime?: string
        createdAt: string
        updatedAt: string
        investigationNotes?: string
        resolution?: string
        safetyRatingImpact?: number
    } {
        return {
            id: data.id,
            merchantId: data.merchant_id,
            merchantName: data.merchant_name,
            reporterId: data.reporter_id,
            reporterName: data.reporter_name,
            reportType: data.report_type,
            description: data.description,
            severity: data.severity,
            status: data.status,
            evidenceFiles: data.evidence_files,
            location: data.location,
            incidentTime: data.incident_time,
            createdAt: data.created_at,
            updatedAt: data.updated_at,
            investigationNotes: data.investigation_notes,
            resolution: data.resolution,
            safetyRatingImpact: data.safety_rating_impact
        }
    }
}

// ==================== 导出服务实例 ====================

export const trustScoreAnalyticsService = new TrustScoreAnalyticsService()

// ==================== 便捷函数 ====================

/**
 * 获取用户完整信任分报告
 * @param role 用户角色
 * @param userId 用户ID
 */
export async function getUserTrustScoreReport(role: UserRole, userId: number): Promise<{
    profile: TrustScoreProfileResponse
    history: TrustScoreHistoryResponse
    analysis: ReturnType<TrustScoreAnalyticsService['analyzeTrustScoreStatus']>
    riskAssessment: {
        currentRisk: string
        trendAnalysis: string
        recommendations: string[]
    }
}> {
    const [profile, history] = await Promise.all([
        trustScoreSystemService.getTrustScoreProfile(role, userId),
        trustScoreSystemService.getTrustScoreHistory(role, userId, 1, 50)
    ])

    const analysis = trustScoreAnalyticsService.analyzeTrustScoreStatus(profile)

    // 简化的风险评估
    const riskAssessment = {
        currentRisk: analysis.riskLevel,
        trendAnalysis: analyzeTrend(history.history),
        recommendations: analysis.recommendations
    }

    return {
        profile,
        history,
        analysis,
        riskAssessment
    }
}

/**
 * 执行综合风控检查
 * @param userId 用户ID
 * @param role 用户角色
 * @param actionType 操作类型
 * @param contextData 上下文数据
 */
export async function performComprehensiveRiskCheck(
    userId: number,
    role: UserRole,
    actionType: string,
    contextData: Record<string, any>
): Promise<{
    trustScore: TrustScoreProfileResponse
    fraudDetection: FraudDetectionResponse
    riskAnalysis: ReturnType<TrustScoreAnalyticsService['analyzeFraudDetection']>
    finalDecision: {
        allowed: boolean
        reason: string
        requiredActions: string[]
        monitoringLevel: 'normal' | 'enhanced' | 'strict'
    }
}> {
    const [trustScore, fraudDetection] = await Promise.all([
        trustScoreSystemService.getTrustScoreProfile(role, userId),
        fraudDetectionService.detectFraud({
            user_id: userId,
            role,
            action_type: actionType,
            context_data: contextData
        })
    ])

    const riskAnalysis = trustScoreAnalyticsService.analyzeFraudDetection(fraudDetection)

    // 综合决策逻辑
    const finalDecision = makeFinalRiskDecision(trustScore, fraudDetection, riskAnalysis)

    return {
        trustScore,
        fraudDetection,
        riskAnalysis,
        finalDecision
    }
}

/**
 * 分析趋势
 */
function analyzeTrend(history: TrustScoreHistoryResponse['history']): string {
    if (history.length < 2) return '数据不足'

    const recent = history.slice(0, 5)
    const totalChange = recent.reduce((sum, record) => sum + record.change_amount, 0)

    if (totalChange > 10) return '上升趋势'
    if (totalChange < -10) return '下降趋势'
    return '保持稳定'
}

/**
 * 做出最终风险决策
 */
function makeFinalRiskDecision(
    trustScore: TrustScoreProfileResponse,
    fraudDetection: FraudDetectionResponse,
    riskAnalysis: ReturnType<TrustScoreAnalyticsService['analyzeFraudDetection']>
): {
    allowed: boolean
    reason: string
    requiredActions: string[]
    monitoringLevel: 'normal' | 'enhanced' | 'strict'
} {
    // 如果被风控系统阻止，直接拒绝
    if (fraudDetection.is_blocked) {
        return {
            allowed: false,
            reason: '触发风控规则，操作被阻止',
            requiredActions: riskAnalysis.actionPlan.immediate,
            monitoringLevel: 'strict'
        }
    }

    // 如果信任分过低，需要额外验证
    if (trustScore.current_score < 60) {
        return {
            allowed: fraudDetection.additional_verification_required ? false : true,
            reason: '信任分较低，需要额外验证',
            requiredActions: ['完成身份验证', '提升信任分'],
            monitoringLevel: 'enhanced'
        }
    }

    // 如果风险等级较高，增强监控
    if (fraudDetection.risk_level === 'high' || fraudDetection.risk_level === 'critical') {
        return {
            allowed: true,
            reason: '检测到高风险行为，允许操作但加强监控',
            requiredActions: riskAnalysis.actionPlan.shortTerm,
            monitoringLevel: 'enhanced'
        }
    }

    // 正常情况
    return {
        allowed: true,
        reason: '风险评估通过',
        requiredActions: [],
        monitoringLevel: 'normal'
    }
}

/**
 * 格式化信任分等级显示
 * @param level 信任分等级
 */
export function formatTrustLevel(level: TrustLevel): string {
    const levelMap: Record<TrustLevel, string> = {
        excellent: '优秀',
        good: '良好',
        average: '一般',
        poor: '较差',
        critical: '危险'
    }
    return levelMap[level] || level
}

/**
 * 格式化风险状态显示
 * @param status 风险状态
 */
export function formatRiskStatus(status: RiskStatus): string {
    const statusMap: Record<RiskStatus, string> = {
        safe: '安全',
        warning: '警告',
        high_risk: '高风险',
        blocked: '已阻止'
    }
    return statusMap[status] || status
}

/**
 * 格式化食品安全等级显示
 * @param level 食品安全等级
 */
export function formatFoodSafetyLevel(level: FoodSafetyLevel): string {
    const levelMap: Record<FoodSafetyLevel, string> = {
        A: 'A级(优秀)',
        B: 'B级(良好)',
        C: 'C级(一般)',
        D: 'D级(较差)'
    }
    return levelMap[level] || level
}

/**
 * 计算信任分颜色
 * @param score 信任分
 */
export function getTrustScoreColor(score: number): string {
    if (score >= 90) return '#52c41a' // 绿色
    if (score >= 80) return '#1890ff' // 蓝色
    if (score >= 70) return '#faad14' // 橙色
    if (score >= 60) return '#fa8c16' // 深橙色
    return '#f5222d' // 红色
}

/**
 * 验证风控检测请求
 * @param request 检测请求
 */
export function validateFraudDetectionRequest(request: FraudDetectionRequest): { valid: boolean; message?: string } {
    if (!request.user_id || request.user_id <= 0) {
        return { valid: false, message: '用户ID无效' }
    }

    if (!request.role || !['customer', 'merchant', 'rider', 'operator'].includes(request.role)) {
        return { valid: false, message: '用户角色无效' }
    }

    if (!request.action_type || request.action_type.trim() === '') {
        return { valid: false, message: '操作类型不能为空' }
    }

    if (!request.context_data || typeof request.context_data !== 'object') {
        return { valid: false, message: '上下文数据格式错误' }
    }

    return { valid: true }
}

/**
 * 验证食品安全报告请求
 * @param request 报告请求
 */
export function validateFoodSafetyReportRequest(request: FoodSafetyReportRequest): { valid: boolean; message?: string } {
    if (!request.merchant_id || request.merchant_id <= 0) {
        return { valid: false, message: '商户ID无效' }
    }

    if (!request.reporter_id || request.reporter_id <= 0) {
        return { valid: false, message: '举报人ID无效' }
    }

    if (!request.report_type || !['hygiene', 'quality', 'safety', 'other'].includes(request.report_type)) {
        return { valid: false, message: '报告类型无效' }
    }

    if (!request.description || request.description.trim() === '') {
        return { valid: false, message: '描述不能为空' }
    }

    if (!request.severity || !['low', 'medium', 'high', 'critical'].includes(request.severity)) {
        return { valid: false, message: '严重程度无效' }
    }

    return { valid: true }
}