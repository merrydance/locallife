/**
 * 地图服务
 * 提供地图相关功能：路线规划、坐标解码、标记创建等
 */

import { logger } from '../utils/logger'
import { request } from '../utils/request'
import { locationService } from '../utils/location'

/**
 * 地图点坐标
 */
export interface MapPoint {
    latitude: number
    longitude: number
}

/**
 * 路线规划结果
 */
export interface RouteResult {
    points: MapPoint[]
    distance: number  // 距离（米）
    duration: number  // 时长（秒）
}

/**
 * 地图标记
 */
export interface MapMarker {
    id: number
    latitude: number
    longitude: number
    width: number
    height: number
    iconPath: string
    callout: {
        content: string
        color: string
        fontSize: number
        padding: number
        borderRadius: number
        display: 'ALWAYS'
        bgColor: string
    }
}

/**
 * 地图路线
 */
export interface MapPolyline {
    points: MapPoint[]
    color: string
    width: number
    dottedLine?: boolean
    arrowLine?: boolean
}

/**
 * 地图服务类
 */
class MapService {
    /**
     * 规划路线（使用后端代理的腾讯地图骑行路线API）
     * 后端接口：GET /v1/location/direction/bicycling
     * 参数：from=lat,lng&to=lat,lng&policy=0
     */
    async planRoute(from: MapPoint, to: MapPoint, policy: number = 0): Promise<RouteResult> {
        try {
            const fromStr = `${from.latitude},${from.longitude}`
            const toStr = `${to.latitude},${to.longitude}`

            logger.info('开始规划路线', {
                from: fromStr,
                to: toStr,
                policy
            }, 'MapService.planRoute')

            // 调用后端代理接口
            const data = await request<any>({
                url: '/v1/location/direction/bicycling',
                method: 'GET',
                data: {
                    from: fromStr,
                    to: toStr,
                    policy
                }
            })

            // 后端返回包装格式，request() 会自动解包返回 data 部分
            // data 是腾讯地图的原始响应
            if (data.status === 0 && data.result?.routes?.[0]) {
                const route = data.result.routes[0]
                const points = this.decodePolyline(route.polyline)

                const result = {
                    points,
                    distance: route.distance || 0,
                    duration: route.duration || 0
                }

                logger.info('路线规划成功', {
                    distance: route.distance,
                    duration: route.duration,
                    pointsCount: points.length
                }, 'MapService.planRoute')

                return result
            } else {
                const errorMsg = data.message || '路线规划失败'
                logger.error('路线规划失败', data, 'MapService.planRoute')
                throw new Error(errorMsg)
            }
        } catch (err) {
            logger.error('路线规划请求失败', err, 'MapService.planRoute')
            throw err
        }
    }

    /**
     * 解码腾讯地图压缩坐标
     * 腾讯地图返回的坐标是压缩格式：
     * - 前两个是完整坐标
     * - 后面是增量值（单位：10^-6 度）
     * - 解压算法：coors[i] = coors[i-2] + coors[i] / 1000000
     */
    decodePolyline(coors: number[]): MapPoint[] {
        if (!coors || coors.length < 2) {
            return []
        }

        // 复制数组避免修改原数据
        const decoded = [...coors]

        // 从第3个元素开始解压（索引2）
        for (let i = 2; i < decoded.length; i++) {
            decoded[i] = decoded[i - 2] + decoded[i] / 1000000
        }

        // 转换为 MapPoint 数组
        const points: MapPoint[] = []
        for (let i = 0; i < decoded.length; i += 2) {
            const lat = decoded[i]
            const lng = decoded[i + 1]

            // 验证坐标范围
            if (lat >= -90 && lat <= 90 && lng >= -180 && lng <= 180) {
                points.push({ latitude: lat, longitude: lng })
            } else {
                logger.warn('跳过无效坐标', { lat, lng }, 'MapService.decodePolyline')
            }
        }

        logger.debug('坐标解码完成', {
            originalCount: coors.length / 2,
            decodedCount: points.length
        }, 'MapService.decodePolyline')

        return points
    }

    /**
     * 创建地图标记
     */
    createMarker(
        id: number,
        point: MapPoint,
        label: string,
        iconPath: string,
        options?: {
            width?: number
            height?: number
            calloutColor?: string
            calloutBgColor?: string
        }
    ): MapMarker {
        return {
            id,
            latitude: point.latitude,
            longitude: point.longitude,
            width: options?.width || 40,
            height: options?.height || 40,
            iconPath,
            callout: {
                content: label,
                color: options?.calloutColor || '#333',
                fontSize: 14,
                padding: 6,
                borderRadius: 12,
                display: 'ALWAYS',
                bgColor: options?.calloutBgColor || '#fff'
            }
        }
    }

    /**
     * 调整地图视野以包含所有点
     */
    adjustMapView(mapId: string, points: MapPoint[], padding?: number[]): void {
        if (!points || points.length === 0) {
            logger.warn('没有点需要调整视野', undefined, 'MapService.adjustMapView')
            return
        }

        const mapCtx = wx.createMapContext(mapId)
        mapCtx.includePoints({
            points,
            padding: padding || [80, 40, 80, 40]
        })

        logger.debug('地图视野已调整', {
            pointsCount: points.length
        }, 'MapService.adjustMapView')
    }

    /**
     * 创建路线（折线）
     */
    createPolyline(
        points: MapPoint[],
        options?: {
            color?: string
            width?: number
            dottedLine?: boolean
            arrowLine?: boolean
        }
    ): MapPolyline {
        return {
            points,
            color: options?.color || '#1d63ff',
            width: options?.width || 6,
            dottedLine: options?.dottedLine || false,
            arrowLine: options?.arrowLine || false
        }
    }

    /**
     * 逆地理编码（坐标转地址）
     * 使用后端代理接口
     */
    async reverseGeocode(point: MapPoint): Promise<string> {
        try {
            const locationInfo = await locationService.reverseGeocode(point.latitude, point.longitude)
            const address = locationInfo.street || locationInfo.district || locationInfo.address
            logger.info('逆地理编码成功', { address }, 'MapService.reverseGeocode')
            return address
        } catch (err) {
            logger.error('逆地理编码失败', err, 'MapService.reverseGeocode')
            throw err
        }
    }

    /**
     * 计算两点之间的直线距离（米）
     */
    calculateDistance(point1: MapPoint, point2: MapPoint): number {
        const R = 6371000 // 地球半径（米）
        const lat1 = point1.latitude * Math.PI / 180
        const lat2 = point2.latitude * Math.PI / 180
        const deltaLat = (point2.latitude - point1.latitude) * Math.PI / 180
        const deltaLng = (point2.longitude - point1.longitude) * Math.PI / 180

        const a = Math.sin(deltaLat / 2) * Math.sin(deltaLat / 2) +
            Math.cos(lat1) * Math.cos(lat2) *
            Math.sin(deltaLng / 2) * Math.sin(deltaLng / 2)
        const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a))

        return Math.round(R * c)
    }

    /**
     * 格式化距离显示
     */
    formatDistance(meters: number): string {
        if (meters < 1000) {
            return `${meters}米`
        }
        return `${(meters / 1000).toFixed(1)}公里`
    }

    /**
     * 格式化时长显示
     */
    formatDuration(seconds: number): string {
        if (seconds < 60) {
            return `${seconds}秒`
        }
        const minutes = Math.floor(seconds / 60)
        if (minutes < 60) {
            return `${minutes}分钟`
        }
        const hours = Math.floor(minutes / 60)
        const remainMinutes = minutes % 60
        return `${hours}小时${remainMinutes}分钟`
    }
}

// 导出单例
export const mapService = new MapService()
