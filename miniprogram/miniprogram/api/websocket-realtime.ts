/**
 * WebSocket和实时通信接口模块
 * 基于swagger.json完全重构，提供WebSocket连接和实时位置追踪
 */

import { request } from '../utils/request';
import { getToken } from '../utils/auth';

// ==================== 类型定义 ====================

// WebSocket连接参数
export interface WebSocketConnectionParams {
    user_id?: number;              // 实际的用户 ID (用于鉴权校验)
    role: 'customer' | 'merchant' | 'rider' | 'operator' | 'admin';
    entity_id?: number;            // 角色相关的实体 ID (如 merchant_id, rider_id)
    channels?: string[];           // 直接指定订阅频道
}

export interface WebSocketMessage {
    type: string;
    channel: string;
    data: any;
    timestamp: string;
    message_id: string;
}

// 实时位置追踪类型
export interface RiderLocationUpdate {
    delivery_id: number;
    latitude: number;
    longitude: number;
    accuracy?: number;
    heading?: number;
    speed?: number;
    timestamp: string;
}

export interface DeliveryTrackingInfo {
    delivery_id: number;
    order_id: number;
    rider_id: number;
    rider_name: string;
    rider_phone: string;
    current_location: {
        latitude: number;
        longitude: number;
        accuracy?: number;
        heading?: number;
        speed?: number;
        updated_at: string;
    };
    status: 'assigned' | 'pickup_started' | 'pickup_completed' | 'delivery_started' | 'delivered';
    estimated_arrival_time?: string;
    route_points?: Array<{
        latitude: number;
        longitude: number;
        timestamp: string;
    }>;
}

// WebSocket事件类型
export interface WebSocketEventHandlers {
    onOpen?: () => void;
    onClose?: (code: number, reason: string) => void;
    onError?: (error: Error) => void;
    onMessage?: (message: WebSocketMessage) => void;
    onOrderUpdate?: (orderData: any) => void;
    onDeliveryUpdate?: (deliveryData: any) => void;
    onLocationUpdate?: (locationData: RiderLocationUpdate) => void;
    onNotification?: (notification: any) => void;
}

// ==================== WebSocket管理器 ====================

/**
 * WebSocket连接管理器
 */
export class WebSocketManager {
    private socketTask: WechatMiniprogram.SocketTask | null = null;
    private connectionParams: WebSocketConnectionParams | null = null;
    private eventHandlers: WebSocketEventHandlers = {};
    private reconnectAttempts = 0;
    private maxReconnectAttempts = 5;
    private reconnectInterval = 3000;
    private heartbeatInterval: ReturnType<typeof setInterval> | null = null;
    private isManualClose = false;
    private isConnecting = false;
    private stableConnectionTimer: any = null;

    /**
     * 连接WebSocket
     * 使用 wx.connectSocket 建立 WebSocket 连接
     */
    async connect(params: WebSocketConnectionParams, handlers: WebSocketEventHandlers = {}): Promise<void> {
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
            await this.closeCurrentTask('Re-connecting');
        }

        try {
            // 获取认证 token
            const token = getToken();

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
                    console.error('WebSocket 连接请求失败:', error);
                    this.isConnecting = false; // 连接请求失败，重置连接状态
                    this.eventHandlers.onError?.(new Error(error.errMsg));
                }
            });

            this.setupEventListeners();

        } catch (error) {
            this.isConnecting = false; // 捕获到错误，重置连接状态
            console.error('WebSocket 系统错误:', error);
            this.eventHandlers.onError?.(error as Error);
            throw error;
        }
    }

    /**
     * 构建 WebSocket URL
     * 注意：后端文档明确说明仅需要 token 这一项 Query Parameter
     * 角色、用户ID 和 频道订阅由后端从 Token 中自动解析
     */
    private buildWebSocketUrl(): string {
        // 从 API_BASE 构建 WSS URL
        // API_BASE 格式: https://xxx.com -> wss://xxx.com/v1/ws
        const { API_CONFIG } = require('../config/index');
        const baseUrl = API_CONFIG.BASE_URL || 'https://llapi.merrydance.cn';
        const wsBase = baseUrl.replace('https://', 'wss://').replace('http://', 'ws://');

        // 获取 token 用于认证
        const token = getToken()?.trim();

        // 构建查询参数 - 仅保留 token
        const queryParts: string[] = [];
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
    private setupEventListeners(): void {
        if (!this.socketTask) return;

        this.socketTask.onOpen(() => {
            console.log('WebSocket连接已物理建立');
            this.isConnecting = false; // 连接成功建立，重置连接状态
            this.startHeartbeat();
            this.eventHandlers.onOpen?.();

            // 稳定性判定：如果在 5 秒内没有断开，则认为连接已稳定，重置重连计数
            if (this.stableConnectionTimer) clearTimeout(this.stableConnectionTimer);
            this.stableConnectionTimer = setTimeout(() => {
                this.reconnectAttempts = 0;
                console.log('WebSocket 连接稳定已达 5s，重置重连尝试次数');
                this.stableConnectionTimer = null;
            }, 5000);
        });

        this.socketTask.onClose((res) => {
            console.log('WebSocket 连接关闭:', res.code, res.reason);
            this.isConnecting = false;
            this.stopHeartbeat();
            this.eventHandlers.onClose?.(res.code, res.reason);

            if (!this.isManualClose) {
                this.attemptReconnect();
            }
        });

        this.socketTask.onError((error: WechatMiniprogram.GeneralCallbackResult) => {
            console.error('WebSocket错误:', error);
            this.eventHandlers.onError?.(new Error(error.errMsg));
        });

        this.socketTask.onMessage((res) => {
            try {
                const message: WebSocketMessage = JSON.parse(res.data as string);
                this.handleMessage(message);
            } catch (error) {
                console.error('解析WebSocket消息失败:', error);
            }
        });
    }

    /**
     * 处理接收到的消息
     */
    private handleMessage(message: WebSocketMessage): void {
        this.eventHandlers.onMessage?.(message);

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
                this.eventHandlers.onOrderUpdate?.(message.data);
                break;
            case 'delivery_update':
                this.eventHandlers.onDeliveryUpdate?.(message.data);
                break;
            case 'location_update':
                this.eventHandlers.onLocationUpdate?.(message.data);
                break;
            case 'notification':
                this.eventHandlers.onNotification?.(message.data);
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
    send(type: string, data: any, channel?: string): void {
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
    subscribe(channels: string[]): void {
        this.send('subscribe', { channels });
    }

    /**
     * 取消订阅频道
     */
    unsubscribe(channels: string[]): void {
        this.send('unsubscribe', { channels });
    }

    /**
     * 开始心跳
     * 后端 60 秒超时，每 45 秒发送一次心跳以保持连接
     */
    private startHeartbeat(): void {
        if (this.heartbeatInterval) clearInterval(this.heartbeatInterval);
        this.heartbeatInterval = setInterval(() => {
            if (this.isConnected()) {
                this.send('ping', { timestamp: new Date().toISOString() });
            }
        }, 45000);
    }

    /**
     * 停止心跳
     */
    private stopHeartbeat(): void {
        if (this.heartbeatInterval) {
            clearInterval(this.heartbeatInterval);
            this.heartbeatInterval = null;
        }
    }

    /**
     * 尝试重连
     */
    private attemptReconnect(): void {
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
    private async closeCurrentTask(reason: string = 'Close'): Promise<void> {
        if (!this.socketTask) return;

        await new Promise<void>((resolve) => {
            if (!this.socketTask) return resolve();

            const task = this.socketTask;
            let resolved = false;

            const finish = () => {
                if (resolved) return;
                resolved = true;
                resolve();
            };

            task.onClose(finish);
            task.onError(finish);

            try {
                task.close({ code: 1000, reason });
            } catch (e) {
                finish();
            }

            // 强制超时释放
            setTimeout(finish, 1000);
        });

        this.socketTask = null;
    }

    /**
     * 关闭连接
     */
    public async close(): Promise<void> {
        this.isManualClose = true;
        this.stopHeartbeat();
        await this.closeCurrentTask('Manual close');
    }

    /**
     * 获取连接状态
     * 注意：微信小程序 SocketTask 没有 readyState，返回简化状态
     */
    getReadyState(): number {
        return this.socketTask ? 1 : 3; // 1=OPEN, 3=CLOSED
    }

    /**
     * 是否已连接 (处于物理已开启或正在开启状态)
     */
    public isConnected(): boolean {
        // 如果 socketTask 不存在，肯定没连接
        if (!this.socketTask) return false;

        // 如果进入了手动关闭流程，认为已断开
        if (this.isManualClose) return false;

        return true;
    }

    /**
     * 生成消息ID
     */
    private generateMessageId(): string {
        return `${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
    }
}

// ==================== 实时位置追踪 ====================

/**
 * 配送实时追踪管理器
 */
export class DeliveryTrackingManager {
    private trackingMap = new Map<number, ReturnType<typeof setInterval>>();
    private wsManager: WebSocketManager | null = null;

    /**
     * 设置WebSocket管理器
     */
    setWebSocketManager(wsManager: WebSocketManager): void {
        this.wsManager = wsManager;
    }

    /**
     * 获取配送追踪信息
     */
    async getDeliveryTracking(deliveryId: number): Promise<DeliveryTrackingInfo> {
        return request({
            url: `/v1/delivery/${deliveryId}/track`,
            method: 'GET'
        });
    }

    /**
     * 开始追踪配送
     */
    async startTracking(
        deliveryId: number,
        onLocationUpdate: (trackingInfo: DeliveryTrackingInfo) => void,
        interval: number = 5000
    ): Promise<void> {
        // 停止之前的追踪
        this.stopTracking(deliveryId);

        // 立即获取一次位置信息
        try {
            const trackingInfo = await this.getDeliveryTracking(deliveryId);
            onLocationUpdate(trackingInfo);
        } catch (error) {
            console.error('获取配送追踪信息失败:', error);
        }

        // 设置定时轮询
        const timeoutId = setInterval(async () => {
            try {
                const trackingInfo = await this.getDeliveryTracking(deliveryId);
                onLocationUpdate(trackingInfo);

                // 如果配送已完成，停止追踪
                if (trackingInfo.status === 'delivered') {
                    this.stopTracking(deliveryId);
                }
            } catch (error) {
                console.error('轮询配送追踪信息失败:', error);
            }
        }, interval);

        this.trackingMap.set(deliveryId, timeoutId);

        // 如果有WebSocket连接，订阅实时位置更新
        if (this.wsManager && this.wsManager.isConnected()) {
            this.wsManager.subscribe([`delivery_${deliveryId}`]);
        }
    }

    /**
     * 停止追踪配送
     */
    stopTracking(deliveryId: number): void {
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
    stopAllTracking(): void {
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
    getTrackingDeliveries(): number[] {
        return Array.from(this.trackingMap.keys());
    }
}

// ==================== 骑手位置上报 ====================

/**
 * 骑手位置上报管理器
 */
export class RiderLocationManager {
    private wsManager: WebSocketManager | null = null;
    private locationInterval: ReturnType<typeof setInterval> | null = null;
    private isReporting = false;

    /**
     * 设置WebSocket管理器
     */
    setWebSocketManager(wsManager: WebSocketManager): void {
        this.wsManager = wsManager;
    }

    /**
     * 开始位置上报
     */
    startLocationReporting(deliveryId: number, interval: number = 10000): void {
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
                    const locationUpdate: RiderLocationUpdate = {
                        delivery_id: deliveryId,
                        latitude: res.latitude,
                        longitude: res.longitude,
                        accuracy: res.accuracy,
                        speed: res.speed,
                        timestamp: new Date().toISOString()
                    };

                    // 通过WebSocket发送位置更新
                    this.wsManager!.send('location_update', locationUpdate, `delivery_${deliveryId}`);
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
    stopLocationReporting(): void {
        this.isReporting = false;

        if (this.locationInterval) {
            clearInterval(this.locationInterval);
            this.locationInterval = null;
        }
    }

    /**
     * 是否正在上报位置
     */
    isLocationReporting(): boolean {
        return this.isReporting;
    }
}

// ==================== 便捷函数 ====================

/**
 * WebSocket便捷函数
 */
export class WebSocketUtils {
    private static wsManager: WebSocketManager | null = null;
    private static trackingManager: DeliveryTrackingManager | null = null;
    private static locationManager: RiderLocationManager | null = null;

    /**
     * 初始化WebSocket连接
     */
    static async initializeConnection(
        role: 'customer' | 'merchant' | 'rider' | 'operator' | 'admin',
        userId?: number, // 这里应为实际的 User ID
        handlers: WebSocketEventHandlers = {},
        entityId?: number // 这里应为角色关联的 ID (如 merchantId)
    ): Promise<WebSocketManager> {
        // 单例模式：只有在不存在或已关闭时才创建新实例
        if (!this.wsManager) {
            this.wsManager = new WebSocketManager();
        }

        await this.wsManager.connect({
            user_id: userId,
            role,
            entity_id: entityId
        }, handlers);

        return this.wsManager;
    }

    /**
     * 获取WebSocket管理器实例
     */
    static getWebSocketManager(): WebSocketManager | null {
        return this.wsManager;
    }

    /**
     * 获取配送追踪管理器
     */
    static getDeliveryTrackingManager(): DeliveryTrackingManager {
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
    static getRiderLocationManager(): RiderLocationManager {
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
    static isConnected(): boolean {
        return !!this.wsManager && this.wsManager.isConnected();
    }

    /**
     * 关闭所有连接
     */
    static closeAll(): void {
        this.trackingManager?.stopAllTracking();
        this.locationManager?.stopLocationReporting();
        this.wsManager?.close();

        this.wsManager = null;
        this.trackingManager = null;
        this.locationManager = null;
    }
}

/**
 * 实时通信便捷函数
 */
export class RealtimeUtils {
    /**
     * 为顾客初始化实时通信
     */
    static async initializeForCustomer(
        userId: number,
        onOrderUpdate?: (orderData: any) => void,
        onDeliveryUpdate?: (deliveryData: any) => void
    ): Promise<WebSocketManager> {
        return WebSocketUtils.initializeConnection('customer', userId, {
            onOrderUpdate,
            onDeliveryUpdate,
            onError: (error) => {
                console.error('顾客端WebSocket错误:', error);
            }
        });
    }

    /**
     * 为商户初始化实时通信
     */
    /**
     * 为商户初始化实时通信
     */
    static async initializeForMerchant(
        userId: number,
        merchantId: number,
        handlers: {
            onOpen?: () => void;
            onOrderUpdate?: (orderData: any) => void;
            onNotification?: (notification: any) => void;
            onMessage?: (message: WebSocketMessage) => void;
        } = {}
    ): Promise<WebSocketManager> {
        return WebSocketUtils.initializeConnection('merchant', userId, {
            onOpen: handlers.onOpen,
            onOrderUpdate: handlers.onOrderUpdate,
            onNotification: handlers.onNotification,
            onMessage: handlers.onMessage,
            onError: (error) => {
                console.error('商户端WebSocket错误:', error);
            }
        }, merchantId);
    }

    /**
     * 为骑手初始化实时通信
     */
    static async initializeForRider(
        userId: number,
        onDeliveryUpdate?: (deliveryData: any) => void,
        onLocationUpdate?: (locationData: RiderLocationUpdate) => void
    ): Promise<WebSocketManager> {
        return WebSocketUtils.initializeConnection('rider', userId, {
            onDeliveryUpdate,
            onLocationUpdate,
            onError: (error) => {
                console.error('骑手端WebSocket错误:', error);
            }
        });
    }

    /**
     * 开始追踪订单配送
     */
    static async startOrderTracking(
        deliveryId: number,
        onLocationUpdate: (trackingInfo: DeliveryTrackingInfo) => void
    ): Promise<void> {
        const trackingManager = WebSocketUtils.getDeliveryTrackingManager();
        await trackingManager.startTracking(deliveryId, onLocationUpdate);
    }

    /**
     * 停止追踪订单配送
     */
    static stopOrderTracking(deliveryId: number): void {
        const trackingManager = WebSocketUtils.getDeliveryTrackingManager();
        trackingManager.stopTracking(deliveryId);
    }

    /**
     * 骑手开始位置上报
     */
    static startRiderLocationReporting(deliveryId: number): void {
        const locationManager = WebSocketUtils.getRiderLocationManager();
        locationManager.startLocationReporting(deliveryId);
    }

    /**
     * 骑手停止位置上报
     */
    static stopRiderLocationReporting(): void {
        const locationManager = WebSocketUtils.getRiderLocationManager();
        locationManager.stopLocationReporting();
    }
}

// ==================== 数据适配器 ====================

/**
 * 实时数据适配器
 */
export class RealtimeDataAdapter {
    /**
     * 适配配送追踪信息
     */
    static adaptDeliveryTracking(tracking: DeliveryTrackingInfo): DeliveryTrackingInfo {
        return {
            ...tracking,
            delivery_id: Number(tracking.delivery_id),
            order_id: Number(tracking.order_id),
            rider_id: Number(tracking.rider_id),
            current_location: {
                ...tracking.current_location,
                latitude: Number(tracking.current_location.latitude),
                longitude: Number(tracking.current_location.longitude),
                accuracy: tracking.current_location.accuracy ? Number(tracking.current_location.accuracy) : undefined,
                heading: tracking.current_location.heading ? Number(tracking.current_location.heading) : undefined,
                speed: tracking.current_location.speed ? Number(tracking.current_location.speed) : undefined
            },
            route_points: tracking.route_points?.map(point => ({
                ...point,
                latitude: Number(point.latitude),
                longitude: Number(point.longitude)
            }))
        };
    }

    /**
     * 适配位置更新数据
     */
    static adaptLocationUpdate(update: RiderLocationUpdate): RiderLocationUpdate {
        return {
            ...update,
            delivery_id: Number(update.delivery_id),
            latitude: Number(update.latitude),
            longitude: Number(update.longitude),
            accuracy: update.accuracy ? Number(update.accuracy) : undefined,
            heading: update.heading ? Number(update.heading) : undefined,
            speed: update.speed ? Number(update.speed) : undefined
        };
    }
}

export default {
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