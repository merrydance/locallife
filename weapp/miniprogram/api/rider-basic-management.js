"use strict";
/**
 * 骑手基础管理接口重构 (Task 3.1)
 * 基于swagger.json完全重构，移除所有没有后端支持的旧功能
 * 包含：骑手信息、上下线管理、位置上报、积分管理
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
exports.ScoreManagementUtils = exports.LocationReportManager = exports.locationManagementService = exports.riderBasicManagementService = exports.RiderBasicManagementAdapter = exports.LocationManagementService = exports.RiderBasicManagementService = void 0;
exports.getRiderDashboard = getRiderDashboard;
exports.smartOnlineManagement = smartOnlineManagement;
exports.formatRiderStatus = formatRiderStatus;
exports.formatOnlineStatus = formatOnlineStatus;
exports.formatOnlineDuration = formatOnlineDuration;
exports.formatEarnings = formatEarnings;
exports.validateLocationPoint = validateLocationPoint;
const request_1 = require("../utils/request");
// ==================== 骑手基础管理服务类 ====================
/**
 * 骑手基础管理服务
 * 提供骑手信息查询、状态管理、位置上报等功能
 */
class RiderBasicManagementService {
    /**
     * 获取当前骑手信息
     */
    getRiderInfo() {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/rider/me',
                method: 'GET'
            });
        });
    }
    /**
     * 获取骑手状态
     */
    getRiderStatus() {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/rider/status',
                method: 'GET'
            });
        });
    }
    /**
     * 骑手上线
     */
    goOnline() {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/rider/online',
                method: 'POST'
            });
        });
    }
    /**
     * 骑手下线
     */
    goOffline() {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/rider/offline',
                method: 'POST'
            });
        });
    }
    /**
     * 更新骑手位置
     * @param locationData 位置数据
     */
    updateLocation(locationData) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/rider/location',
                method: 'POST',
                data: locationData
            });
        });
    }
    /**
     * 获取骑手积分信息
     */
    getRiderScore() {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/rider/score',
                method: 'GET'
            });
        });
    }
    /**
     * 获取积分历史记录
     * @param params 查询参数
     */
    getScoreHistory(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/rider/score/history',
                method: 'GET',
                data: params
            });
        });
    }
}
exports.RiderBasicManagementService = RiderBasicManagementService;
// ==================== 位置管理服务类 ====================
/**
 * 位置管理服务
 * 提供位置上报、轨迹管理等功能
 */
class LocationManagementService {
    /**
     * 批量上报位置点
     * @param locations 位置点数组
     */
    batchUpdateLocation(locations) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/rider/location',
                method: 'POST',
                data: { locations }
            });
        });
    }
    /**
     * 单点位置上报
     * @param location 单个位置点
     */
    updateSingleLocation(location) {
        return __awaiter(this, void 0, void 0, function* () {
            return this.batchUpdateLocation([location]);
        });
    }
    /**
     * 获取当前位置
     */
    getCurrentLocation() {
        return __awaiter(this, void 0, void 0, function* () {
            const riderInfo = yield new RiderBasicManagementService().getRiderInfo();
            return {
                latitude: riderInfo.current_latitude || 0,
                longitude: riderInfo.current_longitude || 0,
                updated_at: riderInfo.location_updated_at || ''
            };
        });
    }
}
exports.LocationManagementService = LocationManagementService;
// ==================== 数据适配器 ====================
/**
 * 骑手基础管理数据适配器
 * 处理前端数据格式与后端API数据格式的转换
 */
class RiderBasicManagementAdapter {
    /**
     * 适配骑手响应数据
     */
    static adaptRiderResponse(data) {
        return {
            id: data.id,
            userId: data.user_id,
            realName: data.real_name,
            phone: data.phone,
            status: data.status,
            isOnline: data.is_online,
            currentLatitude: data.current_latitude,
            currentLongitude: data.current_longitude,
            locationUpdatedAt: data.location_updated_at,
            depositAmount: data.deposit_amount,
            frozenDeposit: data.frozen_deposit,
            creditScore: data.credit_score,
            totalOrders: data.total_orders,
            totalEarnings: data.total_earnings,
            onlineDuration: data.online_duration,
            createdAt: data.created_at
        };
    }
    /**
     * 适配骑手状态响应数据
     */
    static adaptRiderStatusResponse(data) {
        return {
            isOnline: data.is_online,
            onlineStatus: data.online_status,
            status: data.status,
            currentLatitude: data.current_latitude,
            currentLongitude: data.current_longitude,
            locationUpdatedAt: data.location_updated_at,
            activeDeliveries: data.active_deliveries,
            canGoOnline: data.can_go_online,
            canGoOffline: data.can_go_offline,
            onlineBlockReason: data.online_block_reason
        };
    }
    /**
     * 适配位置点数据
     */
    static adaptLocationPoint(data) {
        return {
            latitude: data.latitude,
            longitude: data.longitude,
            recorded_at: data.recordedAt,
            accuracy: data.accuracy,
            speed: data.speed,
            heading: data.heading
        };
    }
    /**
     * 适配积分历史记录
     */
    static adaptScoreHistoryItem(data) {
        return {
            id: data.id,
            riderId: data.rider_id,
            orderId: data.order_id,
            scoreChange: data.score_change,
            reason: data.reason,
            description: data.description,
            createdAt: data.created_at
        };
    }
}
exports.RiderBasicManagementAdapter = RiderBasicManagementAdapter;
// ==================== 导出服务实例 ====================
exports.riderBasicManagementService = new RiderBasicManagementService();
exports.locationManagementService = new LocationManagementService();
// ==================== 便捷函数 ====================
/**
 * 获取骑手工作台数据
 */
function getRiderDashboard() {
    return __awaiter(this, void 0, void 0, function* () {
        const [riderInfo, riderStatus, scoreInfo] = yield Promise.all([
            exports.riderBasicManagementService.getRiderInfo(),
            exports.riderBasicManagementService.getRiderStatus(),
            exports.riderBasicManagementService.getRiderScore()
        ]);
        // 今日统计数据需要根据实际接口调整
        const todayStats = {
            onlineDuration: riderInfo.online_duration,
            completedOrders: riderInfo.total_orders,
            earnings: riderInfo.total_earnings
        };
        return {
            riderInfo,
            riderStatus,
            scoreInfo,
            todayStats
        };
    });
}
/**
 * 智能上下线管理
 * @param action 操作类型
 */
function smartOnlineManagement(action) {
    return __awaiter(this, void 0, void 0, function* () {
        try {
            const status = yield exports.riderBasicManagementService.getRiderStatus();
            if (action === 'online') {
                if (!status.can_go_online) {
                    return {
                        success: false,
                        message: status.online_block_reason || '当前无法上线'
                    };
                }
                const riderInfo = yield exports.riderBasicManagementService.goOnline();
                return {
                    success: true,
                    message: '上线成功',
                    riderInfo
                };
            }
            else {
                if (!status.can_go_offline) {
                    return {
                        success: false,
                        message: status.active_deliveries > 0 ? '有配送中的订单，无法下线' : '当前无法下线'
                    };
                }
                const riderInfo = yield exports.riderBasicManagementService.goOffline();
                return {
                    success: true,
                    message: '下线成功',
                    riderInfo
                };
            }
        }
        catch (error) {
            return {
                success: false,
                message: (error === null || error === void 0 ? void 0 : error.message) || `${action === 'online' ? '上线' : '下线'}失败`
            };
        }
    });
}
/**
 * 位置上报管理器
 */
class LocationReportManager {
    constructor() {
        this.reportInterval = 30000; // 30秒上报一次
        this.intervalId = null;
        this.lastLocation = null;
    }
    /**
     * 开始自动位置上报
     * @param interval 上报间隔（毫秒）
     */
    startAutoReport(interval = 30000) {
        this.reportInterval = interval;
        this.stopAutoReport(); // 先停止之前的定时器
        this.intervalId = setInterval(() => __awaiter(this, void 0, void 0, function* () {
            try {
                yield this.reportCurrentLocation();
            }
            catch (error) {
                console.error('位置上报失败:', error);
            }
        }), this.reportInterval);
    }
    /**
     * 停止自动位置上报
     */
    stopAutoReport() {
        if (this.intervalId) {
            clearInterval(this.intervalId);
            this.intervalId = null;
        }
    }
    /**
     * 上报当前位置
     */
    reportCurrentLocation() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                // 获取当前位置（这里需要调用微信小程序的位置API）
                const location = yield this.getCurrentPosition();
                if (location) {
                    const result = yield exports.locationManagementService.updateSingleLocation(location);
                    this.lastLocation = location;
                    return result;
                }
            }
            catch (error) {
                console.error('获取位置失败:', error);
            }
            return null;
        });
    }
    /**
     * 获取当前GPS位置
     */
    getCurrentPosition() {
        return __awaiter(this, void 0, void 0, function* () {
            return new Promise((resolve) => {
                // 微信小程序获取位置
                wx.getLocation({
                    type: 'gcj02',
                    success: (res) => {
                        resolve({
                            latitude: res.latitude,
                            longitude: res.longitude,
                            recorded_at: new Date().toISOString(),
                            accuracy: res.accuracy,
                            speed: res.speed,
                            heading: res.heading
                        });
                    },
                    fail: () => {
                        resolve(null);
                    }
                });
            });
        });
    }
    /**
     * 获取最后上报的位置
     */
    getLastLocation() {
        return this.lastLocation;
    }
}
exports.LocationReportManager = LocationReportManager;
/**
 * 积分管理工具
 */
class ScoreManagementUtils {
    /**
     * 计算积分等级
     * @param score 当前积分
     */
    static calculateScoreLevel(score) {
        if (score >= 100) {
            return {
                level: 'excellent',
                levelName: '优秀骑手',
                canTakeHighValueOrders: true
            };
        }
        else if (score >= 50) {
            return {
                level: 'good',
                levelName: '良好骑手',
                canTakeHighValueOrders: true,
                nextLevelThreshold: 100
            };
        }
        else if (score >= 0) {
            return {
                level: 'normal',
                levelName: '普通骑手',
                canTakeHighValueOrders: true,
                nextLevelThreshold: 50
            };
        }
        else {
            return {
                level: 'restricted',
                levelName: '受限骑手',
                canTakeHighValueOrders: false,
                nextLevelThreshold: 0
            };
        }
    }
    /**
     * 格式化积分变化原因
     * @param reason 原因代码
     */
    static formatScoreChangeReason(reason) {
        const reasonMap = {
            'complete_normal_order': '完成普通订单',
            'complete_high_value_order': '完成高值订单',
            'timeout': '订单超时',
            'damage': '餐损',
            'complaint': '投诉',
            'praise': '表扬',
            'manual_adjustment': '人工调整'
        };
        return reasonMap[reason] || reason;
    }
    /**
     * 预测积分变化影响
     * @param currentScore 当前积分
     * @param scoreChange 积分变化
     */
    static predictScoreImpact(currentScore, scoreChange) {
        const newScore = currentScore + scoreChange;
        const currentLevel = this.calculateScoreLevel(currentScore);
        const newLevel = this.calculateScoreLevel(newScore);
        let warning;
        if (newScore < 0 && currentScore >= 0) {
            warning = '积分将变为负数，将无法接高值单';
        }
        else if (newScore < -50) {
            warning = '积分过低，可能面临账号限制';
        }
        return {
            newScore,
            levelChange: currentLevel.level !== newLevel.level,
            newLevel: newLevel.levelName,
            canTakeHighValueOrders: newLevel.canTakeHighValueOrders,
            warning
        };
    }
}
exports.ScoreManagementUtils = ScoreManagementUtils;
/**
 * 格式化骑手状态显示
 * @param status 骑手状态
 */
function formatRiderStatus(status) {
    const statusMap = {
        pending: '待审核',
        active: '正常',
        suspended: '已暂停',
        rejected: '已拒绝'
    };
    return statusMap[status] || status;
}
/**
 * 格式化在线状态显示
 * @param onlineStatus 在线状态
 */
function formatOnlineStatus(onlineStatus) {
    const statusMap = {
        offline: '离线',
        online: '在线',
        delivering: '配送中'
    };
    return statusMap[onlineStatus] || onlineStatus;
}
/**
 * 计算在线时长显示
 * @param duration 在线时长（秒）
 */
function formatOnlineDuration(duration) {
    const hours = Math.floor(duration / 3600);
    const minutes = Math.floor((duration % 3600) / 60);
    if (hours > 0) {
        return `${hours}小时${minutes}分钟`;
    }
    else {
        return `${minutes}分钟`;
    }
}
/**
 * 格式化收入显示
 * @param amount 金额（分）
 * @param showUnit 是否显示单位
 */
function formatEarnings(amount, showUnit = true) {
    const yuan = (amount / 100).toFixed(2);
    return showUnit ? `¥${yuan}` : yuan;
}
/**
 * 验证位置数据
 * @param location 位置数据
 */
function validateLocationPoint(location) {
    if (!location.latitude || !location.longitude) {
        return { valid: false, message: '经纬度不能为空' };
    }
    if (location.latitude < -90 || location.latitude > 90) {
        return { valid: false, message: '纬度范围应在-90到90之间' };
    }
    if (location.longitude < -180 || location.longitude > 180) {
        return { valid: false, message: '经度范围应在-180到180之间' };
    }
    if (location.accuracy && (location.accuracy < 0 || location.accuracy > 1000)) {
        return { valid: false, message: 'GPS精度应在0到1000米之间' };
    }
    return { valid: true };
}
