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
    rider_gross_amount?: number
    rider_payment_fee?: number
    rider_net_earnings?: number
    profit_sharing_order_id?: number
    profit_sharing_status?: string
    freeze_amount?: number
    item_count?: number
    estimated_pickup_at?: string
    estimated_delivery_at?: string
    picked_at?: string
    delivered_at?: string
    completed_at?: string
    created_at?: string
    assigned_at?: string
    location_updated_at?: string
    notes?: string
    items?: Array<{
        name: string
        quantity: number
    }>
}

export interface DeliveryLocationPoint {
    latitude: number
    longitude: number
    accuracy?: number
    speed?: number
    heading?: number
    recorded_at: string
}

export type DeliveryStatusTheme = 'success' | 'warning' | 'danger' | 'primary' | 'default'

export interface DeliveryProgressView {
    title: string
    time: string
    done: boolean
    active: boolean
}

export function getDeliveryStatusDisplay(status?: Delivery['status']) {
    const isAssignedStage = status === 'assigned' || status === 'picking'
    const isPickedStage = status === 'picked'
    const isDeliveringStage = status === 'delivering'
    const isDeliveredStage = status === 'delivered' || status === 'completed'

    const statusMap: Record<string, { text: string, theme: DeliveryStatusTheme }> = {
        pending: { text: '等待骑手接单', theme: 'default' },
        assigned: { text: '骑手已接单', theme: 'primary' },
        picking: { text: '骑手正在取餐', theme: 'primary' },
        picked: { text: '骑手已取餐', theme: 'primary' },
        delivering: { text: '骑手正在代取', theme: 'primary' },
        delivered: { text: '已送达', theme: 'success' },
        completed: { text: '已送达', theme: 'success' },
        cancelled: { text: '代取已取消', theme: 'warning' },
        exception: { text: '代取异常', theme: 'danger' }
    }

    const meta = statusMap[status || ''] || { text: status || '', theme: 'default' as const }

    return {
        text: meta.text,
        theme: meta.theme,
        isAssignedStage,
        isPickedStage,
        isDeliveringStage,
        isDeliveredStage,
        isLocationTracked: isPickedStage || isDeliveringStage,
        canConfirmReceipt: status === 'delivered'
    }
}

export function buildDeliveryProgress(delivery: Delivery, formatTime: (timeStr: string) => string): DeliveryProgressView[] {
    const statusDisplay = getDeliveryStatusDisplay(delivery.status)
    const isAtLeastAssigned = statusDisplay.isAssignedStage || statusDisplay.isPickedStage || statusDisplay.isDeliveringStage || statusDisplay.isDeliveredStage
    const isAtLeastPicked = statusDisplay.isPickedStage || statusDisplay.isDeliveringStage || statusDisplay.isDeliveredStage
    const isAtLeastDelivering = statusDisplay.isDeliveringStage || statusDisplay.isDeliveredStage

    return [
        {
            title: '商家已接单',
            time: delivery.created_at ? formatTime(delivery.created_at) : '',
            done: true,
            active: false
        },
        {
            title: '骑手已接单',
            time: delivery.assigned_at ? formatTime(delivery.assigned_at) : '',
            done: !!delivery.assigned_at || isAtLeastAssigned,
            active: statusDisplay.isAssignedStage
        },
        {
            title: '骑手已取餐',
            time: delivery.picked_at ? formatTime(delivery.picked_at) : '',
            done: !!delivery.picked_at || isAtLeastPicked,
            active: statusDisplay.isPickedStage
        },
        {
            title: '代取中',
            time: '',
            done: isAtLeastDelivering,
            active: statusDisplay.isDeliveringStage
        },
        {
            title: '已送达',
            time: delivery.delivered_at ? formatTime(delivery.delivered_at) : '',
            done: !!delivery.delivered_at || statusDisplay.isDeliveredStage,
            active: statusDisplay.isDeliveredStage
        }
    ]
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
    static async getRiderLocation(deliveryId: number): Promise<DeliveryLocationPoint> {
        return await request({
            url: `/v1/delivery/${deliveryId}/rider-location`,
            method: 'GET'
        })
    }

    /**
     * 获取代取轨迹
     */
    static async getDeliveryTrack(deliveryId: number, since?: string): Promise<DeliveryLocationPoint[]> {
        return await request({
            url: `/v1/delivery/${deliveryId}/track`,
            method: 'GET',
            data: since ? { since } : undefined
        })
    }
}

export type DeliveryResponse = Delivery;

export default DeliveryService
