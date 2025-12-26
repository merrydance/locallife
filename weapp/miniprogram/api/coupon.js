"use strict";
/**
 * 优惠券系统接口
 * 包含优惠券列表、领取、我的优惠券等功能
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
exports.CouponService = void 0;
const request_1 = require("../utils/request");
// ==================== 优惠券服务 ====================
class CouponService {
    /**
     * 获取可领取的优惠券列表
     * GET /v1/coupons
     */
    static getAvailableCoupons(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/coupons',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 领取优惠券
     * POST /v1/coupons/:id/claim
     */
    static claimCoupon(id) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/coupons/${id}/claim`,
                method: 'POST'
            });
        });
    }
    /**
     * 获取我的优惠券列表
     * GET /v1/user/coupons
     */
    static getMyCoupons(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/user/coupons',
                method: 'GET',
                data: params
            });
        });
    }
}
exports.CouponService = CouponService;
exports.default = CouponService;
