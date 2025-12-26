/**
 * 营销活动管理接口
 * 基于swagger.json完全重构，包含优惠券、充值规则、会员设置等
 */

import { request } from '../utils/request'

// ==================== 优惠券数据类型定义 ====================

/**
 * 优惠券响应 - 对齐 api.voucherResponse
 */
export interface VoucherResponse {
    allowed_order_types: string[]                // 允许的订单类型
    amount: number                               // 优惠金额（分）
    claimed_quantity: number                     // 已领取数量
    code: string                                 // 优惠券码
    created_at: string                           // 创建时间
    description?: string                         // 优惠券描述
    id: number                                   // 优惠券ID
    is_active: boolean                           // 是否激活
    merchant_id: number                          // 商户ID
    min_order_amount: number                     // 最低订单金额（分）
    name: string                                 // 优惠券名称
    total_quantity: number                       // 总发行量
    used_quantity: number                        // 已使用数量
    valid_from: string                           // 生效时间
    valid_until: string                          // 失效时间
}

/**
 * 创建优惠券请求 - 对齐 api.createVoucherRequest
 */
export interface CreateVoucherRequest extends Record<string, unknown> {
    allowed_order_types?: string[]               // 允许的订单类型
    amount: number                               // 优惠金额（必填）
    code: string                                 // 优惠券码（必填）
    description?: string                         // 优惠券描述
    min_order_amount?: number                    // 最低订单金额
    name: string                                 // 优惠券名称（必填）
    total_quantity: number                       // 总发行量（必填）
    valid_from: string                           // 生效时间（必填）
    valid_until: string                          // 失效时间（必填）
}

// ==================== 充值规则数据类型定义 ====================

/**
 * 充值规则响应 - 对齐 api.rechargeRuleResponse
 */
export interface RechargeRuleResponse {
    bonus_amount: number                         // 赠送金额（分）
    created_at: string                           // 创建时间
    id: number                                   // 规则ID
    is_active: boolean                           // 是否激活
    merchant_id: number                          // 商户ID
    recharge_amount: number                      // 充值金额（分）
    valid_from?: string                          // 生效时间
    valid_until?: string                         // 失效时间
}

/**
 * 创建充值规则请求 - 对齐 api.createRechargeRuleRequest
 */
export interface CreateRechargeRuleRequest extends Record<string, unknown> {
    bonus_amount: number                         // 赠送金额（必填）
    recharge_amount: number                      // 充值金额（必填）
    valid_from?: string                          // 生效时间
    valid_until?: string                         // 失效时间
}

/**
 * 更新充值规则请求 - 对齐 api.updateRechargeRuleRequest
 */
export interface UpdateRechargeRuleRequest extends Record<string, unknown> {
    bonus_amount?: number                        // 赠送金额
    is_active?: boolean                          // 是否激活
    recharge_amount?: number                     // 充值金额
    valid_from?: string                          // 生效时间
    valid_until?: string                         // 失效时间
}

// ==================== 会员设置数据类型定义 ====================

/**
 * 会员设置响应 - 对齐 api.membershipSettingsResponse
 */
export interface MembershipSettingsResponse {
    allow_with_discount: boolean                 // 是否允许与折扣叠加
    allow_with_voucher: boolean                  // 是否允许与优惠券叠加
    balance_usable_scenes: string[]              // 余额使用场景
    bonus_usable_scenes: string[]                // 赠送金使用场景
    max_deduction_percent: number                // 最大抵扣比例
    merchant_id: number                          // 商户ID
}

/**
 * 更新会员设置请求 - 对齐 api.updateMembershipSettingsRequest
 */
export interface UpdateMembershipSettingsRequest extends Record<string, unknown> {
    allow_with_discount?: boolean                // 是否允许与折扣叠加
    allow_with_voucher?: boolean                 // 是否允许与优惠券叠加
    balance_usable_scenes?: string[]             // 余额使用场景
    bonus_usable_scenes?: string[]               // 赠送金使用场景
    max_deduction_percent?: number               // 最大抵扣比例
}

// ==================== 显示配置数据类型定义 ====================

/**
 * 显示配置响应 - 对齐 api.getDisplayConfigResponse
 */
export interface GetDisplayConfigResponse {
    created_at?: string                          // 创建时间
    enable_kds?: boolean                         // 是否启用KDS
    enable_print?: boolean                       // 是否启用打印
    enable_voice?: boolean                       // 是否启用语音
    id?: number                                  // 配置ID
    kds_url?: string                             // KDS URL
    merchant_id?: number                         // 商户ID
    print_dine_in?: boolean                      // 堂食打印
    print_reservation?: boolean                  // 预定打印
    print_takeout?: boolean                      // 外卖打印
    updated_at?: string                          // 更新时间
    voice_dine_in?: boolean                      // 堂食语音
    voice_takeout?: boolean                      // 外卖语音
}

// ==================== 营销活动数据类型定义 ====================

/**
 * 商户营销活动响应 - 对齐 api.merchantPromotionsResponse
 */
export interface MerchantPromotionsResponse {
    delivery_fee_rules: PromotionItem[]          // 满返运费
    discount_rules: PromotionItem[]              // 满减活动
    merchant_id: number                          // 商户ID
    vouchers: PromotionItem[]                    // 可领优惠券
}

/**
 * 营销活动项 - 对齐 api.promotionItem
 */
export interface PromotionItem {
    description?: string                         // 优惠描述
    min_amount?: number                          // 起点金额（分）
    title?: string                               // 优惠标题
    type?: string                                // delivery_fee_return, discount, voucher
    valid_until?: string                         // 有效期
    value?: number                               // 优惠金额或比例
}

// ==================== 优惠券管理服务 ====================

/**
 * 优惠券管理服务
 */
export class VoucherManagementService {

    /**
     * 获取商户优惠券列表
     * GET /v1/merchants/{id}/vouchers
     */
    static async getVoucherList(merchantId: number, params: {
        page_id: number                            // 页码（必填）
        page_size: number                          // 每页数量（必填，5-50）
    }): Promise<VoucherResponse[]> {
        return await request({
            url: `/v1/merchants/${merchantId}/vouchers`,
            method: 'GET',
            data: params
        })
    }

    /**
     * 获取可领取优惠券列表
     * GET /v1/merchants/{id}/vouchers/active
     */
    static async getActiveVouchers(merchantId: number, params: {
        page_id: number                            // 页码（必填）
        page_size: number                          // 每页数量（必填，5-50）
    }): Promise<VoucherResponse[]> {
        return await request({
            url: `/v1/merchants/${merchantId}/vouchers/active`,
            method: 'GET',
            data: params
        })
    }

    /**
     * 创建优惠券
     * POST /v1/merchants/{id}/vouchers
     */
    static async createVoucher(merchantId: number, data: CreateVoucherRequest): Promise<VoucherResponse> {
        return await request({
            url: `/v1/merchants/${merchantId}/vouchers`,
            method: 'POST',
            data
        })
    }

    /**
     * 删除优惠券
     * DELETE /v1/merchants/{id}/vouchers/{voucher_id}
     */
    static async deleteVoucher(merchantId: number, voucherId: number): Promise<void> {
        return await request({
            url: `/v1/merchants/${merchantId}/vouchers/${voucherId}`,
            method: 'DELETE'
        })
    }
}

// ==================== 充值规则管理服务 ====================

/**
 * 充值规则管理服务
 */
export class RechargeRuleManagementService {

    /**
     * 获取充值规则列表
     * GET /v1/merchants/{id}/recharge-rules
     */
    static async getRechargeRules(merchantId: number): Promise<RechargeRuleResponse[]> {
        return await request({
            url: `/v1/merchants/${merchantId}/recharge-rules`,
            method: 'GET'
        })
    }

    /**
     * 获取生效中的充值规则
     * GET /v1/merchants/{id}/recharge-rules/active
     */
    static async getActiveRechargeRules(merchantId: number): Promise<RechargeRuleResponse[]> {
        return await request({
            url: `/v1/merchants/${merchantId}/recharge-rules/active`,
            method: 'GET'
        })
    }

    /**
     * 创建充值规则
     * POST /v1/merchants/{id}/recharge-rules
     */
    static async createRechargeRule(merchantId: number, data: CreateRechargeRuleRequest): Promise<RechargeRuleResponse> {
        return await request({
            url: `/v1/merchants/${merchantId}/recharge-rules`,
            method: 'POST',
            data
        })
    }

    /**
     * 更新充值规则
     * PATCH /v1/merchants/{id}/recharge-rules/{rule_id}
     */
    static async updateRechargeRule(
        merchantId: number,
        ruleId: number,
        data: UpdateRechargeRuleRequest
    ): Promise<RechargeRuleResponse> {
        return await request({
            url: `/v1/merchants/${merchantId}/recharge-rules/${ruleId}`,
            method: 'PATCH',
            data
        })
    }

    /**
     * 删除充值规则
     * DELETE /v1/merchants/{id}/recharge-rules/{rule_id}
     */
    static async deleteRechargeRule(merchantId: number, ruleId: number): Promise<void> {
        return await request({
            url: `/v1/merchants/${merchantId}/recharge-rules/${ruleId}`,
            method: 'DELETE'
        })
    }
}

// ==================== 会员设置管理服务 ====================

/**
 * 会员设置管理服务
 */
export class MembershipSettingsService {

    /**
     * 获取商户会员设置
     * GET /v1/merchants/me/membership-settings
     */
    static async getMembershipSettings(): Promise<MembershipSettingsResponse> {
        return await request({
            url: '/v1/merchants/me/membership-settings',
            method: 'GET'
        })
    }

    /**
     * 更新商户会员设置
     * PUT /v1/merchants/me/membership-settings
     */
    static async updateMembershipSettings(data: UpdateMembershipSettingsRequest): Promise<MembershipSettingsResponse> {
        return await request({
            url: '/v1/merchants/me/membership-settings',
            method: 'PUT',
            data
        })
    }
}

// ==================== 营销活动服务 ====================

/**
 * 营销活动服务
 */
export class PromotionService {

    /**
     * 获取商户优惠活动
     * GET /v1/merchants/{id}/promotions
     */
    static async getMerchantPromotions(merchantId: number): Promise<MerchantPromotionsResponse> {
        return await request({
            url: `/v1/merchants/${merchantId}/promotions`,
            method: 'GET'
        })
    }
}

// ==================== 营销管理适配器 ====================

/**
 * 营销管理数据适配器
 */
export class MarketingAdapter {

    /**
     * 格式化金额显示（分转元）
     */
    static formatAmount(amountInCents: number): string {
        return (amountInCents / 100).toFixed(2)
    }

    /**
     * 格式化折扣类型
     */
    static formatDiscountType(type: string): string {
        const typeMap: Record<string, string> = {
            'fixed': '立减',
            'percentage': '折扣'
        }
        return typeMap[type] || type
    }

    /**
     * 格式化折扣值
     */
    static formatDiscountValue(type: string, value: number): string {
        if (type === 'fixed') {
            return `¥${this.formatAmount(value)}`
        } else {
            return `${value / 10}折`
        }
    }

    /**
     * 计算充值优惠比例
     */
    static calculateBonusRate(rechargeAmount: number, bonusAmount: number): string {
        if (rechargeAmount === 0) return '0%'
        const rate = (bonusAmount / rechargeAmount) * 100
        return `${rate.toFixed(1)}%`
    }

    /**
     * 判断优惠券是否已过期
     */
    static isVoucherExpired(voucher: VoucherResponse): boolean {
        return new Date(voucher.valid_until) < new Date()
    }

    /**
     * 判断优惠券是否已领完
     */
    static isVoucherSoldOut(voucher: VoucherResponse): boolean {
        return voucher.claimed_quantity >= voucher.total_quantity
    }

    /**
     * 获取优惠券状态文本
     */
    static getVoucherStatusText(voucher: VoucherResponse): string {
        if (!voucher.is_active) return '已停用'
        if (this.isVoucherExpired(voucher)) return '已过期'
        if (this.isVoucherSoldOut(voucher)) return '已领完'
        return '进行中'
    }

    /**
     * 获取优惠券状态颜色
     */
    static getVoucherStatusColor(voucher: VoucherResponse): string {
        if (!voucher.is_active) return '#999'
        if (this.isVoucherExpired(voucher)) return '#999'
        if (this.isVoucherSoldOut(voucher)) return '#fa8c16'
        return '#52c41a'
    }

    /**
     * 判断充值规则是否已过期
     */
    static isRechargeRuleExpired(rule: RechargeRuleResponse): boolean {
        if (!rule.valid_until) return false
        return new Date(rule.valid_until) < new Date()
    }

    /**
     * 获取充值规则状态文本
     */
    static getRechargeRuleStatusText(rule: RechargeRuleResponse): string {
        if (!rule.is_active) return '已停用'
        if (this.isRechargeRuleExpired(rule)) return '已过期'
        return '进行中'
    }

    /**
     * 格式化适用订单类型
     */
    static formatOrderTypes(types: string[]): string {
        const typeMap: Record<string, string> = {
            'takeout': '外卖',
            'dine_in': '堂食',
            'takeaway': '打包自取',
            'reservation': '预定'
        }
        return types.map(t => typeMap[t] || t).join('、')
    }
}

// ==================== 导出默认服务 ====================

export default {
    VoucherManagementService,
    RechargeRuleManagementService,
    MembershipSettingsService,
    PromotionService,
    MarketingAdapter
}
