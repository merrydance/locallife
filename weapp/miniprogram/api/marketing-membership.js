"use strict";
/**
 * 营销和会员管理接口重构 (Task 2.5)
 * 基于swagger.json完全重构，移除所有没有后端支持的旧功能
 * 包含：优惠券管理、充值规则管理、促销活动管理、会员设置管理
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
exports.promotionManagementService = exports.membershipSettingsService = exports.rechargeRuleManagementService = exports.voucherManagementService = exports.MarketingMembershipAdapter = exports.PromotionManagementService = exports.MembershipSettingsService = exports.RechargeRuleManagementService = exports.VoucherManagementService = void 0;
exports.getAllMarketingTools = getAllMarketingTools;
exports.batchCreateVouchers = batchCreateVouchers;
exports.calculateRechargeBonus = calculateRechargeBonus;
exports.validateVoucherUsage = validateVoucherUsage;
const request_1 = require("../utils/request");
// ==================== 优惠券管理服务类 ====================
/**
 * 优惠券管理服务
 * 提供优惠券的CRUD操作、状态管理等功能
 */
class VoucherManagementService {
    /**
     * 获取商户优惠券列表
     * @param merchantId 商户ID
     * @param params 查询参数
     */
    listVouchers(merchantId, params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/merchants/${merchantId}/vouchers`,
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取商户活跃优惠券列表
     * @param merchantId 商户ID
     */
    listActiveVouchers(merchantId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/merchants/${merchantId}/vouchers/active`,
                method: 'GET'
            });
        });
    }
    /**
     * 创建优惠券
     * @param merchantId 商户ID
     * @param voucherData 优惠券数据
     */
    createVoucher(merchantId, voucherData) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/merchants/${merchantId}/vouchers`,
                method: 'POST',
                data: voucherData
            });
        });
    }
    /**
     * 更新优惠券
     * @param merchantId 商户ID
     * @param voucherId 优惠券ID
     * @param voucherData 更新数据
     */
    updateVoucher(merchantId, voucherId, voucherData) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/merchants/${merchantId}/vouchers/${voucherId}`,
                method: 'PUT',
                data: voucherData
            });
        });
    }
    /**
     * 删除优惠券
     * @param merchantId 商户ID
     * @param voucherId 优惠券ID
     */
    deleteVoucher(merchantId, voucherId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/merchants/${merchantId}/vouchers/${voucherId}`,
                method: 'DELETE'
            });
        });
    }
}
exports.VoucherManagementService = VoucherManagementService;
// ==================== 充值规则管理服务类 ====================
/**
 * 充值规则管理服务
 * 提供充值规则的CRUD操作、状态管理等功能
 */
class RechargeRuleManagementService {
    /**
     * 获取商户充值规则列表
     * @param merchantId 商户ID
     */
    listRechargeRules(merchantId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/merchants/${merchantId}/recharge-rules`,
                method: 'GET'
            });
        });
    }
    /**
     * 获取商户活跃充值规则列表
     * @param merchantId 商户ID
     */
    listActiveRechargeRules(merchantId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/merchants/${merchantId}/recharge-rules/active`,
                method: 'GET'
            });
        });
    }
    /**
     * 创建充值规则
     * @param merchantId 商户ID
     * @param ruleData 充值规则数据
     */
    createRechargeRule(merchantId, ruleData) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/merchants/${merchantId}/recharge-rules`,
                method: 'POST',
                data: ruleData
            });
        });
    }
    /**
     * 更新充值规则
     * @param merchantId 商户ID
     * @param ruleId 规则ID
     * @param ruleData 更新数据
     */
    updateRechargeRule(merchantId, ruleId, ruleData) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/merchants/${merchantId}/recharge-rules/${ruleId}`,
                method: 'PATCH',
                data: ruleData
            });
        });
    }
    /**
     * 删除充值规则
     * @param merchantId 商户ID
     * @param ruleId 规则ID
     */
    deleteRechargeRule(merchantId, ruleId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/merchants/${merchantId}/recharge-rules/${ruleId}`,
                method: 'DELETE'
            });
        });
    }
}
exports.RechargeRuleManagementService = RechargeRuleManagementService;
// ==================== 会员设置管理服务类 ====================
/**
 * 会员设置管理服务
 * 提供会员设置的查询和更新功能
 */
class MembershipSettingsService {
    /**
     * 获取商户会员设置
     */
    getMembershipSettings() {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/merchants/me/membership-settings',
                method: 'GET'
            });
        });
    }
    /**
     * 更新商户会员设置
     * @param settingsData 设置数据
     */
    updateMembershipSettings(settingsData) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/merchants/me/membership-settings',
                method: 'PUT',
                data: settingsData
            });
        });
    }
}
exports.MembershipSettingsService = MembershipSettingsService;
// ==================== 促销活动管理服务类 ====================
/**
 * 促销活动管理服务
 * 提供促销活动的查询功能
 */
class PromotionManagementService {
    /**
     * 获取商户促销活动
     * @param merchantId 商户ID
     */
    getMerchantPromotions(merchantId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/merchants/${merchantId}/promotions`,
                method: 'GET'
            });
        });
    }
    /**
     * 获取配送费促销活动
     * @param merchantId 商户ID
     */
    getDeliveryFeePromotions(merchantId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/delivery-fee/merchants/${merchantId}/promotions`,
                method: 'GET'
            });
        });
    }
    /**
     * 创建配送费促销活动
     * @param merchantId 商户ID
     * @param promotionData 促销数据
     */
    createDeliveryFeePromotion(merchantId, promotionData) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/delivery-fee/merchants/${merchantId}/promotions`,
                method: 'POST',
                data: promotionData
            });
        });
    }
    /**
     * 删除配送费促销活动
     * @param merchantId 商户ID
     * @param promotionId 促销ID
     */
    deleteDeliveryFeePromotion(merchantId, promotionId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/delivery-fee/merchants/${merchantId}/promotions/${promotionId}`,
                method: 'DELETE'
            });
        });
    }
}
exports.PromotionManagementService = PromotionManagementService;
// ==================== 数据适配器 ====================
/**
 * 营销和会员管理数据适配器
 * 处理前端数据格式与后端API数据格式的转换
 */
class MarketingMembershipAdapter {
    /**
     * 适配创建优惠券请求数据
     */
    static adaptCreateVoucherRequest(data) {
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
        };
    }
    /**
     * 适配优惠券响应数据
     */
    static adaptVoucherResponse(data) {
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
        };
    }
    /**
     * 适配创建充值规则请求数据
     */
    static adaptCreateRechargeRuleRequest(data) {
        return {
            recharge_amount: data.rechargeAmount,
            bonus_amount: data.bonusAmount,
            valid_from: data.validFrom,
            valid_until: data.validUntil
        };
    }
    /**
     * 适配充值规则响应数据
     */
    static adaptRechargeRuleResponse(data) {
        return {
            id: data.id,
            merchantId: data.merchant_id,
            rechargeAmount: data.recharge_amount,
            bonusAmount: data.bonus_amount,
            isActive: data.is_active,
            validFrom: data.valid_from,
            validUntil: data.valid_until,
            createdAt: data.created_at
        };
    }
    /**
     * 适配会员设置响应数据
     */
    static adaptMembershipSettingsResponse(data) {
        return {
            merchantId: data.merchant_id,
            balanceUsableScenes: data.balance_usable_scenes,
            bonusUsableScenes: data.bonus_usable_scenes,
            maxDeductionPercent: data.max_deduction_percent,
            allowWithDiscount: data.allow_with_discount,
            allowWithVoucher: data.allow_with_voucher
        };
    }
    /**
     * 适配促销活动响应数据
     */
    static adaptMerchantPromotionsResponse(data) {
        return {
            merchantId: data.merchant_id,
            deliveryFeeRules: data.delivery_fee_rules,
            discountRules: data.discount_rules,
            vouchers: data.vouchers
        };
    }
}
exports.MarketingMembershipAdapter = MarketingMembershipAdapter;
// ==================== 导出服务实例 ====================
exports.voucherManagementService = new VoucherManagementService();
exports.rechargeRuleManagementService = new RechargeRuleManagementService();
exports.membershipSettingsService = new MembershipSettingsService();
exports.promotionManagementService = new PromotionManagementService();
// ==================== 便捷函数 ====================
/**
 * 获取商户所有营销工具
 * @param merchantId 商户ID
 */
function getAllMarketingTools(merchantId) {
    return __awaiter(this, void 0, void 0, function* () {
        const [vouchers, rechargeRules, promotions, membershipSettings] = yield Promise.all([
            exports.voucherManagementService.listActiveVouchers(merchantId),
            exports.rechargeRuleManagementService.listActiveRechargeRules(merchantId),
            exports.promotionManagementService.getMerchantPromotions(merchantId),
            exports.membershipSettingsService.getMembershipSettings()
        ]);
        return {
            vouchers,
            rechargeRules,
            promotions,
            membershipSettings
        };
    });
}
/**
 * 批量创建优惠券
 * @param merchantId 商户ID
 * @param vouchersData 优惠券数据列表
 */
function batchCreateVouchers(merchantId, vouchersData) {
    return __awaiter(this, void 0, void 0, function* () {
        const promises = vouchersData.map((voucherData) => __awaiter(this, void 0, void 0, function* () {
            try {
                const result = yield exports.voucherManagementService.createVoucher(merchantId, voucherData);
                return { voucherId: result.id, success: true, message: '创建成功' };
            }
            catch (error) {
                return {
                    success: false,
                    message: (error === null || error === void 0 ? void 0 : error.message) || '创建失败'
                };
            }
        }));
        return Promise.all(promises);
    });
}
/**
 * 计算充值优惠
 * @param rechargeAmount 充值金额(分)
 * @param rechargeRules 充值规则列表
 */
function calculateRechargeBonus(rechargeAmount, rechargeRules) {
    // 找到最优的充值规则（充值金额小于等于输入金额的最大规则）
    const applicableRule = rechargeRules
        .filter(rule => rule.is_active && rule.recharge_amount <= rechargeAmount)
        .sort((a, b) => b.recharge_amount - a.recharge_amount)[0];
    const bonusAmount = (applicableRule === null || applicableRule === void 0 ? void 0 : applicableRule.bonus_amount) || 0;
    const totalAmount = rechargeAmount + bonusAmount;
    return {
        applicableRule,
        bonusAmount,
        totalAmount
    };
}
/**
 * 验证优惠券是否可用
 * @param voucher 优惠券信息
 * @param orderAmount 订单金额(分)
 * @param orderType 订单类型
 */
function validateVoucherUsage(voucher, orderAmount, orderType) {
    // 检查是否激活
    if (!voucher.is_active) {
        return { valid: false, reason: '优惠券已停用' };
    }
    // 检查是否过期
    const now = new Date();
    const validFrom = new Date(voucher.valid_from);
    const validUntil = new Date(voucher.valid_until);
    if (now < validFrom) {
        return { valid: false, reason: '优惠券尚未生效' };
    }
    if (now > validUntil) {
        return { valid: false, reason: '优惠券已过期' };
    }
    // 检查最低消费金额
    if (voucher.min_order_amount && orderAmount < voucher.min_order_amount) {
        return { valid: false, reason: `订单金额不足${voucher.min_order_amount / 100}元` };
    }
    // 检查订单类型
    if (voucher.allowed_order_types && voucher.allowed_order_types.length > 0) {
        if (!voucher.allowed_order_types.includes(orderType)) {
            return { valid: false, reason: '该优惠券不适用于当前订单类型' };
        }
    }
    // 检查库存
    if (voucher.claimed_quantity >= voucher.total_quantity) {
        return { valid: false, reason: '优惠券已被领完' };
    }
    return { valid: true };
}
