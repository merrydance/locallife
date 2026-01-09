"use strict";
/**
 * WebSocket和实时通信接口模块
 * 基于swagger.json完全重构，提供WebSocket连接和实时位置追踪
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
exports.RealtimeDataAdapter = exports.RealtimeUtils = exports.WebSocketUtils = exports.RiderLocationManager = exports.DeliveryTrackingManager = exports.WebSocketManager = void 0;
const request_1 = require("../utils/request");
const auth_1 = require("../utils/auth");
// ==================== WebSocket管理器 ====================
/**
 * WebSocket连接管理器
 */
class WebSocketManager {
    constructor() {
        this.socketTask = null;
        this.connectionParams = null;
        this.eventHandlers = {};
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 5;
        this.reconnectInterval = 3000;
        this.heartbeatInterval = null;
        this.isManualClose = false;
        this.isConnecting = false;
        this.stableConnectionTimer = null;
    }
    /**
     * 连接WebSocket
     * 使用 wx.connectSocket 建立 WebSocket 连接
     */
    connect(params_1) {
        return __awaiter(this, arguments, void 0, function* (params, handlers = {}) {
            var _a, _b;
            // 如果正在连接中，不重复触发
            if (this.isConnecting) {
                console.log('WebSocket 正在连接中，忽略重复请求');
                return;
            }
            this.isConnecting = true;
            this.connectionParams = params;
            this.eventHandlers = handlers;
            this.isManualClose = false; // 每次发起新连接时重置手动关闭状态
            // 清理先前的连接
            if (this.socketTask) {
                console.log('正在清理先前的 WebSocket 任务...');
                this.stopHeartbeat();
                if (this.stableConnectionTimer) {
                    clearTimeout(this.stableConnectionTimer);
                    this.stableConnectionTimer = null;
                }
                yield this.closeCurrentTask('Re-connecting');
            }
            try {
                // 获取认证 token
                const token = (0, auth_1.getToken)();
                if (!token) {
                    this.isConnecting = false;
                    throw new Error('未登录，无法建立 WebSocket 连接');
                }
                // 构建 WebSocket URL - 仅使用 token
                const wsUrl = this.buildWebSocketUrl();
                console.log('正在开启 WebSocket 物理连接:', wsUrl.split('?')[0]);
                // 使用微信小程序 WebSocket API
                this.socketTask = wx.connectSocket({
                    url: wsUrl,
                    // 注意：移除 header 中的 Authorization，完全依赖 URL query
                    // 因为小程序 connectSocket 对 header 的支持在不同平台表现不一
                    success: () => {
                        console.log('WebSocket 连接请求已成功发送:', wsUrl.split('?')[0]); // 只记录不带 token 的 URL
                    },
                    fail: (error) => {
                        var _a, _b;
                        console.error('WebSocket 连接请求失败:', error);
                        this.isConnecting = false; // 连接请求失败，重置连接状态
                        (_b = (_a = this.eventHandlers).onError) === null || _b === void 0 ? void 0 : _b.call(_a, new Error(error.errMsg));
                    }
                });
                this.setupEventListeners();
            }
            catch (error) {
                this.isConnecting = false; // 捕获到错误，重置连接状态
                console.error('WebSocket 系统错误:', error);
                (_b = (_a = this.eventHandlers).onError) === null || _b === void 0 ? void 0 : _b.call(_a, error);
                throw error;
            }
        });
    }
    /**
     * 构建 WebSocket URL
     * 注意：后端文档明确说明仅需要 token 这一项 Query Parameter
     * 角色、用户ID 和 频道订阅由后端从 Token 中自动解析
     */
    buildWebSocketUrl() {
        var _a;
        // 从 API_BASE 构建 WSS URL
        // API_BASE 格式: https://xxx.com -> wss://xxx.com/v1/ws
        const { API_CONFIG } = require('../config/index');
        const baseUrl = API_CONFIG.BASE_URL || 'https://llapi.merrydance.cn';
        const wsBase = baseUrl.replace('https://', 'wss://').replace('http://', 'ws://');
        // 获取 token 用于认证
        const token = (_a = (0, auth_1.getToken)()) === null || _a === void 0 ? void 0 : _a.trim();
        // 构建查询参数 - 仅保留 token
        const queryParts = [];
        if (token) {
            queryParts.push(`token=${encodeURIComponent(token)}`);
        }
        const queryString = queryParts.length > 0 ? '?' + queryParts.join('&') : '';
        return `${wsBase}/v1/ws${queryString}`;
    }
    /**
     * 设置事件监听器
     * 使用微信小程序 SocketTask 的事件监听方式
     */
    setupEventListeners() {
        if (!this.socketTask)
            return;
        this.socketTask.onOpen(() => {
            var _a, _b;
            console.log('WebSocket连接已物理建立');
            this.isConnecting = false; // 连接成功建立，重置连接状态
            this.startHeartbeat();
            (_b = (_a = this.eventHandlers).onOpen) === null || _b === void 0 ? void 0 : _b.call(_a);
            // 稳定性判定：如果在 5 秒内没有断开，则认为连接已稳定，重置重连计数
            if (this.stableConnectionTimer)
                clearTimeout(this.stableConnectionTimer);
            this.stableConnectionTimer = setTimeout(() => {
                this.reconnectAttempts = 0;
                console.log('WebSocket 连接稳定已达 5s，重置重连尝试次数');
                this.stableConnectionTimer = null;
            }, 5000);
        });
        this.socketTask.onClose((res) => {
            var _a, _b;
            console.log('WebSocket 连接关闭:', res.code, res.reason);
            this.isConnecting = false;
            this.stopHeartbeat();
            (_b = (_a = this.eventHandlers).onClose) === null || _b === void 0 ? void 0 : _b.call(_a, res.code, res.reason);
            if (!this.isManualClose) {
                this.attemptReconnect();
            }
        });
        this.socketTask.onError((error) => {
            var _a, _b;
            console.error('WebSocket错误:', error);
            (_b = (_a = this.eventHandlers).onError) === null || _b === void 0 ? void 0 : _b.call(_a, new Error(error.errMsg));
        });
        this.socketTask.onMessage((res) => {
            try {
                const message = JSON.parse(res.data);
                this.handleMessage(message);
            }
            catch (error) {
                console.error('解析WebSocket消息失败:', error);
            }
        });
    }
    /**
     * 处理接收到的消息
     */
    handleMessage(message) {
        var _a, _b, _c, _d, _e, _f, _g, _h, _j, _k;
        (_b = (_a = this.eventHandlers).onMessage) === null || _b === void 0 ? void 0 : _b.call(_a, message);
        // 1. 自动响应应用层心跳 (Ping -> Pong)
        if (message.type === 'ping') {
            this.send('pong', { timestamp: new Date().toISOString() });
            return;
        }
        // 2. 自动确认重要通知 (ACK)
        if (message.message_id) {
            this.send('ack', { message_id: message.message_id });
        }
        // 3. 根据消息类型分发到对应的处理器
        switch (message.type) {
            case 'order_update':
                (_d = (_c = this.eventHandlers).onOrderUpdate) === null || _d === void 0 ? void 0 : _d.call(_c, message.data);
                break;
            case 'delivery_update':
                (_f = (_e = this.eventHandlers).onDeliveryUpdate) === null || _f === void 0 ? void 0 : _f.call(_e, message.data);
                break;
            case 'location_update':
                (_h = (_g = this.eventHandlers).onLocationUpdate) === null || _h === void 0 ? void 0 : _h.call(_g, message.data);
                break;
            case 'notification':
                (_k = (_j = this.eventHandlers).onNotification) === null || _k === void 0 ? void 0 : _k.call(_j, message.data);
                break;
            case 'pong':
                // 服务器对我们 Ping 的响应，或者由服务器主动发起的应用层 Pong
                console.log('收到 WebSocket Pong');
                break;
            default:
                console.log('收到业务消息:', message.type);
        }
    }
    /**
     * 发送消息
     */
    send(type, data, channel) {
        if (!this.socketTask) {
            console.warn('WebSocket未连接，无法发送消息');
            return;
        }
        const message = {
            type,
            channel: channel || 'default',
            data,
            timestamp: new Date().toISOString(),
            message_id: this.generateMessageId()
        };
        this.socketTask.send({
            data: JSON.stringify(message),
            fail: (err) => {
                console.error('WebSocket发送失败:', err);
            }
        });
    }
    /**
     * 订阅频道
     */
    subscribe(channels) {
        this.send('subscribe', { channels });
    }
    /**
     * 取消订阅频道
     */
    unsubscribe(channels) {
        this.send('unsubscribe', { channels });
    }
    /**
     * 开始心跳
     * 后端 60 秒超时，每 45 秒发送一次心跳以保持连接
     */
    startHeartbeat() {
        if (this.heartbeatInterval)
            clearInterval(this.heartbeatInterval);
        this.heartbeatInterval = setInterval(() => {
            if (this.isConnected()) {
                this.send('ping', { timestamp: new Date().toISOString() });
            }
        }, 45000);
    }
    /**
     * 停止心跳
     */
    stopHeartbeat() {
        if (this.heartbeatInterval) {
            clearInterval(this.heartbeatInterval);
            this.heartbeatInterval = null;
        }
    }
    /**
     * 尝试重连
     */
    attemptReconnect() {
        if (this.reconnectAttempts >= this.maxReconnectAttempts) {
            console.error('WebSocket重连次数已达上限');
            return;
        }
        this.reconnectAttempts++;
        console.log(`WebSocket重连尝试 ${this.reconnectAttempts}/${this.maxReconnectAttempts}`);
        setTimeout(() => {
            if (this.connectionParams) {
                this.connect(this.connectionParams, this.eventHandlers);
            }
        }, this.reconnectInterval * this.reconnectAttempts);
    }
    /**
     * 安全关闭当前任务并等待释放
     */
    closeCurrentTask() {
        return __awaiter(this, arguments, void 0, function* (reason = 'Close') {
            if (!this.socketTask)
                return;
            yield new Promise((resolve) => {
                if (!this.socketTask)
                    return resolve();
                const task = this.socketTask;
                let resolved = false;
                const finish = () => {
                    if (resolved)
                        return;
                    resolved = true;
                    resolve();
                };
                task.onClose(finish);
                task.onError(finish);
                try {
                    task.close({ code: 1000, reason });
                }
                catch (e) {
                    finish();
                }
                // 强制超时释放
                setTimeout(finish, 1000);
            });
            this.socketTask = null;
        });
    }
    /**
     * 关闭连接
     */
    close() {
        return __awaiter(this, void 0, void 0, function* () {
            this.isManualClose = true;
            this.stopHeartbeat();
            yield this.closeCurrentTask('Manual close');
        });
    }
    /**
     * 获取连接状态
     * 注意：微信小程序 SocketTask 没有 readyState，返回简化状态
     */
    getReadyState() {
        return this.socketTask ? 1 : 3; // 1=OPEN, 3=CLOSED
    }
    /**
     * 是否已连接 (处于物理已开启或正在开启状态)
     */
    isConnected() {
        // 如果 socketTask 不存在，肯定没连接
        if (!this.socketTask)
            return false;
        // 如果进入了手动关闭流程，认为已断开
        if (this.isManualClose)
            return false;
        return true;
    }
    /**
     * 生成消息ID
     */
    generateMessageId() {
        return `${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
    }
}
exports.WebSocketManager = WebSocketManager;
// ==================== 实时位置追踪 ====================
/**
 * 配送实时追踪管理器
 */
class DeliveryTrackingManager {
    constructor() {
        this.trackingMap = new Map();
        this.wsManager = null;
    }
    /**
     * 设置WebSocket管理器
     */
    setWebSocketManager(wsManager) {
        this.wsManager = wsManager;
    }
    /**
     * 获取配送追踪信息
     */
    getDeliveryTracking(deliveryId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/delivery/${deliveryId}/track`,
                method: 'GET'
            });
        });
    }
    /**
     * 开始追踪配送
     */
    startTracking(deliveryId_1, onLocationUpdate_1) {
        return __awaiter(this, arguments, void 0, function* (deliveryId, onLocationUpdate, interval = 5000) {
            // 停止之前的追踪
            this.stopTracking(deliveryId);
            // 立即获取一次位置信息
            try {
                const trackingInfo = yield this.getDeliveryTracking(deliveryId);
                onLocationUpdate(trackingInfo);
            }
            catch (error) {
                console.error('获取配送追踪信息失败:', error);
            }
            // 设置定时轮询
            const timeoutId = setInterval(() => __awaiter(this, void 0, void 0, function* () {
                try {
                    const trackingInfo = yield this.getDeliveryTracking(deliveryId);
                    onLocationUpdate(trackingInfo);
                    // 如果配送已完成，停止追踪
                    if (trackingInfo.status === 'delivered') {
                        this.stopTracking(deliveryId);
                    }
                }
                catch (error) {
                    console.error('轮询配送追踪信息失败:', error);
                }
            }), interval);
            this.trackingMap.set(deliveryId, timeoutId);
            // 如果有WebSocket连接，订阅实时位置更新
            if (this.wsManager && this.wsManager.isConnected()) {
                this.wsManager.subscribe([`delivery_${deliveryId}`]);
            }
        });
    }
    /**
     * 停止追踪配送
     */
    stopTracking(deliveryId) {
        const timeoutId = this.trackingMap.get(deliveryId);
        if (timeoutId) {
            clearInterval(timeoutId);
            this.trackingMap.delete(deliveryId);
        }
        // 取消订阅WebSocket频道
        if (this.wsManager && this.wsManager.isConnected()) {
            this.wsManager.unsubscribe([`delivery_${deliveryId}`]);
        }
    }
    /**
     * 停止所有追踪
     */
    stopAllTracking() {
        this.trackingMap.forEach((timeoutId, deliveryId) => {
            clearInterval(timeoutId);
            // 取消订阅WebSocket频道
            if (this.wsManager && this.wsManager.isConnected()) {
                this.wsManager.unsubscribe([`delivery_${deliveryId}`]);
            }
        });
        this.trackingMap.clear();
    }
    /**
     * 获取正在追踪的配送列表
     */
    getTrackingDeliveries() {
        return Array.from(this.trackingMap.keys());
    }
}
exports.DeliveryTrackingManager = DeliveryTrackingManager;
// ==================== 骑手位置上报 ====================
/**
 * 骑手位置上报管理器
 */
class RiderLocationManager {
    constructor() {
        this.wsManager = null;
        this.locationInterval = null;
        this.isReporting = false;
    }
    /**
     * 设置WebSocket管理器
     */
    setWebSocketManager(wsManager) {
        this.wsManager = wsManager;
    }
    /**
     * 开始位置上报
     */
    startLocationReporting(deliveryId, interval = 10000) {
        if (this.isReporting) {
            console.warn('位置上报已在进行中');
            return;
        }
        this.isReporting = true;
        const reportLocation = () => {
            if (!this.wsManager || !this.wsManager.isConnected()) {
                console.warn('WebSocket未连接，无法上报位置');
                return;
            }
            // 获取当前位置
            wx.getLocation({
                type: 'gcj02',
                success: (res) => {
                    const locationUpdate = {
                        delivery_id: deliveryId,
                        latitude: res.latitude,
                        longitude: res.longitude,
                        accuracy: res.accuracy,
                        speed: res.speed,
                        timestamp: new Date().toISOString()
                    };
                    // 通过WebSocket发送位置更新
                    this.wsManager.send('location_update', locationUpdate, `delivery_${deliveryId}`);
                },
                fail: (error) => {
                    console.error('获取位置失败:', error);
                }
            });
        };
        // 立即上报一次
        reportLocation();
        // 设置定时上报
        this.locationInterval = setInterval(reportLocation, interval);
    }
    /**
     * 停止位置上报
     */
    stopLocationReporting() {
        this.isReporting = false;
        if (this.locationInterval) {
            clearInterval(this.locationInterval);
            this.locationInterval = null;
        }
    }
    /**
     * 是否正在上报位置
     */
    isLocationReporting() {
        return this.isReporting;
    }
}
exports.RiderLocationManager = RiderLocationManager;
// ==================== 便捷函数 ====================
/**
 * WebSocket便捷函数
 */
class WebSocketUtils {
    /**
     * 初始化WebSocket连接
     */
    static initializeConnection(role_1, userId_1) {
        return __awaiter(this, arguments, void 0, function* (role, userId, // 这里应为实际的 User ID
        handlers = {}, entityId // 这里应为角色关联的 ID (如 merchantId)
        ) {
            // 单例模式：只有在不存在或已关闭时才创建新实例
            if (!this.wsManager) {
                this.wsManager = new WebSocketManager();
            }
            yield this.wsManager.connect({
                user_id: userId,
                role,
                entity_id: entityId
            }, handlers);
            return this.wsManager;
        });
    }
    /**
     * 获取WebSocket管理器实例
     */
    static getWebSocketManager() {
        return this.wsManager;
    }
    /**
     * 获取配送追踪管理器
     */
    static getDeliveryTrackingManager() {
        if (!this.trackingManager) {
            this.trackingManager = new DeliveryTrackingManager();
            if (this.wsManager) {
                this.trackingManager.setWebSocketManager(this.wsManager);
            }
        }
        return this.trackingManager;
    }
    /**
     * 获取骑手位置管理器
     */
    static getRiderLocationManager() {
        if (!this.locationManager) {
            this.locationManager = new RiderLocationManager();
            if (this.wsManager) {
                this.locationManager.setWebSocketManager(this.wsManager);
            }
        }
        return this.locationManager;
    }
    /**
     * 检查当前是否已有连接
     */
    static isConnected() {
        return !!this.wsManager && this.wsManager.isConnected();
    }
    /**
     * 关闭所有连接
     */
    static closeAll() {
        var _a, _b, _c;
        (_a = this.trackingManager) === null || _a === void 0 ? void 0 : _a.stopAllTracking();
        (_b = this.locationManager) === null || _b === void 0 ? void 0 : _b.stopLocationReporting();
        (_c = this.wsManager) === null || _c === void 0 ? void 0 : _c.close();
        this.wsManager = null;
        this.trackingManager = null;
        this.locationManager = null;
    }
}
exports.WebSocketUtils = WebSocketUtils;
WebSocketUtils.wsManager = null;
WebSocketUtils.trackingManager = null;
WebSocketUtils.locationManager = null;
/**
 * 实时通信便捷函数
 */
class RealtimeUtils {
    /**
     * 为顾客初始化实时通信
     */
    static initializeForCustomer(userId, onOrderUpdate, onDeliveryUpdate) {
        return __awaiter(this, void 0, void 0, function* () {
            return WebSocketUtils.initializeConnection('customer', userId, {
                onOrderUpdate,
                onDeliveryUpdate,
                onError: (error) => {
                    console.error('顾客端WebSocket错误:', error);
                }
            });
        });
    }
    /**
     * 为商户初始化实时通信
     */
    /**
     * 为商户初始化实时通信
     */
    static initializeForMerchant(userId_1, merchantId_1) {
        return __awaiter(this, arguments, void 0, function* (userId, merchantId, handlers = {}) {
            return WebSocketUtils.initializeConnection('merchant', userId, {
                onOpen: handlers.onOpen,
                onOrderUpdate: handlers.onOrderUpdate,
                onNotification: handlers.onNotification,
                onMessage: handlers.onMessage,
                onError: (error) => {
                    console.error('商户端WebSocket错误:', error);
                }
            }, merchantId);
        });
    }
    /**
     * 为骑手初始化实时通信
     */
    static initializeForRider(userId, onDeliveryUpdate, onLocationUpdate) {
        return __awaiter(this, void 0, void 0, function* () {
            return WebSocketUtils.initializeConnection('rider', userId, {
                onDeliveryUpdate,
                onLocationUpdate,
                onError: (error) => {
                    console.error('骑手端WebSocket错误:', error);
                }
            });
        });
    }
    /**
     * 开始追踪订单配送
     */
    static startOrderTracking(deliveryId, onLocationUpdate) {
        return __awaiter(this, void 0, void 0, function* () {
            const trackingManager = WebSocketUtils.getDeliveryTrackingManager();
            yield trackingManager.startTracking(deliveryId, onLocationUpdate);
        });
    }
    /**
     * 停止追踪订单配送
     */
    static stopOrderTracking(deliveryId) {
        const trackingManager = WebSocketUtils.getDeliveryTrackingManager();
        trackingManager.stopTracking(deliveryId);
    }
    /**
     * 骑手开始位置上报
     */
    static startRiderLocationReporting(deliveryId) {
        const locationManager = WebSocketUtils.getRiderLocationManager();
        locationManager.startLocationReporting(deliveryId);
    }
    /**
     * 骑手停止位置上报
     */
    static stopRiderLocationReporting() {
        const locationManager = WebSocketUtils.getRiderLocationManager();
        locationManager.stopLocationReporting();
    }
}
exports.RealtimeUtils = RealtimeUtils;
// ==================== 数据适配器 ====================
/**
 * 实时数据适配器
 */
class RealtimeDataAdapter {
    /**
     * 适配配送追踪信息
     */
    static adaptDeliveryTracking(tracking) {
        var _a;
        return Object.assign(Object.assign({}, tracking), { delivery_id: Number(tracking.delivery_id), order_id: Number(tracking.order_id), rider_id: Number(tracking.rider_id), current_location: Object.assign(Object.assign({}, tracking.current_location), { latitude: Number(tracking.current_location.latitude), longitude: Number(tracking.current_location.longitude), accuracy: tracking.current_location.accuracy ? Number(tracking.current_location.accuracy) : undefined, heading: tracking.current_location.heading ? Number(tracking.current_location.heading) : undefined, speed: tracking.current_location.speed ? Number(tracking.current_location.speed) : undefined }), route_points: (_a = tracking.route_points) === null || _a === void 0 ? void 0 : _a.map(point => (Object.assign(Object.assign({}, point), { latitude: Number(point.latitude), longitude: Number(point.longitude) }))) });
    }
    /**
     * 适配位置更新数据
     */
    static adaptLocationUpdate(update) {
        return Object.assign(Object.assign({}, update), { delivery_id: Number(update.delivery_id), latitude: Number(update.latitude), longitude: Number(update.longitude), accuracy: update.accuracy ? Number(update.accuracy) : undefined, heading: update.heading ? Number(update.heading) : undefined, speed: update.speed ? Number(update.speed) : undefined });
    }
}
exports.RealtimeDataAdapter = RealtimeDataAdapter;
exports.default = {
    // 管理器类
    WebSocketManager,
    DeliveryTrackingManager,
    RiderLocationManager,
    // 便捷函数
    WebSocketUtils,
    RealtimeUtils,
    // 适配器
    RealtimeDataAdapter
};
