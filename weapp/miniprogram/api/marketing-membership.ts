/**
 * 营销和会员管理接口重构 (Task 2.5)
 * 基于swagger.json完全重构，移除所有没有后端支持的旧功能
 * 包含：优惠券管理、充值规则管理、促销活动管理、会员设置管理
 */

import { request } from '../utils/request'

// ==================== 数据类型定义 ====================

/** 订单类型枚举 */
export type OrderType = 'takeout' | 'dine_in' | 'takeaway' | 'reservation'

/** 使用场景枚举 */
export type UsableScene = 'takeout' | 'dine_in' | 'reservation' | 'all'

// ==================== 优惠券管理相关类型 ====================

/** 创建优惠券请求 - 基于swagger api.createVoucherRequest */
export interface CreateVoucherRequest extends Record<string, unknown> {
    name: string
    code: string
    amount: number
    total_quantity: number
    valid_from: string
    valid_until: string
    description?: string
    min_order_amount?: number
    allowed_order_types?: OrderType[]
}

/** 更新优惠券请求 - 基于swagger api.updateVoucherRequest */
/** 更新优惠券请求 - 对齐 api.updateVoucherRequest */
export interface UpdateVoucherRequest extends Record<string, unknown> {
    allowed_order_types?: string[]               // 允许的订单类型
    amount?: number                              // 金额（分）
    description?: string                         // 描述
    is_active?: boolean                          // 是否激活
    min_order_amount?: number                    // 最低订单金额
    name?: string                                // 名称
    total_quantity?: number                      // 总数量
    valid_from?: string                          // 有效期开始
    valid_until?: string                         // 有效期结束
}

/** 优惠券响应 - 基于swagger api.voucherResponse */
export interface VoucherResponse {
    id: number
    merchant_id: number
    name: string
    code: string
    description?: string
    amount: number
    min_order_amount?: number
    total_quantity: number
    claimed_quantity: number
    used_quantity: number
    is_active: boolean
    valid_from: string
    valid_until: string
    allowed_order_types?: OrderType[]
    created_at: string
}

/** 优惠券列表查询参数 */
export interface ListVouchersParams extends Record<string, unknown> {
    page_id: number
    page_size: number
}

// ==================== 充值规则管理相关类型 ====================

/** 创建充值规则请求 - 基于swagger api.createRechargeRuleRequest */
export interface CreateRechargeRuleRequest extends Record<string, unknown> {
    recharge_amount: number
    bonus_amount: number
    valid_from: string
    valid_until: string
}

/** 更新充值规则请求 */
export interface UpdateRechargeRuleRequest extends Record<string, unknown> {
    recharge_amount?: number
    bonus_amount?: number
    valid_from?: string
    valid_until?: string
    is_active?: boolean
}

/** 充值规则响应 - 基于swagger api.rechargeRuleResponse */
export interface RechargeRuleResponse {
    id: number
    merchant_id: number
    recharge_amount: number
    bonus_amount: number
    is_active: boolean
    valid_from: string
    valid_until: string
    created_at: string
}

// ==================== 会员设置相关类型 ====================

/** 更新会员设置请求 - 基于swagger api.updateMembershipSettingsRequest */
export interface UpdateMembershipSettingsRequest extends Record<string, unknown> {
    balance_usable_scenes?: UsableScene[]
    bonus_usable_scenes?: UsableScene[]
    max_deduction_percent?: number
    allow_with_discount?: boolean
    allow_with_voucher?: boolean
}

/** 会员设置响应 - 基于swagger api.membershipSettingsResponse */
export interface MembershipSettingsResponse {
    merchant_id: number
    balance_usable_scenes: UsableScene[]
    bonus_usable_scenes: UsableScene[]
    max_deduction_percent: number
    allow_with_discount: boolean
    allow_with_voucher: boolean
}

// ==================== 促销活动相关类型 ====================

/** 促销项目 - 基于swagger api.promotionItem */
export interface PromotionItem {
    type: string
    title: string
    description: string
    min_amount: number
    value: number
    valid_until: string
}

/** 商户促销活动响应 - 基于swagger api.merchantPromotionsResponse */
export interface MerchantPromotionsResponse {
    merchant_id: number
    delivery_fee_rules: PromotionItem[]
    discount_rules: PromotionItem[]
    vouchers: PromotionItem[]
}

// ==================== 优惠券管理服务类 ====================

/**
 * 优惠券管理服务
 * 提供优惠券的CRUD操作、状态管理等功能
 */
export class VoucherManagementService {
    /**
     * 获取商户优惠券列表
     * @param merchantId 商户ID
     * @param params 查询参数
     */
    async listVouchers(merchantId: number, params: ListVouchersParams): Promise<VoucherResponse[]> {
        return request({
            url: `/v1/merchants/${merchantId}/vouchers`,
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取商户活跃优惠券列表
     * @param merchantId 商户ID
     */
    async listActiveVouchers(merchantId: number): Promise<VoucherResponse[]> {
        return request({
            url: `/v1/merchants/${merchantId}/vouchers/active`,
            method: 'GET'
        })
    }

    /**
     * 创建优惠券
     * @param merchantId 商户ID
     * @param voucherData 优惠券数据
     */
    async createVoucher(merchantId: number, voucherData: CreateVoucherRequest): Promise<VoucherResponse> {
        return request({
            url: `/v1/merchants/${merchantId}/vouchers`,
            method: 'POST',
            data: voucherData
        })
    }

    /**
     * 更新优惠券
     * @param merchantId 商户ID
     * @param voucherId 优惠券ID
     * @param voucherData 更新数据
     */
    async updateVoucher(merchantId: number, voucherId: number, voucherData: UpdateVoucherRequest): Promise<VoucherResponse> {
        return request({
            url: `/v1/merchants/${merchantId}/vouchers/${voucherId}`,
            method: 'PUT',
            data: voucherData
        })
    }

    /**
     * 删除优惠券
     * @param merchantId 商户ID
     * @param voucherId 优惠券ID
     */
    async deleteVoucher(merchantId: number, voucherId: number): Promise<{ message: string }> {
        return request({
            url: `/v1/merchants/${merchantId}/vouchers/${voucherId}`,
            method: 'DELETE'
        })
    }
}

// ==================== 充值规则管理服务类 ====================

/**
 * 充值规则管理服务
 * 提供充值规则的CRUD操作、状态管理等功能
 */
export class RechargeRuleManagementService {
    /**
     * 获取商户充值规则列表
     * @param merchantId 商户ID
     */
    async listRechargeRules(merchantId: number): Promise<RechargeRuleResponse[]> {
        return request({
            url: `/v1/merchants/${merchantId}/recharge-rules`,
            method: 'GET'
        })
    }

    /**
     * 获取商户活跃充值规则列表
     * @param merchantId 商户ID
     */
    async listActiveRechargeRules(merchantId: number): Promise<RechargeRuleResponse[]> {
        return request({
            url: `/v1/merchants/${merchantId}/recharge-rules/active`,
            method: 'GET'
        })
    }

    /**
     * 创建充值规则
     * @param merchantId 商户ID
     * @param ruleData 充值规则数据
     */
    async createRechargeRule(merchantId: number, ruleData: CreateRechargeRuleRequest): Promise<RechargeRuleResponse> {
        return request({
            url: `/v1/merchants/${merchantId}/recharge-rules`,
            method: 'POST',
            data: ruleData
        })
    }

    /**
     * 更新充值规则
     * @param merchantId 商户ID
     * @param ruleId 规则ID
     * @param ruleData 更新数据
     */
    async updateRechargeRule(merchantId: number, ruleId: number, ruleData: UpdateRechargeRuleRequest): Promise<RechargeRuleResponse> {
        return request({
            url: `/v1/merchants/${merchantId}/recharge-rules/${ruleId}`,
            method: 'PATCH',
            data: ruleData
        })
    }

    /**
     * 删除充值规则
     * @param merchantId 商户ID
     * @param ruleId 规则ID
     */
    async deleteRechargeRule(merchantId: number, ruleId: number): Promise<{ message: string }> {
        return request({
            url: `/v1/merchants/${merchantId}/recharge-rules/${ruleId}`,
            method: 'DELETE'
        })
    }
}

// ==================== 会员设置管理服务类 ====================

/**
 * 会员设置管理服务
 * 提供会员设置的查询和更新功能
 */
export class MembershipSettingsService {
    /**
     * 获取商户会员设置
     */
    async getMembershipSettings(): Promise<MembershipSettingsResponse> {
        return request({
            url: '/v1/merchants/me/membership-settings',
            method: 'GET'
        })
    }

    /**
     * 更新商户会员设置
     * @param settingsData 设置数据
     */
    async updateMembershipSettings(settingsData: UpdateMembershipSettingsRequest): Promise<MembershipSettingsResponse> {
        return request({
            url: '/v1/merchants/me/membership-settings',
            method: 'PUT',
            data: settingsData
        })
    }
}

// ==================== 促销活动管理服务类 ====================

/**
 * 促销活动管理服务
 * 提供促销活动的查询功能
 */
export class PromotionManagementService {
    /**
     * 获取商户促销活动
     * @param merchantId 商户ID
     */
    async getMerchantPromotions(merchantId: number): Promise<MerchantPromotionsResponse> {
        return request({
            url: `/v1/merchants/${merchantId}/promotions`,
            method: 'GET'
        })
    }

    /**
     * 获取配送费促销活动
     * @param merchantId 商户ID
     */
    async getDeliveryFeePromotions(merchantId: number): Promise<any[]> {
        return request({
            url: `/delivery-fee/merchants/${merchantId}/promotions`,
            method: 'GET'
        })
    }

    /**
     * 创建配送费促销活动
     * @param merchantId 商户ID
     * @param promotionData 促销数据
     */
    async createDeliveryFeePromotion(merchantId: number, promotionData: Record<string, unknown>): Promise<any> {
        return request({
            url: `/delivery-fee/merchants/${merchantId}/promotions`,
            method: 'POST',
            data: promotionData
        })
    }

    /**
     * 删除配送费促销活动
     * @param merchantId 商户ID
     * @param promotionId 促销ID
     */
    async deleteDeliveryFeePromotion(merchantId: number, promotionId: number): Promise<{ message: string }> {
        return request({
            url: `/delivery-fee/merchants/${merchantId}/promotions/${promotionId}`,
            method: 'DELETE'
        })
    }
}

// ==================== 数据适配器 ====================

/**
 * 营销和会员管理数据适配器
 * 处理前端数据格式与后端API数据格式的转换
 */
export class MarketingMembershipAdapter {
    /**
     * 适配创建优惠券请求数据
     */
    static adaptCreateVoucherRequest(data: {
        name: string
        code: string
        amount: number
        totalQuantity: number
        validFrom: string
        validUntil: string
        description?: string
        minOrderAmount?: number
        allowedOrderTypes?: OrderType[]
    }): CreateVoucherRequest {
        return {
            name: data.name,
            code: data.code,
            amount: data.amount,
            total_quantity: data.totalQuantity,
            valid_from: data.validFrom,
            valid_until: data.validUntil,
            description: data.description,
            min_order_amount: data.minOrderAmount,
            allowed_order_types: data.allowedOrderTypes
        }
    }

    /**
     * 适配优惠券响应数据
     */
    static adaptVoucherResponse(data: VoucherResponse): {
        id: number
        merchantId: number
        name: string
        code: string
        description?: string
        amount: number
        minOrderAmount?: number
        totalQuantity: number
        claimedQuantity: number
        usedQuantity: number
        isActive: boolean
        validFrom: string
        validUntil: string
        allowedOrderTypes?: OrderType[]
        createdAt: string
    } {
        return {
            id: data.id,
            merchantId: data.merchant_id,
            name: data.name,
            code: data.code,
            description: data.description,
            amount: data.amount,
            minOrderAmount: data.min_order_amount,
            totalQuantity: data.total_quantity,
            claimedQuantity: data.claimed_quantity,
            usedQuantity: data.used_quantity,
            isActive: data.is_active,
            validFrom: data.valid_from,
            validUntil: data.valid_until,
            allowedOrderTypes: data.allowed_order_types,
            createdAt: data.created_at
        }
    }

    /**
     * 适配创建充值规则请求数据
     */
    static adaptCreateRechargeRuleRequest(data: {
        rechargeAmount: number
        bonusAmount: number
        validFrom: string
        validUntil: string
    }): CreateRechargeRuleRequest {
        return {
            recharge_amount: data.rechargeAmount,
            bonus_amount: data.bonusAmount,
            valid_from: data.validFrom,
            valid_until: data.validUntil
        }
    }

    /**
     * 适配充值规则响应数据
     */
    static adaptRechargeRuleResponse(data: RechargeRuleResponse): {
        id: number
        merchantId: number
        rechargeAmount: number
        bonusAmount: number
        isActive: boolean
        validFrom: string
        validUntil: string
        createdAt: string
    } {
        return {
            id: data.id,
            merchantId: data.merchant_id,
            rechargeAmount: data.recharge_amount,
            bonusAmount: data.bonus_amount,
            isActive: data.is_active,
            validFrom: data.valid_from,
            validUntil: data.valid_until,
            createdAt: data.created_at
        }
    }

    /**
     * 适配会员设置响应数据
     */
    static adaptMembershipSettingsResponse(data: MembershipSettingsResponse): {
        merchantId: number
        balanceUsableScenes: UsableScene[]
        bonusUsableScenes: UsableScene[]
        maxDeductionPercent: number
        allowWithDiscount: boolean
        allowWithVoucher: boolean
    } {
        return {
            merchantId: data.merchant_id,
            balanceUsableScenes: data.balance_usable_scenes,
            bonusUsableScenes: data.bonus_usable_scenes,
            maxDeductionPercent: data.max_deduction_percent,
            allowWithDiscount: data.allow_with_discount,
            allowWithVoucher: data.allow_with_voucher
        }
    }

    /**
     * 适配促销活动响应数据
     */
    static adaptMerchantPromotionsResponse(data: MerchantPromotionsResponse): {
        merchantId: number
        deliveryFeeRules: PromotionItem[]
        discountRules: PromotionItem[]
        vouchers: PromotionItem[]
    } {
        return {
            merchantId: data.merchant_id,
            deliveryFeeRules: data.delivery_fee_rules,
            discountRules: data.discount_rules,
            vouchers: data.vouchers
        }
    }
}

// ==================== 导出服务实例 ====================

export const voucherManagementService = new VoucherManagementService()
export const rechargeRuleManagementService = new RechargeRuleManagementService()
export const membershipSettingsService = new MembershipSettingsService()
export const promotionManagementService = new PromotionManagementService()

// ==================== 便捷函数 ====================

/**
 * 获取商户所有营销工具
 * @param merchantId 商户ID
 */
export async function getAllMarketingTools(merchantId: number): Promise<{
    vouchers: VoucherResponse[]
    rechargeRules: RechargeRuleResponse[]
    promotions: MerchantPromotionsResponse
    membershipSettings: MembershipSettingsResponse
}> {
    const [vouchers, rechargeRules, promotions, membershipSettings] = await Promise.all([
        voucherManagementService.listActiveVouchers(merchantId),
        rechargeRuleManagementService.listActiveRechargeRules(merchantId),
        promotionManagementService.getMerchantPromotions(merchantId),
        membershipSettingsService.getMembershipSettings()
    ])

    return {
        vouchers,
        rechargeRules,
        promotions,
        membershipSettings
    }
}

/**
 * 批量创建优惠券
 * @param merchantId 商户ID
 * @param vouchersData 优惠券数据列表
 */
export async function batchCreateVouchers(
    merchantId: number,
    vouchersData: CreateVoucherRequest[]
): Promise<{ voucherId?: number; success: boolean; message: string }[]> {
    const promises = vouchersData.map(async (voucherData) => {
        try {
            const result = await voucherManagementService.createVoucher(merchantId, voucherData)
            return { voucherId: result.id, success: true, message: '创建成功' }
        } catch (error: any) {
            return {
                success: false,
                message: error?.message || '创建失败'
            }
        }
    })

    return Promise.all(promises)
}

/**
 * 计算充值优惠
 * @param rechargeAmount 充值金额(分)
 * @param rechargeRules 充值规则列表
 */
export function calculateRechargeBonus(rechargeAmount: number, rechargeRules: RechargeRuleResponse[]): {
    applicableRule?: RechargeRuleResponse
    bonusAmount: number
    totalAmount: number
} {
    // 找到最优的充值规则（充值金额小于等于输入金额的最大规则）
    const applicableRule = rechargeRules
        .filter(rule => rule.is_active && rule.recharge_amount <= rechargeAmount)
        .sort((a, b) => b.recharge_amount - a.recharge_amount)[0]

    const bonusAmount = applicableRule?.bonus_amount || 0
    const totalAmount = rechargeAmount + bonusAmount

    return {
        applicableRule,
        bonusAmount,
        totalAmount
    }
}

/**
 * 验证优惠券是否可用
 * @param voucher 优惠券信息
 * @param orderAmount 订单金额(分)
 * @param orderType 订单类型
 */
export function validateVoucherUsage(
    voucher: VoucherResponse,
    orderAmount: number,
    orderType: OrderType
): { valid: boolean; reason?: string } {
    // 检查是否激活
    if (!voucher.is_active) {
        return { valid: false, reason: '优惠券已停用' }
    }

    // 检查是否过期
    const now = new Date()
    const validFrom = new Date(voucher.valid_from)
    const validUntil = new Date(voucher.valid_until)

    if (now < validFrom) {
        return { valid: false, reason: '优惠券尚未生效' }
    }

    if (now > validUntil) {
        return { valid: false, reason: '优惠券已过期' }
    }

    // 检查最低消费金额
    if (voucher.min_order_amount && orderAmount < voucher.min_order_amount) {
        return { valid: false, reason: `订单金额不足${voucher.min_order_amount / 100}元` }
    }

    // 检查订单类型
    if (voucher.allowed_order_types && voucher.allowed_order_types.length > 0) {
        if (!voucher.allowed_order_types.includes(orderType)) {
            return { valid: false, reason: '该优惠券不适用于当前订单类型' }
        }
    }

    // 检查库存
    if (voucher.claimed_quantity >= voucher.total_quantity) {
        return { valid: false, reason: '优惠券已被领完' }
    }

    return { valid: true }
}