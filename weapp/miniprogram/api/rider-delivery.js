"use strict";
/**
 * 骑手配送管理接口
 * 基于swagger.json完全重构，包含配送任务、异常处理、财务管理等
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
exports.DeliveryAdapter = exports.RiderFinanceService = exports.ExceptionHandlingService = exports.RiderInfoService = exports.DeliveryTaskService = void 0;
const request_1 = require("../utils/request");
// ==================== 配送任务管理服务 ====================
/**
 * 配送任务管理服务
 */
class DeliveryTaskService {
    /**
     * 获取推荐配送任务
     * GET /v1/delivery/recommend
     */
    static getRecommendedTasks() {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/delivery/recommend',
                method: 'GET'
            });
        });
    }
    /**
     * 抢单
     * POST /v1/delivery/grab/:order_id
     */
    static grabOrder(orderId) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/delivery/grab/${orderId}`,
                method: 'POST'
            });
        });
    }
    /**
     * 获取当前配送任务
     * GET /v1/delivery/active
     */
    static getActiveTasks() {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/delivery/active',
                method: 'GET'
            });
        });
    }
    /**
     * 获取配送历史
     * GET /v1/delivery/history
     */
    static getDeliveryHistory(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/delivery/history',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取订单详情
     * GET /v1/delivery/order/:order_id
     */
    static getOrderDetail(orderId) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/delivery/order/${orderId}`,
                method: 'GET'
            });
        });
    }
    /**
     * 开始取餐
     * POST /v1/delivery/:delivery_id/start-pickup
     */
    static startPickup(deliveryId) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/delivery/${deliveryId}/start-pickup`,
                method: 'POST'
            });
        });
    }
    /**
     * 确认取餐
     * POST /v1/delivery/:delivery_id/confirm-pickup
     */
    static confirmPickup(deliveryId) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/delivery/${deliveryId}/confirm-pickup`,
                method: 'POST'
            });
        });
    }
    /**
     * 开始配送
     * POST /v1/delivery/:delivery_id/start-delivery
     */
    static startDelivery(deliveryId) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/delivery/${deliveryId}/start-delivery`,
                method: 'POST'
            });
        });
    }
    /**
     * 确认送达
     * POST /v1/delivery/:delivery_id/confirm-delivery
     */
    static confirmDelivery(deliveryId) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/delivery/${deliveryId}/confirm-delivery`,
                method: 'POST'
            });
        });
    }
    /**
     * 获取骑手位置
     * GET /v1/delivery/:delivery_id/rider-location
     */
    static getRiderLocation(deliveryId) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/v1/delivery/${deliveryId}/rider-location`,
                method: 'GET'
            });
        });
    }
}
exports.DeliveryTaskService = DeliveryTaskService;
// ==================== 骑手信息管理服务 ====================
/**
 * 骑手信息管理服务
 */
class RiderInfoService {
    /**
     * 获取骑手信息
     * GET /v1/rider/me
     */
    static getRiderInfo() {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/rider/me',
                method: 'GET'
            });
        });
    }
    /**
     * 获取骑手状态
     * GET /v1/rider/status
     */
    static getRiderStatus() {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/rider/status',
                method: 'GET'
            });
        });
    }
    /**
     * 上线
     * POST /v1/rider/online
     */
    static goOnline() {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/rider/online',
                method: 'POST'
            });
        });
    }
    /**
     * 下线
     * POST /v1/rider/offline
     */
    static goOffline() {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/rider/offline',
                method: 'POST'
            });
        });
    }
    /**
     * 上报位置
     * POST /v1/rider/location
     */
    static reportLocation(data) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/rider/location',
                method: 'POST',
                data
            });
        });
    }
    /**
     * 获取信用分
     * GET /v1/rider/score
     */
    static getScore() {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/rider/score',
                method: 'GET'
            });
        });
    }
    /**
     * 获取信用分历史
     * GET /v1/rider/score/history
     */
    static getScoreHistory(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/rider/score/history',
                method: 'GET',
                data: params
            });
        });
    }
}
exports.RiderInfoService = RiderInfoService;
// ==================== 异常处理服务 ====================
/**
 * 异常处理服务
 */
class ExceptionHandlingService {
    /**
     * 上报异常
     * POST /rider/orders/{id}/exception
     */
    static reportException(orderId, data) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/rider/orders/${orderId}/exception`,
                method: 'POST',
                data
            });
        });
    }
    /**
     * 上报延迟
     * POST /rider/orders/{id}/delay
     */
    static reportDelay(orderId, data) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: `/rider/orders/${orderId}/delay`,
                method: 'POST',
                data
            });
        });
    }
}
exports.ExceptionHandlingService = ExceptionHandlingService;
// ==================== 财务管理服务 ====================
/**
 * 财务管理服务
 */
class RiderFinanceService {
    /**
     * 获取保证金信息
     * GET /v1/rider/deposit
     */
    static getDeposit() {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/rider/deposit',
                method: 'GET'
            });
        });
    }
    /**
     * 获取保证金记录
     * GET /v1/rider/deposits
     */
    static getDepositRecords(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/rider/deposits',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 提现
     * POST /v1/rider/withdraw
     */
    static withdraw(data) {
        return __awaiter(this, void 0, void 0, function* () {
            return yield (0, request_1.request)({
                url: '/v1/rider/withdraw',
                method: 'POST',
                data
            });
        });
    }
}
exports.RiderFinanceService = RiderFinanceService;
// ==================== 配送管理适配器 ====================
/**
 * 配送管理数据适配器
 */
class DeliveryAdapter {
    /**
     * 格式化金额显示（分转元）
     */
    static formatAmount(amountInCents) {
        return (amountInCents / 100).toFixed(2);
    }
    /**
     * 格式化距离显示（统一中文格式：米/公里）
     */
    static formatDistance(distanceInMeters) {
        if (distanceInMeters < 1000) {
            return `${Math.round(distanceInMeters)}米`;
        }
        else {
            return `${(distanceInMeters / 1000).toFixed(1)}公里`;
        }
    }
    /**
     * 格式化配送状态
     */
    static formatDeliveryStatus(status) {
        const statusMap = {
            'pending': '待接单',
            'accepted': '已接单',
            'picking_up': '取餐中',
            'picked_up': '已取餐',
            'delivering': '配送中',
            'delivered': '已送达',
            'cancelled': '已取消'
        };
        return statusMap[status] || status;
    }
    /**
     * 获取配送状态颜色
     */
    static getStatusColor(status) {
        const colorMap = {
            'pending': '#f39c12',
            'accepted': '#3498db',
            'picking_up': '#e74c3c',
            'picked_up': '#9b59b6',
            'delivering': '#e67e22',
            'delivered': '#27ae60',
            'cancelled': '#95a5a6'
        };
        return colorMap[status] || '#95a5a6';
    }
    /**
     * 格式化骑手状态
     */
    static formatRiderStatus(status) {
        const statusMap = {
            'online': '在线',
            'offline': '离线',
            'busy': '忙碌中'
        };
        return statusMap[status] || status;
    }
    /**
     * 获取骑手状态颜色
     */
    static getRiderStatusColor(status) {
        const colorMap = {
            'online': '#52c41a',
            'offline': '#999',
            'busy': '#fa8c16'
        };
        return colorMap[status] || '#999';
    }
    /**
     * 计算预计送达时间
     */
    static calculateEstimatedArrival(createdAt, estimatedTime) {
        const created = new Date(createdAt);
        const arrival = new Date(created.getTime() + estimatedTime * 60 * 1000);
        const hours = ('0' + arrival.getHours()).slice(-2);
        const minutes = ('0' + arrival.getMinutes()).slice(-2);
        return `${hours}:${minutes}`;
    }
}
exports.DeliveryAdapter = DeliveryAdapter;
// ==================== 导出默认服务 ====================
exports.default = {
    DeliveryTaskService,
    RiderInfoService,
    ExceptionHandlingService,
    RiderFinanceService,
    DeliveryAdapter
};
