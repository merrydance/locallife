"use strict";
/**
 * 信任分和风控接口重构 (Task 5.3)
 * 基于swagger.json完全重构，移除所有没有后端支持的旧功能
 * 包含：信任分系统、风控检测、申诉审核、食品安全
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
exports.trustScoreAnalyticsService = exports.TrustScoreSystemAdapter = exports.TrustScoreAnalyticsService = exports.foodSafetyService = exports.fraudDetectionService = exports.trustScoreSystemService = exports.FoodSafetyService = exports.FraudDetectionService = exports.TrustScoreSystemService = void 0;
exports.getUserTrustScoreReport = getUserTrustScoreReport;
exports.performComprehensiveRiskCheck = performComprehensiveRiskCheck;
exports.formatTrustLevel = formatTrustLevel;
exports.formatRiskStatus = formatRiskStatus;
exports.formatFoodSafetyLevel = formatFoodSafetyLevel;
exports.getTrustScoreColor = getTrustScoreColor;
exports.validateFraudDetectionRequest = validateFraudDetectionRequest;
exports.validateFoodSafetyReportRequest = validateFoodSafetyReportRequest;
const request_1 = require("../utils/request");
// ==================== 信任分系统服务类 ====================
/**
 * 信任分系统服务
 * 提供信任分查询、历史记录等功能
 */
class TrustScoreSystemService {
    /**
     * 获取用户信任分档案
     * @param role 用户角色
     * @param userId 用户ID
     */
    getTrustScoreProfile(role, userId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/trust-score/profiles/${role}/${userId}`,
                method: 'GET'
            });
        });
    }
    /**
     * 获取信任分历史记录
     * @param role 用户角色
     * @param userId 用户ID
     * @param page 页码
     * @param limit 每页数量
     */
    getTrustScoreHistory(role_1, userId_1) {
        return __awaiter(this, arguments, void 0, function* (role, userId, page = 1, limit = 20) {
            return (0, request_1.request)({
                url: `/v1/trust-score/history/${role}/${userId}`,
                method: 'GET',
                data: { page, limit }
            });
        });
    }
    /**
     * 提交信任分申诉
     * @param appealData 申诉数据
     */
    submitTrustScoreAppeal(appealData) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/trust-score/appeals',
                method: 'POST',
                data: appealData
            });
        });
    }
    /**
     * 提交索赔申请
     * @param claimData 索赔数据
     */
    submitClaim(claimData) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/trust-score/claims',
                method: 'POST',
                data: claimData
            });
        });
    }
    /**
     * 审核索赔申请
     * @param claimId 索赔ID
     * @param reviewData 审核数据
     */
    reviewClaim(claimId, reviewData) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/trust-score/claims/${claimId}/review`,
                method: 'PATCH',
                data: reviewData
            });
        });
    }
    /**
     * 申请信任分恢复
     * @param recoveryData 恢复数据
     */
    requestTrustScoreRecovery(recoveryData) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/trust-score/recovery',
                method: 'POST',
                data: recoveryData
            });
        });
    }
}
exports.TrustScoreSystemService = TrustScoreSystemService;
// ==================== 风控检测服务类 ====================
/**
 * 风控检测服务
 * 提供风险检测、商户暂停等功能
 */
class FraudDetectionService {
    /**
     * 执行风控检测
     * @param detectionData 检测数据
     */
    detectFraud(detectionData) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/trust-score/fraud/detect',
                method: 'POST',
                data: detectionData
            });
        });
    }
    /**
     * 暂停商户
     * @param merchantId 商户ID
     * @param suspendData 暂停数据
     */
    suspendMerchant(merchantId, suspendData) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/trust-score/merchants/${merchantId}/suspend`,
                method: 'PATCH',
                data: suspendData
            });
        });
    }
}
exports.FraudDetectionService = FraudDetectionService;
// ==================== 食品安全服务类 ====================
/**
 * 食品安全服务
 * 提供食品安全报告等功能
 */
class FoodSafetyService {
    /**
     * 提交食品安全报告
     * @param reportData 报告数据
     */
    submitFoodSafetyReport(reportData) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/trust-score/food-safety/report',
                method: 'POST',
                data: reportData
            });
        });
    }
}
exports.FoodSafetyService = FoodSafetyService;
exports.trustScoreSystemService = new TrustScoreSystemService();
exports.fraudDetectionService = new FraudDetectionService();
exports.foodSafetyService = new FoodSafetyService();
// ==================== 信任分分析服务类 ====================
/**
 * 信任分分析服务
 * 提供信任分分析、风险评估等功能
 */
class TrustScoreAnalyticsService {
    /**
     * 分析用户信任分状况
     * @param profile 信任分档案
     */
    analyzeTrustScoreStatus(profile) {
        const currentScore = profile.current_score;
        const riskFactors = profile.risk_factors;
        const restrictions = profile.restrictions;
        // 评估健康状况
        let healthStatus = 'healthy';
        if (currentScore < 60)
            healthStatus = 'critical';
        else if (currentScore < 80)
            healthStatus = 'warning';
        // 评估风险等级
        let riskLevel = 'low';
        const highRiskFactors = riskFactors.filter(factor => Math.abs(factor.impact) > 10);
        if (highRiskFactors.length > 2)
            riskLevel = 'high';
        else if (highRiskFactors.length > 0)
            riskLevel = 'medium';
        // 生成建议
        const recommendations = this.generateTrustScoreRecommendations(profile);
        // 计算下次审核日期
        const nextReviewDate = this.calculateNextReviewDate(profile);
        // 生成改善计划
        const improvementPlan = this.generateImprovementPlan(profile);
        // 统计限制情况
        const activeRestrictions = restrictions.filter(r => r.is_active);
        const now = new Date();
        const expiringSoon = activeRestrictions.filter(r => {
            if (!r.end_date)
                return false;
            const endDate = new Date(r.end_date);
            const daysUntilExpiry = (endDate.getTime() - now.getTime()) / (1000 * 60 * 60 * 24);
            return daysUntilExpiry <= 7 && daysUntilExpiry > 0;
        });
        const permanent = activeRestrictions.filter(r => !r.end_date);
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
        };
    }
    /**
     * 分析风控检测结果
     * @param detection 风控检测结果
     */
    analyzeFraudDetection(detection) {
        const riskScore = detection.risk_score;
        const patterns = detection.detected_patterns;
        const actions = detection.recommended_actions;
        // 风险评估
        const riskAssessment = {
            level: detection.risk_level,
            score: riskScore,
            confidence: patterns.length > 0 ?
                patterns.reduce((sum, p) => sum + p.confidence, 0) / patterns.length : 0
        };
        // 威胁分析
        const primaryThreats = patterns
            .filter(p => p.confidence > 0.7)
            .map(p => p.pattern_type);
        const riskPatterns = patterns.map(p => p.description);
        let urgencyLevel = 'low';
        if (detection.is_blocked)
            urgencyLevel = 'critical';
        else if (riskScore > 80)
            urgencyLevel = 'high';
        else if (riskScore > 60)
            urgencyLevel = 'medium';
        // 行动计划
        const immediateActions = actions
            .filter(a => a.priority === 'high')
            .map(a => a.action);
        const shortTermActions = actions
            .filter(a => a.priority === 'medium')
            .map(a => a.action);
        const monitoringActions = actions
            .filter(a => a.priority === 'low')
            .map(a => a.action);
        // 预估解决时间
        let estimatedResolutionTime = '1-2小时';
        if (urgencyLevel === 'critical')
            estimatedResolutionTime = '立即处理';
        else if (urgencyLevel === 'high')
            estimatedResolutionTime = '30分钟内';
        else if (urgencyLevel === 'medium')
            estimatedResolutionTime = '2-4小时';
        else
            estimatedResolutionTime = '24小时内';
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
        };
    }
    /**
     * 生成信任分建议
     */
    generateTrustScoreRecommendations(profile) {
        const recommendations = [];
        const score = profile.current_score;
        const breakdown = profile.score_breakdown;
        const riskFactors = profile.risk_factors;
        if (score < 60) {
            recommendations.push('信任分过低，建议立即采取改善措施');
        }
        if (breakdown.behavior_score < 70) {
            recommendations.push('行为评分偏低，建议规范操作行为');
        }
        if (breakdown.compliance_score < 80) {
            recommendations.push('合规评分不足，建议加强规则学习');
        }
        if (breakdown.service_score < 75) {
            recommendations.push('服务评分有待提升，建议改善服务质量');
        }
        if (riskFactors.length > 3) {
            recommendations.push('风险因素较多，建议逐项处理');
        }
        if (profile.restrictions.some(r => r.is_active)) {
            recommendations.push('存在活跃限制，建议尽快解除');
        }
        return recommendations;
    }
    /**
     * 计算下次审核日期
     */
    calculateNextReviewDate(profile) {
        const lastUpdated = new Date(profile.last_updated);
        const score = profile.current_score;
        // 根据信任分确定审核周期
        let reviewCycleDays = 30; // 默认30天
        if (score < 60)
            reviewCycleDays = 7; // 低分用户每周审核
        else if (score < 80)
            reviewCycleDays = 14; // 中等分数用户两周审核
        else if (score >= 90)
            reviewCycleDays = 60; // 高分用户两月审核
        const nextReview = new Date(lastUpdated.getTime() + reviewCycleDays * 24 * 60 * 60 * 1000);
        return nextReview.toISOString().split('T')[0];
    }
    /**
     * 生成改善计划
     */
    generateImprovementPlan(profile) {
        const plan = [];
        const breakdown = profile.score_breakdown;
        const riskFactors = profile.risk_factors;
        // 基于评分分解生成改善计划
        if (breakdown.behavior_score < 70) {
            plan.push({
                action: '完成行为规范培训',
                expectedImpact: 10,
                timeframe: '1周',
                priority: 'high'
            });
        }
        if (breakdown.compliance_score < 80) {
            plan.push({
                action: '学习平台规则并通过考试',
                expectedImpact: 8,
                timeframe: '3天',
                priority: 'high'
            });
        }
        if (breakdown.service_score < 75) {
            plan.push({
                action: '提升服务质量，获得更多好评',
                expectedImpact: 12,
                timeframe: '2周',
                priority: 'medium'
            });
        }
        // 基于风险因素生成改善计划
        riskFactors.forEach(factor => {
            if (Math.abs(factor.impact) > 5) {
                plan.push({
                    action: `处理风险因素：${factor.factor}`,
                    expectedImpact: Math.abs(factor.impact),
                    timeframe: '1周',
                    priority: Math.abs(factor.impact) > 10 ? 'high' : 'medium'
                });
            }
        });
        return plan.sort((a, b) => {
            const priorityOrder = { high: 3, medium: 2, low: 1 };
            return priorityOrder[b.priority] - priorityOrder[a.priority];
        });
    }
}
exports.TrustScoreAnalyticsService = TrustScoreAnalyticsService;
// ==================== 数据适配器 ====================
/**
 * 信任分系统数据适配器
 * 处理前端数据格式与后端API数据格式的转换
 */
class TrustScoreSystemAdapter {
    /**
     * 适配信任分档案数据
     */
    static adaptTrustScoreProfile(data) {
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
        };
    }
    /**
     * 适配风控检测响应数据
     */
    static adaptFraudDetectionResponse(data) {
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
        };
    }
    /**
     * 适配食品安全报告数据
     */
    static adaptFoodSafetyReport(data) {
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
        };
    }
}
exports.TrustScoreSystemAdapter = TrustScoreSystemAdapter;
// ==================== 导出服务实例 ====================
exports.trustScoreAnalyticsService = new TrustScoreAnalyticsService();
// ==================== 便捷函数 ====================
/**
 * 获取用户完整信任分报告
 * @param role 用户角色
 * @param userId 用户ID
 */
function getUserTrustScoreReport(role, userId) {
    return __awaiter(this, void 0, void 0, function* () {
        const [profile, history] = yield Promise.all([
            exports.trustScoreSystemService.getTrustScoreProfile(role, userId),
            exports.trustScoreSystemService.getTrustScoreHistory(role, userId, 1, 50)
        ]);
        const analysis = exports.trustScoreAnalyticsService.analyzeTrustScoreStatus(profile);
        // 简化的风险评估
        const riskAssessment = {
            currentRisk: analysis.riskLevel,
            trendAnalysis: analyzeTrend(history.history),
            recommendations: analysis.recommendations
        };
        return {
            profile,
            history,
            analysis,
            riskAssessment
        };
    });
}
/**
 * 执行综合风控检查
 * @param userId 用户ID
 * @param role 用户角色
 * @param actionType 操作类型
 * @param contextData 上下文数据
 */
function performComprehensiveRiskCheck(userId, role, actionType, contextData) {
    return __awaiter(this, void 0, void 0, function* () {
        const [trustScore, fraudDetection] = yield Promise.all([
            exports.trustScoreSystemService.getTrustScoreProfile(role, userId),
            exports.fraudDetectionService.detectFraud({
                user_id: userId,
                role,
                action_type: actionType,
                context_data: contextData
            })
        ]);
        const riskAnalysis = exports.trustScoreAnalyticsService.analyzeFraudDetection(fraudDetection);
        // 综合决策逻辑
        const finalDecision = makeFinalRiskDecision(trustScore, fraudDetection, riskAnalysis);
        return {
            trustScore,
            fraudDetection,
            riskAnalysis,
            finalDecision
        };
    });
}
/**
 * 分析趋势
 */
function analyzeTrend(history) {
    if (history.length < 2)
        return '数据不足';
    const recent = history.slice(0, 5);
    const totalChange = recent.reduce((sum, record) => sum + record.change_amount, 0);
    if (totalChange > 10)
        return '上升趋势';
    if (totalChange < -10)
        return '下降趋势';
    return '保持稳定';
}
/**
 * 做出最终风险决策
 */
function makeFinalRiskDecision(trustScore, fraudDetection, riskAnalysis) {
    // 如果被风控系统阻止，直接拒绝
    if (fraudDetection.is_blocked) {
        return {
            allowed: false,
            reason: '触发风控规则，操作被阻止',
            requiredActions: riskAnalysis.actionPlan.immediate,
            monitoringLevel: 'strict'
        };
    }
    // 如果信任分过低，需要额外验证
    if (trustScore.current_score < 60) {
        return {
            allowed: fraudDetection.additional_verification_required ? false : true,
            reason: '信任分较低，需要额外验证',
            requiredActions: ['完成身份验证', '提升信任分'],
            monitoringLevel: 'enhanced'
        };
    }
    // 如果风险等级较高，增强监控
    if (fraudDetection.risk_level === 'high' || fraudDetection.risk_level === 'critical') {
        return {
            allowed: true,
            reason: '检测到高风险行为，允许操作但加强监控',
            requiredActions: riskAnalysis.actionPlan.shortTerm,
            monitoringLevel: 'enhanced'
        };
    }
    // 正常情况
    return {
        allowed: true,
        reason: '风险评估通过',
        requiredActions: [],
        monitoringLevel: 'normal'
    };
}
/**
 * 格式化信任分等级显示
 * @param level 信任分等级
 */
function formatTrustLevel(level) {
    const levelMap = {
        excellent: '优秀',
        good: '良好',
        average: '一般',
        poor: '较差',
        critical: '危险'
    };
    return levelMap[level] || level;
}
/**
 * 格式化风险状态显示
 * @param status 风险状态
 */
function formatRiskStatus(status) {
    const statusMap = {
        safe: '安全',
        warning: '警告',
        high_risk: '高风险',
        blocked: '已阻止'
    };
    return statusMap[status] || status;
}
/**
 * 格式化食品安全等级显示
 * @param level 食品安全等级
 */
function formatFoodSafetyLevel(level) {
    const levelMap = {
        A: 'A级(优秀)',
        B: 'B级(良好)',
        C: 'C级(一般)',
        D: 'D级(较差)'
    };
    return levelMap[level] || level;
}
/**
 * 计算信任分颜色
 * @param score 信任分
 */
function getTrustScoreColor(score) {
    if (score >= 90)
        return '#52c41a'; // 绿色
    if (score >= 80)
        return '#1890ff'; // 蓝色
    if (score >= 70)
        return '#faad14'; // 橙色
    if (score >= 60)
        return '#fa8c16'; // 深橙色
    return '#f5222d'; // 红色
}
/**
 * 验证风控检测请求
 * @param request 检测请求
 */
function validateFraudDetectionRequest(request) {
    if (!request.user_id || request.user_id <= 0) {
        return { valid: false, message: '用户ID无效' };
    }
    if (!request.role || !['customer', 'merchant', 'rider', 'operator'].includes(request.role)) {
        return { valid: false, message: '用户角色无效' };
    }
    if (!request.action_type || request.action_type.trim() === '') {
        return { valid: false, message: '操作类型不能为空' };
    }
    if (!request.context_data || typeof request.context_data !== 'object') {
        return { valid: false, message: '上下文数据格式错误' };
    }
    return { valid: true };
}
/**
 * 验证食品安全报告请求
 * @param request 报告请求
 */
function validateFoodSafetyReportRequest(request) {
    if (!request.merchant_id || request.merchant_id <= 0) {
        return { valid: false, message: '商户ID无效' };
    }
    if (!request.reporter_id || request.reporter_id <= 0) {
        return { valid: false, message: '举报人ID无效' };
    }
    if (!request.report_type || !['hygiene', 'quality', 'safety', 'other'].includes(request.report_type)) {
        return { valid: false, message: '报告类型无效' };
    }
    if (!request.description || request.description.trim() === '') {
        return { valid: false, message: '描述不能为空' };
    }
    if (!request.severity || !['low', 'medium', 'high', 'critical'].includes(request.severity)) {
        return { valid: false, message: '严重程度无效' };
    }
    return { valid: true };
}
