"use strict";
/**
 * 优惠券系统接口
 * 包含优惠券列表、领取、我的优惠券等功能
 */
Object.defineProperty(exports, "__esModule", { value: true });
exports.CouponService = void 0;
const request_1 = require("../utils/request");
// ==================== 优惠券服务 ====================
class CouponService {
    /**
     * 获取可领取的优惠券列表
     * GET /v1/coupons
     */
    static async getAvailableCoupons(params) {
        return await (0, request_1.request)({
            url: '/v1/coupons',
            method: 'GET',
            data: params
        });
    }
    /**
     * 领取优惠券
     * POST /v1/coupons/:id/claim
     */
    static async claimCoupon(id) {
        return await (0, request_1.request)({
            url: `/v1/coupons/${id}/claim`,
            method: 'POST'
        });
    }
    /**
     * 获取我的优惠券列表
     * GET /v1/user/coupons
     */
    static async getMyCoupons(params) {
        return await (0, request_1.request)({
            url: '/v1/user/coupons',
            method: 'GET',
            data: params
        });
    }
}
exports.CouponService = CouponService;
exports.default = CouponService;
