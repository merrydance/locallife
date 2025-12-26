"use strict";
/**
 * 配送任务管理接口重构 (Task 3.2)
 * 基于swagger.json完全重构，移除所有没有后端支持的旧功能
 * 包含：任务获取、配送流程、任务历史
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
exports.DeliveryReminderManager = exports.deliveryProcessService = exports.deliveryTaskManagementService = exports.DeliveryTaskManagementAdapter = exports.DeliveryProcessService = exports.DeliveryTaskManagementService = void 0;
exports.getRiderDeliveryDashboard = getRiderDeliveryDashboard;
exports.getSmartOrderRecommendations = getSmartOrderRecommendations;
exports.calculateDeliveryEfficiency = calculateDeliveryEfficiency;
exports.formatDeliveryStatus = formatDeliveryStatus;
exports.formatDistance = formatDistance;
exports.formatDeliveryFee = formatDeliveryFee;
exports.calculateEstimatedArrival = calculateEstimatedArrival;
exports.validateDeliveryAction = validateDeliveryAction;
const request_1 = require("../utils/request");
// ==================== 配送任务管理服务类 ====================
/**
 * 配送任务管理服务
 * 提供任务获取、配送流程管理等功能
 */
class DeliveryTaskManagementService {
    /**
     * 获取推荐订单列表
     * @param params 查询参数
     */
    getRecommendedOrders(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/delivery/recommend',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 抢单
     * @param orderId 订单ID
     */
    grabOrder(orderId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/delivery/grab/${orderId}`,
                method: 'POST'
            });
        });
    }
    /**
     * 获取当前活跃配送任务
     */
    getActiveDeliveries() {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/delivery/active',
                method: 'GET'
            });
        });
    }
    /**
     * 获取配送历史
     * @param params 查询参数
     */
    getDeliveryHistory(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/delivery/history',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取配送任务详情
     * @param deliveryId 配送任务ID
     */
    getDeliveryDetail(deliveryId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/delivery/${deliveryId}`,
                method: 'GET'
            });
        });
    }
    /**
     * 开始取餐
     * @param deliveryId 配送任务ID
     * @param actionData 操作数据
     */
    startPickup(deliveryId, actionData) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/delivery/${deliveryId}/start-pickup`,
                method: 'POST',
                data: actionData || {}
            });
        });
    }
    /**
     * 确认取餐完成
     * @param deliveryId 配送任务ID
     * @param actionData 操作数据
     */
    confirmPickup(deliveryId, actionData) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/delivery/${deliveryId}/confirm-pickup`,
                method: 'POST',
                data: actionData || {}
            });
        });
    }
    /**
     * 开始配送
     * @param deliveryId 配送任务ID
     * @param actionData 操作数据
     */
    startDelivery(deliveryId, actionData) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/delivery/${deliveryId}/start-delivery`,
                method: 'POST',
                data: actionData || {}
            });
        });
    }
    /**
     * 确认送达
     * @param deliveryId 配送任务ID
     * @param actionData 操作数据
     */
    confirmDelivery(deliveryId, actionData) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/delivery/${deliveryId}/confirm-delivery`,
                method: 'POST',
                data: actionData || {}
            });
        });
    }
}
exports.DeliveryTaskManagementService = DeliveryTaskManagementService;
// ==================== 配送流程管理服务类 ====================
/**
 * 配送流程管理服务
 * 提供配送流程的状态管理和操作指导
 */
class DeliveryProcessService {
    /**
     * 获取配送流程的下一步操作
     * @param delivery 配送任务信息
     */
    getNextAction(delivery) {
        switch (delivery.status) {
            case 'assigned':
                return {
                    action: 'start_pickup',
                    actionText: '开始取餐',
                    canExecute: true
                };
            case 'picked_up':
                return {
                    action: 'confirm_pickup',
                    actionText: '确认取餐',
                    canExecute: true
                };
            case 'delivering':
                return {
                    action: 'start_delivery',
                    actionText: '开始配送',
                    canExecute: true
                };
            case 'delivered':
                return {
                    action: 'confirm_delivery',
                    actionText: '确认送达',
                    canExecute: true
                };
            case 'completed':
                return {
                    action: 'none',
                    actionText: '已完成',
                    canExecute: false,
                    reason: '配送任务已完成'
                };
            case 'cancelled':
                return {
                    action: 'none',
                    actionText: '已取消',
                    canExecute: false,
                    reason: '配送任务已取消'
                };
            default:
                return {
                    action: 'unknown',
                    actionText: '未知状态',
                    canExecute: false,
                    reason: '未知的配送状态'
                };
        }
    }
    /**
     * 执行配送流程操作
     * @param delivery 配送任务信息
     * @param actionData 操作数据
     */
    executeDeliveryAction(delivery, actionData) {
        return __awaiter(this, void 0, void 0, function* () {
            const nextAction = this.getNextAction(delivery);
            if (!nextAction.canExecute) {
                return {
                    success: false,
                    message: nextAction.reason || '无法执行操作'
                };
            }
            try {
                const service = new DeliveryTaskManagementService();
                let updatedDelivery;
                switch (nextAction.action) {
                    case 'start_pickup':
                        updatedDelivery = yield service.startPickup(delivery.id, actionData);
                        break;
                    case 'confirm_pickup':
                        updatedDelivery = yield service.confirmPickup(delivery.id, actionData);
                        break;
                    case 'start_delivery':
                        updatedDelivery = yield service.startDelivery(delivery.id, actionData);
                        break;
                    case 'confirm_delivery':
                        updatedDelivery = yield service.confirmDelivery(delivery.id, actionData);
                        break;
                    default:
                        return {
                            success: false,
                            message: '未知的操作类型'
                        };
                }
                return {
                    success: true,
                    message: `${nextAction.actionText}成功`,
                    updatedDelivery
                };
            }
            catch (error) {
                return {
                    success: false,
                    message: (error === null || error === void 0 ? void 0 : error.message) || `${nextAction.actionText}失败`
                };
            }
        });
    }
}
exports.DeliveryProcessService = DeliveryProcessService;
// ==================== 数据适配器 ====================
/**
 * 配送任务管理数据适配器
 * 处理前端数据格式与后端API数据格式的转换
 */
class DeliveryTaskManagementAdapter {
    /**
     * 适配推荐订单响应数据
     */
    static adaptRecommendedOrderResponse(data) {
        return {
            orderId: data.order_id,
            merchantId: data.merchant_id,
            deliveryFee: data.delivery_fee,
            pickupLatitude: data.pickup_latitude,
            pickupLongitude: data.pickup_longitude,
            deliveryLatitude: data.delivery_latitude,
            deliveryLongitude: data.delivery_longitude,
            distance: data.distance,
            distanceToPickup: data.distance_to_pickup,
            realDistance: data.real_distance,
            realDuration: data.real_duration,
            estimatedMinutes: data.estimated_minutes,
            expiresAt: data.expires_at,
            totalScore: data.total_score,
            distanceScore: data.distance_score,
            profitScore: data.profit_score,
            routeScore: data.route_score,
            urgencyScore: data.urgency_score
        };
    }
    /**
     * 适配配送响应数据
     */
    static adaptDeliveryResponse(data) {
        return {
            id: data.id,
            orderId: data.order_id,
            riderId: data.rider_id,
            status: data.status,
            deliveryFee: data.delivery_fee,
            riderEarnings: data.rider_earnings,
            distance: data.distance,
            pickupAddress: data.pickup_address,
            pickupContact: data.pickup_contact,
            pickupPhone: data.pickup_phone,
            pickupLatitude: data.pickup_latitude,
            pickupLongitude: data.pickup_longitude,
            deliveryAddress: data.delivery_address,
            deliveryContact: data.delivery_contact,
            deliveryPhone: data.delivery_phone,
            deliveryLatitude: data.delivery_latitude,
            deliveryLongitude: data.delivery_longitude,
            estimatedPickupAt: data.estimated_pickup_at,
            estimatedDeliveryAt: data.estimated_delivery_at,
            createdAt: data.created_at,
            assignedAt: data.assigned_at,
            pickedAt: data.picked_at,
            deliveredAt: data.delivered_at,
            completedAt: data.completed_at
        };
    }
}
exports.DeliveryTaskManagementAdapter = DeliveryTaskManagementAdapter;
// ==================== 导出服务实例 ====================
exports.deliveryTaskManagementService = new DeliveryTaskManagementService();
exports.deliveryProcessService = new DeliveryProcessService();
// ==================== 便捷函数 ====================
/**
 * 获取骑手配送工作台数据
 * @param latitude 当前纬度
 * @param longitude 当前经度
 */
function getRiderDeliveryDashboard(latitude, longitude) {
    return __awaiter(this, void 0, void 0, function* () {
        const [recommendedOrders, activeDeliveries] = yield Promise.all([
            exports.deliveryTaskManagementService.getRecommendedOrders({ latitude, longitude }),
            exports.deliveryTaskManagementService.getActiveDeliveries()
        ]);
        // 今日统计数据需要根据实际接口调整
        const todayStats = {
            completedDeliveries: 0,
            totalEarnings: 0,
            totalDistance: 0,
            avgDeliveryTime: 0
        };
        return {
            recommendedOrders,
            activeDeliveries,
            todayStats
        };
    });
}
/**
 * 智能抢单推荐
 * @param orders 推荐订单列表
 * @param preferences 骑手偏好设置
 */
function getSmartOrderRecommendations(orders, preferences = {}) {
    const highPriority = [];
    const recommended = [];
    const others = [];
    orders.forEach(order => {
        // 高优先级：高分且符合偏好
        if (order.total_score >= 80 &&
            (!preferences.maxDistance || order.distance_to_pickup <= preferences.maxDistance) &&
            (!preferences.minDeliveryFee || order.delivery_fee >= preferences.minDeliveryFee)) {
            highPriority.push(order);
        }
        // 推荐：中等分数
        else if (order.total_score >= 60) {
            recommended.push(order);
        }
        // 其他
        else {
            others.push(order);
        }
    });
    return { highPriority, recommended, others };
}
/**
 * 计算配送效率指标
 * @param deliveries 配送记录列表
 */
function calculateDeliveryEfficiency(deliveries) {
    const completedDeliveries = deliveries.filter(d => d.status === 'completed');
    if (completedDeliveries.length === 0) {
        return {
            avgDeliveryTime: 0,
            onTimeRate: 0,
            totalDistance: 0,
            totalEarnings: 0,
            avgEarningsPerKm: 0,
            completionRate: 0
        };
    }
    // 计算平均配送时间
    const totalDeliveryTime = completedDeliveries.reduce((sum, delivery) => {
        if (delivery.assigned_at && delivery.completed_at) {
            const assignedTime = new Date(delivery.assigned_at).getTime();
            const completedTime = new Date(delivery.completed_at).getTime();
            return sum + (completedTime - assignedTime);
        }
        return sum;
    }, 0);
    const avgDeliveryTime = totalDeliveryTime / completedDeliveries.length / (1000 * 60); // 转换为分钟
    // 计算准时率（这里需要根据实际业务逻辑调整）
    const onTimeDeliveries = completedDeliveries.filter(delivery => {
        if (delivery.estimated_delivery_at && delivery.delivered_at) {
            const estimatedTime = new Date(delivery.estimated_delivery_at).getTime();
            const actualTime = new Date(delivery.delivered_at).getTime();
            return actualTime <= estimatedTime;
        }
        return false;
    });
    const onTimeRate = (onTimeDeliveries.length / completedDeliveries.length) * 100;
    // 计算总距离和收入
    const totalDistance = completedDeliveries.reduce((sum, d) => sum + d.distance, 0);
    const totalEarnings = completedDeliveries.reduce((sum, d) => sum + d.rider_earnings, 0);
    const avgEarningsPerKm = totalDistance > 0 ? totalEarnings / (totalDistance / 1000) : 0;
    // 完成率
    const completionRate = (completedDeliveries.length / deliveries.length) * 100;
    return {
        avgDeliveryTime,
        onTimeRate,
        totalDistance,
        totalEarnings,
        avgEarningsPerKm,
        completionRate
    };
}
/**
 * 配送任务提醒管理器
 */
class DeliveryReminderManager {
    constructor() {
        this.reminders = new Map();
    }
    /**
     * 设置配送提醒
     * @param delivery 配送任务
     * @param callback 提醒回调
     */
    setDeliveryReminder(delivery, callback) {
        // 清除已有提醒
        this.clearReminder(delivery.id);
        const now = new Date().getTime();
        // 取餐提醒
        if (delivery.estimated_pickup_at && delivery.status === 'assigned') {
            const pickupTime = new Date(delivery.estimated_pickup_at).getTime();
            const reminderTime = pickupTime - 5 * 60 * 1000; // 提前5分钟提醒
            if (reminderTime > now) {
                const timeoutId = setTimeout(() => {
                    callback(delivery, 'pickup_reminder');
                }, reminderTime - now);
                this.reminders.set(delivery.id, timeoutId);
            }
        }
        // 送达提醒
        if (delivery.estimated_delivery_at && delivery.status === 'delivering') {
            const deliveryTime = new Date(delivery.estimated_delivery_at).getTime();
            const reminderTime = deliveryTime - 5 * 60 * 1000; // 提前5分钟提醒
            if (reminderTime > now) {
                const timeoutId = setTimeout(() => {
                    callback(delivery, 'delivery_reminder');
                }, reminderTime - now);
                this.reminders.set(delivery.id, timeoutId);
            }
        }
    }
    /**
     * 清除提醒
     * @param deliveryId 配送任务ID
     */
    clearReminder(deliveryId) {
        const timeoutId = this.reminders.get(deliveryId);
        if (timeoutId) {
            clearTimeout(timeoutId);
            this.reminders.delete(deliveryId);
        }
    }
    /**
     * 清除所有提醒
     */
    clearAllReminders() {
        this.reminders.forEach(timeoutId => clearTimeout(timeoutId));
        this.reminders.clear();
    }
}
exports.DeliveryReminderManager = DeliveryReminderManager;
/**
 * 格式化配送状态显示
 * @param status 配送状态
 */
function formatDeliveryStatus(status) {
    const statusMap = {
        assigned: '已分配',
        picked_up: '已取餐',
        delivering: '配送中',
        delivered: '已送达',
        completed: '已完成',
        cancelled: '已取消'
    };
    return statusMap[status] || status;
}
/**
 * 格式化距离显示
 * @param distance 距离（米）
 */
function formatDistance(distance) {
    if (distance < 1000) {
        return `${distance}m`;
    }
    else {
        return `${(distance / 1000).toFixed(1)}km`;
    }
}
/**
 * 格式化配送费显示
 * @param fee 配送费（分）
 * @param showUnit 是否显示单位
 */
function formatDeliveryFee(fee, showUnit = true) {
    const yuan = (fee / 100).toFixed(2);
    return showUnit ? `¥${yuan}` : yuan;
}
/**
 * 计算预估到达时间
 * @param startTime 开始时间
 * @param estimatedMinutes 预估分钟数
 */
function calculateEstimatedArrival(startTime, estimatedMinutes) {
    const start = new Date(startTime);
    const arrival = new Date(start.getTime() + estimatedMinutes * 60 * 1000);
    return arrival.toISOString();
}
/**
 * 验证配送操作数据
 * @param actionData 操作数据
 */
function validateDeliveryAction(actionData) {
    if (actionData.latitude && (actionData.latitude < -90 || actionData.latitude > 90)) {
        return { valid: false, message: '纬度范围应在-90到90之间' };
    }
    if (actionData.longitude && (actionData.longitude < -180 || actionData.longitude > 180)) {
        return { valid: false, message: '经度范围应在-180到180之间' };
    }
    if (actionData.notes && actionData.notes.length > 500) {
        return { valid: false, message: '备注不能超过500个字符' };
    }
    if (actionData.photos && actionData.photos.length > 9) {
        return { valid: false, message: '照片数量不能超过9张' };
    }
    return { valid: true };
}
