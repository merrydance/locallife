"use strict";
/**
 * 商户订单管理接口
 * 基于swagger.json完全重构，仅保留后端支持的接口
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
exports.OrderManagementAdapter = exports.KitchenDisplayService = exports.MerchantOrderManagementService = void 0;
const request_1 = require("../utils/request");
// ==================== 商户订单管理服务 ====================
/**
 * 商户订单管理服务
 * 基于swagger.json完全重构，仅包含后端支持的接口
 */
class MerchantOrderManagementService {
    /**
     * 获取商户订单列表
     * GET /v1/merchant/orders
     */
    static getOrderList(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/merchant/orders',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取订单统计
     * GET /v1/merchant/orders/stats
     */
    static getOrderStats(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/merchant/orders/stats',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取订单详情
     * GET /v1/merchant/orders/{id}
     */
    static getOrderDetail(orderId) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/merchant/orders/${orderId}`,
                method: 'GET'
            });
        });
    }
    /**
     * 商户接单
     * POST /v1/merchant/orders/{id}/accept
     */
    static acceptOrder(orderId) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/merchant/orders/${orderId}/accept`,
                method: 'POST'
            });
        });
    }
    /**
     * 商户拒单
     * POST /v1/merchant/orders/{id}/reject
     */
    static rejectOrder(orderId, data) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/merchant/orders/${orderId}/reject`,
                method: 'POST',
                data
            });
        });
    }
    /**
     * 标记订单准备完成
     * POST /v1/merchant/orders/{id}/ready
     */
    static markOrderReady(orderId) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/merchant/orders/${orderId}/ready`,
                method: 'POST'
            });
        });
    }
    /**
     * 完成订单（堂食/打包自取）
     * POST /v1/merchant/orders/{id}/complete
     */
    static completeOrder(orderId) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/merchant/orders/${orderId}/complete`,
                method: 'POST'
            });
        });
    }
}
exports.MerchantOrderManagementService = MerchantOrderManagementService;
// ==================== KDS后厨管理服务 ====================
/**
 * KDS后厨管理服务
 * 基于swagger.json完全重构，仅包含后端支持的接口
 */
class KitchenDisplayService {
    /**
     * 获取厨房订单列表
     * GET /v1/kitchen/orders
     */
    static getKitchenOrders() {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/kitchen/orders',
                method: 'GET'
            });
        });
    }
    /**
     * 获取厨房订单详情
     * GET /v1/kitchen/orders/{id}
     */
    static getKitchenOrderDetail(orderId) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/kitchen/orders/${orderId}`,
                method: 'GET'
            });
        });
    }
    /**
     * 开始制作订单
     * POST /v1/kitchen/orders/{id}/preparing
     */
    static startPreparing(orderId) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/kitchen/orders/${orderId}/preparing`,
                method: 'POST'
            });
        });
    }
    /**
     * 标记订单制作完成
     * POST /v1/kitchen/orders/{id}/ready
     */
    static markKitchenOrderReady(orderId) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/kitchen/orders/${orderId}/ready`,
                method: 'POST'
            });
        });
    }
}
exports.KitchenDisplayService = KitchenDisplayService;
// ==================== 订单管理适配器 ====================
/**
 * 订单管理数据适配器
 * 处理前端展示数据和后端API数据之间的转换
 */
class OrderManagementAdapter {
    /**
     * 格式化订单状态显示文本
     */
    static formatOrderStatus(status) {
        const statusMap = {
            'pending': '待支付',
            'paid': '已支付',
            'preparing': '制作中',
            'ready': '待配送/待取餐',
            'delivering': '配送中',
            'completed': '已完成',
            'cancelled': '已取消'
        };
        return statusMap[status] || status;
    }
    /**
     * 格式化订单类型显示文本
     */
    static formatOrderType(orderType) {
        const typeMap = {
            'takeout': '外卖',
            'dine_in': '堂食',
            'takeaway': '打包自取',
            'reservation': '预定点菜'
        };
        return typeMap[orderType] || orderType;
    }
    /**
     * 格式化支付方式显示文本
     */
    static formatPaymentMethod(paymentMethod) {
        const methodMap = {
            'wechat': '微信支付',
            'balance': '余额支付'
        };
        return methodMap[paymentMethod] || paymentMethod;
    }
    /**
     * 计算订单实际支付金额
     */
    static calculateActualAmount(order) {
        return order.subtotal + order.delivery_fee - order.delivery_fee_discount - order.discount_amount;
    }
    /**
     * 格式化金额显示（分转元）
     */
    static formatAmount(amountInCents) {
        return (amountInCents / 100).toFixed(2);
    }
    /**
     * 格式化距离显示
     */
    static formatDistance(distanceInMeters) {
        if (!distanceInMeters)
            return '--';
        if (distanceInMeters < 1000) {
            return `${distanceInMeters}m`;
        }
        else {
            return `${(distanceInMeters / 1000).toFixed(1)}km`;
        }
    }
    /**
     * 判断订单是否可以接单
     */
    static canAcceptOrder(order) {
        return order.status === 'paid';
    }
    /**
     * 判断订单是否可以拒单
     */
    static canRejectOrder(order) {
        return ['paid', 'preparing'].includes(order.status);
    }
    /**
     * 判断订单是否可以标记为准备完成
     */
    static canMarkReady(order) {
        return order.status === 'preparing';
    }
    /**
     * 判断订单是否可以完成
     */
    static canCompleteOrder(order) {
        return order.status === 'ready' && ['dine_in', 'takeaway'].includes(order.order_type);
    }
    /**
     * 获取订单状态对应的颜色
     */
    static getStatusColor(status) {
        const colorMap = {
            'pending': '#f39c12', // 橙色
            'paid': '#3498db', // 蓝色
            'preparing': '#e74c3c', // 红色
            'ready': '#f39c12', // 橙色
            'delivering': '#9b59b6', // 紫色
            'completed': '#27ae60', // 绿色
            'cancelled': '#95a5a6' // 灰色
        };
        return colorMap[status] || '#95a5a6';
    }
    /**
     * 计算订单制作时长（分钟）
     */
    static calculatePreparationTime(order) {
        if (!order.preparing_started_at || !order.ready_at) {
            return null;
        }
        const startTime = new Date(order.preparing_started_at);
        const endTime = new Date(order.ready_at);
        return Math.round((endTime.getTime() - startTime.getTime()) / (1000 * 60));
    }
    /**
     * 判断订单是否超时
     */
    static isOrderOverdue(order) {
        if (!order.preparing_started_at || order.ready_at || !order.estimated_time) {
            return false;
        }
        const startTime = new Date(order.preparing_started_at);
        const now = new Date();
        const elapsedMinutes = (now.getTime() - startTime.getTime()) / (1000 * 60);
        return elapsedMinutes > order.estimated_time;
    }
    /**
     * 获取订单剩余制作时间（分钟）
     */
    static getRemainingTime(order) {
        if (!order.preparing_started_at || order.ready_at || !order.estimated_time) {
            return 0;
        }
        const startTime = new Date(order.preparing_started_at);
        const now = new Date();
        const elapsedMinutes = (now.getTime() - startTime.getTime()) / (1000 * 60);
        return Math.max(0, order.estimated_time - elapsedMinutes);
    }
}
exports.OrderManagementAdapter = OrderManagementAdapter;
// ==================== 导出默认服务 ====================
exports.default = MerchantOrderManagementService;
