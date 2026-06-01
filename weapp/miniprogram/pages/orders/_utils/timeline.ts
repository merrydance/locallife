/**
 * 订单时间线生成工具
 */

import { OrderResponse } from '../_main_shared/api/order'
import { buildCustomerOrderStatusView } from './customer-order-status-view'

export interface TimelineNode {
    title: string
    desc: string
    time: string
    status: 'finished' | 'active' | 'pending'
}

/**
 * 根据订单数据生成时间线
 */
export function generateOrderTimeline(order: OrderResponse): TimelineNode[] {
    const nodes: TimelineNode[] = []
    const statusView = buildCustomerOrderStatusView(order)

    // 根据订单状态生成对应的时间线节点
    switch (order.status) {
        case 'completed':
        case 'user_delivered':
            nodes.push({
                title: statusView.label,
                desc: statusView.description,
                time: order.completed_at || order.user_delivered_at || '',
                status: 'finished'
            })
            // 继续添加之前的节点
            if (order.order_type === 'takeout') {
                nodes.push({
                    title: '确认收货',
                    desc: '已确认收到订单',
                    time: order.user_delivered_at || order.completed_at || '',
                    status: 'finished'
                })
            }
            break

        case 'delivering':
        case 'courier_accepted':
        case 'picked':
        case 'rider_delivered':
            nodes.push({
                title: statusView.label,
                desc: statusView.description,
                time: '进行中',
                status: 'active'
            })
            break

        case 'ready':
            nodes.push({
                title: statusView.label,
                desc: statusView.description,
                time: '待代取',
                status: 'active'
            })
            break

        case 'preparing':
        case 'paid':
            nodes.push({
                title: statusView.label,
                desc: statusView.description,
                time: '制作中',
                status: 'active'
            })
            break
    }

    // 添加已支付节点
    if (order.paid_at) {
        nodes.push({
            title: '已支付',
            desc: '支付成功',
            time: order.paid_at,
            status: 'finished'
        })
    }

    // 添加订单创建节点
    nodes.push({
        title: '订单创建',
        desc: `订单号：${order.order_no}`,
        time: order.created_at,
        status: 'finished'
    })

    return nodes
}

/**
 * 格式化时间显示
 */
export function formatTimelineTime(timeStr: string): string {
    if (!timeStr || timeStr === '进行中' || timeStr === '待代取' || timeStr === '制作中') {
        return timeStr
    }

    try {
        const date = new Date(timeStr)
        const now = new Date()
        const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())
        const targetDate = new Date(date.getFullYear(), date.getMonth(), date.getDate())

        const hours = ('0' + date.getHours()).slice(-2)
        const minutes = ('0' + date.getMinutes()).slice(-2)
        const timeOnly = `${hours}:${minutes}`

        if (targetDate.getTime() === today.getTime()) {
            return `今天 ${timeOnly}`
        } else if (targetDate.getTime() === today.getTime() - 86400000) {
            return `昨天 ${timeOnly}`
        } else {
            const month = ('0' + (date.getMonth() + 1)).slice(-2)
            const day = ('0' + date.getDate()).slice(-2)
            return `${month}-${day} ${timeOnly}`
        }
    } catch (e) {
        return timeStr
    }
}
