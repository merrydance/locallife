/**
 * 平台管理接口重构 (Task 5.2)
 * 基于swagger.json完全重构，移除所有没有后端支持的旧功能
 * 包含：商户审核、骑手审核、配送费配置、高峰时段管理
 */

import { request } from '../utils/request'

// ==================== 数据类型定义 ====================

/** 审核状态枚举 */
export type ReviewStatus = 'pending' | 'approved' | 'rejected'

/** 申请状态枚举 */
export type ApplicationStatus = 'draft' | 'submitted' | 'reviewing' | 'approved' | 'rejected' | 'cancelled' | 'pending'

// ==================== 商户审核相关类型 ====================

/** 商户申请列表响应 - 基于swagger api.listMerchantApplicationsResponse */
export interface ListMerchantApplicationsResponse {
    applications: MerchantApplicationItem[]
    total: number
    page: number
    limit: number
    has_more: boolean
    stats: {
        pending_count: number
        approved_count: number
        rejected_count: number
        avg_review_time: number
    }
}

/** 商户申请项 - 基于swagger api.merchantApplicationItem */
export interface MerchantApplicationItem {
    id: number
    user_id: number
    business_name: string
    contact_person: string
    contact_phone: string
    business_address: string
    business_category: string
    license_number: string
    status: ApplicationStatus
    submitted_at: string
    reviewed_at?: string
    reviewer_id?: number
    reviewer_name?: string
    review_notes?: string
    region_id: number
    region_name: string
}

/** 商户申请审核请求 - 对齐 api.reviewMerchantApplicationRequest */
export interface ReviewMerchantApplicationRequest extends Record<string, unknown> {
    application_id: number
    approve: boolean
    reject_reason?: string
}

// ==================== 骑手审核相关类型 ====================

/** 骑手列表响应 - 基于swagger api.listAdminRidersResponse */
export interface ListAdminRidersResponse {
    riders: AdminRiderItem[]
    total: number
    page: number
    limit: number
    has_more: boolean
    stats: {
        pending_count: number
        approved_count: number
        rejected_count: number
        active_count: number
    }
}

/** 管理员骑手项 - 基于swagger api.adminRiderItem */
export interface AdminRiderItem {
    id: number
    user_id: number
    name: string
    phone: string
    id_card: string
    region_id: number
    region_name: string
    vehicle_type: 'bicycle' | 'electric' | 'motorcycle'
    status: ApplicationStatus
    applied_at: string
    approved_at?: string
    reviewer_id?: number
    reviewer_name?: string
    review_notes?: string
    documents: {
        id_card_front?: string
        id_card_back?: string
        health_certificate?: string
        vehicle_license?: string
    }
}

/** 骑手审核请求 - 基于swagger api.approveRiderRequest */
export interface ApproveRiderRequest extends Record<string, unknown> {
    review_notes?: string
}

/** 骑手拒绝请求 - 基于swagger api.rejectRiderRequest */
export interface RejectRiderRequest extends Record<string, unknown> {
    rejection_reason: string
    review_notes?: string
}

// ==================== 配送费配置相关类型 ====================

/** 配送费配置响应 - 对齐 api.deliveryFeeConfigResponse */
export interface DeliveryFeeConfigResponse {
    base_distance: number
    base_fee: number
    created_at: string
    distance_fee_per_km: number
    id: number
    is_active: boolean
    max_fee: number
    min_fee: number
    region_id: number
    region_name: string
    updated_at: string
    value_ratio: number
    min_distance: number
    max_distance: number
    free_delivery_threshold: number
    surge_multiplier: number
    created_by: string
}

/** 创建配送费配置请求 - 对齐 api.createDeliveryFeeConfigRequest */
export interface CreateDeliveryFeeConfigRequest extends Record<string, unknown> {
    base_distance: number
    base_fee: number
    distance_fee_per_km: number
    max_fee?: number
    min_fee?: number
    region_id: number
    value_ratio?: number
    min_distance: number
    max_distance: number
    free_delivery_threshold: number
    surge_multiplier: number
}

/** 更新配送费配置请求 - 对齐 api.updateDeliveryFeeConfigRequest */
export interface UpdateDeliveryFeeConfigRequest extends Record<string, unknown> {
    base_distance?: number
    base_fee?: number
    extra_fee_per_km?: number
    is_active?: boolean
    max_fee?: number
    min_fee?: number
    value_ratio?: number
}

// ==================== 高峰时段配置相关类型 ====================

/** 高峰时段配置响应 - 对齐 api.peakHourConfigResponse */
export interface PeakHourConfigResponse {
    coefficient: number
    created_at: string
    days_of_week: number[]
    end_time: string
    id: number
    is_active: boolean
    region_id: number
    region_name: string
    start_time: string
    updated_at: string
    multiplier: number
}

/** 创建高峰时段配置请求 - 对齐 api.createPeakHourConfigRequest */
export interface CreatePeakHourConfigRequest extends Record<string, unknown> {
    coefficient: number
    days_of_week: number[]
    end_time: string
    region_id: number
    start_time: string
    multiplier: number
}

// ==================== 查询参数类型 ====================

/** 商户申请查询参数 */
export interface MerchantApplicationQueryParams extends Record<string, unknown> {
    status?: ApplicationStatus
    region_id?: number
    business_category?: string
    submitted_start?: string
    submitted_end?: string
    keyword?: string
    sort_by?: 'submitted_at' | 'reviewed_at' | 'business_name'
    sort_order?: 'asc' | 'desc'
    page?: number
    limit?: number
}

/** 骑手查询参数 */
export interface AdminRiderQueryParams extends Record<string, unknown> {
    status?: ApplicationStatus
    region_id?: number
    vehicle_type?: 'bicycle' | 'electric' | 'motorcycle'
    applied_start?: string
    applied_end?: string
    keyword?: string
    sort_by?: 'applied_at' | 'approved_at' | 'name'
    sort_order?: 'asc' | 'desc'
    page?: number
    limit?: number
}

// ==================== 平台管理服务类 ====================

/**
 * 平台管理服务
 * 提供商户审核、骑手审核、配送费配置等功能
 */
export class PlatformManagementService {
    /**
     * 获取商户申请列表
     * @param params 查询参数
     */
    async getMerchantApplications(params: MerchantApplicationQueryParams): Promise<ListMerchantApplicationsResponse> {
        return request({
            url: '/v1/admin/merchants/applications',
            method: 'GET',
            data: params
        })
    }

    /**
     * 审核商户申请
     * @param applicationId 申请ID
     * @param reviewData 审核数据
     */
    async reviewMerchantApplication(
        applicationId: number,
        reviewData: ReviewMerchantApplicationRequest
    ): Promise<MerchantApplicationItem> {
        return request({
            url: '/v1/admin/merchants/applications/review',
            method: 'POST',
            data: Object.assign({ application_id: applicationId }, reviewData)
        })
    }

    /**
     * 获取骑手列表
     * @param params 查询参数
     */
    async getAdminRiders(params: AdminRiderQueryParams): Promise<ListAdminRidersResponse> {
        return request({
            url: '/v1/admin/riders',
            method: 'GET',
            data: params
        })
    }

    /**
     * 批准骑手申请
     * @param riderId 骑手ID
     * @param approveData 批准数据
     */
    async approveRider(riderId: number, approveData: ApproveRiderRequest): Promise<AdminRiderItem> {
        return request({
            url: `/v1/admin/riders/${riderId}/approve`,
            method: 'POST',
            data: approveData
        })
    }

    /**
     * 拒绝骑手申请
     * @param riderId 骑手ID
     * @param rejectData 拒绝数据
     */
    async rejectRider(riderId: number, rejectData: RejectRiderRequest): Promise<AdminRiderItem> {
        return request({
            url: `/v1/admin/riders/${riderId}/reject`,
            method: 'POST',
            data: rejectData
        })
    }

    /**
     * 获取配送费配置
     * @param regionId 区域ID
     */
    async getDeliveryFeeConfig(regionId: number): Promise<DeliveryFeeConfigResponse> {
        return request({
            url: `/delivery-fee/config/${regionId}`,
            method: 'GET'
        })
    }

    /**
     * 创建配送费配置
     * @param regionId 区域ID
     * @param configData 配置数据
     */
    async createDeliveryFeeConfig(
        regionId: number,
        configData: CreateDeliveryFeeConfigRequest
    ): Promise<DeliveryFeeConfigResponse> {
        return request({
            url: `/delivery-fee/regions/${regionId}/config`,
            method: 'POST',
            data: configData
        })
    }

    /**
     * 更新配送费配置
     * @param regionId 区域ID
     * @param configData 配置数据
     */
    async updateDeliveryFeeConfig(
        regionId: number,
        configData: UpdateDeliveryFeeConfigRequest
    ): Promise<DeliveryFeeConfigResponse> {
        return request({
            url: `/delivery-fee/regions/${regionId}/config`,
            method: 'PATCH',
            data: configData
        })
    }

    /**
     * 获取高峰时段配置列表
     * @param regionId 区域ID
     */
    async getPeakHourConfigs(regionId: number): Promise<PeakHourConfigResponse[]> {
        return request({
            url: `/operator/regions/${regionId}/peak-hours`,
            method: 'GET'
        })
    }

    /**
     * 创建高峰时段配置
     * @param regionId 区域ID
     * @param configData 配置数据
     */
    async createPeakHourConfig(
        regionId: number,
        configData: CreatePeakHourConfigRequest
    ): Promise<PeakHourConfigResponse> {
        return request({
            url: `/operator/regions/${regionId}/peak-hours`,
            method: 'POST',
            data: configData
        })
    }

    /**
     * 删除高峰时段配置
     * @param configId 配置ID
     */
    async deletePeakHourConfig(configId: number): Promise<void> {
        return request({
            url: `/operator/peak-hours/${configId}`,
            method: 'DELETE'
        })
    }
}

export const platformManagementService = new PlatformManagementService()
// ==================== 审核分析服务类 ====================

/**
 * 审核分析服务
 * 提供审核效率分析、质量评估等功能
 */
export class ReviewAnalyticsService {
    /**
     * 分析商户审核效率
     * @param applications 商户申请列表
     */
    analyzeMerchantReviewEfficiency(applications: MerchantApplicationItem[]): {
        efficiency: {
            avgReviewTime: number
            approvalRate: number
            rejectionRate: number
            pendingCount: number
        }
        distribution: {
            byStatus: Map<ApplicationStatus, number>
            byCategory: Map<string, number>
            byRegion: Map<string, number>
        }
        trends: {
            dailyApplications: Array<{ date: string; count: number }>
            reviewTimeTrend: Array<{ date: string; avgTime: number }>
        }
        insights: string[]
        recommendations: string[]
    } {
        const total = applications.length
        const approved = applications.filter(app => app.status === 'approved')
        const rejected = applications.filter(app => app.status === 'rejected')
        const pending = applications.filter(app => app.status === 'pending' || app.status === 'reviewing')

        // 计算审核时间
        const reviewedApps = applications.filter(app => app.reviewed_at && app.submitted_at)
        const avgReviewTime = reviewedApps.length > 0
            ? reviewedApps.reduce((sum, app) => {
                const submitTime = new Date(app.submitted_at).getTime()
                const reviewTime = new Date(app.reviewed_at!).getTime()
                return sum + (reviewTime - submitTime) / (1000 * 60 * 60) // 转换为小时
            }, 0) / reviewedApps.length
            : 0

        // 计算通过率和拒绝率
        const approvalRate = total > 0 ? (approved.length / total) * 100 : 0
        const rejectionRate = total > 0 ? (rejected.length / total) * 100 : 0

        // 分布统计
        const statusDistribution = new Map<ApplicationStatus, number>()
        const categoryDistribution = new Map<string, number>()
        const regionDistribution = new Map<string, number>()

        applications.forEach(app => {
            statusDistribution.set(app.status, (statusDistribution.get(app.status) || 0) + 1)
            categoryDistribution.set(app.business_category, (categoryDistribution.get(app.business_category) || 0) + 1)
            regionDistribution.set(app.region_name, (regionDistribution.get(app.region_name) || 0) + 1)
        })

        // 趋势分析（简化版）
        const dailyApplicationsMap = new Map<string, number>()
        const reviewTimeMap = new Map<string, { total: number; count: number }>()

        applications.forEach(app => {
            const submitDate = app.submitted_at.split('T')[0]
            dailyApplicationsMap.set(submitDate, (dailyApplicationsMap.get(submitDate) || 0) + 1)

            if (app.reviewed_at) {
                const reviewDate = app.reviewed_at.split('T')[0]
                const submitTime = new Date(app.submitted_at).getTime()
                const reviewTime = new Date(app.reviewed_at).getTime()
                const timeDiff = (reviewTime - submitTime) / (1000 * 60 * 60)

                const existing = reviewTimeMap.get(reviewDate) || { total: 0, count: 0 }
                reviewTimeMap.set(reviewDate, {
                    total: existing.total + timeDiff,
                    count: existing.count + 1
                })
            }
        })

        const dailyApplications = Array.from(dailyApplicationsMap.entries())
            .map(([date, count]) => ({ date, count }))
            .sort((a, b) => a.date.localeCompare(b.date))

        const reviewTimeTrend = Array.from(reviewTimeMap.entries())
            .map(([date, data]) => ({ date, avgTime: data.total / data.count }))
            .sort((a, b) => a.date.localeCompare(b.date))

        // 生成洞察和建议
        const insights = this.generateMerchantReviewInsights({
            avgReviewTime,
            approvalRate,
            rejectionRate,
            pendingCount: pending.length
        })

        const recommendations = this.generateMerchantReviewRecommendations({
            avgReviewTime,
            approvalRate,
            rejectionRate,
            pendingCount: pending.length
        })

        return {
            efficiency: {
                avgReviewTime,
                approvalRate,
                rejectionRate,
                pendingCount: pending.length
            },
            distribution: {
                byStatus: statusDistribution,
                byCategory: categoryDistribution,
                byRegion: regionDistribution
            },
            trends: {
                dailyApplications,
                reviewTimeTrend
            },
            insights,
            recommendations
        }
    }

    /**
     * 分析骑手审核效率
     * @param riders 骑手列表
     */
    analyzeRiderReviewEfficiency(riders: AdminRiderItem[]): {
        efficiency: {
            avgReviewTime: number
            approvalRate: number
            rejectionRate: number
            pendingCount: number
        }
        distribution: {
            byStatus: Map<ApplicationStatus, number>
            byVehicleType: Map<string, number>
            byRegion: Map<string, number>
        }
        documentCompleteness: {
            idCardComplete: number
            healthCertComplete: number
            vehicleLicenseComplete: number
            allComplete: number
        }
        insights: string[]
        recommendations: string[]
    } {
        const total = riders.length
        const approved = riders.filter(rider => rider.status === 'approved')
        const rejected = riders.filter(rider => rider.status === 'rejected')
        const pending = riders.filter(rider => rider.status === 'pending' || rider.status === 'reviewing')

        // 计算审核时间
        const reviewedRiders = riders.filter(rider => rider.approved_at && rider.applied_at)
        const avgReviewTime = reviewedRiders.length > 0
            ? reviewedRiders.reduce((sum, rider) => {
                const applyTime = new Date(rider.applied_at).getTime()
                const approveTime = new Date(rider.approved_at!).getTime()
                return sum + (approveTime - applyTime) / (1000 * 60 * 60) // 转换为小时
            }, 0) / reviewedRiders.length
            : 0

        // 计算通过率和拒绝率
        const approvalRate = total > 0 ? (approved.length / total) * 100 : 0
        const rejectionRate = total > 0 ? (rejected.length / total) * 100 : 0

        // 分布统计
        const statusDistribution = new Map<ApplicationStatus, number>()
        const vehicleTypeDistribution = new Map<string, number>()
        const regionDistribution = new Map<string, number>()

        riders.forEach(rider => {
            statusDistribution.set(rider.status, (statusDistribution.get(rider.status) || 0) + 1)
            vehicleTypeDistribution.set(rider.vehicle_type, (vehicleTypeDistribution.get(rider.vehicle_type) || 0) + 1)
            regionDistribution.set(rider.region_name, (regionDistribution.get(rider.region_name) || 0) + 1)
        })

        // 文档完整性统计
        let idCardComplete = 0
        let healthCertComplete = 0
        let vehicleLicenseComplete = 0
        let allComplete = 0

        riders.forEach(rider => {
            const docs = rider.documents
            const hasIdCard = docs.id_card_front && docs.id_card_back
            const hasHealthCert = docs.health_certificate
            const hasVehicleLicense = docs.vehicle_license

            if (hasIdCard) idCardComplete++
            if (hasHealthCert) healthCertComplete++
            if (hasVehicleLicense) vehicleLicenseComplete++
            if (hasIdCard && hasHealthCert && hasVehicleLicense) allComplete++
        })

        // 生成洞察和建议
        const insights = this.generateRiderReviewInsights({
            avgReviewTime,
            approvalRate,
            rejectionRate,
            pendingCount: pending.length
        })

        const recommendations = this.generateRiderReviewRecommendations({
            avgReviewTime,
            approvalRate,
            rejectionRate,
            pendingCount: pending.length
        })

        return {
            efficiency: {
                avgReviewTime,
                approvalRate,
                rejectionRate,
                pendingCount: pending.length
            },
            distribution: {
                byStatus: statusDistribution,
                byVehicleType: vehicleTypeDistribution,
                byRegion: regionDistribution
            },
            documentCompleteness: {
                idCardComplete,
                healthCertComplete,
                vehicleLicenseComplete,
                allComplete
            },
            insights,
            recommendations
        }
    }

    /**
     * 生成商户审核洞察
     */
    private generateMerchantReviewInsights(efficiency: any): string[] {
        const insights: string[] = []

        if (efficiency.avgReviewTime > 72) { // 超过3天
            insights.push('商户审核时间较长，可能影响商户入驻体验')
        } else if (efficiency.avgReviewTime < 24) { // 少于1天
            insights.push('商户审核效率较高，响应及时')
        }

        if (efficiency.approvalRate > 80) {
            insights.push('商户申请通过率较高，申请质量良好')
        } else if (efficiency.approvalRate < 50) {
            insights.push('商户申请通过率偏低，需要优化申请流程或标准')
        }

        if (efficiency.pendingCount > 50) {
            insights.push('待审核商户申请较多，建议增加审核人员')
        }

        return insights
    }

    /**
     * 生成商户审核建议
     */
    private generateMerchantReviewRecommendations(efficiency: any): string[] {
        const recommendations: string[] = []

        if (efficiency.avgReviewTime > 72) {
            recommendations.push('优化审核流程，缩短审核时间')
            recommendations.push('考虑增加审核人员或实施并行审核')
        }

        if (efficiency.rejectionRate > 30) {
            recommendations.push('完善申请指导文档，提高申请质量')
            recommendations.push('提供申请前的资质预检服务')
        }

        if (efficiency.pendingCount > 50) {
            recommendations.push('建立审核优先级机制')
            recommendations.push('考虑部分审核环节自动化')
        }

        return recommendations
    }

    /**
     * 生成骑手审核洞察
     */
    private generateRiderReviewInsights(efficiency: any): string[] {
        const insights: string[] = []

        if (efficiency.avgReviewTime > 48) { // 超过2天
            insights.push('骑手审核时间较长，可能影响骑手入职体验')
        } else if (efficiency.avgReviewTime < 12) { // 少于12小时
            insights.push('骑手审核效率很高，响应迅速')
        }

        if (efficiency.approvalRate > 85) {
            insights.push('骑手申请通过率较高，申请质量良好')
        } else if (efficiency.approvalRate < 60) {
            insights.push('骑手申请通过率偏低，需要分析拒绝原因')
        }

        if (efficiency.pendingCount > 100) {
            insights.push('待审核骑手申请较多，建议加快审核进度')
        }

        return insights
    }

    /**
     * 生成骑手审核建议
     */
    private generateRiderReviewRecommendations(efficiency: any): string[] {
        const recommendations: string[] = []

        if (efficiency.avgReviewTime > 48) {
            recommendations.push('简化骑手审核流程，提高审核效率')
            recommendations.push('实施文档预审核，减少重复审核')
        }

        if (efficiency.rejectionRate > 25) {
            recommendations.push('优化骑手申请指引，提高申请成功率')
            recommendations.push('提供申请前的资格自检工具')
        }

        if (efficiency.pendingCount > 100) {
            recommendations.push('增加审核人员或延长审核时间')
            recommendations.push('建立快速审核通道')
        }

        return recommendations
    }
}

// ==================== 数据适配器 ====================

/**
 * 平台管理数据适配器
 * 处理前端数据格式与后端API数据格式的转换
 */
export class PlatformManagementAdapter {
    /**
     * 适配商户申请项数据
     */
    static adaptMerchantApplicationItem(data: MerchantApplicationItem): {
        id: number
        userId: number
        businessName: string
        contactPerson: string
        contactPhone: string
        businessAddress: string
        businessCategory: string
        licenseNumber: string
        status: ApplicationStatus
        submittedAt: string
        reviewedAt?: string
        reviewerId?: number
        reviewerName?: string
        reviewNotes?: string
        regionId: number
        regionName: string
    } {
        return {
            id: data.id,
            userId: data.user_id,
            businessName: data.business_name,
            contactPerson: data.contact_person,
            contactPhone: data.contact_phone,
            businessAddress: data.business_address,
            businessCategory: data.business_category,
            licenseNumber: data.license_number,
            status: data.status,
            submittedAt: data.submitted_at,
            reviewedAt: data.reviewed_at,
            reviewerId: data.reviewer_id,
            reviewerName: data.reviewer_name,
            reviewNotes: data.review_notes,
            regionId: data.region_id,
            regionName: data.region_name
        }
    }

    /**
     * 适配管理员骑手项数据
     */
    static adaptAdminRiderItem(data: AdminRiderItem): {
        id: number
        userId: number
        name: string
        phone: string
        idCard: string
        regionId: number
        regionName: string
        vehicleType: 'bicycle' | 'electric' | 'motorcycle'
        status: ApplicationStatus
        appliedAt: string
        approvedAt?: string
        reviewerId?: number
        reviewerName?: string
        reviewNotes?: string
        documents: {
            idCardFront?: string
            idCardBack?: string
            healthCertificate?: string
            vehicleLicense?: string
        }
    } {
        return {
            id: data.id,
            userId: data.user_id,
            name: data.name,
            phone: data.phone,
            idCard: data.id_card,
            regionId: data.region_id,
            regionName: data.region_name,
            vehicleType: data.vehicle_type,
            status: data.status,
            appliedAt: data.applied_at,
            approvedAt: data.approved_at,
            reviewerId: data.reviewer_id,
            reviewerName: data.reviewer_name,
            reviewNotes: data.review_notes,
            documents: {
                idCardFront: data.documents.id_card_front,
                idCardBack: data.documents.id_card_back,
                healthCertificate: data.documents.health_certificate,
                vehicleLicense: data.documents.vehicle_license
            }
        }
    }

    /**
     * 适配配送费配置数据
     */
    static adaptDeliveryFeeConfig(data: DeliveryFeeConfigResponse): {
        id: number
        regionId: number
        regionName: string
        baseFee: number
        distanceFeePerKm: number
        minDistance: number
        maxDistance: number
        freeDeliveryThreshold: number
        surgeMultiplier: number
        isActive: boolean
        createdAt: string
        updatedAt: string
        createdBy: string
    } {
        return {
            id: data.id,
            regionId: data.region_id,
            regionName: data.region_name,
            baseFee: data.base_fee,
            distanceFeePerKm: data.distance_fee_per_km,
            minDistance: data.min_distance,
            maxDistance: data.max_distance,
            freeDeliveryThreshold: data.free_delivery_threshold,
            surgeMultiplier: data.surge_multiplier,
            isActive: data.is_active,
            createdAt: data.created_at,
            updatedAt: data.updated_at,
            createdBy: data.created_by
        }
    }

    /**
     * 适配高峰时段配置数据
     */
    static adaptPeakHourConfig(data: PeakHourConfigResponse): {
        id: number
        regionId: number
        regionName: string
        startTime: string
        endTime: string
        multiplier: number
        daysOfWeek: number[]
        isActive: boolean
        createdAt: string
        updatedAt: string
    } {
        return {
            id: data.id,
            regionId: data.region_id,
            regionName: data.region_name,
            startTime: data.start_time,
            endTime: data.end_time,
            multiplier: data.multiplier,
            daysOfWeek: data.days_of_week,
            isActive: data.is_active,
            createdAt: data.created_at,
            updatedAt: data.updated_at
        }
    }
}

// ==================== 导出服务实例 ====================

export const reviewAnalyticsService = new ReviewAnalyticsService()

// ==================== 便捷函数 ====================

/**
 * 获取平台管理工作台数据
 */
export async function getPlatformManagementDashboard(): Promise<{
    merchantApplications: {
        pending: MerchantApplicationItem[]
        stats: ListMerchantApplicationsResponse['stats']
        efficiency: ReturnType<ReviewAnalyticsService['analyzeMerchantReviewEfficiency']>
    }
    riderApplications: {
        pending: AdminRiderItem[]
        stats: ListAdminRidersResponse['stats']
        efficiency: ReturnType<ReviewAnalyticsService['analyzeRiderReviewEfficiency']>
    }
    systemConfig: {
        deliveryFeeConfigs: DeliveryFeeConfigResponse[]
        peakHourConfigs: PeakHourConfigResponse[]
    }
}> {
    const [merchantApps, riders] = await Promise.all([
        platformManagementService.getMerchantApplications({
            status: 'pending',
            limit: 50
        }),
        platformManagementService.getAdminRiders({
            status: 'pending',
            limit: 50
        })
    ])

    // 获取所有申请进行效率分析
    const [allMerchantApps, allRiders] = await Promise.all([
        platformManagementService.getMerchantApplications({ limit: 1000 }),
        platformManagementService.getAdminRiders({ limit: 1000 })
    ])

    const merchantEfficiency = reviewAnalyticsService.analyzeMerchantReviewEfficiency(allMerchantApps.applications)
    const riderEfficiency = reviewAnalyticsService.analyzeRiderReviewEfficiency(allRiders.riders)

    // 获取系统配置（示例区域ID为1）
    const [deliveryFeeConfigs, peakHourConfigs] = await Promise.all([
        platformManagementService.getDeliveryFeeConfig(1).catch(() => null),
        platformManagementService.getPeakHourConfigs(1).catch(() => [])
    ])

    return {
        merchantApplications: {
            pending: merchantApps.applications,
            stats: merchantApps.stats,
            efficiency: merchantEfficiency
        },
        riderApplications: {
            pending: riders.riders,
            stats: riders.stats,
            efficiency: riderEfficiency
        },
        systemConfig: {
            deliveryFeeConfigs: deliveryFeeConfigs ? [deliveryFeeConfigs] : [],
            peakHourConfigs: peakHourConfigs
        }
    }
}

/**
 * 批量审核商户申请
 * @param applicationIds 申请ID列表
 * @param reviewData 审核数据
 */
export async function batchReviewMerchantApplications(
    applicationIds: number[],
    reviewData: ReviewMerchantApplicationRequest
): Promise<{
    success: number[]
    failed: Array<{ id: number; error: string }>
}> {
    const success: number[] = []
    const failed: Array<{ id: number; error: string }> = []

    for (const applicationId of applicationIds) {
        try {
            await platformManagementService.reviewMerchantApplication(applicationId, reviewData)
            success.push(applicationId)
        } catch (error) {
            failed.push({
                id: applicationId,
                error: error instanceof Error ? error.message : '审核失败'
            })
        }
    }

    return { success, failed }
}

/**
 * 批量审核骑手申请
 * @param riderIds 骑手ID列表
 * @param action 操作类型
 * @param actionData 操作数据
 */
export async function batchReviewRiders(
    riderIds: number[],
    action: 'approve' | 'reject',
    actionData: ApproveRiderRequest | RejectRiderRequest
): Promise<{
    success: number[]
    failed: Array<{ id: number; error: string }>
}> {
    const success: number[] = []
    const failed: Array<{ id: number; error: string }> = []

    for (const riderId of riderIds) {
        try {
            if (action === 'approve') {
                await platformManagementService.approveRider(riderId, actionData as ApproveRiderRequest)
            } else {
                await platformManagementService.rejectRider(riderId, actionData as RejectRiderRequest)
            }
            success.push(riderId)
        } catch (error) {
            failed.push({
                id: riderId,
                error: error instanceof Error ? error.message : '审核失败'
            })
        }
    }

    return { success, failed }
}

/**
 * 格式化申请状态显示
 * @param status 申请状态
 */
export function formatApplicationStatus(status: ApplicationStatus): string {
    const statusMap: Record<ApplicationStatus, string> = {
        draft: '草稿',
        submitted: '已提交',
        reviewing: '审核中',
        approved: '已通过',
        rejected: '已拒绝',
        cancelled: '已取消',
        pending: '待审核'
    }
    return statusMap[status] || status
}

/**
 * 格式化审核状态显示
 * @param status 审核状态
 */
export function formatReviewStatus(status: ReviewStatus): string {
    const statusMap: Record<ReviewStatus, string> = {
        pending: '待审核',
        approved: '已通过',
        rejected: '已拒绝'
    }
    return statusMap[status] || status
}

/**
 * 格式化车辆类型显示
 * @param type 车辆类型
 */
export function formatVehicleType(type: 'bicycle' | 'electric' | 'motorcycle'): string {
    const typeMap = {
        bicycle: '自行车',
        electric: '电动车',
        motorcycle: '摩托车'
    }
    return typeMap[type] || type
}

/**
 * 格式化星期显示
 * @param dayOfWeek 星期数字（0-6）
 */
export function formatDayOfWeek(dayOfWeek: number): string {
    const dayMap = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']
    return dayMap[dayOfWeek] || `第${dayOfWeek}天`
}

/**
 * 格式化时间显示
 * @param time 时间字符串（HH:MM）
 */
export function formatTime(time: string): string {
    return time
}

/**
 * 验证配送费配置
 * @param config 配送费配置
 */
export function validateDeliveryFeeConfig(config: CreateDeliveryFeeConfigRequest): { valid: boolean; message?: string } {
    if (config.base_fee < 0) {
        return { valid: false, message: '基础配送费不能为负数' }
    }

    if (config.distance_fee_per_km < 0) {
        return { valid: false, message: '距离费用不能为负数' }
    }

    if (config.min_distance < 0) {
        return { valid: false, message: '最小配送距离不能为负数' }
    }

    if (config.max_distance <= config.min_distance) {
        return { valid: false, message: '最大配送距离必须大于最小配送距离' }
    }

    if (config.free_delivery_threshold < 0) {
        return { valid: false, message: '免配送费门槛不能为负数' }
    }

    if (config.surge_multiplier < 1) {
        return { valid: false, message: '高峰倍数不能小于1' }
    }

    return { valid: true }
}

/**
 * 验证高峰时段配置
 * @param config 高峰时段配置
 */
export function validatePeakHourConfig(config: CreatePeakHourConfigRequest): { valid: boolean; message?: string } {
    // 验证时间格式
    const timeRegex = /^([01]?[0-9]|2[0-3]):[0-5][0-9]$/
    if (!timeRegex.test(config.start_time)) {
        return { valid: false, message: '开始时间格式不正确，应为HH:MM' }
    }

    if (!timeRegex.test(config.end_time)) {
        return { valid: false, message: '结束时间格式不正确，应为HH:MM' }
    }

    // 验证时间逻辑
    const startTime = config.start_time.split(':').map(Number)
    const endTime = config.end_time.split(':').map(Number)
    const startMinutes = startTime[0] * 60 + startTime[1]
    const endMinutes = endTime[0] * 60 + endTime[1]

    if (startMinutes >= endMinutes) {
        return { valid: false, message: '结束时间必须晚于开始时间' }
    }

    // 验证倍数
    if (config.multiplier < 1) {
        return { valid: false, message: '高峰倍数不能小于1' }
    }

    if (config.multiplier > 5) {
        return { valid: false, message: '高峰倍数不能超过5' }
    }

    // 验证星期
    if (!Array.isArray(config.days_of_week) || config.days_of_week.length === 0) {
        return { valid: false, message: '必须选择至少一天' }
    }

    const validDays = config.days_of_week.every(day => day >= 0 && day <= 6)
    if (!validDays) {
        return { valid: false, message: '星期数字必须在0-6之间' }
    }

    return { valid: true }
}