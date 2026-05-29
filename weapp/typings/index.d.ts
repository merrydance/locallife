/// <reference path="./types/index.d.ts" />



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
        currentRegion?: {
            id: number
            name: string
        }
        userRole: 'guest' | 'customer' | 'merchant' | 'rider' | 'operator'
        // 完整角色列表（小写），由 user_center 首次拉取后缓存，用于 onShow 快速恢复工作台
        userRoles?: string[]
        userWorkbenches?: import('../miniprogram/api/auth').UserWorkbenchResponse[]
        userId?: number
        merchantId?: string
        merchantName?: string
        // 多店铺切换支持
        currentMerchantId?: number
        merchantInfo?: {
            id: number
            name: string
            logo_url?: string
            is_open: boolean
            status: string
        }

        // (内部使用) 上次定位上下文
        _lastLocationContext?: {
            lat: number
            lng: number
            time: number
            name: string
            address?: string
        } | null
    }
    _hasHandledInitialShow?: boolean
    userInfoReadyCallback?: WechatMiniprogram.GetUserInfoSuccessCallback
    silentLogin(attempt?: number): void
    getLocation(): void
    getLocationCoordinates(): void
    reverseGeocodeWhenReady(retryCount?: number): Promise<void>
    isDemoMode(): boolean
    bootstrapDemoUser(): void
    reportErrorToMonitor(error: any, type: string): void
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    _applyUserInfo(user: any): void
    _refreshThenLoadUser(max: number, delays: number[]): void
    _doWxLogin(attempt: number, max: number, delays: number[]): void

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
