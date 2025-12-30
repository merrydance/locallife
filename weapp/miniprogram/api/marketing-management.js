"use strict";
/**
 * 营销活动管理接口
 * 基于swagger.json完全重构，包含优惠券、充值规则、会员设置等
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
exports.MarketingAdapter = exports.PromotionService = exports.MembershipSettingsService = exports.RechargeRuleManagementService = exports.DiscountRuleManagementService = exports.VoucherManagementService = void 0;
const request_1 = require("../utils/request");
// ==================== 优惠券管理服务 ====================
/**
 * 优惠券管理服务
 */
class VoucherManagementService {
    /**
     * 获取商户优惠券列表
     * GET /v1/merchants/{id}/vouchers
     */
    static getVoucherList(merchantId, params) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/merchants/${merchantId}/vouchers?page_id=${params.page_id}&page_size=${params.page_size}`,
                method: 'GET'
            });
        });
    }
    /**
     * 获取可领取优惠券列表
     * GET /v1/merchants/{id}/vouchers/active
     */
    static getActiveVouchers(merchantId, params) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/merchants/${merchantId}/vouchers/active`,
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 创建优惠券
     * POST /v1/merchants/{id}/vouchers
     */
    static createVoucher(merchantId, data) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/merchants/${merchantId}/vouchers`,
                method: 'POST',
                data
            });
        });
    }
    /**
     * 删除优惠券
     * DELETE /v1/merchants/{id}/vouchers/{voucher_id}
     */
    static deleteVoucher(merchantId, voucherId) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/merchants/${merchantId}/vouchers/${voucherId}`,
                method: 'DELETE'
            });
        });
    }
    /**
     * 更新优惠券
     * PATCH /v1/merchants/{id}/vouchers/{voucher_id}
     */
    static updateVoucher(merchantId, voucherId, data) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/merchants/${merchantId}/vouchers/${voucherId}`,
                method: 'PATCH',
                data
            });
        });
    }
}
exports.VoucherManagementService = VoucherManagementService;
/**
 * 满减规则管理服务
 */
class DiscountRuleManagementService {
    /**
     * 获取满减规则列表
     * GET /v1/merchants/{id}/discounts
     */
    static getDiscountRuleList(merchantId, params) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/merchants/${merchantId}/discounts?page_id=${params.page_id}&page_size=${params.page_size}`,
                method: 'GET'
            });
        });
    }
    /**
     * 创建满减规则
     * POST /v1/merchants/{merchantId}/discounts
     */
    static createDiscountRule(merchantId, data) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/merchants/${merchantId}/discounts`,
                method: 'POST',
                data
            });
        });
    }
    /**
     * 更新满减规则
     * PATCH /v1/merchants/{merchantId}/discounts/{id}
     * 后端要求 id 必须在请求体中
     */
    static updateDiscountRule(merchantId, ruleId, data) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/merchants/${merchantId}/discounts/${ruleId}`,
                method: 'PATCH',
                data: Object.assign({ id: ruleId }, data)
            });
        });
    }
    /**
     * 删除满减规则
     * DELETE /v1/merchants/{merchantId}/discounts/{id}
     */
    static deleteDiscountRule(merchantId, ruleId) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/merchants/${merchantId}/discounts/${ruleId}`,
                method: 'DELETE'
            });
        });
    }
}
exports.DiscountRuleManagementService = DiscountRuleManagementService;
// ==================== 充值规则管理服务 ====================
/**
 * 充值规则管理服务
 */
class RechargeRuleManagementService {
    /**
     * 获取充值规则列表
     * GET /v1/merchants/{id}/recharge-rules
     */
    static getRechargeRules(merchantId) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/merchants/${merchantId}/recharge-rules`,
                method: 'GET'
            });
        });
    }
    /**
     * 获取生效中的充值规则
     * GET /v1/merchants/{id}/recharge-rules/active
     */
    static getActiveRechargeRules(merchantId) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/merchants/${merchantId}/recharge-rules/active`,
                method: 'GET'
            });
        });
    }
    /**
     * 创建充值规则
     * POST /v1/merchants/{id}/recharge-rules
     */
    static createRechargeRule(merchantId, data) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/merchants/${merchantId}/recharge-rules`,
                method: 'POST',
                data
            });
        });
    }
    /**
     * 更新充值规则
     * PATCH /v1/merchants/{id}/recharge-rules/{rule_id}
     */
    static updateRechargeRule(merchantId, ruleId, data) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/merchants/${merchantId}/recharge-rules/${ruleId}`,
                method: 'PATCH',
                data
            });
        });
    }
    /**
     * 删除充值规则
     * DELETE /v1/merchants/{id}/recharge-rules/{rule_id}
     */
    static deleteRechargeRule(merchantId, ruleId) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/merchants/${merchantId}/recharge-rules/${ruleId}`,
                method: 'DELETE'
            });
        });
    }
}
exports.RechargeRuleManagementService = RechargeRuleManagementService;
// ==================== 会员设置管理服务 ====================
/**
 * 会员设置管理服务
 */
class MembershipSettingsService {
    /**
     * 获取商户会员设置
     * GET /v1/merchants/me/membership-settings
     */
    static getMembershipSettings() {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/merchants/me/membership-settings',
                method: 'GET'
            });
        });
    }
    /**
     * 更新商户会员设置
     * PUT /v1/merchants/me/membership-settings
     */
    static updateMembershipSettings(data) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/merchants/me/membership-settings',
                method: 'PUT',
                data
            });
        });
    }
}
exports.MembershipSettingsService = MembershipSettingsService;
// ==================== 营销活动服务 ====================
/**
 * 营销活动服务
 */
class PromotionService {
    /**
     * 获取商户优惠活动
     * GET /v1/merchants/{id}/promotions
     */
    static getMerchantPromotions(merchantId) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/merchants/${merchantId}/promotions`,
                method: 'GET'
            });
        });
    }
}
exports.PromotionService = PromotionService;
// ==================== 营销管理适配器 ====================
/**
 * 营销管理数据适配器
 */
class MarketingAdapter {
    /**
     * 格式化金额显示（分转元）
     */
    static formatAmount(amountInCents) {
        return (amountInCents / 100).toFixed(2);
    }
    /**
     * 格式化折扣类型
     */
    static formatDiscountType(type) {
        const typeMap = {
            'fixed': '立减',
            'percentage': '折扣'
        };
        return typeMap[type] || type;
    }
    /**
     * 格式化折扣值
     */
    static formatDiscountValue(type, value) {
        if (type === 'fixed') {
            return `¥${this.formatAmount(value)}`;
        }
        else {
            return `${value / 10}折`;
        }
    }
    /**
     * 计算充值优惠比例
     */
    static calculateBonusRate(rechargeAmount, bonusAmount) {
        if (rechargeAmount === 0)
            return '0%';
        const rate = (bonusAmount / rechargeAmount) * 100;
        return `${rate.toFixed(1)}%`;
    }
    /**
     * 判断优惠券是否已过期
     */
    static isVoucherExpired(voucher) {
        return new Date(voucher.valid_until) < new Date();
    }
    /**
     * 判断优惠券是否已领完
     */
    static isVoucherSoldOut(voucher) {
        return voucher.claimed_quantity >= voucher.total_quantity;
    }
    /**
     * 获取优惠券状态文本
     */
    static getVoucherStatusText(voucher) {
        if (!voucher.is_active)
            return '已停用';
        if (this.isVoucherExpired(voucher))
            return '已过期';
        if (this.isVoucherSoldOut(voucher))
            return '已领完';
        return '进行中';
    }
    /**
     * 获取优惠券状态颜色
     */
    static getVoucherStatusColor(voucher) {
        if (!voucher.is_active)
            return '#999';
        if (this.isVoucherExpired(voucher))
            return '#999';
        if (this.isVoucherSoldOut(voucher))
            return '#fa8c16';
        return '#52c41a';
    }
    /**
     * 判断充值规则是否已过期
     */
    static isRechargeRuleExpired(rule) {
        if (!rule.valid_until)
            return false;
        return new Date(rule.valid_until) < new Date();
    }
    /**
     * 获取充值规则状态文本
     */
    static getRechargeRuleStatusText(rule) {
        if (!rule.is_active)
            return '已停用';
        if (this.isRechargeRuleExpired(rule))
            return '已过期';
        return '进行中';
    }
    /**
     * 格式化适用订单类型
     */
    static formatOrderTypes(types) {
        const typeMap = {
            'takeout': '外卖',
            'dine_in': '堂食',
            'takeaway': '打包自取',
            'reservation': '预定'
        };
        return types.map(t => typeMap[t] || t).join('、');
    }
}
exports.MarketingAdapter = MarketingAdapter;
// ==================== 导出默认服务 ====================
exports.default = {
    VoucherManagementService,
    RechargeRuleManagementService,
    MembershipSettingsService,
    PromotionService,
    MarketingAdapter
};
