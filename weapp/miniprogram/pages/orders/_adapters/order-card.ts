/**
 * 订单卡片适配器
 * 将OrderResponse转换为订单列表展示所需的OrderCard格式
 */

import { OrderPaymentContext, OrderResponse, getPayableAmount } from '../_main_shared/api/order'
import { getPublicImageUrl } from '../../../utils/image'

export interface OrderCardViewModel {
    id: number
    orderNo: string
    merchantName: string
    status: 'ready' | 'delivering' | 'preparing' | 'completed' | 'pending' | 'cancelled'
    statusClass: string
    statusLabel: string
    highlight: string
    createTimeDisplay: string
    totalDisplay: string
    totalAmount: number
    badges: string[]
    previewItems: PreviewItemViewModel[]
    canReorder: boolean
    canCancel: boolean
    canPay: boolean
    paidAt?: string
    actions?: string[]
    paymentContext?: OrderPaymentContext
    itemCount: number
    merchantId: number
    merchantPhone?: string
}

export interface PreviewItemViewModel {
    dishId: number
    dishName: string
    quantity: number
    imageUrl: string
}

export const OrderCardAdapter = {
    /**
     * 将OrderResponse转换为OrderCardViewModel
     */
    toCardViewModel(order: OrderResponse): OrderCardViewModel {
        const status = mapStatus(order)
        const actions = order.actions || []
        const canCancel = actions.includes('cancel')
        const canPay = actions.includes('pay')
        return {
            id: order.id,
            orderNo: order.order_no,
            merchantName: order.merchant_name || '未知商户',
            status,
            statusClass: status,
            statusLabel: getStatusLabel(order),
            highlight: generateHighlight(order),
            createTimeDisplay: formatCreatedAt(order.created_at),
            totalDisplay: formatPrice(getPayableAmount(order)),
            totalAmount: order.total_amount,
            badges: generateBadges(order),
            previewItems: extractPreviewItems(order),
            canReorder: ['completed', 'cancelled', 'user_delivered'].includes(order.status),
            canCancel,
            canPay,
            paidAt: order.paid_at,
            actions,
            paymentContext: order.payment_context,
            itemCount: order.items ? order.items.reduce((acc, item) => acc + item.quantity, 0) : 0,
            merchantId: order.merchant_id,
            merchantPhone: order.merchant_phone
        }
    },

    /**
     * 按状态优先级排序订单
     */
    sortByPriority(orders: OrderCardViewModel[]): OrderCardViewModel[] {
        const priority: Record<OrderCardViewModel['status'], number> = {
            preparing: 3,
            delivering: 2,
            ready: 2,
            completed: 1,
            pending: 0,
            cancelled: 0
        }

        return [...orders].sort((a, b) => {
            const diff = (priority[b.status] || 0) - (priority[a.status] || 0)
            if (diff !== 0) {
                return diff
            }
            // 相同状态按ID倒序（新订单在前）
            return b.id - a.id
        })
    }
}

/**
 * 映射订单状态到展示状态
 */
function mapStatus(order: OrderResponse): 'ready' | 'delivering' | 'preparing' | 'completed' | 'pending' | 'cancelled' {
    const status = order.status
    
    // Reservations: Prevent premature "preparing" mapping
    if (order.order_type === 'reservation' && status === 'paid') {
        if (order.fulfillment_status !== 'preparing' && order.fulfillment_status !== 'ready' && order.fulfillment_status !== 'completed') {
            return 'pending'
        }
    }

    switch (status) {
        case 'user_delivered':
        case 'completed':
            return 'completed'
        case 'cancelled':
            return 'cancelled'
        case 'courier_accepted':
        case 'picked':
        case 'rider_delivered':
        case 'delivering':
            return 'delivering'
        case 'ready':
            return 'ready'
        case 'preparing':
        case 'paid':
            return 'preparing'
        case 'pending':
            return 'pending'
        default:
            return 'pending'
    }
}

/**
 * 获取状态标签
 */
function getStatusLabel(order: OrderResponse): string {
    if (order.status_hint && order.status_hint.trim()) {
        return order.status_hint
    }
    
    const status = order.status
    
    // Reservations: Prevent premature "制作中"
    if (order.order_type === 'reservation' && status === 'paid') {
        if (order.fulfillment_status !== 'preparing' && order.fulfillment_status !== 'ready' && order.fulfillment_status !== 'completed') {
            return '等待制作'
        }
    }

    // Contextual ready labels
    if (status === 'ready') {
        if (order.order_type === 'takeaway') return '请到店取餐'
        if (order.order_type === 'dine_in' || order.order_type === 'reservation') return '已出餐/已上齐'
        return '等待跑腿接单'
    }

    const labels: Record<string, string> = {
        'pending': '待支付',
        'paid': '商家已接单',
        'preparing': '制作中',
        'courier_accepted': '骑手已接单',
        'picked': '骑手已取餐',
        'delivering': '派送中',
        'rider_delivered': '待确认收货',
        'user_delivered': '已送达',
        'completed': '已完成',
        'cancelled': '已取消'
    }
    return labels[status] || status
}

/**
 * 生成高亮信息
 */
function generateHighlight(order: OrderResponse): string {
    if (order.status_hint && order.status_hint.trim()) {
        return order.status_hint
    }
    
    if (order.order_type === 'reservation') {
        const time = order.estimated_delivery_at ? formatCreatedAt(order.estimated_delivery_at) : ''
        return time ? `预约时间: ${time.replace('今天 · ', '').replace('昨天 · ', '')}` : '预订点菜订单'
    }
    if (order.order_type === 'dine_in') {
        return order.table_id ? `堂食 - 桌号: ${order.table_id}` : '堂食订单'
    }
    if (order.order_type === 'takeaway') {
        return '打包自取订单'
    }

    switch (order.status) {
        case 'courier_accepted':
            return '骑手已接单，正在前往取餐'
        case 'picked':
            return '骑手已取餐，正在代取'
        case 'delivering':
            return '骑手正在代取中，请耐心等待'
        case 'rider_delivered':
            return '订单已送达，请确认收餐'
        case 'ready':
            return '商家已备餐，等待跑腿接单'
        case 'preparing':
            return '商家正在制作您的餐品'
        case 'completed':
            return '订单已完成，感谢您的惠顾'
        default:
            return ''
    }
}

/**
 * 生成订单徽章
 */
function generateBadges(order: OrderResponse): string[] {
    const badges: string[] = []

    // 订单类型徽章
    if (order.order_type === 'takeout') {
        badges.push('外卖')
    } else if (order.order_type === 'dine_in') {
        badges.push('堂食')
    } else if (order.order_type === 'takeaway') {
        badges.push('自取')
    } else if (order.order_type === 'reservation') {
        badges.push('预订')
    }

    // 支付方式徽章（OrderResponse中没有payment_method字段）
    // 暂时注释

    // 优惠徽章
    if (order.discount_amount > 0) {
        badges.push(`已减¥${(order.discount_amount / 100).toFixed(2)}`)
    }

    const extraBadges = normalizeBadges(order.badges)
    return [...badges, ...extraBadges]
}

function normalizeBadges(badges?: Array<{ text: string }> | string[]): string[] {
    if (!badges || badges.length === 0) return []
    if (typeof badges[0] === 'string') {
        return badges as string[]
    }
    return (badges as Array<{ text: string }>).map((badge) => badge.text).filter(Boolean)
}

/**
 * 提取前3个菜品作为预览
 */
function extractPreviewItems(order: OrderResponse): PreviewItemViewModel[] {
    if (!order.items || order.items.length === 0) {
        return []
    }

    return order.items.slice(0, 3).map((item) => ({
        dishId: item.dish_id || 0,
        dishName: item.name,  // 对齐swagger: 使用name而非dish_name
        quantity: item.quantity,
        imageUrl: getPublicImageUrl(item.image_url) || 'https://tdesign.gtimg.com/mobile/demos/example1.png'  // 对齐swagger: 使用image_url
    }))
}

/**
 * 格式化价格
 */
function formatPrice(amount: number): string {
    return `¥${(amount / 100).toFixed(2)}`
}

/**
 * 格式化创建时间
 */
function formatCreatedAt(timeStr: string): string {
    try {
        const date = new Date(timeStr)
        const now = new Date()
        const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())
        const targetDate = new Date(date.getFullYear(), date.getMonth(), date.getDate())

        const hours = ('0' + date.getHours()).slice(-2)
        const minutes = ('0' + date.getMinutes()).slice(-2)
        const timeOnly = `${hours}:${minutes}`

        if (targetDate.getTime() === today.getTime()) {
            return `今天 · ${timeOnly}`
        } else if (targetDate.getTime() === today.getTime() - 86400000) {
            return `昨天 · ${timeOnly}`
        } else {
            const month = ('0' + (date.getMonth() + 1)).slice(-2)
            const day = ('0' + date.getDate()).slice(-2)
            return `${month}-${day} · ${timeOnly}`
        }
    } catch (e) {
        return timeStr
    }
}

