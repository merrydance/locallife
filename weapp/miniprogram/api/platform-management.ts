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

// ==================== 运营商审核相关类型 ====================

export interface AdminOperatorApplicationItem {
    id: number
    user_id: number
    region_id: number
    region_name: string
    region_code: string
    name: string
    contact_name: string
    contact_phone: string
    business_license_media_asset_id?: number
    business_license_number: string
    legal_person_name: string
    legal_person_id_number: string
    requested_contract_years: number
    status: string
    submitted_at?: string
    created_at: string
}

export interface AdminOperatorApplicationDetail extends AdminOperatorApplicationItem {
    business_license_asset_id?: number
    id_card_front_asset_id?: number
    id_card_back_asset_id?: number
    reject_reason?: string
    updated_at?: string
    reviewed_at?: string
}

export interface ListAdminOperatorApplicationsResponse {
    applications: AdminOperatorApplicationItem[]
    total: number
    page: number
    limit: number
    has_more: boolean
}

export interface ListAdminOperatorApplicationsParams extends Record<string, unknown> {
    page?: number
    limit?: number
}

export interface RejectOperatorApplicationRequest extends Record<string, unknown> {
    reject_reason: string
}

// ==================== 集团入驻审核相关类型 ====================

export interface AdminGroupApplicationItem {
    id: number
    applicant_user_id: number
    group_name: string
    contact_phone: string
    license_number?: string
    license_image_asset_id?: number
    address?: string
    region_id?: number
    status: string
    reject_reason?: string
    reviewed_by?: number
    reviewed_at?: string
    created_at: string
    updated_at: string
}

export interface ListAdminGroupApplicationsResponse {
    applications: AdminGroupApplicationItem[]
    total: number
    page: number
    limit: number
    has_more: boolean
}

export interface ListAdminGroupApplicationsParams extends Record<string, unknown> {
    status?: 'draft' | 'submitted' | 'approved' | 'rejected'
    page?: number
    limit?: number
}

export interface ReviewGroupApplicationRequest extends Record<string, unknown> {
    status: 'approved' | 'rejected'
    reject_reason?: string
}

// ==================== 区域扩展申请（管理后台）====================

export interface AdminRegionExpansionApplicationItem {
    id: number
    operator_id: number
    operator_name: string
    contact_name: string
    contact_phone: string
    region_id: number
    region_name: string
    region_code: string
    status: 'pending' | 'approved' | 'rejected' | string
    reject_reason?: string
    created_at: string
}

export interface AdminRegionExpansionApplicationsResponse {
    applications: AdminRegionExpansionApplicationItem[]
    total: number
    page: number
    limit: number
}

// ==================== 平台规则与运营配置相关类型 ====================

export interface PlatformRuleItem {
    id: number
    name: string
    category: string
    status: string
    current_version_id?: number
    created_at?: string
    updated_at?: string
}

export interface ListPlatformRulesResponse {
    rules: PlatformRuleItem[]
    count: number
}

export interface ListPlatformRulesParams extends Record<string, unknown> {
    limit?: number
    offset?: number
}

export interface PlatformOperationalConfigItem {
    id: string
    name: string
    key: string
    value: string
    unit: string
    desc: string
    category: string
    editable: boolean
}

export interface ListPlatformOperationalConfigsResponse {
    rules: PlatformOperationalConfigItem[]
}

export interface UpdatePlatformOperationalConfigRequest extends Record<string, unknown> {
    value: string
}

export interface PlatformProfitSharingConfigItem {
    id: number
    status: string
    order_source: string
    region_id?: number
    merchant_id?: number
    platform_rate: number
    operator_rate: number
    rider_enabled: boolean
    priority: number
    effective_at?: string
    expires_at?: string
    created_by?: number
    created_at: string
    updated_at: string
}

export interface PlatformProfitSharingConfigListResponse {
    items: PlatformProfitSharingConfigItem[]
    page: number
    limit: number
}

export interface ListPlatformProfitSharingConfigsParams extends Record<string, unknown> {
    status?: string
    order_source?: string
    page?: number
    limit?: number
}

export interface CreatePlatformProfitSharingConfigRequest extends Record<string, unknown> {
    status: string
    order_source: string
    platform_rate: number
    operator_rate: number
    rider_enabled?: boolean
    priority?: number
}

export interface UpdatePlatformProfitSharingConfigRequest extends Record<string, unknown> {
    status?: string
    order_source?: string
    platform_rate?: number
    operator_rate?: number
    rider_enabled?: boolean
    priority?: number
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

    private typeReviewEfficiency(efficiency: {
        avgReviewTime: number
        approvalRate: number
        rejectionRate: number
        pendingCount: number
    }): {
        avgReviewTime: number
        approvalRate: number
        rejectionRate: number
        pendingCount: number
    } {
        return efficiency
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
            data: Object.assign({ application_id: applicationId }, reviewData),
            strictEnvelope: true
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
            data: approveData,
            strictEnvelope: true
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
            data: rejectData,
            strictEnvelope: true
        })
    }

    /**
     * 获取待审运营商申请列表
     */
    async getAdminOperatorApplications(params: ListAdminOperatorApplicationsParams): Promise<ListAdminOperatorApplicationsResponse> {
        return request({
            url: '/v1/admin/operators/applications',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取运营商申请详情
     */
    async getAdminOperatorApplicationDetail(applicationID: number): Promise<AdminOperatorApplicationDetail> {
        return request({
            url: `/v1/admin/operators/applications/${applicationID}`,
            method: 'GET'
        })
    }

    /**
     * 通过运营商申请
     */
    async approveOperatorApplication(applicationID: number): Promise<AdminOperatorApplicationItem> {
        return request({
            url: `/v1/admin/operators/applications/${applicationID}/approve`,
            method: 'POST',
            strictEnvelope: true
        })
    }

    /**
     * 驳回运营商申请
     */
    async rejectOperatorApplication(applicationID: number, data: RejectOperatorApplicationRequest): Promise<AdminOperatorApplicationItem> {
        return request({
            url: `/v1/admin/operators/applications/${applicationID}/reject`,
            method: 'POST',
            data,
            strictEnvelope: true
        })
    }

    /**
     * 获取集团入驻申请列表（平台）
     */
    async getAdminGroupApplications(params: ListAdminGroupApplicationsParams): Promise<ListAdminGroupApplicationsResponse> {
        return request({
            url: '/v1/admin/groups/applications',
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取集团入驻申请详情（平台）
     */
    async getAdminGroupApplicationDetail(applicationID: number): Promise<AdminGroupApplicationItem> {
        return request({
            url: `/v1/admin/groups/applications/${applicationID}`,
            method: 'GET'
        })
    }

    /**
     * 审核集团入驻申请（平台）
     */
    async reviewAdminGroupApplication(applicationID: number, data: ReviewGroupApplicationRequest): Promise<AdminGroupApplicationItem> {
        return request({
            url: `/v1/admin/groups/applications/${applicationID}/review`,
            method: 'POST',
            data,
            strictEnvelope: true
        })
    }

    // ── 区域扩展申请（管理后台）──────────────────────────────────

    /**
     * 分页获取区域扩展申请列表（仅限 pending）
     */
    async getAdminRegionExpansionApplications(params: { page?: number, limit?: number }): Promise<AdminRegionExpansionApplicationsResponse> {
        return request({
            url: '/v1/admin/operators/region-applications',
            method: 'GET',
            data: params
        })
    }

    /**
     * 审批通过区域扩展申请
     */
    async approveRegionExpansionApplication(id: number): Promise<AdminRegionExpansionApplicationItem> {
        return request({
            url: `/v1/admin/operators/region-applications/${id}/approve`,
            method: 'POST',
            strictEnvelope: true
        })
    }

    /**
     * 驳回区域扩展申请
     */
    async rejectRegionExpansionApplication(id: number, data: { reject_reason: string }): Promise<AdminRegionExpansionApplicationItem> {
        return request({
            url: `/v1/admin/operators/region-applications/${id}/reject`,
            method: 'POST',
            data,
            strictEnvelope: true
        })
    }

    /**
     * 获取平台规则列表
     */
    async getPlatformRules(params: ListPlatformRulesParams): Promise<ListPlatformRulesResponse> {
        return request({
            url: '/v1/platform/rules',
            method: 'GET',
            data: params
        })
    }

    /**
     * 停用平台规则
     */
    async disablePlatformRule(ruleID: number, reason?: string): Promise<PlatformRuleItem> {
        return request({
            url: `/v1/platform/rules/${ruleID}/disable`,
            method: 'POST',
            data: { reason: reason || '' }
        })
    }

    /**
     * 获取平台维护的运营配置
     */
    async getPlatformOperationalConfigs(): Promise<ListPlatformOperationalConfigsResponse> {
        return request({
            url: '/v1/platform/operational-configs',
            method: 'GET'
        })
    }

    /**
     * 更新平台维护的运营配置项
     */
    async updatePlatformOperationalConfig(key: string, data: UpdatePlatformOperationalConfigRequest): Promise<{ message: string }> {
        return request({
            url: `/v1/platform/operational-configs/${key}`,
            method: 'PATCH',
            data,
            strictEnvelope: true
        })
    }

    /**
     * 获取平台分账配置列表
     */
    async listPlatformProfitSharingConfigs(
        params: ListPlatformProfitSharingConfigsParams
    ): Promise<PlatformProfitSharingConfigListResponse> {
        return request({
            url: '/v1/platform/profit-sharing/configs',
            method: 'GET',
            data: params
        })
    }

    /**
     * 创建平台分账配置
     */
    async createPlatformProfitSharingConfig(
        data: CreatePlatformProfitSharingConfigRequest
    ): Promise<PlatformProfitSharingConfigItem> {
        return request({
            url: '/v1/platform/profit-sharing/configs',
            method: 'POST',
            data,
            strictEnvelope: true
        })
    }

    /**
     * 更新平台分账配置
     */
    async updatePlatformProfitSharingConfig(
        configId: number,
        data: UpdatePlatformProfitSharingConfigRequest
    ): Promise<PlatformProfitSharingConfigItem> {
        return request({
            url: `/v1/platform/profit-sharing/configs/${configId}`,
            method: 'PATCH',
            data,
            strictEnvelope: true
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
    failed: Array<{ id: number, error: string }>
}> {
    const success: number[] = []
    const failed: Array<{ id: number, error: string }> = []

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
    failed: Array<{ id: number, error: string }>
}> {
    const success: number[] = []
    const failed: Array<{ id: number, error: string }> = []

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