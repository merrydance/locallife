import { request } from '../utils/request'

export interface RecommendedOrder {
    order_id: number
    merchant_id: number
    merchant_name?: string
    merchant_address?: string
    customer_address?: string
    item_count?: number
    total_score: number
    distance_to_pickup: number
    real_distance?: number
    estimated_minutes: number
    delivery_fee: number
    distance: number // Merchant to Customer
    pickup_longitude: number
    pickup_latitude: number
    delivery_longitude: number
    delivery_latitude: number
    expires_at: string
}

export interface Delivery {
    id: number
    order_id: number
    order_no?: string
    rider_id?: number
    merchant_name?: string
    pickup_address: string
    pickup_longitude: number
    pickup_latitude: number
    pickup_contact?: string
    pickup_phone?: string
    delivery_address: string
    delivery_longitude: number
    delivery_latitude: number
    delivery_contact?: string
    delivery_phone?: string
    status: 'pending' | 'assigned' | 'picking' | 'picked' | 'delivering' | 'delivered' | 'completed' | 'cancelled' | 'exception'
    delivery_fee: number
    rider_earnings: number
    freeze_amount?: number
    item_count?: number
    estimated_pickup_at?: string
    estimated_delivery_at?: string
    picked_at?: string
    delivered_at?: string
    created_at?: string
    assigned_at?: string
    notes?: string
}

export class DeliveryService {
    /**
     * 获取推荐接单列表 (抢单池)
     */
    static async getRecommendedOrders(lng: number, lat: number): Promise<RecommendedOrder[]> {
        return await request({
            url: '/v1/delivery/recommend',
            method: 'GET',
            data: { longitude: lng, latitude: lat }
        })
    }

    /**
     * 抢单
     */
    static async grabOrder(orderId: number): Promise<Delivery> {
        return await request({
            url: `/v1/delivery/grab/${orderId}`,
            method: 'POST'
        })
    }

    /**
     * 开始取餐 (前往商家)
     */
    static async startPickup(deliveryId: number): Promise<Delivery> {
        return await request({
            url: `/v1/delivery/${deliveryId}/start-pickup`,
            method: 'POST'
        })
    }

    /**
     * 确认取餐 (已拿到餐品)
     */
    static async confirmPickup(deliveryId: number): Promise<Delivery> {
        return await request({
            url: `/v1/delivery/${deliveryId}/confirm-pickup`,
            method: 'POST'
        })
    }

    /**
     * 开始送餐 (前往客户)
     */
    static async startDelivery(deliveryId: number): Promise<Delivery> {
        return await request({
            url: `/v1/delivery/${deliveryId}/start-delivery`,
            method: 'POST'
        })
    }

    /**
     * 确认送达
     */
    static async confirmDelivery(deliveryId: number): Promise<Delivery> {
        return await request({
            url: `/v1/delivery/${deliveryId}/confirm-delivery`,
            method: 'POST'
        })
    }

    /**
     * 获取详情 (通过订单ID)
     */
    static async getDeliveryByOrder(orderId: number): Promise<Delivery> {
        return await request({
            url: `/v1/delivery/order/${orderId}`,
            method: 'GET'
        })
    }

    /**
     * 获取骑手位置
     */
    static async getRiderLocation(deliveryId: number): Promise<{ latitude: number; longitude: number }> {
        return await request({
            url: `/v1/delivery/${deliveryId}/rider-location`,
            method: 'GET'
        })
    }
}

export type DeliveryResponse = Delivery;

export default DeliveryService
