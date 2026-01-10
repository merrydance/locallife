/// <reference path="./types/index.d.ts" />

/** 设备平台信息（用于跨平台适配） */
interface DevicePlatformInfo {
    type: string          // platform 原始值：android, ios, ohos, windows, mac, ohos_pc, devtools
    isAndroid: boolean    // Android 手机
    isIos: boolean        // iOS 设备
    isOhos: boolean       // 鸿蒙 Next 手机
    isPc: boolean         // PC 端（Windows/Mac/鸿蒙PC）
    isDevtools: boolean   // 开发者工具
}

interface IAppOption {
    globalData: {
        userInfo: WechatMiniprogram.UserInfo | null
        // location is an object with extended address information
        location: {
            name?: string
            address?: string
            province?: string
            city?: string
            district?: string
        }
        latitude: number | null
        longitude: number | null
        userRole: 'guest' | 'customer' | 'merchant' | 'rider' | 'operator'
        userId?: string
        merchantId?: string
        // 多店铺切换支持
        currentMerchantId?: string
        merchantInfo?: {
            id: string
            name: string
            logo_url?: string
            is_open: boolean
            status: string
        }
        // 设备平台信息（用于跨平台适配）
        devicePlatform: DevicePlatformInfo | null
        // (内部使用) 上次定位上下文
        _lastLocationContext?: {
            lat: number
            lng: number
            time: number
            name: string
            address?: string
        } | null
    }
    userInfoReadyCallback?: WechatMiniprogram.GetUserInfoSuccessCallback
    silentLogin(): void
    getLocation(): void
    getLocationCoordinates(): void
    reverseGeocodeWhenReady(retryCount?: number): Promise<void>
    isDemoMode(): boolean
    bootstrapDemoUser(): void
    reportErrorToMonitor(error: any, type: string): void
    clearApiCache(): void
    initDevicePlatform(): void
}

// 微信小程序 Performance API 扩展
interface WxPerformance extends Performance {
    memory?: {
        jsHeapSizeUsed: number
        jsHeapSizeLimit: number
        totalJSHeapSize: number
    }
}

// 全局类型扩展
declare const global: {
    perfMonitor?: unknown
} & Record<string, unknown>
