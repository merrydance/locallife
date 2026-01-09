"use strict";
/**
 * 优惠券服务
 * 使用真实后端API
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
const marketing_membership_1 = require("../api/marketing-membership");
class CouponService {
    constructor() { }
    static getInstance() {
        if (!CouponService.instance) {
            CouponService.instance = new CouponService();
        }
        return CouponService.instance;
    }
    /**
     * 获取商户可用优惠券
     */
    getAvailableCoupons(merchantId) {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const merchantIdNum = parseInt(merchantId);
                if (isNaN(merchantIdNum)) {
                    return [];
                }
                const vouchers = yield marketing_membership_1.voucherManagementService.listActiveVouchers(merchantIdNum);
                return this.convertVouchersToCoupons(vouchers);
            }
            catch (error) {
                console.error('获取优惠券失败:', error);
                return [];
            }
        });
    }
    /**
     * 获取用户优惠券
     * 注意：后端暂无用户优惠券列表接口，返回空数组
     */
    getUserCoupons() {
        return __awaiter(this, void 0, void 0, function* () {
            // TODO: 后端需要实现 GET /v1/customers/vouchers 接口
            console.warn('getUserCoupons: 后端暂无用户优惠券列表接口');
            return [];
        });
    }
    /**
     * 领取优惠券
     * 注意：后端暂无领取优惠券接口
     */
    claimCoupon(couponId) {
        return __awaiter(this, void 0, void 0, function* () {
            // TODO: 后端需要实现 POST /v1/vouchers/{id}/claim 接口
            console.warn('claimCoupon: 后端暂无领取优惠券接口');
            return false;
        });
    }
    /**
     * 计算优惠金额
     */
    calculateDiscount(amount, coupon) {
        if (amount < coupon.minAmount)
            return 0;
        if (coupon.type === 'AMOUNT') {
            return coupon.value;
        }
        else if (coupon.type === 'DISCOUNT') {
            // value is percentage, e.g., 85 for 8.5折
            const discount = Math.floor(amount * (100 - coupon.value) / 100);
            return discount;
        }
        return 0;
    }
    /**
     * 转换后端优惠券格式为前端格式
     */
    convertVouchersToCoupons(vouchers) {
        const now = new Date();
        return vouchers.map(voucher => {
            const validFrom = new Date(voucher.valid_from);
            const validUntil = new Date(voucher.valid_until);
            let status = 'AVAILABLE';
            if (now > validUntil) {
                status = 'EXPIRED';
            }
            else if (voucher.claimed_quantity >= voucher.total_quantity) {
                status = 'USED'; // 已领完
            }
            return {
                id: String(voucher.id),
                name: voucher.name,
                type: 'AMOUNT', // 后端目前只支持金额类型
                value: voucher.amount,
                minAmount: voucher.min_order_amount || 0,
                startTime: voucher.valid_from,
                endTime: voucher.valid_until,
                status,
                description: voucher.description,
                code: voucher.code
            };
        });
    }
}
exports.CouponService = CouponService;
exports.default = CouponService.getInstance();
