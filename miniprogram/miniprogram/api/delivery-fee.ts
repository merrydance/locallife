/**
 * 配送费管理接口 (Phase 4)
 * 基于 swagger.json 实现
 * 包含：配送费计算、区域配置、峰时配置、商户促销
 */

import { request } from '../utils/request'

// ==================== 数据类型定义 ====================

/** 配送费计算请求 */
export interface CalculateDeliveryFeeRequest {
    merchant_id: number
    user_location: {
        latitude: number
        longitude: number
    }
    items_price: number // 分
}

/** 配送费计算结果 */
export interface DeliveryFeeResult {
    total_fee: number          // 总配送费
    base_fee: number           // 基础费
    distance_fee: number       // 距离费
    peak_fee: number           // 峰时加价
    delivery_promotion: number // 配送优惠抵扣
    final_fee: number          // 最终费用
    distance_meters: number    // 配送距离(米)
    region_id: number          // 所属区域ID
}

/** 配送费配置响应 */
export interface DeliveryFeeConfigResponse {
    id: number
    region_id: number
    base_fee: number             // 基础配送费(分)
    base_distance: number        // 基础配送距离(米)
    extra_distance_fee: number   // 超出距离每公里的费用(分)
    min_order_amount: number     // 起送价(分)
    max_delivery_distance: number// 最大配送范围(米)
    is_active: boolean
    created_at: string
}

/** 创建/更新配送费配置请求 */
export interface CreateDeliveryFeeConfigRequest {
    base_fee: number
    base_distance: number
    extra_distance_fee: number
    min_order_amount: number
    max_delivery_distance: number
    is_active?: boolean
}

/** 峰时配置响应 */
export interface PeakHourConfigResponse {
    id: number
    region_id: number
    start_time: string // HH:MM
    end_time: string   // HH:MM
    multiplier: number // 倍率 (e.g. 1.5)
    extra_fee: number  // 额外加价(分)
    days_of_week: number[] // [1,2,3,4,5,6,7]
    is_active: boolean
    name?: string
}

/** 创建峰时配置请求 */
export interface CreatePeakHourConfigRequest {
    start_time: string
    end_time: string
    multiplier: number
    extra_fee: number
    days_of_week: number[]
    name?: string
    is_active?: boolean
}

/** 配送优惠响应 */
export interface DeliveryPromotionResponse {
    id: number
    merchant_id: number
    name: string
    promotion_type: 'fixed_amount' | 'percentage' | 'free_shipping'
    discount_value: number
    min_order_amount: number
    start_time: string
    end_time: string
    is_active: boolean
}

/** 创建配送优惠请求 */
export interface CreateDeliveryPromotionRequest {
    name: string
    promotion_type: 'fixed_amount' | 'percentage' | 'free_shipping'
    discount_value: number
    min_order_amount: number
    start_time: string
    end_time: string
    is_active?: boolean
}

// ==================== 配送费管理服务类 ====================

export class DeliveryFeeService {
    /**
     * 计算配送费
     */
    async calculateFee(data: CalculateDeliveryFeeRequest): Promise<DeliveryFeeResult> {
        return request({
            url: '/v1/delivery-fee/calculate',
            method: 'POST',
            data
        })
    }

    /**
     * 获取区域配送费配置
     * @param regionId 区域ID
     */
    async getRegionConfig(regionId: number): Promise<DeliveryFeeConfigResponse> {
        return request({
            url: `/v1/delivery-fee/config/${regionId}`,
            method: 'GET'
        })
    }

    /**
     * 创建/更新区域配送费配置 (Operator)
     * @param regionId 区域ID
     * @param data 配置数据
     */
    async updateRegionConfig(regionId: number, data: CreateDeliveryFeeConfigRequest): Promise<DeliveryFeeConfigResponse> {
        // 尝试创建，如果已存在则后端会返回409建议走PATCH，或者前端先查询。
        // 根据Swagger，POST是Create，PATCH是Update。
        // 这里合并逻辑：通常业务上会先Get，若无则Post，若有则Patch。
        // 或者是为了简化 UI 调用，我们可以拆分。
        
        // 此处严格遵循 Swagger: POST /delivery-fee/regions/{id}/config
        return request({
            url: `/v1/delivery-fee/regions/${regionId}/config`,
            method: 'POST',
            data
        })
    }
    
    async patchRegionConfig(regionId: number, data: Partial<CreateDeliveryFeeConfigRequest>): Promise<DeliveryFeeConfigResponse> {
        return request({
            url: `/v1/delivery-fee/regions/${regionId}/config`,
            method: 'PATCH',
            data
        })
    }

    /**
     * 获取区域峰时配置列表 (Operator)
     */
    async getPeakConfigs(regionId: number): Promise<PeakHourConfigResponse[]> {
        return request({
            url: `/v1/operator/regions/${regionId}/peak-hours`,
            method: 'GET'
        })
    }

    /**
     * 创建峰时配置 (Operator)
     */
    async createPeakConfig(regionId: number, data: CreatePeakHourConfigRequest): Promise<PeakHourConfigResponse> {
        return request({
            url: `/v1/operator/regions/${regionId}/peak-hours`,
            method: 'POST',
            data
        })
    }
    
    /**
     * 删除峰时配置 (Operator)
     */
    async deletePeakConfig(id: number): Promise<void> {
        return request({
            url: `/v1/operator/peak-hours/${id}`,
            method: 'DELETE'
        })
    }

    /**
     * 获取商户配送优惠列表 (Merchant)
     */
    async getMerchantPromotions(merchantId: number): Promise<DeliveryPromotionResponse[]> {
        return request({
            url: `/v1/delivery-fee/merchants/${merchantId}/promotions`,
            method: 'GET'
        })
    }

    /**
     * 创建商户配送优惠 (Merchant)
     */
    async createMerchantPromotion(merchantId: number, data: CreateDeliveryPromotionRequest): Promise<DeliveryPromotionResponse> {
        return request({
            url: `/v1/delivery-fee/merchants/${merchantId}/promotions`,
            method: 'POST',
            data
        })
    }

    /**
     * 删除商户配送优惠 (Merchant)
     */
    async deleteMerchantPromotion(merchantId: number, promoId: number): Promise<void> {
        return request({
            url: `/v1/delivery-fee/merchants/${merchantId}/promotions/${promoId}`,
            method: 'DELETE'
        })
    }
}

// ==================== 数据适配器 ====================

export class DeliveryFeeAdapter {
    static formatFee(fee: number): string {
        return (fee / 100).toFixed(2)
    }

    static formatDistance(meters: number): string {
        if (meters < 1000) return `${meters}m`
        return `${(meters / 1000).toFixed(1)}km`
    }

    static formatPromotionType(type: string): string {
        const map: Record<string, string> = {
            fixed_amount: '立减',
            percentage: '折扣',
            free_shipping: '免运费'
        }
        return map[type] || type
    }
}

export const deliveryFeeService = new DeliveryFeeService()
