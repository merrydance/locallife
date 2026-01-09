"use strict";
/**
 * 地图服务
 * 提供地图相关功能：路线规划、坐标解码、标记创建等
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
exports.mapService = void 0;
const logger_1 = require("../utils/logger");
const request_1 = require("../utils/request");
const location_1 = require("../utils/location");
/**
 * 地图服务类
 */
class MapService {
    /**
     * 规划路线（使用后端代理的腾讯地图骑行路线API）
     * 后端接口：GET /v1/location/direction/bicycling
     * 参数：from=lat,lng&to=lat,lng&policy=0
     */
    planRoute(from_1, to_1) {
        return __awaiter(this, arguments, void 0, function* (from, to, policy = 0) {
            var _a, _b;
            try {
                const fromStr = `${from.latitude},${from.longitude}`;
                const toStr = `${to.latitude},${to.longitude}`;
                logger_1.logger.info('开始规划路线', {
                    from: fromStr,
                    to: toStr,
                    policy
                }, 'MapService.planRoute');
                // 调用后端代理接口
                const data = yield (0, request_1.request)({
                    url: '/v1/location/direction/bicycling',
                    method: 'GET',
                    data: {
                        from: fromStr,
                        to: toStr,
                        policy
                    }
                });
                // 后端返回包装格式，request() 会自动解包返回 data 部分
                // data 是腾讯地图的原始响应
                if (data.status === 0 && ((_b = (_a = data.result) === null || _a === void 0 ? void 0 : _a.routes) === null || _b === void 0 ? void 0 : _b[0])) {
                    const route = data.result.routes[0];
                    const points = this.decodePolyline(route.polyline);
                    const result = {
                        points,
                        distance: route.distance || 0,
                        duration: route.duration || 0
                    };
                    logger_1.logger.info('路线规划成功', {
                        distance: route.distance,
                        duration: route.duration,
                        pointsCount: points.length
                    }, 'MapService.planRoute');
                    return result;
                }
                else {
                    const errorMsg = data.message || '路线规划失败';
                    logger_1.logger.error('路线规划失败', data, 'MapService.planRoute');
                    throw new Error(errorMsg);
                }
            }
            catch (err) {
                logger_1.logger.error('路线规划请求失败', err, 'MapService.planRoute');
                throw err;
            }
        });
    }
    /**
     * 解码腾讯地图压缩坐标
     * 腾讯地图返回的坐标是压缩格式：
     * - 前两个是完整坐标
     * - 后面是增量值（单位：10^-6 度）
     * - 解压算法：coors[i] = coors[i-2] + coors[i] / 1000000
     */
    decodePolyline(coors) {
        if (!coors || coors.length < 2) {
            return [];
        }
        // 复制数组避免修改原数据
        const decoded = [...coors];
        // 从第3个元素开始解压（索引2）
        for (let i = 2; i < decoded.length; i++) {
            decoded[i] = decoded[i - 2] + decoded[i] / 1000000;
        }
        // 转换为 MapPoint 数组
        const points = [];
        for (let i = 0; i < decoded.length; i += 2) {
            const lat = decoded[i];
            const lng = decoded[i + 1];
            // 验证坐标范围
            if (lat >= -90 && lat <= 90 && lng >= -180 && lng <= 180) {
                points.push({ latitude: lat, longitude: lng });
            }
            else {
                logger_1.logger.warn('跳过无效坐标', { lat, lng }, 'MapService.decodePolyline');
            }
        }
        logger_1.logger.debug('坐标解码完成', {
            originalCount: coors.length / 2,
            decodedCount: points.length
        }, 'MapService.decodePolyline');
        return points;
    }
    /**
     * 创建地图标记
     */
    createMarker(id, point, label, iconPath, options) {
        return {
            id,
            latitude: point.latitude,
            longitude: point.longitude,
            width: (options === null || options === void 0 ? void 0 : options.width) || 40,
            height: (options === null || options === void 0 ? void 0 : options.height) || 40,
            iconPath,
            callout: {
                content: label,
                color: (options === null || options === void 0 ? void 0 : options.calloutColor) || '#333',
                fontSize: 14,
                padding: 6,
                borderRadius: 12,
                display: 'ALWAYS',
                bgColor: (options === null || options === void 0 ? void 0 : options.calloutBgColor) || '#fff'
            }
        };
    }
    /**
     * 调整地图视野以包含所有点
     */
    adjustMapView(mapId, points, padding) {
        if (!points || points.length === 0) {
            logger_1.logger.warn('没有点需要调整视野', undefined, 'MapService.adjustMapView');
            return;
        }
        const mapCtx = wx.createMapContext(mapId);
        mapCtx.includePoints({
            points,
            padding: padding || [80, 40, 80, 40]
        });
        logger_1.logger.debug('地图视野已调整', {
            pointsCount: points.length
        }, 'MapService.adjustMapView');
    }
    /**
     * 创建路线（折线）
     */
    createPolyline(points, options) {
        return {
            points,
            color: (options === null || options === void 0 ? void 0 : options.color) || '#1d63ff',
            width: (options === null || options === void 0 ? void 0 : options.width) || 6,
            dottedLine: (options === null || options === void 0 ? void 0 : options.dottedLine) || false,
            arrowLine: (options === null || options === void 0 ? void 0 : options.arrowLine) || false
        };
    }
    /**
     * 逆地理编码（坐标转地址）
     * 使用后端代理接口
     */
    reverseGeocode(point) {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const locationInfo = yield location_1.locationService.reverseGeocode(point.latitude, point.longitude);
                const address = locationInfo.street || locationInfo.district || locationInfo.address;
                logger_1.logger.info('逆地理编码成功', { address }, 'MapService.reverseGeocode');
                return address;
            }
            catch (err) {
                logger_1.logger.error('逆地理编码失败', err, 'MapService.reverseGeocode');
                throw err;
            }
        });
    }
    /**
     * 计算两点之间的直线距离（米）
     */
    calculateDistance(point1, point2) {
        const R = 6371000; // 地球半径（米）
        const lat1 = point1.latitude * Math.PI / 180;
        const lat2 = point2.latitude * Math.PI / 180;
        const deltaLat = (point2.latitude - point1.latitude) * Math.PI / 180;
        const deltaLng = (point2.longitude - point1.longitude) * Math.PI / 180;
        const a = Math.sin(deltaLat / 2) * Math.sin(deltaLat / 2) +
            Math.cos(lat1) * Math.cos(lat2) *
                Math.sin(deltaLng / 2) * Math.sin(deltaLng / 2);
        const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
        return Math.round(R * c);
    }
    /**
     * 格式化距离显示
     */
    formatDistance(meters) {
        if (meters < 1000) {
            return `${meters}米`;
        }
        return `${(meters / 1000).toFixed(1)}公里`;
    }
    /**
     * 格式化时长显示
     */
    formatDuration(seconds) {
        if (seconds < 60) {
            return `${seconds}秒`;
        }
        const minutes = Math.floor(seconds / 60);
        if (minutes < 60) {
            return `${minutes}分钟`;
        }
        const hours = Math.floor(minutes / 60);
        const remainMinutes = minutes % 60;
        return `${hours}小时${remainMinutes}分钟`;
    }
}
// 导出单例
exports.mapService = new MapService();
