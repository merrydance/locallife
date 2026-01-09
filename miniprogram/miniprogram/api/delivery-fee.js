"use strict";
/**
 * 配送费管理接口 (Phase 4)
 * 基于 swagger.json 实现
 * 包含：配送费计算、区域配置、峰时配置、商户促销
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
exports.deliveryFeeService = exports.DeliveryFeeAdapter = exports.DeliveryFeeService = void 0;
const request_1 = require("../utils/request");
// ==================== 配送费管理服务类 ====================
class DeliveryFeeService {
    /**
     * 计算配送费
     */
    calculateFee(data) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/delivery-fee/calculate',
                method: 'POST',
                data
            });
        });
    }
    /**
     * 获取区域配送费配置
     * @param regionId 区域ID
     */
    getRegionConfig(regionId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/delivery-fee/config/${regionId}`,
                method: 'GET'
            });
        });
    }
    /**
     * 创建/更新区域配送费配置 (Operator)
     * @param regionId 区域ID
     * @param data 配置数据
     */
    updateRegionConfig(regionId, data) {
        return __awaiter(this, void 0, void 0, function* () {
            // 尝试创建，如果已存在则后端会返回409建议走PATCH，或者前端先查询。
            // 根据Swagger，POST是Create，PATCH是Update。
            // 这里合并逻辑：通常业务上会先Get，若无则Post，若有则Patch。
            // 或者是为了简化 UI 调用，我们可以拆分。
            // 此处严格遵循 Swagger: POST /delivery-fee/regions/{id}/config
            return (0, request_1.request)({
                url: `/v1/delivery-fee/regions/${regionId}/config`,
                method: 'POST',
                data
            });
        });
    }
    patchRegionConfig(regionId, data) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/delivery-fee/regions/${regionId}/config`,
                method: 'PATCH',
                data
            });
        });
    }
    /**
     * 获取区域峰时配置列表 (Operator)
     */
    getPeakConfigs(regionId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/operator/regions/${regionId}/peak-hours`,
                method: 'GET'
            });
        });
    }
    /**
     * 创建峰时配置 (Operator)
     */
    createPeakConfig(regionId, data) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/operator/regions/${regionId}/peak-hours`,
                method: 'POST',
                data
            });
        });
    }
    /**
     * 删除峰时配置 (Operator)
     */
    deletePeakConfig(id) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/operator/peak-hours/${id}`,
                method: 'DELETE'
            });
        });
    }
    /**
     * 获取商户配送优惠列表 (Merchant)
     */
    getMerchantPromotions(merchantId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/delivery-fee/merchants/${merchantId}/promotions`,
                method: 'GET'
            });
        });
    }
    /**
     * 创建商户配送优惠 (Merchant)
     */
    createMerchantPromotion(merchantId, data) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/delivery-fee/merchants/${merchantId}/promotions`,
                method: 'POST',
                data
            });
        });
    }
    /**
     * 删除商户配送优惠 (Merchant)
     */
    deleteMerchantPromotion(merchantId, promoId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/delivery-fee/merchants/${merchantId}/promotions/${promoId}`,
                method: 'DELETE'
            });
        });
    }
}
exports.DeliveryFeeService = DeliveryFeeService;
// ==================== 数据适配器 ====================
class DeliveryFeeAdapter {
    static formatFee(fee) {
        return (fee / 100).toFixed(2);
    }
    static formatDistance(meters) {
        if (meters < 1000)
            return `${meters}m`;
        return `${(meters / 1000).toFixed(1)}km`;
    }
    static formatPromotionType(type) {
        const map = {
            fixed_amount: '立减',
            percentage: '折扣',
            free_shipping: '免运费'
        };
        return map[type] || type;
    }
}
exports.DeliveryFeeAdapter = DeliveryFeeAdapter;
exports.deliveryFeeService = new DeliveryFeeService();
