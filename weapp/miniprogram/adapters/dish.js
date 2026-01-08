"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.DishAdapter = void 0;
const image_1 = require("../utils/image");
class DishAdapter {
    /**
     * 将菜品响应DTO转换为视图模型 - 基于swagger api.dishResponse
     */
    static toViewModel(dto) {
        var _a, _b, _c;
        return {
            id: dto.id,
            name: dto.name,
            imageUrl: (0, image_1.getPublicImageUrl)(dto.image_url),
            price: dto.price,
            priceDisplay: `¥${(dto.price / 100).toFixed(2)}`,
            shopName: '商户名称', // 需要从商户信息获取
            merchantId: dto.merchant_id,
            attributes: ((_a = dto.ingredients) === null || _a === void 0 ? void 0 : _a.map(ing => ing.name)) || [],
            spicyLevel: 0, // 从tags中解析辣度
            salesBadge: '', // 菜品详情中没有销量信息
            ratingDisplay: '0.0', // 菜品详情中没有评分信息
            distance: '距离未知',
            deliveryTimeDisplay: DishAdapter.formatDeliveryTime(dto.prepare_time),
            deliveryFeeDisplay: '配送费待定',
            discountRule: '',
            tags: ((_b = dto.tags) === null || _b === void 0 ? void 0 : _b.map(tag => tag.name)) || [],
            isPremade: ((_c = dto.tags) === null || _c === void 0 ? void 0 : _c.some(tag => tag.name.includes('预制'))) || false,
            customization_groups: dto.customization_groups,
            member_price: dto.member_price,
            is_available: dto.is_available,
            prepare_time: dto.prepare_time
        };
    }
    /**
     * 将菜品摘要DTO转换为视图模型 - 基于swagger api.dishSummary (用于Feed流)
     */
    static fromSummaryDTO(dto) {
        var _a;
        return {
            id: dto.id,
            name: dto.name,
            imageUrl: (0, image_1.getPublicImageUrl)(dto.image_url),
            price: dto.price,
            priceDisplay: `¥${(dto.price / 100).toFixed(2)}`,
            shopName: dto.merchant_name || '未知商家',
            merchantId: dto.merchant_id,
            attributes: [], // 摘要数据中没有配料信息
            spicyLevel: 0, // 从tags中解析辣度
            salesBadge: DishAdapter.formatSales(dto.monthly_sales || 0),
            ratingDisplay: '0.0', // 摘要数据中没有评分
            distance: DishAdapter.formatDistance(dto.distance || 0),
            deliveryTimeDisplay: DishAdapter.formatDeliveryTimeSeconds(dto.estimated_delivery_time || 0),
            deliveryFeeDisplay: DishAdapter.formatDeliveryFee(dto.estimated_delivery_fee || 0),
            discountRule: '',
            tags: dto.tags || [],
            isPremade: ((_a = dto.tags) === null || _a === void 0 ? void 0 : _a.includes('预制')) || false,
            distance_meters: dto.distance || 0,
            member_price: dto.member_price,
            is_available: dto.is_available
        };
    }
    static formatSales(sales) {
        if (sales >= 1000) {
            return `月销${(sales / 1000).toFixed(1)}k`;
        }
        return `月销${sales}`;
    }
    static formatDistance(meters) {
        if (!meters || meters === 0) {
            return '距离未知';
        }
        if (meters < 1000) {
            return `${Math.round(meters)}米`;
        }
        return `${(meters / 1000).toFixed(1)}公里`;
    }
    static formatDeliveryTime(minutes) {
        if (!minutes || minutes === 0) {
            return '时间待定';
        }
        return `${minutes}分钟`;
    }
    static formatDeliveryTimeSeconds(seconds) {
        if (!seconds || seconds === 0) {
            return '时间待定';
        }
        const minutes = Math.round(seconds / 60);
        if (minutes < 60) {
            return `约${minutes}分钟`;
        }
        const hours = Math.floor(minutes / 60);
        const remainingMinutes = minutes % 60;
        if (remainingMinutes === 0) {
            return `约${hours}小时`;
        }
        return `约${hours}小时${remainingMinutes}分`;
    }
    static formatDeliveryFee(fee) {
        if (fee === 0) {
            return '免代取费';
        }
        // 添加"起"表示这是起步价，实际费用可能因订单金额而更高
        return `代取${(fee / 100).toFixed(0)}元起`;
    }
    static formatDiscountRule(threshold, discountAmount) {
        if (threshold > 0) {
            const discount = discountAmount || Math.floor(threshold / 10 / 100);
            return `满${(threshold / 100).toFixed(0)}返${discount}元`;
        }
        return '';
    }
}
exports.DishAdapter = DishAdapter;
// 兼容性：保留旧方法名
DishAdapter.fromFeedDTO = DishAdapter.fromSummaryDTO;
