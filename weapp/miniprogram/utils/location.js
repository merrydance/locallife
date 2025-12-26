"use strict";
/**
 * 位置服务工具类
 * 通过后端接口调用腾讯LBS获取用户位置信息
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
exports.locationService = void 0;
exports.getDeviceId = getDeviceId;
const logger_1 = require("./logger");
const request_1 = require("./request");
/**
 * 位置服务类
 */
class LocationService {
    /**
     * 获取当前位置（经纬度）
     */
    getCurrentLocation() {
        return __awaiter(this, void 0, void 0, function* () {
            return new Promise((resolve, reject) => {
                wx.getLocation({
                    type: 'gcj02', // 返回可以用于wx.openLocation的坐标
                    success: (res) => {
                        logger_1.logger.info('获取位置成功', {
                            latitude: res.latitude,
                            longitude: res.longitude
                        }, 'LocationService.getCurrentLocation');
                        resolve({
                            latitude: res.latitude,
                            longitude: res.longitude
                        });
                    },
                    fail: (err) => {
                        logger_1.logger.warn('获取位置失败', err, 'LocationService.getCurrentLocation');
                        reject(err);
                    }
                });
            });
        });
    }
    /**
     * 逆地理编码 - 通过后端接口将经纬度转换为地址
     * 后端接口: GET /v1/location/reverse-geocode?latitude=xxx&longitude=xxx
     */
    reverseGeocode(latitude, longitude) {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const response = yield (0, request_1.request)({
                    url: '/v1/location/reverse-geocode',
                    method: 'GET',
                    data: {
                        latitude,
                        longitude
                    }
                });
                logger_1.logger.info('逆地理编码成功', response, 'LocationService.reverseGeocode');
                // 补充经纬度信息（后端可能不返回）
                if (!response.latitude)
                    response.latitude = latitude;
                if (!response.longitude)
                    response.longitude = longitude;
                return response;
            }
            catch (err) {
                logger_1.logger.error('逆地理编码失败', err, 'LocationService.reverseGeocode');
                // 即使逆地理编码失败，也返回基本的位置信息
                return {
                    latitude,
                    longitude,
                    address: `${latitude.toFixed(6)}, ${longitude.toFixed(6)}`,
                    province: '',
                    city: '',
                    district: ''
                };
            }
        });
    }
    /**
     * 打开位置选择器（用于getLocation失败时的兜底方案）
     * 注意：这个方法不会自动调用，需要用户主动触发
     */
    chooseLocation() {
        return __awaiter(this, void 0, void 0, function* () {
            return new Promise((resolve) => {
                wx.chooseLocation({
                    success: (res) => __awaiter(this, void 0, void 0, function* () {
                        try {
                            // 使用后端逆地理编码获取详细地址信息
                            const locationInfo = yield this.reverseGeocode(res.latitude, res.longitude);
                            // 合并用户选择的信息和逆地理编码结果
                            const finalInfo = Object.assign(Object.assign({}, locationInfo), { address: res.address || locationInfo.address });
                            logger_1.logger.info('用户选择位置成功', finalInfo, 'LocationService.chooseLocation');
                            resolve(finalInfo);
                        }
                        catch (err) {
                            // 逆地理编码失败，使用用户选择的基本信息
                            logger_1.logger.warn('逆地理编码失败，使用用户选择的信息', err, 'LocationService.chooseLocation');
                            resolve({
                                latitude: res.latitude,
                                longitude: res.longitude,
                                address: res.address || res.name,
                                province: '',
                                city: '',
                                district: '',
                                street: res.name
                            });
                        }
                    }),
                    fail: (err) => {
                        logger_1.logger.warn('用户取消选择位置', err, 'LocationService.chooseLocation');
                        resolve(null);
                    }
                });
            });
        });
    }
    /**
     * 获取完整的位置信息（位置+地址）
     */
    getFullLocationInfo() {
        return __awaiter(this, void 0, void 0, function* () {
            // 1. 获取经纬度
            const location = yield this.getCurrentLocation();
            // 2. 逆地理编码
            const locationInfo = yield this.reverseGeocode(location.latitude, location.longitude);
            return locationInfo;
        });
    }
    /**
     * 保存位置到全局状态
     */
    saveToGlobal(location) {
        try {
            const app = getApp();
            if (app && app.globalData) {
                app.globalData.latitude = location.latitude || null;
                app.globalData.longitude = location.longitude || null;
                app.globalData.location = {
                    name: location.address,
                    address: location.address,
                    province: location.province,
                    city: location.city,
                    district: location.district
                };
                logger_1.logger.info('位置信息已保存到全局状态', location, 'LocationService.saveToGlobal');
            }
        }
        catch (err) {
            logger_1.logger.error('保存位置到全局状态失败', err, 'LocationService.saveToGlobal');
        }
    }
    /**
     * 从全局状态读取位置
     */
    getFromGlobal() {
        try {
            const app = getApp();
            if (app && app.globalData && app.globalData.latitude && app.globalData.longitude) {
                const location = app.globalData.location;
                return {
                    latitude: app.globalData.latitude,
                    longitude: app.globalData.longitude,
                    address: (location === null || location === void 0 ? void 0 : location.address) || (location === null || location === void 0 ? void 0 : location.name) || '',
                    province: (location === null || location === void 0 ? void 0 : location.province) || '',
                    city: (location === null || location === void 0 ? void 0 : location.city) || '',
                    district: (location === null || location === void 0 ? void 0 : location.district) || ''
                };
            }
            return null;
        }
        catch (err) {
            logger_1.logger.error('从全局状态读取位置失败', err, 'LocationService.getFromGlobal');
            return null;
        }
    }
    /**
     * 检查位置权限
     */
    checkLocationPermission() {
        return __awaiter(this, void 0, void 0, function* () {
            return new Promise((resolve) => {
                wx.getSetting({
                    success: (res) => {
                        const hasPermission = res.authSetting['scope.userLocation'] === true;
                        logger_1.logger.debug('位置权限检查', { hasPermission }, 'LocationService.checkLocationPermission');
                        resolve(hasPermission);
                    },
                    fail: () => {
                        resolve(false);
                    }
                });
            });
        });
    }
    /**
     * 请求位置权限
     */
    requestLocationPermission() {
        return __awaiter(this, void 0, void 0, function* () {
            return new Promise((resolve) => {
                wx.authorize({
                    scope: 'scope.userLocation',
                    success: () => {
                        logger_1.logger.info('用户授予位置权限', undefined, 'LocationService.requestLocationPermission');
                        resolve(true);
                    },
                    fail: () => {
                        logger_1.logger.warn('用户拒绝位置权限', undefined, 'LocationService.requestLocationPermission');
                        resolve(false);
                    }
                });
            });
        });
    }
    /**
     * 获取位置信息（带权限检查）
     */
    getLocationWithPermission() {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                // 1. 检查权限
                const hasPermission = yield this.checkLocationPermission();
                if (!hasPermission) {
                    // 2. 请求权限
                    const granted = yield this.requestLocationPermission();
                    if (!granted) {
                        logger_1.logger.warn('用户未授予位置权限', undefined, 'LocationService.getLocationWithPermission');
                        return null;
                    }
                }
                // 3. 获取位置信息
                const locationInfo = yield this.getFullLocationInfo();
                // 4. 保存到全局
                this.saveToGlobal(locationInfo);
                return locationInfo;
            }
            catch (err) {
                logger_1.logger.error('获取位置信息失败', err, 'LocationService.getLocationWithPermission');
                return null;
            }
        });
    }
}
// 导出单例
exports.locationService = new LocationService();
/**
 * 生成设备ID
 */
function getDeviceId() {
    const STORAGE_KEY = 'device_id';
    // 优先使用缓存的device_id
    try {
        let deviceId = wx.getStorageSync(STORAGE_KEY);
        if (deviceId) {
            return deviceId;
        }
    }
    catch (err) {
        logger_1.logger.warn('读取缓存的device_id失败', err, 'getDeviceId');
    }
    // 生成新的device_id (使用时间戳+随机数)
    const deviceId = `mp_${Date.now()}_${Math.random().toString(36).substring(2, 11)}`;
    try {
        wx.setStorageSync(STORAGE_KEY, deviceId);
        logger_1.logger.info('生成新的device_id', { deviceId }, 'getDeviceId');
    }
    catch (err) {
        logger_1.logger.warn('保存device_id失败', err, 'getDeviceId');
    }
    return deviceId;
}
