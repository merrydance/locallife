"use strict";
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    var desc = Object.getOwnPropertyDescriptor(m, k);
    if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
      desc = { enumerable: true, get: function() { return m[k]; } };
    }
    Object.defineProperty(o, k2, desc);
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __setModuleDefault = (this && this.__setModuleDefault) || (Object.create ? (function(o, v) {
    Object.defineProperty(o, "default", { enumerable: true, value: v });
}) : function(o, v) {
    o["default"] = v;
});
var __importStar = (this && this.__importStar) || (function () {
    var ownKeys = function(o) {
        ownKeys = Object.getOwnPropertyNames || function (o) {
            var ar = [];
            for (var k in o) if (Object.prototype.hasOwnProperty.call(o, k)) ar[ar.length] = k;
            return ar;
        };
        return ownKeys(o);
    };
    return function (mod) {
        if (mod && mod.__esModule) return mod;
        var result = {};
        if (mod != null) for (var k = ownKeys(mod), i = 0; i < k.length; i++) if (k[i] !== "default") __createBinding(result, mod, k[i]);
        __setModuleDefault(result, mod);
        return result;
    };
})();
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
const order_1 = require("../../../api/order");
const delivery_1 = require("../../../api/delivery");
const logger_1 = require("../../../utils/logger");
Page({
    data: {
        orderId: 0,
        deliveryId: 0,
        navBarHeight: 88,
        loading: true,
        // 配送信息
        delivery: null,
        riderName: '',
        riderPhone: '',
        riderPhoneDisplay: '',
        estimatedDeliveryTime: '',
        deliveryStatus: '',
        deliveryStatusText: '',
        // 地图相关
        mapCenter: { latitude: 39.9, longitude: 116.4 },
        scale: 14,
        markers: [],
        polyline: [],
        includePoints: [],
        // 进度
        progress: [],
        // 位置刷新定时器
        locationTimer: null
    },
    onLoad(options) {
        if (options.orderId) {
            this.setData({ orderId: parseInt(options.orderId) });
            this.loadDeliveryData();
        }
    },
    onUnload() {
        // 清理定时器
        if (this.data.locationTimer) {
            clearInterval(this.data.locationTimer);
        }
    },
    loadDeliveryData() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                // 1. 获取订单信息
                const order = yield (0, order_1.getOrderDetail)(this.data.orderId);
                // 2. 获取配送信息
                const delivery = yield (0, delivery_1.getDeliveryByOrder)(this.data.orderId);
                // 3. 处理骑手信息
                const riderPhone = delivery.pickup_phone || '';
                const riderPhoneDisplay = riderPhone ? riderPhone.replace(/(\d{3})\d{4}(\d{4})/, '$1****$2') : '';
                // 4. 生成配送进度
                const progress = this.generateProgress(delivery);
                // 5. 设置地图标记点
                const merchantPoint = {
                    latitude: delivery.pickup_latitude,
                    longitude: delivery.pickup_longitude
                };
                const customerPoint = {
                    latitude: delivery.delivery_latitude,
                    longitude: delivery.delivery_longitude
                };
                this.setData({
                    delivery,
                    deliveryId: delivery.id,
                    riderName: '骑手', // 后端暂无骑手姓名字段
                    riderPhone,
                    riderPhoneDisplay,
                    estimatedDeliveryTime: delivery.estimated_delivery_at ? this.formatTime(delivery.estimated_delivery_at) : '计算中',
                    deliveryStatus: delivery.status,
                    deliveryStatusText: this.getStatusText(delivery.status),
                    progress,
                    loading: false
                });
                // 6. 设置地图
                this.setupMap(merchantPoint, customerPoint);
                // 7. 开始位置追踪（配送中状态）
                if (delivery.status === 'delivering' || delivery.status === 'picked') {
                    this.startLocationTracking();
                }
            }
            catch (error) {
                logger_1.logger.error('加载配送信息失败', error, 'tracking.loadDeliveryData');
                wx.showToast({ title: '加载失败', icon: 'error' });
                this.setData({ loading: false });
            }
        });
    },
    generateProgress(delivery) {
        const progress = [
            {
                title: '商家已接单',
                time: delivery.created_at ? this.formatTime(delivery.created_at) : '',
                done: true,
                active: false
            },
            {
                title: '骑手已接单',
                time: delivery.assigned_at ? this.formatTime(delivery.assigned_at) : '',
                done: !!delivery.assigned_at,
                active: delivery.status === 'assigned'
            },
            {
                title: '骑手已取餐',
                time: delivery.picked_at ? this.formatTime(delivery.picked_at) : '',
                done: !!delivery.picked_at,
                active: delivery.status === 'picked'
            },
            {
                title: '配送中',
                time: '',
                done: delivery.status === 'delivering',
                active: delivery.status === 'delivering'
            },
            {
                title: '已送达',
                time: delivery.delivered_at ? this.formatTime(delivery.delivered_at) : '',
                done: !!delivery.delivered_at,
                active: delivery.status === 'delivered'
            }
        ];
        return progress;
    },
    getStatusText(status) {
        const statusMap = {
            'pending': '等待骑手接单',
            'assigned': '骑手已接单',
            'picking': '骑手正在取餐',
            'picked': '骑手已取餐',
            'delivering': '骑手正在配送',
            'delivered': '已送达',
            'cancelled': '配送已取消'
        };
        return statusMap[status] || status;
    },
    formatTime(timeStr) {
        try {
            const date = new Date(timeStr);
            const hours = ('0' + date.getHours()).slice(-2);
            const minutes = ('0' + date.getMinutes()).slice(-2);
            return `${hours}:${minutes}`;
        }
        catch (_a) {
            return '';
        }
    },
    setupMap(merchantPoint, customerPoint) {
        return __awaiter(this, void 0, void 0, function* () {
            const markers = [
                this.buildMarker(1, merchantPoint, '商家', '/assets/merchant.png'),
                this.buildMarker(3, customerPoint, '我', '/assets/customer.png')
            ];
            const includePoints = [merchantPoint, customerPoint];
            // 计算地图中心
            const mapCenter = {
                latitude: (merchantPoint.latitude + customerPoint.latitude) / 2,
                longitude: (merchantPoint.longitude + customerPoint.longitude) / 2
            };
            this.setData({ markers, includePoints, mapCenter });
            // 获取骑手位置
            yield this.updateRiderLocation();
            // 规划路线
            this.planRoute(merchantPoint, customerPoint);
        });
    },
    updateRiderLocation() {
        return __awaiter(this, void 0, void 0, function* () {
            const { deliveryId, delivery } = this.data;
            if (!deliveryId || !delivery)
                return;
            try {
                const location = yield (0, delivery_1.getRiderLocation)(deliveryId);
                if (location && location.latitude && location.longitude) {
                    const riderPoint = {
                        latitude: location.latitude,
                        longitude: location.longitude
                    };
                    // 更新骑手标记
                    const markers = [...this.data.markers];
                    const riderMarkerIndex = markers.findIndex(m => m.id === 2);
                    const riderMarker = this.buildMarker(2, riderPoint, '骑手', '/assets/rider.png');
                    if (riderMarkerIndex >= 0) {
                        markers[riderMarkerIndex] = riderMarker;
                    }
                    else {
                        markers.push(riderMarker);
                    }
                    // 更新includePoints
                    const includePoints = [
                        { latitude: delivery.pickup_latitude, longitude: delivery.pickup_longitude },
                        riderPoint,
                        { latitude: delivery.delivery_latitude, longitude: delivery.delivery_longitude }
                    ];
                    this.setData({ markers, includePoints });
                }
            }
            catch (error) {
                logger_1.logger.warn('获取骑手位置失败', error, 'tracking.updateRiderLocation');
            }
        });
    },
    startLocationTracking() {
        // 每10秒刷新一次骑手位置
        const timer = setInterval(() => {
            this.updateRiderLocation();
        }, 10000);
        this.setData({ locationTimer: timer });
    },
    planRoute(merchantPoint, customerPoint) {
        return __awaiter(this, void 0, void 0, function* () {
            var _a, _b;
            try {
                const { request } = yield Promise.resolve().then(() => __importStar(require('../../../utils/request')));
                const fromStr = `${merchantPoint.latitude},${merchantPoint.longitude}`;
                const toStr = `${customerPoint.latitude},${customerPoint.longitude}`;
                const data = yield request({
                    url: '/v1/location/direction/bicycling',
                    method: 'GET',
                    data: { from: fromStr, to: toStr, policy: 0 }
                });
                if (data.status === 0 && ((_b = (_a = data.result) === null || _a === void 0 ? void 0 : _a.routes) === null || _b === void 0 ? void 0 : _b[0])) {
                    const route = data.result.routes[0];
                    const points = this.decodePolyline(route.polyline);
                    this.setData({
                        polyline: [{
                                points,
                                color: '#1d63ff',
                                width: 8,
                                arrowLine: true
                            }]
                    });
                }
                else {
                    this.useFallbackRoute(merchantPoint, customerPoint);
                }
            }
            catch (error) {
                logger_1.logger.warn('路线规划失败', error, 'tracking.planRoute');
                this.useFallbackRoute(merchantPoint, customerPoint);
            }
        });
    },
    decodePolyline(coors) {
        const decoded = [...coors];
        for (let i = 2; i < decoded.length; i++) {
            decoded[i] = decoded[i - 2] + decoded[i] / 1000000;
        }
        const points = [];
        for (let i = 0; i < decoded.length; i += 2) {
            const lat = decoded[i];
            const lng = decoded[i + 1];
            if (lat >= -90 && lat <= 90 && lng >= -180 && lng <= 180) {
                points.push({ latitude: lat, longitude: lng });
            }
        }
        return points;
    },
    useFallbackRoute(merchantPoint, customerPoint) {
        this.setData({
            polyline: [{
                    points: [merchantPoint, customerPoint],
                    color: '#1d63ff',
                    width: 6,
                    dottedLine: true
                }]
        });
    },
    buildMarker(id, point, label, iconPath) {
        return {
            id,
            latitude: point.latitude,
            longitude: point.longitude,
            width: 40,
            height: 40,
            iconPath,
            callout: {
                content: label,
                color: '#333',
                fontSize: 14,
                padding: 6,
                borderRadius: 12,
                display: 'ALWAYS',
                bgColor: '#fff'
            }
        };
    },
    onCallRider() {
        const { riderPhone } = this.data;
        if (riderPhone) {
            wx.makePhoneCall({ phoneNumber: riderPhone });
        }
        else {
            wx.showToast({ title: '暂无骑手电话', icon: 'none' });
        }
    },
    onNavHeight(event) {
        this.setData({ navBarHeight: event.detail.navBarHeight });
    },
    onRefresh() {
        this.loadDeliveryData();
    }
});
