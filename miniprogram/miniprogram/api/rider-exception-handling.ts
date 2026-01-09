/**
 * 骑手异常和申诉处理接口重构 (Task 3.3)
 * 基于swagger.json完全重构，移除所有没有后端支持的旧功能
 * 包含：异常上报、延迟申报、申诉管理、索赔处理
 * 
 * 注意：申诉和索赔的基础接口已在appeals-customer-service.ts中定义
 * 这里主要扩展骑手特有的异常处理功能
 */

import { request } from '../utils/request'
import {
    AppealResponse,
    ClaimResponse,
    CreateAppealRequest,
    AppealStatus,
    ClaimStatus,
    ClaimType,
    appealManagementService,
    claimManagementService
} from './appeals-customer-service'

// ==================== 数据类型定义 ====================

/** 异常类型枚举 */
export type ExceptionType = 'customer_unreachable' | 'merchant_not_ready' | 'weather_issue' | 'road_blocked' | 'other'

/** 异常状态枚举 */
export type ExceptionStatus = 'pending' | 'resolved' | 'dismissed'

/** 延迟状态枚举 */
export type DelayStatus = 'pending' | 'acknowledged'



// ==================== 异常上报相关类型 ====================

/** 异常上报请求 - 基于swagger api.exceptionReportRequest */
export interface ExceptionReportRequest extends Record<string, unknown> {
    exception_type: ExceptionType
    description: string
    evidence_urls?: string[]
}

/** 异常上报响应 - 基于swagger api.exceptionReportResponse */
export interface ExceptionReportResponse {
    id: number
    order_id: number
    exception_type: ExceptionType
    description: string
    evidence_urls: string[]
    status: ExceptionStatus
    reported_at: string
}

// ==================== 延迟申报相关类型 ====================

/** 延迟申报请求 - 基于swagger api.delayReportRequest */
export interface DelayReportRequest extends Record<string, unknown> {
    reason: string
    expected_minutes: number
}

/** 延迟申报响应 - 基于swagger api.delayReportResponse */
export interface DelayReportResponse {
    order_id: number
    reason: string
    expected_minutes: number
    status: DelayStatus
    reported_at: string
}

// ==================== 异常处理服务类 ====================

/**
 * 骑手异常处理服务
 * 提供异常上报、延迟申报等功能
 */
export class RiderExceptionHandlingService {
    /**
     * 上报配送异常
     * @param orderId 订单ID
     * @param exceptionData 异常数据
     */
    async reportException(orderId: number, exceptionData: ExceptionReportRequest): Promise<ExceptionReportResponse> {
        return request({
            url: `/rider/orders/${orderId}/exception`,
            method: 'POST',
            data: exceptionData
        })
    }

    /**
     * 申报配送延迟
     * @param orderId 订单ID
     * @param delayData 延迟数据
     */
    async reportDelay(orderId: number, delayData: DelayReportRequest): Promise<DelayReportResponse> {
        return request({
            url: `/rider/orders/${orderId}/delay`,
            method: 'POST',
            data: delayData
        })
    }

    /**
     * 获取骑手申诉列表
     * @param params 查询参数
     */
    async getRiderAppeals(params: {
        page_id: number
        page_size: number
        status?: AppealStatus
    }): Promise<{
        appeals: AppealResponse[]
        total: number
        page_id: number
        page_size: number
        has_more: boolean
    }> {
        return appealManagementService.getRiderAppeals(params)
    }

    /**
     * 获取骑手申诉详情
     * @param appealId 申诉ID
     */
    async getRiderAppealDetail(appealId: number): Promise<AppealResponse> {
        return appealManagementService.getRiderAppealDetail(appealId)
    }

    /**
     * 创建骑手申诉
     * @param appealData 申诉数据
     */
    async createRiderAppeal(appealData: CreateAppealRequest): Promise<AppealResponse> {
        return appealManagementService.createRiderAppeal(appealData)
    }

    /**
     * 获取骑手索赔列表
     * @param params 查询参数
     */
    async getRiderClaims(params: {
        page_id: number
        page_size: number
        status?: ClaimStatus
        claim_type?: ClaimType
    }): Promise<{
        claims: ClaimResponse[]
        total: number
        page_id: number
        page_size: number
        has_more: boolean
    }> {
        return claimManagementService.getRiderClaims(params)
    }

    /**
     * 获取骑手索赔详情
     * @param claimId 索赔ID
     */
    async getRiderClaimDetail(claimId: number): Promise<ClaimResponse> {
        return claimManagementService.getRiderClaimDetail(claimId)
    }
}

// ==================== 异常处理工具类 ====================

/**
 * 异常处理工具类
 * 提供异常分析、建议等功能
 */
export class ExceptionHandlingUtils {
    /**
     * 获取异常类型的处理建议
     * @param exceptionType 异常类型
     */
    static getExceptionHandlingSuggestion(exceptionType: ExceptionType): {
        title: string
        description: string
        actions: string[]
        preventionTips: string[]
    } {
        const suggestions = {
            customer_unreachable: {
                title: '顾客联系不上',
                description: '无法联系到顾客，可能影响正常配送',
                actions: [
                    '多次拨打顾客电话',
                    '发送短信通知顾客',
                    '在配送地址附近等待5-10分钟',
                    '联系平台客服协助处理'
                ],
                preventionTips: [
                    '配送前提前联系顾客确认',
                    '注意顾客备注的特殊要求',
                    '保持手机畅通便于顾客联系'
                ]
            },
            merchant_not_ready: {
                title: '商户出餐未准备好',
                description: '商户出餐延迟，影响配送时效',
                actions: [
                    '与商户确认出餐时间',
                    '如延迟较长可申报延迟',
                    '及时通知顾客可能的延迟',
                    '合理安排其他订单'
                ],
                preventionTips: [
                    '到店前电话确认出餐情况',
                    '了解商户的出餐习惯',
                    '高峰期预留充足时间'
                ]
            },
            weather_issue: {
                title: '天气原因',
                description: '恶劣天气影响正常配送',
                actions: [
                    '注意行车安全，降低车速',
                    '选择相对安全的配送路线',
                    '及时通知顾客可能延迟',
                    '必要时寻找临时避雨点'
                ],
                preventionTips: [
                    '关注天气预报',
                    '准备雨具和防护用品',
                    '恶劣天气谨慎接单'
                ]
            },
            road_blocked: {
                title: '道路阻塞',
                description: '交通拥堵或道路施工影响配送',
                actions: [
                    '使用导航寻找替代路线',
                    '预估延迟时间并申报',
                    '通知顾客可能的延迟',
                    '合理调整配送顺序'
                ],
                preventionTips: [
                    '熟悉配送区域路况',
                    '关注交通信息',
                    '预留充足的配送时间'
                ]
            },
            other: {
                title: '其他异常',
                description: '其他影响配送的异常情况',
                actions: [
                    '详细描述异常情况',
                    '提供相关证据材料',
                    '联系平台客服协助',
                    '确保顾客和商户知情'
                ],
                preventionTips: [
                    '提高风险意识',
                    '遇到问题及时沟通',
                    '保留相关证据'
                ]
            }
        }

        return suggestions[exceptionType]
    }

    /**
     * 验证异常上报数据
     * @param exceptionData 异常数据
     */
    static validateExceptionReport(exceptionData: ExceptionReportRequest): { valid: boolean; message?: string } {
        if (!exceptionData.exception_type) {
            return { valid: false, message: '请选择异常类型' }
        }

        if (!exceptionData.description || exceptionData.description.trim().length < 5) {
            return { valid: false, message: '异常描述至少需要5个字符' }
        }

        if (exceptionData.description.length > 500) {
            return { valid: false, message: '异常描述不能超过500个字符' }
        }

        if (exceptionData.evidence_urls && exceptionData.evidence_urls.length > 9) {
            return { valid: false, message: '证据图片不能超过9张' }
        }

        return { valid: true }
    }

    /**
     * 验证延迟申报数据
     * @param delayData 延迟数据
     */
    static validateDelayReport(delayData: DelayReportRequest): { valid: boolean; message?: string } {
        if (!delayData.reason || delayData.reason.trim().length < 5) {
            return { valid: false, message: '延迟原因至少需要5个字符' }
        }

        if (delayData.reason.length > 500) {
            return { valid: false, message: '延迟原因不能超过500个字符' }
        }

        if (!delayData.expected_minutes || delayData.expected_minutes < 5 || delayData.expected_minutes > 120) {
            return { valid: false, message: '预计延迟时间应在5-120分钟之间' }
        }

        return { valid: true }
    }

    /**
     * 生成异常处理报告
     * @param exceptions 异常列表
     */
    static generateExceptionReport(exceptions: ExceptionReportResponse[]): {
        totalExceptions: number
        exceptionsByType: Record<ExceptionType, number>
        mostCommonType: ExceptionType | null
        resolutionRate: number
        avgResolutionTime: number
    } {
        const totalExceptions = exceptions.length
        const exceptionsByType: Record<ExceptionType, number> = {
            customer_unreachable: 0,
            merchant_not_ready: 0,
            weather_issue: 0,
            road_blocked: 0,
            other: 0
        }

        exceptions.forEach(exception => {
            exceptionsByType[exception.exception_type]++
        })

        // 找出最常见的异常类型
        const entries = Object.keys(exceptionsByType).map(key => [key, exceptionsByType[key as ExceptionType]] as [string, number])
        const mostCommonType = entries
            .reduce((max: { type: ExceptionType | null, count: number }, [type, count]: [string, number]) =>
                count > max.count ? { type: type as ExceptionType, count } : max,
                { type: null as ExceptionType | null, count: 0 }).type

        // 计算解决率
        const resolvedExceptions = exceptions.filter(e => e.status === 'resolved').length
        const resolutionRate = totalExceptions > 0 ? (resolvedExceptions / totalExceptions) * 100 : 0

        // 计算平均解决时间（这里需要根据实际数据结构调整）
        const avgResolutionTime = 0 // 需要根据实际的解决时间数据计算

        return {
            totalExceptions,
            exceptionsByType,
            mostCommonType,
            resolutionRate,
            avgResolutionTime
        }
    }
}

// ==================== 数据适配器 ====================

/**
 * 骑手异常处理数据适配器
 * 处理前端数据格式与后端API数据格式的转换
 */
export class RiderExceptionHandlingAdapter {
    /**
     * 适配异常上报请求数据
     */
    static adaptExceptionReportRequest(data: {
        exceptionType: ExceptionType
        description: string
        evidenceUrls?: string[]
    }): ExceptionReportRequest {
        return {
            exception_type: data.exceptionType,
            description: data.description,
            evidence_urls: data.evidenceUrls
        }
    }

    /**
     * 适配异常上报响应数据
     */
    static adaptExceptionReportResponse(data: ExceptionReportResponse): {
        id: number
        orderId: number
        exceptionType: ExceptionType
        description: string
        evidenceUrls: string[]
        status: ExceptionStatus
        reportedAt: string
    } {
        return {
            id: data.id,
            orderId: data.order_id,
            exceptionType: data.exception_type,
            description: data.description,
            evidenceUrls: data.evidence_urls,
            status: data.status,
            reportedAt: data.reported_at
        }
    }

    /**
     * 适配延迟申报请求数据
     */
    static adaptDelayReportRequest(data: {
        reason: string
        expectedMinutes: number
    }): DelayReportRequest {
        return {
            reason: data.reason,
            expected_minutes: data.expectedMinutes
        }
    }

    /**
     * 适配延迟申报响应数据
     */
    static adaptDelayReportResponse(data: DelayReportResponse): {
        orderId: number
        reason: string
        expectedMinutes: number
        status: DelayStatus
        reportedAt: string
    } {
        return {
            orderId: data.order_id,
            reason: data.reason,
            expectedMinutes: data.expected_minutes,
            status: data.status,
            reportedAt: data.reported_at
        }
    }
}

// ==================== 导出服务实例 ====================

export const riderExceptionHandlingService = new RiderExceptionHandlingService()

// ==================== 便捷函数 ====================

/**
 * 获取骑手异常处理工作台数据
 */
export async function getRiderExceptionDashboard(): Promise<{
    pendingAppeals: AppealResponse[]
    pendingClaims: ClaimResponse[]
    recentExceptions: any[] // 需要根据实际接口调整
    stats: {
        totalAppeals: number
        totalClaims: number
        totalExceptions: number
        resolutionRate: number
    }
}> {
    const [appealsResult, claimsResult] = await Promise.all([
        riderExceptionHandlingService.getRiderAppeals({ page_id: 1, page_size: 10, status: 'pending' }),
        riderExceptionHandlingService.getRiderClaims({ page_id: 1, page_size: 10, status: 'pending' })
    ])

    // 异常记录需要根据实际接口调整
    const recentExceptions: any[] = []

    return {
        pendingAppeals: appealsResult.appeals,
        pendingClaims: claimsResult.claims,
        recentExceptions,
        stats: {
            totalAppeals: appealsResult.total,
            totalClaims: claimsResult.total,
            totalExceptions: recentExceptions.length,
            resolutionRate: 0 // 需要根据实际数据计算
        }
    }
}

/**
 * 智能异常处理建议
 * @param orderId 订单ID
 * @param currentStatus 当前配送状态
 * @param issueDescription 问题描述
 */
export function getSmartExceptionSuggestion(
    orderId: number,
    currentStatus: string,
    issueDescription: string
): {
    suggestedType: ExceptionType
    suggestedActions: string[]
    urgencyLevel: 'low' | 'medium' | 'high'
    shouldReportDelay: boolean
    estimatedDelayMinutes?: number
} {
    const description = issueDescription.toLowerCase()

    // 基于关键词智能判断异常类型
    let suggestedType: ExceptionType = 'other'
    let urgencyLevel: 'low' | 'medium' | 'high' = 'medium'
    let shouldReportDelay = false
    let estimatedDelayMinutes: number | undefined

    if (description.includes('联系不上') || description.includes('电话') || description.includes('顾客')) {
        suggestedType = 'customer_unreachable'
        urgencyLevel = 'high'
        shouldReportDelay = true
        estimatedDelayMinutes = 15
    } else if (description.includes('商户') || description.includes('出餐') || description.includes('准备')) {
        suggestedType = 'merchant_not_ready'
        urgencyLevel = 'medium'
        shouldReportDelay = true
        estimatedDelayMinutes = 20
    } else if (description.includes('天气') || description.includes('下雨') || description.includes('雪')) {
        suggestedType = 'weather_issue'
        urgencyLevel = 'high'
        shouldReportDelay = true
        estimatedDelayMinutes = 30
    } else if (description.includes('堵车') || description.includes('道路') || description.includes('施工')) {
        suggestedType = 'road_blocked'
        urgencyLevel = 'medium'
        shouldReportDelay = true
        estimatedDelayMinutes = 25
    }

    const suggestion = ExceptionHandlingUtils.getExceptionHandlingSuggestion(suggestedType)

    return {
        suggestedType,
        suggestedActions: suggestion.actions,
        urgencyLevel,
        shouldReportDelay,
        estimatedDelayMinutes
    }
}

/**
 * 批量处理异常上报
 * @param reports 异常上报列表
 */
export async function batchReportExceptions(reports: Array<{
    orderId: number
    exceptionData: ExceptionReportRequest
}>): Promise<{ orderId: number; success: boolean; message: string; reportId?: number }[]> {
    const promises = reports.map(async ({ orderId, exceptionData }) => {
        try {
            const result = await riderExceptionHandlingService.reportException(orderId, exceptionData)
            return {
                orderId,
                success: true,
                message: '上报成功',
                reportId: result.id
            }
        } catch (error: any) {
            return {
                orderId,
                success: false,
                message: error?.message || '上报失败'
            }
        }
    })

    return Promise.all(promises)
}

/**
 * 格式化异常类型显示
 * @param exceptionType 异常类型
 */
export function formatExceptionType(exceptionType: ExceptionType): string {
    const typeMap: Record<ExceptionType, string> = {
        customer_unreachable: '顾客联系不上',
        merchant_not_ready: '商户出餐未准备好',
        weather_issue: '天气原因',
        road_blocked: '道路阻塞',
        other: '其他异常'
    }
    return typeMap[exceptionType] || exceptionType
}

/**
 * 格式化异常状态显示
 * @param status 异常状态
 */
export function formatExceptionStatus(status: ExceptionStatus): string {
    const statusMap: Record<ExceptionStatus, string> = {
        pending: '待处理',
        resolved: '已解决',
        dismissed: '已驳回'
    }
    return statusMap[status] || status
}

/**
 * 格式化延迟状态显示
 * @param status 延迟状态
 */
export function formatDelayStatus(status: DelayStatus): string {
    const statusMap: Record<DelayStatus, string> = {
        pending: '待确认',
        acknowledged: '已确认'
    }
    return statusMap[status] || status
}

/**
 * 计算异常处理时效
 * @param reportedAt 上报时间
 * @param resolvedAt 解决时间
 */
export function calculateExceptionResolutionTime(reportedAt: string, resolvedAt?: string): {
    hours: number
    minutes: number
    isOverdue: boolean
} {
    const reported = new Date(reportedAt)
    const resolved = resolvedAt ? new Date(resolvedAt) : new Date()

    const diffMs = resolved.getTime() - reported.getTime()
    const diffHours = Math.floor(diffMs / (1000 * 60 * 60))
    const diffMinutes = Math.floor((diffMs % (1000 * 60 * 60)) / (1000 * 60))

    // 假设异常处理时效为2小时
    const isOverdue = diffHours >= 2

    return {
        hours: diffHours,
        minutes: diffMinutes,
        isOverdue
    }
}

/**
 * 生成异常处理建议
 * @param exceptionHistory 历史异常记录
 */
export function generateExceptionPreventionSuggestions(exceptionHistory: ExceptionReportResponse[]): string[] {
    const suggestions: string[] = []
    const report = ExceptionHandlingUtils.generateExceptionReport(exceptionHistory)

    if (report.mostCommonType) {
        const typeInfo = ExceptionHandlingUtils.getExceptionHandlingSuggestion(report.mostCommonType)
        suggestions.push(`您最常遇到的是${formatExceptionType(report.mostCommonType)}问题，建议：`)
        suggestions.push(...typeInfo.preventionTips)
    }

    if (report.resolutionRate < 80) {
        suggestions.push('异常解决率较低，建议提供更详细的异常描述和证据材料')
    }

    if (report.totalExceptions > 10) {
        suggestions.push('异常频率较高，建议加强配送前的准备工作和风险预判')
    }

    return suggestions
}