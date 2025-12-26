"use strict";
/**
 * 地理位置工具函数
 * 用于前端计算距离和配送时间
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
exports.haversineDistance = haversineDistance;
exports.estimateDeliveryTime = estimateDeliveryTime;
exports.isValidCoordinate = isValidCoordinate;
exports.getUserLocation = getUserLocation;
exports.enrichMerchantsWithDistance = enrichMerchantsWithDistance;
const logger_1 = require("./logger");
const EARTH_RADIUS_KM = 6371; // 地球半径（千米）
const AVERAGE_DELIVERY_SPEED_KMH = 20; // 平均配送速度（公里/小时）
const ROAD_FACTOR = 1.3; // 路况系数（城市道路实际速度更慢）
const BASE_PREP_TIME_MINUTES = 15; // 基础备餐时间（分钟）
/**
 * 角度转弧度
 */
function toRadians(degrees) {
    return degrees * Math.PI / 180;
}
/**
 * 使用 Haversine 公式计算两点之间的距离（千米）
 * @param lat1 起点纬度
 * @param lng1 起点经度
 * @param lat2 终点纬度
 * @param lng2 终点经度
 * @returns 距离（千米）
 */
function haversineDistance(lat1, lng1, lat2, lng2) {
    const dLat = toRadians(lat2 - lat1);
    const dLng = toRadians(lng2 - lng1);
    const a = Math.sin(dLat / 2) * Math.sin(dLat / 2) +
        Math.cos(toRadians(lat1)) *
            Math.cos(toRadians(lat2)) *
            Math.sin(dLng / 2) *
            Math.sin(dLng / 2);
    const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
    return EARTH_RADIUS_KM * c;
}
/**
 * 估算配送时间（分钟）
 * @param distanceKm 距离（千米）
 * @param prepMinutes 备餐时间（分钟）
 * @returns 预计配送时间（分钟）
 */
function estimateDeliveryTime(distanceKm, prepMinutes = BASE_PREP_TIME_MINUTES) {
    if (distanceKm <= 0) {
        return prepMinutes;
    }
    // 配送时间 = 距离 / 速度 * 60（转换为分钟）
    const deliveryMinutes = (distanceKm / AVERAGE_DELIVERY_SPEED_KMH) * 60 * ROAD_FACTOR;
    // 总时间 = 备餐时间 + 配送时间
    const totalMinutes = prepMinutes + deliveryMinutes;
    // 向上取整到5分钟
    return Math.ceil(totalMinutes / 5) * 5;
}
/**
 * 验证坐标是否有效
 */
function isValidCoordinate(lat, lng) {
    return lat >= -90 && lat <= 90 && lng >= -180 && lng <= 180;
}
/**
 * 获取用户当前位置
 * @returns Promise<{latitude: number, longitude: number}>
 */
function getUserLocation() {
    return new Promise((resolve, reject) => {
        // 优先尝试获取 app.globalData 中的缓存位置
        const app = getApp();
        if (app && app.globalData && typeof app.globalData.latitude === 'number' && typeof app.globalData.longitude === 'number') {
            logger_1.logger.debug('使用缓存位置', {
                latitude: app.globalData.latitude,
                longitude: app.globalData.longitude
            }, 'getUserLocation');
            resolve({
                latitude: app.globalData.latitude,
                longitude: app.globalData.longitude
            });
            return;
        }
        wx.getLocation({
            type: 'gcj02', // 国测局坐标系（中国强制使用）
            success: (res) => {
                // 更新全局缓存
                if (app && app.globalData) {
                    app.globalData.latitude = res.latitude;
                    app.globalData.longitude = res.longitude;
                }
                logger_1.logger.info('位置获取成功', {
                    latitude: res.latitude,
                    longitude: res.longitude
                }, 'getUserLocation');
                resolve({
                    latitude: res.latitude,
                    longitude: res.longitude
                });
            },
            fail: (err) => {
                logger_1.logger.warn('获取位置失败', err, 'getUserLocation');
                reject(err);
            }
        });
    });
}
/**
 * 为商户列表添加距离和 ETA 信息
 * @param merchants 商户列表
 * @returns 包含距离和 ETA 的商户列表
 */
function enrichMerchantsWithDistance(merchants) {
    return __awaiter(this, void 0, void 0, function* () {
        try {
            const userLocation = yield getUserLocation();
            return merchants.map((m) => {
                if (!m.merchant_latitude || !m.merchant_longitude) {
                    return Object.assign(Object.assign({}, m), { distance_km: 0, distance_meters: 0, delivery_time_minutes: m.prep_minutes || BASE_PREP_TIME_MINUTES });
                }
                const distanceKm = haversineDistance(userLocation.latitude, userLocation.longitude, m.merchant_latitude, m.merchant_longitude);
                const etaMinutes = estimateDeliveryTime(distanceKm, m.prep_minutes || BASE_PREP_TIME_MINUTES);
                return Object.assign(Object.assign({}, m), { distance_km: Math.round(distanceKm * 100) / 100, distance_meters: Math.round(distanceKm * 1000), delivery_time_minutes: etaMinutes });
            });
        }
        catch (error) {
            logger_1.logger.warn('无法获取用户位置,返回原始数据', error, 'enrichMerchantsWithDistance');
            // 如果获取位置失败，返回原始数据
            return merchants.map((m) => (Object.assign(Object.assign({}, m), { distance_km: 0, distance_meters: 0, delivery_time_minutes: m.prep_minutes || BASE_PREP_TIME_MINUTES })));
        }
    });
}
