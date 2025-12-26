/**
 * 位置服务工具类
 * 通过后端接口调用腾讯LBS获取用户位置信息
 */

import { logger } from './logger'
import { request } from './request'

/**
 * 位置信息接口（对齐后端 API 响应）
 */
export interface LocationInfo {
    latitude?: number        // 纬度（可选，因为后端可能不返回）
    longitude?: number       // 经度（可选，因为后端可能不返回）
    address: string         // 地址
    formatted_address?: string // 格式化地址（完整地址）
    province: string        // 省份
    city: string           // 城市
    district: string       // 区县
    street?: string        // 街道
    street_number?: string // 门牌号
}

/**
 * 位置服务类
 */
class LocationService {
    /**
     * 获取当前位置（经纬度）
     */
    async getCurrentLocation(): Promise<{ latitude: number; longitude: number }> {
        return new Promise((resolve, reject) => {
            wx.getLocation({
                type: 'gcj02', // 返回可以用于wx.openLocation的坐标
                success: (res) => {
                    logger.info('获取位置成功', {
                        latitude: res.latitude,
                        longitude: res.longitude
                    }, 'LocationService.getCurrentLocation')
                    resolve({
                        latitude: res.latitude,
                        longitude: res.longitude
                    })
                },
                fail: (err) => {
                    logger.warn('获取位置失败', err, 'LocationService.getCurrentLocation')
                    reject(err)
                }
            })
        })
    }

    /**
     * 逆地理编码 - 通过后端接口将经纬度转换为地址
     * 后端接口: GET /v1/location/reverse-geocode?latitude=xxx&longitude=xxx
     */
    async reverseGeocode(latitude: number, longitude: number): Promise<LocationInfo> {
        try {
            const response = await request<LocationInfo>({
                url: '/v1/location/reverse-geocode',
                method: 'GET',
                data: {
                    latitude,
                    longitude
                }
            })

            logger.info('逆地理编码成功', response, 'LocationService.reverseGeocode')

            // 补充经纬度信息（后端可能不返回）
            if (!response.latitude) response.latitude = latitude
            if (!response.longitude) response.longitude = longitude

            return response
        } catch (err) {
            logger.error('逆地理编码失败', err, 'LocationService.reverseGeocode')
            // 即使逆地理编码失败，也返回基本的位置信息
            return {
                latitude,
                longitude,
                address: `${latitude.toFixed(6)}, ${longitude.toFixed(6)}`,
                province: '',
                city: '',
                district: ''
            }
        }
    }

    /**
     * 打开位置选择器（用于getLocation失败时的兜底方案）
     * 注意：这个方法不会自动调用，需要用户主动触发
     */
    async chooseLocation(): Promise<LocationInfo | null> {
        return new Promise((resolve) => {
            wx.chooseLocation({
                success: async (res) => {
                    try {
                        // 使用后端逆地理编码获取详细地址信息
                        const locationInfo = await this.reverseGeocode(res.latitude, res.longitude)

                        // 合并用户选择的信息和逆地理编码结果
                        const finalInfo: LocationInfo = {
                            ...locationInfo,
                            address: res.address || locationInfo.address
                        }

                        logger.info('用户选择位置成功', finalInfo, 'LocationService.chooseLocation')
                        resolve(finalInfo)
                    } catch (err) {
                        // 逆地理编码失败，使用用户选择的基本信息
                        logger.warn('逆地理编码失败，使用用户选择的信息', err, 'LocationService.chooseLocation')
                        resolve({
                            latitude: res.latitude,
                            longitude: res.longitude,
                            address: res.address || res.name,
                            province: '',
                            city: '',
                            district: '',
                            street: res.name
                        })
                    }
                },
                fail: (err) => {
                    logger.warn('用户取消选择位置', err, 'LocationService.chooseLocation')
                    resolve(null)
                }
            })
        })
    }

    /**
     * 获取完整的位置信息（位置+地址）
     */
    async getFullLocationInfo(): Promise<LocationInfo> {
        // 1. 获取经纬度
        const location = await this.getCurrentLocation()

        // 2. 逆地理编码
        const locationInfo = await this.reverseGeocode(location.latitude, location.longitude)

        return locationInfo
    }

    /**
     * 保存位置到全局状态
     */
    saveToGlobal(location: LocationInfo): void {
        try {
            const app = getApp<IAppOption>()
            if (app && app.globalData) {
                app.globalData.latitude = location.latitude || null
                app.globalData.longitude = location.longitude || null
                app.globalData.location = {
                    name: location.address,
                    address: location.address,
                    province: location.province,
                    city: location.city,
                    district: location.district
                }
                logger.info('位置信息已保存到全局状态', location, 'LocationService.saveToGlobal')
            }
        } catch (err) {
            logger.error('保存位置到全局状态失败', err, 'LocationService.saveToGlobal')
        }
    }

    /**
     * 从全局状态读取位置
     */
    getFromGlobal(): LocationInfo | null {
        try {
            const app = getApp<IAppOption>()
            if (app && app.globalData && app.globalData.latitude && app.globalData.longitude) {
                const location = app.globalData.location as any
                return {
                    latitude: app.globalData.latitude,
                    longitude: app.globalData.longitude,
                    address: location?.address || location?.name || '',
                    province: location?.province || '',
                    city: location?.city || '',
                    district: location?.district || ''
                }
            }
            return null
        } catch (err) {
            logger.error('从全局状态读取位置失败', err, 'LocationService.getFromGlobal')
            return null
        }
    }

    /**
     * 检查位置权限
     */
    async checkLocationPermission(): Promise<boolean> {
        return new Promise((resolve) => {
            wx.getSetting({
                success: (res) => {
                    const hasPermission = res.authSetting['scope.userLocation'] === true
                    logger.debug('位置权限检查', { hasPermission }, 'LocationService.checkLocationPermission')
                    resolve(hasPermission)
                },
                fail: () => {
                    resolve(false)
                }
            })
        })
    }

    /**
     * 请求位置权限
     */
    async requestLocationPermission(): Promise<boolean> {
        return new Promise((resolve) => {
            wx.authorize({
                scope: 'scope.userLocation',
                success: () => {
                    logger.info('用户授予位置权限', undefined, 'LocationService.requestLocationPermission')
                    resolve(true)
                },
                fail: () => {
                    logger.warn('用户拒绝位置权限', undefined, 'LocationService.requestLocationPermission')
                    resolve(false)
                }
            })
        })
    }

    /**
     * 获取位置信息（带权限检查）
     */
    async getLocationWithPermission(): Promise<LocationInfo | null> {
        try {
            // 1. 检查权限
            const hasPermission = await this.checkLocationPermission()

            if (!hasPermission) {
                // 2. 请求权限
                const granted = await this.requestLocationPermission()
                if (!granted) {
                    logger.warn('用户未授予位置权限', undefined, 'LocationService.getLocationWithPermission')
                    return null
                }
            }

            // 3. 获取位置信息
            const locationInfo = await this.getFullLocationInfo()

            // 4. 保存到全局
            this.saveToGlobal(locationInfo)

            return locationInfo
        } catch (err) {
            logger.error('获取位置信息失败', err, 'LocationService.getLocationWithPermission')
            return null
        }
    }
}

// 导出单例
export const locationService = new LocationService()

/**
 * 生成设备ID
 */
export function getDeviceId(): string {
    const STORAGE_KEY = 'device_id'

    // 优先使用缓存的device_id
    try {
        let deviceId = wx.getStorageSync(STORAGE_KEY)
        if (deviceId) {
            return deviceId
        }
    } catch (err) {
        logger.warn('读取缓存的device_id失败', err, 'getDeviceId')
    }

    // 生成新的device_id (使用时间戳+随机数)
    const deviceId = `mp_${Date.now()}_${Math.random().toString(36).substring(2, 11)}`

    try {
        wx.setStorageSync(STORAGE_KEY, deviceId)
        logger.info('生成新的device_id', { deviceId }, 'getDeviceId')
    } catch (err) {
        logger.warn('保存device_id失败', err, 'getDeviceId')
    }

    return deviceId
}
