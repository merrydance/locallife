import { request } from '../utils/request'

/**
 * 配送单响应 - 对齐 api.deliveryResponse
 */
export interface DeliveryResponse {
    id: number
    order_id: number
    rider_id?: number
    status: string
    pickup_address: string
    pickup_contact: string
    pickup_phone: string
    pickup_latitude: number
    pickup_longitude: number
    delivery_address: string
    delivery_contact: string
    delivery_phone: string
    delivery_latitude: number
    delivery_longitude: number
    distance: number
    delivery_fee: number
    rider_earnings?: number
    estimated_pickup_at?: string
    estimated_delivery_at?: string
    assigned_at?: string
    picked_at?: string
    delivered_at?: string
    completed_at?: string
    created_at: string
}

/**
 * 骑手位置响应 - 对齐 api.locationResponse
 */
export interface LocationResponse {
    latitude: number
    longitude: number
    accuracy: number
    speed: number
    heading: number
    recorded_at: string
}

/**
 * 获取骑手最新位置
 * GET /v1/delivery/:delivery_id/rider-location
 */
export function getRiderLocation(deliveryId: number) {
    return request<LocationResponse>({
        url: `/v1/delivery/${deliveryId}/rider-location`,
        method: 'GET'
    })
}

/**
 * 骑手确认取餐
 * POST /v1/delivery/:delivery_id/confirm-pickup
 */
export function confirmPickup(deliveryId: number) {
    return request<DeliveryResponse>({
        url: `/v1/delivery/${deliveryId}/confirm-pickup`,
        method: 'POST'
    })
}

/**
 * 骑手确认送达
 * POST /v1/delivery/:delivery_id/confirm-delivery
 */
export function confirmDelivery(deliveryId: number) {
    return request<DeliveryResponse>({
        url: `/v1/delivery/${deliveryId}/confirm-delivery`,
        method: 'POST'
    })
}

/**
 * 获取配送轨迹
 * GET /v1/delivery/:delivery_id/track
 */
export function getDeliveryTrack(deliveryId: number, since?: string) {
    return request<LocationResponse[]>({
        url: `/v1/delivery/${deliveryId}/track`,
        method: 'GET',
        data: since ? { since } : undefined
    })
}

/**
 * 获取配送单详情
 * GET /v1/delivery/:delivery_id
 */
export function getDeliveryDetail(deliveryId: number) {
    return request<DeliveryResponse>({
        url: `/v1/delivery/${deliveryId}`,
        method: 'GET'
    })
}

/**
 * 根据订单ID获取配送信息
 * GET /v1/delivery/order/:order_id
 */
export function getDeliveryByOrder(orderId: number) {
    return request<DeliveryResponse>({
        url: `/v1/delivery/order/${orderId}`,
        method: 'GET'
    })
}
