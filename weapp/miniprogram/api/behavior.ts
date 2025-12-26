import { request } from '../utils/request'

// 基于swagger.json的行为追踪接口

/**
 * 行为追踪请求 - 对齐 api.trackBehaviorRequest
 */
export interface TrackBehaviorRequest extends Record<string, unknown> {
    behavior_type: 'view' | 'detail' | 'cart' | 'purchase'  // 行为类型枚举
    merchant_id?: number  // 商户ID，最小值1
    dish_id?: number  // 菜品ID，最小值1
    combo_id?: number  // 套餐ID，最小值1
    duration?: number  // 停留时长（秒），最大24小时
}

/**
 * 行为追踪响应
 */
export interface TrackBehaviorResponse {
    success: boolean
    message?: string
}

/**
 * 上报用户行为埋点 - POST /v1/behaviors/track
 */
export function trackBehavior(data: TrackBehaviorRequest) {
    return request<TrackBehaviorResponse>({
        url: '/v1/behaviors/track',
        method: 'POST',
        data
    })
}

/**
 * 便捷方法：追踪浏览行为
 */
export function trackView(params: {
    merchant_id?: number
    dish_id?: number
    combo_id?: number
    duration?: number
}) {
    return trackBehavior({
        behavior_type: 'view',
        ...params
    })
}

/**
 * 便捷方法：追踪详情查看行为
 */
export function trackDetail(params: {
    merchant_id?: number
    dish_id?: number
    combo_id?: number
    duration?: number
}) {
    return trackBehavior({
        behavior_type: 'detail',
        ...params
    })
}

/**
 * 便捷方法：追踪加购行为
 */
export function trackAddToCart(params: {
    merchant_id?: number
    dish_id?: number
    combo_id?: number
}) {
    return trackBehavior({
        behavior_type: 'cart',
        ...params
    })
}

/**
 * 便捷方法：追踪购买行为
 */
export function trackPurchase(params: {
    merchant_id?: number
    dish_id?: number
    combo_id?: number
}) {
    return trackBehavior({
        behavior_type: 'purchase',
        ...params
    })
}